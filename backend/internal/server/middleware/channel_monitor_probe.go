package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelMonitorProbe 识别渠道监控的签名探测头（fork 扩展）。
//
//   - 无探测头：直接放行，零开销。
//   - 有探测头：一律从请求中剥离（绝不转发给上游）；验签通过才把选号提示放进 ctx，
//     并在响应头写出前回填实际服务的账号 ID（X-Sub2API-Probe-Account）。
//   - 验签失败：静默当作普通请求。
func ChannelMonitorProbe() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Request.Header
		signature := h.Get(service.ProbeHeaderSignature)
		exclude := h.Get(service.ProbeHeaderExclude)
		prefer := h.Get(service.ProbeHeaderPrefer)
		if signature == "" && exclude == "" && prefer == "" {
			c.Next()
			return
		}
		h.Del(service.ProbeHeaderSignature)
		h.Del(service.ProbeHeaderExclude)
		h.Del(service.ProbeHeaderPrefer)
		h.Del(service.ProbeHeaderServedAccount)

		hints, ok := service.VerifyProbeHeaders(signature, exclude, prefer, probeCandidateAPIKeys(c.Request), time.Now())
		if !ok {
			c.Next()
			return
		}
		ctx, rec := service.WithProbeSelectionHints(c.Request.Context(), hints)
		c.Request = c.Request.WithContext(ctx)
		c.Writer = &probeResponseWriter{ResponseWriter: c.Writer, rec: rec}
		c.Next()
	}
}

// probeCandidateAPIKeys 按监控 adapter 的鉴权方式提取候选 key。
func probeCandidateAPIKeys(r *http.Request) []string {
	keys := make([]string, 0, 3)
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
			keys = append(keys, strings.TrimSpace(auth[7:]))
		}
	}
	for _, name := range []string{"x-api-key", "x-goog-api-key"} {
		if v := strings.TrimSpace(r.Header.Get(name)); v != "" {
			keys = append(keys, v)
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("key")); v != "" {
		keys = append(keys, v)
	}
	return keys
}

// probeResponseWriter 在响应头首次写出前注入实际服务账号 ID。
type probeResponseWriter struct {
	gin.ResponseWriter
	rec *service.ProbeSelectionRecord
}

func (w *probeResponseWriter) injectServedAccount() {
	if w.ResponseWriter.Written() {
		return
	}
	if id := w.rec.ServedAccountID(); id > 0 {
		w.ResponseWriter.Header().Set(service.ProbeHeaderServedAccount, strconv.FormatInt(id, 10))
	}
}

func (w *probeResponseWriter) WriteHeader(code int) {
	w.injectServedAccount()
	w.ResponseWriter.WriteHeader(code)
}

func (w *probeResponseWriter) WriteHeaderNow() {
	w.injectServedAccount()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *probeResponseWriter) Write(data []byte) (int, error) {
	w.injectServedAccount()
	return w.ResponseWriter.Write(data)
}

func (w *probeResponseWriter) WriteString(s string) (int, error) {
	w.injectServedAccount()
	return w.ResponseWriter.WriteString(s)
}
