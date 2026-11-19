//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// affiliateWithdrawMemRepo 是提现申请的内存实现，模拟额度冻结与退回。
type affiliateWithdrawMemRepo struct {
	AffiliateRepository
	quota    map[int64]float64
	requests map[int64]*AffiliateWithdrawRequest
	nextID   int64
	creates  int
}

func newAffiliateWithdrawMemRepo() *affiliateWithdrawMemRepo {
	return &affiliateWithdrawMemRepo{quota: map[int64]float64{}, requests: map[int64]*AffiliateWithdrawRequest{}}
}

func (r *affiliateWithdrawMemRepo) CreateWithdrawRequest(_ context.Context, in AffiliateWithdrawRequestCreate) (*AffiliateWithdrawRequest, error) {
	r.creates++
	for _, req := range r.requests {
		if req.UserID == in.UserID && req.Status == AffiliateWithdrawStatusPending {
			return nil, ErrAffiliateWithdrawPendingExists
		}
	}
	if r.quota[in.UserID] < in.Amount {
		return nil, ErrAffiliateQuotaInsufficient
	}
	r.quota[in.UserID] -= in.Amount
	r.nextID++
	req := &AffiliateWithdrawRequest{ID: r.nextID, UserID: in.UserID, Amount: in.Amount, AlipayAccount: in.AlipayAccount, QRImage: in.QRImage, Status: AffiliateWithdrawStatusPending}
	r.requests[req.ID] = req
	cp := *req
	return &cp, nil
}

func (r *affiliateWithdrawMemRepo) ResolveWithdrawRequest(_ context.Context, id, expectUserID int64, status, note string, reviewerID *int64) (*AffiliateWithdrawRequest, error) {
	req, ok := r.requests[id]
	if !ok || (expectUserID > 0 && req.UserID != expectUserID) {
		return nil, ErrAffiliateWithdrawNotFound
	}
	if req.Status != AffiliateWithdrawStatusPending {
		return nil, ErrAffiliateWithdrawNotPending
	}
	if status != AffiliateWithdrawStatusPaid {
		r.quota[req.UserID] += req.Amount
	}
	req.Status = status
	req.AdminNote = note
	req.ReviewedBy = reviewerID
	cp := *req
	return &cp, nil
}

func (r *affiliateWithdrawMemRepo) GetWithdrawRequest(_ context.Context, id int64) (*AffiliateWithdrawRequest, error) {
	req, ok := r.requests[id]
	if !ok {
		return nil, ErrAffiliateWithdrawNotFound
	}
	cp := *req
	return &cp, nil
}

func (r *affiliateWithdrawMemRepo) ListWithdrawRequests(_ context.Context, filter AffiliateWithdrawRequestFilter) ([]AffiliateWithdrawRequest, int64, error) {
	out := []AffiliateWithdrawRequest{}
	for _, req := range r.requests {
		if filter.UserID > 0 && req.UserID != filter.UserID {
			continue
		}
		if filter.Status != "" && req.Status != filter.Status {
			continue
		}
		cp := *req
		cp.QRImage = ""
		out = append(out, cp)
	}
	return out, int64(len(out)), nil
}

func (r *affiliateWithdrawMemRepo) GetPendingWithdrawRequest(_ context.Context, userID int64) (*AffiliatePendingWithdraw, error) {
	for _, req := range r.requests {
		if req.UserID == userID && req.Status == AffiliateWithdrawStatusPending {
			return &AffiliatePendingWithdraw{ID: req.ID, Amount: req.Amount}, nil
		}
	}
	return nil, nil
}

var testAffiliateWithdrawQR = "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("fake-png-bytes"))

func newAffiliateWithdrawTestService(t *testing.T, values map[string]string) (*AffiliateService, *affiliateWithdrawMemRepo) {
	t.Helper()
	settings := map[string]string{
		SettingKeyAffiliateEnabled:           "true",
		SettingKeyAffiliateWithdrawEnabled:   "true",
		SettingKeyAffiliateWithdrawMinAmount: "50",
	}
	for k, v := range values {
		settings[k] = v
	}
	repo := newAffiliateWithdrawMemRepo()
	settingSvc := NewSettingService(&affiliateServiceTestSettingRepo{values: settings}, &config.Config{})
	return &AffiliateService{repo: repo, settingService: settingSvc}, repo
}

func TestSubmitWithdrawRequest_RespectsSwitches(t *testing.T) {
	t.Parallel()
	for _, values := range []map[string]string{
		{SettingKeyAffiliateWithdrawEnabled: "false"},
		{SettingKeyAffiliateEnabled: "false"},
	} {
		svc, repo := newAffiliateWithdrawTestService(t, values)
		repo.quota[1] = 100
		_, err := svc.SubmitWithdrawRequest(context.Background(), 1, 60, "a@b.com", testAffiliateWithdrawQR)
		require.ErrorIs(t, err, ErrAffiliateWithdrawDisabled)
		require.Zero(t, repo.creates)
	}
}

func TestSubmitWithdrawRequest_ValidatesInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		amount  float64
		account string
		qr      string
		want    error
	}{
		{"below min", 49.99, "a@b.com", testAffiliateWithdrawQR, ErrAffiliateWithdrawBelowMin},
		{"zero", 0, "a@b.com", testAffiliateWithdrawQR, ErrAffiliateWithdrawAmountInvalid},
		{"empty account", 60, "  ", testAffiliateWithdrawQR, ErrAffiliateWithdrawAccountInvalid},
		{"long account", 60, strings.Repeat("a", 129), testAffiliateWithdrawQR, ErrAffiliateWithdrawAccountInvalid},
		{"no qr", 60, "a@b.com", "", ErrAffiliateWithdrawQRInvalid},
		{"url qr", 60, "a@b.com", "https://example.com/qr.png", ErrAffiliateWithdrawQRInvalid},
		{"svg qr", 60, "a@b.com", "data:image/svg+xml;base64,PHN2Zy8+", ErrAffiliateWithdrawQRInvalid},
		{"bad base64", 60, "a@b.com", "data:image/png;base64,@@@", ErrAffiliateWithdrawQRInvalid},
		{"huge qr", 60, "a@b.com", "data:image/png;base64," + strings.Repeat("A", AffiliateWithdrawQRImageMaxBytes), ErrAffiliateWithdrawQRInvalid},
	}
	for _, tc := range cases {
		svc, repo := newAffiliateWithdrawTestService(t, nil)
		repo.quota[1] = 100
		_, err := svc.SubmitWithdrawRequest(context.Background(), 1, tc.amount, tc.account, tc.qr)
		require.ErrorIs(t, err, tc.want, tc.name)
		require.Zero(t, repo.creates, tc.name)
	}
}

func TestSubmitWithdrawRequest_HoldsQuotaAndBlocksSecondPending(t *testing.T) {
	t.Parallel()
	svc, repo := newAffiliateWithdrawTestService(t, nil)
	repo.quota[1] = 120

	req, err := svc.SubmitWithdrawRequest(context.Background(), 1, 80, " alipay@example.com ", testAffiliateWithdrawQR)
	require.NoError(t, err)
	require.Equal(t, AffiliateWithdrawStatusPending, req.Status)
	require.Equal(t, "alipay@example.com", req.AlipayAccount)
	require.Empty(t, req.QRImage, "user response must not echo the QR image")
	require.InDelta(t, 40, repo.quota[1], 1e-9)

	_, err = svc.SubmitWithdrawRequest(context.Background(), 1, 50, "alipay@example.com", testAffiliateWithdrawQR)
	require.ErrorIs(t, err, ErrAffiliateWithdrawPendingExists)
}

func TestSubmitWithdrawRequest_InsufficientQuota(t *testing.T) {
	t.Parallel()
	svc, repo := newAffiliateWithdrawTestService(t, nil)
	repo.quota[1] = 55
	_, err := svc.SubmitWithdrawRequest(context.Background(), 1, 60, "a@b.com", testAffiliateWithdrawQR)
	require.ErrorIs(t, err, ErrAffiliateQuotaInsufficient)
	require.InDelta(t, 55, repo.quota[1], 1e-9)
}

func TestWithdrawRequest_RejectAndCancelReturnQuota(t *testing.T) {
	t.Parallel()
	svc, repo := newAffiliateWithdrawTestService(t, nil)
	repo.quota[1] = 100

	req, err := svc.SubmitWithdrawRequest(context.Background(), 1, 60, "a@b.com", testAffiliateWithdrawQR)
	require.NoError(t, err)

	_, err = svc.AdminRejectWithdrawRequest(context.Background(), req.ID, 9, "  ")
	require.ErrorIs(t, err, ErrAffiliateWithdrawReasonRequired)

	rejected, err := svc.AdminRejectWithdrawRequest(context.Background(), req.ID, 9, "收款码无法识别")
	require.NoError(t, err)
	require.Equal(t, AffiliateWithdrawStatusRejected, rejected.Status)
	require.Equal(t, "收款码无法识别", rejected.AdminNote)
	require.InDelta(t, 100, repo.quota[1], 1e-9)

	_, err = svc.AdminMarkWithdrawPaid(context.Background(), req.ID, 9, "")
	require.ErrorIs(t, err, ErrAffiliateWithdrawNotPending)

	req2, err := svc.SubmitWithdrawRequest(context.Background(), 1, 70, "a@b.com", testAffiliateWithdrawQR)
	require.NoError(t, err)
	_, err = svc.CancelWithdrawRequest(context.Background(), 2, req2.ID)
	require.ErrorIs(t, err, ErrAffiliateWithdrawNotFound, "other users cannot cancel")
	cancelled, err := svc.CancelWithdrawRequest(context.Background(), 1, req2.ID)
	require.NoError(t, err)
	require.Equal(t, AffiliateWithdrawStatusCancelled, cancelled.Status)
	require.InDelta(t, 100, repo.quota[1], 1e-9)
}

func TestWithdrawRequest_MarkPaidKeepsQuotaDeducted(t *testing.T) {
	t.Parallel()
	svc, repo := newAffiliateWithdrawTestService(t, nil)
	repo.quota[1] = 100

	req, err := svc.SubmitWithdrawRequest(context.Background(), 1, 100, "a@b.com", testAffiliateWithdrawQR)
	require.NoError(t, err)
	paid, err := svc.AdminMarkWithdrawPaid(context.Background(), req.ID, 9, "已转账")
	require.NoError(t, err)
	require.Equal(t, AffiliateWithdrawStatusPaid, paid.Status)
	require.NotNil(t, paid.ReviewedBy)
	require.Equal(t, int64(9), *paid.ReviewedBy)
	require.InDelta(t, 0, repo.quota[1], 1e-9)

	_, err = svc.CancelWithdrawRequest(context.Background(), 1, req.ID)
	require.ErrorIs(t, err, ErrAffiliateWithdrawNotPending)
}

func TestSettingService_AffiliateWithdrawSettingsRoundTrip(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		AffiliateWithdrawEnabled:   true,
		AffiliateWithdrawMinAmount: 30,
		AffiliateLevelRates:        []float64{10, 1, 0.1},
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyAffiliateWithdrawEnabled])
	require.Equal(t, "30.00000000", repo.updates[SettingKeyAffiliateWithdrawMinAmount])
	require.Equal(t, "[10,1,0.1]", repo.updates[SettingKeyAffiliateLevelRates])

	parsed := svc.parseSettings(map[string]string{
		SettingKeyAffiliateWithdrawEnabled:   "true",
		SettingKeyAffiliateWithdrawMinAmount: "30.00000000",
	})
	require.True(t, parsed.AffiliateWithdrawEnabled)
	require.Equal(t, 30.0, parsed.AffiliateWithdrawMinAmount)

	parsedDefault := svc.parseSettings(map[string]string{})
	require.False(t, parsedDefault.AffiliateWithdrawEnabled)
	require.Equal(t, AffiliateWithdrawMinAmountDefault, parsedDefault.AffiliateWithdrawMinAmount)
}
