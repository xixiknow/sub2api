package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ResolveAffiliateWithdrawRequest 是标记已打款 / 驳回的请求体。
type ResolveAffiliateWithdrawRequest struct {
	Note string `json:"note"`
}

// ListWithdrawRequests lists Alipay withdrawal requests.
// GET /api/v1/admin/affiliates/withdrawals?status=pending&search=&start_at=&end_at=
func (h *AffiliateHandler) ListWithdrawRequests(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	recordFilter := parseAffiliateRecordFilter(c, page, pageSize)
	filter := service.AffiliateWithdrawRequestFilter{
		Status:   strings.TrimSpace(c.Query("status")),
		Search:   recordFilter.Search,
		StartAt:  recordFilter.StartAt,
		EndAt:    recordFilter.EndAt,
		Page:     recordFilter.Page,
		PageSize: recordFilter.PageSize,
	}
	items, total, err := h.affiliateService.AdminListWithdrawRequests(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, recordFilter.Page, recordFilter.PageSize)
}

// GetWithdrawRequest returns one withdrawal request including its QR image.
// GET /api/v1/admin/affiliates/withdrawals/:id
func (h *AffiliateHandler) GetWithdrawRequest(c *gin.Context) {
	id, ok := parseAffiliateWithdrawID(c)
	if !ok {
		return
	}
	item, err := h.affiliateService.AdminGetWithdrawRequest(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// MarkWithdrawPaid confirms the admin has paid the user offline.
// POST /api/v1/admin/affiliates/withdrawals/:id/paid
func (h *AffiliateHandler) MarkWithdrawPaid(c *gin.Context) {
	h.resolveWithdraw(c, true)
}

// RejectWithdrawRequest rejects a withdrawal and returns the held quota.
// POST /api/v1/admin/affiliates/withdrawals/:id/reject
func (h *AffiliateHandler) RejectWithdrawRequest(c *gin.Context) {
	h.resolveWithdraw(c, false)
}

func (h *AffiliateHandler) resolveWithdraw(c *gin.Context, paid bool) {
	id, ok := parseAffiliateWithdrawID(c)
	if !ok {
		return
	}
	var req ResolveAffiliateWithdrawRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}
	var adminID int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		adminID = subject.UserID
	}

	var (
		item *service.AffiliateWithdrawRequest
		err  error
	)
	if paid {
		item, err = h.affiliateService.AdminMarkWithdrawPaid(c.Request.Context(), id, adminID, req.Note)
	} else {
		item, err = h.affiliateService.AdminRejectWithdrawRequest(c.Request.Context(), id, adminID, req.Note)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func parseAffiliateWithdrawID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}
