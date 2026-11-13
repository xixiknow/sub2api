package serverchan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpointURL(t *testing.T) {
	u, err := EndpointURL("SCT12345abcDEF")
	if err != nil || u != "https://sctapi.ftqq.com/SCT12345abcDEF.send" {
		t.Fatalf("turbo: %q %v", u, err)
	}
	u, err = EndpointURL("sctp987t0123abc")
	if err != nil || u != "https://987.push.ft07.com/send/sctp987t0123abc.send" {
		t.Fatalf("sc3: %q %v", u, err)
	}
	for _, bad := range []string{"", "abc", "SCT", "sctpxt1", "SCT12/../x"} {
		if _, err := EndpointURL(bad); err == nil || ValidKey(bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
	if MaskKey("SCT12345abcdef") != "SCT123****cdef" {
		t.Fatalf("mask = %s", MaskKey("SCT12345abcdef"))
	}
}

func newTestClient(t *testing.T, status int, resp string, got *map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			_ = json.NewDecoder(r.Body).Decode(got)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.URLFor = func(string) (string, error) { return srv.URL, nil }
	return c
}

func TestSend(t *testing.T) {
	var got map[string]string
	c := newTestClient(t, 200, `{"code":0,"message":"","data":{}}`, &got)
	title := "第一行\n第二行" + strings.Repeat("长", 40)
	if err := c.Send(context.Background(), "SCT1abc", title, "**md**"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got["title"], "\n") || len([]rune(got["title"])) != TitleMaxRunes || got["desp"] != "**md**" {
		t.Fatalf("payload = %+v", got)
	}

	c = newTestClient(t, 200, `{"code":40001,"message":"bad pushkey"}`, nil)
	if err := c.Send(context.Background(), "SCT1abc", "t", ""); err == nil || !strings.Contains(err.Error(), "bad pushkey") {
		t.Fatalf("err = %v", err)
	}
	c = newTestClient(t, 500, `oops`, nil)
	if err := c.Send(context.Background(), "SCT1abc", "t", ""); err == nil || !strings.Contains(err.Error(), "http 500") {
		t.Fatalf("err = %v", err)
	}
	c = newTestClient(t, 200, `not json`, nil)
	if err := c.Send(context.Background(), "SCT1abc", "t", ""); err == nil {
		t.Fatal("accepted non-json response")
	}
}
