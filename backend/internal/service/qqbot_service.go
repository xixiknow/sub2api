package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot"
	"github.com/lib/pq"
)

const (
	SettingKeyQQBotEnabled   = "qqbot_enabled"
	SettingKeyQQBotAppID     = "qqbot_app_id"
	SettingKeyQQBotAppSecret = "qqbot_app_secret"
)

var (
	ErrQQBotDisabled       = infraerrors.NotFound("QQBOT_DISABLED", "QQ bot is not enabled")
	ErrQQBotBadSignature   = infraerrors.Unauthorized("QQBOT_BAD_SIGNATURE", "invalid QQ bot signature")
	ErrQQBotBadPayload     = infraerrors.BadRequest("QQBOT_BAD_PAYLOAD", "invalid QQ bot payload")
	ErrQQBotBindingMissing = infraerrors.NotFound("QQBOT_BINDING_NOT_FOUND", "QQ binding not found")
)

// QQBotConfig is the admin configuration. AppSecret is never returned to clients.
type QQBotConfig struct {
	Enabled   bool
	AppID     string
	AppSecret string
}

// QQBotConfigView is the admin-facing view (secret masked).
type QQBotConfigView struct {
	Enabled          bool   `json:"enabled"`
	AppID            string `json:"app_id"`
	SecretConfigured bool   `json:"secret_configured"`
}

// QQBotConfigUpdate updates the config; empty AppSecret keeps the stored one.
type QQBotConfigUpdate struct {
	Enabled   bool   `json:"enabled"`
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
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

	clientMu  sync.Mutex
	clientKey string
	client    *qqbot.Client

	dedup   *ttlSet
	limiter *ttlCounter
	images  *imageCache
	sem     chan struct{}
	now     func() time.Time
	// send is replaced in tests.
	send func(ctx context.Context, cfg QQBotConfig, m *qqbot.Message, r botReply)
}

func NewQQBotService(
	db *sql.DB,
	settingRepo SettingRepository,
	settings *SettingService,
	userRepo UserRepository,
	groupRepo GroupRepository,
	checkin *CheckinService,
	monitor *ChannelMonitorService,
) *QQBotService {
	s := &QQBotService{
		db: db, settingRepo: settingRepo, settings: settings, userRepo: userRepo,
		groupRepo: groupRepo, checkin: checkin, monitor: monitor,
		dedup:   newTTLSet(10 * time.Minute),
		limiter: newTTLCounter(time.Minute),
		images:  &imageCache{},
		sem:     make(chan struct{}, 8),
		now:     time.Now,
	}
	s.send = s.sendReply
	return s
}

// ---- config ----

func (s *QQBotService) loadConfig(ctx context.Context) (QQBotConfig, error) {
	v, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyQQBotEnabled, SettingKeyQQBotAppID, SettingKeyQQBotAppSecret})
	if err != nil {
		return QQBotConfig{}, err
	}
	return QQBotConfig{
		Enabled:   v[SettingKeyQQBotEnabled] == "true",
		AppID:     strings.TrimSpace(v[SettingKeyQQBotAppID]),
		AppSecret: strings.TrimSpace(v[SettingKeyQQBotAppSecret]),
	}, nil
}

func (c QQBotConfig) ready() bool { return c.Enabled && c.AppID != "" && c.AppSecret != "" }

func (s *QQBotService) GetConfig(ctx context.Context) (*QQBotConfigView, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &QQBotConfigView{Enabled: cfg.Enabled, AppID: cfg.AppID, SecretConfigured: cfg.AppSecret != ""}, nil
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
		SettingKeyQQBotEnabled: map[bool]string{true: "true", false: "false"}[in.Enabled],
		SettingKeyQQBotAppID:   appID,
	}
	if secret != "" {
		updates[SettingKeyQQBotAppSecret] = secret
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, err
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &QQBotConfigView{Enabled: cfg.Enabled, AppID: cfg.AppID, SecretConfigured: cfg.AppSecret != ""}, nil
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
	if err != nil || !ok || msg.UserOpenID == "" || msg.ReplyTarget() == "" {
		return ack, nil
	}
	select {
	case s.sem <- struct{}{}:
	default:
		logger.LegacyPrintf("service.qqbot", "[QQBot] busy, dropping message %s", msg.MsgID)
		return ack, nil
	}
	go func() {
		defer func() { <-s.sem }()
		defer func() {
			if r := recover(); r != nil {
				logger.LegacyPrintf("service.qqbot", "[QQBot] panic handling message: %v", r)
			}
		}()
		bg, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		s.send(bg, cfg, msg, s.HandleCommand(bg, cfg, msg))
	}()
	return ack, nil
}

// botReply is either text, or an image with a text fallback.
type botReply struct {
	Text  string
	Image []byte
}

func (s *QQBotService) sendReply(ctx context.Context, cfg QQBotConfig, m *qqbot.Message, r botReply) {
	client := s.clientFor(cfg)
	seq := 1
	if len(r.Image) > 0 {
		fileInfo, err := client.UploadImage(ctx, m, r.Image)
		if err == nil {
			if err = client.SendReply(ctx, m, seq, qqbot.Reply{FileInfo: fileInfo}); err == nil {
				return
			}
		}
		logger.LegacyPrintf("service.qqbot", "[QQBot] image reply failed, falling back to text: %v", err)
		seq++
	}
	if r.Text == "" {
		return
	}
	if err := client.SendReply(ctx, m, seq, qqbot.Reply{Text: r.Text}); err != nil {
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

// checkinCaptchaHash is the synthetic per-day fingerprint for bot check-ins:
// the QQ binding replaces the web captcha, and the daily (user, date) limit still applies.
func qqbotCheckinHash(userID int64, openid string, day time.Time) string {
	sum := sha256.Sum256([]byte("qqbot:" + day.Format("2006-01-02") + ":" + openid + ":" + strconv.FormatInt(userID, 10)))
	return hex.EncodeToString(sum[:])
}
