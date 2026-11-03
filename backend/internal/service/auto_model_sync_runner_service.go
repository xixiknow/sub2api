package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

// Extra keys for account-level auto upstream model sync.
const (
	// AutoModelSyncExtraKey stores the account-level switch ({"enabled":true}).
	AutoModelSyncExtraKey = "auto_model_sync"
	// AutoModelSyncStatusExtraKey stores the last sync outcome written by the runner.
	AutoModelSyncStatusExtraKey = "auto_model_sync_status"
)

// AutoModelSyncConfig is the account-level configuration persisted in Account.Extra.
type AutoModelSyncConfig struct {
	Enabled bool `json:"enabled"`
}

// AutoModelSyncStatus records the runner's latest attempt per account.
// A failed sync never mutates model configuration: the previous metadata
// snapshot and group allowlists are left untouched.
type AutoModelSyncStatus struct {
	LastSyncAt   string `json:"last_sync_at,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	LastNewCount int    `json:"last_new_models,omitempty"`
}

// AutoModelSyncConfigFromExtra parses the account switch from Extra.
func AutoModelSyncConfigFromExtra(extra map[string]any) AutoModelSyncConfig {
	raw, ok := extra[AutoModelSyncExtraKey]
	if !ok || raw == nil {
		return AutoModelSyncConfig{}
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return AutoModelSyncConfig{}
	}
	var cfg AutoModelSyncConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return AutoModelSyncConfig{}
	}
	return cfg
}

// AutoModelSyncStatusFromExtra parses the runner status from Extra.
func AutoModelSyncStatusFromExtra(extra map[string]any) *AutoModelSyncStatus {
	raw, ok := extra[AutoModelSyncStatusExtraKey]
	if !ok || raw == nil {
		return nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var status AutoModelSyncStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil
	}
	return &status
}

// AutoModelSyncEnabled reports whether the account opted into auto sync.
func (a *Account) AutoModelSyncEnabled() bool {
	if a == nil {
		return false
	}
	return AutoModelSyncConfigFromExtra(a.Extra).Enabled
}

// GetAutoModelSyncStatus returns the stored sync status, if any.
func (a *Account) GetAutoModelSyncStatus() *AutoModelSyncStatus {
	if a == nil {
		return nil
	}
	return AutoModelSyncStatusFromExtra(a.Extra)
}

// clampAutoModelSyncIntervalHours parses a raw setting value into a bounded
// interval, falling back to the default when missing or invalid.
func clampAutoModelSyncIntervalHours(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return AutoModelSyncIntervalHoursDefault
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return AutoModelSyncIntervalHoursDefault
	}
	return clampAutoModelSyncIntervalHoursInt(v)
}

// clampAutoModelSyncIntervalHoursInt clamps an already-parsed interval.
func clampAutoModelSyncIntervalHoursInt(v int) int {
	if v < AutoModelSyncIntervalHoursMin {
		return AutoModelSyncIntervalHoursDefault
	}
	if v > AutoModelSyncIntervalHoursMax {
		return AutoModelSyncIntervalHoursMax
	}
	return v
}

// GetAutoModelSyncIntervalHours returns the global auto-sync interval in hours.
func (s *SettingService) GetAutoModelSyncIntervalHours(ctx context.Context) int {
	if s == nil || s.settingRepo == nil {
		return AutoModelSyncIntervalHoursDefault
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAutoModelSyncIntervalHours)
	if err != nil {
		return AutoModelSyncIntervalHoursDefault
	}
	return clampAutoModelSyncIntervalHours(raw)
}

// autoModelSyncDue reports whether an account is due for another sync.
func autoModelSyncDue(status *AutoModelSyncStatus, interval time.Duration, now time.Time) bool {
	if status == nil || strings.TrimSpace(status.LastSyncAt) == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, status.LastSyncAt)
	if err != nil {
		return true
	}
	return now.Sub(last) >= interval
}

const autoModelSyncMaxWorkers = 3

// AutoModelSyncAccountRepository 提供到期扫描所需的账号列表与 Extra 写回能力。
// 窄接口，测试无需实现完整 AccountRepository。
type AutoModelSyncAccountRepository interface {
	ListAllWithFilters(ctx context.Context, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, error)
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
}

// AutoModelSyncGroupRepository 提供白名单追加所需的分组读取与更新能力。
type AutoModelSyncGroupRepository interface {
	GetByID(ctx context.Context, id int64) (*Group, error)
	Update(ctx context.Context, group *Group) error
}

// AutoModelSyncRunnerService periodically syncs upstream model catalogs for
// accounts that opted in via Extra["auto_model_sync"], then appends newly
// discovered models to the account's groups' model allowlists (add-only, never
// removes existing entries, deduplicated case-insensitively).
type AutoModelSyncRunnerService struct {
	accountRepo          AutoModelSyncAccountRepository
	groupRepo            AutoModelSyncGroupRepository
	accountTestSvc       *AccountTestService
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	cfg                  *config.Config

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once

	inFlight sync.Map // accountID -> struct{}
}

// NewAutoModelSyncRunnerService creates a new runner.
func NewAutoModelSyncRunnerService(
	accountRepo AutoModelSyncAccountRepository,
	groupRepo AutoModelSyncGroupRepository,
	accountTestSvc *AccountTestService,
	settingService *SettingService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	cfg *config.Config,
) *AutoModelSyncRunnerService {
	return &AutoModelSyncRunnerService{
		accountRepo:          accountRepo,
		groupRepo:            groupRepo,
		accountTestSvc:       accountTestSvc,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		cfg:                  cfg,
	}
}

// Start begins the cron ticker (every minute).
func (s *AutoModelSyncRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}
		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.auto_model_sync_runner", "[AutoModelSyncRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.auto_model_sync_runner", "[AutoModelSyncRunner] started (tick=every minute)")
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *AutoModelSyncRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.auto_model_sync_runner", "[AutoModelSyncRunner] cron stop timed out")
			}
		}
	})
}

func (s *AutoModelSyncRunnerService) runScheduled() {
	// Delay so execution lands mid-minute, away from the other runners' :10.
	time.Sleep(30 * time.Second)

	if s == nil || s.accountRepo == nil || s.accountTestSvc == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	interval := time.Duration(s.settingService.GetAutoModelSyncIntervalHours(ctx)) * time.Hour
	now := time.Now()

	accounts, err := s.accountRepo.ListAllWithFilters(ctx, "", "", "", "", 0, "")
	if err != nil {
		logger.LegacyPrintf("service.auto_model_sync_runner", "[AutoModelSyncRunner] ListAllWithFilters error: %v", err)
		return
	}

	due := make([]*Account, 0)
	for i := range accounts {
		account := &accounts[i]
		if !account.AutoModelSyncEnabled() {
			continue
		}
		if !autoModelSyncDue(account.GetAutoModelSyncStatus(), interval, now) {
			continue
		}
		due = append(due, account)
	}
	if len(due) == 0 {
		return
	}

	logger.LegacyPrintf("service.auto_model_sync_runner", "[AutoModelSyncRunner] %d accounts due for model sync", len(due))

	sem := make(chan struct{}, autoModelSyncMaxWorkers)
	var wg sync.WaitGroup
	for _, account := range due {
		if _, loaded := s.inFlight.LoadOrStore(account.ID, struct{}{}); loaded {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(acc *Account) {
			defer wg.Done()
			defer func() { <-sem }()
			defer s.inFlight.Delete(acc.ID)
			s.syncOne(ctx, acc)
		}(account)
	}
	wg.Wait()
}

// syncOne refreshes one account's upstream model catalog and merges new model
// IDs into its groups' allowlists. Failures only update the recorded status.
func (s *AutoModelSyncRunnerService) syncOne(ctx context.Context, account *Account) {
	catalog, err := s.accountTestSvc.SyncUpstreamModelCatalog(ctx, account)

	status := AutoModelSyncStatus{LastSyncAt: time.Now().UTC().Format(time.RFC3339)}
	if err != nil {
		status.LastError = sanitizeAutoModelSyncError(err)
		s.writeStatus(ctx, account.ID, status)
		slog.Warn("auto_model_sync_failed",
			"account_id", account.ID,
			"error", err,
		)
		return
	}
	status.LastNewCount = s.mergeGroupAllowlists(ctx, account, catalog)
	s.writeStatus(ctx, account.ID, status)
	slog.Info("auto_model_sync_completed",
		"account_id", account.ID,
		"models", len(catalog.Models),
		"new_allowlist_models", status.LastNewCount,
	)
}

// sanitizeAutoModelSyncError keeps only the safe client-facing message; the
// full error is already logged server-side.
func sanitizeAutoModelSyncError(err error) string {
	message := err.Error()
	if syncErr, ok := err.(*UpstreamModelSyncError); ok && syncErr != nil {
		return syncErr.SafeMessage()
	}
	if len(message) > 300 {
		return message[:300]
	}
	return message
}

func (s *AutoModelSyncRunnerService) writeStatus(ctx context.Context, accountID int64, status AutoModelSyncStatus) {
	if s.accountRepo == nil {
		return
	}
	if err := s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{AutoModelSyncStatusExtraKey: status}); err != nil {
		slog.Warn("auto_model_sync_status_write_failed", "account_id", accountID, "error", err)
	}
}

// mergeGroupAllowlists appends newly discovered model IDs to every enabled
// allowlist of the account's groups. Existing entries are never removed and
// duplicates (case-insensitive) are never introduced. Returns the number of
// appended model IDs across all groups.
func (s *AutoModelSyncRunnerService) mergeGroupAllowlists(ctx context.Context, account *Account, catalog *UpstreamModelCatalog) int {
	if s.groupRepo == nil || catalog == nil || len(catalog.Models) == 0 {
		return 0
	}
	totalAppended := 0
	for _, groupID := range account.GroupIDs {
		group, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil || group == nil {
			continue
		}
		appended := appendModelsToGroupAllowlist(group, catalog.Models)
		if appended == 0 {
			continue
		}
		// normalizeGroupModelAllowlist rejects enabled-with-empty lists, but our
		// merge only ever runs on an existing non-empty list, so it cannot fail.
		normalized, normErr := normalizeGroupModelAllowlist(group.ModelAllowlist)
		if normErr != nil {
			slog.Warn("auto_model_sync_allowlist_normalize_failed", "account_id", account.ID, "group_id", groupID, "error", normErr)
			continue
		}
		group.ModelAllowlist = normalized
		if err := s.groupRepo.Update(ctx, group); err != nil {
			slog.Warn("auto_model_sync_allowlist_update_failed", "account_id", account.ID, "group_id", groupID, "error", err)
			continue
		}
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
		}
		totalAppended += appended
		slog.Info("auto_model_sync_allowlist_appended",
			"account_id", account.ID,
			"group_id", groupID,
			"appended", appended,
		)
	}
	return totalAppended
}

// appendModelsToGroupAllowlist adds model IDs missing from the group's enabled
// allowlist, preserving existing order and existing spellings. It returns the
// number of added entries; 0 means nothing changed (no persistence needed).
func appendModelsToGroupAllowlist(group *Group, modelIDs []string) int {
	if group == nil || !group.ModelAllowlistEnabled() || len(modelIDs) == 0 {
		return 0
	}
	existing := make(map[string]struct{}, len(group.ModelAllowlist.Models))
	for _, entry := range group.ModelAllowlist.Models {
		existing[strings.ToLower(strings.TrimSpace(entry))] = struct{}{}
	}
	appended := 0
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		if _, exists := existing[strings.ToLower(modelID)]; exists {
			continue
		}
		existing[strings.ToLower(modelID)] = struct{}{}
		group.ModelAllowlist.Models = append(group.ModelAllowlist.Models, modelID)
		appended++
	}
	return appended
}
