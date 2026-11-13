//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type settlementUsageLogStub struct {
	id          int64
	userID      int64
	cost        float64
	billingType int8
	createdAt   time.Time
}

type affiliateSettlementRepoStub struct {
	cursor       int64
	logs         []settlementUsageLogStub
	invitees     map[int64]bool
	durationArgs []int
	advanceCalls []int64
	lockedInTx   bool
}

func (r *affiliateSettlementRepoStub) LockConsumptionSettlementCursor(ctx context.Context) (int64, error) {
	r.lockedInTx = dbent.TxFromContext(ctx) != nil
	return r.cursor, nil
}

func (r *affiliateSettlementRepoStub) GetConsumptionSettlementUpperBound(_ context.Context, afterID int64, before time.Time) (int64, error) {
	upper := afterID
	for _, l := range r.logs {
		if l.id > afterID && !l.createdAt.After(before) && l.id > upper {
			upper = l.id
		}
	}
	return upper, nil
}

func (r *affiliateSettlementRepoStub) SumInviteeBalanceConsumption(_ context.Context, afterID, upperID int64, durationDays int) ([]AffiliateInviteeConsumption, error) {
	r.durationArgs = append(r.durationArgs, durationDays)
	sums := map[int64]float64{}
	order := []int64{}
	for _, l := range r.logs {
		if l.id <= afterID || l.id > upperID || l.billingType != BillingTypeBalance || l.cost <= 0 || !r.invitees[l.userID] {
			continue
		}
		if _, ok := sums[l.userID]; !ok {
			order = append(order, l.userID)
		}
		sums[l.userID] += l.cost
	}
	out := make([]AffiliateInviteeConsumption, 0, len(order))
	for _, id := range order {
		out = append(out, AffiliateInviteeConsumption{UserID: id, Amount: sums[id]})
	}
	return out, nil
}

func (r *affiliateSettlementRepoStub) AdvanceConsumptionSettlementCursor(_ context.Context, upperID int64) error {
	r.advanceCalls = append(r.advanceCalls, upperID)
	if upperID > r.cursor {
		r.cursor = upperID
	}
	return nil
}

func newAffiliateSettlementTestService(t *testing.T, settlementRepo *affiliateSettlementRepoStub, settings map[string]string, now time.Time) (*AffiliateConsumptionSettlementService, *paymentFulfillmentAffiliateRepoStub, int64) {
	t.Helper()
	inviteeID := int64(42)
	inviterID := int64(9001)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		inviteeSummary: &AffiliateSummary{UserID: inviteeID, AffCode: "INVITEE", InviterID: &inviterID, CreatedAt: now.Add(-24 * time.Hour)},
		inviterSummary: &AffiliateSummary{UserID: inviterID, AffCode: "INVITER", CreatedAt: now.Add(-48 * time.Hour)},
	}
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{values: settings}, nil)
	affiliateSvc := NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
	svc := NewAffiliateConsumptionSettlementService(newPaymentConfigServiceTestClient(t), settlementRepo, affiliateSvc, settingSvc, &config.Config{})
	svc.now = func() time.Time { return now }
	return svc, affiliateRepo, inviterID
}

func TestAffiliateConsumptionSettlement_SettlesBalanceConsumptionOnly(t *testing.T) {
	now := time.Now()
	old := now.Add(-time.Hour)
	repo := &affiliateSettlementRepoStub{
		cursor:   10,
		invitees: map[int64]bool{42: true},
		logs: []settlementUsageLogStub{
			{id: 9, userID: 42, cost: 100, billingType: BillingTypeBalance, createdAt: old},                   // 已结算
			{id: 11, userID: 42, cost: 6, billingType: BillingTypeBalance, createdAt: old},                    // 计入
			{id: 12, userID: 42, cost: 50, billingType: BillingTypeSubscription, createdAt: old},              // 订阅不计
			{id: 13, userID: 77, cost: 80, billingType: BillingTypeBalance, createdAt: old},                   // 无邀请人
			{id: 14, userID: 42, cost: 4, billingType: BillingTypeBalance, createdAt: old},                    // 计入
			{id: 15, userID: 42, cost: 30, billingType: BillingTypeBalance, createdAt: now.Add(-time.Minute)}, // 未过延迟窗口
		},
	}
	svc, affiliateRepo, inviterID := newAffiliateSettlementTestService(t, repo, map[string]string{
		SettingKeyAffiliateEnabled:            "true",
		SettingKeyAffiliateRebateRate:         "10",
		SettingKeyAffiliateRebateFreezeHours:  "0",
		SettingKeyAffiliateRebateDurationDays: "30",
	}, now)

	result, err := svc.SettleOnce(context.Background())
	require.NoError(t, err)
	require.True(t, repo.lockedInTx)
	require.Equal(t, int64(10), result.FromUsageLogID)
	require.Equal(t, int64(14), result.ToUsageLogID)
	require.Equal(t, 1, result.InviteeCount)
	require.InDelta(t, 1.0, result.TotalRebate, 1e-8)
	require.Equal(t, []int{30}, repo.durationArgs)

	require.Len(t, affiliateRepo.accrueCalls, 1)
	call := affiliateRepo.accrueCalls[0]
	require.Equal(t, inviterID, call.inviterID)
	require.Equal(t, int64(42), call.inviteeUserID)
	require.InDelta(t, 1.0, call.amount, 1e-8)
	require.Equal(t, AffiliateRebateSourceConsumption, call.sourceType)
	require.Nil(t, call.sourceOrderID)
	require.NotNil(t, call.settlementID)
	require.Equal(t, int64(14), *call.settlementID)
	require.Equal(t, 1, call.level)
	require.Equal(t, int64(14), repo.cursor)

	// 再次结算：没有新的已过延迟窗口的日志，不产生返利
	result, err = svc.SettleOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(14), result.ToUsageLogID)
	require.Len(t, affiliateRepo.accrueCalls, 1)
	require.Equal(t, []int64{14}, repo.advanceCalls)

	// 时间推进后，id=15 进入结算
	svc.now = func() time.Time { return now.Add(10 * time.Minute) }
	result, err = svc.SettleOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(15), result.ToUsageLogID)
	require.Len(t, affiliateRepo.accrueCalls, 2)
	require.InDelta(t, 3.0, affiliateRepo.accrueCalls[1].amount, 1e-8)
	require.Equal(t, int64(15), *affiliateRepo.accrueCalls[1].settlementID)
	require.Equal(t, int64(15), repo.cursor)
}

func TestAffiliateConsumptionSettlement_DisabledAdvancesCursorWithoutRebate(t *testing.T) {
	now := time.Now()
	repo := &affiliateSettlementRepoStub{
		cursor:   0,
		invitees: map[int64]bool{42: true},
		logs: []settlementUsageLogStub{
			{id: 1, userID: 42, cost: 10, billingType: BillingTypeBalance, createdAt: now.Add(-time.Hour)},
		},
	}
	svc, affiliateRepo, _ := newAffiliateSettlementTestService(t, repo, map[string]string{
		SettingKeyAffiliateEnabled:    "false",
		SettingKeyAffiliateRebateRate: "10",
	}, now)

	result, err := svc.SettleOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.InviteeCount)
	require.Empty(t, affiliateRepo.accrueCalls)
	require.Empty(t, repo.durationArgs)
	require.Equal(t, int64(1), repo.cursor)
}

func TestAffiliateConsumptionSettlement_SimpleModeSkips(t *testing.T) {
	now := time.Now()
	repo := &affiliateSettlementRepoStub{
		invitees: map[int64]bool{42: true},
		logs: []settlementUsageLogStub{
			{id: 1, userID: 42, cost: 10, billingType: BillingTypeBalance, createdAt: now.Add(-time.Hour)},
		},
	}
	svc, affiliateRepo, _ := newAffiliateSettlementTestService(t, repo, map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "10",
	}, now)
	svc.cfg = &config.Config{RunMode: config.RunModeSimple}

	result, err := svc.SettleOnce(context.Background())
	require.NoError(t, err)
	require.Nil(t, result)
	require.Empty(t, affiliateRepo.accrueCalls)
	require.Empty(t, repo.advanceCalls)
}
