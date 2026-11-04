package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUpstreamDeclaredRateSnapshot(t *testing.T) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	t.Run("records declared base rate", func(t *testing.T) {
		account := upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.35, now.Add(-time.Hour), time.Minute)
		rate := upstreamDeclaredRateSnapshot(account, now)
		require.NotNil(t, rate)
		require.InDelta(t, 0.35, *rate, 1e-9)
	})

	t.Run("stale snapshot still recorded", func(t *testing.T) {
		account := upstreamCostTestAccount(1, UpstreamBillingProbeStatusFailed, 0.5, now.Add(-30*24*time.Hour), time.Minute)
		rate := upstreamDeclaredRateSnapshot(account, now)
		require.NotNil(t, rate)
		require.InDelta(t, 0.5, *rate, 1e-9)
	})

	t.Run("applies peak multiplier at request time", func(t *testing.T) {
		account := upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.4, now, time.Minute)
		data := account.Extra[UpstreamBillingProbeExtraKey].(map[string]any)["data"].(map[string]any)
		data["peak_rate_enabled"] = true
		data["peak_start"] = "09:00"
		data["peak_end"] = "12:00"
		data["timezone"] = "UTC"
		data["peak_rate_multiplier"] = 2.0

		inPeak := upstreamDeclaredRateSnapshot(account, now)
		require.NotNil(t, inPeak)
		require.InDelta(t, 0.8, *inPeak, 1e-9)

		offPeak := upstreamDeclaredRateSnapshot(account, now.Add(5*time.Hour))
		require.NotNil(t, offPeak)
		require.InDelta(t, 0.4, *offPeak, 1e-9)
	})

	t.Run("nil without probe data", func(t *testing.T) {
		require.Nil(t, upstreamDeclaredRateSnapshot(nil, now))
		require.Nil(t, upstreamDeclaredRateSnapshot(upstreamCostTestOAuthAccount(2), now))
		require.Nil(t, upstreamDeclaredRateSnapshot(&Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, now))
	})
}
