package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const (
	EmailCampaignStatusDraft     = "draft"
	EmailCampaignStatusPending   = "pending"
	EmailCampaignStatusRunning   = "running"
	EmailCampaignStatusCompleted = "completed"
	EmailCampaignStatusCanceled  = "canceled"
	EmailCampaignStatusFailed    = "failed"

	EmailCampaignRecipientPending = "pending"
	EmailCampaignRecipientSent    = "sent"
	EmailCampaignRecipientFailed  = "failed"
	EmailCampaignRecipientSkipped = "skipped"

	emailCampaignDefaultThrottle = 30
	emailCampaignMaxThrottle     = 600
	emailCampaignBatchSize       = 50
	emailCampaignPollInterval    = 10 * time.Second
	// A running campaign whose worker stopped heartbeating is reclaimed after this.
	emailCampaignStaleAfter = 2 * time.Minute
	// Abort a campaign when SMTP keeps failing instead of failing every recipient.
	emailCampaignMaxConsecutiveFailures = 10
	emailCampaignMaxListItems           = 500
	emailCampaignAudienceSampleSize     = 10
)

var (
	ErrEmailCampaignNotFound   = infraerrors.NotFound("EMAIL_CAMPAIGN_NOT_FOUND", "email campaign not found")
	ErrEmailCampaignNotDraft   = infraerrors.Conflict("EMAIL_CAMPAIGN_NOT_DRAFT", "only draft campaigns can be changed")
	ErrEmailCampaignNoAudience = infraerrors.BadRequest("EMAIL_CAMPAIGN_NO_AUDIENCE", "no users match the audience")
	ErrEmailCampaignNoSMTP     = infraerrors.BadRequest("EMAIL_CAMPAIGN_SMTP_NOT_CONFIGURED", "SMTP is not configured")
)

// EmailCampaignAudience selects recipients. All set conditions are combined with AND;
// UserIDs and Emails together form one OR-ed explicit list.
type EmailCampaignAudience struct {
	InactiveDays   *int       `json:"inactive_days,omitempty"`
	NeverUsedAPI   bool       `json:"never_used_api,omitempty"`
	BalanceMin     *float64   `json:"balance_min,omitempty"`
	BalanceMax     *float64   `json:"balance_max,omitempty"`
	RegisteredFrom *time.Time `json:"registered_from,omitempty"`
	RegisteredTo   *time.Time `json:"registered_to,omitempty"`
	UserIDs        []int64    `json:"user_ids,omitempty"`
	Emails         []string   `json:"emails,omitempty"`
	IncludeAdmins  bool       `json:"include_admins,omitempty"`
}

type EmailCampaign struct {
	ID                int64                 `json:"id"`
	Name              string                `json:"name"`
	Subject           string                `json:"subject"`
	BodyHTML          string                `json:"body_html"`
	LayoutMode        string                `json:"layout_mode"`
	Title             string                `json:"title"`
	AccentColor       string                `json:"accent_color"`
	Locale            string                `json:"locale"`
	Audience          EmailCampaignAudience `json:"audience"`
	Status            string                `json:"status"`
	ThrottlePerMinute int                   `json:"throttle_per_minute"`
	TotalCount        int                   `json:"total_count"`
	SentCount         int                   `json:"sent_count"`
	FailedCount       int                   `json:"failed_count"`
	SkippedCount      int                   `json:"skipped_count"`
	ErrorMessage      string                `json:"error_message"`
	CreatedBy         *int64                `json:"created_by,omitempty"`
	StartedAt         *time.Time            `json:"started_at,omitempty"`
	FinishedAt        *time.Time            `json:"finished_at,omitempty"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

type EmailCampaignInput struct {
	Name              string                `json:"name"`
	Subject           string                `json:"subject"`
	BodyHTML          string                `json:"body_html"`
	LayoutMode        string                `json:"layout_mode"`
	Title             string                `json:"title"`
	AccentColor       string                `json:"accent_color"`
	Locale            string                `json:"locale"`
	Audience          EmailCampaignAudience `json:"audience"`
	ThrottlePerMinute int                   `json:"throttle_per_minute"`
}

type EmailCampaignRecipient struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Email          string     `json:"email"`
	RecipientName  string     `json:"recipient_name"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
	Status         string     `json:"status"`
	Error          string     `json:"error"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
}

type EmailCampaignAudienceUser struct {
	UserID         int64      `json:"user_id"`
	Email          string     `json:"email"`
	Username       string     `json:"username"`
	Balance        float64    `json:"balance"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
}

type EmailCampaignAudiencePreview struct {
	Count  int                         `json:"count"`
	Sample []EmailCampaignAudienceUser `json:"sample"`
}

type emailCampaignSender interface {
	SendCampaign(ctx context.Context, content EmailCampaignContent, recipient EmailCampaignRecipientVars, checkUnsubscribe bool) error
	PreviewCampaign(ctx context.Context, content EmailCampaignContent) (NotificationEmailPreview, error)
}

// EmailCampaignService manages marketing email campaigns and runs the throttled sender.
type EmailCampaignService struct {
	db           *sql.DB
	sender       emailCampaignSender
	emailService *EmailService
	instanceID   string
	now          func() time.Time

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewEmailCampaignService(db *sql.DB, notificationEmailService *NotificationEmailService, emailService *EmailService) *EmailCampaignService {
	s := &EmailCampaignService{
		db:           db,
		emailService: emailService,
		instanceID:   uuid.NewString(),
		now:          time.Now,
		stopCh:       make(chan struct{}),
	}
	// Avoid storing a typed nil pointer in the interface.
	if notificationEmailService != nil {
		s.sender = notificationEmailService
	}
	return s
}

// ProvideEmailCampaignService creates and starts EmailCampaignService.
func ProvideEmailCampaignService(db *sql.DB, notificationEmailService *NotificationEmailService, emailService *EmailService) *EmailCampaignService {
	svc := NewEmailCampaignService(db, notificationEmailService, emailService)
	svc.Start()
	return svc
}

func (s *EmailCampaignService) Start() {
	if s == nil || s.db == nil || s.sender == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(emailCampaignPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *EmailCampaignService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

// ---- audience ----

func normalizeEmailCampaignAudience(a EmailCampaignAudience) EmailCampaignAudience {
	if a.InactiveDays != nil && *a.InactiveDays <= 0 {
		a.InactiveDays = nil
	}
	ids := make([]int64, 0, len(a.UserIDs))
	seenIDs := map[int64]struct{}{}
	for _, id := range a.UserIDs {
		if _, ok := seenIDs[id]; id > 0 && !ok {
			seenIDs[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	a.UserIDs = ids
	emails := make([]string, 0, len(a.Emails))
	seenEmails := map[string]struct{}{}
	for _, email := range a.Emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if _, ok := seenEmails[email]; email != "" && !ok {
			seenEmails[email] = struct{}{}
			emails = append(emails, email)
		}
	}
	a.Emails = emails
	return a
}

// emailCampaignAudienceWhere builds the WHERE clause over users u. Args start at $1.
func emailCampaignAudienceWhere(a EmailCampaignAudience, now time.Time) (string, []any) {
	conds := []string{"u.deleted_at IS NULL", "u.status = 'active'", "u.email <> ''"}
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if !a.IncludeAdmins {
		conds = append(conds, "u.role <> 'admin'")
	}
	if a.InactiveDays != nil {
		cutoff := arg(now.Add(-time.Duration(*a.InactiveDays) * 24 * time.Hour))
		conds = append(conds,
			"u.created_at < "+cutoff,
			"(u.last_active_at IS NULL OR u.last_active_at < "+cutoff+")",
			"(u.last_login_at IS NULL OR u.last_login_at < "+cutoff+")",
			"NOT EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = u.id AND ul.created_at >= "+cutoff+")",
		)
	}
	if a.NeverUsedAPI {
		conds = append(conds, "NOT EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = u.id)")
	}
	if a.BalanceMin != nil {
		conds = append(conds, "u.balance >= "+arg(*a.BalanceMin))
	}
	if a.BalanceMax != nil {
		conds = append(conds, "u.balance <= "+arg(*a.BalanceMax))
	}
	if a.RegisteredFrom != nil {
		conds = append(conds, "u.created_at >= "+arg(*a.RegisteredFrom))
	}
	if a.RegisteredTo != nil {
		conds = append(conds, "u.created_at < "+arg(*a.RegisteredTo))
	}
	var explicit []string
	if len(a.UserIDs) > 0 {
		explicit = append(explicit, "u.id = ANY("+arg(pq.Array(a.UserIDs))+")")
	}
	if len(a.Emails) > 0 {
		explicit = append(explicit, "LOWER(u.email) = ANY("+arg(pq.Array(a.Emails))+")")
	}
	if len(explicit) > 0 {
		conds = append(conds, "("+strings.Join(explicit, " OR ")+")")
	}
	return strings.Join(conds, " AND "), args
}

const emailCampaignLastActivityExpr = `GREATEST(u.created_at, u.last_active_at, u.last_login_at,
	(SELECT MAX(ul.created_at) FROM usage_logs ul WHERE ul.user_id = u.id))`

func (s *EmailCampaignService) PreviewAudience(ctx context.Context, audience EmailCampaignAudience) (EmailCampaignAudiencePreview, error) {
	where, args := emailCampaignAudienceWhere(normalizeEmailCampaignAudience(audience), s.now())
	var out EmailCampaignAudiencePreview
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users u WHERE "+where, args...).Scan(&out.Count); err != nil {
		return out, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.id, u.email, u.username, u.balance, `+emailCampaignLastActivityExpr+`
		FROM users u WHERE `+where+` ORDER BY u.id DESC LIMIT `+strconv.Itoa(emailCampaignAudienceSampleSize), args...)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	out.Sample = []EmailCampaignAudienceUser{}
	for rows.Next() {
		var u EmailCampaignAudienceUser
		var last sql.NullTime
		if err := rows.Scan(&u.UserID, &u.Email, &u.Username, &u.Balance, &last); err != nil {
			return out, err
		}
		if last.Valid {
			u.LastActivityAt = &last.Time
		}
		out.Sample = append(out.Sample, u)
	}
	return out, rows.Err()
}

// ---- CRUD ----

func (s *EmailCampaignService) normalizeInput(in EmailCampaignInput) (EmailCampaignInput, EmailCampaignContent, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		in.Name = strings.TrimSpace(in.Subject)
	}
	if len([]rune(in.Name)) > 200 {
		return in, EmailCampaignContent{}, infraerrors.BadRequest("EMAIL_CAMPAIGN_INVALID", "name cannot exceed 200 characters")
	}
	if in.ThrottlePerMinute <= 0 {
		in.ThrottlePerMinute = emailCampaignDefaultThrottle
	}
	if in.ThrottlePerMinute > emailCampaignMaxThrottle {
		in.ThrottlePerMinute = emailCampaignMaxThrottle
	}
	in.Audience = normalizeEmailCampaignAudience(in.Audience)
	content := normalizeEmailCampaignContent(EmailCampaignContent{
		Subject: in.Subject, BodyHTML: in.BodyHTML, LayoutMode: in.LayoutMode,
		Title: in.Title, AccentColor: in.AccentColor, Locale: in.Locale,
	})
	if len([]rune(content.Title)) > 200 {
		return in, content, infraerrors.BadRequest("EMAIL_CAMPAIGN_INVALID", "title cannot exceed 200 characters")
	}
	if err := ValidateEmailCampaignContent(content); err != nil {
		return in, content, infraerrors.BadRequest("EMAIL_CAMPAIGN_INVALID", err.Error())
	}
	in.Subject, in.BodyHTML, in.LayoutMode = content.Subject, content.BodyHTML, content.LayoutMode
	in.Title, in.AccentColor, in.Locale = content.Title, content.AccentColor, content.Locale
	return in, content, nil
}

func (s *EmailCampaignService) Create(ctx context.Context, in EmailCampaignInput, createdBy int64) (*EmailCampaign, error) {
	in, _, err := s.normalizeInput(in)
	if err != nil {
		return nil, err
	}
	audience, err := json.Marshal(in.Audience)
	if err != nil {
		return nil, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `INSERT INTO email_campaigns
		(name, subject, body_html, layout_mode, title, accent_color, locale, audience, throttle_per_minute, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		in.Name, in.Subject, in.BodyHTML, in.LayoutMode, in.Title, in.AccentColor, in.Locale, string(audience), in.ThrottlePerMinute, nullableUserID(createdBy),
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *EmailCampaignService) Update(ctx context.Context, id int64, in EmailCampaignInput) (*EmailCampaign, error) {
	in, _, err := s.normalizeInput(in)
	if err != nil {
		return nil, err
	}
	audience, err := json.Marshal(in.Audience)
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE email_campaigns SET name=$2, subject=$3, body_html=$4, layout_mode=$5, title=$6,
		accent_color=$7, locale=$8, audience=$9, throttle_per_minute=$10, updated_at=NOW()
		WHERE id=$1 AND status='draft'`,
		id, in.Name, in.Subject, in.BodyHTML, in.LayoutMode, in.Title, in.AccentColor, in.Locale, string(audience), in.ThrottlePerMinute)
	if err != nil {
		return nil, err
	}
	if err := s.requireAffected(ctx, id, res); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *EmailCampaignService) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM email_campaigns WHERE id=$1 AND status IN ('draft','completed','canceled','failed')`, id)
	if err != nil {
		return err
	}
	return s.requireAffected(ctx, id, res)
}

// requireAffected maps a zero-row update to not-found or wrong-status.
func (s *EmailCampaignService) requireAffected(ctx context.Context, id int64, res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return ErrEmailCampaignNotDraft
}

const emailCampaignSelect = `SELECT c.id, c.name, c.subject, c.body_html, c.layout_mode, c.title, c.accent_color, c.locale,
	c.audience, c.status, c.throttle_per_minute, c.total_count, c.error_message, c.created_by,
	c.started_at, c.finished_at, c.created_at, c.updated_at,
	COALESCE(r.sent, 0), COALESCE(r.failed, 0), COALESCE(r.skipped, 0)
	FROM email_campaigns c
	LEFT JOIN LATERAL (
		SELECT COUNT(*) FILTER (WHERE status='sent') AS sent,
			COUNT(*) FILTER (WHERE status='failed') AS failed,
			COUNT(*) FILTER (WHERE status='skipped') AS skipped
		FROM email_campaign_recipients WHERE campaign_id = c.id
	) r ON true`

type emailCampaignScanner interface {
	Scan(dest ...any) error
}

func scanEmailCampaign(row emailCampaignScanner) (*EmailCampaign, error) {
	var c EmailCampaign
	var audience []byte
	var createdBy sql.NullInt64
	var startedAt, finishedAt sql.NullTime
	if err := row.Scan(&c.ID, &c.Name, &c.Subject, &c.BodyHTML, &c.LayoutMode, &c.Title, &c.AccentColor, &c.Locale,
		&audience, &c.Status, &c.ThrottlePerMinute, &c.TotalCount, &c.ErrorMessage, &createdBy,
		&startedAt, &finishedAt, &c.CreatedAt, &c.UpdatedAt,
		&c.SentCount, &c.FailedCount, &c.SkippedCount); err != nil {
		return nil, err
	}
	if len(audience) > 0 {
		_ = json.Unmarshal(audience, &c.Audience)
	}
	if createdBy.Valid {
		c.CreatedBy = &createdBy.Int64
	}
	if startedAt.Valid {
		c.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		c.FinishedAt = &finishedAt.Time
	}
	return &c, nil
}

func (s *EmailCampaignService) Get(ctx context.Context, id int64) (*EmailCampaign, error) {
	c, err := scanEmailCampaign(s.db.QueryRowContext(ctx, emailCampaignSelect+` WHERE c.id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmailCampaignNotFound
	}
	return c, err
}

func (s *EmailCampaignService) List(ctx context.Context, page, pageSize int, status, search string) ([]EmailCampaign, int64, error) {
	page, pageSize = normalizeEmailCampaignPage(page, pageSize)
	conds := []string{"TRUE"}
	var args []any
	if status = strings.TrimSpace(status); status != "" {
		args = append(args, status)
		conds = append(conds, "c.status = $"+strconv.Itoa(len(args)))
	}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		conds = append(conds, "(c.name ILIKE $"+strconv.Itoa(len(args))+" OR c.subject ILIKE $"+strconv.Itoa(len(args))+")")
	}
	where := strings.Join(conds, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_campaigns c WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, emailCampaignSelect+" WHERE "+where+
		fmt.Sprintf(" ORDER BY c.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []EmailCampaign{}
	for rows.Next() {
		c, err := scanEmailCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *c)
	}
	return items, total, rows.Err()
}

func (s *EmailCampaignService) ListRecipients(ctx context.Context, campaignID int64, page, pageSize int, status string) ([]EmailCampaignRecipient, int64, error) {
	page, pageSize = normalizeEmailCampaignPage(page, pageSize)
	args := []any{campaignID}
	where := "campaign_id = $1"
	if status = strings.TrimSpace(status); status != "" {
		args = append(args, status)
		where += " AND status = $2"
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_campaign_recipients WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, email, recipient_name, last_activity_at, status, error, sent_at
		FROM email_campaign_recipients WHERE `+where+fmt.Sprintf(" ORDER BY id LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []EmailCampaignRecipient{}
	for rows.Next() {
		var r EmailCampaignRecipient
		var last, sent sql.NullTime
		if err := rows.Scan(&r.ID, &r.UserID, &r.Email, &r.RecipientName, &last, &r.Status, &r.Error, &sent); err != nil {
			return nil, 0, err
		}
		if last.Valid {
			r.LastActivityAt = &last.Time
		}
		if sent.Valid {
			r.SentAt = &sent.Time
		}
		items = append(items, r)
	}
	return items, total, rows.Err()
}

func normalizeEmailCampaignPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > emailCampaignMaxListItems {
		pageSize = emailCampaignMaxListItems
	}
	return page, pageSize
}

func nullableUserID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

// ---- preview / test / launch / cancel ----

func (s *EmailCampaignService) Preview(ctx context.Context, in EmailCampaignInput) (NotificationEmailPreview, error) {
	_, content, err := s.normalizeInput(in)
	if err != nil {
		return NotificationEmailPreview{}, err
	}
	return s.sender.PreviewCampaign(ctx, content)
}

// SendTest sends the campaign content to the given admin user's own mailbox.
func (s *EmailCampaignService) SendTest(ctx context.Context, in EmailCampaignInput, adminUserID int64) (string, error) {
	_, content, err := s.normalizeInput(in)
	if err != nil {
		return "", err
	}
	if err := s.ensureSMTP(ctx); err != nil {
		return "", err
	}
	var email, username string
	var balance float64
	err = s.db.QueryRowContext(ctx, `SELECT email, username, balance FROM users WHERE id=$1 AND deleted_at IS NULL`, adminUserID).Scan(&email, &username, &balance)
	if err != nil {
		return "", err
	}
	err = s.sender.SendCampaign(ctx, content, EmailCampaignRecipientVars{
		UserID: adminUserID, Email: email, Name: username, DaysInactive: "14", Balance: strconv.FormatFloat(balance, 'f', 2, 64),
	}, false)
	if err != nil {
		return "", infraerrors.BadRequest("EMAIL_CAMPAIGN_SEND_FAILED", err.Error())
	}
	return email, nil
}

func (s *EmailCampaignService) ensureSMTP(ctx context.Context) error {
	if s.emailService == nil {
		return ErrEmailCampaignNoSMTP
	}
	cfg, err := s.emailService.GetSMTPConfig(ctx)
	if err != nil || cfg == nil || strings.TrimSpace(cfg.Host) == "" {
		return ErrEmailCampaignNoSMTP
	}
	return nil
}

// Launch snapshots the audience into the recipient table and queues the campaign.
func (s *EmailCampaignService) Launch(ctx context.Context, id int64) (*EmailCampaign, error) {
	if err := s.ensureSMTP(ctx); err != nil {
		return nil, err
	}
	campaign, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if campaign.Status != EmailCampaignStatusDraft {
		return nil, ErrEmailCampaignNotDraft
	}
	if _, _, err := s.normalizeInput(campaignInput(campaign)); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM email_campaigns WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return nil, err
	}
	if status != EmailCampaignStatusDraft {
		return nil, ErrEmailCampaignNotDraft
	}
	where, args := emailCampaignAudienceWhere(normalizeEmailCampaignAudience(campaign.Audience), s.now())
	args = append(args, id)
	res, err := tx.ExecContext(ctx, `INSERT INTO email_campaign_recipients (campaign_id, user_id, email, recipient_name, balance, last_activity_at)
		SELECT $`+strconv.Itoa(len(args))+`, u.id, u.email, u.username, u.balance, `+emailCampaignLastActivityExpr+`
		FROM users u WHERE `+where+`
		ON CONFLICT (campaign_id, user_id) DO NOTHING`, args...)
	if err != nil {
		return nil, err
	}
	total, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, ErrEmailCampaignNoAudience
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_campaigns SET status='pending', total_count=$2, error_message='', updated_at=NOW() WHERE id=$1`, id, total); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	slog.Info("email campaign launched", "campaign_id", id, "recipients", total)
	return s.Get(ctx, id)
}

func campaignInput(c *EmailCampaign) EmailCampaignInput {
	return EmailCampaignInput{
		Name: c.Name, Subject: c.Subject, BodyHTML: c.BodyHTML, LayoutMode: c.LayoutMode, Title: c.Title,
		AccentColor: c.AccentColor, Locale: c.Locale, Audience: c.Audience, ThrottlePerMinute: c.ThrottlePerMinute,
	}
}

func (s *EmailCampaignService) Cancel(ctx context.Context, id int64) (*EmailCampaign, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE email_campaigns SET status='canceled', finished_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND status IN ('pending','running')`, id)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		if _, err := s.Get(ctx, id); err != nil {
			return nil, err
		}
		return nil, infraerrors.Conflict("EMAIL_CAMPAIGN_NOT_RUNNING", "campaign is not pending or running")
	}
	return s.Get(ctx, id)
}

// ---- worker ----

func (s *EmailCampaignService) runOnce() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	campaign, err := s.claim(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && ctx.Err() == nil {
			slog.Warn("email campaign claim failed", "error", err)
		}
		return
	}
	s.process(ctx, campaign)
}

func (s *EmailCampaignService) claim(ctx context.Context) (*EmailCampaign, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `UPDATE email_campaigns SET status='running', worker_id=$1, heartbeat_at=NOW(),
		started_at=COALESCE(started_at, NOW()), updated_at=NOW()
		WHERE id = (
			SELECT id FROM email_campaigns
			WHERE status='pending' OR (status='running' AND (heartbeat_at IS NULL OR heartbeat_at < $2))
			ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED
		) RETURNING id`, s.instanceID, s.now().Add(-emailCampaignStaleAfter)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// heartbeat refreshes ownership; false means the campaign was canceled or taken over.
func (s *EmailCampaignService) heartbeat(ctx context.Context, id int64) bool {
	res, err := s.db.ExecContext(ctx, `UPDATE email_campaigns SET heartbeat_at=NOW() WHERE id=$1 AND worker_id=$2 AND status='running'`, id, s.instanceID)
	if err != nil {
		return false
	}
	n, err := res.RowsAffected()
	return err == nil && n > 0
}

func (s *EmailCampaignService) finish(ctx context.Context, id int64, status, message string) {
	_, err := s.db.ExecContext(ctx, `UPDATE email_campaigns SET status=$3, error_message=$4, finished_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND worker_id=$2 AND status='running'`, id, s.instanceID, status, message)
	if err != nil {
		slog.Warn("email campaign finish failed", "campaign_id", id, "error", err)
	}
}

func (s *EmailCampaignService) process(ctx context.Context, campaign *EmailCampaign) {
	content := EmailCampaignContent{
		Subject: campaign.Subject, BodyHTML: campaign.BodyHTML, LayoutMode: campaign.LayoutMode,
		Title: campaign.Title, AccentColor: campaign.AccentColor, Locale: campaign.Locale,
	}
	throttle := campaign.ThrottlePerMinute
	if throttle <= 0 {
		throttle = emailCampaignDefaultThrottle
	}
	interval := time.Minute / time.Duration(throttle)
	consecutiveFailures := 0
	slog.Info("email campaign started", "campaign_id", campaign.ID, "total", campaign.TotalCount)

	for {
		batch, err := s.pendingRecipients(ctx, campaign.ID)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("email campaign load recipients failed", "campaign_id", campaign.ID, "error", err)
			}
			return
		}
		if len(batch) == 0 {
			s.finish(ctx, campaign.ID, EmailCampaignStatusCompleted, "")
			slog.Info("email campaign completed", "campaign_id", campaign.ID)
			return
		}
		for _, r := range batch {
			if ctx.Err() != nil || !s.heartbeat(ctx, campaign.ID) {
				return
			}
			sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := s.sender.SendCampaign(sendCtx, content, EmailCampaignRecipientVars{
				UserID: r.UserID, Email: r.Email, Name: r.RecipientName,
				DaysInactive: emailCampaignDaysInactive(r.LastActivityAt, s.now()),
				Balance:      strconv.FormatFloat(r.Balance, 'f', 2, 64),
			}, true)
			cancel()
			status, message := EmailCampaignRecipientSent, ""
			switch {
			case errors.Is(err, ErrEmailCampaignRecipientUnsubscribed):
				status, message = EmailCampaignRecipientSkipped, "unsubscribed"
				consecutiveFailures = 0
			case err != nil:
				status, message = EmailCampaignRecipientFailed, truncateEmailCampaignError(err.Error())
				consecutiveFailures++
			default:
				consecutiveFailures = 0
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE email_campaign_recipients SET status=$2, error=$3,
				sent_at=CASE WHEN $4 THEN NOW() ELSE sent_at END WHERE id=$1`, r.ID, status, message, status == EmailCampaignRecipientSent); err != nil {
				slog.Warn("email campaign update recipient failed", "campaign_id", campaign.ID, "error", err)
				return
			}
			if consecutiveFailures >= emailCampaignMaxConsecutiveFailures {
				s.finish(ctx, campaign.ID, EmailCampaignStatusFailed, "too many consecutive send failures: "+message)
				return
			}
			if status == EmailCampaignRecipientSkipped {
				continue
			}
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return
			}
		}
	}
}

type emailCampaignPendingRecipient struct {
	ID             int64
	UserID         int64
	Email          string
	RecipientName  string
	Balance        float64
	LastActivityAt *time.Time
}

func (s *EmailCampaignService) pendingRecipients(ctx context.Context, campaignID int64) ([]emailCampaignPendingRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, email, recipient_name, balance, last_activity_at
		FROM email_campaign_recipients WHERE campaign_id=$1 AND status='pending' ORDER BY id LIMIT $2`, campaignID, emailCampaignBatchSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []emailCampaignPendingRecipient
	for rows.Next() {
		var r emailCampaignPendingRecipient
		var last sql.NullTime
		if err := rows.Scan(&r.ID, &r.UserID, &r.Email, &r.RecipientName, &r.Balance, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			r.LastActivityAt = &last.Time
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func emailCampaignDaysInactive(last *time.Time, now time.Time) string {
	if last == nil {
		return ""
	}
	days := int(now.Sub(*last).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return strconv.Itoa(days)
}

func truncateEmailCampaignError(msg string) string {
	if r := []rune(msg); len(r) > 500 {
		return string(r[:500])
	}
	return msg
}
