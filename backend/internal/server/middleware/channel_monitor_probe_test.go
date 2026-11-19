package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeSelectHandler 模拟网关 handler：经由 service 选号（这里直接用提示里的 Prefer 记录账号）。
func newProbeTestRouter(t *testing.T, onRequest func(c *gin.Context)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ChannelMonitorProbe())
	r.POST("/v1/messages", func(c *gin.Context) {
		onRequest(c)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestChannelMonitorProbe_ValidSignatureInjectsHintsAndServedAccount(t *testing.T) {
	service.InitChannelMonitorProbeSigner("mw-secret")
	t.Cleanup(func() { service.InitChannelMonitorProbeSigner("") })

	var leaked []string
	var gotRecord bool
	r := newProbeTestRouter(t, func(c *gin.Context) {
		for _, h := range []string{service.ProbeHeaderSignature, service.ProbeHeaderExclude, service.ProbeHeaderPrefer} {
			if c.Request.Header.Get(h) != "" {
				leaked = append(leaked, h)
			}
		}
		// 经过真实选号装饰器：用一个总是成功的 selector 记录选中账号 77。
		rec := service.ProbeServedAccountForTest(c.Request.Context(), 77)
		gotRecord = rec
	})

	headers := service.BuildProbeHeaders("sk-mw", service.ProbeHints{Exclude: []int64{5}, Prefer: 77}, time.Now())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "sk-mw")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, leaked, "probe headers must be stripped before reaching handlers/upstream")
	require.True(t, gotRecord)
	require.Equal(t, "77", w.Header().Get(service.ProbeHeaderServedAccount))
}

func TestChannelMonitorProbe_InvalidSignatureIsIgnoredAndStripped(t *testing.T) {
	service.InitChannelMonitorProbeSigner("mw-secret")
	t.Cleanup(func() { service.InitChannelMonitorProbeSigner("") })

	var leaked bool
	var gotRecord bool
	r := newProbeTestRouter(t, func(c *gin.Context) {
		leaked = c.Request.Header.Get(service.ProbeHeaderPrefer) != ""
		gotRecord = service.ProbeServedAccountForTest(c.Request.Context(), 77)
	})

	// 用别的 key 签名：本请求的 key 无法通过校验。
	headers := service.BuildProbeHeaders("sk-other", service.ProbeHints{Prefer: 77}, time.Now())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer sk-mw")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.False(t, leaked)
	require.False(t, gotRecord)
	require.Empty(t, w.Header().Get(service.ProbeHeaderServedAccount))
}

func TestChannelMonitorProbe_NoHeadersPassThrough(t *testing.T) {
	called := false
	r := newProbeTestRouter(t, func(c *gin.Context) {
		called = true
		require.False(t, service.ProbeServedAccountForTest(context.Background(), 1))
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.True(t, called)
	require.Empty(t, w.Header().Get(service.ProbeHeaderServedAccount))
}
