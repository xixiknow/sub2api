package repository

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDramaVideoClientSeedanceVideoTaskPaths(t *testing.T) {
	var createSeen, getSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			createSeen = true
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Errorf("unexpected authorization header: %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"task_123","task_id":"task_123","object":"video","status":"queued","progress":0,"metadata":{"url":"https://example.com/content.mp4"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/task_123":
			getSeen = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"task_123","task_id":"task_123","object":"video","status":"completed","progress":100,"metadata":{"url":"https://example.com/content.mp4"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &dramaVideoClient{httpClient: srv.Client()}
	account := &service.Account{Credentials: map[string]any{
		"api_key":  "test-key",
		"base_url": srv.URL,
	}}

	task, err := client.CreateSeedanceVideoTask(t.Context(), account, []byte(`{"model":"seedance-2.0-0826","prompt":"test"}`))
	require.NoError(t, err)
	require.True(t, createSeen)
	require.Equal(t, "task_123", task.PublicID())
	require.Equal(t, "https://example.com/content.mp4", task.ContentURL())

	fetched, err := client.GetSeedanceVideoTask(t.Context(), account, "task_123")
	require.NoError(t, err)
	require.True(t, getSeen)
	require.Equal(t, "task_123", fetched.PublicID())
	require.Equal(t, "completed", strings.ToLower(fetched.Status))
}

func TestDramaVideoClientSeedanceVideoTaskUpstreamErrorPropagation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"insufficient_user_quota","message":"预扣费额度失败, 用户剩余额度: ＄0.300000, 需要预扣费额度: ＄2.700000","data":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &dramaVideoClient{httpClient: srv.Client()}
	account := &service.Account{Credentials: map[string]any{
		"api_key":  "test-key",
		"base_url": srv.URL,
	}}

	_, err := client.CreateSeedanceVideoTask(t.Context(), account, []byte(`{"model":"seedance-2.0-fast-0826","prompt":"test","seconds":5,"resolution":"720p"}`))
	require.Error(t, err)

	var upstreamErr *service.SeedanceVideoUpstreamError
	require.ErrorAs(t, err, &upstreamErr)
	require.Equal(t, http.StatusForbidden, upstreamErr.UpstreamStatusCode())
	require.Equal(t, "insufficient_user_quota", infraerrors.Reason(err))
	require.Equal(t, "预扣费额度失败, 用户剩余额度: ＄0.300000, 需要预扣费额度: ＄2.700000", infraerrors.Message(err))
	require.Contains(t, upstreamErr.UpstreamErrorDetail(), `"code":"insufficient_user_quota"`)
}
