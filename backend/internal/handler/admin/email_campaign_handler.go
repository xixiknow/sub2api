package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// EmailCampaignHandler handles admin marketing email campaigns.
type EmailCampaignHandler struct {
	campaignService *service.EmailCampaignService
}

func NewEmailCampaignHandler(campaignService *service.EmailCampaignService) *EmailCampaignHandler {
	return &EmailCampaignHandler{campaignService: campaignService}
}

func parseEmailCampaignID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid campaign ID")
		return 0, false
	}
	return id, true
}

func bindEmailCampaignInput(c *gin.Context) (service.EmailCampaignInput, bool) {
	var req service.EmailCampaignInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return req, false
	}
	return req, true
}

// List GET /api/v1/admin/email-campaigns
func (h *EmailCampaignHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 200 {
		search = search[:200]
	}
	items, total, err := h.campaignService.List(c.Request.Context(), page, pageSize, c.Query("status"), search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// Get GET /api/v1/admin/email-campaigns/:id
func (h *EmailCampaignHandler) Get(c *gin.Context) {
	id, ok := parseEmailCampaignID(c)
	if !ok {
		return
	}
	item, err := h.campaignService.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// Create POST /api/v1/admin/email-campaigns
func (h *EmailCampaignHandler) Create(c *gin.Context) {
	req, ok := bindEmailCampaignInput(c)
	if !ok {
		return
	}
	var createdBy int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		createdBy = subject.UserID
	}
	item, err := h.campaignService.Create(c.Request.Context(), req, createdBy)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// Update PUT /api/v1/admin/email-campaigns/:id
func (h *EmailCampaignHandler) Update(c *gin.Context) {
	id, ok := parseEmailCampaignID(c)
	if !ok {
		return
	}
	req, ok := bindEmailCampaignInput(c)
	if !ok {
		return
	}
	item, err := h.campaignService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// Delete DELETE /api/v1/admin/email-campaigns/:id
func (h *EmailCampaignHandler) Delete(c *gin.Context) {
	id, ok := parseEmailCampaignID(c)
	if !ok {
		return
	}
	if err := h.campaignService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Campaign deleted"})
}

// PreviewAudience POST /api/v1/admin/email-campaigns/audience/preview
func (h *EmailCampaignHandler) PreviewAudience(c *gin.Context) {
	var req service.EmailCampaignAudience
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.campaignService.PreviewAudience(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

// Preview POST /api/v1/admin/email-campaigns/preview
func (h *EmailCampaignHandler) Preview(c *gin.Context) {
	req, ok := bindEmailCampaignInput(c)
	if !ok {
		return
	}
	out, err := h.campaignService.Preview(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

// SendTest POST /api/v1/admin/email-campaigns/test
func (h *EmailCampaignHandler) SendTest(c *gin.Context) {
	req, ok := bindEmailCampaignInput(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	email, err := h.campaignService.SendTest(c.Request.Context(), req, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"email": email})
}

// Launch POST /api/v1/admin/email-campaigns/:id/send
func (h *EmailCampaignHandler) Launch(c *gin.Context) {
	id, ok := parseEmailCampaignID(c)
	if !ok {
		return
	}
	item, err := h.campaignService.Launch(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// Cancel POST /api/v1/admin/email-campaigns/:id/cancel
func (h *EmailCampaignHandler) Cancel(c *gin.Context) {
	id, ok := parseEmailCampaignID(c)
	if !ok {
		return
	}
	item, err := h.campaignService.Cancel(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// ListRecipients GET /api/v1/admin/email-campaigns/:id/recipients
func (h *EmailCampaignHandler) ListRecipients(c *gin.Context) {
	id, ok := parseEmailCampaignID(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.campaignService.ListRecipients(c.Request.Context(), id, page, pageSize, c.Query("status"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}
