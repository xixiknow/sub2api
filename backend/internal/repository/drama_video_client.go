package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	dramaVideoMaxResponseBytes = 2 * 1024 * 1024
	dramaVideoMaxDownloadBytes = 1024 * 1024 * 1024
)

type dramaVideoClient struct {
	httpClient *http.Client
}

func NewDramaVideoClient(cfg *config.Config) service.DramaVideoClient {
	_ = cfg
	return &dramaVideoClient{httpClient: &http.Client{
		Timeout:       6 * time.Minute,
		CheckRedirect: dramaVideoRedirectPolicy,
	}}
}

func dramaVideoRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) == 0 || req == nil || req.URL == nil || via[0] == nil || via[0].URL == nil {
		return nil
	}
	if !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
		req.Header.Del("Authorization")
	}
	return nil
}

func (c *dramaVideoClient) CreateVideo(ctx context.Context, account *service.Account, path string, body []byte) (*service.DramaVideoUpstreamTask, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty video request body")
	}
	req, err := c.newRequest(ctx, account, http.MethodPost, normalizeDramaCreatePath(path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doJSON(req)
}

func (c *dramaVideoClient) GetVideo(ctx context.Context, account *service.Account, path, taskID string) (*service.DramaVideoUpstreamTask, error) {
	req, err := c.newRequest(ctx, account, http.MethodGet, dramaStatusPath(path, taskID), nil)
	if err != nil {
		return nil, err
	}
	return c.doJSONWithRetry(ctx, req, 3)
}

func (c *dramaVideoClient) DownloadVideo(ctx context.Context, account *service.Account, taskID string) (*service.DramaVideoDownload, error) {
	path := service.DramaVideoCreatePathVideos + "/" + url.PathEscape(strings.TrimSpace(taskID)) + "/content"
	req, err := c.newRequest(ctx, account, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "video/mp4, video/*, application/octet-stream, */*")
	resp, err := c.doWithRetry(ctx, req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, dramaVideoMaxResponseBytes))
		return nil, fmt.Errorf("Drama download failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	data, err := io.ReadAll(io.LimitReader(resp.Body, dramaVideoMaxDownloadBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > dramaVideoMaxDownloadBytes {
		return nil, fmt.Errorf("Drama download exceeds %d bytes", dramaVideoMaxDownloadBytes)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("Drama download returned empty body")
	}
	if !isDramaVideoContentType(ct) {
		return nil, fmt.Errorf("Drama download returned unexpected content type %q", ct)
	}
	if !hasDramaMP4Signature(data) {
		return nil, fmt.Errorf("Drama download did not look like an MP4 file")
	}
	if ct == "" {
		ct = "video/mp4"
	}
	return &service.DramaVideoDownload{ContentType: ct, Data: data}, nil
}

func (c *dramaVideoClient) newRequest(ctx context.Context, account *service.Account, method, path string, body io.Reader) (*http.Request, error) {
	if account == nil {
		return nil, fmt.Errorf("nil Drama account")
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(account.GetCredential("token"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Drama account api_key is empty")
	}
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if baseURL == "" {
		baseURL = service.DramaVideoDefaultBaseURL
	}
	endpoint, err := joinDramaURL(baseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *dramaVideoClient) doJSON(req *http.Request) (*service.DramaVideoUpstreamTask, error) {
	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeDramaTaskResponse(resp)
}

func (c *dramaVideoClient) doJSONWithRetry(ctx context.Context, req *http.Request, attempts int) (*service.DramaVideoUpstreamTask, error) {
	resp, err := c.doWithRetry(ctx, req, attempts)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeDramaTaskResponse(resp)
}

func (c *dramaVideoClient) doWithRetry(ctx context.Context, req *http.Request, attempts int) (*http.Response, error) {
	if attempts <= 0 {
		attempts = 1
	}
	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		attemptReq := req.Clone(ctx)
		resp, err := client.Do(attemptReq)
		if err == nil && resp != nil && !isDramaRetryableStatus(resp.StatusCode) {
			return resp, nil
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("Drama retryable status %d", resp.StatusCode)
		} else if err != nil {
			lastErr = err
		}
		if i+1 >= attempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(i+1) * 500 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func decodeDramaTaskResponse(resp *http.Response) (*service.DramaVideoUpstreamTask, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, dramaVideoMaxResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Drama upstream error: status=%d body=%s", resp.StatusCode, string(body))
	}
	var task service.DramaVideoUpstreamTask
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("decode Drama task response: %w", err)
	}
	if task.PublicID() == "" {
		return nil, fmt.Errorf("Drama task response missing id")
	}
	return &task, nil
}

func isDramaRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func isDramaVideoContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		return true
	}
	contentType = strings.Split(contentType, ";")[0]
	return strings.HasPrefix(contentType, "video/") ||
		contentType == "application/mp4" ||
		contentType == "application/octet-stream"
}

func hasDramaMP4Signature(data []byte) bool {
	return len(data) >= 12 && string(data[4:8]) == "ftyp"
}

func normalizeDramaCreatePath(path string) string {
	path = strings.TrimSpace(path)
	if path == service.DramaVideoCreatePathGens {
		return service.DramaVideoCreatePathGens
	}
	return service.DramaVideoCreatePathVideos
}

func dramaStatusPath(createPath, taskID string) string {
	return normalizeDramaCreatePath(createPath) + "/" + url.PathEscape(strings.TrimSpace(taskID))
}

func joinDramaURL(baseURL, path string) (string, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid Drama base_url")
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	return base.String(), nil
}
