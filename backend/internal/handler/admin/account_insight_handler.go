package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AccountInsightHandler 处理账号树视图统计与上游余额查询。
type AccountInsightHandler struct {
	treeService    *service.AccountTreeService
	balanceService *service.UpstreamBalanceService
}

func NewAccountInsightHandler(treeService *service.AccountTreeService, balanceService *service.UpstreamBalanceService) *AccountInsightHandler {
	return &AccountInsightHandler{treeService: treeService, balanceService: balanceService}
}

// TreeSummary GET /api/v1/admin/accounts/tree-summary
func (h *AccountInsightHandler) TreeSummary(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}
	items, err := h.treeService.GroupSummary(c.Request.Context(), service.AccountTreeSummaryFilter{
		Platform:    c.Query("platform"),
		AccountType: c.Query("type"),
		Search:      search,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// RefreshAccountBalance POST /api/v1/admin/accounts/:id/upstream-balance/refresh
func (h *AccountInsightHandler) RefreshAccountBalance(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if h.balanceService == nil {
		response.ErrorFrom(c, service.ErrUpstreamBalanceUnavailable)
		return
	}
	result, err := h.balanceService.QueryAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type upstreamBalanceBatchRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required"`
}

// BatchRefreshAccountBalance POST /api/v1/admin/accounts/upstream-balance/batch
func (h *AccountInsightHandler) BatchRefreshAccountBalance(c *gin.Context) {
	var req upstreamBalanceBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.AccountIDs) > service.UpstreamBalanceMaxBatchSize {
		response.BadRequest(c, "Too many accounts")
		return
	}
	if h.balanceService == nil {
		response.ErrorFrom(c, service.ErrUpstreamBalanceUnavailable)
		return
	}
	response.Success(c, h.balanceService.QueryAccounts(c.Request.Context(), req.AccountIDs))
}
