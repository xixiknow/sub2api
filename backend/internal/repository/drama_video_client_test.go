package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDramaVideoClientCreateUsesPathAndBearer(t *testing.T) {
	var gotPath, gotAuth, gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotModel = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"up_1","status":"queued"}`))
	}))
	defer server.Close()

	client := &dramaVideoClient{httpClient: server.Client()}
	account := &service.Account{
		Platform: service.PlatformDrama,
		Credentials: map[string]any{
			"api_key":  "drama-token",
			"base_url": server.URL,
		},
	}
	task, err := client.CreateVideo(context.Background(), account, service.DramaVideoCreatePathGens, []byte(`{"model":"seedance2.0-F-720p"}`))
	require.NoError(t, err)
	require.Equal(t, "up_1", task.PublicID())
	require.Equal(t, "/v1/video/generations", gotPath)
	require.Equal(t, "Bearer drama-token", gotAuth)
	require.Contains(t, gotModel, "seedance2.0-F-720p")
}

func TestDramaVideoClientGetVideoUsesCreatePath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"up_2","status":"completed"}`))
	}))
	defer server.Close()

	client := &dramaVideoClient{httpClient: server.Client()}
	account := &service.Account{Credentials: map[string]any{"api_key": "k", "base_url": server.URL}}
	task, err := client.GetVideo(context.Background(), account, service.DramaVideoCreatePathVideos, "abc")
	require.NoError(t, err)
	require.Equal(t, "up_2", task.PublicID())
	require.True(t, strings.HasPrefix(gotPath, "/v1/videos/"))
}
