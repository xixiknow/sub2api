package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseSub2APIUsageBalance_WalletMode(t *testing.T) {
	result, err := parseSub2APIUsageBalance([]byte(`{"mode":"unrestricted","isValid":true,"planName":"钱包余额","remaining":12.5,"unit":"USD","balance":12.5}`))
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceStatusOK, result.Status)
	require.Equal(t, UpstreamBalanceSourceSub2API, result.Source)
	require.NotNil(t, result.Balance)
	require.InDelta(t, 12.5, *result.Balance, 1e-9)
}

func TestParseSub2APIUsageBalance_QuotaLimited(t *testing.T) {
	result, err := parseSub2APIUsageBalance([]byte(`{"mode":"quota_limited","isValid":true,"quota":{"limit":100,"used":30,"remaining":70,"unit":"USD"},"remaining":70}`))
	require.NoError(t, err)
	require.InDelta(t, 70, *result.Balance, 1e-9)
	require.InDelta(t, 30, *result.Used, 1e-9)
	require.InDelta(t, 100, *result.Limit, 1e-9)
}

func TestParseSub2APIUsageBalance_RejectsForeignPayload(t *testing.T) {
	_, err := parseSub2APIUsageBalance([]byte(`{"object":"list","data":[]}`))
	require.Error(t, err)
}

func TestParseNewAPIKeyBalance(t *testing.T) {
	result, err := parseNewAPIKeyBalance(
		[]byte(`{"object":"billing_subscription","hard_limit_usd":50,"soft_limit_usd":50}`),
		[]byte(`{"object":"list","total_usage":1234.5}`),
	)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceSourceNewAPIKey, result.Source)
	require.InDelta(t, 12.345, *result.Used, 1e-9)
	require.InDelta(t, 50, *result.Limit, 1e-9)
	require.InDelta(t, 37.655, *result.Balance, 1e-9)
	require.False(t, result.Unlimited)
}

func TestParseNewAPIKeyBalance_Unlimited(t *testing.T) {
	result, err := parseNewAPIKeyBalance(
		[]byte(`{"object":"billing_subscription","hard_limit_usd":100000000}`),
		[]byte(`{"object":"list","total_usage":100}`),
	)
	require.NoError(t, err)
	require.True(t, result.Unlimited)
	require.Nil(t, result.Balance)
}

func TestUpstreamBalanceJoinURL(t *testing.T) {
	require.Equal(t, "https://relay.example.com/dashboard/billing/usage", upstreamBalanceJoinURL("https://relay.example.com/v1/", "/dashboard/billing/usage"))
	require.Equal(t, "https://relay.example.com/api/user/self", upstreamBalanceJoinURL("https://relay.example.com", "/api/user/self"))
}

func TestDecodeUpstreamBalanceSnapshot(t *testing.T) {
	balance := 3.0
	extra := map[string]any{UpstreamBalanceExtraKey: &UpstreamBalanceResult{Status: UpstreamBalanceStatusOK, Balance: &balance, FetchedAt: time.Now()}}
	snap := DecodeUpstreamBalanceSnapshot(extra)
	require.NotNil(t, snap)
	require.InDelta(t, 3, *snap.Balance, 1e-9)
	require.Nil(t, DecodeUpstreamBalanceSnapshot(map[string]any{}))
}

func TestIsUpstreamBalanceAccount(t *testing.T) {
	relay := &Account{Type: AccountTypeAPIKey, Platform: PlatformAnthropic, Credentials: map[string]any{"base_url": "https://relay.example.com", "api_key": "sk-x"}}
	require.True(t, IsUpstreamBalanceAccount(relay))
	official := &Account{Type: AccountTypeAPIKey, Platform: PlatformAnthropic, Credentials: map[string]any{"base_url": "https://api.anthropic.com", "api_key": "sk-x"}}
	require.False(t, IsUpstreamBalanceAccount(official))
	oauth := &Account{Type: AccountTypeOAuth, Platform: PlatformAnthropic}
	require.False(t, IsUpstreamBalanceAccount(oauth))
}

func TestSelectUpstreamBalanceRunnerAccounts(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	relay := func(id int64, extra map[string]any) Account {
		return Account{ID: id, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Extra: extra,
			Credentials: map[string]any{"base_url": "https://relay.example.com", "api_key": "sk"}}
	}
	accounts := []Account{
		relay(1, map[string]any{UpstreamBalanceExtraKey: &UpstreamBalanceResult{Status: UpstreamBalanceStatusOK, FetchedAt: now.Add(-time.Hour)}}),
		relay(2, nil), // never fetched → first
		relay(3, map[string]any{UpstreamBalanceExtraKey: &UpstreamBalanceResult{Status: UpstreamBalanceStatusUnsupported, FetchedAt: now.Add(-time.Hour)}}),      // skipped
		relay(4, map[string]any{UpstreamBalanceExtraKey: &UpstreamBalanceResult{Status: UpstreamBalanceStatusUnsupported, FetchedAt: now.Add(-48 * time.Hour)}}), // retried
		{ID: 5, Type: AccountTypeAPIKey, Platform: PlatformAnthropic, Credentials: map[string]any{"base_url": "https://api.anthropic.com"}},
	}
	require.Equal(t, []int64{2, 4, 1}, selectUpstreamBalanceRunnerAccounts(accounts, now, 10))
	require.Equal(t, []int64{2}, selectUpstreamBalanceRunnerAccounts(accounts, now, 1))
}

func TestProfitTotalsFinalize(t *testing.T) {
	totals := ProfitTotals{Revenue: 10, Cost: 6}
	totals.finalize()
	require.InDelta(t, 4, totals.Profit, 1e-9)
	require.InDelta(t, 0.4, totals.Margin, 1e-9)

	zero := ProfitTotals{Revenue: 0, Cost: 2}
	zero.finalize()
	require.InDelta(t, -2, zero.Profit, 1e-9)
	require.Zero(t, zero.Margin)
}

func TestValidateProfitFilter(t *testing.T) {
	now := time.Now()
	require.NoError(t, validateProfitFilter(ProfitFilter{StartTime: now.Add(-24 * time.Hour), EndTime: now}))
	require.Error(t, validateProfitFilter(ProfitFilter{StartTime: now, EndTime: now}))
	require.Error(t, validateProfitFilter(ProfitFilter{StartTime: now.Add(-200 * 24 * time.Hour), EndTime: now}))
}
