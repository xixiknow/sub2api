//go:build unit

package service

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateServiceTestRepo struct {
	invite              *AffiliateRegistrationInvite
	consumed            bool
	qualifiedInvitees   int
	qualifiedInviteeErr error
}

type affiliateServiceTestSettingRepo struct {
	values map[string]string
}

func (r *affiliateServiceTestSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *affiliateServiceTestSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *affiliateServiceTestSettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[key] = value
	return nil
}

func (r *affiliateServiceTestSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

func (r *affiliateServiceTestSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *affiliateServiceTestSettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *affiliateServiceTestSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func newFreeRegistrationSeatSettingService() *SettingService {
	return NewSettingService(&affiliateServiceTestSettingRepo{
		values: map[string]string{
			SettingKeyAffiliateRegistrationSeatCost: "0",
		},
	}, nil)
}

func (r *affiliateServiceTestRepo) EnsureUserAffiliate(context.Context, int64) (*AffiliateSummary, error) {
	return &AffiliateSummary{}, nil
}

func (r *affiliateServiceTestRepo) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	return &AffiliateSummary{}, nil
}

func (r *affiliateServiceTestRepo) GetRegistrationInviteByCode(_ context.Context, code string) (*AffiliateRegistrationInvite, error) {
	if r.invite == nil || strings.ToUpper(strings.TrimSpace(code)) != r.invite.AffCode {
		return nil, ErrAffiliateProfileNotFound
	}
	out := *r.invite
	return &out, nil
}

func (r *affiliateServiceTestRepo) BindInviter(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (r *affiliateServiceTestRepo) ConsumeRegistrationInviteSeat(ctx context.Context, code string, inviteeUserID int64) (*AffiliateRegistrationInvite, error) {
	r.consumed = true
	return r.GetRegistrationInviteByCode(ctx, code)
}

func (r *affiliateServiceTestRepo) RestoreRegistrationInviteSeat(context.Context, string, int64) error {
	return nil
}

func (r *affiliateServiceTestRepo) PurchaseRegistrationSeats(context.Context, int64, int, float64) (*AffiliateRegistrationSeatPurchaseResult, error) {
	return &AffiliateRegistrationSeatPurchaseResult{}, nil
}

func (r *affiliateServiceTestRepo) AccrueQuota(context.Context, int64, int64, float64, int, *int64, int, float64) (bool, error) {
	return false, nil
}

func (r *affiliateServiceTestRepo) ClawbackQuotaForOrder(context.Context, int64, float64) (AffiliateClawbackResult, error) {
	return AffiliateClawbackResult{}, nil
}

func (r *affiliateServiceTestRepo) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}

func (r *affiliateServiceTestRepo) GetLevelRebateSummary(context.Context, int64) ([]AffiliateLevelRebateSummary, error) {
	return nil, nil
}

func (r *affiliateServiceTestRepo) GetLevelDetails(context.Context, int64, int) ([]AffiliateLevelDetail, error) {
	return nil, nil
}

func (r *affiliateServiceTestRepo) CountQualifiedInvitees(context.Context, int64) (int, error) {
	if r.qualifiedInviteeErr != nil {
		return 0, r.qualifiedInviteeErr
	}
	return r.qualifiedInvitees, nil
}

func (r *affiliateServiceTestRepo) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (r *affiliateServiceTestRepo) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	return 0, 0, nil
}

func (r *affiliateServiceTestRepo) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return nil, nil
}

func (r *affiliateServiceTestRepo) UpdateUserAffCode(context.Context, int64, string) error {
	return nil
}

func (r *affiliateServiceTestRepo) ResetUserAffCode(context.Context, int64) (string, error) {
	return "", nil
}

func (r *affiliateServiceTestRepo) SetUserRebateRate(context.Context, int64, *float64) error {
	return nil
}

func (r *affiliateServiceTestRepo) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	return nil
}

func (r *affiliateServiceTestRepo) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	return nil, 0, nil
}

func (r *affiliateServiceTestRepo) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	return nil, 0, nil
}

func (r *affiliateServiceTestRepo) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	return nil, 0, nil
}

func (r *affiliateServiceTestRepo) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	return nil, 0, nil
}

func (r *affiliateServiceTestRepo) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	return &AffiliateUserOverview{}, nil
}

// TestResolveRebateRatePercent_PerUserOverride verifies that per-inviter
// AffRebateRatePercent overrides the global rate, that NULL falls back to the
// global rate, and that out-of-range exclusive rates are clamped silently.
//
// SettingService is left nil here so globalRebateRatePercent returns the
// documented default (AffiliateRebateRateDefault = 5%) — this exercises the
// fallback path without spinning up a settings stub.
func TestResolveRebateRatePercent_PerUserOverride(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	// nil exclusive rate → falls back to global default (5%)
	require.InDelta(t, AffiliateRebateRateDefault,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}), 1e-9)

	// exclusive rate set → overrides global
	rate := 50.0
	require.InDelta(t, 50.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &rate}), 1e-9)

	// exclusive rate 0 → returns 0 (no rebate, intentional)
	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &zero}), 1e-9)

	// exclusive rate above max → clamped to Max
	tooHigh := 250.0
	require.InDelta(t, AffiliateRebateRateMax,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}), 1e-9)

	// exclusive rate below min → clamped to Min
	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooLow}), 1e-9)
}

func TestDefaultAffiliateLevelRates_StartAtFiveAndLockDownline(t *testing.T) {
	t.Parallel()
	require.Equal(t, []float64{5, 1, 0.5}, defaultAffiliateLevelRates())
	require.Equal(t, "[5,1,0.5]", AffiliateLevelRatesDefault)
	require.InDelta(t, 5.0, AffiliateRebateRateDefault, 1e-9)
}

func TestEffectiveLevelRateRules_LocksDownlineUntilInviteTasks(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{settingService: NewSettingService(&affiliateServiceTestSettingRepo{
		values: map[string]string{
			SettingKeyAffiliateLevelRates: "[5,1,0.5]",
		},
	}, nil)}

	locked := svc.effectiveLevelRateRules(context.Background(), &AffiliateSummary{AffCount: 10}, 2)
	require.InDelta(t, 5, locked[0].RatePercent, 1e-9)
	require.True(t, locked[0].Unlocked)
	require.InDelta(t, 0, locked[1].RatePercent, 1e-9)
	require.Equal(t, "locked", locked[1].Source)
	require.False(t, locked[1].Unlocked)
	require.Equal(t, AffiliateLevel2UnlockInviteCount, locked[1].UnlockInviteCount)
	require.InDelta(t, 0, locked[2].RatePercent, 1e-9)
	require.Equal(t, "locked", locked[2].Source)
	require.False(t, locked[2].Unlocked)
	require.Equal(t, AffiliateLevel3UnlockInviteCount, locked[2].UnlockInviteCount)

	level2 := svc.effectiveLevelRateRules(context.Background(), &AffiliateSummary{AffCount: 10}, AffiliateLevel2UnlockInviteCount)
	require.InDelta(t, 1, level2[1].RatePercent, 1e-9)
	require.Equal(t, "global", level2[1].Source)
	require.True(t, level2[1].Unlocked)
	require.InDelta(t, 0, level2[2].RatePercent, 1e-9)

	level3 := svc.effectiveLevelRateRules(context.Background(), &AffiliateSummary{AffCount: 10}, AffiliateLevel3UnlockInviteCount)
	require.InDelta(t, 1, level3[1].RatePercent, 1e-9)
	require.InDelta(t, 0.5, level3[2].RatePercent, 1e-9)
	require.Equal(t, "global", level3[2].Source)
	require.True(t, level3[2].Unlocked)
}

func TestValidateRegistrationInviteCode_FreeSeatsDoesNotRequireQuota(t *testing.T) {
	t.Parallel()
	repo := &affiliateServiceTestRepo{
		invite: &AffiliateRegistrationInvite{
			UserID:                    10,
			AffCode:                   "VIP2026",
			RegistrationSeatTotal:     0,
			RegistrationSeatUsed:      0,
			RegistrationSeatAvailable: 0,
		},
	}
	svc := &AffiliateService{repo: repo, settingService: newFreeRegistrationSeatSettingService()}

	invite, err := svc.ValidateRegistrationInviteCode(context.Background(), "vip2026")
	require.NoError(t, err)
	require.Equal(t, "VIP2026", invite.AffCode)
}

func TestConsumeRegistrationInviteSeat_FreeSeatsDoesNotConsume(t *testing.T) {
	t.Parallel()
	repo := &affiliateServiceTestRepo{
		invite: &AffiliateRegistrationInvite{
			UserID:                    10,
			AffCode:                   "VIP2026",
			RegistrationSeatTotal:     0,
			RegistrationSeatUsed:      0,
			RegistrationSeatAvailable: 0,
		},
	}
	svc := &AffiliateService{repo: repo, settingService: newFreeRegistrationSeatSettingService()}

	invite, err := svc.ConsumeRegistrationInviteSeat(context.Background(), "VIP2026", 22)
	require.NoError(t, err)
	require.Equal(t, "VIP2026", invite.AffCode)
	require.False(t, repo.consumed)
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled
// safely handles a nil settingService dependency by returning the default
// (off). This protects callers from nil-pointer crashes in misconfigured
// environments.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
