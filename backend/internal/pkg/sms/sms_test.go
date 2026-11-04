package sms

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 11, 4, 8, 30, 0, 0, time.UTC)

func TestAliyunPercentEncode(t *testing.T) {
	cases := map[string]string{
		"a b":       "a%20b",
		"a*b":       "a%2Ab",
		"~x":        "~x",
		`{"n":"中"}`: "%7B%22n%22%3A%22%E4%B8%AD%22%7D",
	}
	for in, want := range cases {
		if got := aliyunPercentEncode(in); got != want {
			t.Fatalf("aliyunPercentEncode(%q) = %q, want %q", in, got, want)
		}
	}
	if got := aliyunCanonicalQuery(map[string]string{"b": "2", "a": "1 1"}); got != "a=1%201&b=2" {
		t.Fatalf("canonical query = %q", got)
	}
}

func TestAliyunSendSignsRequest(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r.Clone(context.Background())
		_, _ = io.WriteString(w, `{"Code":"OK","Message":"OK"}`)
	}))
	defer srv.Close()

	p := &aliyunProvider{
		cfg:    Config{Provider: ProviderAliyun, AccessKeyID: "ak", AccessKeySecret: "secret", SignName: "签名", Endpoint: srv.URL},
		client: srv.Client(),
		now:    func() time.Time { return fixedNow },
		nonce:  func() string { return "nonce-1" },
	}
	err := p.Send(context.Background(), Message{
		Phones:     []string{"+8613800000000", " 13900000000 ", "13900000000"},
		TemplateID: "SMS_1",
		Params:     []Param{{Name: "count", Value: "2"}, {Name: "name", Value: "relay a"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	q := gotReq.URL.Query()
	if q.Get("PhoneNumbers") != "8613800000000,13900000000" || q.Get("TemplateCode") != "SMS_1" || q.Get("SignName") != "签名" {
		t.Fatalf("unexpected query: %v", q)
	}
	var tp map[string]string
	if err := json.Unmarshal([]byte(q.Get("TemplateParam")), &tp); err != nil || tp["count"] != "2" || tp["name"] != "relay a" {
		t.Fatalf("template param = %q", q.Get("TemplateParam"))
	}
	if gotReq.Header.Get("x-acs-action") != "SendSms" || gotReq.Header.Get("x-acs-date") != "2026-11-04T08:30:00Z" {
		t.Fatalf("unexpected headers: %v", gotReq.Header)
	}

	// 按规范独立重算签名。
	host := strings.TrimPrefix(srv.URL, "http://")
	canonical := "POST\n/\n" + gotReq.URL.RawQuery + "\n" +
		"host:" + host + "\n" +
		"x-acs-action:SendSms\n" +
		"x-acs-content-sha256:" + sha256Hex("") + "\n" +
		"x-acs-date:2026-11-04T08:30:00Z\n" +
		"x-acs-signature-nonce:nonce-1\n" +
		"x-acs-version:2017-05-25\n" +
		"\nhost;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version\n" +
		sha256Hex("")
	sig := hex.EncodeToString(hmacSHA256([]byte("secret"), "ACS3-HMAC-SHA256\n"+sha256Hex(canonical)))
	want := "ACS3-HMAC-SHA256 Credential=ak,SignedHeaders=host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version,Signature=" + sig
	if got := gotReq.Header.Get("Authorization"); got != want {
		t.Fatalf("authorization\n got %s\nwant %s", got, want)
	}
}

func TestAliyunSendReportsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"Code":"isv.BUSINESS_LIMIT_CONTROL","Message":"触发流控"}`)
	}))
	defer srv.Close()
	p, err := New(Config{Provider: ProviderAliyun, AccessKeyID: "ak", AccessKeySecret: "s", SignName: "x", Endpoint: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	err = p.Send(context.Background(), Message{Phones: []string{"13800000000"}, TemplateID: "T"})
	if err == nil || !strings.Contains(err.Error(), "BUSINESS_LIMIT_CONTROL") {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestTencentSendSignsRequest(t *testing.T) {
	var gotReq *http.Request
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r.Clone(context.Background())
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"Response":{"SendStatusSet":[{"Code":"Ok","PhoneNumber":"+8613800000000"}],"RequestId":"r"}}`)
	}))
	defer srv.Close()

	p := &tencentProvider{
		cfg:    Config{Provider: ProviderTencent, AccessKeyID: "AKID", AccessKeySecret: "key", SignName: "签名", SdkAppID: "1400000000", Endpoint: srv.URL},
		client: srv.Client(),
		now:    func() time.Time { return fixedNow },
	}
	err := p.Send(context.Background(), Message{
		Phones:     []string{"13800000000"},
		TemplateID: "123",
		Params:     []Param{{Name: "count", Value: "2"}, {Name: "name", Value: "relay"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	var body struct {
		PhoneNumberSet   []string
		SmsSdkAppId      string
		TemplateParamSet []string
	}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatal(err)
	}
	if body.PhoneNumberSet[0] != "+8613800000000" || body.SmsSdkAppId != "1400000000" || strings.Join(body.TemplateParamSet, ",") != "2,relay" {
		t.Fatalf("unexpected body: %s", gotBody)
	}
	if gotReq.Header.Get("X-TC-Timestamp") != "1793781000" || gotReq.Header.Get("X-TC-Action") != "SendSms" {
		t.Fatalf("unexpected headers: %v", gotReq.Header)
	}

	host := strings.TrimPrefix(srv.URL, "http://")
	canonical := "POST\n/\n\n" +
		"content-type:application/json; charset=utf-8\n" +
		"host:" + host + "\n" +
		"x-tc-action:sendsms\n" +
		"\ncontent-type;host;x-tc-action\n" + sha256Hex(string(gotBody))
	stringToSign := "TC3-HMAC-SHA256\n1793781000\n2026-11-04/sms/tc3_request\n" + sha256Hex(canonical)
	k := hmacSHA256(hmacSHA256(hmacSHA256([]byte("TC3key"), "2026-11-04"), "sms"), "tc3_request")
	want := "TC3-HMAC-SHA256 Credential=AKID/2026-11-04/sms/tc3_request, SignedHeaders=content-type;host;x-tc-action, Signature=" + hex.EncodeToString(hmacSHA256(k, stringToSign))
	if got := gotReq.Header.Get("Authorization"); got != want {
		t.Fatalf("authorization\n got %s\nwant %s", got, want)
	}
}

func TestTencentSendReportsErrors(t *testing.T) {
	for name, resp := range map[string]string{
		"api error":       `{"Response":{"Error":{"Code":"AuthFailure.SignatureFailure","Message":"bad"}}}`,
		"partial failure": `{"Response":{"SendStatusSet":[{"Code":"LimitExceeded.PhoneNumberDailyLimit","PhoneNumber":"+86138"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, resp)
			}))
			defer srv.Close()
			p, err := New(Config{Provider: ProviderTencent, AccessKeyID: "a", AccessKeySecret: "b", SignName: "c", SdkAppID: "1", Endpoint: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Send(context.Background(), Message{Phones: []string{"138"}, TemplateID: "1"}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(Config{Provider: ProviderAliyun}); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
	if _, err := New(Config{Provider: ProviderTencent, AccessKeyID: "a", AccessKeySecret: "b", SignName: "c"}); err != ErrNotConfigured {
		t.Fatalf("tencent without SdkAppID: %v", err)
	}
	if _, err := New(Config{Provider: "foo", AccessKeyID: "a", AccessKeySecret: "b", SignName: "c"}); err == nil {
		t.Fatal("expected unknown provider error")
	}
}
