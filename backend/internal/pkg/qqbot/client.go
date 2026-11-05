package qqbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	defaultTokenURL = "https://bots.qq.com/app/getAppAccessToken"
	defaultAPIBase  = "https://api.sgroup.qq.com"
)

// Message types.
const (
	MsgTypeText  = 0
	MsgTypeMedia = 7
)

// Client calls the QQ bot OpenAPI with a cached app access token.
type Client struct {
	appID, secret     string
	tokenURL, apiBase string
	http              *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewClient(appID, secret string) *Client {
	return &Client{
		appID: appID, secret: secret,
		tokenURL: defaultTokenURL, apiBase: defaultAPIBase,
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// APIError is a non-2xx response from the QQ API.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("qqbot api: http %d: %s", e.Status, e.Body) }

// accessToken returns a cached token, refreshing 60s before expiry.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"appId": c.appID, "clientSecret": c.secret})
	var out struct {
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"` // documented as string, tolerate number
	}
	if err := c.do(ctx, http.MethodPost, c.tokenURL, "", body, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("qqbot: empty access token")
	}
	secs, _ := strconv.Atoi(string(bytes.Trim(out.ExpiresIn, `"`)))
	if secs <= 0 {
		secs = 7200
	}
	c.token = out.AccessToken
	c.expires = time.Now().Add(time.Duration(secs)*time.Second - time.Minute)
	return c.token, nil
}

func (c *Client) do(ctx context.Context, method, url, token string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "QQBot "+token)
		req.Header.Set("X-Union-Appid", c.appID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (c *Client) api(ctx context.Context, path string, body any, out any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	err = c.do(ctx, http.MethodPost, c.apiBase+path, token, data, out)
	if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusUnauthorized {
		c.mu.Lock()
		c.token = "" // force refresh next time
		c.mu.Unlock()
	}
	return err
}

func scenePath(scene Scene, target string) string {
	if scene == SceneGroup {
		return "/v2/groups/" + target
	}
	return "/v2/users/" + target
}

// Reply is a passive reply to an incoming message.
type Reply struct {
	Text     string
	FileInfo string // media file_info from UploadImage; switches to msg_type=7
}

// SendReply sends a passive reply (needs the incoming msg_id; seq distinguishes
// multiple replies to the same message).
func (c *Client) SendReply(ctx context.Context, m *Message, seq int, r Reply) error {
	body := map[string]any{"msg_id": m.MsgID, "msg_seq": seq, "msg_type": MsgTypeText, "content": r.Text}
	if r.FileInfo != "" {
		body["msg_type"] = MsgTypeMedia
		body["media"] = map[string]string{"file_info": r.FileInfo}
		if r.Text == "" {
			body["content"] = " "
		}
	}
	return c.api(ctx, scenePath(m.Scene, m.ReplyTarget())+"/messages", body, nil)
}

// UploadImage uploads PNG bytes as rich media and returns file_info for SendReply.
func (c *Client) UploadImage(ctx context.Context, m *Message, png []byte) (string, error) {
	var out struct {
		FileInfo string `json:"file_info"`
	}
	body := map[string]any{"file_type": 1, "file_data": base64.StdEncoding.EncodeToString(png), "srv_send_msg": false}
	if err := c.api(ctx, scenePath(m.Scene, m.ReplyTarget())+"/files", body, &out); err != nil {
		return "", err
	}
	if out.FileInfo == "" {
		return "", fmt.Errorf("qqbot: upload returned no file_info")
	}
	return out.FileInfo, nil
}
