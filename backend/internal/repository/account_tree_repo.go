package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountTreeRepository struct {
	db *sql.DB
}

func NewAccountTreeRepository(db *sql.DB) service.AccountTreeRepository {
	return &accountTreeRepository{db: db}
}

func (r *accountTreeRepository) GroupSummary(ctx context.Context, filter service.AccountTreeSummaryFilter) ([]service.AccountTreeSummaryItem, error) {
	where := []string{"a.deleted_at IS NULL"}
	args := []any{}
	if filter.Platform != "" {
		args = append(args, filter.Platform)
		where = append(where, fmt.Sprintf("a.platform = $%d", len(args)))
	}
	if filter.AccountType != "" {
		args = append(args, filter.AccountType)
		where = append(where, fmt.Sprintf("a.type = $%d", len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+escapeLikePattern(search)+"%")
		where = append(where, fmt.Sprintf("a.name ILIKE $%d", len(args)))
	}
	query := `
SELECT COALESCE(ag.group_id, 0) AS group_id,
       COUNT(*) AS total,
       COUNT(*) FILTER (WHERE a.status = 'active') AS active,
       COUNT(*) FILTER (WHERE a.status = 'active' AND a.schedulable) AS schedulable,
       COUNT(*) FILTER (WHERE a.status = 'error') AS error
FROM accounts a
LEFT JOIN account_groups ag ON ag.account_id = a.id
WHERE ` + strings.Join(where, " AND ") + `
GROUP BY 1
ORDER BY 1`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.AccountTreeSummaryItem{}
	for rows.Next() {
		var item service.AccountTreeSummaryItem
		if err := rows.Scan(&item.GroupID, &item.Total, &item.Active, &item.Schedulable, &item.Error); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
