package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// affiliateWithdrawRequestBodyMaxBytes 限制提现请求体（含 base64 收款码）。
const affiliateWithdrawRequestBodyMaxBytes = 1 << 20

// SubmitAffiliateWithdrawRequest 是用户提交支付宝提现的请求体。
type SubmitAffiliateWithdrawRequest struct {
	Amount        float64 `json:"amount"`
	AlipayAccount string  `json:"alipay_account"`
	QRImage       string  `json:"qr_image"`
}

// SubmitAffiliateWithdraw submits an Alipay withdrawal request; the amount is
// held from the available affiliate quota until an admin resolves it.
// POST /api/v1/user/aff/withdrawals
func (h *UserHandler) SubmitAffiliateWithdraw(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, affiliateWithdrawRequestBodyMaxBytes)
	var req SubmitAffiliateWithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.affiliateService.SubmitWithdrawRequest(c.Request.Context(), subject.UserID, req.Amount, req.AlipayAccount, req.QRImage)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListAffiliateWithdraws lists the current user's withdrawal requests.
// GET /api/v1/user/aff/withdrawals
func (h *UserHandler) ListAffiliateWithdraws(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.affiliateService.ListUserWithdrawRequests(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// CancelAffiliateWithdraw cancels the current user's pending withdrawal request.
// POST /api/v1/user/aff/withdrawals/:id/cancel
func (h *UserHandler) CancelAffiliateWithdraw(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, service.ErrAffiliateWithdrawNotFound)
		return
	}

	result, err := h.affiliateService.CancelWithdrawRequest(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
