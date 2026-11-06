//go:build unit

package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func profitMaxRatePtr(v float64) *float64 { return &v }

func TestProfitControlThresholdForMaxAccountRate(t *testing.T) {
	t.Run("nil cap keeps margin threshold", func(t *testing.T) {
		group := &Group{ProfitMinMargin: 0.3, ProfitSafetyBuffer: 0.05}
		require.InDelta(t, 0.15*(1-0.35), profitControlThresholdFor(group, 0.15), 1e-12)
	})

	t.Run("cap alone with zero margin caps threshold", func(t *testing.T) {
		group := &Group{ProfitMaxAccountRate: profitMaxRatePtr(0.1)}
		require.InDelta(t, 0.1, profitControlThresholdFor(group, 0.15), 1e-12)
	})

	t.Run("stricter of margin threshold and cap wins", func(t *testing.T) {
		group := &Group{ProfitMinMargin: 0.5, ProfitMaxAccountRate: profitMaxRatePtr(0.1)}
		require.InDelta(t, 0.075, profitControlThresholdFor(group, 0.15), 1e-12)
		group.ProfitMinMargin = 0.1
		require.InDelta(t, 0.1, profitControlThresholdFor(group, 0.15), 1e-12)
	})

	t.Run("cap above downstream does not raise threshold", func(t *testing.T) {
		group := &Group{ProfitMaxAccountRate: profitMaxRatePtr(5)}
		require.InDelta(t, 0.15, profitControlThresholdFor(group, 0.15), 1e-12)
	})

	t.Run("zero cap admits only free accounts", func(t *testing.T) {
		group := &Group{ProfitMaxAccountRate: profitMaxRatePtr(0)}
		require.Zero(t, profitControlThresholdFor(group, 0.15))
	})

	t.Run("invalid cap values are ignored", func(t *testing.T) {
		for _, v := range []float64{-1, math.NaN(), math.Inf(1)} {
			group := &Group{ProfitMaxAccountRate: profitMaxRatePtr(v)}
			require.InDelta(t, 0.15, profitControlThresholdFor(group, 0.15), 1e-12)
		}
	})
}

// 用户场景：分组倍率 0.15、margin/buffer=0、最高调度倍率 0.1，
// 账号 0.1/0.08/0.12/0.06/0.07/0.08 中仅 0.12 被否决（0.1 恰好等于上限放行）。
func TestProfitMaxAccountRateGatesBothSchedulers(t *testing.T) {
	rates := []float64{0.1, 0.08, 0.12, 0.06, 0.07, 0.08}
	expectVetoed := []bool{false, false, true, false, false, false}

	check := func(t *testing.T, veto func(*Account) bool) {
		for i, rate := range rates {
			r := rate
			require.Equal(t, expectVetoed[i], veto(&Account{ID: int64(i + 1), RateMultiplier: &r}), "rate=%v", rate)
		}
	}

	t.Run("openai gate", func(t *testing.T) {
		group := profitControlTestGroup(7, 0, 0)
		group.RateMultiplier = 0.15
		group.ProfitMaxAccountRate = profitMaxRatePtr(0.1)
		groupID := group.ID
		svc := &OpenAIGatewayService{}
		gate := svc.resolveOpenAIProfitControlGate(profitControlTestCtx(group), &groupID)
		require.NotNil(t, gate)
		require.InDelta(t, 0.1, gate.threshold, 1e-12)
		ctx := context.WithValue(profitControlTestCtx(group), openAIProfitControlGateCtxKey{}, gate)
		check(t, func(a *Account) bool {
			vetoed, _ := openAIProfitControlVetoReason(ctx, a)
			return vetoed
		})
	})

	t.Run("gateway gate", func(t *testing.T) {
		group := gatewayProfitTestGroup(101, PlatformAnthropic)
		group.RateMultiplier = 0.15
		group.ProfitMaxAccountRate = profitMaxRatePtr(0.1)
		groupID := group.ID
		svc := &GatewayService{}
		ctx := svc.withGatewayProfitControlGate(gatewayProfitTestContext(group), &groupID)
		gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
		require.NotNil(t, gate)
		require.InDelta(t, 0.1, gate.threshold, 1e-12)
		check(t, func(a *Account) bool {
			return !svc.isGatewayAccountProfitEligible(ctx, a)
		})
	})
}

func TestNormalizeAndValidateProfitMaxAccountRate(t *testing.T) {
	require.Nil(t, NormalizeProfitMaxAccountRate(PlatformOpenAI, nil))
	require.Nil(t, NormalizeProfitMaxAccountRate(PlatformOpenAI, profitMaxRatePtr(-1)), "负数约定为清除")
	require.Nil(t, NormalizeProfitMaxAccountRate(PlatformComposite, profitMaxRatePtr(0.1)), "不支持平台不携带上限")

	in := profitMaxRatePtr(0.1)
	out := NormalizeProfitMaxAccountRate(PlatformAnthropic, in)
	require.NotNil(t, out)
	require.InDelta(t, 0.1, *out, 1e-12)
	require.NotSame(t, in, out)

	require.NoError(t, ValidateProfitMaxAccountRate(nil))
	require.NoError(t, ValidateProfitMaxAccountRate(profitMaxRatePtr(0)))
	require.NoError(t, ValidateProfitMaxAccountRate(profitMaxRatePtr(0.1)))
	require.Error(t, ValidateProfitMaxAccountRate(profitMaxRatePtr(math.NaN())))
	require.Error(t, ValidateProfitMaxAccountRate(profitMaxRatePtr(math.Inf(1))))
	require.Error(t, ValidateProfitMaxAccountRate(profitMaxRatePtr(-0.5)))
}

// 认证快照 L2 JSON 往返必须保留最高调度倍率，否则直连流量的门会静默丢失上限。
func TestAPIKeyAuthSnapshotKeepsProfitMaxAccountRate(t *testing.T) {
	svc := &APIKeyService{}
	apiKey := profitAuthTestAPIKey()
	apiKey.Group.ProfitMaxAccountRate = profitMaxRatePtr(0.03)

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	var restored APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &restored))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &restored)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group.ProfitMaxAccountRate)
	require.InDelta(t, 0.03, *materialized.Group.ProfitMaxAccountRate, 1e-12)

	ctx := context.WithValue(context.Background(), ctxkey.Group, materialized.Group)
	gate := (&OpenAIGatewayService{}).resolveOpenAIProfitControlGate(ctx, materialized.GroupID)
	require.NotNil(t, gate)
	// margin 阈值 0.06×0.75=0.045，上限 0.03 更严。
	require.InDelta(t, 0.03, gate.threshold, 1e-12)
}
