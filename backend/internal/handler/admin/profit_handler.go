package admin

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ProfitHandler 盈利运营统计。
type ProfitHandler struct {
	profitService *service.ProfitService
}

func NewProfitHandler(profitService *service.ProfitService) *ProfitHandler {
	return &ProfitHandler{profitService: profitService}
}

func parseProfitFilter(c *gin.Context) (service.ProfitFilter, bool) {
	startTime, endTime := parseTimeRange(c)
	filter := service.ProfitFilter{
		StartTime:           startTime,
		EndTime:             endTime,
		Platform:            c.Query("platform"),
		IncludeSubscription: parseBoolQueryWithDefault(c.Query("include_subscription"), false),
	}
	for name, target := range map[string]*int64{"group_id": &filter.GroupID, "account_id": &filter.AccountID} {
		if raw := c.Query(name); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || v < 0 {
				response.ErrorFrom(c, infraerrors.BadRequest("PROFIT_INVALID_FILTER", "invalid "+name))
				return filter, false
			}
			*target = v
		}
	}
	return filter, true
}

// Summary GET /api/v1/admin/profit/summary
func (h *ProfitHandler) Summary(c *gin.Context) {
	filter, ok := parseProfitFilter(c)
	if !ok {
		return
	}
	out, err := h.profitService.Summary(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

// Breakdown GET /api/v1/admin/profit/breakdown?dimension=group|account|platform|model
func (h *ProfitHandler) Breakdown(c *gin.Context) {
	filter, ok := parseProfitFilter(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	items, err := h.profitService.Breakdown(c.Request.Context(), filter, c.DefaultQuery("dimension", service.ProfitDimensionGroup), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// Trend GET /api/v1/admin/profit/trend?granularity=day|hour
func (h *ProfitHandler) Trend(c *gin.Context) {
	filter, ok := parseProfitFilter(c)
	if !ok {
		return
	}
	points, err := h.profitService.Trend(c.Request.Context(), filter, c.DefaultQuery("granularity", "day"), c.Query("timezone"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, points)
}
