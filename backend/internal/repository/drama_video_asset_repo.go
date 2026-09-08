package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type dramaVideoAssetRepository struct {
	db *sql.DB
}

func NewDramaVideoAssetRepository(db *sql.DB) service.DramaVideoAssetRepository {
	return &dramaVideoAssetRepository{db: db}
}

const dramaVideoAssetSelectColumns = `
	id, asset_id, user_id, kind, mime, bytes, sha256, COALESCE(original_name, ''),
	storage_path, created_at, last_used_at, deleted_at`

func (r *dramaVideoAssetRepository) Create(ctx context.Context, asset *service.DramaVideoAsset) (*service.DramaVideoAsset, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video asset repository db is nil")
	}
	query := `
		INSERT INTO drama_video_assets (
			asset_id, user_id, kind, mime, bytes, sha256, original_name, storage_path, created_at, last_used_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		RETURNING ` + dramaVideoAssetSelectColumns
	return scanDramaVideoAsset(r.db.QueryRowContext(ctx, query,
		asset.AssetID, asset.UserID, asset.Kind, asset.MIME, asset.Bytes, asset.SHA256,
		dramaNullString(asset.OriginalName), asset.StoragePath,
	))
}

func (r *dramaVideoAssetRepository) GetByAssetID(ctx context.Context, assetID string) (*service.DramaVideoAsset, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video asset repository db is nil")
	}
	query := `SELECT ` + dramaVideoAssetSelectColumns + ` FROM drama_video_assets WHERE asset_id = $1`
	asset, err := scanDramaVideoAsset(r.db.QueryRowContext(ctx, query, strings.TrimSpace(assetID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrDramaVideoAssetNotFound
	}
	return asset, err
}

func (r *dramaVideoAssetRepository) GetActiveByUserSHA(ctx context.Context, userID int64, sha256 string) (*service.DramaVideoAsset, error) {
	query := `SELECT ` + dramaVideoAssetSelectColumns + `
		FROM drama_video_assets WHERE user_id = $1 AND sha256 = $2 AND deleted_at IS NULL`
	asset, err := scanDramaVideoAsset(r.db.QueryRowContext(ctx, query, userID, strings.TrimSpace(sha256)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrDramaVideoAssetNotFound
	}
	return asset, err
}

func (r *dramaVideoAssetRepository) ListByUser(ctx context.Context, userID int64, kind string, limit, offset int) ([]*service.DramaVideoAsset, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	kind = strings.TrimSpace(kind)
	var rows *sql.Rows
	var err error
	if kind == "" {
		rows, err = r.db.QueryContext(ctx, `SELECT `+dramaVideoAssetSelectColumns+`
			FROM drama_video_assets WHERE user_id = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, `SELECT `+dramaVideoAssetSelectColumns+`
			FROM drama_video_assets WHERE user_id = $1 AND kind = $2 AND deleted_at IS NULL
			ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`, userID, kind, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDramaVideoAssetRows(rows)
}

func (r *dramaVideoAssetRepository) SumBytesByUser(ctx context.Context, userID int64) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(bytes), 0) FROM drama_video_assets WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Int64, nil
}

func (r *dramaVideoAssetRepository) SoftDelete(ctx context.Context, assetID string, deletedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE drama_video_assets SET deleted_at = $2 WHERE asset_id = $1 AND deleted_at IS NULL`,
		strings.TrimSpace(assetID), deletedAt)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrDramaVideoAssetNotFound
	}
	return nil
}

func (r *dramaVideoAssetRepository) TouchLastUsed(ctx context.Context, assetIDs []string, usedAt time.Time) error {
	if len(assetIDs) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE drama_video_assets SET last_used_at = $2 WHERE asset_id = ANY($1) AND deleted_at IS NULL`,
		pq.StringArray(assetIDs), usedAt)
	return err
}

func (r *dramaVideoAssetRepository) ListExpired(ctx context.Context, cutoff time.Time, limit int) ([]*service.DramaVideoAsset, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+dramaVideoAssetSelectColumns+`
		FROM drama_video_assets
		WHERE deleted_at IS NULL AND last_used_at <= $1
		ORDER BY last_used_at ASC
		LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDramaVideoAssetRows(rows)
}

func (r *dramaVideoAssetRepository) AdminList(ctx context.Context, userID int64, kind string, limit, offset int) ([]*service.DramaVideoAsset, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := `deleted_at IS NULL`
	args := []any{}
	n := 1
	if userID > 0 {
		where += ` AND user_id = $` + strconv.Itoa(n)
		args = append(args, userID)
		n++
	}
	if kind = strings.TrimSpace(kind); kind != "" {
		where += ` AND kind = $` + strconv.Itoa(n)
		args = append(args, kind)
		n++
	}
	var total int
	countQuery := `SELECT COUNT(*) FROM drama_video_assets WHERE ` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	query := `SELECT ` + dramaVideoAssetSelectColumns + `
		FROM drama_video_assets WHERE ` + where + `
		ORDER BY created_at DESC, id DESC LIMIT $` + strconv.Itoa(n) + ` OFFSET $` + strconv.Itoa(n+1)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanDramaVideoAssetRows(rows)
	return items, total, err
}

func scanDramaVideoAssetRows(rows *sql.Rows) ([]*service.DramaVideoAsset, error) {
	out := make([]*service.DramaVideoAsset, 0)
	for rows.Next() {
		asset, err := scanDramaVideoAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, asset)
	}
	return out, rows.Err()
}

func scanDramaVideoAsset(row rowScanner) (*service.DramaVideoAsset, error) {
	var asset service.DramaVideoAsset
	var deletedAt sql.NullTime
	if err := row.Scan(
		&asset.ID, &asset.AssetID, &asset.UserID, &asset.Kind, &asset.MIME, &asset.Bytes, &asset.SHA256,
		&asset.OriginalName, &asset.StoragePath, &asset.CreatedAt, &asset.LastUsedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		v := deletedAt.Time
		asset.DeletedAt = &v
	}
	return &asset, nil
}
