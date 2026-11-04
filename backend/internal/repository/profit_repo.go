package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// profitRepository 直接聚合 usage_logs（预聚合表不含账号/品牌/模型维度，也无法套用品牌成本系数）。
type profitRepository struct {
	db *sql.DB
}

func NewProfitRepository(db *sql.DB) service.ProfitRepository {
	return &profitRepository{db: db}
}

const (
	profitFrom = `
FROM usage_logs ul
LEFT JOIN accounts a ON a.id = ul.account_id`
	// 上游声明倍率，按优先级：
	//  1. 用量日志写入时记录的快照 upstream_rate_multiplier（含当时高峰系数）；
	//  2. 历史日志无快照时，取账号当前探测到的基准倍率 resolved_rate_multiplier；
	//  3. 账号未探测过时回退到用量日志里的账号倍率快照，再回退 1。
	profitUpstreamRateExpr = `COALESCE(
		ul.upstream_rate_multiplier,
		CASE WHEN jsonb_typeof(a.extra #> '{upstream_billing_probe,data,resolved_rate_multiplier}') = 'number'
			THEN (a.extra #>> '{upstream_billing_probe,data,resolved_rate_multiplier}')::numeric END,
		ul.account_rate_multiplier, 1)`
	profitCostExpr = `ul.total_cost * ` + profitUpstreamRateExpr
	profitAggCols  = `
	COUNT(*) AS requests,
	COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS total_tokens,
	COALESCE(SUM(ul.total_cost), 0) AS standard_cost,
	COALESCE(SUM(ul.actual_cost), 0) AS revenue,
	COALESCE(SUM(` + profitCostExpr + `), 0) AS cost`
)

// profitWhere 构造公共过滤；withBillingType=false 时不加订阅过滤（汇总需要同时统计订阅行）。
func profitWhere(filter service.ProfitFilter, withBillingType bool) (string, []any) {
	conds := []string{"ul.created_at >= $1", "ul.created_at < $2"}
	args := []any{filter.StartTime, filter.EndTime}
	add := func(cond string, value any) {
		args = append(args, value)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if filter.GroupID > 0 {
		add("ul.group_id = $%d", filter.GroupID)
	}
	if filter.AccountID > 0 {
		add("ul.account_id = $%d", filter.AccountID)
	}
	if p := strings.TrimSpace(filter.Platform); p != "" {
		add("a.platform = $%d", p)
	}
	if withBillingType && !filter.IncludeSubscription {
		add("ul.billing_type = $%d", int16(service.BillingTypeBalance))
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func (r *profitRepository) GetProfitSummary(ctx context.Context, filter service.ProfitFilter) (*service.ProfitSummary, error) {
	where, args := profitWhere(filter, false)
	includeSub := "FALSE"
	if filter.IncludeSubscription {
		includeSub = "TRUE"
	}
	subCond := fmt.Sprintf("ul.billing_type = %d", service.BillingTypeSubscription)
	counted := fmt.Sprintf("(%s OR NOT (%s))", includeSub, subCond)
	query := fmt.Sprintf(`
SELECT
	COUNT(*) FILTER (WHERE %[1]s),
	COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens) FILTER (WHERE %[1]s), 0),
	COALESCE(SUM(ul.total_cost) FILTER (WHERE %[1]s), 0),
	COALESCE(SUM(ul.actual_cost) FILTER (WHERE %[1]s), 0),
	COALESCE(SUM(%[3]s) FILTER (WHERE %[1]s), 0),
	COUNT(*) FILTER (WHERE %[2]s),
	COALESCE(SUM(ul.actual_cost) FILTER (WHERE %[2]s), 0),
	COALESCE(SUM(%[3]s) FILTER (WHERE %[2]s), 0)
%[4]s%[5]s`, counted, subCond, profitCostExpr, profitFrom, where)
	out := &service.ProfitSummary{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&out.Requests, &out.TotalTokens, &out.StandardCost, &out.Revenue, &out.Cost,
		&out.SubscriptionRequests, &out.SubscriptionConsumption, &out.SubscriptionCost,
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *profitRepository) GetProfitBreakdown(ctx context.Context, filter service.ProfitFilter, dimension string, limit int) ([]service.ProfitBreakdownItem, error) {
	where, args := profitWhere(filter, true)
	var selectCols, joins, groupBy string
	switch dimension {
	case service.ProfitDimensionGroup:
		selectCols = `COALESCE(ul.group_id, 0) AS id, COALESCE(MAX(g.name), '') AS name, '' AS platform, MAX(g.rate_multiplier)::float8 AS rate`
		joins = ` LEFT JOIN groups g ON g.id = ul.group_id`
		groupBy = `COALESCE(ul.group_id, 0)`
	case service.ProfitDimensionAccount:
		selectCols = `ul.account_id AS id, COALESCE(MAX(a.name), '') AS name, COALESCE(MAX(a.platform), '') AS platform, MAX(a.rate_multiplier)::float8 AS rate`
		groupBy = `ul.account_id`
	case service.ProfitDimensionPlatform:
		selectCols = `0::bigint AS id, COALESCE(a.platform, '') AS name, COALESCE(a.platform, '') AS platform, NULL::float8 AS rate`
		groupBy = `COALESCE(a.platform, '')`
	case service.ProfitDimensionModel:
		modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")
		selectCols = `0::bigint AS id, ` + modelExpr + ` AS name, '' AS platform, NULL::float8 AS rate`
		groupBy = modelExpr
	default:
		return nil, service.ErrProfitInvalidFilter
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
SELECT %s, %s
%s%s%s
GROUP BY %s
ORDER BY revenue DESC
LIMIT $%d`, selectCols, profitAggCols, profitFrom, joins, where, groupBy, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.ProfitBreakdownItem{}
	for rows.Next() {
		var item service.ProfitBreakdownItem
		var rate sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.Name, &item.Platform, &rate,
			&item.Requests, &item.TotalTokens, &item.StandardCost, &item.Revenue, &item.Cost); err != nil {
			return nil, err
		}
		if rate.Valid {
			v := rate.Float64
			item.RateMultiplier = &v
		}
		if dimension == service.ProfitDimensionModel || dimension == service.ProfitDimensionPlatform {
			item.Key = item.Name
		} else {
			item.Key = strconv.FormatInt(item.ID, 10)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *profitRepository) GetProfitTrend(ctx context.Context, filter service.ProfitFilter, granularity, timezone string) ([]service.ProfitTrendPoint, error) {
	where, args := profitWhere(filter, true)
	args = append(args, timezone)
	tzArg := len(args)
	format := "YYYY-MM-DD"
	if granularity == "hour" {
		format = "YYYY-MM-DD HH24:00"
	}
	bucket := fmt.Sprintf("to_char(date_trunc('%s', ul.created_at AT TIME ZONE $%d::text), '%s')", granularity, tzArg, format)
	query := fmt.Sprintf(`
SELECT %s AS bucket, %s
%s%s
GROUP BY 1
ORDER BY 1`, bucket, profitAggCols, profitFrom, where)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.ProfitTrendPoint{}
	for rows.Next() {
		var p service.ProfitTrendPoint
		if err := rows.Scan(&p.Date, &p.Requests, &p.TotalTokens, &p.StandardCost, &p.Revenue, &p.Cost); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
