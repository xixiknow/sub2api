package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GrowthHandler handles admin growth badge and automatic benefit management.
type GrowthHandler struct {
	growthService *service.GrowthService
}

func NewGrowthHandler(growthService *service.GrowthService) *GrowthHandler {
	return &GrowthHandler{growthService: growthService}
}

// GET /api/v1/admin/growth/badges
func (h *GrowthHandler) ListBadges(c *gin.Context) {
	badges, err := h.growthService.ListBadgeDefinitions(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, badges)
}

// GET /api/v1/admin/growth/benefit-rules
func (h *GrowthHandler) ListBenefitRules(c *gin.Context) {
	rules, err := h.growthService.ListBenefitRules(
		c.Request.Context(),
		c.Query("badge_id"),
		c.Query("benefit_type"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

type UpsertBadgeBenefitRuleRequest struct {
	BadgeID                    string   `json:"badge_id" binding:"required"`
	Name                       string   `json:"name"`
	BenefitType                string   `json:"benefit_type" binding:"required"`
	GroupID                    *int64   `json:"group_id"`
	RateMultiplier             *float64 `json:"rate_multiplier"`
	AffiliateRebateRatePercent *float64 `json:"affiliate_rebate_rate_percent"`
	Enabled                    *bool    `json:"enabled"`
}

// POST /api/v1/admin/growth/benefit-rules
func (h *GrowthHandler) CreateBenefitRule(c *gin.Context) {
	var req UpsertBadgeBenefitRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rule, err := h.growthService.CreateBenefitRule(c.Request.Context(), badgeBenefitRuleInputFromRequest(req, true))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rule)
}

// PUT /api/v1/admin/growth/benefit-rules/:id
func (h *GrowthHandler) UpdateBenefitRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid rule ID")
		return
	}
	var req UpsertBadgeBenefitRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rule, err := h.growthService.UpdateBenefitRule(c.Request.Context(), id, badgeBenefitRuleInputFromRequest(req, true))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rule)
}

// DELETE /api/v1/admin/growth/benefit-rules/:id
func (h *GrowthHandler) DeleteBenefitRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid rule ID")
		return
	}
	if err := h.growthService.DeleteBenefitRule(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

// GET /api/v1/admin/growth/users
func (h *GrowthHandler) ListUsers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.growthService.ListGrowthUsers(c.Request.Context(), service.GrowthUserFilter{
		Search:   c.Query("search"),
		BadgeID:  c.Query("badge_id"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// POST /api/v1/admin/growth/badges/recompute
func (h *GrowthHandler) RecomputeBadges(c *gin.Context) {
	result, err := h.growthService.RecomputeAllBadges(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func badgeBenefitRuleInputFromRequest(req UpsertBadgeBenefitRuleRequest, defaultEnabled bool) service.BadgeBenefitRuleInput {
	enabled := defaultEnabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return service.BadgeBenefitRuleInput{
		BadgeID:                    req.BadgeID,
		Name:                       req.Name,
		BenefitType:                req.BenefitType,
		GroupID:                    req.GroupID,
		RateMultiplier:             req.RateMultiplier,
		AffiliateRebateRatePercent: req.AffiliateRebateRatePercent,
		Enabled:                    enabled,
	}
}
