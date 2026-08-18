package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAsyncVideoTaskRead(t *testing.T) {
	require.True(t, isAsyncVideoTaskRead(http.MethodGet, "/v1/videos/vidtask_123"))
	require.True(t, isAsyncVideoTaskRead(http.MethodGet, "/videos/vidtask_123/content"))
	require.True(t, isAsyncVideoTaskRead(http.MethodGet, "/v1/videos/generations/vidtask_123"))
	require.True(t, isAsyncVideoTaskRead(http.MethodGet, "/videos/generations/vidtask_123/content"))
	require.False(t, isAsyncVideoTaskRead(http.MethodPost, "/v1/videos/vidtask_123"))
	require.False(t, isAsyncVideoTaskRead(http.MethodGet, "/v1/videos"))
	require.False(t, isAsyncVideoTaskRead(http.MethodGet, "/v1/videos/generations"))
}
