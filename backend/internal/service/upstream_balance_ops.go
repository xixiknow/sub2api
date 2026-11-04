package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sms"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	SettingKeyUpstreamBalanceAlertConfig = "upstream_balance_alert_config"

	upstreamBalanceExhaustedReasonPrefix = "upstream_balance_exhausted:"
	upstreamBalanceSnapshotRetention     = 30 * 24 * time.Hour

	UpstreamBalanceAlertStatusSent   = "sent"
	UpstreamBalanceAlertStatusFailed = "failed"
	UpstreamBalanceAlertLevelTest    = "test"

	// 阿里云单个模板变量最长 35 个字符。
	upstreamBalanceSMSParamMaxLen = 35
)

var (
	ErrUpstreamBalanceAlertConfigInvalid = infraerrors.BadRequest("UPSTREAM_BALANCE_ALERT_CONFIG_INVALID", "invalid upstream balance alert config")
	ErrUpstreamBalanceSMSNotConfigured   = infraerrors.BadRequest("UPSTREAM_BALANCE_SMS_NOT_CONFIGURED", "sms provider or phone numbers are not configured")
	ErrUpstreamBalanceRunnerBusy         = infraerrors.Conflict("UPSTREAM_BALANCE_RUNNER_BUSY", "upstream balance refresh is already running")
	// 加密密钥变更（如未配置 totp.encryption_key 时每次启动随机生成）后旧密文无法解密，需要重新填写。
	ErrUpstreamBalanceSMSSecretUnreadable = infraerrors.BadRequest("UPSTREAM_BALANCE_SMS_SECRET_UNREADABLE", "stored sms secret cannot be decrypted, please re-enter it")

	upstreamBalanceQuietTimeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

// UpstreamBalanceSMSConfig 是短信服务商配置。AccessKeySecret 只在请求中出现，存储时加密到 AccessKeySecretEnc。
type UpstreamBalanceSMSConfig struct {
	Provider           string `json:"provider"`
	AccessKeyID        string `json:"access_key_id"`
	AccessKeySecret    string `json:"access_key_secret,omitempty"`
	AccessKeySecretEnc string `json:"access_key_secret_enc,omitempty"`
	AccessKeySecretSet bool   `json:"access_key_secret_set"`
	SignName           string `json:"sign_name"`
	TemplateID         string `json:"template_id"`
	SdkAppID           string `json:"sdk_app_id,omitempty"`
	Region             string `json:"region,omitempty"`
	Endpoint           string `json:"endpoint,omitempty"`
}

// UpstreamBalanceAlertConfig 是上游余额维护的全局配置。
type UpstreamBalanceAlertConfig struct {
	Enabled                bool                     `json:"enabled"`
	WarnDays               float64                  `json:"warn_days"`
	CriticalDays           float64                  `json:"critical_days"`
	MinBalance             float64                  `json:"min_balance"`
	TargetDays             float64                  `json:"target_days"`
	AutoPauseExhausted     bool                     `json:"auto_pause_exhausted"`
	RefreshIntervalMinutes int                      `json:"refresh_interval_minutes"`
	Phones                 []string                 `json:"phones"`
	DailyLimit             int                      `json:"daily_limit"`
	QuietStart             string                   `json:"quiet_start"`
	QuietEnd               string                   `json:"quiet_end"`
	SMS                    UpstreamBalanceSMSConfig `json:"sms"`
}

func defaultUpstreamBalanceAlertConfig() UpstreamBalanceAlertConfig {
	return UpstreamBalanceAlertConfig{
		WarnDays:               3,
		CriticalDays:           1,
		MinBalance:             10,
		TargetDays:             7,
		AutoPauseExhausted:     true,
		RefreshIntervalMinutes: int(upstreamBalanceRunnerInterval / time.Minute),
		DailyLimit:             10,
		SMS:                    UpstreamBalanceSMSConfig{Provider: sms.ProviderAliyun},
	}
}

func (c UpstreamBalanceAlertConfig) thresholds() UpstreamBalanceThresholds {
	return UpstreamBalanceThresholds{WarnDays: c.WarnDays, CriticalDays: c.CriticalDays, MinBalance: c.MinBalance, TargetDays: c.TargetDays}
}

// UpstreamBalanceAlert 是一条短信提醒记录。
type UpstreamBalanceAlert struct {
	ID            int64             `json:"id"`
	Level         string            `json:"level"`
	AccountLevels map[string]string `json:"account_levels"`
	Content       string            `json:"content"`
	Provider      string            `json:"provider"`
	Phones        string            `json:"phones"`
	Status        string            `json:"status"`
	Error         string            `json:"error,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
}

// UpstreamBalanceHistory 是单个账号的余额曲线与每日成本。
type UpstreamBalanceHistory struct {
	AccountID int64                  `json:"account_id"`
	Points    []UpstreamBalancePoint `json:"points"`
	DailyCost []UpstreamCostBucket   `json:"daily_cost"`
}

// UpstreamBalanceOverviewFilter 过滤总览中的钱包。
type UpstreamBalanceOverviewFilter struct {
	Platform string
	Status   string
	GroupID  int64
	Search   string
}

// UpstreamBalanceOpsRepository 持久化余额历史与提醒记录。
type UpstreamBalanceOpsRepository interface {
	InsertSnapshot(ctx context.Context, accountID int64, result *UpstreamBalanceResult) error
	ListSnapshots(ctx context.Context, accountIDs []int64, since time.Time) (map[int64][]UpstreamBalancePoint, error)
	PruneSnapshots(ctx context.Context, before time.Time) (int64, error)
	GetAccountUpstreamCost(ctx context.Context, accountIDs []int64, since time.Time) (map[int64]float64, error)
	GetAccountDailyUpstreamCost(ctx context.Context, accountIDs []int64, since time.Time, timezone string) ([]UpstreamCostBucket, error)
	InsertAlert(ctx context.Context, alert *UpstreamBalanceAlert) error
	ListAlerts(ctx context.Context, since time.Time, limit int) ([]UpstreamBalanceAlert, error)
}

// UpstreamBalanceOpsService 负责余额历史、消耗预测、充值优先级、短信提醒与耗尽停调度。
type UpstreamBalanceOpsService struct {
	repo        UpstreamBalanceOpsRepository
	accountRepo AccountRepository
	settingRepo SettingRepository
	encryptor   SecretEncryptor
	balance     *UpstreamBalanceService
	newSMS      func(sms.Config) (sms.Provider, error)
	now         func() time.Time
}

func NewUpstreamBalanceOpsService(
	repo UpstreamBalanceOpsRepository,
	accountRepo AccountRepository,
	settingRepo SettingRepository,
	encryptor SecretEncryptor,
	balance *UpstreamBalanceService,
) *UpstreamBalanceOpsService {
	svc := &UpstreamBalanceOpsService{
		repo:        repo,
		accountRepo: accountRepo,
		settingRepo: settingRepo,
		encryptor:   encryptor,
		balance:     balance,
		newSMS:      sms.New,
		now:         time.Now,
	}
	if balance != nil {
		balance.ops = svc
	}
	return svc
}

// ---- 配置 ----

func (s *UpstreamBalanceOpsService) loadConfig(ctx context.Context) (UpstreamBalanceAlertConfig, error) {
	cfg := defaultUpstreamBalanceAlertConfig()
	if s.settingRepo == nil {
		return cfg, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyUpstreamBalanceAlertConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return cfg, nil
		}
		return cfg, err
	}
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultUpstreamBalanceAlertConfig(), fmt.Errorf("decode upstream balance alert config: %w", err)
	}
	return cfg, nil
}

// GetConfig 返回脱敏后的配置。
func (s *UpstreamBalanceOpsService) GetConfig(ctx context.Context) (*UpstreamBalanceAlertConfig, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	out := sanitizeUpstreamBalanceAlertConfig(cfg)
	return &out, nil
}

// UpdateConfig 校验并保存配置。AccessKeySecret 为空表示保持原值。
func (s *UpstreamBalanceOpsService) UpdateConfig(ctx context.Context, input UpstreamBalanceAlertConfig) (*UpstreamBalanceAlertConfig, error) {
	current, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	next, err := normalizeUpstreamBalanceAlertConfig(input)
	if err != nil {
		return nil, err
	}
	next.SMS.AccessKeySecretEnc = current.SMS.AccessKeySecretEnc
	if secret := strings.TrimSpace(input.SMS.AccessKeySecret); secret != "" {
		if s.encryptor == nil {
			return nil, ErrUpstreamBalanceAlertConfigInvalid.WithMetadata(map[string]string{"field": "sms.access_key_secret"})
		}
		enc, err := s.encryptor.Encrypt(secret)
		if err != nil {
			return nil, fmt.Errorf("encrypt sms secret: %w", err)
		}
		next.SMS.AccessKeySecretEnc = enc
	}
	next.SMS.AccessKeySecret = ""
	next.SMS.AccessKeySecretSet = false
	data, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyUpstreamBalanceAlertConfig, string(data)); err != nil {
		return nil, err
	}
	out := sanitizeUpstreamBalanceAlertConfig(next)
	return &out, nil
}

func sanitizeUpstreamBalanceAlertConfig(cfg UpstreamBalanceAlertConfig) UpstreamBalanceAlertConfig {
	cfg.SMS.AccessKeySecretSet = cfg.SMS.AccessKeySecretEnc != ""
	cfg.SMS.AccessKeySecretEnc = ""
	cfg.SMS.AccessKeySecret = ""
	if cfg.Phones == nil {
		cfg.Phones = []string{}
	}
	return cfg
}

func normalizeUpstreamBalanceAlertConfig(in UpstreamBalanceAlertConfig) (UpstreamBalanceAlertConfig, error) {
	invalid := func(field string) (UpstreamBalanceAlertConfig, error) {
		return in, ErrUpstreamBalanceAlertConfigInvalid.WithMetadata(map[string]string{"field": field})
	}
	if in.CriticalDays < 0 || in.WarnDays < in.CriticalDays || in.WarnDays > 365 {
		return invalid("warn_days")
	}
	if in.MinBalance < 0 {
		return invalid("min_balance")
	}
	if in.TargetDays <= 0 || in.TargetDays > 365 {
		return invalid("target_days")
	}
	if in.RefreshIntervalMinutes == 0 {
		in.RefreshIntervalMinutes = int(upstreamBalanceRunnerInterval / time.Minute)
	}
	if in.RefreshIntervalMinutes < 10 || in.RefreshIntervalMinutes > 1440 {
		return invalid("refresh_interval_minutes")
	}
	if in.DailyLimit < 0 || in.DailyLimit > 1000 {
		return invalid("daily_limit")
	}
	if (in.QuietStart == "") != (in.QuietEnd == "") ||
		(in.QuietStart != "" && (!upstreamBalanceQuietTimeRe.MatchString(in.QuietStart) || !upstreamBalanceQuietTimeRe.MatchString(in.QuietEnd))) {
		return invalid("quiet_hours")
	}
	phones := make([]string, 0, len(in.Phones))
	seen := map[string]struct{}{}
	for _, phone := range in.Phones {
		phone = strings.ReplaceAll(strings.TrimSpace(phone), " ", "")
		if phone == "" {
			continue
		}
		if !isPlausiblePhone(phone) {
			return invalid("phones")
		}
		if _, ok := seen[phone]; ok {
			continue
		}
		seen[phone] = struct{}{}
		phones = append(phones, phone)
	}
	if len(phones) > 20 {
		return invalid("phones")
	}
	in.Phones = phones
	in.SMS.Provider = strings.TrimSpace(in.SMS.Provider)
	if in.SMS.Provider != sms.ProviderAliyun && in.SMS.Provider != sms.ProviderTencent {
		return invalid("sms.provider")
	}
	in.SMS.AccessKeyID = strings.TrimSpace(in.SMS.AccessKeyID)
	in.SMS.SignName = strings.TrimSpace(in.SMS.SignName)
	in.SMS.TemplateID = strings.TrimSpace(in.SMS.TemplateID)
	in.SMS.SdkAppID = strings.TrimSpace(in.SMS.SdkAppID)
	in.SMS.Region = strings.TrimSpace(in.SMS.Region)
	in.SMS.Endpoint = strings.TrimSpace(in.SMS.Endpoint)
	if in.SMS.Endpoint != "" && !strings.HasPrefix(in.SMS.Endpoint, "https://") && !strings.HasPrefix(in.SMS.Endpoint, "http://127.0.0.1") && !strings.HasPrefix(in.SMS.Endpoint, "http://localhost") {
		return invalid("sms.endpoint")
	}
	return in, nil
}

func isPlausiblePhone(phone string) bool {
	digits := strings.TrimPrefix(phone, "+")
	if len(digits) < 6 || len(digits) > 16 {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// UpdateAccountOverride 设置单个账号的阈值覆盖；全部为空时清除。
func (s *UpstreamBalanceOpsService) UpdateAccountOverride(ctx context.Context, accountID int64, override UpstreamBalanceAlertOverride) error {
	for _, v := range []*float64{override.WarnDays, override.CriticalDays, override.MinBalance} {
		if v != nil && (*v < 0 || *v > 1e9) {
			return ErrUpstreamBalanceAlertConfigInvalid
		}
	}
	if _, err := s.accountRepo.GetByID(ctx, accountID); err != nil {
		return err
	}
	var value any
	if !override.isEmpty() {
		value = override
	}
	return s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{UpstreamBalanceAlertExtraKey: value})
}

// refreshInterval 供 runner 读取刷新间隔。
func (s *UpstreamBalanceOpsService) refreshInterval(ctx context.Context) time.Duration {
	cfg, err := s.loadConfig(ctx)
	if err != nil || cfg.RefreshIntervalMinutes <= 0 {
		return upstreamBalanceRunnerInterval
	}
	return time.Duration(cfg.RefreshIntervalMinutes) * time.Minute
}

// ---- 快照 ----

// recordSnapshot 在每次查询后写入历史；余额恢复时顺带解除本服务的停调度。
func (s *UpstreamBalanceOpsService) recordSnapshot(ctx context.Context, account *Account, result *UpstreamBalanceResult) {
	if result == nil || result.Status != UpstreamBalanceStatusOK || (result.Balance == nil && !result.Unlimited) {
		return
	}
	if err := s.repo.InsertSnapshot(ctx, account.ID, result); err != nil {
		slog.Warn("upstream_balance_snapshot_insert_failed", "account_id", account.ID, "error", err)
	}
	if (result.Unlimited || (result.Balance != nil && *result.Balance > 0)) && upstreamBalancePausedByUs(account, s.now()) {
		if err := s.accountRepo.ClearTempUnschedulable(ctx, account.ID); err != nil {
			slog.Warn("upstream_balance_resume_failed", "account_id", account.ID, "error", err)
		}
	}
}

// ---- 总览 ----

func (s *UpstreamBalanceOpsService) listRelayAccounts(ctx context.Context) ([]*Account, error) {
	// 仓储的 active 过滤会排除临时停调度账号，这里按账号状态自行过滤。
	accounts, err := s.accountRepo.ListAllWithFilters(ctx, "", AccountTypeAPIKey, "", "", 0, "")
	if err != nil {
		return nil, err
	}
	out := make([]*Account, 0, len(accounts))
	for i := range accounts {
		if accounts[i].Status == StatusActive && IsUpstreamBalanceAccount(&accounts[i]) {
			out = append(out, &accounts[i])
		}
	}
	return out, nil
}

func (s *UpstreamBalanceOpsService) buildWallets(ctx context.Context, accounts []*Account, cfg UpstreamBalanceAlertConfig) ([]UpstreamWallet, error) {
	now := s.now()
	ids := make([]int64, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}
	snapshots, err := s.repo.ListSnapshots(ctx, ids, now.Add(-7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	cost24h, err := s.repo.GetAccountUpstreamCost(ctx, ids, now.Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	cost7d, err := s.repo.GetAccountUpstreamCost(ctx, ids, now.Add(-7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	return buildUpstreamWallets(upstreamForecastInput{
		Accounts:   accounts,
		Snapshots:  snapshots,
		Cost24h:    cost24h,
		Cost7d:     cost7d,
		Thresholds: cfg.thresholds(),
		Now:        now,
	}), nil
}

// Overview 返回 KPI 与按充值优先级排序的钱包列表。
func (s *UpstreamBalanceOpsService) Overview(ctx context.Context, filter UpstreamBalanceOverviewFilter) (*UpstreamBalanceOverview, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := s.listRelayAccounts(ctx)
	if err != nil {
		return nil, err
	}
	wallets, err := s.buildWallets(ctx, accounts, cfg)
	if err != nil {
		return nil, err
	}
	filtered := make([]UpstreamWallet, 0, len(wallets))
	for _, w := range wallets {
		if upstreamWalletMatches(w, filter) {
			filtered = append(filtered, w)
		}
	}
	return &UpstreamBalanceOverview{
		Summary:     summarizeUpstreamWallets(filtered),
		Wallets:     filtered,
		Thresholds:  cfg.thresholds(),
		GeneratedAt: s.now().UTC(),
	}, nil
}

func upstreamWalletMatches(w UpstreamWallet, f UpstreamBalanceOverviewFilter) bool {
	if f.Status != "" {
		if f.Status == "alerting" {
			if upstreamAlertLevelRank(w.Status) == 0 {
				return false
			}
		} else if w.Status != f.Status {
			return false
		}
	}
	search := strings.ToLower(strings.TrimSpace(f.Search))
	if f.Platform == "" && f.GroupID == 0 && search == "" {
		return true
	}
	if search != "" && strings.Contains(w.Site, search) && f.Platform == "" && f.GroupID == 0 {
		return true
	}
	for _, m := range w.Members {
		if f.Platform != "" && m.Platform != f.Platform {
			continue
		}
		if f.GroupID > 0 && !containsInt64(m.GroupIDs, f.GroupID) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(m.Name), search) && !strings.Contains(w.Site, search) {
			continue
		}
		return true
	}
	return false
}

// History 返回账号近 days 天的余额曲线与每日本平台上游成本。
func (s *UpstreamBalanceOpsService) History(ctx context.Context, accountID int64, days int, tz string) (*UpstreamBalanceHistory, error) {
	if days <= 0 || days > 30 {
		days = 7
	}
	if strings.TrimSpace(tz) == "" {
		tz = timezone.Name()
	}
	if _, err := time.LoadLocation(tz); err != nil {
		tz = "UTC"
	}
	since := s.now().Add(-time.Duration(days) * 24 * time.Hour)
	points, err := s.repo.ListSnapshots(ctx, []int64{accountID}, since)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.GetAccountDailyUpstreamCost(ctx, []int64{accountID}, since, tz)
	if err != nil {
		return nil, err
	}
	out := &UpstreamBalanceHistory{AccountID: accountID, Points: points[accountID], DailyCost: daily}
	if out.Points == nil {
		out.Points = []UpstreamBalancePoint{}
	}
	if out.DailyCost == nil {
		out.DailyCost = []UpstreamCostBucket{}
	}
	return out, nil
}

// ListAlerts 返回近 days 天的提醒记录。
func (s *UpstreamBalanceOpsService) ListAlerts(ctx context.Context, days int) ([]UpstreamBalanceAlert, error) {
	if days <= 0 || days > 90 {
		days = 30
	}
	alerts, err := s.repo.ListAlerts(ctx, s.now().Add(-time.Duration(days)*24*time.Hour), 500)
	if err != nil {
		return nil, err
	}
	if alerts == nil {
		alerts = []UpstreamBalanceAlert{}
	}
	return alerts, nil
}

// RefreshAll 立即刷新全部中转账号余额并执行一次提醒评估。
func (s *UpstreamBalanceOpsService) RefreshAll(ctx context.Context) error {
	if s.balance == nil {
		return ErrUpstreamBalanceUnavailable
	}
	if !s.balance.runOnce(ctx) {
		return ErrUpstreamBalanceRunnerBusy
	}
	return nil
}

// ---- runner 评估：停调度与短信 ----

// evaluate 在 runner 刷新完成后执行（已持有 leader 锁）。
func (s *UpstreamBalanceOpsService) evaluate(ctx context.Context) {
	if n, err := s.repo.PruneSnapshots(ctx, s.now().Add(-upstreamBalanceSnapshotRetention)); err != nil {
		slog.Warn("upstream_balance_snapshot_prune_failed", "error", err)
	} else if n > 0 {
		slog.Info("upstream_balance_snapshot_pruned", "rows", n)
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		slog.Warn("upstream_balance_alert_config_load_failed", "error", err)
		return
	}
	accounts, err := s.listRelayAccounts(ctx)
	if err != nil {
		slog.Warn("upstream_balance_evaluate_list_failed", "error", err)
		return
	}
	wallets, err := s.buildWallets(ctx, accounts, cfg)
	if err != nil {
		slog.Warn("upstream_balance_evaluate_build_failed", "error", err)
		return
	}
	byID := make(map[int64]*Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID] = a
	}
	s.applyExhaustedPause(ctx, cfg, wallets, byID)
	if err := s.sendAlerts(ctx, cfg, wallets); err != nil {
		slog.Warn("upstream_balance_alert_failed", "error", err)
	}
}

// applyExhaustedPause 余额耗尽时临时停调度，余额恢复后解除（仅本服务写入的停调度）。
func (s *UpstreamBalanceOpsService) applyExhaustedPause(ctx context.Context, cfg UpstreamBalanceAlertConfig, wallets []UpstreamWallet, accounts map[int64]*Account) {
	until := s.now().Add(2 * time.Duration(cfg.RefreshIntervalMinutes) * time.Minute)
	for _, w := range wallets {
		if w.Stale {
			continue
		}
		for _, m := range w.Members {
			account := accounts[m.AccountID]
			if account == nil {
				continue
			}
			switch {
			case w.Status == UpstreamWalletStatusExhausted && cfg.AutoPauseExhausted:
				// 已被其他来源停调时不覆盖其 reason；本服务的停调度则续期。
				if !account.IsSchedulable() && !upstreamBalancePausedByUs(account, s.now()) {
					continue
				}
				reason := fmt.Sprintf("%s上游余额 %s 已耗尽", upstreamBalanceExhaustedReasonPrefix, formatUpstreamAmount(w.Balance))
				if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
					slog.Warn("upstream_balance_pause_failed", "account_id", account.ID, "error", err)
				}
			case w.Status != UpstreamWalletStatusExhausted && w.Status != UpstreamWalletStatusUnknown && upstreamBalancePausedByUs(account, s.now()):
				if err := s.accountRepo.ClearTempUnschedulable(ctx, account.ID); err != nil {
					slog.Warn("upstream_balance_resume_failed", "account_id", account.ID, "error", err)
				}
			}
		}
	}
}

func (s *UpstreamBalanceOpsService) smsProvider(cfg UpstreamBalanceAlertConfig) (sms.Provider, error) {
	if len(cfg.Phones) == 0 || cfg.SMS.TemplateID == "" || cfg.SMS.AccessKeySecretEnc == "" || s.encryptor == nil {
		return nil, ErrUpstreamBalanceSMSNotConfigured
	}
	secret, err := s.encryptor.Decrypt(cfg.SMS.AccessKeySecretEnc)
	if err != nil {
		slog.Warn("upstream_balance_sms_secret_decrypt_failed", "error", err)
		return nil, ErrUpstreamBalanceSMSSecretUnreadable
	}
	provider, err := s.newSMS(sms.Config{
		Provider:        cfg.SMS.Provider,
		AccessKeyID:     cfg.SMS.AccessKeyID,
		AccessKeySecret: secret,
		SignName:        cfg.SMS.SignName,
		SdkAppID:        cfg.SMS.SdkAppID,
		Region:          cfg.SMS.Region,
		Endpoint:        cfg.SMS.Endpoint,
	})
	if errors.Is(err, sms.ErrNotConfigured) {
		return nil, ErrUpstreamBalanceSMSNotConfigured
	}
	return provider, err
}

// sendAlerts 把告急钱包合并成一条汇总短信。同一钱包同一级别 24 小时内只发一次，级别升高立即再发。
func (s *UpstreamBalanceOpsService) sendAlerts(ctx context.Context, cfg UpstreamBalanceAlertConfig, wallets []UpstreamWallet) error {
	if !cfg.Enabled {
		return nil
	}
	now := s.now()
	quiet := inUpstreamQuietHours(now.In(timezone.Location()), cfg.QuietStart, cfg.QuietEnd)
	alerting := make([]UpstreamWallet, 0)
	for _, w := range wallets {
		if w.Stale || w.AlertDisabled || upstreamAlertLevelRank(w.Status) == 0 {
			continue
		}
		if quiet && w.Status != UpstreamWalletStatusExhausted {
			continue
		}
		alerting = append(alerting, w)
	}
	if len(alerting) == 0 {
		return nil
	}
	recent, err := s.repo.ListAlerts(ctx, now.Add(-24*time.Hour), 500)
	if err != nil {
		return err
	}
	if !upstreamAlertsNeedSend(alerting, recent) {
		return nil
	}
	startOfDay := timezone.StartOfDay(now.In(timezone.Location()))
	sentToday := 0
	for _, a := range recent {
		if a.Status == UpstreamBalanceAlertStatusSent && a.Level != UpstreamBalanceAlertLevelTest && !a.CreatedAt.Before(startOfDay) {
			sentToday++
		}
	}
	if cfg.DailyLimit > 0 && sentToday >= cfg.DailyLimit {
		slog.Warn("upstream_balance_alert_daily_limit_reached", "sent_today", sentToday, "limit", cfg.DailyLimit)
		return nil
	}
	provider, err := s.smsProvider(cfg)
	if err != nil {
		return err
	}
	params := upstreamAlertSMSParams(alerting)
	levels := make(map[string]string, len(alerting))
	level := UpstreamWalletStatusWarn
	for _, w := range alerting {
		levels[strconv.FormatInt(w.PrimaryAccountID, 10)] = w.Status
		if upstreamAlertLevelRank(w.Status) > upstreamAlertLevelRank(level) {
			level = w.Status
		}
	}
	return s.deliver(ctx, provider, cfg, level, levels, params)
}

func (s *UpstreamBalanceOpsService) deliver(ctx context.Context, provider sms.Provider, cfg UpstreamBalanceAlertConfig, level string, levels map[string]string, params []sms.Param) error {
	sendErr := provider.Send(ctx, sms.Message{Phones: cfg.Phones, TemplateID: cfg.SMS.TemplateID, Params: params})
	alert := &UpstreamBalanceAlert{
		Level:         level,
		AccountLevels: levels,
		Content:       renderUpstreamAlertContent(params),
		Provider:      provider.Name(),
		Phones:        maskPhones(cfg.Phones),
		Status:        UpstreamBalanceAlertStatusSent,
	}
	if sendErr != nil {
		alert.Status = UpstreamBalanceAlertStatusFailed
		alert.Error = truncateRunes(sendErr.Error(), 500)
	}
	if err := s.repo.InsertAlert(ctx, alert); err != nil {
		slog.Warn("upstream_balance_alert_record_failed", "error", err)
	}
	return sendErr
}

// SendTestSMS 用示例数据发一条测试短信。
func (s *UpstreamBalanceOpsService) SendTestSMS(ctx context.Context) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	provider, err := s.smsProvider(cfg)
	if err != nil {
		return err
	}
	params := []sms.Param{{Name: "count", Value: "1"}, {Name: "name", Value: "测试上游"}, {Name: "balance", Value: "9.99"}, {Name: "days", Value: "0.5"}}
	if err := s.deliver(ctx, provider, cfg, UpstreamBalanceAlertLevelTest, map[string]string{}, params); err != nil {
		return infraerrors.BadRequest("UPSTREAM_BALANCE_SMS_SEND_FAILED", truncateRunes(err.Error(), 300))
	}
	return nil
}

// upstreamAlertsNeedSend 判断是否有钱包的级别高于其 24 小时内已发送的级别。
func upstreamAlertsNeedSend(alerting []UpstreamWallet, recent []UpstreamBalanceAlert) bool {
	sent := make(map[string]int)
	for _, a := range recent {
		if a.Status != UpstreamBalanceAlertStatusSent {
			continue
		}
		for key, level := range a.AccountLevels {
			if rank := upstreamAlertLevelRank(level); rank > sent[key] {
				sent[key] = rank
			}
		}
	}
	for _, w := range alerting {
		if upstreamAlertLevelRank(w.Status) > sent[strconv.FormatInt(w.PrimaryAccountID, 10)] {
			return true
		}
	}
	return false
}

// upstreamAlertSMSParams 生成模板变量：count、name、balance、days（腾讯云按此顺序）。
func upstreamAlertSMSParams(alerting []UpstreamWallet) []sms.Param {
	top := alerting[0]
	name := top.Site
	if len(top.Members) > 0 {
		name = top.Members[0].Name
		if len(top.Members) > 1 {
			name = fmt.Sprintf("%s等%d个", truncateRunes(name, 24), len(top.Members))
		}
	}
	days := "-"
	if top.DaysLeft != nil {
		days = strconv.FormatFloat(*top.DaysLeft, 'f', 1, 64)
	}
	return []sms.Param{
		{Name: "count", Value: strconv.Itoa(len(alerting))},
		{Name: "name", Value: truncateRunes(name, upstreamBalanceSMSParamMaxLen)},
		{Name: "balance", Value: formatUpstreamAmount(top.Balance)},
		{Name: "days", Value: days},
	}
}

func renderUpstreamAlertContent(params []sms.Param) string {
	values := map[string]string{}
	for _, p := range params {
		values[p.Name] = p.Value
	}
	return fmt.Sprintf("%s个上游余额告急，最紧急：%s，余额%s，约%s天用完", values["count"], values["name"], values["balance"], values["days"])
}

func formatUpstreamAmount(v *float64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// inUpstreamQuietHours 判断是否处于静默时段（支持跨零点，如 23:00-08:00）。
func inUpstreamQuietHours(now time.Time, start, end string) bool {
	if start == "" || end == "" || start == end {
		return false
	}
	current := now.Format("15:04")
	if start < end {
		return current >= start && current < end
	}
	return current >= start || current < end
}

func maskPhones(phones []string) string {
	masked := make([]string, len(phones))
	for i, p := range phones {
		if len(p) > 7 {
			masked[i] = p[:3] + "****" + p[len(p)-4:]
		} else {
			masked[i] = p
		}
	}
	return strings.Join(masked, ",")
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// decodeExtraJSON 把 accounts.extra[key] 解码到 out。
func decodeExtraJSON(extra map[string]any, key string, out any) bool {
	if extra == nil {
		return false
	}
	raw, ok := extra[key]
	if !ok || raw == nil {
		return false
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, out) == nil
}
