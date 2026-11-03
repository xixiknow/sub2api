package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProfitRepoBreakdownAccountDimension(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	mock.ExpectQuery(`(?s)SELECT ul.account_id AS id.*COALESCE\(ul.account_stats_cost, ul.total_cost\) \* COALESCE\(ul.account_rate_multiplier, 1\).*ul.group_id = \$3.*ul.billing_type = \$4.*GROUP BY ul.account_id.*LIMIT \$5`).
		WithArgs(start, end, int64(7), int16(service.BillingTypeBalance), 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "platform", "rate", "requests", "total_tokens", "standard_cost", "revenue", "cost"}).
			AddRow(int64(11), "relay-1", "anthropic", 0.8, int64(3), int64(900), 1.5, 2.0, 1.2))

	repo := NewProfitRepository(db)
	items, err := repo.GetProfitBreakdown(context.Background(), service.ProfitFilter{StartTime: start, EndTime: end, GroupID: 7}, service.ProfitDimensionAccount, 50)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "11", items[0].Key)
	require.Equal(t, "anthropic", items[0].Platform)
	require.NotNil(t, items[0].RateMultiplier)
	require.InDelta(t, 0.8, *items[0].RateMultiplier, 1e-9)
	require.InDelta(t, 2.0, items[0].Revenue, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitRepoSummaryCountsSubscriptionSeparately(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	mock.ExpectQuery(`(?s)FILTER \(WHERE \(FALSE OR NOT \(ul.billing_type = 1\)\)\).*a.platform = \$3`).
		WithArgs(start, end, "openai").
		WillReturnRows(sqlmock.NewRows([]string{"a", "b", "c", "d", "e", "f", "g", "h"}).
			AddRow(int64(10), int64(1000), 5.0, 8.0, 3.0, int64(2), 1.0, 0.4))

	repo := NewProfitRepository(db)
	summary, err := repo.GetProfitSummary(context.Background(), service.ProfitFilter{StartTime: start, EndTime: end, Platform: "openai"})
	require.NoError(t, err)
	require.Equal(t, int64(10), summary.Requests)
	require.InDelta(t, 8.0, summary.Revenue, 1e-9)
	require.InDelta(t, 1.0, summary.SubscriptionConsumption, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitRepoTrendCastsTimezone(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	mock.ExpectQuery(`(?s)date_trunc\('day', ul.created_at AT TIME ZONE \$4::text\).*GROUP BY 1`).
		WithArgs(start, end, int16(service.BillingTypeBalance), "Asia/Shanghai").
		WillReturnRows(sqlmock.NewRows([]string{"bucket", "requests", "total_tokens", "standard_cost", "revenue", "cost"}).
			AddRow("2026-11-01", int64(1), int64(10), 0.1, 0.2, 0.05))

	repo := NewProfitRepository(db)
	points, err := repo.GetProfitTrend(context.Background(), service.ProfitFilter{StartTime: start, EndTime: end}, "day", "Asia/Shanghai")
	require.NoError(t, err)
	require.Len(t, points, 1)
	require.Equal(t, "2026-11-01", points[0].Date)
	require.NoError(t, mock.ExpectationsWereMet())
}
