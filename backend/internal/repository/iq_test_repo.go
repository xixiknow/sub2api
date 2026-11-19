package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 模型智力测试仓储（fork 扩展，原生 SQL）。

type iqTestRepository struct {
	db *sql.DB
}

func NewIQTestRepository(db *sql.DB) service.IQTestRepository {
	return &iqTestRepository{db: db}
}

// ---------------- questions ----------------

const iqQuestionColumns = `id, slug, kind, title, prompt, keywords, builtin, enabled, created_at, updated_at`

func scanIQQuestion(row interface{ Scan(...any) error }) (*service.IQQuestion, error) {
	q := &service.IQQuestion{}
	var kw []byte
	if err := row.Scan(&q.ID, &q.Slug, &q.Kind, &q.Title, &q.Prompt, &kw, &q.Builtin, &q.Enabled, &q.CreatedAt, &q.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrIQQuestionNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(kw, &q.Keywords)
	if q.Keywords == nil {
		q.Keywords = []string{}
	}
	return q, nil
}

func iqJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return []byte("{}")
	}
	return b
}

func iqJSONArray(v []string) []byte {
	if v == nil {
		v = []string{}
	}
	b, _ := json.Marshal(v)
	return b
}

func (r *iqTestRepository) UpsertBuiltinQuestion(ctx context.Context, q *service.IQQuestion) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO iq_questions (slug, kind, title, prompt, keywords, builtin, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, TRUE, TRUE, NOW(), NOW())
		ON CONFLICT (slug) DO UPDATE SET kind = EXCLUDED.kind, title = EXCLUDED.title, prompt = EXCLUDED.prompt,
			keywords = EXCLUDED.keywords, builtin = TRUE, updated_at = NOW()
	`, q.Slug, q.Kind, q.Title, q.Prompt, iqJSONArray(q.Keywords))
	return err
}

func (r *iqTestRepository) ListQuestions(ctx context.Context) ([]*service.IQQuestion, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+iqQuestionColumns+` FROM iq_questions ORDER BY builtin DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*service.IQQuestion
	for rows.Next() {
		q, err := scanIQQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *iqTestRepository) GetQuestion(ctx context.Context, slug string) (*service.IQQuestion, error) {
	return scanIQQuestion(r.db.QueryRowContext(ctx, `SELECT `+iqQuestionColumns+` FROM iq_questions WHERE slug = $1`, slug))
}

func (r *iqTestRepository) SaveQuestion(ctx context.Context, q *service.IQQuestion) (*service.IQQuestion, error) {
	return scanIQQuestion(r.db.QueryRowContext(ctx, `
		INSERT INTO iq_questions (slug, kind, title, prompt, keywords, builtin, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, $6, NOW(), NOW())
		ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title, prompt = EXCLUDED.prompt, keywords = EXCLUDED.keywords,
			enabled = EXCLUDED.enabled, updated_at = NOW()
		WHERE iq_questions.builtin = FALSE
		RETURNING `+iqQuestionColumns,
		q.Slug, q.Kind, q.Title, q.Prompt, iqJSONArray(q.Keywords), q.Enabled))
}

func (r *iqTestRepository) DeleteQuestion(ctx context.Context, slug string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM iq_questions WHERE slug = $1 AND builtin = FALSE`, slug)
	return err
}

// ---------------- plans ----------------

const iqPlanColumns = `id, account_id, model_id, question_slug, parallel_count, cron_expression, enabled, fail_action,
	fail_threshold, consecutive_fails, action_applied, max_results, last_verdict, last_run_at, next_run_at, created_at, updated_at`

func scanIQPlan(row interface{ Scan(...any) error }) (*service.IQTestPlan, error) {
	p := &service.IQTestPlan{}
	var lastRun, nextRun sql.NullTime
	if err := row.Scan(&p.ID, &p.AccountID, &p.ModelID, &p.QuestionSlug, &p.ParallelCount, &p.CronExpression, &p.Enabled,
		&p.FailAction, &p.FailThreshold, &p.ConsecutiveFails, &p.ActionApplied, &p.MaxResults, &p.LastVerdict,
		&lastRun, &nextRun, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if lastRun.Valid {
		t := lastRun.Time
		p.LastRunAt = &t
	}
	if nextRun.Valid {
		t := nextRun.Time
		p.NextRunAt = &t
	}
	return p, nil
}

func (r *iqTestRepository) listPlans(ctx context.Context, query string, args ...any) ([]*service.IQTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*service.IQTestPlan
	for rows.Next() {
		p, err := scanIQPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *iqTestRepository) CreatePlan(ctx context.Context, p *service.IQTestPlan) (*service.IQTestPlan, error) {
	return scanIQPlan(r.db.QueryRowContext(ctx, `
		INSERT INTO iq_test_plans (account_id, model_id, question_slug, parallel_count, cron_expression, enabled, fail_action,
			fail_threshold, max_results, next_run_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING `+iqPlanColumns,
		p.AccountID, p.ModelID, p.QuestionSlug, p.ParallelCount, p.CronExpression, p.Enabled, p.FailAction,
		p.FailThreshold, p.MaxResults, p.NextRunAt))
}

func (r *iqTestRepository) UpdatePlan(ctx context.Context, p *service.IQTestPlan) (*service.IQTestPlan, error) {
	return scanIQPlan(r.db.QueryRowContext(ctx, `
		UPDATE iq_test_plans SET model_id = $2, question_slug = $3, parallel_count = $4, cron_expression = $5, enabled = $6,
			fail_action = $7, fail_threshold = $8, max_results = $9, next_run_at = $10, updated_at = NOW()
		WHERE id = $1
		RETURNING `+iqPlanColumns,
		p.ID, p.ModelID, p.QuestionSlug, p.ParallelCount, p.CronExpression, p.Enabled, p.FailAction,
		p.FailThreshold, p.MaxResults, p.NextRunAt))
}

func (r *iqTestRepository) GetPlan(ctx context.Context, id int64) (*service.IQTestPlan, error) {
	return scanIQPlan(r.db.QueryRowContext(ctx, `SELECT `+iqPlanColumns+` FROM iq_test_plans WHERE id = $1`, id))
}

func (r *iqTestRepository) DeletePlan(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM iq_test_plans WHERE id = $1`, id)
	return err
}

func (r *iqTestRepository) ListPlansByAccount(ctx context.Context, accountID int64) ([]*service.IQTestPlan, error) {
	return r.listPlans(ctx, `SELECT `+iqPlanColumns+` FROM iq_test_plans WHERE account_id = $1 ORDER BY id DESC`, accountID)
}

func (r *iqTestRepository) ListDuePlans(ctx context.Context, now time.Time) ([]*service.IQTestPlan, error) {
	return r.listPlans(ctx, `SELECT `+iqPlanColumns+` FROM iq_test_plans WHERE enabled = TRUE AND next_run_at <= $1 ORDER BY next_run_at ASC`, now)
}

func (r *iqTestRepository) ListPenaltyPlans(ctx context.Context) ([]*service.IQTestPlan, error) {
	return r.listPlans(ctx, `SELECT `+iqPlanColumns+` FROM iq_test_plans WHERE action_applied = TRUE AND fail_action = $1`, service.IQActionHealthPenalty)
}

func (r *iqTestRepository) UpdatePlanAfterRun(ctx context.Context, p *service.IQTestPlan) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE iq_test_plans SET consecutive_fails = $2, action_applied = $3, last_verdict = $4, last_run_at = $5,
			next_run_at = $6, updated_at = NOW()
		WHERE id = $1
	`, p.ID, p.ConsecutiveFails, p.ActionApplied, p.LastVerdict, p.LastRunAt, p.NextRunAt)
	return err
}

// ---------------- results ----------------

const iqResultColumns = `id, plan_id, account_id, model_id, question_slug, round_id, prompt, response_text, verdict, score,
	detail, error_message, label, latency_ms, created_at`

func scanIQResult(row interface{ Scan(...any) error }) (*service.IQTestResult, error) {
	res := &service.IQTestResult{}
	var planID sql.NullInt64
	var score sql.NullFloat64
	var detail []byte
	if err := row.Scan(&res.ID, &planID, &res.AccountID, &res.ModelID, &res.QuestionSlug, &res.RoundID, &res.Prompt,
		&res.ResponseText, &res.Verdict, &score, &detail, &res.ErrorMessage, &res.Label, &res.LatencyMs, &res.CreatedAt); err != nil {
		return nil, err
	}
	if planID.Valid {
		v := planID.Int64
		res.PlanID = &v
	}
	if score.Valid {
		v := score.Float64
		res.Score = &v
	}
	_ = json.Unmarshal(detail, &res.Detail)
	if res.Detail == nil {
		res.Detail = map[string]any{}
	}
	return res, nil
}

func (r *iqTestRepository) CreateResult(ctx context.Context, res *service.IQTestResult) (*service.IQTestResult, error) {
	return scanIQResult(r.db.QueryRowContext(ctx, `
		INSERT INTO iq_test_results (plan_id, account_id, model_id, question_slug, round_id, prompt, response_text, verdict,
			score, detail, error_message, latency_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		RETURNING `+iqResultColumns,
		res.PlanID, res.AccountID, res.ModelID, res.QuestionSlug, res.RoundID, res.Prompt, res.ResponseText, res.Verdict,
		res.Score, iqJSON(res.Detail), res.ErrorMessage, res.LatencyMs))
}

func (r *iqTestRepository) GetResult(ctx context.Context, id int64) (*service.IQTestResult, error) {
	return scanIQResult(r.db.QueryRowContext(ctx, `SELECT `+iqResultColumns+` FROM iq_test_results WHERE id = $1`, id))
}

func (r *iqTestRepository) ListResults(ctx context.Context, f service.IQResultFilter) ([]*service.IQTestResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+iqResultColumns+` FROM iq_test_results
		WHERE ($1 = 0 OR account_id = $1) AND ($2 = 0 OR plan_id = $2)
		ORDER BY id DESC LIMIT $3
	`, f.AccountID, f.PlanID, f.Limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*service.IQTestResult
	for rows.Next() {
		res, err := scanIQResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *iqTestRepository) SetResultLabel(ctx context.Context, id int64, label string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE iq_test_results SET label = $2 WHERE id = $1`, id, label)
	return err
}

func (r *iqTestRepository) PruneResults(ctx context.Context, planID int64, keep int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM iq_test_results WHERE plan_id = $1 AND label = '' AND id NOT IN (
			SELECT id FROM iq_test_results WHERE plan_id = $1 ORDER BY id DESC LIMIT $2
		)
	`, planID, keep)
	return err
}

// ---------------- samples ----------------

func (r *iqTestRepository) CreateSample(ctx context.Context, s *service.IQReferenceSample) (*service.IQReferenceSample, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO iq_reference_samples (question_slug, model_id, label, source, note, svg, features, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id, created_at
	`, s.QuestionSlug, s.ModelID, s.Label, s.Source, s.Note, s.SVG, iqJSON(s.Features))
	out := *s
	if err := row.Scan(&out.ID, &out.CreatedAt); err != nil {
		return nil, err
	}
	out.SVG = ""
	return &out, nil
}

func (r *iqTestRepository) ListSamples(ctx context.Context, questionSlug string, withSVG bool) ([]*service.IQReferenceSample, error) {
	svgCol := `''`
	if withSVG {
		svgCol = `svg`
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, question_slug, model_id, label, source, note, `+svgCol+`, created_at
		FROM iq_reference_samples WHERE question_slug = $1 ORDER BY id DESC
	`, questionSlug)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*service.IQReferenceSample
	for rows.Next() {
		s := &service.IQReferenceSample{}
		if err := rows.Scan(&s.ID, &s.QuestionSlug, &s.ModelID, &s.Label, &s.Source, &s.Note, &s.SVG, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *iqTestRepository) DeleteSample(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM iq_reference_samples WHERE id = $1`, id)
	return err
}

func (r *iqTestRepository) SampleStats(ctx context.Context) ([]service.IQSampleStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT question_slug, model_id, label, COUNT(*) FROM iq_reference_samples
		GROUP BY question_slug, model_id, label ORDER BY question_slug, model_id, label
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.IQSampleStat
	for rows.Next() {
		var s service.IQSampleStat
		if err := rows.Scan(&s.QuestionSlug, &s.ModelID, &s.Label, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---------------- classifier ----------------

func (r *iqTestRepository) GetClassifier(ctx context.Context, questionSlug string) (*service.IQClassifierInfo, error) {
	info := &service.IQClassifierInfo{QuestionSlug: questionSlug}
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT model, n_good, n_bad, accuracy, trained_at FROM iq_classifier_models WHERE question_slug = $1
	`, questionSlug).Scan(&raw, &info.NGood, &info.NBad, &info.Accuracy, &info.TrainedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m service.IQLogisticModel
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	info.Model = &m
	return info, nil
}

func (r *iqTestRepository) SaveClassifier(ctx context.Context, info *service.IQClassifierInfo) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO iq_classifier_models (question_slug, model, n_good, n_bad, accuracy, trained_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (question_slug) DO UPDATE SET model = EXCLUDED.model, n_good = EXCLUDED.n_good, n_bad = EXCLUDED.n_bad,
			accuracy = EXCLUDED.accuracy, trained_at = EXCLUDED.trained_at
	`, info.QuestionSlug, iqJSON(info.Model), info.NGood, info.NBad, info.Accuracy, info.TrainedAt)
	return err
}
