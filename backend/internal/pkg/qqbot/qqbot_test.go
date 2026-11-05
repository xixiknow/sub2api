package qqbot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Vector from the official "event emit" doc (webhook validation example).
func TestSignValidationDocVector(t *testing.T) {
	s, err := NewSigner("DG5g3B4j9X2KOErG")
	if err != nil {
		t.Fatal(err)
	}
	got := s.SignValidation("1725442341", "Arq0D5A61EgUu4OxUvOp")
	want := "87befc99c42c651b3aac0278e71ada338433ae26fcb24307bdc5ad38c1adc2d01bcfcadc0842edac85e85205028a1132afe09280305f13aa6909ffc2d652c706"
	if got != want {
		t.Fatalf("signature mismatch\n got %s\nwant %s", got, want)
	}
}

func TestVerify(t *testing.T) {
	s, _ := NewSigner("abc")
	body := []byte(`{"op":0}`)
	sig := s.sign("1700000000", body)
	if err := s.Verify(sig, "1700000000", body); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := s.Verify(sig, "1700000001", body); err == nil {
		t.Fatal("tampered timestamp accepted")
	}
	if err := s.Verify(sig, "1700000000", []byte(`{"op":1}`)); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := s.Verify("zz", "1", body); err == nil {
		t.Fatal("garbage signature accepted")
	}
}

func TestParseMessage(t *testing.T) {
	raw := `{"id":"e1","op":0,"t":"GROUP_AT_MESSAGE_CREATE","d":{"id":"m1","content":" <@!123> /签到 ","group_openid":"G1","author":{"member_openid":"U1"}}}`
	var p Payload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	m, ok, err := ParseMessage(&p)
	if err != nil || !ok {
		t.Fatalf("parse: ok=%v err=%v", ok, err)
	}
	if m.Scene != SceneGroup || m.MsgID != "m1" || m.UserOpenID != "U1" || m.ReplyTarget() != "G1" || m.Content != "/签到" {
		t.Fatalf("unexpected message %+v", m)
	}
	p = Payload{T: EventC2CMessage, D: json.RawMessage(`{"id":"m2","content":"帮助","author":{"user_openid":"U2"}}`)}
	m, ok, _ = ParseMessage(&p)
	if !ok || m.Scene != SceneC2C || m.ReplyTarget() != "U2" || m.Content != "帮助" {
		t.Fatalf("unexpected c2c message %+v", m)
	}
	if _, ok, _ := ParseMessage(&Payload{T: "FRIEND_ADD"}); ok {
		t.Fatal("unexpected event accepted")
	}
}

func TestClientSendFlow(t *testing.T) {
	var tokenCalls int
	var paths []string
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/token":
			tokenCalls++
			_, _ = w.Write([]byte(`{"access_token":"TK","expires_in":"7200"}`))
			return
		}
		if r.Header.Get("Authorization") != "QQBot TK" {
			w.WriteHeader(401)
			return
		}
		paths = append(paths, r.URL.Path)
		_ = json.Unmarshal(b, &lastBody)
		if strings.HasSuffix(r.URL.Path, "/files") {
			_, _ = w.Write([]byte(`{"file_info":"FI"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := NewClient("app", "secret")
	c.tokenURL, c.apiBase = srv.URL+"/token", srv.URL
	m := &Message{Scene: SceneGroup, MsgID: "m1", GroupOpenID: "G1", UserOpenID: "U1"}
	ctx := context.Background()
	if err := c.SendReply(ctx, m, 1, Reply{Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	fi, err := c.UploadImage(ctx, m, []byte{1, 2, 3})
	if err != nil || fi != "FI" {
		t.Fatalf("upload: %v %q", err, fi)
	}
	if err := c.SendReply(ctx, m, 2, Reply{FileInfo: fi}); err != nil {
		t.Fatal(err)
	}
	if tokenCalls != 1 {
		t.Fatalf("token fetched %d times, want 1", tokenCalls)
	}
	want := []string{"/v2/groups/G1/messages", "/v2/groups/G1/files", "/v2/groups/G1/messages"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths %v", paths)
	}
	if lastBody["msg_type"].(float64) != MsgTypeMedia || lastBody["msg_id"] != "m1" {
		t.Fatalf("media reply body %v", lastBody)
	}
}
