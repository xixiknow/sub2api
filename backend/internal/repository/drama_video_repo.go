package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type dramaVideoRepository struct {
	db *sql.DB
}

func NewDramaVideoRepository(db *sql.DB) service.DramaVideoTaskRepository {
	return &dramaVideoRepository{db: db}
}

func (r *dramaVideoRepository) Create(ctx context.Context, params service.CreateDramaVideoTaskParams) (*service.DramaVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video repository db is nil")
	}
	if strings.TrimSpace(params.Status) == "" {
		params.Status = service.DramaVideoStatusQueued
	}
	query := `
		INSERT INTO drama_video_tasks (
			task_id, user_id, api_key_id, group_id, account_id, model, upstream_model,
			status, progress, request_hash, resolution, aspect_ratio,
			duration_seconds, hold_amount, asset_ids, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, NOW(), NOW()
		)
		RETURNING ` + dramaVideoSelectColumns
	return scanDramaVideoTask(r.db.QueryRowContext(ctx, query,
		params.TaskID,
		params.UserID,
		params.APIKeyID,
		params.GroupID,
		dramaNullInt64(params.AccountID),
		params.Model,
		params.UpstreamModel,
		params.Status,
		params.Progress,
		dramaNullString(params.RequestHash),
		dramaNullString(params.Resolution),
		dramaNullString(params.AspectRatio),
		dramaNullInt(params.DurationSeconds),
		params.HoldAmount,
		pq.StringArray(params.AssetIDs),
	))
}

func (r *dramaVideoRepository) GetByTaskID(ctx context.Context, taskID string) (*service.DramaVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video repository db is nil")
	}
	query := `SELECT ` + dramaVideoSelectColumns + ` FROM drama_video_tasks WHERE task_id = $1`
	task, err := scanDramaVideoTask(r.db.QueryRowContext(ctx, query, strings.TrimSpace(taskID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrDramaVideoTaskNotFound
	}
	return task, err
}

func (r *dramaVideoRepository) GetForOwner(ctx context.Context, owner service.DramaVideoOwner, taskID string) (*service.DramaVideoTask, error) {
	task, err := r.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.UserID != owner.UserID || task.APIKeyID != owner.APIKeyID {
		return nil, service.ErrDramaVideoTaskNotFound
	}
	return task, nil
}

func (r *dramaVideoRepository) ListByAPIKey(ctx context.Context, userID, apiKeyID int64, limit, offset int) ([]*service.DramaVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video repository db is nil")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT ` + dramaVideoSelectColumns + `
		FROM drama_video_tasks
		WHERE user_id = $1 AND api_key_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.db.QueryContext(ctx, query, userID, apiKeyID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]*service.DramaVideoTask, 0, limit)
	for rows.Next() {
		task, err := scanDramaVideoTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *dramaVideoRepository) UpdateStatus(ctx context.Context, update service.DramaVideoTaskStatusUpdate) (*service.DramaVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video repository db is nil")
	}
	status := strings.TrimSpace(update.Status)
	if status == "" {
		return r.GetByTaskID(ctx, update.TaskID)
	}
	query := `
		UPDATE drama_video_tasks
		SET status = $2,
			progress = COALESCE($3, progress),
			upstream_task_id = COALESCE(NULLIF($4, ''), upstream_task_id),
			error = $5,
			submitted_at = COALESCE($6, submitted_at),
			completed_at = COALESCE($7, completed_at),
			updated_at = NOW()
		WHERE task_id = $1
		RETURNING ` + dramaVideoSelectColumns
	task, err := scanDramaVideoTask(r.db.QueryRowContext(ctx, query,
		strings.TrimSpace(update.TaskID),
		status,
		dramaNullIntPtr(update.Progress),
		dramaNullStringPtr(update.UpstreamTaskID),
		dramaNullJSON(update.Error),
		dramaNullTimePtr(update.SubmittedAt),
		dramaNullTimePtr(update.CompletedAt),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrDramaVideoTaskNotFound
	}
	return task, err
}

func (r *dramaVideoRepository) MarkCompleted(ctx context.Context, update service.DramaVideoTaskCompletionUpdate) (*service.DramaVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video repository db is nil")
	}
	query := `
		UPDATE drama_video_tasks
		SET status = $2,
			progress = 100,
			actual_cost = $3,
			output_path = NULLIF($4, ''),
			output_mime = NULLIF($5, ''),
			output_bytes = $6,
			output_sha256 = NULLIF($7, ''),
			completed_at = $8,
			output_expires_at = $9,
			updated_at = NOW()
		WHERE task_id = $1
		RETURNING ` + dramaVideoSelectColumns
	task, err := scanDramaVideoTask(r.db.QueryRowContext(ctx, query,
		strings.TrimSpace(update.TaskID),
		service.DramaVideoStatusCompleted,
		update.ActualCost,
		update.OutputPath,
		update.OutputMIME,
		update.OutputBytes,
		update.OutputSHA256,
		update.CompletedAt,
		dramaNullTimePtr(update.OutputExpiresAt),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrDramaVideoTaskNotFound
	}
	return task, err
}

func (r *dramaVideoRepository) ListOutputsDueForCleanup(ctx context.Context, now time.Time, limit int) ([]*service.DramaVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("drama video repository db is nil")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT ` + dramaVideoSelectColumns + `
		FROM drama_video_tasks
		WHERE output_deleted_at IS NULL
			AND status = $1
			AND output_expires_at IS NOT NULL
			AND output_expires_at <= $2
		ORDER BY output_expires_at ASC
		LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, service.DramaVideoStatusCompleted, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]*service.DramaVideoTask, 0, limit)
	for rows.Next() {
		task, err := scanDramaVideoTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *dramaVideoRepository) MarkOutputDeleted(ctx context.Context, taskID string, deletedAt time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("drama video repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE drama_video_tasks
		SET output_deleted_at = $2, output_path = '', updated_at = NOW()
		WHERE task_id = $1 AND output_deleted_at IS NULL`,
		strings.TrimSpace(taskID), deletedAt)
	return err
}

const dramaVideoSelectColumns = `
	id,
	task_id,
	COALESCE(upstream_task_id, ''),
	user_id,
	api_key_id,
	group_id,
	account_id,
	model,
	upstream_model,
	status,
	progress,
	COALESCE(request_hash, ''),
	COALESCE(resolution, ''),
	COALESCE(aspect_ratio, ''),
	COALESCE(duration_seconds, 0),
	COALESCE(hold_amount, 0),
	actual_cost,
	COALESCE(output_path, ''),
	COALESCE(output_mime, ''),
	COALESCE(output_bytes, 0),
	COALESCE(output_sha256, ''),
	COALESCE(asset_ids, '{}'),
	error,
	created_at,
	updated_at,
	submitted_at,
	completed_at,
	output_expires_at,
	output_deleted_at`

func scanDramaVideoTask(row rowScanner) (*service.DramaVideoTask, error) {
	var task service.DramaVideoTask
	var accountID sql.NullInt64
	var actualCost sql.NullFloat64
	var errRaw []byte
	var submittedAt sql.NullTime
	var completedAt sql.NullTime
	var expiresAt sql.NullTime
	var deletedAt sql.NullTime
	var assetIDs pq.StringArray
	if err := row.Scan(
		&task.ID,
		&task.TaskID,
		&task.UpstreamTaskID,
		&task.UserID,
		&task.APIKeyID,
		&task.GroupID,
		&accountID,
		&task.Model,
		&task.UpstreamModel,
		&task.Status,
		&task.Progress,
		&task.RequestHash,
		&task.Resolution,
		&task.AspectRatio,
		&task.DurationSeconds,
		&task.HoldAmount,
		&actualCost,
		&task.OutputPath,
		&task.OutputMIME,
		&task.OutputBytes,
		&task.OutputSHA256,
		&assetIDs,
		&errRaw,
		&task.CreatedAt,
		&task.UpdatedAt,
		&submittedAt,
		&completedAt,
		&expiresAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	task.AssetIDs = []string(assetIDs)
	if accountID.Valid {
		v := accountID.Int64
		task.AccountID = &v
	}
	if actualCost.Valid {
		v := actualCost.Float64
		task.ActualCost = &v
	}
	if len(errRaw) > 0 && json.Valid(errRaw) {
		task.Error = json.RawMessage(errRaw)
	}
	if submittedAt.Valid {
		v := submittedAt.Time
		task.SubmittedAt = &v
	}
	if completedAt.Valid {
		v := completedAt.Time
		task.CompletedAt = &v
	}
	if expiresAt.Valid {
		v := expiresAt.Time
		task.OutputExpiresAt = &v
	}
	if deletedAt.Valid {
		v := deletedAt.Time
		task.OutputDeletedAt = &v
	}
	return &task, nil
}

func dramaNullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	return sql.NullString{String: s, Valid: s != ""}
}

func dramaNullStringPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return dramaNullString(*s)
}

func dramaNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}

func dramaNullInt(v int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(v), Valid: v > 0}
}

func dramaNullIntPtr(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func dramaNullTimePtr(t *time.Time) sql.NullTime {
	if t == nil || t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func dramaNullJSON(raw json.RawMessage) any {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	return []byte(raw)
}
