package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// --- stubs ---

type autoModelSyncAccountRepoStub struct {
	mu       sync.Mutex
	accounts []Account
	listErr  error
	extraLog map[int64][]map[string]any
	credLog  map[int64][]map[string]any
}

func (s *autoModelSyncAccountRepoStub) UpdateCredentials(_ context.Context, id int64, credentials map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.credLog == nil {
		s.credLog = make(map[int64][]map[string]any)
	}
	s.credLog[id] = append(s.credLog[id], credentials)
	return nil
}

func (s *autoModelSyncAccountRepoStub) ListAllWithFilters(_ context.Context, _, _, _, _ string, _ int64, _ string) ([]Account, error) {
	return s.accounts, s.listErr
}

func (s *autoModelSyncAccountRepoStub) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.extraLog == nil {
		s.extraLog = make(map[int64][]map[string]any)
	}
	s.extraLog[id] = append(s.extraLog[id], updates)
	return nil
}

type autoModelSyncGroupRepoStub struct {
	mu        sync.Mutex
	groups    map[int64]*Group
	updates   []*Group
	updateErr error
}

func (s *autoModelSyncGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	if group, ok := s.groups[id]; ok {
		return group, nil
	}
	return nil, errors.New("group not found")
}

func (s *autoModelSyncGroupRepoStub) Update(_ context.Context, group *Group) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, group)
	return nil
}

type autoModelSyncAuthCacheInvalidatorStub struct {
	mu       sync.Mutex
	groupIDs []int64
}

func (s *autoModelSyncAuthCacheInvalidatorStub) InvalidateAuthCacheByKey(_ context.Context, _ string) {
}
func (s *autoModelSyncAuthCacheInvalidatorStub) InvalidateAuthCacheByUserID(_ context.Context, _ int64) {
}
func (s *autoModelSyncAuthCacheInvalidatorStub) InvalidateAuthCacheByGroupID(_ context.Context, groupID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.groupIDs = append(s.groupIDs, groupID)
}

func newAutoModelSyncTestService(
	accountRepo *autoModelSyncAccountRepoStub,
	groupRepo *autoModelSyncGroupRepoStub,
	upstream *httpUpstreamRecorder,
	invalidator *autoModelSyncAuthCacheInvalidatorStub,
) *AutoModelSyncRunnerService {
	accountTestSvc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}
	return &AutoModelSyncRunnerService{
		accountRepo:          accountRepo,
		groupRepo:            groupRepo,
		accountTestSvc:       accountTestSvc,
		settingService:       &SettingService{},
		authCacheInvalidator: invalidator,
		cfg:                  &config.Config{},
	}
}

// --- interval helpers ---

func TestClampAutoModelSyncIntervalHours(t *testing.T) {
	t.Parallel()

	require.Equal(t, 24, clampAutoModelSyncIntervalHours(""))
	require.Equal(t, 24, clampAutoModelSyncIntervalHours("abc"))
	require.Equal(t, 24, clampAutoModelSyncIntervalHours("-5"))
	require.Equal(t, 6, clampAutoModelSyncIntervalHours("6"))
	require.Equal(t, 24, clampAutoModelSyncIntervalHoursInt(0))
	require.Equal(t, 720, clampAutoModelSyncIntervalHoursInt(100000))
	require.Equal(t, 48, clampAutoModelSyncIntervalHoursInt(48))
}

func TestAutoModelSyncDue(t *testing.T) {
	t.Parallel()

	now := time.Now()
	require.True(t, autoModelSyncDue(nil, time.Hour, now))
	require.True(t, autoModelSyncDue(&AutoModelSyncStatus{}, time.Hour, now))
	require.True(t, autoModelSyncDue(&AutoModelSyncStatus{LastSyncAt: "not-a-time"}, time.Hour, now))
	require.False(t, autoModelSyncDue(&AutoModelSyncStatus{
		LastSyncAt: now.Add(-30 * time.Minute).UTC().Format(time.RFC3339),
	}, time.Hour, now))
	require.True(t, autoModelSyncDue(&AutoModelSyncStatus{
		LastSyncAt: now.Add(-2 * time.Hour).UTC().Format(time.RFC3339),
	}, time.Hour, now))
}

// --- allowlist merge ---

func TestAppendModelsToGroupAllowlist(t *testing.T) {
	t.Parallel()

	t.Run("disabled allowlist is untouched", func(t *testing.T) {
		t.Parallel()
		group := &Group{ModelAllowlist: GroupModelAllowlist{Enabled: false, Models: []string{"a"}}}
		require.Equal(t, 0, appendModelsToGroupAllowlist(group, []string{"b"}))
		require.Equal(t, []string{"a"}, group.ModelAllowlist.Models)
	})

	t.Run("appends missing entries case-insensitively preserving order", func(t *testing.T) {
		t.Parallel()
		group := &Group{ModelAllowlist: GroupModelAllowlist{
			Enabled: true,
			Models:  []string{"Claude-Sonnet-4-5", "gpt-5", "*"},
		}}
		appended := appendModelsToGroupAllowlist(group, []string{
			"claude-sonnet-4-5", // duplicate (case differs)
			"GPT-5",             // duplicate (case differs)
			"gemini-2.5-pro",
			"", // ignored
		})
		require.Equal(t, 1, appended)
		require.Equal(t, []string{"Claude-Sonnet-4-5", "gpt-5", "*", "gemini-2.5-pro"}, group.ModelAllowlist.Models)
	})

	t.Run("empty catalog is no-op", func(t *testing.T) {
		t.Parallel()
		group := &Group{ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"a"}}}
		require.Equal(t, 0, appendModelsToGroupAllowlist(group, nil))
		require.Equal(t, []string{"a"}, group.ModelAllowlist.Models)
	})
}

// --- syncOne end to end (upstream stubbed via httpUpstreamRecorder) ---

func autoModelSyncTestAccount() *Account {
	return &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			AutoModelSyncExtraKey: map[string]any{"enabled": true},
		},
		GroupIDs: []int64{7},
		Credentials: map[string]any{
			"api_key":  "key",
			"base_url": "https://provider.example/v1",
		},
	}
}

func TestSyncOneSuccessAppendsNewModelsAndWritesStatus(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"claude-sonnet-4-5"},{"id":"gpt-5"}]}`)),
	}}
	accountRepo := &autoModelSyncAccountRepoStub{}
	groupRepo := &autoModelSyncGroupRepoStub{groups: map[int64]*Group{
		7: {
			ID:             7,
			Platform:       PlatformAnthropic,
			ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"claude-sonnet-4-5"}},
		},
	}}
	invalidator := &autoModelSyncAuthCacheInvalidatorStub{}
	runner := newAutoModelSyncTestService(accountRepo, groupRepo, upstream, invalidator)

	runner.syncOne(context.Background(), autoModelSyncTestAccount())

	// Whitelist: add-only, deduplicated, existing entries untouched.
	require.Len(t, groupRepo.updates, 1)
	require.Equal(t, []string{"claude-sonnet-4-5", "gpt-5"}, groupRepo.updates[0].ModelAllowlist.Models)
	require.Equal(t, []int64{7}, invalidator.groupIDs)

	// Status: success recorded, new-model count reported.
	updates := accountRepo.extraLog[42]
	require.Len(t, updates, 1)
	status, ok := updates[0][AutoModelSyncStatusExtraKey].(AutoModelSyncStatus)
	require.True(t, ok)
	require.Empty(t, status.LastError)
	require.Equal(t, 1, status.LastNewCount)
	require.NotEmpty(t, status.LastSyncAt)
}

func TestSyncOneFailureKeepsAllowlistAndRecordsError(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
	}}
	accountRepo := &autoModelSyncAccountRepoStub{}
	groupRepo := &autoModelSyncGroupRepoStub{groups: map[int64]*Group{
		7: {
			ID:             7,
			Platform:       PlatformAnthropic,
			ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"claude-sonnet-4-5"}},
		},
	}}
	invalidator := &autoModelSyncAuthCacheInvalidatorStub{}
	runner := newAutoModelSyncTestService(accountRepo, groupRepo, upstream, invalidator)

	runner.syncOne(context.Background(), autoModelSyncTestAccount())

	// Failure must never touch model configuration.
	require.Empty(t, groupRepo.updates)
	require.Empty(t, invalidator.groupIDs)

	updates := accountRepo.extraLog[42]
	require.Len(t, updates, 1)
	status, ok := updates[0][AutoModelSyncStatusExtraKey].(AutoModelSyncStatus)
	require.True(t, ok)
	require.NotEmpty(t, status.LastError)
	require.Zero(t, status.LastNewCount)
}

func TestSyncOneSkipsDisabledAllowlistGroups(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5"}]}`)),
	}}
	accountRepo := &autoModelSyncAccountRepoStub{}
	groupRepo := &autoModelSyncGroupRepoStub{groups: map[int64]*Group{
		7: {ID: 7, Platform: PlatformOpenAI, ModelAllowlist: GroupModelAllowlist{Enabled: false}},
	}}
	runner := newAutoModelSyncTestService(accountRepo, groupRepo, upstream, nil)

	runner.syncOne(context.Background(), autoModelSyncTestAccount())

	require.Empty(t, groupRepo.updates)
	updates := accountRepo.extraLog[42]
	require.Len(t, updates, 1)
}

// --- account model_mapping merge ---

func autoModelSyncOKUpstream(body string) *httpUpstreamRecorder {
	return &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}}
}

func autoModelSyncLastStatus(t *testing.T, repo *autoModelSyncAccountRepoStub, id int64) AutoModelSyncStatus {
	t.Helper()
	updates := repo.extraLog[id]
	require.NotEmpty(t, updates)
	status, ok := updates[len(updates)-1][AutoModelSyncStatusExtraKey].(AutoModelSyncStatus)
	require.True(t, ok)
	return status
}

func TestAppendModelsToAccountModelMapping(t *testing.T) {
	creds := map[string]any{"model_mapping": map[string]any{"GPT-5": "gpt-5", "alias": "gpt-4o"}}
	merged, n := appendModelsToAccountModelMapping(creds, []string{"gpt-5", " ", "gpt-*", "gpt-4o", "gpt-4o"})
	require.Equal(t, 1, n)
	require.Equal(t, map[string]any{"GPT-5": "gpt-5", "alias": "gpt-4o", "gpt-4o": "gpt-4o"}, merged)
	// Source map must not be mutated.
	require.Len(t, creds["model_mapping"].(map[string]any), 2)

	_, n = appendModelsToAccountModelMapping(map[string]any{}, []string{"gpt-5"})
	require.Zero(t, n)
	_, n = appendModelsToAccountModelMapping(map[string]any{"model_mapping": map[string]any{}}, []string{"gpt-5"})
	require.Zero(t, n)
}

func TestSyncOneAppendsToAccountWhitelistMapping(t *testing.T) {
	upstream := autoModelSyncOKUpstream(`{"data":[{"id":"gpt-5"},{"id":"gpt-5.1"},{"id":"o3"}]}`)
	accountRepo := &autoModelSyncAccountRepoStub{}
	groupRepo := &autoModelSyncGroupRepoStub{groups: map[int64]*Group{}}
	invalidator := &autoModelSyncAuthCacheInvalidatorStub{}
	runner := newAutoModelSyncTestService(accountRepo, groupRepo, upstream, invalidator)

	account := autoModelSyncTestAccount()
	account.Credentials["model_mapping"] = map[string]any{"gpt-5": "gpt-5", "fast": "o3"}
	runner.syncOne(context.Background(), account)

	require.Len(t, accountRepo.credLog[42], 1)
	written := accountRepo.credLog[42][0]
	require.Equal(t, "key", written["api_key"])
	require.Equal(t, map[string]any{
		"gpt-5":   "gpt-5",
		"fast":    "o3", // rename preserved
		"gpt-5.1": "gpt-5.1",
		"o3":      "o3",
	}, written["model_mapping"])
	require.Equal(t, []int64{7}, invalidator.groupIDs)
	require.Equal(t, 2, autoModelSyncLastStatus(t, accountRepo, 42).LastNewAccountModels)
}

func TestSyncOneLeavesEmptyAccountMappingAlone(t *testing.T) {
	upstream := autoModelSyncOKUpstream(`{"data":[{"id":"gpt-5"}]}`)
	accountRepo := &autoModelSyncAccountRepoStub{}
	runner := newAutoModelSyncTestService(accountRepo, &autoModelSyncGroupRepoStub{}, upstream, &autoModelSyncAuthCacheInvalidatorStub{})

	runner.syncOne(context.Background(), autoModelSyncTestAccount())

	require.Empty(t, accountRepo.credLog)
	require.Zero(t, autoModelSyncLastStatus(t, accountRepo, 42).LastNewAccountModels)
}

func TestSyncOneSkipsAccountMappingInPassthrough(t *testing.T) {
	upstream := autoModelSyncOKUpstream(`{"data":[{"id":"gpt-5.1"}]}`)
	accountRepo := &autoModelSyncAccountRepoStub{}
	runner := newAutoModelSyncTestService(accountRepo, &autoModelSyncGroupRepoStub{}, upstream, &autoModelSyncAuthCacheInvalidatorStub{})

	account := autoModelSyncTestAccount()
	account.Extra["openai_passthrough"] = true
	account.Credentials["model_mapping"] = map[string]any{"gpt-5": "gpt-5"}
	runner.syncOne(context.Background(), account)

	require.Empty(t, accountRepo.credLog)
}

func TestSyncOneFailureKeepsAccountMapping(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
	}}
	accountRepo := &autoModelSyncAccountRepoStub{}
	runner := newAutoModelSyncTestService(accountRepo, &autoModelSyncGroupRepoStub{}, upstream, &autoModelSyncAuthCacheInvalidatorStub{})

	account := autoModelSyncTestAccount()
	account.Credentials["model_mapping"] = map[string]any{"gpt-5": "gpt-5"}
	runner.syncOne(context.Background(), account)

	require.Empty(t, accountRepo.credLog)
	require.NotEmpty(t, autoModelSyncLastStatus(t, accountRepo, 42).LastError)
}

func TestMergeAccountModelMappingSkipsConfiguredFallback(t *testing.T) {
	accountRepo := &autoModelSyncAccountRepoStub{}
	runner := newAutoModelSyncTestService(accountRepo, &autoModelSyncGroupRepoStub{}, nil, &autoModelSyncAuthCacheInvalidatorStub{})

	account := autoModelSyncTestAccount()
	account.Credentials["model_mapping"] = map[string]any{"gpt-5": "gpt-5"}
	fallback := &UpstreamModelCatalog{Models: []string{"gpt-5", "gpt-5.1"}, liveList: false}

	require.Zero(t, runner.mergeAccountModelMapping(context.Background(), account, fallback))
	require.Empty(t, accountRepo.credLog)

	live := &UpstreamModelCatalog{Models: []string{"gpt-5", "gpt-5.1"}, liveList: true}
	require.Equal(t, 1, runner.mergeAccountModelMapping(context.Background(), account, live))
	require.Len(t, accountRepo.credLog[42], 1)
}
