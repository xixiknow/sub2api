package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type upstreamBalanceOpsRepository struct {
	db *sql.DB
}

func NewUpstreamBalanceOpsRepository(db *sql.DB) service.UpstreamBalanceOpsRepository {
	return &upstreamBalanceOpsRepository{db: db}
}

func (r *upstreamBalanceOpsRepository) InsertSnapshot(ctx context.Context, accountID int64, result *service.UpstreamBalanceResult) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO upstream_balance_snapshots (account_id, balance, used, limit_amount, unlimited, source, fetched_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		accountID, nullFloat(result.Balance), nullFloat(result.Used), nullFloat(result.Limit), result.Unlimited, result.Source, result.FetchedAt)
	return err
}

func (r *upstreamBalanceOpsRepository) ListSnapshots(ctx context.Context, accountIDs []int64, since time.Time) (map[int64][]service.UpstreamBalancePoint, error) {
	out := make(map[int64][]service.UpstreamBalancePoint, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT account_id, balance, fetched_at
FROM upstream_balance_snapshots
WHERE account_id = ANY($1) AND fetched_at >= $2 AND balance IS NOT NULL
ORDER BY account_id, fetched_at`, pq.Array(accountIDs), since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			accountID int64
			point     service.UpstreamBalancePoint
		)
		if err := rows.Scan(&accountID, &point.Balance, &point.At); err != nil {
			return nil, err
		}
		out[accountID] = append(out[accountID], point)
	}
	return out, rows.Err()
}

func (r *upstreamBalanceOpsRepository) PruneSnapshots(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM upstream_balance_snapshots WHERE fetched_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetAccountUpstreamCost 按账号汇总本平台记录的上游成本，成本口径与盈利统计一致。
func (r *upstreamBalanceOpsRepository) GetAccountUpstreamCost(ctx context.Context, accountIDs []int64, since time.Time) (map[int64]float64, error) {
	out := make(map[int64]float64, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT ul.account_id, COALESCE(SUM(`+profitCostExpr+`), 0)`+profitFrom+`
WHERE ul.account_id = ANY($1) AND ul.created_at >= $2
GROUP BY ul.account_id`, pq.Array(accountIDs), since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			accountID int64
			cost      float64
		)
		if err := rows.Scan(&accountID, &cost); err != nil {
			return nil, err
		}
		out[accountID] = cost
	}
	return out, rows.Err()
}

func (r *upstreamBalanceOpsRepository) GetAccountDailyUpstreamCost(ctx context.Context, accountIDs []int64, since time.Time, timezone string) ([]service.UpstreamCostBucket, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT TO_CHAR(DATE_TRUNC('day', ul.created_at AT TIME ZONE $3::text), 'YYYY-MM-DD') AS bucket,
	COALESCE(SUM(`+profitCostExpr+`), 0)`+profitFrom+`
WHERE ul.account_id = ANY($1) AND ul.created_at >= $2
GROUP BY bucket
ORDER BY bucket`, pq.Array(accountIDs), since, timezone)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.UpstreamCostBucket
	for rows.Next() {
		var bucket service.UpstreamCostBucket
		if err := rows.Scan(&bucket.Date, &bucket.Cost); err != nil {
			return nil, err
		}
		out = append(out, bucket)
	}
	return out, rows.Err()
}

func (r *upstreamBalanceOpsRepository) InsertAlert(ctx context.Context, alert *service.UpstreamBalanceAlert) error {
	levels, err := json.Marshal(alert.AccountLevels)
	if err != nil {
		return fmt.Errorf("marshal account levels: %w", err)
	}
	return r.db.QueryRowContext(ctx, `
INSERT INTO upstream_balance_alerts (level, account_levels, content, provider, phones, status, error)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at`,
		alert.Level, levels, alert.Content, alert.Provider, alert.Phones, alert.Status, alert.Error,
	).Scan(&alert.ID, &alert.CreatedAt)
}

func (r *upstreamBalanceOpsRepository) ListAlerts(ctx context.Context, since time.Time, limit int) ([]service.UpstreamBalanceAlert, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, level, account_levels, content, provider, phones, status, error, created_at
FROM upstream_balance_alerts
WHERE created_at >= $1
ORDER BY created_at DESC
LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.UpstreamBalanceAlert
	for rows.Next() {
		var (
			alert  service.UpstreamBalanceAlert
			levels []byte
		)
		if err := rows.Scan(&alert.ID, &alert.Level, &levels, &alert.Content, &alert.Provider, &alert.Phones, &alert.Status, &alert.Error, &alert.CreatedAt); err != nil {
			return nil, err
		}
		if len(levels) > 0 {
			_ = json.Unmarshal(levels, &alert.AccountLevels)
		}
		out = append(out, alert)
	}
	return out, rows.Err()
}

func nullFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
