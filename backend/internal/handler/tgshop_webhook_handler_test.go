package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// clientSign reproduces telegram-shop's sub2api.Client.sign exactly, so this
// test fails if either side's signing scheme drifts.
func clientSign(secret, timestamp, nonce string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write([]byte(nonce))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func newSignedRequest(secret, ts, nonce string, body []byte) *gin.Context {
	req := httptest.NewRequest("POST", "/api/v1/payment/webhook/tgshop", strings.NewReader(string(body)))
	req.Header.Set("X-TGShop-Timestamp", ts)
	req.Header.Set("X-TGShop-Nonce", nonce)
	req.Header.Set("X-TGShop-Signature", clientSign(secret, ts, nonce, body))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

func TestTGShopVerifySignature(t *testing.T) {
	const secret = "shared-secret-123"
	h := &TGShopWebhookHandler{secret: secret}
	body := []byte(`{"order_no":"tgshop_20240614abc","email":"u@example.com","amount":50,"status":"success"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	// Valid signature accepted.
	if !h.verifySignature(newSignedRequest(secret, ts, "nonce-1", body), body) {
		t.Fatal("valid signature rejected")
	}

	// Wrong secret rejected.
	bad := newSignedRequest("wrong-secret", ts, "nonce-2", body)
	if h.verifySignature(bad, body) {
		t.Fatal("signature with wrong secret accepted")
	}

	// Stale timestamp rejected (outside 5-minute window).
	stale := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if h.verifySignature(newSignedRequest(secret, stale, "nonce-3", body), body) {
		t.Fatal("stale timestamp accepted")
	}

	// Tampered body rejected (signature computed over original body).
	tampered := newSignedRequest(secret, ts, "nonce-4", body)
	if h.verifySignature(tampered, []byte(`{"amount":9999}`)) {
		t.Fatal("tampered body accepted")
	}
}
