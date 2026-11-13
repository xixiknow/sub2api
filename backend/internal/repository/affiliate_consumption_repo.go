package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// LockConsumptionSettlementCursor 在当前事务内锁定消费返利结算游标并返回已结算到的
// usage_logs.id。必须在事务上下文中调用；游标行缺失时（迁移未写入）以当前最大 id
// 初始化，避免对历史消费补发返利。
func (r *affiliateRepository) LockConsumptionSettlementCursor(ctx context.Context) (int64, error) {
	tx := dbent.TxFromContext(ctx)
	if tx == nil {
		return 0, fmt.Errorf("lock consumption settlement cursor: transaction required")
	}
	client := tx.Client()
	if _, err := client.ExecContext(ctx, `
INSERT INTO affiliate_consumption_settlement_cursor (id, last_usage_log_id, updated_at)
SELECT 1, COALESCE(MAX(id), 0), NOW() FROM usage_logs
ON CONFLICT (id) DO NOTHING`); err != nil {
		return 0, fmt.Errorf("init consumption settlement cursor: %w", err)
	}
	rows, err := client.QueryContext(ctx,
		`SELECT last_usage_log_id FROM affiliate_consumption_settlement_cursor WHERE id = 1 FOR UPDATE`)
	if err != nil {
		return 0, fmt.Errorf("lock consumption settlement cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var cursor int64
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("lock consumption settlement cursor: row missing")
	}
	if err := rows.Scan(&cursor); err != nil {
		return 0, err
	}
	return cursor, rows.Close()
}

// GetConsumptionSettlementUpperBound 返回 id > afterID 且 created_at <= before 的最大
// usage_logs.id；没有新日志时返回 afterID。
func (r *affiliateRepository) GetConsumptionSettlementUpperBound(ctx context.Context, afterID int64, before time.Time) (int64, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx,
		`SELECT COALESCE(MAX(id), $1) FROM usage_logs WHERE id > $1 AND created_at <= $2`,
		afterID, before)
	if err != nil {
		return 0, fmt.Errorf("query consumption settlement upper bound: %w", err)
	}
	defer func() { _ = rows.Close() }()
	upper := afterID
	if rows.Next() {
		if err := rows.Scan(&upper); err != nil {
			return 0, err
		}
	}
	return upper, rows.Close()
}

// SumInviteeBalanceConsumption 汇总 (afterID, upperID] 区间内、已绑定邀请人的用户
// 的余额扣费（billing_type=0、actual_cost>0）。durationDays>0 时只计返利有效期内
// （user_affiliates.created_at + durationDays 之前）的消费。
func (r *affiliateRepository) SumInviteeBalanceConsumption(ctx context.Context, afterID, upperID int64, durationDays int) ([]service.AffiliateInviteeConsumption, error) {
	if upperID <= afterID {
		return nil, nil
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT ul.user_id,
       SUM(ul.actual_cost)::double precision
FROM usage_logs ul
JOIN user_affiliates ua ON ua.user_id = ul.user_id
WHERE ul.id > $1
  AND ul.id <= $2
  AND ul.billing_type = $3
  AND ul.actual_cost > 0
  AND ua.inviter_id IS NOT NULL
  AND ($4 <= 0 OR ul.created_at <= ua.created_at + make_interval(days => $4))
GROUP BY ul.user_id
ORDER BY ul.user_id`, afterID, upperID, service.BillingTypeBalance, durationDays)
	if err != nil {
		return nil, fmt.Errorf("sum invitee balance consumption: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.AffiliateInviteeConsumption, 0)
	for rows.Next() {
		var item service.AffiliateInviteeConsumption
		var amount sql.NullFloat64
		if err := rows.Scan(&item.UserID, &amount); err != nil {
			return nil, err
		}
		item.Amount = amount.Float64
		out = append(out, item)
	}
	return out, rows.Err()
}

// AdvanceConsumptionSettlementCursor 把结算游标推进到 upperID（只增不减）。
func (r *affiliateRepository) AdvanceConsumptionSettlementCursor(ctx context.Context, upperID int64) error {
	client := clientFromContext(ctx, r.client)
	if _, err := client.ExecContext(ctx, `
UPDATE affiliate_consumption_settlement_cursor
SET last_usage_log_id = GREATEST(last_usage_log_id, $1), updated_at = NOW()
WHERE id = 1`, upperID); err != nil {
		return fmt.Errorf("advance consumption settlement cursor: %w", err)
	}
	return nil
}
