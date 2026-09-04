package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDramaVideoErrorSetsSeedanceUpstreamContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := service.NewSeedanceVideoUpstreamError(http.StatusForbidden, []byte(`{"code":"insufficient_user_quota","message":"预扣费额度失败, 用户剩余额度: ＄0.300000, 需要预扣费额度: ＄2.700000","data":null}`))
	dramaVideoError(c, err)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), `"code":"insufficient_user_quota"`)
	require.Contains(t, w.Body.String(), `"message":"预扣费额度失败, 用户剩余额度: ＄0.300000, 需要预扣费额度: ＄2.700000"`)

	status, ok := c.Get(service.OpsUpstreamStatusCodeKey)
	require.True(t, ok)
	require.Equal(t, 403, status)

	message, ok := c.Get(service.OpsUpstreamErrorMessageKey)
	require.True(t, ok)
	require.Equal(t, "预扣费额度失败, 用户剩余额度: ＄0.300000, 需要预扣费额度: ＄2.700000", message)

	detail, ok := c.Get(service.OpsUpstreamErrorDetailKey)
	require.True(t, ok)
	detailStr, ok := detail.(string)
	require.True(t, ok)
	require.Contains(t, detailStr, `"insufficient_user_quota"`)
}
