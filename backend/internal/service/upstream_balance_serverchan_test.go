package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type fakeServerChan struct {
	sent []string // "key|title|desp"
	fail map[string]error
}

func (f *fakeServerChan) send(_ context.Context, key, title, desp string) error {
	f.sent = append(f.sent, key+"|"+title+"|"+desp)
	return f.fail[key]
}

func useServerChan(t *testing.T, svc *UpstreamBalanceOpsService, keys ...string) *fakeServerChan {
	t.Helper()
	fake := &fakeServerChan{fail: map[string]error{}}
	svc.sendServerChan = fake.send
	cfg, _ := svc.loadConfig(context.Background())
	cfg.Channel = UpstreamBalanceChannelServerChan
	cfg.Phones = nil
	cfg.SMS.AccessKeySecret = ""
	cfg.ServerChan.SendKeys = keys
	if _, err := svc.UpdateConfig(context.Background(), cfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	return fake
}

func TestEvaluateSendsServerChanDigestInsteadOfSMS(t *testing.T) {
	exhausted := *relayAccount(1, "dead", "https://a.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 0, fptr(100)))
	low := *relayAccount(2, "low", "https://b.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 5, fptr(100)))
	svc, repo, _, provider := newTestOpsService(t, []Account{exhausted, low})
	fake := useServerChan(t, svc, "SCT111aaa", "sctp22t333bbb")

	svc.evaluate(context.Background())

	if len(provider.sent) != 0 {
		t.Fatalf("sms sent on serverchan channel")
	}
	if len(fake.sent) != 2 {
		t.Fatalf("serverchan sent %d, want 2 (one per key)", len(fake.sent))
	}
	parts := strings.SplitN(fake.sent[0], "|", 3)
	if parts[1] != "2 个上游余额告急：a.example.com 已耗尽" || !strings.Contains(parts[2], "**[已耗尽] a.example.com**") || !strings.Contains(parts[2], "[预警] b.example.com") {
		t.Fatalf("message = %q", fake.sent[0])
	}
	if len(repo.alerts) != 1 || repo.alerts[0].Provider != UpstreamBalanceChannelServerChan || repo.alerts[0].Status != UpstreamBalanceAlertStatusSent || strings.Contains(repo.alerts[0].Phones, "111aaa") {
		t.Fatalf("alerts = %+v", repo.alerts)
	}

	svc.evaluate(context.Background())
	if len(fake.sent) != 2 {
		t.Fatalf("resent within 24h")
	}
}

func TestServerChanPartialAndTotalFailure(t *testing.T) {
	low := *relayAccount(2, "low", "https://b.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 5, fptr(100)))
	svc, repo, _, _ := newTestOpsService(t, []Account{low})
	fake := useServerChan(t, svc, "SCT111aaa", "SCT222bbb")
	fake.fail["SCT111aaa"] = errors.New("boom")
	fake.fail["SCT222bbb"] = errors.New("boom")

	svc.evaluate(context.Background())
	if len(repo.alerts) != 1 || repo.alerts[0].Status != UpstreamBalanceAlertStatusFailed {
		t.Fatalf("alerts = %+v", repo.alerts)
	}
	// 全部失败不算已发送：下一轮重试；一个成功即记为 sent。
	delete(fake.fail, "SCT222bbb")
	svc.evaluate(context.Background())
	if len(repo.alerts) != 2 || repo.alerts[1].Status != UpstreamBalanceAlertStatusSent || repo.alerts[1].Error == "" {
		t.Fatalf("alerts = %+v", repo.alerts)
	}
}

func TestServerChanKeysEncryptedAndKept(t *testing.T) {
	svc, _, _, _ := newTestOpsService(t, nil)
	useServerChan(t, svc, "SCT111aaa", " SCT111aaa ", "")
	stored, _ := svc.loadConfig(context.Background())
	if !strings.HasPrefix(stored.ServerChan.SendKeysEnc, "enc:") || stored.ServerChan.SendKeysCount != 1 || len(stored.ServerChan.SendKeys) != 0 {
		t.Fatalf("stored = %+v", stored.ServerChan)
	}
	view, _ := svc.GetConfig(context.Background())
	if view.ServerChan.SendKeysEnc != "" || view.ServerChan.SendKeysCount != 1 || view.Channel != UpstreamBalanceChannelServerChan {
		t.Fatalf("view = %+v", view.ServerChan)
	}
	// 不传 key → 保持原值。
	view.MinBalance = 30
	if _, err := svc.UpdateConfig(context.Background(), *view); err != nil {
		t.Fatal(err)
	}
	again, _ := svc.loadConfig(context.Background())
	if again.ServerChan.SendKeysEnc != stored.ServerChan.SendKeysEnc || again.ServerChan.SendKeysCount != 1 {
		t.Fatalf("keys lost: %+v", again.ServerChan)
	}
	bad := *view
	bad.ServerChan.SendKeys = []string{"not-a-key"}
	if _, err := svc.UpdateConfig(context.Background(), bad); err == nil {
		t.Fatal("invalid key accepted")
	}
	bad = *view
	bad.Channel = "wechat"
	if _, err := svc.UpdateConfig(context.Background(), bad); err == nil {
		t.Fatal("invalid channel accepted")
	}
}

func TestSendTestUsesServerChan(t *testing.T) {
	svc, repo, _, provider := newTestOpsService(t, nil)
	fake := useServerChan(t, svc, "SCT111aaa")
	if err := svc.SendTestSMS(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fake.sent) != 1 || len(provider.sent) != 0 || repo.alerts[0].Level != UpstreamBalanceAlertLevelTest {
		t.Fatalf("sent=%v sms=%d alerts=%+v", fake.sent, len(provider.sent), repo.alerts)
	}

	svc2, _, _, _ := newTestOpsService(t, nil)
	cfg, _ := svc2.loadConfig(context.Background())
	cfg.Channel = UpstreamBalanceChannelServerChan
	cfg.SMS.AccessKeySecret = ""
	if _, err := svc2.UpdateConfig(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := svc2.SendTestSMS(context.Background()); infraerrors.Reason(err) != "UPSTREAM_BALANCE_SERVERCHAN_NOT_CONFIGURED" {
		t.Fatalf("err = %v", err)
	}
}

func sub2apiSnap(balance float64) *UpstreamBalanceResult {
	return okSnap(UpstreamBalanceSourceSub2API, balance, nil)
}

func TestUpstreamWalletMerging(t *testing.T) {
	accounts := []*Account{
		relayAccount(1, "hz-a", "https://api.hzapi.vip", sub2apiSnap(31.84)),
		relayAccount(2, "hz-b", "https://vip.hzapi.vip/v1", sub2apiSnap(31.85)), // 子域不同、差 1 分 → 合并
		relayAccount(3, "hz-other", "https://api.hzapi.vip", sub2apiSnap(80)),   // 同站点不同余额 → 不合并
		relayAccount(4, "newapi-a", "https://pp.dog", okSnap(UpstreamBalanceSourceNewAPIKey, 35.72, fptr(100))),
		relayAccount(5, "newapi-b", "https://pp.dog", okSnap(UpstreamBalanceSourceNewAPIKey, 35.72, fptr(100))), // key 级额度 → 不合并
		relayAccount(6, "sp-top", "https://supeai.top", sub2apiSnap(23.91)),
		relayAccount(7, "sp-cc", "https://supeai.cc", sub2apiSnap(23.91)), // 跨域名 → 靠标签
	}
	accounts[5].Extra[UpstreamWalletExtraKey] = "SupeAI"
	accounts[6].Extra[UpstreamWalletExtraKey] = " SupeAI "

	keys := upstreamWalletKeys(accounts)
	if keys[1] != keys[2] || keys[1] == keys[3] {
		t.Fatalf("hzapi keys = %q %q %q", keys[1], keys[2], keys[3])
	}
	if keys[4] == keys[5] {
		t.Fatalf("new-api keys merged: %q", keys[4])
	}
	if keys[6] != keys[7] || keys[6] != "label:supeai" {
		t.Fatalf("label keys = %q %q", keys[6], keys[7])
	}
	if keys[1] != "wallet:hzapi.vip:1" {
		t.Fatalf("key not stable on min id: %q", keys[1])
	}

	wallets := buildUpstreamWallets(upstreamForecastInput{Accounts: accounts, Now: forecastNow, Thresholds: UpstreamBalanceThresholds{WarnDays: 3, CriticalDays: 1, MinBalance: 10, TargetDays: 7}})
	if len(wallets) != 5 {
		t.Fatalf("wallets = %d, want 5", len(wallets))
	}
	for _, w := range wallets {
		if w.Key == "label:supeai" && (w.Label != "SupeAI" || len(w.Members) != 2) {
			t.Fatalf("label wallet = %+v", w)
		}
	}
}

func TestIdleLowBalanceIsWarnNotCritical(t *testing.T) {
	idle := relayAccount(1, "idle", "https://q.example.com", okSnap(UpstreamBalanceSourceNewAPIKey, 4.94, fptr(100)))
	wallets := buildUpstreamWallets(upstreamForecastInput{Accounts: []*Account{idle}, Now: forecastNow, Thresholds: UpstreamBalanceThresholds{WarnDays: 3, CriticalDays: 1, MinBalance: 10, TargetDays: 7}})
	if len(wallets) != 1 || !wallets[0].Idle || wallets[0].Status != UpstreamWalletStatusWarn {
		t.Fatalf("wallet = %+v", wallets[0])
	}
}

func TestSetWalletLabel(t *testing.T) {
	a := *relayAccount(1, "a", "https://a.example.com", nil)
	svc, _, accountRepo, _ := newTestOpsService(t, []Account{a})
	if err := svc.SetWalletLabel(context.Background(), []int64{1}, strings.Repeat("长", 65)); err == nil {
		t.Fatal("long label accepted")
	}
	if err := svc.SetWalletLabel(context.Background(), nil, "x"); err == nil {
		t.Fatal("empty ids accepted")
	}
	if err := svc.SetWalletLabel(context.Background(), []int64{1}, " SupeAI "); err != nil {
		t.Fatal(err)
	}
	if got := accountRepo.extraUpdates[1][UpstreamWalletExtraKey]; got != "SupeAI" {
		t.Fatalf("extra = %v", accountRepo.extraUpdates[1])
	}
}
