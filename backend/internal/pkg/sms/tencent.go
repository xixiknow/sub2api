package sms

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	tencentDefaultEndpoint = "https://sms.tencentcloudapi.com"
	tencentService         = "sms"
	tencentAction          = "SendSms"
	tencentVersion         = "2021-01-11"
	tencentDefaultRegion   = "ap-guangzhou"
	tencentAlgorithm       = "TC3-HMAC-SHA256"
	tencentContentType     = "application/json; charset=utf-8"
)

// tencentProvider 调用腾讯云 SendSms 2021-01-11，使用 TC3-HMAC-SHA256 签名。
type tencentProvider struct {
	cfg    Config
	client *http.Client
	now    func() time.Time
}

func (p *tencentProvider) Name() string { return ProviderTencent }

func (p *tencentProvider) Send(ctx context.Context, msg Message) error {
	phones := normalizePhones(msg.Phones)
	if len(phones) == 0 {
		return fmt.Errorf("no phone numbers")
	}
	for i, phone := range phones {
		// 腾讯云要求 E.164 格式，未带国家码的默认按中国大陆。
		if !strings.HasPrefix(phone, "+") {
			phones[i] = "+86" + phone
		}
	}
	values := make([]string, len(msg.Params))
	for i, param := range msg.Params {
		values[i] = param.Value
	}
	payload, err := json.Marshal(map[string]any{
		"PhoneNumberSet":   phones,
		"SmsSdkAppId":      p.cfg.SdkAppID,
		"SignName":         p.cfg.SignName,
		"TemplateId":       msg.TemplateID,
		"TemplateParamSet": values,
	})
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(p.cfg.Endpoint, "/")
	if endpoint == "" {
		endpoint = tencentDefaultEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	region := p.cfg.Region
	if region == "" {
		region = tencentDefaultRegion
	}
	now := p.now().UTC()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", tencentContentType)
	req.Header.Set("Host", parsed.Host)
	req.Header.Set("X-TC-Action", tencentAction)
	req.Header.Set("X-TC-Version", tencentVersion)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(now.Unix(), 10))
	req.Header.Set("X-TC-Region", region)
	req.Header.Set("Authorization", tencentAuthorization(p.cfg.AccessKeyID, p.cfg.AccessKeySecret, parsed.Host, string(payload), now))

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("tencent sms request failed: %w", err)
	}
	body, err := readBody(resp)
	if err != nil {
		return fmt.Errorf("tencent sms read response: %w", err)
	}
	var out struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
			SendStatusSet []struct {
				Code        string `json:"Code"`
				Message     string `json:"Message"`
				PhoneNumber string `json:"PhoneNumber"`
			} `json:"SendStatusSet"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("tencent sms http %d: invalid response", resp.StatusCode)
	}
	if out.Response.Error != nil {
		return fmt.Errorf("tencent sms %s: %s", out.Response.Error.Code, out.Response.Error.Message)
	}
	var failed []string
	for _, status := range out.Response.SendStatusSet {
		if !strings.EqualFold(status.Code, "Ok") {
			failed = append(failed, status.PhoneNumber+" "+status.Code+": "+status.Message)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("tencent sms partial failure: %s", strings.Join(failed, "; "))
	}
	return nil
}

// tencentAuthorization 生成 TC3-HMAC-SHA256 的 Authorization 头。
func tencentAuthorization(secretID, secretKey, host, payload string, now time.Time) string {
	date := now.UTC().Format("2006-01-02")
	signedHeaders := "content-type;host;x-tc-action"
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		"content-type:" + tencentContentType + "\n" +
			"host:" + host + "\n" +
			"x-tc-action:" + strings.ToLower(tencentAction) + "\n",
		signedHeaders,
		sha256Hex(payload),
	}, "\n")
	scope := date + "/" + tencentService + "/tc3_request"
	stringToSign := strings.Join([]string{
		tencentAlgorithm,
		strconv.FormatInt(now.Unix(), 10),
		scope,
		sha256Hex(canonicalRequest),
	}, "\n")
	secretDate := hmacSHA256([]byte("TC3"+secretKey), date)
	secretService := hmacSHA256(secretDate, tencentService)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", tencentAlgorithm, secretID, scope, signedHeaders, signature)
}
