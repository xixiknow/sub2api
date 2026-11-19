//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func withFastMonitorBackoff(t *testing.T) {
	t.Helper()
	prev := monitorProbeRetryBackoffs
	monitorProbeRetryBackoffs = []time.Duration{0}
	t.Cleanup(func() { monitorProbeRetryBackoffs = prev })
}

func withProbeSigner(t *testing.T) {
	t.Helper()
	InitChannelMonitorProbeSigner("unit-test-secret")
	t.Cleanup(func() { InitChannelMonitorProbeSigner("") })
}



func newMonitorTestServer(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

// fakeGateway 模拟网关：按提示选号并回传账号；behaviour 决定每个账号返回什么。
type fakeGateway struct {
	t        *testing.T
	apiKey   string
	accounts []int64
	behavior map[int64]string // accountID -> status
	served   []int64
	hints    []ProbeHints
}

func (g *fakeGateway) check(_ context.Context, opts *CheckOptions) *CheckResult {
	hints, ok := VerifyProbeHeaders(opts.probeHeaders[ProbeHeaderSignature], opts.probeHeaders[ProbeHeaderExclude], opts.probeHeaders[ProbeHeaderPrefer], []string{g.apiKey}, time.Now())
	require.True(g.t, ok, "probe headers must verify")
	g.hints = append(g.hints, hints)

	excluded := map[int64]bool{}
	for _, id := range hints.Exclude {
		excluded[id] = true
	}
	pick := int64(0)
	for _, id := range g.accounts {
		if id == hints.Prefer {
			pick = id
			break
		}
	}
	if pick == 0 {
		for _, id := range g.accounts {
			if !excluded[id] {
				pick = id
				break
			}
		}
	}
	if pick == 0 { // 软排除兜底
		pick = g.accounts[0]
	}
	g.served = append(g.served, pick)
	opts.servedAccountID = pick
	status := g.behavior[pick]
	lat := 800
	if status == MonitorStatusDegraded {
		lat = 8000
	}
	return &CheckResult{Model: "m", Status: status, LatencyMs: intPtr(lat), CheckedAt: time.Now()}
}

func testPolicy() MonitorProbeRetryPolicy {
	return MonitorProbeRetryPolicy{MaxRetries: 3, Budget: 60 * time.Second}
}

func TestRunCheckWithRetry_SwitchesAccountAndRemembersBest(t *testing.T) {
	withFastMonitorBackoff(t)
	withProbeSigner(t)
	storeMonitorBestAccount(1001, "m", 0)

	g := &fakeGateway{t: t, apiKey: "sk-a", accounts: []int64{11, 12}, behavior: map[int64]string{
		11: MonitorStatusDegraded,
		12: MonitorStatusOperational,
	}}
	res := runCheckWithRetry(context.Background(), 1001, "m", "sk-a", nil, testPolicy(), g.check)

	require.Equal(t, MonitorStatusOperational, res.Status)
	require.Equal(t, []int64{11, 12}, g.served)
	require.Equal(t, []int64{11}, g.hints[1].Exclude, "slow account must be soft-excluded on retry")
	require.Contains(t, res.Message, "retried 1")
	require.Equal(t, int64(12), loadMonitorBestAccount(1001, "m"))

	// 下一轮优先探测上轮最佳账号，一次即绿。
	g.served, g.hints = nil, nil
	res = runCheckWithRetry(context.Background(), 1001, "m", "sk-a", nil, testPolicy(), g.check)
	require.Equal(t, MonitorStatusOperational, res.Status)
	require.Equal(t, []int64{12}, g.served)
	require.Equal(t, int64(12), g.hints[0].Prefer)
	require.Empty(t, res.Message)
}

func TestRunCheckWithRetry_SingleAccountRetriesSameTarget(t *testing.T) {
	withFastMonitorBackoff(t)
	calls := 0
	check := func(_ context.Context, _ *CheckOptions) *CheckResult {
		calls++
		if calls < 3 {
			return &CheckResult{Model: "m", Status: MonitorStatusError, Message: "upstream HTTP 502", LatencyMs: intPtr(100)}
		}
		return &CheckResult{Model: "m", Status: MonitorStatusOperational, LatencyMs: intPtr(900)}
	}
	res := runCheckWithRetry(context.Background(), 1002, "m", "sk-b", nil, testPolicy(), check)
	require.Equal(t, MonitorStatusOperational, res.Status)
	require.Equal(t, 3, calls)
	require.Contains(t, res.Message, "retried 2")
}

func TestRunCheckWithRetry_KeepsBestWhenAllAttemptsBad(t *testing.T) {
	withFastMonitorBackoff(t)
	seq := []*CheckResult{
		{Model: "m", Status: MonitorStatusError, LatencyMs: intPtr(100)},
		{Model: "m", Status: MonitorStatusDegraded, LatencyMs: intPtr(9000)},
		{Model: "m", Status: MonitorStatusDegraded, LatencyMs: intPtr(7000)},
		{Model: "m", Status: MonitorStatusFailed, LatencyMs: intPtr(500)},
	}
	i := 0
	check := func(_ context.Context, _ *CheckOptions) *CheckResult {
		r := seq[i]
		i++
		return r
	}
	res := runCheckWithRetry(context.Background(), 1003, "m", "", nil, testPolicy(), check)
	require.Equal(t, 4, i, "first attempt + 3 retries")
	require.Equal(t, MonitorStatusDegraded, res.Status)
	require.Equal(t, 7000, *res.LatencyMs)
}

func TestRunCheckWithRetry_ZeroRetriesRunsOnce(t *testing.T) {
	calls := 0
	check := func(_ context.Context, _ *CheckOptions) *CheckResult {
		calls++
		return &CheckResult{Model: "m", Status: MonitorStatusError}
	}
	res := runCheckWithRetry(context.Background(), 1004, "m", "", nil, MonitorProbeRetryPolicy{MaxRetries: 0, Budget: time.Minute}, check)
	require.Equal(t, 1, calls)
	require.Equal(t, MonitorStatusError, res.Status)
}

func TestRunCheckWithRetry_StopsWhenBudgetExhausted(t *testing.T) {
	prev := monitorProbeRetryBackoffs
	monitorProbeRetryBackoffs = []time.Duration{time.Second}
	t.Cleanup(func() { monitorProbeRetryBackoffs = prev })

	calls := 0
	check := func(_ context.Context, _ *CheckOptions) *CheckResult {
		calls++
		return &CheckResult{Model: "m", Status: MonitorStatusError}
	}
	// 预算不足以容纳「退避 + 最短单次超时」，不应重试。
	runCheckWithRetry(context.Background(), 1005, "m", "", nil, MonitorProbeRetryPolicy{MaxRetries: 3, Budget: 3 * time.Second}, check)
	require.Equal(t, 1, calls)
}

func TestRunCheckWithRetry_ReportsChallengeFailureToHealthTracker(t *testing.T) {
	withFastMonitorBackoff(t)
	withProbeSigner(t)
	const badID = int64(987654)
	DefaultAccountHealthTracker().ClearPenalty(badID)
	before := DefaultAccountHealthTracker().Snapshot(badID)

	g := &fakeGateway{t: t, apiKey: "sk-c", accounts: []int64{badID, badID + 1}, behavior: map[int64]string{
		badID:     MonitorStatusFailed,
		badID + 1: MonitorStatusOperational,
	}}
	storeMonitorBestAccount(1006, "m", 0)
	runCheckWithRetry(context.Background(), 1006, "m", "sk-c", nil, testPolicy(), g.check)

	after := DefaultAccountHealthTracker().Snapshot(badID)
	require.Greater(t, after.ErrorRate, before.ErrorRate)
}

func TestNormalizeMonitorProbeRetryPolicyAndParsers(t *testing.T) {
	require.Equal(t, 5, normalizeMonitorProbeRetryPolicy(99, 90).MaxRetries)
	require.Equal(t, 0, normalizeMonitorProbeRetryPolicy(-1, 90).MaxRetries)
	require.Equal(t, 90*time.Second, normalizeMonitorProbeRetryPolicy(3, 0).Budget)
	require.Equal(t, 3, parseMonitorProbeMaxRetries(""))
	require.Equal(t, 0, parseMonitorProbeMaxRetries("0"))
	require.Equal(t, 90, parseMonitorProbeBudgetSeconds("abc"))
	require.Equal(t, 30, parseMonitorProbeBudgetSeconds("5"))
	require.Equal(t, 300, parseMonitorProbeBudgetSeconds("9999"))
}

func TestProbeHeaders_SignVerify(t *testing.T) {
	withProbeSigner(t)
	now := time.Now()
	h := BuildProbeHeaders("sk-x", ProbeHints{Exclude: []int64{3, 4}, Prefer: 9}, now)
	require.NotNil(t, h)

	hints, ok := VerifyProbeHeaders(h[ProbeHeaderSignature], h[ProbeHeaderExclude], h[ProbeHeaderPrefer], []string{"other", "sk-x"}, now)
	require.True(t, ok)
	require.Equal(t, []int64{3, 4}, hints.Exclude)
	require.Equal(t, int64(9), hints.Prefer)

	// 其他 key 无法重放
	_, ok = VerifyProbeHeaders(h[ProbeHeaderSignature], h[ProbeHeaderExclude], h[ProbeHeaderPrefer], []string{"sk-attacker"}, now)
	require.False(t, ok)
	// 篡改提示
	_, ok = VerifyProbeHeaders(h[ProbeHeaderSignature], "3", h[ProbeHeaderPrefer], []string{"sk-x"}, now)
	require.False(t, ok)
	// 过期
	_, ok = VerifyProbeHeaders(h[ProbeHeaderSignature], h[ProbeHeaderExclude], h[ProbeHeaderPrefer], []string{"sk-x"}, now.Add(2*time.Minute))
	require.False(t, ok)
}

func TestProbeHeaders_DisabledWithoutSecret(t *testing.T) {
	InitChannelMonitorProbeSigner("")
	require.Nil(t, BuildProbeHeaders("sk-x", ProbeHints{Prefer: 1}, time.Now()))
	_, ok := VerifyProbeHeaders("1.abc", "", "", []string{"sk-x"}, time.Now())
	require.False(t, ok)
}

func TestProbeHeaderNamesAreForbiddenInUserHeaders(t *testing.T) {
	require.True(t, IsForbiddenHeaderName("X-Sub2API-Probe"))
	require.True(t, IsForbiddenHeaderName("x-sub2api-probe-prefer"))
	require.False(t, IsForbiddenHeaderName("X-Custom"))
	require.NoError(t, validateExtraHeaders(map[string]string{"X-Custom": "1"}))
	require.Error(t, validateExtraHeaders(map[string]string{"X-Sub2API-Probe-Exclude": "1"}))
}

func TestCallProviderSendsProbeHeadersAndReadsServedAccount(t *testing.T) {
	var gotProbe string
	srv := newMonitorTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotProbe = r.Header.Get(ProbeHeaderPrefer)
		w.Header().Set(ProbeHeaderServedAccount, "42")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}]}`))
	})
	opts := &CheckOptions{probeHeaders: map[string]string{ProbeHeaderPrefer: "7"}}
	_, _, status, err := callProvider(context.Background(), MonitorProviderOpenAI, srv, "sk", "gpt", "p", opts)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "7", gotProbe)
	require.Equal(t, int64(42), opts.servedAccountID)
}

// ---------- 网关选号装饰器 ----------

type probeRepoStub struct {
	AccountRepository
	accounts []Account
}

func (r *probeRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return r.accounts, nil
}

// fakeSelector 选第一个未被排除的账号；全部排除时返回 ErrNoAvailableAccounts。
func fakeSelector(ids []int64, calls *[]map[int64]struct{}) probeSelectFunc {
	return func(_ context.Context, excluded map[int64]struct{}) (*AccountSelectionResult, error) {
		*calls = append(*calls, excluded)
		for _, id := range ids {
			if _, ok := excluded[id]; !ok {
				return &AccountSelectionResult{Account: &Account{ID: id}}, nil
			}
		}
		return nil, ErrNoAvailableAccounts
	}
}

func probeState(h ProbeHints) *probeSelectionState {
	return &probeSelectionState{hints: h, record: &ProbeSelectionRecord{}}
}

func TestSelectWithProbeHints_PrefersAccount(t *testing.T) {
	gid := int64(5)
	repo := &probeRepoStub{accounts: []Account{{ID: 1}, {ID: 2}, {ID: 3}}}
	var calls []map[int64]struct{}
	st := probeState(ProbeHints{Prefer: 3})
	res, err := selectWithProbeHints(context.Background(), st, repo, &gid, map[int64]struct{}{}, fakeSelector([]int64{1, 2, 3}, &calls))
	require.NoError(t, err)
	require.Equal(t, int64(3), res.Account.ID)
	require.Equal(t, int64(3), st.record.ServedAccountID())
	require.Len(t, calls, 1)
}

func TestSelectWithProbeHints_SoftExcludeFallsBackForSingleAccount(t *testing.T) {
	gid := int64(5)
	repo := &probeRepoStub{accounts: []Account{{ID: 1}}}
	var calls []map[int64]struct{}
	st := probeState(ProbeHints{Exclude: []int64{1}})
	res, err := selectWithProbeHints(context.Background(), st, repo, &gid, map[int64]struct{}{}, fakeSelector([]int64{1}, &calls))
	require.NoError(t, err)
	require.Equal(t, int64(1), res.Account.ID, "soft exclusion must not cause no-available-accounts")
	require.Len(t, calls, 2)
}

func TestSelectWithProbeHints_SoftExcludePicksOther(t *testing.T) {
	gid := int64(5)
	repo := &probeRepoStub{accounts: []Account{{ID: 1}, {ID: 2}}}
	var calls []map[int64]struct{}
	st := probeState(ProbeHints{Exclude: []int64{1}})
	res, err := selectWithProbeHints(context.Background(), st, repo, &gid, map[int64]struct{}{}, fakeSelector([]int64{1, 2}, &calls))
	require.NoError(t, err)
	require.Equal(t, int64(2), res.Account.ID)
}

func TestSelectWithProbeHints_PreferAlreadyFailedIsIgnored(t *testing.T) {
	gid := int64(5)
	repo := &probeRepoStub{accounts: []Account{{ID: 1}, {ID: 2}}}
	var calls []map[int64]struct{}
	st := probeState(ProbeHints{Prefer: 2})
	// 网关 failover 已把 2 标为失败：优选不得把它拉回来。
	res, err := selectWithProbeHints(context.Background(), st, repo, &gid, map[int64]struct{}{2: {}}, fakeSelector([]int64{1, 2}, &calls))
	require.NoError(t, err)
	require.Equal(t, int64(1), res.Account.ID)
}

func TestSelectWithProbeHints_PropagatesBaseError(t *testing.T) {
	gid := int64(5)
	st := probeState(ProbeHints{})
	_, err := selectWithProbeHints(context.Background(), st, nil, &gid, nil, func(context.Context, map[int64]struct{}) (*AccountSelectionResult, error) {
		return nil, errors.New("boom")
	})
	require.EqualError(t, err, "boom")
	require.Zero(t, st.record.ServedAccountID())
}

func TestProbeSelectionStateConsumedPreventsRecursion(t *testing.T) {
	ctx, _ := WithProbeSelectionHints(context.Background(), ProbeHints{Prefer: 1})
	require.NotNil(t, probeSelectionStateFrom(ctx))
	require.Nil(t, probeSelectionStateFrom(markProbeSelectionConsumed(ctx)))
	require.True(t, strings.HasPrefix(strings.ToLower(ProbeHeaderServedAccount), probeHeaderPrefix))
}
