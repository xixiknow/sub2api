package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 盈利统计口径（逐条 usage_log）：
//
//	收入 revenue = actual_cost（token 标准计费 × 分组倍率，已含用户专属倍率 / 高峰倍率）
//	成本 cost    = total_cost * 上游声明倍率（token 标准计费 × 请求时刻的上游声明倍率快照 upstream_rate_multiplier；
//	               历史日志无快照时取账号当前探测的基准倍率，未探测时回退 account_rate_multiplier，再回退 1；
//	               不使用 account_stats_cost）
//	利润 profit  = revenue - cost
//
// 订阅计费行的 actual_cost 是套餐额度消耗而非现金收入，默认不计入，单独汇总。

const (
	ProfitDimensionGroup    = "group"
	ProfitDimensionAccount  = "account"
	ProfitDimensionPlatform = "platform"
	ProfitDimensionModel    = "model"

	profitMaxRange = 92 * 24 * time.Hour
)

var ErrProfitInvalidFilter = infraerrors.BadRequest("PROFIT_INVALID_FILTER", "invalid profit filter")

// ProfitFilter 盈利查询过滤条件。
type ProfitFilter struct {
	StartTime           time.Time
	EndTime             time.Time
	GroupID             int64
	AccountID           int64
	Platform            string
	IncludeSubscription bool
}

// ProfitTotals 一组聚合值。
type ProfitTotals struct {
	Requests     int64   `json:"requests"`
	TotalTokens  int64   `json:"total_tokens"`
	StandardCost float64 `json:"standard_cost"` // total_cost：倍率前的标准价
	Revenue      float64 `json:"revenue"`
	Cost         float64 `json:"cost"`
	Profit       float64 `json:"profit"`
	Margin       float64 `json:"margin"` // profit / revenue；revenue 为 0 时为 0
}

// ProfitSummary 区间汇总。
type ProfitSummary struct {
	ProfitTotals
	// 订阅计费行：套餐额度消耗与其上游成本（不论 IncludeSubscription 都会返回）
	SubscriptionConsumption float64 `json:"subscription_consumption"`
	SubscriptionCost        float64 `json:"subscription_cost"`
	SubscriptionRequests    int64   `json:"subscription_requests"`
}

// ProfitBreakdownItem 按维度拆分的一行。
type ProfitBreakdownItem struct {
	ProfitTotals
	Key      string `json:"key"`
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform,omitempty"`
	// 配置倍率：分组维度为分组倍率，账号维度为账号上游倍率
	RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
}

// ProfitTrendPoint 趋势点。
type ProfitTrendPoint struct {
	ProfitTotals
	Date string `json:"date"`
}

// ProfitRepository 盈利统计查询。
type ProfitRepository interface {
	GetProfitSummary(ctx context.Context, filter ProfitFilter) (*ProfitSummary, error)
	GetProfitBreakdown(ctx context.Context, filter ProfitFilter, dimension string, limit int) ([]ProfitBreakdownItem, error)
	GetProfitTrend(ctx context.Context, filter ProfitFilter, granularity, timezone string) ([]ProfitTrendPoint, error)
}

// ProfitService 盈利运营统计。
type ProfitService struct {
	repo ProfitRepository
}

func NewProfitService(repo ProfitRepository) *ProfitService {
	return &ProfitService{repo: repo}
}

func (s *ProfitService) Summary(ctx context.Context, filter ProfitFilter) (*ProfitSummary, error) {
	if err := validateProfitFilter(filter); err != nil {
		return nil, err
	}
	out, err := s.repo.GetProfitSummary(ctx, filter)
	if err != nil {
		return nil, err
	}
	out.ProfitTotals.finalize()
	return out, nil
}

func (s *ProfitService) Breakdown(ctx context.Context, filter ProfitFilter, dimension string, limit int) ([]ProfitBreakdownItem, error) {
	if err := validateProfitFilter(filter); err != nil {
		return nil, err
	}
	switch dimension {
	case ProfitDimensionGroup, ProfitDimensionAccount, ProfitDimensionPlatform, ProfitDimensionModel:
	default:
		return nil, ErrProfitInvalidFilter.WithMetadata(map[string]string{"field": "dimension"})
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	items, err := s.repo.GetProfitBreakdown(ctx, filter, dimension, limit)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].ProfitTotals.finalize()
	}
	return items, nil
}

func (s *ProfitService) Trend(ctx context.Context, filter ProfitFilter, granularity, timezone string) ([]ProfitTrendPoint, error) {
	if err := validateProfitFilter(filter); err != nil {
		return nil, err
	}
	if granularity != "hour" {
		granularity = "day"
	}
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, ErrProfitInvalidFilter.WithMetadata(map[string]string{"field": "timezone"})
	}
	points, err := s.repo.GetProfitTrend(ctx, filter, granularity, timezone)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].ProfitTotals.finalize()
	}
	return points, nil
}

func (t *ProfitTotals) finalize() {
	t.Profit = t.Revenue - t.Cost
	if t.Revenue > 0 {
		t.Margin = t.Profit / t.Revenue
	} else {
		t.Margin = 0
	}
}

func validateProfitFilter(filter ProfitFilter) error {
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() || !filter.EndTime.After(filter.StartTime) {
		return ErrProfitInvalidFilter.WithMetadata(map[string]string{"field": "time_range"})
	}
	if filter.EndTime.Sub(filter.StartTime) > profitMaxRange {
		return ErrProfitInvalidFilter.WithMetadata(map[string]string{"field": "time_range", "max_days": "92"})
	}
	return nil
}
