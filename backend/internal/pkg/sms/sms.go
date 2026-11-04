// Package sms 提供阿里云、腾讯云短信发送，直接调用 HTTP API 并手写签名，不引入 SDK。
package sms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ProviderAliyun  = "aliyun"
	ProviderTencent = "tencent"

	defaultTimeout  = 10 * time.Second
	maxResponseSize = 64 * 1024
)

// Param 是一个模板变量。阿里云按名字传 JSON 对象，腾讯云按顺序传数组。
type Param struct {
	Name  string
	Value string
}

// Message 是一次模板短信发送请求。
type Message struct {
	Phones     []string
	TemplateID string
	Params     []Param
}

// Provider 是短信服务商的统一接口。
type Provider interface {
	Name() string
	Send(ctx context.Context, msg Message) error
}

// Config 描述服务商凭据。Endpoint 为空时使用官方地址（测试时可指向本地 mock）。
type Config struct {
	Provider        string
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	SdkAppID        string // 仅腾讯云
	Region          string // 仅腾讯云，默认 ap-guangzhou
	Endpoint        string
	HTTPClient      *http.Client
}

var ErrNotConfigured = errors.New("sms provider is not configured")

// New 按配置构造服务商。
func New(cfg Config) (Provider, error) {
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" || strings.TrimSpace(cfg.SignName) == "" {
		return nil, ErrNotConfigured
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	switch cfg.Provider {
	case ProviderAliyun:
		return &aliyunProvider{cfg: cfg, client: client, now: time.Now, nonce: randomNonce}, nil
	case ProviderTencent:
		if strings.TrimSpace(cfg.SdkAppID) == "" {
			return nil, ErrNotConfigured
		}
		return &tencentProvider{cfg: cfg, client: client, now: time.Now}, nil
	default:
		return nil, fmt.Errorf("unknown sms provider %q", cfg.Provider)
	}
}

func readBody(resp *http.Response) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
}

// normalizePhones 去空白、去重。
func normalizePhones(phones []string) []string {
	out := make([]string, 0, len(phones))
	seen := make(map[string]struct{}, len(phones))
	for _, p := range phones {
		p = strings.ReplaceAll(strings.TrimSpace(p), " ", "")
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
