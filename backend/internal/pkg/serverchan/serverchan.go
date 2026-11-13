// Package serverchan sends push notifications through Server酱 (ServerChan).
//
// Two key formats are supported:
//   - Turbo  (SendKey "SCT..."):       POST https://sctapi.ftqq.com/<key>.send
//   - Server酱³ (SendKey "sctp<uid>t..."): POST https://<uid>.push.ft07.com/send/<key>.send
package serverchan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	turboBase = "https://sctapi.ftqq.com"
	// TitleMaxRunes keeps titles within what the WeChat card shows.
	TitleMaxRunes = 32
)

var (
	sc3KeyRe   = regexp.MustCompile(`^sctp(\d+)t[A-Za-z0-9_-]+$`)
	turboKeyRe = regexp.MustCompile(`^SCT[A-Za-z0-9_-]+$`)

	// ErrInvalidKey is returned for keys that match neither format.
	ErrInvalidKey = errors.New("serverchan: invalid SendKey")
)

// ValidKey reports whether key looks like a Turbo or Server酱³ SendKey.
func ValidKey(key string) bool {
	key = strings.TrimSpace(key)
	return len(key) <= 128 && (turboKeyRe.MatchString(key) || sc3KeyRe.MatchString(key))
}

// MaskKey hides the middle of a key for display and logs.
func MaskKey(key string) string {
	if len(key) <= 10 {
		return strings.Repeat("*", len(key))
	}
	return key[:6] + "****" + key[len(key)-4:]
}

// Client sends messages. The zero value is not usable; use New.
type Client struct {
	HTTP *http.Client
	// URLFor builds the endpoint for a key; replaceable in tests.
	URLFor func(key string) (string, error)
}

// New returns a client with a 15s timeout.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, URLFor: EndpointURL}
}

// EndpointURL returns the send endpoint for key.
func EndpointURL(key string) (string, error) {
	key = strings.TrimSpace(key)
	if m := sc3KeyRe.FindStringSubmatch(key); m != nil {
		return fmt.Sprintf("https://%s.push.ft07.com/send/%s.send", m[1], key), nil
	}
	if turboKeyRe.MatchString(key) {
		return turboBase + "/" + key + ".send", nil
	}
	return "", ErrInvalidKey
}

// Send pushes one message. title must not contain line breaks; desp is Markdown.
func (c *Client) Send(ctx context.Context, key, title, desp string) error {
	endpoint, err := c.URLFor(key)
	if err != nil {
		return err
	}
	title = strings.Join(strings.Fields(title), " ")
	if r := []rune(title); len(r) > TitleMaxRunes {
		title = string(r[:TitleMaxRunes])
	}
	body, _ := json.Marshal(map[string]string{"title": title, "desp": desp})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("serverchan: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// The URL contains the key; never surface it.
		return errors.New("serverchan: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var out struct {
		Code    *int   `json:"code"`
		Message string `json:"message"`
		Info    string `json:"info"`
	}
	_ = json.Unmarshal(raw, &out)
	msg := strings.TrimSpace(out.Message)
	if msg == "" {
		msg = strings.TrimSpace(out.Info)
	}
	if resp.StatusCode/100 != 2 {
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("serverchan: http %d: %s", resp.StatusCode, msg)
	}
	if out.Code == nil {
		return errors.New("serverchan: unexpected response")
	}
	if *out.Code != 0 {
		return fmt.Errorf("serverchan: code %d: %s", *out.Code, msg)
	}
	return nil
}
