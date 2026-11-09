package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot/render"
	"github.com/lib/pq"
)

const (
	SettingKeyQQBotEnabled   = "qqbot_enabled"
	SettingKeyQQBotAppID     = "qqbot_app_id"
	SettingKeyQQBotAppSecret = "qqbot_app_secret"

	SettingKeyQQBotPosterAPIKey     = "qqbot_poster_api_key"
	SettingKeyQQBotPosterModel      = "qqbot_poster_model"
	SettingKeyQQBotPosterDailyLimit = "qqbot_poster_daily_limit"

	qqbotPosterModelDefault      = "gpt-image-2"
	qqbotPosterDailyLimitDefault = 3
	qqbotPosterDailyLimitMax     = 50

	// Fast commands share qqbotFastSlots; a message waits up to qqbotFastWait for a slot.
	qqbotFastSlots   = 32
	qqbotFastWait    = 20 * time.Second
	qqbotFastTimeout = 45 * time.Second

	qqbotBindCodeTTL = 10 * time.Minute
	qqbotBindCodeLen = 8
)

var (
	ErrQQBotDisabled       = infraerrors.NotFound("QQBOT_DISABLED", "QQ bot is not enabled")
	ErrQQBotBadSignature   = infraerrors.Unauthorized("QQBOT_BAD_SIGNATURE", "invalid QQ bot signature")
	ErrQQBotBadPayload     = infraerrors.BadRequest("QQBOT_BAD_PAYLOAD", "invalid QQ bot payload")
	ErrQQBotBindingMissing = infraerrors.NotFound("QQBOT_BINDING_NOT_FOUND", "QQ binding not found")
	ErrQQBotAlreadyBound   = infraerrors.Conflict("QQBOT_ALREADY_BOUND", "account is already bound to a QQ; unbind first")
	ErrQQBotTooFrequent    = infraerrors.TooManyRequests("QQBOT_TOO_FREQUENT", "too many requests, please retry later")

	errQQBotBindCodeInvalid = errors.New("qqbot bind code invalid or expired")
	errQQBotBindConflict    = errors.New("qqbot binding conflict")
)

// QQBotConfig is the admin configuration. Secrets are never returned to clients.
type QQBotConfig struct {
	Enabled   bool
	AppID     string
	AppSecret string

	PosterAPIKey     string // site API key used to generate poster art through this gateway
	PosterModel      string
	PosterDailyLimit int // new AI illustrations per user per day; 0 = never generate
}

// QQBotConfigView is the admin-facing view (secrets masked).
type QQBotConfigView struct {
	Enabled          bool   `json:"enabled"`
	AppID            string `json:"app_id"`
	SecretConfigured bool   `json:"secret_configured"`

	PosterKeyConfigured bool   `json:"poster_key_configured"`
	PosterModel         string `json:"poster_model"`
	PosterDailyLimit    int    `json:"poster_daily_limit"`
}

// QQBotConfigUpdate updates the config; empty secrets keep the stored values.
type QQBotConfigUpdate struct {
	Enabled   bool   `json:"enabled"`
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`

	PosterAPIKey     string `json:"poster_api_key"`
	PosterModel      string `json:"poster_model"`
	PosterDailyLimit *int   `json:"poster_daily_limit"`
}

// QQBotBinding is a user's QQ binding as shown on the profile page.
type QQBotBinding struct {
	Enabled   bool       `json:"enabled"` // bot configured and switched on
	Bound     bool       `json:"bound"`
	OpenID    string     `json:"openid,omitempty"` // masked
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// QQBotService handles the QQ bot webhook and chat commands.
type QQBotService struct {
	db          *sql.DB
	settingRepo SettingRepository
	settings    *SettingService
	userRepo    UserRepository
	groupRepo   GroupRepository
	checkin     *CheckinService
	monitor     *ChannelMonitorService
	affiliate   *AffiliateService
	appCfg      *config.Config

	clientMu  sync.Mutex
	clientKey string
	client    *qqbot.Client

	dedup   *ttlSet
	limiter *ttlCounter
	images  *imageCache
	sem     chan struct{} // fast command slots
	poster  *posterPipeline
	now     func() time.Time

	// The hooks below are replaced in tests.
	send          func(ctx context.Context, cfg QQBotConfig, m *qqbot.Message, r botReply)
	generateArt   func(ctx context.Context, cfg QQBotConfig, prompt string) ([]byte, error)
	invitePlanner func(ctx context.Context, cfg QQBotConfig, m *qqbot.Message) (*invitePlan, string)
	renderPoster  func(in render.PosterInput) ([]byte, error)
}

func NewQQBotService(
	db *sql.DB,
	settingRepo SettingRepository,
	settings *SettingService,
	userRepo UserRepository,
	groupRepo GroupRepository,
	checkin *CheckinService,
	monitor *ChannelMonitorService,
	affiliate *AffiliateService,
	appCfg *config.Config,
) *QQBotService {
	s := &QQBotService{
		db: db, settingRepo: settingRepo, settings: settings, userRepo: userRepo,
		groupRepo: groupRepo, checkin: checkin, monitor: monitor,
		affiliate: affiliate, appCfg: appCfg,
		dedup:   newTTLSet(10 * time.Minute),
		limiter: newTTLCounter(time.Minute),
		images:  &imageCache{},
		sem:     make(chan struct{}, qqbotFastSlots),
		poster:  newPosterPipeline(),
		now:     time.Now,
	}
	s.send = s.sendReply
	s.generateArt = s.generateArtViaGateway
	s.invitePlanner = s.buildInvitePlan
	s.renderPoster = func(in render.PosterInput) ([]byte, error) { return render.RenderPosterJPEG(in, posterJPEGQuality) }
	return s
}

// ---- config ----

func (s *QQBotService) loadConfig(ctx context.Context) (QQBotConfig, error) {
	v, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyQQBotEnabled, SettingKeyQQBotAppID, SettingKeyQQBotAppSecret,
		SettingKeyQQBotPosterAPIKey, SettingKeyQQBotPosterModel, SettingKeyQQBotPosterDailyLimit,
	})
	if err != nil {
		return QQBotConfig{}, err
	}
	cfg := QQBotConfig{
		Enabled:          v[SettingKeyQQBotEnabled] == "true",
		AppID:            strings.TrimSpace(v[SettingKeyQQBotAppID]),
		AppSecret:        strings.TrimSpace(v[SettingKeyQQBotAppSecret]),
		PosterAPIKey:     strings.TrimSpace(v[SettingKeyQQBotPosterAPIKey]),
		PosterModel:      strings.TrimSpace(v[SettingKeyQQBotPosterModel]),
		PosterDailyLimit: qqbotPosterDailyLimitDefault,
	}
	if cfg.PosterModel == "" {
		cfg.PosterModel = qqbotPosterModelDefault
	}
	if raw := strings.TrimSpace(v[SettingKeyQQBotPosterDailyLimit]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 && n <= qqbotPosterDailyLimitMax {
			cfg.PosterDailyLimit = n
		}
	}
	return cfg, nil
}

func (c QQBotConfig) ready() bool { return c.Enabled && c.AppID != "" && c.AppSecret != "" }

func (c QQBotConfig) view() *QQBotConfigView {
	return &QQBotConfigView{
		Enabled: c.Enabled, AppID: c.AppID, SecretConfigured: c.AppSecret != "",
		PosterKeyConfigured: c.PosterAPIKey != "", PosterModel: c.PosterModel, PosterDailyLimit: c.PosterDailyLimit,
	}
}

func (s *QQBotService) GetConfig(ctx context.Context) (*QQBotConfigView, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.view(), nil
}

func (s *QQBotService) UpdateConfig(ctx context.Context, in QQBotConfigUpdate) (*QQBotConfigView, error) {
	appID := strings.TrimSpace(in.AppID)
	if len(appID) > 64 {
		return nil, infraerrors.BadRequest("QQBOT_BAD_APPID", "AppID is too long")
	}
	current, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	secret := strings.TrimSpace(in.AppSecret)
	if in.Enabled && (appID == "" || (secret == "" && current.AppSecret == "")) {
		return nil, infraerrors.BadRequest("QQBOT_INCOMPLETE", "AppID and AppSecret are required to enable the QQ bot")
	}
	updates := map[string]string{
		SettingKeyQQBotEnabled: strconv.FormatBool(in.Enabled),
		SettingKeyQQBotAppID:   appID,
	}
	if secret != "" {
		updates[SettingKeyQQBotAppSecret] = secret
	}
	if key := strings.TrimSpace(in.PosterAPIKey); key != "" {
		if len(key) > 512 {
			return nil, infraerrors.BadRequest("QQBOT_BAD_POSTER_KEY", "poster API key is too long")
		}
		updates[SettingKeyQQBotPosterAPIKey] = key
	}
	if model := strings.TrimSpace(in.PosterModel); model != "" {
		if len(model) > 128 {
			return nil, infraerrors.BadRequest("QQBOT_BAD_POSTER_MODEL", "poster model is too long")
		}
		updates[SettingKeyQQBotPosterModel] = model
	}
	if in.PosterDailyLimit != nil {
		if *in.PosterDailyLimit < 0 || *in.PosterDailyLimit > qqbotPosterDailyLimitMax {
			return nil, infraerrors.BadRequest("QQBOT_BAD_POSTER_LIMIT", "poster daily limit must be between 0 and 50")
		}
		updates[SettingKeyQQBotPosterDailyLimit] = strconv.Itoa(*in.PosterDailyLimit)
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, err
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.view(), nil
}

// clientFor reuses one client (and its cached access token) per credential pair.
func (s *QQBotService) clientFor(cfg QQBotConfig) *qqbot.Client {
	key := cfg.AppID + "\x00" + cfg.AppSecret
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	if s.client == nil || s.clientKey != key {
		s.client, s.clientKey = qqbot.NewClient(cfg.AppID, cfg.AppSecret), key
	}
	return s.client
}

// ---- webhook ----

// HandleWebhook verifies and handles one callback. It returns the JSON body to
// write back: the validation answer for op=13, an ACK otherwise. Commands are
// processed asynchronously because QQ expects the callback to return quickly.
func (s *QQBotService) HandleWebhook(ctx context.Context, signature, timestamp string, body []byte) (any, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.ready() {
		return nil, ErrQQBotDisabled
	}
	signer, err := qqbot.NewSigner(cfg.AppSecret)
	if err != nil {
		return nil, ErrQQBotDisabled
	}
	var p qqbot.Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, ErrQQBotBadPayload
	}
	if p.Op == qqbot.OpValidation {
		var req qqbot.ValidationRequest
		if err := json.Unmarshal(p.D, &req); err != nil || req.PlainToken == "" {
			return nil, ErrQQBotBadPayload
		}
		return qqbot.ValidationResponse{PlainToken: req.PlainToken, Signature: signer.SignValidation(req.EventTs, req.PlainToken)}, nil
	}
	if err := signer.Verify(signature, timestamp, body); err != nil {
		return nil, ErrQQBotBadSignature
	}
	ack := map[string]int{"op": qqbot.OpHTTPCallbackAck, "d": 0}
	if p.Op != qqbot.OpDispatch {
		return ack, nil
	}
	if p.ID != "" && !s.dedup.add(p.ID) {
		return ack, nil // QQ re-delivery
	}
	msg, ok, err := qqbot.ParseMessage(&p)
	if err != nil || !ok || msg.FromBot || msg.UserOpenID == "" || msg.ReplyTarget() == "" {
		return ack, nil
	}
	if !msg.Addressed && !isSlashCommand(msg.Content) {
		return ack, nil // ordinary group chatter (full-message mode)
	}
	go s.dispatch(cfg, msg)
	return ack, nil
}

// dispatch runs one command off the webhook goroutine. /邀请 goes to the poster
// pipeline (its own limits); every other command waits for a fast slot instead
// of being dropped when the bot is busy.
func (s *QQBotService) dispatch(cfg QQBotConfig, msg *qqbot.Message) {
	defer func() {
		if r := recover(); r != nil {
			logger.LegacyPrintf("service.qqbot", "[QQBot] panic handling message: %v", r)
		}
	}()
	if cmd, _ := parseQQBotCommand(msg.Content); isInviteCommand(cmd) {
		s.handleInvite(cfg, msg)
		return
	}
	wait := time.NewTimer(qqbotFastWait)
	defer wait.Stop()
	select {
	case s.sem <- struct{}{}:
	case <-wait.C:
		logger.LegacyPrintf("service.qqbot", "[QQBot] no free slot after %s, dropping message %s", qqbotFastWait, msg.MsgID)
		return
	}
	defer func() { <-s.sem }()
	ctx, cancel := context.WithTimeout(context.Background(), qqbotFastTimeout)
	defer cancel()
	if r := s.HandleCommand(ctx, cfg, msg); r.Text != "" || len(r.Image) > 0 {
		s.send(ctx, cfg, msg, r)
	}
}

// isSlashCommand reports whether a group message without @ is meant for the bot.
func isSlashCommand(content string) bool {
	return strings.HasPrefix(content, "/") || strings.HasPrefix(content, "／")
}

// botReply is either text, or an image whose Text is sent only when the image
// is missing or fails to send.
type botReply struct {
	Text  string
	Image []byte
}

// sendReply sends one passive reply; each send takes the message's next msg_seq.
func (s *QQBotService) sendReply(ctx context.Context, cfg QQBotConfig, m *qqbot.Message, r botReply) {
	client := s.clientFor(cfg)
	if len(r.Image) > 0 {
		fileInfo, err := client.UploadImage(ctx, m, r.Image)
		if err == nil {
			if err = client.SendReply(ctx, m, m.NextSeq(), qqbot.Reply{FileInfo: fileInfo}); err == nil {
				return
			}
		}
		logger.LegacyPrintf("service.qqbot", "[QQBot] image reply failed, falling back to text: %v", err)
	}
	if r.Text == "" {
		return
	}
	if err := client.SendReply(ctx, m, m.NextSeq(), qqbot.Reply{Text: r.Text}); err != nil {
		logger.LegacyPrintf("service.qqbot", "[QQBot] send reply failed: %v", err)
	}
}

// ---- bindings ----

func (s *QQBotService) bindingUser(ctx context.Context, appID, openid string) (int64, error) {
	var uid int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM qqbot_bindings WHERE app_id=$1 AND openid=$2`, appID, openid).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrQQBotBindingMissing
	}
	return uid, err
}

// GetUserBinding returns the binding of a site user (for the profile page).
func (s *QQBotService) GetUserBinding(ctx context.Context, userID int64) (*QQBotBinding, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	var openid string
	var created time.Time
	err = s.db.QueryRowContext(ctx, `SELECT openid, created_at FROM qqbot_bindings WHERE app_id=$1 AND user_id=$2`, cfg.AppID, userID).Scan(&openid, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return &QQBotBinding{Enabled: cfg.ready()}, nil
	}
	if err != nil {
		return nil, err
	}
	return &QQBotBinding{Enabled: cfg.ready(), Bound: true, OpenID: maskMiddle(openid), CreatedAt: &created}, nil
}

// QQBotBindCode is a freshly generated one-time binding code.
type QQBotBindCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateBindCode issues a one-time code the logged-in user sends to the bot as
// "/绑定 <code>". Only the hash is stored; a new code replaces older ones.
func (s *QQBotService) CreateBindCode(ctx context.Context, userID int64) (*QQBotBindCode, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.ready() {
		return nil, ErrQQBotDisabled
	}
	if s.limiter.incr("code:"+strconv.FormatInt(userID, 10)) > qqbotBindLimitPerMinute {
		return nil, ErrQQBotTooFrequent
	}
	var existing int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM qqbot_bindings WHERE app_id=$1 AND user_id=$2`, cfg.AppID, userID).Scan(&existing)
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrQQBotAlreadyBound
	}
	code, err := newQQBotBindCode()
	if err != nil {
		return nil, err
	}
	now := s.now()
	expires := now.Add(qqbotBindCodeTTL)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Drop this user's previous codes and any expired codes in one go.
	if _, err := tx.ExecContext(ctx, `DELETE FROM qqbot_bind_codes WHERE user_id=$1 OR expires_at<=$2`, userID, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO qqbot_bind_codes (app_id, user_id, code_hash, expires_at, created_at) VALUES ($1,$2,$3,$4,$5)`,
		cfg.AppID, userID, qqbotBindCodeHash(cfg.AppID, code), expires, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &QQBotBindCode{Code: code, ExpiresAt: expires}, nil
}

// consumeBindCode atomically redeems a code for openid. It returns
// errQQBotBindCodeInvalid for unknown/expired/used codes and
// errQQBotBindConflict when the account is already bound to another QQ.
func (s *QQBotService) consumeBindCode(ctx context.Context, cfg QQBotConfig, code, openid string, scene qqbot.Scene) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var uid int64
	err = tx.QueryRowContext(ctx,
		`DELETE FROM qqbot_bind_codes WHERE app_id=$1 AND code_hash=$2 AND expires_at>$3 RETURNING user_id`,
		cfg.AppID, qqbotBindCodeHash(cfg.AppID, code), s.now()).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errQQBotBindCodeInvalid
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO qqbot_bindings (app_id, openid, user_id, scene, created_at) VALUES ($1,$2,$3,$4,$5)`,
		cfg.AppID, openid, uid, string(scene), s.now()); err != nil {
		if isUniqueViolation(err) {
			// Only reachable through a race (both sides are checked before);
			// the rollback leaves the code valid until it expires.
			return 0, errQQBotBindConflict
		}
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uid, nil
}

// qqbotBindCodeAlphabet omits look-alike characters (0/O, 1/I/L).
const qqbotBindCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newQQBotBindCode() (string, error) {
	buf := make([]byte, qqbotBindCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, qqbotBindCodeLen)
	n := len(qqbotBindCodeAlphabet)
	limit := 256 - 256%n // reject values that would bias the modulo
	for i := 0; i < qqbotBindCodeLen; {
		if int(buf[i]) >= limit {
			if _, err := rand.Read(buf[i : i+1]); err != nil {
				return "", err
			}
			continue
		}
		out[i] = qqbotBindCodeAlphabet[int(buf[i])%n]
		i++
	}
	return string(out), nil
}

// normalizeQQBotBindCode accepts "abcd-2345", "ABCD 2345" etc.
func normalizeQQBotBindCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r == '-' || r == ' ' || r == '　' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func validQQBotBindCode(code string) bool {
	if len(code) != qqbotBindCodeLen {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !strings.ContainsRune(qqbotBindCodeAlphabet, rune(code[i])) {
			return false
		}
	}
	return true
}

func qqbotBindCodeHash(appID, code string) string {
	sum := sha256.Sum256([]byte("qqbot-bind:" + appID + ":" + code))
	return hex.EncodeToString(sum[:])
}

// UnbindUser removes a site user's QQ binding (profile page).
func (s *QQBotService) UnbindUser(ctx context.Context, userID int64) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM qqbot_bindings WHERE app_id=$1 AND user_id=$2`, cfg.AppID, userID)
	return err
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

// qqbotCheckinHash is the synthetic per-day fingerprint for bot check-ins:
// the QQ binding replaces the web captcha, and the daily (user, date) limit still applies.
func qqbotCheckinHash(userID int64, openid string, day time.Time) string {
	sum := sha256.Sum256([]byte("qqbot:" + day.Format("2006-01-02") + ":" + openid + ":" + strconv.FormatInt(userID, 10)))
	return hex.EncodeToString(sum[:])
}
