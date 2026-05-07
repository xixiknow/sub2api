package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestDulupaySignAndVerify(t *testing.T) {
	t.Parallel()

	key := mustDulupayTestKey(t)
	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "sub2_1",
		"money":        "12.30",
		"sign":         "ignored",
		"sign_type":    "SHA256WithRSA",
		"empty":        "",
	}
	sign, err := dulupaySign(params, key)
	if err != nil {
		t.Fatalf("dulupaySign error: %v", err)
	}
	if sign == "" {
		t.Fatal("expected non-empty signature")
	}
	if !dulupayVerifySign(params, &key.PublicKey, sign) {
		t.Fatal("expected valid signature")
	}
	params["money"] = "99.99"
	if dulupayVerifySign(params, &key.PublicKey, sign) {
		t.Fatal("tampered params should not verify")
	}
}

func TestDulupayCreatePaymentQRCode(t *testing.T) {
	t.Parallel()

	key := mustDulupayTestKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pay/create" {
			t.Fatalf("path = %q, want /api/pay/create", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		params := dulupayFormToMap(r.PostForm)
		if params["pid"] != "pid-1" || params["out_trade_no"] != "sub2_100" || params["type"] != "alipay" {
			t.Fatalf("unexpected params: %#v", params)
		}
		if !dulupayVerifySign(params, &key.PublicKey, params["sign"]) {
			t.Fatalf("provider request signature did not verify: %#v", params)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok","trade_no":"dp_1","pay_type":"qrcode","pay_info":"https://pay.example/qr"}`))
	}))
	defer server.Close()

	prov, err := NewDulupay("inst-1", map[string]string{
		"pid":                "pid-1",
		"merchantPrivateKey": dulupayPrivateKeyPEM(t, key),
		"platformPublicKey":  dulupayPublicKeyPEM(t, &key.PublicKey),
		"apiBase":            server.URL,
	})
	if err != nil {
		t.Fatalf("NewDulupay: %v", err)
	}
	resp, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2_100",
		Amount:      "12.30",
		PaymentType: payment.TypeAlipay,
		Subject:     "Balance recharge",
		ClientIP:    "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if resp.TradeNo != "dp_1" || resp.QRCode != "https://pay.example/qr" || resp.PayURL != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDulupayVerifyNotification(t *testing.T) {
	t.Parallel()

	key := mustDulupayTestKey(t)
	prov := &Dulupay{
		config:    map[string]string{"pid": "pid-1"},
		publicKey: &key.PublicKey,
	}
	params := map[string]string{
		"pid":          "pid-1",
		"trade_no":     "dp_1",
		"out_trade_no": "sub2_100",
		"trade_status": "TRADE_SUCCESS",
		"money":        "12.30",
		"timestamp":    "1760000000",
	}
	sign, err := dulupaySign(params, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("sign", sign)
	values.Set("sign_type", "SHA256WithRSA")

	notify, err := prov.VerifyNotification(context.Background(), values.Encode(), nil)
	if err != nil {
		t.Fatalf("VerifyNotification: %v", err)
	}
	if notify.OrderID != "sub2_100" || notify.TradeNo != "dp_1" || notify.Status != payment.ProviderStatusSuccess {
		t.Fatalf("unexpected notification: %+v", notify)
	}
}

func dulupayFormToMap(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for k := range values {
		out[k] = values.Get(k)
	}
	return out
}

func mustDulupayTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key
}

func dulupayPrivateKeyPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	return strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})))
}

func dulupayPublicKeyPEM(t *testing.T, key *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	return strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})))
}
