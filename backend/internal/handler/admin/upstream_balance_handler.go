package admin

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// UpstreamBalanceHandler 处理上游余额维护：总览、历史、提醒配置与记录。
type UpstreamBalanceHandler struct {
	opsService *service.UpstreamBalanceOpsService
}

func NewUpstreamBalanceHandler(opsService *service.UpstreamBalanceOpsService) *UpstreamBalanceHandler {
	return &UpstreamBalanceHandler{opsService: opsService}
}

// Overview GET /api/v1/admin/upstream-balance/overview
func (h *UpstreamBalanceHandler) Overview(c *gin.Context) {
	groupID, _ := strconv.ParseInt(c.Query("group_id"), 10, 64)
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}
	overview, err := h.opsService.Overview(c.Request.Context(), service.UpstreamBalanceOverviewFilter{
		Platform: c.Query("platform"),
		Status:   c.Query("status"),
		GroupID:  groupID,
		Search:   search,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

// History GET /api/v1/admin/upstream-balance/accounts/:id/history
func (h *UpstreamBalanceHandler) History(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	history, err := h.opsService.History(c.Request.Context(), accountID, days, c.Query("timezone"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, history)
}

// UpdateAccountAlert PUT /api/v1/admin/upstream-balance/accounts/:id/alert
func (h *UpstreamBalanceHandler) UpdateAccountAlert(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req service.UpstreamBalanceAlertOverride
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.opsService.UpdateAccountOverride(c.Request.Context(), accountID, req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, req)
}

// UpdateWalletLabel PUT /api/v1/admin/upstream-balance/wallet-label
// 给一组账号设置相同的钱包标签（手动合并）；label 为空表示取消。
func (h *UpstreamBalanceHandler) UpdateWalletLabel(c *gin.Context) {
	var req struct {
		AccountIDs []int64 `json:"account_ids"`
		Label      string  `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.opsService.SetWalletLabel(c.Request.Context(), req.AccountIDs, req.Label); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": len(req.AccountIDs)})
}

// GetSettings GET /api/v1/admin/upstream-balance/settings
func (h *UpstreamBalanceHandler) GetSettings(c *gin.Context) {
	cfg, err := h.opsService.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateSettings PUT /api/v1/admin/upstream-balance/settings
func (h *UpstreamBalanceHandler) UpdateSettings(c *gin.Context) {
	var req service.UpstreamBalanceAlertConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.opsService.UpdateConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// TestSMS POST /api/v1/admin/upstream-balance/settings/test-sms
func (h *UpstreamBalanceHandler) TestSMS(c *gin.Context) {
	if err := h.opsService.SendTestSMS(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"sent": true})
}

// ListAlerts GET /api/v1/admin/upstream-balance/alerts
func (h *UpstreamBalanceHandler) ListAlerts(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	alerts, err := h.opsService.ListAlerts(c.Request.Context(), days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, alerts)
}

// RefreshAll POST /api/v1/admin/upstream-balance/refresh-all
// 刷新全部中转账号余额并执行一次提醒评估；与请求生命周期解耦，避免前端断开导致中途取消。
func (h *UpstreamBalanceHandler) RefreshAll(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Minute)
	defer cancel()
	if err := h.opsService.RefreshAll(ctx); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"refreshed": true})
}
