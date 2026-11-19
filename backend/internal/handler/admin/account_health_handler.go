package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// BatchAccountHealthRequest 批量查询账号实时健康分（fork 扩展）。
type BatchAccountHealthRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

// GetBatchHealth 返回当前实例内存中的账号健康快照（错误率 / 首字延迟 / 惩罚）。
// 注意：健康数据是进程内的，多实例部署时只反映处理本次请求的实例。
func (h *AccountHandler) GetBatchHealth(c *gin.Context) {
	var req BatchAccountHealthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ids := normalizeInt64IDList(req.AccountIDs)
	tracker := service.DefaultAccountHealthTracker()
	out := make(map[string]service.AccountHealthSnapshot, len(ids))
	for _, id := range ids {
		out[strconv.FormatInt(id, 10)] = tracker.Snapshot(id)
	}
	response.Success(c, gin.H{"health": out})
}
