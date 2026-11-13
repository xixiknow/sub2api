package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/sms"
)

var forecastNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func fptr(v float64) *float64 { return &v }

func relayAccount(id int64, name, baseURL string, snap *UpstreamBalanceResult) *Account {
	extra := map[string]any{}
	if snap != nil {
		extra[UpstreamBalanceExtraKey] = snap
	}
	return &Account{
		ID: id, Name: name, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"api_key": "k", "base_url": baseURL},
		Extra:       extra,
	}
}

func okSnap(source string, balance float64, limit *float64) *UpstreamBalanceResult {
	return &UpstreamBalanceResult{Status: UpstreamBalanceStatusOK, Source: source, Balance: fptr(balance), Limit: limit, Currency: "USD", FetchedAt: forecastNow.Add(-10 * time.Minute)}
}

func points(start time.Time, step time.Duration, balances ...float64) []UpstreamBalancePoint {
	out := make([]UpstreamBalancePoint, len(balances))
	for i, b := range balances {
		out[i] = UpstreamBalancePoint{At: start.Add(time.Duration(i) * step), Balance: b}
	}
	return out
}

func TestUpstreamBurnFromPoints(t *testing.T) {
	since := forecastNow.Add(-24 * time.Hour)
	// 12 小时内 100 → 90 → 120（充值）→ 110：消耗 10 + 10 = 20，折算每天 40。
	got := upstreamBurnFromPoints(points(forecastNow.Add(-12*time.Hour), 4*time.Hour, 100, 90, 120, 110), since)
	if got == nil || *got != 40 {
		t.Fatalf("burn = %v, want 40", got)
	}
	// 覆盖不足 6 小时 → nil。
	if got := upstreamBurnFromPoints(points(forecastNow.Add(-2*time.Hour), time.Hour, 100, 90), since); got != nil {
		t.Fatalf("short span should be nil, got %v", *got)
	}
	// 只有一个点 → nil；窗口外的点被忽略。
	if got := upstreamBurnFromPoints(points(forecastNow.Add(-48*time.Hour), 40*time.Hour, 100, 50), since); got != nil {
		t.Fatalf("single point in window should be nil, got %v", *got)
	}
}

func TestBuildUpstreamWalletsForecastAndPriority(t *testing.T) {
	thresholds := UpstreamBalanceThresholds{WarnDays: 3, CriticalDays: 1, MinBalance: 10, TargetDays: 7}
	a1 := relayAccount(1, "healthy", "https://a.example.com/v1", okSnap(UpstreamBalanceSourceNewAPIKey, 500, fptr(1000)))
	a2 := relayAccount(2, "low-days", "https://b.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 30, fptr(100)))
	a3 := relayAccount(3, "exhausted", "https://c.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 0, fptr(100)))
	a4 := relayAccount(4, "failed", "https://d.example.com", &UpstreamBalanceResult{Status: UpstreamBalanceStatusFailed, Error: "http_401", FetchedAt: forecastNow})
	a5 := relayAccount(5, "unlimited", "https://e.example.com", &UpstreamBalanceResult{Status: UpstreamBalanceStatusOK, Unlimited: true, FetchedAt: forecastNow})

	wallets := buildUpstreamWallets(upstreamForecastInput{
		Accounts: []*Account{a1, a2, a3, a4, a5},
		Snapshots: map[int64][]UpstreamBalancePoint{
			// a2：12 小时内 60 → 30，日消耗 60，可用 0.5 天 → critical。
			2: points(forecastNow.Add(-12*time.Hour), 6*time.Hour, 60, 45, 30),
		},
		// a1 无快照历史，用本平台用量兜底：24h 成本 50 → 可用 10 天。
		Cost24h:    map[int64]float64{1: 50, 2: 5},
		Cost7d:     map[int64]float64{1: 300, 2: 100},
		Thresholds: thresholds,
		Now:        forecastNow,
	})

	order := make([]string, len(wallets))
	for i, w := range wallets {
		order[i] = w.Members[0].Name + ":" + w.Status
	}
	want := "exhausted:exhausted,low-days:critical,failed:unknown,healthy:ok,unlimited:unlimited"
	if strings.Join(order, ",") != want {
		t.Fatalf("order = %s\nwant    %s", strings.Join(order, ","), want)
	}

	low := wallets[1]
	if low.DailyBurnSource != UpstreamBurnSourceSnapshot || *low.DailyBurn != 60 || *low.DaysLeft != 0.5 {
		t.Fatalf("low wallet burn=%v src=%s days=%v", *low.DailyBurn, low.DailyBurnSource, *low.DaysLeft)
	}
	if *low.SuggestRecharge != 60*7-30 {
		t.Fatalf("suggest = %v", *low.SuggestRecharge)
	}
	healthy := wallets[3]
	if healthy.DailyBurnSource != UpstreamBurnSourceUsage || *healthy.DailyBurn != 50 || *healthy.DaysLeft != 10 {
		t.Fatalf("healthy wallet burn=%v src=%s days=%v", *healthy.DailyBurn, healthy.DailyBurnSource, *healthy.DaysLeft)
	}
	if healthy.TrafficShare != 0.75 {
		t.Fatalf("traffic share = %v", healthy.TrafficShare)
	}

	summary := summarizeUpstreamWallets(wallets)
	if summary.TotalBalance != 530 || summary.Exhausted != 1 || summary.Critical != 1 || summary.Unknown != 1 || summary.Unlimited != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestBuildUpstreamWalletsMergesSharedSub2APIWallet(t *testing.T) {
	// 同站点、余额相同、无 key 级限额的 sub2api key 视为同一钱包。
	a1 := relayAccount(11, "key-a", "https://relay.example.com", okSnap(UpstreamBalanceSourceSub2API, 42.5, nil))
	a2 := relayAccount(12, "key-b", "https://RELAY.example.com/v1", okSnap(UpstreamBalanceSourceSub2API, 42.5, nil))
	// 有 key 级限额的不合并。
	a3 := relayAccount(13, "key-quota", "https://relay.example.com", okSnap(UpstreamBalanceSourceSub2API, 42.5, fptr(50)))
	a2.Extra[UpstreamBalanceAlertExtraKey] = map[string]any{"min_balance": 50}

	wallets := buildUpstreamWallets(upstreamForecastInput{
		Accounts: []*Account{a1, a2, a3},
		Snapshots: map[int64][]UpstreamBalancePoint{
			11: points(forecastNow.Add(-12*time.Hour), 12*time.Hour, 52.5, 42.5),
			12: points(forecastNow.Add(-12*time.Hour), 12*time.Hour, 52.5, 42.5),
		},
		Cost24h:    map[int64]float64{11: 6, 12: 4},
		Thresholds: UpstreamBalanceThresholds{WarnDays: 3, CriticalDays: 1, MinBalance: 10, TargetDays: 7},
		Now:        forecastNow,
	})
	if len(wallets) != 2 {
		t.Fatalf("wallets = %d, want 2", len(wallets))
	}
	var shared *UpstreamWallet
	for i := range wallets {
		if len(wallets[i].Members) == 2 {
			shared = &wallets[i]
		}
	}
	if shared == nil || shared.PrimaryAccountID != 11 || shared.Site != "relay.example.com" {
		t.Fatalf("shared wallet not merged: %+v", wallets)
	}
	// 共享钱包的快照消耗取最大值而非相加：12 小时降 10 → 每天 20。
	if *shared.DailyBurn != 20 {
		t.Fatalf("shared burn = %v, want 20", *shared.DailyBurn)
	}
	// 账号覆盖 min_balance=50 → 余额 42.5 < 50 → warn（可用 2.1 天也 < 3）。
	if shared.Thresholds.MinBalance != 50 || shared.Status != UpstreamWalletStatusWarn {
		t.Fatalf("thresholds=%+v status=%s", shared.Thresholds, shared.Status)
	}
}

func TestUpstreamWalletStatus(t *testing.T) {
	th := UpstreamBalanceThresholds{WarnDays: 3, CriticalDays: 1, MinBalance: 10}
	cases := []struct {
		balance float64
		days    *float64
		idle    bool
		want    string
	}{
		{0, nil, false, UpstreamWalletStatusExhausted},
		{-1, fptr(0), false, UpstreamWalletStatusExhausted},
		{100, fptr(0.5), false, UpstreamWalletStatusCritical},
		{4, nil, false, UpstreamWalletStatusCritical},
		{100, fptr(2), false, UpstreamWalletStatusWarn},
		{8, nil, false, UpstreamWalletStatusWarn},
		{100, fptr(5), false, UpstreamWalletStatusOK},
		{100, nil, false, UpstreamWalletStatusOK},
		// 闲置：余额低最多预警，耗尽仍为耗尽。
		{4, nil, true, UpstreamWalletStatusWarn},
		{0, nil, true, UpstreamWalletStatusExhausted},
		{100, nil, true, UpstreamWalletStatusOK},
	}
	for _, c := range cases {
		if got := upstreamWalletStatus(c.balance, c.days, th, c.idle); got != c.want {
			t.Fatalf("status(%v,%v,idle=%v) = %s, want %s", c.balance, c.days, c.idle, got, c.want)
		}
	}
}

func TestInUpstreamQuietHours(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 1, 1, h, m, 0, 0, time.UTC) }
	if !inUpstreamQuietHours(at(23, 30), "23:00", "08:00") || !inUpstreamQuietHours(at(7, 59), "23:00", "08:00") {
		t.Fatal("expected wrapped window to match")
	}
	if inUpstreamQuietHours(at(8, 0), "23:00", "08:00") || inUpstreamQuietHours(at(12, 0), "", "") {
		t.Fatal("unexpected quiet match")
	}
	if !inUpstreamQuietHours(at(13, 0), "12:00", "14:00") || inUpstreamQuietHours(at(14, 0), "12:00", "14:00") {
		t.Fatal("same-day window mismatch")
	}
}

func TestUpstreamAlertsNeedSend(t *testing.T) {
	warn := UpstreamWallet{PrimaryAccountID: 1, Status: UpstreamWalletStatusWarn}
	critical := UpstreamWallet{PrimaryAccountID: 1, Status: UpstreamWalletStatusCritical}
	sentWarn := []UpstreamBalanceAlert{{Status: UpstreamBalanceAlertStatusSent, AccountLevels: map[string]string{"1": "warn"}}}
	failedWarn := []UpstreamBalanceAlert{{Status: UpstreamBalanceAlertStatusFailed, AccountLevels: map[string]string{"1": "warn"}}}
	if !upstreamAlertsNeedSend([]UpstreamWallet{warn}, nil) {
		t.Fatal("first alert should send")
	}
	if upstreamAlertsNeedSend([]UpstreamWallet{warn}, sentWarn) {
		t.Fatal("same level within 24h should not resend")
	}
	if !upstreamAlertsNeedSend([]UpstreamWallet{critical}, sentWarn) {
		t.Fatal("escalation should resend")
	}
	if !upstreamAlertsNeedSend([]UpstreamWallet{warn}, failedWarn) {
		t.Fatal("failed delivery should be retried")
	}
}

func TestNormalizeUpstreamBalanceAlertConfig(t *testing.T) {
	cfg := defaultUpstreamBalanceAlertConfig()
	cfg.Phones = []string{" 138 0000 0000", "13800000000", ""}
	out, err := normalizeUpstreamBalanceAlertConfig(cfg)
	if err != nil || len(out.Phones) != 1 || out.Phones[0] != "13800000000" {
		t.Fatalf("phones = %v err=%v", out.Phones, err)
	}
	bad := []func(c *UpstreamBalanceAlertConfig){
		func(c *UpstreamBalanceAlertConfig) { c.WarnDays = 0.5 },
		func(c *UpstreamBalanceAlertConfig) { c.TargetDays = 0 },
		func(c *UpstreamBalanceAlertConfig) { c.QuietStart = "25:00"; c.QuietEnd = "08:00" },
		func(c *UpstreamBalanceAlertConfig) { c.QuietStart = "23:00" },
		func(c *UpstreamBalanceAlertConfig) { c.Phones = []string{"abc"} },
		func(c *UpstreamBalanceAlertConfig) { c.SMS.Provider = "twilio" },
		func(c *UpstreamBalanceAlertConfig) { c.SMS.Endpoint = "http://evil.example.com" },
		func(c *UpstreamBalanceAlertConfig) { c.RefreshIntervalMinutes = 5 },
	}
	for i, mutate := range bad {
		c := defaultUpstreamBalanceAlertConfig()
		mutate(&c)
		if _, err := normalizeUpstreamBalanceAlertConfig(c); err == nil {
			t.Fatalf("case %d should be invalid", i)
		}
	}
}

// ---- evaluate：停调度 + 短信 ----

type fakeUpstreamOpsRepo struct {
	snapshots map[int64][]UpstreamBalancePoint
	cost24h   map[int64]float64
	alerts    []UpstreamBalanceAlert
	inserted  int
}

func (r *fakeUpstreamOpsRepo) InsertSnapshot(_ context.Context, _ int64, _ *UpstreamBalanceResult) error {
	r.inserted++
	return nil
}
func (r *fakeUpstreamOpsRepo) ListSnapshots(context.Context, []int64, time.Time) (map[int64][]UpstreamBalancePoint, error) {
	return r.snapshots, nil
}
func (r *fakeUpstreamOpsRepo) PruneSnapshots(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeUpstreamOpsRepo) GetAccountUpstreamCost(_ context.Context, _ []int64, since time.Time) (map[int64]float64, error) {
	return r.cost24h, nil
}
func (r *fakeUpstreamOpsRepo) GetAccountDailyUpstreamCost(context.Context, []int64, time.Time, string) ([]UpstreamCostBucket, error) {
	return nil, nil
}
func (r *fakeUpstreamOpsRepo) InsertAlert(_ context.Context, a *UpstreamBalanceAlert) error {
	a.ID = int64(len(r.alerts) + 1)
	a.CreatedAt = forecastNow
	r.alerts = append(r.alerts, *a)
	return nil
}
func (r *fakeUpstreamOpsRepo) ListAlerts(context.Context, time.Time, int) ([]UpstreamBalanceAlert, error) {
	return r.alerts, nil
}

type fakeUpstreamAccountRepo struct {
	AccountRepository
	accounts     []Account
	paused       map[int64]string
	cleared      []int64
	extraUpdates map[int64]map[string]any
}

func (r *fakeUpstreamAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			return &r.accounts[i], nil
		}
	}
	return nil, ErrAccountNotFound
}
func (r *fakeUpstreamAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	if r.extraUpdates == nil {
		r.extraUpdates = map[int64]map[string]any{}
	}
	r.extraUpdates[id] = updates
	return nil
}

func (r *fakeUpstreamAccountRepo) ListAllWithFilters(context.Context, string, string, string, string, int64, string) ([]Account, error) {
	return r.accounts, nil
}
func (r *fakeUpstreamAccountRepo) SetTempUnschedulable(_ context.Context, id int64, _ time.Time, reason string) error {
	r.paused[id] = reason
	return nil
}
func (r *fakeUpstreamAccountRepo) ClearTempUnschedulable(_ context.Context, id int64) error {
	r.cleared = append(r.cleared, id)
	return nil
}

type upstreamOpsSettingRepo struct {
	SettingRepository
	values map[string]string
}

func (r *upstreamOpsSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	v, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (r *upstreamOpsSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

type upstreamOpsTestEncryptor struct{}

func (upstreamOpsTestEncryptor) Encrypt(s string) (string, error) { return "enc:" + s, nil }
func (upstreamOpsTestEncryptor) Decrypt(s string) (string, error) {
	if !strings.HasPrefix(s, "enc:") {
		return "", errors.New("bad ciphertext")
	}
	return strings.TrimPrefix(s, "enc:"), nil
}

type fakeSMSProvider struct {
	sent []sms.Message
	err  error
}

func (p *fakeSMSProvider) Name() string { return "fake" }
func (p *fakeSMSProvider) Send(_ context.Context, msg sms.Message) error {
	p.sent = append(p.sent, msg)
	return p.err
}

func newTestOpsService(t *testing.T, accounts []Account) (*UpstreamBalanceOpsService, *fakeUpstreamOpsRepo, *fakeUpstreamAccountRepo, *fakeSMSProvider) {
	t.Helper()
	repo := &fakeUpstreamOpsRepo{}
	accountRepo := &fakeUpstreamAccountRepo{accounts: accounts, paused: map[int64]string{}}
	settings := &upstreamOpsSettingRepo{values: map[string]string{}}
	provider := &fakeSMSProvider{}
	svc := NewUpstreamBalanceOpsService(repo, accountRepo, settings, upstreamOpsTestEncryptor{}, nil)
	svc.now = func() time.Time { return forecastNow }
	svc.newSMS = func(sms.Config) (sms.Provider, error) { return provider, nil }
	cfg := defaultUpstreamBalanceAlertConfig()
	cfg.Enabled = true
	cfg.Phones = []string{"13800000000"}
	cfg.SMS = UpstreamBalanceSMSConfig{Provider: sms.ProviderAliyun, AccessKeyID: "ak", AccessKeySecret: "secret", SignName: "s", TemplateID: "SMS_1"}
	if _, err := svc.UpdateConfig(context.Background(), cfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	return svc, repo, accountRepo, provider
}

func TestEvaluatePausesExhaustedAndSendsOneDigest(t *testing.T) {
	exhausted := *relayAccount(1, "dead", "https://a.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 0, fptr(100)))
	low := *relayAccount(2, "low", "https://b.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 5, fptr(100)))
	until := forecastNow.Add(time.Hour)
	recovered := *relayAccount(3, "recovered", "https://c.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 500, fptr(1000)))
	recovered.TempUnschedulableUntil = &until
	recovered.TempUnschedulableReason = upstreamBalanceExhaustedReasonPrefix + "old"

	svc, repo, accountRepo, provider := newTestOpsService(t, []Account{exhausted, low, recovered})
	svc.evaluate(context.Background())

	if _, ok := accountRepo.paused[1]; !ok || len(accountRepo.paused) != 1 {
		t.Fatalf("paused = %v, want only account 1", accountRepo.paused)
	}
	if len(accountRepo.cleared) != 1 || accountRepo.cleared[0] != 3 {
		t.Fatalf("cleared = %v, want [3]", accountRepo.cleared)
	}
	if len(provider.sent) != 1 {
		t.Fatalf("sent %d sms, want 1", len(provider.sent))
	}
	params := provider.sent[0].Params
	if params[0].Value != "2" || params[1].Value != "dead" || params[2].Value != "0.00" {
		t.Fatalf("params = %+v", params)
	}
	if len(repo.alerts) != 1 || repo.alerts[0].Level != UpstreamWalletStatusExhausted || repo.alerts[0].Phones != "138****0000" {
		t.Fatalf("alerts = %+v", repo.alerts)
	}

	// 第二轮：状态不变，不重复发送。
	svc.evaluate(context.Background())
	if len(provider.sent) != 1 {
		t.Fatalf("resent within 24h: %d", len(provider.sent))
	}
}

func TestEvaluateRespectsDailyLimitAndDisabled(t *testing.T) {
	low := *relayAccount(2, "low", "https://b.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 5, fptr(100)))
	svc, repo, _, provider := newTestOpsService(t, []Account{low})
	repo.alerts = []UpstreamBalanceAlert{{Status: UpstreamBalanceAlertStatusSent, Level: "warn", AccountLevels: map[string]string{"99": "warn"}, CreatedAt: forecastNow}}
	cfg, _ := svc.loadConfig(context.Background())
	cfg.DailyLimit = 1
	cfg.SMS.AccessKeySecret = ""
	if _, err := svc.UpdateConfig(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	svc.evaluate(context.Background())
	if len(provider.sent) != 0 {
		t.Fatalf("daily limit ignored: sent %d", len(provider.sent))
	}

	// 账号级关闭提醒。
	low.Extra[UpstreamBalanceAlertExtraKey] = map[string]any{"disabled": true}
	svc2, _, _, provider2 := newTestOpsService(t, []Account{low})
	svc2.evaluate(context.Background())
	if len(provider2.sent) != 0 {
		t.Fatalf("disabled account alerted")
	}
}

func TestUpdateConfigKeepsSecretAndMasks(t *testing.T) {
	svc, _, _, _ := newTestOpsService(t, nil)
	cfg, err := svc.GetConfig(context.Background())
	if err != nil || !cfg.SMS.AccessKeySecretSet || cfg.SMS.AccessKeySecret != "" || cfg.SMS.AccessKeySecretEnc != "" {
		t.Fatalf("config not masked: %+v err=%v", cfg.SMS, err)
	}
	// 不传 secret → 保留原值。
	cfg.MinBalance = 20
	if _, err := svc.UpdateConfig(context.Background(), *cfg); err != nil {
		t.Fatal(err)
	}
	stored, _ := svc.loadConfig(context.Background())
	if stored.SMS.AccessKeySecretEnc != "enc:secret" || stored.MinBalance != 20 {
		t.Fatalf("stored = %+v", stored)
	}
}
