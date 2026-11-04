package sms

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	aliyunDefaultEndpoint = "https://dysmsapi.aliyuncs.com"
	aliyunAction          = "SendSms"
	aliyunVersion         = "2017-05-25"
	aliyunAlgorithm       = "ACS3-HMAC-SHA256"
)

// aliyunProvider 调用 dysmsapi SendSms，使用 V3（ACS3-HMAC-SHA256）签名。
type aliyunProvider struct {
	cfg    Config
	client *http.Client
	now    func() time.Time
	nonce  func() string
}

func (p *aliyunProvider) Name() string { return ProviderAliyun }

func (p *aliyunProvider) Send(ctx context.Context, msg Message) error {
	phones := normalizePhones(msg.Phones)
	if len(phones) == 0 {
		return fmt.Errorf("no phone numbers")
	}
	for i, phone := range phones {
		phones[i] = strings.TrimPrefix(phone, "+")
	}
	params := make(map[string]string, len(msg.Params))
	for _, param := range msg.Params {
		params[param.Name] = param.Value
	}
	templateParam, err := json.Marshal(params)
	if err != nil {
		return err
	}
	query := map[string]string{
		"PhoneNumbers":  strings.Join(phones, ","),
		"SignName":      p.cfg.SignName,
		"TemplateCode":  msg.TemplateID,
		"TemplateParam": string(templateParam),
	}
	endpoint := strings.TrimRight(p.cfg.Endpoint, "/")
	if endpoint == "" {
		endpoint = aliyunDefaultEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	queryString := aliyunCanonicalQuery(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/?"+queryString, nil)
	if err != nil {
		return err
	}
	headers := map[string]string{
		"host":                  parsed.Host,
		"x-acs-action":          aliyunAction,
		"x-acs-version":         aliyunVersion,
		"x-acs-date":            p.now().UTC().Format("2006-01-02T15:04:05Z"),
		"x-acs-signature-nonce": p.nonce(),
		"x-acs-content-sha256":  sha256Hex(""),
	}
	req.Header.Set("Authorization", aliyunAuthorization(p.cfg.AccessKeyID, p.cfg.AccessKeySecret, http.MethodPost, queryString, headers, ""))
	for k, v := range headers {
		if k == "host" {
			continue
		}
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("aliyun sms request failed: %w", err)
	}
	body, err := readBody(resp)
	if err != nil {
		return fmt.Errorf("aliyun sms read response: %w", err)
	}
	var out struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("aliyun sms http %d: invalid response", resp.StatusCode)
	}
	if out.Code != "OK" {
		return fmt.Errorf("aliyun sms %s: %s", out.Code, out.Message)
	}
	return nil
}

// aliyunAuthorization 生成 V3 签名的 Authorization 头。headers 的 key 必须为小写。
func aliyunAuthorization(accessKeyID, secret, method, canonicalQuery string, headers map[string]string, body string) string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	var canonicalHeaders strings.Builder
	for _, name := range names {
		canonicalHeaders.WriteString(name + ":" + strings.TrimSpace(headers[name]) + "\n")
	}
	signedHeaders := strings.Join(names, ";")
	canonicalRequest := strings.Join([]string{
		method,
		"/",
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		sha256Hex(body),
	}, "\n")
	stringToSign := aliyunAlgorithm + "\n" + sha256Hex(canonicalRequest)
	signature := hex.EncodeToString(hmacSHA256([]byte(secret), stringToSign))
	return fmt.Sprintf("%s Credential=%s,SignedHeaders=%s,Signature=%s", aliyunAlgorithm, accessKeyID, signedHeaders, signature)
}

// aliyunCanonicalQuery 按 key 排序，RFC3986 编码（空格为 %20，保留 ~）。
func aliyunCanonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, aliyunPercentEncode(k)+"="+aliyunPercentEncode(params[k]))
	}
	return strings.Join(parts, "&")
}

func aliyunPercentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	return strings.ReplaceAll(encoded, "%7E", "~")
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func randomNonce() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
