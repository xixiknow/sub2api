package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.AffiliateWithdrawRequestRepository = (*affiliateRepository)(nil)

const affiliateWithdrawRequestColumns = `
       awr.id,
       awr.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       awr.amount::double precision,
       awr.alipay_account,
       awr.status,
       COALESCE(awr.admin_note, ''),
       awr.reviewed_by,
       awr.reviewed_at,
       awr.created_at,
       awr.updated_at`

// CreateWithdrawRequest 在一个事务内冻结额度并创建提现申请。
func (r *affiliateRepository) CreateWithdrawRequest(ctx context.Context, in service.AffiliateWithdrawRequestCreate) (*service.AffiliateWithdrawRequest, error) {
	if in.UserID <= 0 {
		return nil, service.ErrUserNotFound
	}
	if in.Amount <= 0 {
		return nil, service.ErrAffiliateWithdrawAmountInvalid
	}

	var requestID int64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, in.UserID); err != nil {
			return err
		}
		// 先检查待审核申请，给出明确错误；唯一索引兜底并发提交。
		pending, err := queryPendingWithdraw(txCtx, txClient, in.UserID)
		if err != nil {
			return err
		}
		if pending != nil {
			return service.ErrAffiliateWithdrawPendingExists
		}
		if _, err := thawFrozenQuotaTx(txCtx, txClient, in.UserID); err != nil {
			return fmt.Errorf("thaw before withdraw request: %w", err)
		}

		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota - $1,
    updated_at = NOW()
WHERE user_id = $2
  AND aff_quota >= $1`, in.Amount, in.UserID)
		if err != nil {
			return fmt.Errorf("hold affiliate quota: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("hold affiliate quota: %w", err)
		}
		if affected == 0 {
			return service.ErrAffiliateQuotaInsufficient
		}

		ledgerID, err := insertAffiliateWithdrawLedger(txCtx, txClient, in.UserID, "withdraw_request", in.Amount)
		if err != nil {
			return err
		}

		rows, err := txClient.QueryContext(txCtx, `
INSERT INTO affiliate_withdraw_requests (
    user_id, amount, alipay_account, qr_image, status, hold_ledger_id, created_at, updated_at
)
VALUES ($1, $2, $3, $4, 'pending', $5, NOW(), NOW())
RETURNING id`, in.UserID, in.Amount, in.AlipayAccount, in.QRImage, ledgerID)
		if err != nil {
			if isAffiliateUniqueViolation(err) {
				return service.ErrAffiliateWithdrawPendingExists
			}
			return fmt.Errorf("insert affiliate withdraw request: %w", err)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				if isAffiliateUniqueViolation(err) {
					return service.ErrAffiliateWithdrawPendingExists
				}
				return err
			}
			return fmt.Errorf("insert affiliate withdraw request: no id returned")
		}
		if err := rows.Scan(&requestID); err != nil {
			return err
		}
		return rows.Close()
	})
	if err != nil {
		return nil, err
	}
	return r.getWithdrawRequest(ctx, clientFromContext(ctx, r.client), requestID, false)
}

// ResolveWithdrawRequest 处理待审核申请。驳回/撤销会退回额度。
func (r *affiliateRepository) ResolveWithdrawRequest(ctx context.Context, id, expectUserID int64, status, note string, reviewerID *int64) (*service.AffiliateWithdrawRequest, error) {
	switch status {
	case service.AffiliateWithdrawStatusPaid, service.AffiliateWithdrawStatusRejected, service.AffiliateWithdrawStatusCancelled:
	default:
		return nil, fmt.Errorf("invalid withdraw resolve status %q", status)
	}
	if id <= 0 {
		return nil, service.ErrAffiliateWithdrawNotFound
	}

	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		rows, err := txClient.QueryContext(txCtx, `
SELECT user_id, amount::double precision, status
FROM affiliate_withdraw_requests
WHERE id = $1
FOR UPDATE`, id)
		if err != nil {
			return fmt.Errorf("lock affiliate withdraw request: %w", err)
		}
		var (
			userID  int64
			amount  float64
			current string
		)
		found := rows.Next()
		if found {
			if err := rows.Scan(&userID, &amount, &current); err != nil {
				_ = rows.Close()
				return err
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		_ = rows.Close()
		if !found || (expectUserID > 0 && userID != expectUserID) {
			return service.ErrAffiliateWithdrawNotFound
		}
		if current != service.AffiliateWithdrawStatusPending {
			return service.ErrAffiliateWithdrawNotPending
		}

		var releaseLedgerID sql.NullInt64
		if status != service.AffiliateWithdrawStatusPaid {
			if _, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota + $1,
    updated_at = NOW()
WHERE user_id = $2`, amount, userID); err != nil {
				return fmt.Errorf("release affiliate quota: %w", err)
			}
			ledgerID, err := insertAffiliateWithdrawLedger(txCtx, txClient, userID, "withdraw_reverse", amount)
			if err != nil {
				return err
			}
			releaseLedgerID = sql.NullInt64{Int64: ledgerID, Valid: true}
		}

		var reviewer sql.NullInt64
		if reviewerID != nil && *reviewerID > 0 {
			reviewer = sql.NullInt64{Int64: *reviewerID, Valid: true}
		}
		var adminNote sql.NullString
		if note = strings.TrimSpace(note); note != "" {
			adminNote = sql.NullString{String: note, Valid: true}
		}
		if _, err := txClient.ExecContext(txCtx, `
UPDATE affiliate_withdraw_requests
SET status = $1,
    admin_note = $2,
    reviewed_by = $3,
    reviewed_at = NOW(),
    release_ledger_id = $4,
    updated_at = NOW()
WHERE id = $5
  AND status = 'pending'`, status, adminNote, reviewer, releaseLedgerID, id); err != nil {
			return fmt.Errorf("resolve affiliate withdraw request: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.getWithdrawRequest(ctx, clientFromContext(ctx, r.client), id, false)
}

func (r *affiliateRepository) GetWithdrawRequest(ctx context.Context, id int64) (*service.AffiliateWithdrawRequest, error) {
	return r.getWithdrawRequest(ctx, clientFromContext(ctx, r.client), id, true)
}

func (r *affiliateRepository) getWithdrawRequest(ctx context.Context, client affiliateQueryExecer, id int64, withImage bool) (*service.AffiliateWithdrawRequest, error) {
	if id <= 0 {
		return nil, service.ErrAffiliateWithdrawNotFound
	}
	imageColumn := "''"
	if withImage {
		imageColumn = "awr.qr_image"
	}
	rows, err := client.QueryContext(ctx, `
SELECT`+affiliateWithdrawRequestColumns+`,
       `+imageColumn+`
FROM affiliate_withdraw_requests awr
JOIN users u ON u.id = awr.user_id
WHERE awr.id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get affiliate withdraw request: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateWithdrawNotFound
	}
	item, err := scanAffiliateWithdrawRequest(rows, true)
	if err != nil {
		return nil, err
	}
	return item, rows.Err()
}

func (r *affiliateRepository) ListWithdrawRequests(ctx context.Context, filter service.AffiliateWithdrawRequestFilter) ([]service.AffiliateWithdrawRequest, int64, error) {
	client := clientFromContext(ctx, r.client)
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 7)
	if filter.UserID > 0 {
		args = append(args, filter.UserID)
		clauses = append(clauses, fmt.Sprintf("awr.user_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("awr.status = $%d", len(args)))
	}
	if filter.StartAt != nil {
		args = append(args, *filter.StartAt)
		clauses = append(clauses, fmt.Sprintf("awr.created_at >= $%d", len(args)))
	}
	if filter.EndAt != nil {
		args = append(args, *filter.EndAt)
		clauses = append(clauses, fmt.Sprintf("awr.created_at <= $%d", len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		n := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(LOWER(u.email) LIKE $%d OR LOWER(u.username) LIKE $%d OR u.id::text LIKE $%d OR LOWER(awr.alipay_account) LIKE $%d)",
			n, n, n, n))
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	baseJoin := `
FROM affiliate_withdraw_requests awr
JOIN users u ON u.id = awr.user_id` + where

	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+baseJoin, args...)
	if err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT`+affiliateWithdrawRequestColumns+`
`+baseJoin+`
ORDER BY awr.created_at DESC, awr.id DESC
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate withdraw requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateWithdrawRequest, 0)
	for rows.Next() {
		item, err := scanAffiliateWithdrawRequest(rows, false)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) GetPendingWithdrawRequest(ctx context.Context, userID int64) (*service.AffiliatePendingWithdraw, error) {
	return queryPendingWithdraw(ctx, clientFromContext(ctx, r.client), userID)
}

func queryPendingWithdraw(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliatePendingWithdraw, error) {
	if userID <= 0 {
		return nil, nil
	}
	rows, err := client.QueryContext(ctx, `
SELECT id, amount::double precision, created_at
FROM affiliate_withdraw_requests
WHERE user_id = $1
  AND status = 'pending'
ORDER BY id DESC
LIMIT 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("query pending affiliate withdraw: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var pending service.AffiliatePendingWithdraw
	if err := rows.Scan(&pending.ID, &pending.Amount, &pending.CreatedAt); err != nil {
		return nil, err
	}
	return &pending, rows.Err()
}

// insertAffiliateWithdrawLedger 写一条带额度快照的提现流水，返回流水 ID。
func insertAffiliateWithdrawLedger(ctx context.Context, client affiliateQueryExecer, userID int64, action string, amount float64) (int64, error) {
	snapshot, err := queryAffiliateTransferSnapshot(ctx, client, userID)
	if err != nil {
		return 0, err
	}
	rows, err := client.QueryContext(ctx, `
INSERT INTO user_affiliate_ledger (
    user_id, action, amount, source_user_id,
    balance_after, aff_quota_after, aff_frozen_quota_after, aff_history_quota_after,
    created_at, updated_at
)
VALUES ($1, $2, $3, NULL, $4, $5, $6, $7, NOW(), NOW())
RETURNING id`,
		userID, action, amount,
		snapshot.BalanceAfter, snapshot.AvailableQuotaAfter, snapshot.FrozenQuotaAfter, snapshot.HistoryQuotaAfter,
	)
	if err != nil {
		return 0, fmt.Errorf("insert affiliate %s ledger: %w", action, err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("insert affiliate %s ledger: no id returned", action)
	}
	var id int64
	if err := rows.Scan(&id); err != nil {
		return 0, err
	}
	return id, rows.Close()
}

type affiliateRowScanner interface {
	Scan(dest ...any) error
}

func scanAffiliateWithdrawRequest(rows affiliateRowScanner, withImageColumn bool) (*service.AffiliateWithdrawRequest, error) {
	var (
		item       service.AffiliateWithdrawRequest
		reviewedBy sql.NullInt64
		reviewedAt sql.NullTime
		image      string
	)
	dest := []any{
		&item.ID,
		&item.UserID,
		&item.UserEmail,
		&item.Username,
		&item.Amount,
		&item.AlipayAccount,
		&item.Status,
		&item.AdminNote,
		&reviewedBy,
		&reviewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	}
	if withImageColumn {
		dest = append(dest, &image)
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	item.ReviewedBy = nullableInt64Ptr(reviewedBy)
	if reviewedAt.Valid {
		t := reviewedAt.Time
		item.ReviewedAt = &t
	}
	item.QRImage = image
	return &item, nil
}
