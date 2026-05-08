package provider

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	dulupayDefaultAPIBase  = "https://api.dulupay.com"
	dulupayHTTPTimeout     = 10 * time.Second
	dulupaySignTypeRSA     = "RSA"
	dulupaySignTypeSHA256  = "SHA256WithRSA"
	dulupayDefaultMethod   = "web"
	dulupayDevicePC        = "pc"
	dulupayDeviceMobile    = "mobile"
	dulupayNotifySuccess   = "TRADE_SUCCESS"
	dulupayStatusUnpaid    = "0"
	dulupayStatusPaid      = "1"
	dulupayStatusRefunded  = "2"
	maxDulupayResponseSize = 1 << 20
)

// Dulupay implements payment.Provider for Dulupay V2 RSA payments.
type Dulupay struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// NewDulupay creates a Dulupay provider.
// config keys: pid, merchantPrivateKey, platformPublicKey, apiBase, notifyUrl, returnUrl
func NewDulupay(instanceID string, config map[string]string) (*Dulupay, error) {
	for _, k := range []string{"pid", "merchantPrivateKey", "platformPublicKey"} {
		if strings.TrimSpace(config[k]) == "" {
			return nil, fmt.Errorf("dulupay config missing required key: %s", k)
		}
	}

	privateKey, err := parseRSAPrivateKey(config["merchantPrivateKey"])
	if err != nil {
		return nil, fmt.Errorf("dulupay parse merchantPrivateKey: %w", err)
	}
	publicKey, err := parseRSAPublicKey(config["platformPublicKey"])
	if err != nil {
		return nil, fmt.Errorf("dulupay parse platformPublicKey: %w", err)
	}

	cfg := cloneStringMap(config)
	cfg["apiBase"] = normalizeDulupayAPIBase(cfg["apiBase"])
	if cfg["apiBase"] == "" {
		cfg["apiBase"] = dulupayDefaultAPIBase
	}

	return &Dulupay{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: dulupayHTTPTimeout},
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

func (d *Dulupay) Name() string        { return "Dulupay" }
func (d *Dulupay) ProviderKey() string { return payment.TypeDulupay }
func (d *Dulupay) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay, payment.TypeWxpay}
}

func (d *Dulupay) MerchantIdentityMetadata() map[string]string {
	if d == nil {
		return nil
	}
	pid := strings.TrimSpace(d.config["pid"])
	if pid == "" {
		return nil
	}
	return map[string]string{"pid": pid}
}

func (d *Dulupay) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	notifyURL, returnURL := d.resolveURLs(req)
	params := map[string]string{
		"pid":          d.config["pid"],
		"method":       dulupayValueOrDefault(d.config["method"], dulupayDefaultMethod),
		"device":       dulupayDevice(req.IsMobile),
		"type":         req.PaymentType,
		"out_trade_no": req.OrderID,
		"notify_url":   notifyURL,
		"return_url":   returnURL,
		"name":         req.Subject,
		"money":        req.Amount,
		"clientip":     req.ClientIP,
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
	}
	body, err := d.postSigned(ctx, "/api/pay/create", params)
	if err != nil {
		return nil, fmt.Errorf("dulupay create: %w", err)
	}
	var resp struct {
		Code         any    `json:"code"`
		Msg          string `json:"msg"`
		TradeNo      string `json:"trade_no"`
		PayType      string `json:"pay_type"`
		PayInfo      string `json:"pay_info"`
		PayURL       string `json:"payurl"`
		PayURLAlt    string `json:"pay_url"`
		QRCode       string `json:"qrcode"`
		QRCodeAlt    string `json:"qr_code"`
		URLScheme    string `json:"urlscheme"`
		URLSchemeAlt string `json:"url_scheme"`
		Sign         string `json:"sign"`
		SignType     string `json:"sign_type"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("dulupay parse create: %w", err)
	}
	if !dulupayCodeOK(resp.Code) {
		return nil, fmt.Errorf("dulupay error: %s", strings.TrimSpace(resp.Msg))
	}

	result := &payment.CreatePaymentResponse{TradeNo: strings.TrimSpace(resp.TradeNo)}
	dulupayApplyTypedPaymentInfo(result, resp.PayType, resp.PayInfo)
	dulupayApplyDirectPaymentFields(result, resp.PayURL, resp.PayURLAlt, resp.QRCode, resp.QRCodeAlt, resp.URLScheme, resp.URLSchemeAlt)
	return result, nil
}

func dulupayApplyTypedPaymentInfo(result *payment.CreatePaymentResponse, payType, payInfo string) {
	if result == nil {
		return
	}
	payType = strings.ToLower(strings.TrimSpace(payType))
	payInfo = strings.TrimSpace(payInfo)
	if payInfo == "" {
		return
	}
	switch payType {
	case "qrcode":
		dulupaySetQRCode(result, payInfo)
	case "jump":
		dulupaySetPayURL(result, payInfo)
	case "urlscheme", "jsapi":
		dulupaySetPayURL(result, payInfo)
	case "html":
		dulupaySetPayURL(result, dulupayHTMLPayURL(payInfo))
	default:
		if dulupayLooksLikeURL(payInfo) {
			dulupaySetPayURL(result, payInfo)
		} else {
			dulupaySetQRCode(result, payInfo)
		}
	}
}

func dulupayApplyDirectPaymentFields(result *payment.CreatePaymentResponse, payURL, payURLAlt, qrCode, qrCodeAlt, urlScheme, urlSchemeAlt string) {
	if result == nil {
		return
	}
	dulupaySetQRCode(result, qrCode)
	dulupaySetQRCode(result, qrCodeAlt)
	dulupaySetPayURL(result, payURL)
	dulupaySetPayURL(result, payURLAlt)
	dulupaySetPayURL(result, urlScheme)
	dulupaySetPayURL(result, urlSchemeAlt)
}

func dulupaySetPayURL(result *payment.CreatePaymentResponse, payURL string) {
	payURL = strings.TrimSpace(payURL)
	if result == nil || payURL == "" {
		return
	}
	if result.PayURL == "" {
		result.PayURL = payURL
	}
}

func dulupaySetQRCode(result *payment.CreatePaymentResponse, qrCode string) {
	qrCode = strings.TrimSpace(qrCode)
	if result == nil || qrCode == "" || result.QRCode != "" {
		return
	}
	result.QRCode = qrCode
}

func dulupayLooksLikeURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.Contains(value, "://")
}

func (d *Dulupay) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, fmt.Errorf("dulupay query missing trade number")
	}
	params := map[string]string{
		"pid":          d.config["pid"],
		"out_trade_no": tradeNo,
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
	}
	body, err := d.postSigned(ctx, "/api/pay/query", params)
	if err != nil {
		return nil, fmt.Errorf("dulupay query: %w", err)
	}
	var resp struct {
		Code        any    `json:"code"`
		Msg         string `json:"msg"`
		TradeNo     string `json:"trade_no"`
		OutTradeNo  string `json:"out_trade_no"`
		Status      any    `json:"status"`
		TradeStatus string `json:"trade_status"`
		Money       string `json:"money"`
		PaidAt      string `json:"paid_at"`
		EndTime     string `json:"endtime"`
		EndTimeAlt  string `json:"end_time"`
		AddTime     string `json:"addtime"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("dulupay parse query: %w", err)
	}
	if !dulupayCodeOK(resp.Code) {
		return nil, fmt.Errorf("dulupay query error: %s", strings.TrimSpace(resp.Msg))
	}
	status := payment.ProviderStatusPending
	switch strings.TrimSpace(fmt.Sprint(resp.Status)) {
	case dulupayStatusPaid:
		status = payment.ProviderStatusPaid
	case dulupayStatusRefunded:
		status = payment.ProviderStatusRefunded
	case dulupayStatusUnpaid, "":
		status = payment.ProviderStatusPending
	default:
		status = payment.ProviderStatusFailed
	}
	if strings.EqualFold(resp.TradeStatus, dulupayNotifySuccess) {
		status = payment.ProviderStatusPaid
	}
	amount, _ := strconv.ParseFloat(resp.Money, 64)
	paidAt := strings.TrimSpace(resp.PaidAt)
	if paidAt == "" {
		paidAt = strings.TrimSpace(resp.EndTime)
	}
	if paidAt == "" {
		paidAt = strings.TrimSpace(resp.EndTimeAlt)
	}
	if paidAt == "" {
		paidAt = strings.TrimSpace(resp.AddTime)
	}
	return &payment.QueryOrderResponse{
		TradeNo:  dulupayValueOrDefault(resp.TradeNo, tradeNo),
		Status:   status,
		Amount:   amount,
		PaidAt:   paidAt,
		Metadata: d.MerchantIdentityMetadata(),
	}, nil
}

func (d *Dulupay) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return nil, fmt.Errorf("parse notify: %w", err)
	}
	params := make(map[string]string, len(values))
	for k := range values {
		params[k] = values.Get(k)
	}
	sign := params["sign"]
	if sign == "" {
		return nil, fmt.Errorf("missing sign")
	}
	if signType := strings.TrimSpace(params["sign_type"]); signType != "" && !dulupaySignTypeSupported(signType) {
		return nil, fmt.Errorf("unsupported sign_type: %s", signType)
	}
	if !dulupayVerifySign(params, d.publicKey, sign) {
		return nil, fmt.Errorf("invalid signature")
	}

	status := payment.ProviderStatusFailed
	if params["trade_status"] == dulupayNotifySuccess || strings.TrimSpace(params["status"]) == dulupayStatusPaid {
		status = payment.ProviderStatusSuccess
	}
	amount, _ := strconv.ParseFloat(params["money"], 64)
	metadata := d.MerchantIdentityMetadata()
	if metadata == nil {
		metadata = map[string]string{}
	}
	for _, k := range []string{"pid", "api_trade_no", "type"} {
		if v := strings.TrimSpace(params[k]); v != "" {
			metadata[k] = v
		}
	}
	return &payment.PaymentNotification{
		TradeNo:  params["trade_no"],
		OrderID:  params["out_trade_no"],
		Amount:   amount,
		Status:   status,
		RawData:  rawBody,
		Metadata: metadata,
	}, nil
}

func (d *Dulupay) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	params := map[string]string{
		"pid":       d.config["pid"],
		"money":     req.Amount,
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	if orderID := strings.TrimSpace(req.OrderID); orderID != "" {
		params["out_trade_no"] = orderID
	} else if tradeNo := strings.TrimSpace(req.TradeNo); tradeNo != "" {
		params["trade_no"] = tradeNo
	} else {
		return nil, fmt.Errorf("dulupay refund missing order identifier")
	}
	if reason := strings.TrimSpace(req.Reason); reason != "" {
		params["reason"] = reason
	}
	body, err := d.postSigned(ctx, "/api/pay/refund", params)
	if err != nil {
		return nil, fmt.Errorf("dulupay refund: %w", err)
	}
	var resp struct {
		Code     any    `json:"code"`
		Msg      string `json:"msg"`
		RefundID string `json:"refund_id"`
		TradeNo  string `json:"trade_no"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("dulupay parse refund: %w", err)
	}
	if !dulupayCodeOK(resp.Code) {
		return nil, fmt.Errorf("dulupay refund failed: %s", strings.TrimSpace(resp.Msg))
	}
	return &payment.RefundResponse{
		RefundID: dulupayValueOrDefault(resp.RefundID, dulupayValueOrDefault(resp.TradeNo, req.OrderID)),
		Status:   payment.ProviderStatusSuccess,
	}, nil
}

func (d *Dulupay) resolveURLs(req payment.CreatePaymentRequest) (string, string) {
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = d.config["notifyUrl"]
	}
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = d.config["returnUrl"]
	}
	return notifyURL, returnURL
}

func (d *Dulupay) postSigned(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	params = cloneStringMap(params)
	sign, err := dulupaySign(params, d.privateKey)
	if err != nil {
		return nil, err
	}
	params["sign"] = sign
	params["sign_type"] = dulupaySignTypeRSA

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint(path), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := d.httpClient
	if client == nil {
		client = &http.Client{Timeout: dulupayHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDulupayResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.Join(strings.Fields(string(body)), " "))
	}
	return body, nil
}

func (d *Dulupay) endpoint(path string) string {
	return strings.TrimRight(d.config["apiBase"], "/") + "/" + strings.TrimLeft(path, "/")
}

func normalizeDulupayAPIBase(apiBase string) string {
	base := strings.TrimSpace(apiBase)
	if base == "" {
		return ""
	}
	if parsed, err := url.Parse(base); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.RawQuery = ""
		parsed.Fragment = ""
		parsed.RawPath = ""
		parsed.Path = trimDulupayEndpointPath(parsed.Path)
		return strings.TrimRight(parsed.String(), "/")
	}
	return strings.TrimRight(trimDulupayEndpointPath(base), "/")
}

func trimDulupayEndpointPath(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	lower := strings.ToLower(path)
	for _, endpoint := range []string{"/api/pay/create", "/api/pay/query", "/api/pay/refund"} {
		if strings.HasSuffix(lower, endpoint) {
			return strings.TrimRight(path[:len(path)-len(endpoint)], "/")
		}
	}
	return path
}

func dulupaySign(params map[string]string, key *rsa.PrivateKey) (string, error) {
	if key == nil {
		return "", fmt.Errorf("missing RSA private key")
	}
	digest := sha256.Sum256([]byte(dulupaySignContent(params)))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func dulupayVerifySign(params map[string]string, key *rsa.PublicKey, sign string) bool {
	if key == nil {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sign))
	if err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(dulupaySignContent(params)))
	err = rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig)
	return err == nil
}

func dulupaySignTypeSupported(signType string) bool {
	signType = strings.TrimSpace(signType)
	return strings.EqualFold(signType, dulupaySignTypeRSA) || strings.EqualFold(signType, dulupaySignTypeSHA256)
}

func dulupaySignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			_ = buf.WriteByte('&')
		}
		_, _ = buf.WriteString(k + "=" + params[k])
	}
	return buf.String()
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, err := parsePEMBlock(raw, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return key, nil
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	block, err := parsePEMBlock(raw, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
		return nil, fmt.Errorf("not an RSA public key")
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func parsePEMBlock(raw string, defaultLabel string) (*pem.Block, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, fmt.Errorf("empty key")
	}
	if !strings.Contains(text, "-----BEGIN ") {
		text = "-----BEGIN " + defaultLabel + "-----\n" + chunkBase64(text, 64) + "\n-----END " + defaultLabel + "-----"
	}
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	return block, nil
}

func chunkBase64(s string, n int) string {
	if n <= 0 {
		return s
	}
	s = strings.Join(strings.Fields(s), "")
	if len(s) <= n {
		return s
	}
	var b strings.Builder
	for len(s) > n {
		b.WriteString(s[:n])
		b.WriteByte('\n')
		s = s[n:]
	}
	b.WriteString(s)
	return b.String()
}

func dulupayDevice(isMobile bool) string {
	if isMobile {
		return dulupayDeviceMobile
	}
	return dulupayDevicePC
}

func dulupayCodeOK(code any) bool {
	switch v := code.(type) {
	case float64:
		return int(v) == 0 || int(v) == 1
	case int:
		return v == 0 || v == 1
	case string:
		switch strings.TrimSpace(v) {
		case "0", "1":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func dulupayValueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func dulupayHTMLPayURL(html string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return ""
	}
	return "data:text/html;charset=utf-8," + url.QueryEscape(html)
}

var _ payment.Provider = (*Dulupay)(nil)
var _ payment.MerchantIdentityProvider = (*Dulupay)(nil)
