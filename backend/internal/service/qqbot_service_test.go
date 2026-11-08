package service

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot"
)

const qqbotTestSecret = "DG5g3B4j9X2KOErG"

func qqbotTestSign(timestamp string, body []byte) string {
	seed := qqbotTestSecret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	priv := ed25519.NewKeyFromSeed([]byte(seed[:ed25519.SeedSize]))
	return hex.EncodeToString(ed25519.Sign(priv, append([]byte(timestamp), body...)))
}

type qqbotSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func (s *qqbotSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, k := range keys {
		if v, ok := s.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func newQQBotTestService(enabled bool) (*QQBotService, chan botReply) {
	repo := &qqbotSettingRepoStub{values: map[string]string{
		SettingKeyQQBotEnabled:   map[bool]string{true: "true", false: "false"}[enabled],
		SettingKeyQQBotAppID:     "11111111",
		SettingKeyQQBotAppSecret: qqbotTestSecret,
	}}
	s := NewQQBotService(nil, repo, nil, nil, nil, nil, nil, nil, nil)
	sent := make(chan botReply, 4)
	var mu sync.Mutex
	s.send = func(_ context.Context, _ QQBotConfig, _ *qqbot.Message, r botReply) {
		mu.Lock()
		defer mu.Unlock()
		sent <- r
	}
	return s, sent
}

func TestQQBotWebhookValidation(t *testing.T) {
	s, _ := newQQBotTestService(true)
	body := []byte(`{"d":{"plain_token":"Arq0D5A61EgUu4OxUvOp","event_ts":"1725442341"},"op":13}`)
	out, err := s.HandleWebhook(context.Background(), "", "", body)
	if err != nil {
		t.Fatal(err)
	}
	resp := out.(qqbot.ValidationResponse)
	if resp.PlainToken != "Arq0D5A61EgUu4OxUvOp" || !strings.HasPrefix(resp.Signature, "87befc99c42c651b") {
		t.Fatalf("unexpected validation response %+v", resp)
	}
}

func TestQQBotWebhookDisabled(t *testing.T) {
	s, _ := newQQBotTestService(false)
	_, err := s.HandleWebhook(context.Background(), "", "", []byte(`{"op":13}`))
	if infraerrors.Code(err) != 404 {
		t.Fatalf("want 404 when disabled, got %v", err)
	}
}

func TestQQBotWebhookDispatch(t *testing.T) {
	s, sent := newQQBotTestService(true)
	body := []byte(`{"id":"evt-1","op":0,"t":"C2C_MESSAGE_CREATE","d":{"id":"m1","content":"/帮助","author":{"user_openid":"U1"}}}`)
	ts := "1725442341"

	if _, err := s.HandleWebhook(context.Background(), qqbotTestSign(ts, body), ts+"0", body); infraerrors.Code(err) != 401 {
		t.Fatalf("tampered timestamp: want 401, got %v", err)
	}

	out, err := s.HandleWebhook(context.Background(), qqbotTestSign(ts, body), ts, body)
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := json.Marshal(out); string(raw) != `{"d":0,"op":12}` {
		t.Fatalf("unexpected ack %s", raw)
	}
	select {
	case r := <-sent:
		if !strings.Contains(r.Text, "/签到") || !strings.Contains(r.Text, "/绑定") {
			t.Fatalf("help reply missing commands: %q", r.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no reply sent")
	}

	// Re-delivery of the same event id is acked but not processed again.
	if _, err := s.HandleWebhook(context.Background(), qqbotTestSign(ts, body), ts, body); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-sent:
		t.Fatalf("duplicate event processed: %q", r.Text)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestParseQQBotCommand(t *testing.T) {
	cases := []struct{ in, cmd, arg string }{
		{"/签到", "签到", ""},
		{"  ／绑定   a@b.com ", "绑定", "a@b.com"},
		{"状态", "状态", ""},
		{"/HELP", "help", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		cmd, arg := parseQQBotCommand(c.in)
		if cmd != c.cmd || arg != c.arg {
			t.Errorf("parse(%q) = (%q,%q), want (%q,%q)", c.in, cmd, arg, c.cmd, c.arg)
		}
	}
}

func TestQQBotHelpers(t *testing.T) {
	day := time.Date(2026, 11, 5, 0, 0, 0, 0, checkinZone)
	h1 := qqbotCheckinHash(1, "U1", day)
	if len(h1) != 64 || h1 != qqbotCheckinHash(1, "U1", day) {
		t.Fatal("hash must be stable 32-byte hex")
	}
	if h1 == qqbotCheckinHash(2, "U1", day) || h1 == qqbotCheckinHash(1, "U1", day.AddDate(0, 0, 1)) {
		t.Fatal("hash must differ per user and per day")
	}
	if formatCents(10) != "¥0.10" || formatCents(100) != "¥1.00" {
		t.Fatalf("formatCents: %s %s", formatCents(10), formatCents(100))
	}
	c := newTTLCounter(time.Minute)
	for i := 1; i <= 6; i++ {
		if n := c.incr("k"); n != i {
			t.Fatalf("counter=%d want %d", n, i)
		}
	}
	if maskMiddle("ABCDEFGHIJKL") != "ABCD****IJKL" {
		t.Fatal(maskMiddle("ABCDEFGHIJKL"))
	}
	if qqbotMonitorLevel(MonitorStatusFailed) != "down" || qqbotMonitorLevel("") != "unknown" {
		t.Fatal("monitor level mapping")
	}
}
