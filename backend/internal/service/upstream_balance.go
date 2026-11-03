package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	// 账号级余额快照存放在 accounts.extra，无需迁移。
	UpstreamBalanceExtraKey       = "upstream_balance"
	UpstreamBalanceSourceExtraKey = "upstream_balance_source"

	UpstreamBalanceSourceSub2API   = "sub2api"    // GET {base}/v1/usage
	UpstreamBalanceSourceNewAPIKey = "newapi_key" // GET {base}/dashboard/billing/subscription + usage

	UpstreamBalanceStatusOK          = "ok"
	UpstreamBalanceStatusUnsupported = "unsupported"
	UpstreamBalanceStatusFailed      = "failed"

	upstreamBalanceRequestTimeout = 10 * time.Second
	upstreamBalanceMaxBodyBytes   = 256 * 1024
	upstreamBalanceMaxBatchSize   = 50
	upstreamBalanceConcurrency    = 4
	// new-api 对无限额令牌把 hard_limit_usd 写成 1e8。
	upstreamBalanceUnlimitedThreshold = 100000000.0

	upstreamBalanceRunnerInterval    = 30 * time.Minute
	upstreamBalanceRunnerMaxAccounts = 200
	// 上游不支持余额端点的账号，24 小时内不再自动重试。
	upstreamBalanceUnsupportedRetry    = 24 * time.Hour
	upstreamBalanceRunnerLeaderLockKey = "upstream:balance:runner:leader"
	upstreamBalanceRunnerLeaderLockTTL = 10 * time.Minute
)

// UpstreamBalanceMaxBatchSize 限制一次手动批量查询的账号数。
const UpstreamBalanceMaxBatchSize = upstreamBalanceMaxBatchSize

var (
	ErrUpstreamBalanceUnavailable    = infraerrors.ServiceUnavailable("UPSTREAM_BALANCE_UNAVAILABLE", "upstream balance query is unavailable")
	ErrUpstreamBalanceAccountInvalid = infraerrors.BadRequest("UPSTREAM_BALANCE_ACCOUNT_INVALID", "only API key accounts with a relay base_url support upstream balance query")
)

// UpstreamBalanceResult 统一的上游余额结果。金额单位为上游额度的 USD。
type UpstreamBalanceResult struct {
	Status     string    `json:"status"`
	Source     string    `json:"source,omitempty"`
	Balance    *float64  `json:"balance,omitempty"`
	Used       *float64  `json:"used,omitempty"`
	Limit      *float64  `json:"limit,omitempty"`
	Unlimited  bool      `json:"unlimited,omitempty"`
	Currency   string    `json:"currency,omitempty"`
	FetchedAt  time.Time `json:"fetched_at"`
	HTTPStatus int       `json:"http_status,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// UpstreamBalanceAccountResult 批量查询中单个账号的结果。
type UpstreamBalanceAccountResult struct {
	AccountID int64                  `json:"account_id"`
	Result    *UpstreamBalanceResult `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// DecodeUpstreamBalanceSnapshot 从 accounts.extra 读取账号级余额快照。
func DecodeUpstreamBalanceSnapshot(extra map[string]any) *UpstreamBalanceResult {
	if extra == nil {
		return nil
	}
	raw, ok := extra[UpstreamBalanceExtraKey]
	if !ok || raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out UpstreamBalanceResult
	if err := json.Unmarshal(data, &out); err != nil || out.Status == "" {
		return nil
	}
	return &out
}

// UpstreamBalanceService 查询中转上游（sub2api、new-api/one-api）的余额。
type UpstreamBalanceService struct {
	accountRepo        AccountRepository
	accountTestService *AccountTestService

	group      singleflight.Group
	slots      chan struct{}
	now        func() time.Time
	instanceID string
	lockCache  LeaderLockCache
	db         *sql.DB
	stopCh     chan struct{}
}

func NewUpstreamBalanceService(
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
) *UpstreamBalanceService {
	return &UpstreamBalanceService{
		accountRepo:        accountRepo,
		accountTestService: accountTestService,
		slots:              make(chan struct{}, upstreamBalanceConcurrency),
		now:                time.Now,
		instanceID:         uuid.NewString(),
		stopCh:             make(chan struct{}),
	}
}

// ProvideUpstreamBalanceService 构造服务并启动周期刷新（所有中转 API Key 账号）。
func ProvideUpstreamBalanceService(
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
	lockCache LeaderLockCache,
	db *sql.DB,
) *UpstreamBalanceService {
	svc := NewUpstreamBalanceService(accountRepo, accountTestService)
	svc.lockCache = lockCache
	svc.db = db
	go svc.runLoop()
	return svc
}

func (s *UpstreamBalanceService) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// IsUpstreamBalanceAccount 判断账号是否可以按 key 查询中转上游余额。
func IsUpstreamBalanceAccount(account *Account) bool {
	if account == nil || account.Type != AccountTypeAPIKey || account.ParentAccountID != nil {
		return false
	}
	baseURL := account.GetCredential("base_url")
	return baseURL != "" && !upstreamBillingProbeTargetIsOfficialAPI(baseURL)
}

// QueryAccount 查询单个账号的上游余额并写入快照。
func (s *UpstreamBalanceService) QueryAccount(ctx context.Context, accountID int64) (*UpstreamBalanceResult, error) {
	if s == nil || s.accountRepo == nil {
		return nil, ErrUpstreamBalanceUnavailable
	}
	key := "account:" + strconv.FormatInt(accountID, 10)
	value, err, _ := s.group.Do(key, func() (any, error) {
		select {
		case s.slots <- struct{}{}:
			defer func() { <-s.slots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		account, err := s.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if !IsUpstreamBalanceAccount(account) {
			return nil, ErrUpstreamBalanceAccountInvalid
		}
		result := s.queryAccountBalance(ctx, account)
		updates := map[string]any{UpstreamBalanceExtraKey: result}
		if result.Status == UpstreamBalanceStatusOK && result.Source != "" {
			updates[UpstreamBalanceSourceExtraKey] = result.Source
		}
		if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	result, _ := value.(*UpstreamBalanceResult)
	return result, nil
}

// QueryAccounts 有界并发的批量查询。
func (s *UpstreamBalanceService) QueryAccounts(ctx context.Context, accountIDs []int64) []UpstreamBalanceAccountResult {
	if len(accountIDs) > upstreamBalanceMaxBatchSize {
		accountIDs = accountIDs[:upstreamBalanceMaxBatchSize]
	}
	results := make([]UpstreamBalanceAccountResult, len(accountIDs))
	var group errgroup.Group
	for i, accountID := range accountIDs {
		i, accountID := i, accountID
		results[i].AccountID = accountID
		group.Go(func() error {
			result, err := s.QueryAccount(ctx, accountID)
			if err != nil {
				results[i].Error = safeUpstreamBalanceError(err)
				return nil
			}
			results[i].Result = result
			return nil
		})
	}
	_ = group.Wait()
	return results
}

func (s *UpstreamBalanceService) queryAccountBalance(ctx context.Context, account *Account) *UpstreamBalanceResult {
	now := s.now().UTC()
	if s.accountTestService == nil || s.accountTestService.httpUpstream == nil {
		return upstreamBalanceFailure(now, "", 0, "transport_unavailable")
	}
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return upstreamBalanceFailure(now, "", 0, "missing_api_key")
	}
	baseURL, err := s.accountTestService.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return upstreamBalanceFailure(now, "", 0, "invalid_base_url")
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil {
			return upstreamBalanceFailure(now, "", 0, "proxy_unavailable")
		}
		proxyURL = account.Proxy.URL()
	}
	// 上次成功的来源优先，避免每次都先撞一遍不支持的端点。
	sources := []string{UpstreamBalanceSourceSub2API, UpstreamBalanceSourceNewAPIKey}
	if preferred, _ := account.Extra[UpstreamBalanceSourceExtraKey].(string); preferred == UpstreamBalanceSourceNewAPIKey {
		sources = []string{UpstreamBalanceSourceNewAPIKey, UpstreamBalanceSourceSub2API}
	}
	var last *UpstreamBalanceResult
	for _, source := range sources {
		var result *UpstreamBalanceResult
		switch source {
		case UpstreamBalanceSourceSub2API:
			result = s.querySub2API(ctx, account, baseURL, apiKey, proxyURL)
		case UpstreamBalanceSourceNewAPIKey:
			result = s.queryNewAPIKey(ctx, account, baseURL, apiKey, proxyURL)
		}
		if result.Status == UpstreamBalanceStatusOK {
			return result
		}
		// 鉴权失败对两种来源一致，没必要继续试。
		if result.HTTPStatus == http.StatusUnauthorized || result.HTTPStatus == http.StatusForbidden {
			return result
		}
		last = result
	}
	if last != nil && last.Status == UpstreamBalanceStatusFailed {
		return last
	}
	return &UpstreamBalanceResult{Status: UpstreamBalanceStatusUnsupported, FetchedAt: now, Error: "unsupported"}
}

func (s *UpstreamBalanceService) querySub2API(ctx context.Context, account *Account, baseURL, apiKey, proxyURL string) *UpstreamBalanceResult {
	now := s.now().UTC()
	body, status, err := s.doAccountGet(ctx, account, buildOpenAIEndpointURL(baseURL, "/v1/usage"), apiKey, proxyURL)
	if err != nil {
		return upstreamBalanceFailure(now, UpstreamBalanceSourceSub2API, status, err.Error())
	}
	result, err := parseSub2APIUsageBalance(body)
	if err != nil {
		return &UpstreamBalanceResult{Status: UpstreamBalanceStatusUnsupported, Source: UpstreamBalanceSourceSub2API, FetchedAt: now, HTTPStatus: status, Error: "unsupported"}
	}
	result.FetchedAt = now
	result.HTTPStatus = status
	return result
}

func (s *UpstreamBalanceService) queryNewAPIKey(ctx context.Context, account *Account, baseURL, apiKey, proxyURL string) *UpstreamBalanceResult {
	now := s.now().UTC()
	subBody, status, err := s.doAccountGet(ctx, account, upstreamBalanceJoinURL(baseURL, "/dashboard/billing/subscription"), apiKey, proxyURL)
	if err != nil {
		return upstreamBalanceFailure(now, UpstreamBalanceSourceNewAPIKey, status, err.Error())
	}
	usageBody, usageStatus, err := s.doAccountGet(ctx, account, upstreamBalanceJoinURL(baseURL, "/dashboard/billing/usage"), apiKey, proxyURL)
	if err != nil {
		return upstreamBalanceFailure(now, UpstreamBalanceSourceNewAPIKey, usageStatus, err.Error())
	}
	result, err := parseNewAPIKeyBalance(subBody, usageBody)
	if err != nil {
		return &UpstreamBalanceResult{Status: UpstreamBalanceStatusUnsupported, Source: UpstreamBalanceSourceNewAPIKey, FetchedAt: now, HTTPStatus: status, Error: "unsupported"}
	}
	result.FetchedAt = now
	result.HTTPStatus = status
	return result
}

func (s *UpstreamBalanceService) doAccountGet(ctx context.Context, account *Account, url, apiKey, proxyURL string) ([]byte, int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, upstreamBalanceRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, errors.New("request_build_failed")
	}
	profile := HTTPUpstreamProfileDefault
	if account.Platform == PlatformOpenAI {
		profile = HTTPUpstreamProfileOpenAI
	}
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(req.Context(), profile)))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	account.ApplyHeaderOverrides(req.Header)
	var tlsProfile *tlsfingerprint.Profile
	if s.accountTestService.tlsFPProfileService != nil {
		tlsProfile = s.accountTestService.tlsFPProfileService.ResolveTLSProfile(account)
	}
	return s.readResponse(s.accountTestService.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile))
}

func (s *UpstreamBalanceService) readResponse(resp *http.Response, err error) ([]byte, int, error) {
	if err != nil {
		return nil, 0, errors.New("request_failed")
	}
	if resp == nil || resp.Body == nil {
		return nil, 0, errors.New("empty_response")
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamBalanceMaxBodyBytes+1))
	if readErr != nil {
		return nil, resp.StatusCode, errors.New("response_read_failed")
	}
	if len(body) > upstreamBalanceMaxBodyBytes {
		return nil, resp.StatusCode, errors.New("response_too_large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("http_%d", resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

// parseSub2APIUsageBalance 解析 sub2api 网关 GET /v1/usage 的响应。
func parseSub2APIUsageBalance(body []byte) (*UpstreamBalanceResult, error) {
	var payload struct {
		Mode      string   `json:"mode"`
		IsValid   *bool    `json:"isValid"`
		Remaining *float64 `json:"remaining"`
		Balance   *float64 `json:"balance"`
		Unit      string   `json:"unit"`
		Quota     *struct {
			Limit     *float64 `json:"limit"`
			Used      *float64 `json:"used"`
			Remaining *float64 `json:"remaining"`
		} `json:"quota"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Mode == "" || payload.IsValid == nil {
		return nil, errors.New("not a sub2api usage response")
	}
	result := &UpstreamBalanceResult{Status: UpstreamBalanceStatusOK, Source: UpstreamBalanceSourceSub2API, Currency: "USD"}
	if payload.Unit != "" {
		result.Currency = payload.Unit
	}
	switch {
	case payload.Quota != nil && payload.Quota.Remaining != nil:
		result.Balance = payload.Quota.Remaining
		result.Used = payload.Quota.Used
		result.Limit = payload.Quota.Limit
	case payload.Balance != nil:
		result.Balance = payload.Balance
	case payload.Remaining != nil:
		result.Balance = payload.Remaining
	default:
		// 无总额度限制的 key：上游没有可读余额。
		result.Unlimited = true
	}
	return result, nil
}

// parseNewAPIKeyBalance 解析 new-api/one-api 的 OpenAI 兼容账单端点。
// subscription.hard_limit_usd = 剩余 + 已用；usage.total_usage 单位为美分。
func parseNewAPIKeyBalance(subscriptionBody, usageBody []byte) (*UpstreamBalanceResult, error) {
	var sub struct {
		Object       string   `json:"object"`
		HardLimitUSD *float64 `json:"hard_limit_usd"`
	}
	if err := json.Unmarshal(subscriptionBody, &sub); err != nil {
		return nil, err
	}
	if sub.HardLimitUSD == nil {
		return nil, errors.New("missing hard_limit_usd")
	}
	var usage struct {
		TotalUsage *float64 `json:"total_usage"`
	}
	if err := json.Unmarshal(usageBody, &usage); err != nil {
		return nil, err
	}
	if usage.TotalUsage == nil {
		return nil, errors.New("missing total_usage")
	}
	used := roundUpstreamBalance(*usage.TotalUsage / 100)
	result := &UpstreamBalanceResult{Status: UpstreamBalanceStatusOK, Source: UpstreamBalanceSourceNewAPIKey, Currency: "USD", Used: &used}
	if *sub.HardLimitUSD >= upstreamBalanceUnlimitedThreshold {
		result.Unlimited = true
		return result, nil
	}
	limit := roundUpstreamBalance(*sub.HardLimitUSD)
	balance := roundUpstreamBalance(limit - used)
	result.Limit = &limit
	result.Balance = &balance
	return result, nil
}

func (s *UpstreamBalanceService) runLoop() {
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-timer.C:
			s.runOnce(context.Background())
			timer.Reset(upstreamBalanceRunnerInterval)
		}
	}
}

func (s *UpstreamBalanceService) runOnce(ctx context.Context) {
	if s.accountRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, upstreamBalanceRunnerLeaderLockTTL)
	defer cancel()
	release, ok := s.acquireLeader(ctx)
	if !ok {
		return
	}
	defer release()

	accounts, err := s.accountRepo.ListAllWithFilters(ctx, "", AccountTypeAPIKey, StatusActive, "", 0, "")
	if err != nil {
		slog.Warn("upstream_balance_runner_list_accounts_failed", "error", err)
		return
	}
	accountIDs := selectUpstreamBalanceRunnerAccounts(accounts, s.now(), upstreamBalanceRunnerMaxAccounts)
	for start := 0; start < len(accountIDs); start += upstreamBalanceMaxBatchSize {
		end := min(start+upstreamBalanceMaxBatchSize, len(accountIDs))
		s.QueryAccounts(ctx, accountIDs[start:end])
	}
}

// selectUpstreamBalanceRunnerAccounts 选出本轮要刷新的账号：仅中转 API Key 账号，
// 近期确认不支持的跳过，最久未刷新的优先。
func selectUpstreamBalanceRunnerAccounts(accounts []Account, now time.Time, limit int) []int64 {
	type candidate struct {
		id        int64
		fetchedAt time.Time
	}
	candidates := make([]candidate, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if !IsUpstreamBalanceAccount(account) {
			continue
		}
		var fetchedAt time.Time
		if snap := DecodeUpstreamBalanceSnapshot(account.Extra); snap != nil {
			if snap.Status == UpstreamBalanceStatusUnsupported && now.Sub(snap.FetchedAt) < upstreamBalanceUnsupportedRetry {
				continue
			}
			fetchedAt = snap.FetchedAt
		}
		candidates = append(candidates, candidate{id: account.ID, fetchedAt: fetchedAt})
	}
	sort.Slice(candidates, func(a, b int) bool {
		if !candidates[a].fetchedAt.Equal(candidates[b].fetchedAt) {
			return candidates[a].fetchedAt.Before(candidates[b].fetchedAt)
		}
		return candidates[a].id < candidates[b].id
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	ids := make([]int64, len(candidates))
	for i, c := range candidates {
		ids[i] = c.id
	}
	return ids
}

func (s *UpstreamBalanceService) acquireLeader(ctx context.Context) (func(), bool) {
	if s.lockCache != nil {
		acquired, err := s.lockCache.TryAcquireLeaderLock(ctx, upstreamBalanceRunnerLeaderLockKey, s.instanceID, upstreamBalanceRunnerLeaderLockTTL)
		if err != nil || !acquired {
			return nil, false
		}
		return func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = s.lockCache.ReleaseLeaderLock(releaseCtx, upstreamBalanceRunnerLeaderLockKey, s.instanceID)
		}, true
	}
	if s.db != nil {
		release, ok, err := tryAcquireDBAdvisoryLockWithError(ctx, s.db, hashAdvisoryLockID(upstreamBalanceRunnerLeaderLockKey))
		if err != nil || !ok {
			return nil, false
		}
		return release, true
	}
	return func() {}, true
}

func upstreamBalanceFailure(now time.Time, source string, status int, reason string) *UpstreamBalanceResult {
	return &UpstreamBalanceResult{Status: UpstreamBalanceStatusFailed, Source: source, FetchedAt: now, HTTPStatus: status, Error: reason}
}

func upstreamBalanceJoinURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	// 用户常把 base_url 填成 https://host/v1，账单端点挂在根路径。
	base = strings.TrimSuffix(base, "/v1")
	return base + path
}

func roundUpstreamBalance(v float64) float64 {
	const scale = 1e6
	if v >= 0 {
		return float64(int64(v*scale+0.5)) / scale
	}
	return float64(int64(v*scale-0.5)) / scale
}

func safeUpstreamBalanceError(err error) string {
	var appErr *infraerrors.ApplicationError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "query_failed"
}
