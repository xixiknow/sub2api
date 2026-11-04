package service

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	UpstreamWalletStatusExhausted = "exhausted"
	UpstreamWalletStatusCritical  = "critical"
	UpstreamWalletStatusWarn      = "warn"
	UpstreamWalletStatusUnknown   = "unknown"
	UpstreamWalletStatusOK        = "ok"
	UpstreamWalletStatusUnlimited = "unlimited"

	UpstreamBurnSourceSnapshot = "snapshot"
	UpstreamBurnSourceUsage    = "usage"

	// 快照覆盖时长不足时不可靠，改用本平台用量兜底。
	upstreamBurnMinSpan = 6 * time.Hour
	// 快照超过该时长视为过期：不触发提醒与停调度。
	upstreamBalanceStaleAfter = 3 * time.Hour

	UpstreamBalanceAlertExtraKey = "upstream_balance_alert"
)

// UpstreamBalancePoint 是一条余额历史快照。
type UpstreamBalancePoint struct {
	At      time.Time `json:"t"`
	Balance float64   `json:"balance"`
}

// UpstreamCostBucket 是按天汇总的本平台上游成本。
type UpstreamCostBucket struct {
	Date string  `json:"date"`
	Cost float64 `json:"cost"`
}

// UpstreamBalanceThresholds 是判定告急的阈值。
type UpstreamBalanceThresholds struct {
	WarnDays     float64 `json:"warn_days"`
	CriticalDays float64 `json:"critical_days"`
	MinBalance   float64 `json:"min_balance"`
	TargetDays   float64 `json:"target_days"`
}

// UpstreamBalanceAlertOverride 是账号级阈值覆盖，存于 accounts.extra.upstream_balance_alert。
type UpstreamBalanceAlertOverride struct {
	Disabled     bool     `json:"disabled,omitempty"`
	WarnDays     *float64 `json:"warn_days,omitempty"`
	CriticalDays *float64 `json:"critical_days,omitempty"`
	MinBalance   *float64 `json:"min_balance,omitempty"`
}

func (o *UpstreamBalanceAlertOverride) isEmpty() bool {
	return o == nil || (!o.Disabled && o.WarnDays == nil && o.CriticalDays == nil && o.MinBalance == nil)
}

// DecodeUpstreamBalanceAlertOverride 读取账号级阈值覆盖。
func DecodeUpstreamBalanceAlertOverride(extra map[string]any) *UpstreamBalanceAlertOverride {
	var out UpstreamBalanceAlertOverride
	if !decodeExtraJSON(extra, UpstreamBalanceAlertExtraKey, &out) || out.isEmpty() {
		return nil
	}
	return &out
}

// UpstreamWalletMember 是共用同一上游钱包的一个账号。
type UpstreamWalletMember struct {
	AccountID   int64                         `json:"account_id"`
	Name        string                        `json:"name"`
	Platform    string                        `json:"platform"`
	GroupIDs    []int64                       `json:"group_ids,omitempty"`
	Schedulable bool                          `json:"schedulable"`
	PausedByUs  bool                          `json:"paused_by_balance"`
	Cost24h     float64                       `json:"cost_24h"`
	Cost7d      float64                       `json:"cost_7d"`
	Override    *UpstreamBalanceAlertOverride `json:"override,omitempty"`
}

// UpstreamWallet 是一个上游钱包（通常一个账号一个；sub2api 同一用户的多个 key 共用余额）。
type UpstreamWallet struct {
	Key              string                    `json:"key"`
	PrimaryAccountID int64                     `json:"primary_account_id"`
	Site             string                    `json:"site"`
	Source           string                    `json:"source,omitempty"`
	Members          []UpstreamWalletMember    `json:"members"`
	FetchStatus      string                    `json:"fetch_status"`
	FetchError       string                    `json:"fetch_error,omitempty"`
	FetchedAt        *time.Time                `json:"fetched_at,omitempty"`
	Stale            bool                      `json:"stale"`
	Balance          *float64                  `json:"balance,omitempty"`
	Used             *float64                  `json:"used,omitempty"`
	Limit            *float64                  `json:"limit,omitempty"`
	Unlimited        bool                      `json:"unlimited"`
	Currency         string                    `json:"currency,omitempty"`
	DailyBurn        *float64                  `json:"daily_burn,omitempty"`
	DailyBurnSource  string                    `json:"daily_burn_source,omitempty"`
	Burn7d           *float64                  `json:"burn_7d,omitempty"`
	Burn7dSource     string                    `json:"burn_7d_source,omitempty"`
	DaysLeft         *float64                  `json:"days_left,omitempty"`
	ExhaustAt        *time.Time                `json:"exhaust_at,omitempty"`
	SuggestRecharge  *float64                  `json:"suggest_recharge,omitempty"`
	TrafficShare     float64                   `json:"traffic_share"`
	Status           string                    `json:"status"`
	AlertDisabled    bool                      `json:"alert_disabled"`
	Thresholds       UpstreamBalanceThresholds `json:"thresholds"`

	snapshotBurn24h *float64
	snapshotBurn7d  *float64
}

// UpstreamBalanceOverviewSummary 是总览 KPI。
type UpstreamBalanceOverviewSummary struct {
	Wallets      int      `json:"wallets"`
	TotalBalance float64  `json:"total_balance"`
	DailyBurn    float64  `json:"daily_burn"`
	DaysLeft     *float64 `json:"days_left,omitempty"`
	Exhausted    int      `json:"exhausted"`
	Critical     int      `json:"critical"`
	Warn         int      `json:"warn"`
	Unknown      int      `json:"unknown"`
	Unlimited    int      `json:"unlimited"`
}

// UpstreamBalanceOverview 是上游余额维护页的数据。
type UpstreamBalanceOverview struct {
	Summary     UpstreamBalanceOverviewSummary `json:"summary"`
	Wallets     []UpstreamWallet               `json:"wallets"`
	Thresholds  UpstreamBalanceThresholds      `json:"thresholds"`
	GeneratedAt time.Time                      `json:"generated_at"`
}

// upstreamForecastInput 是一次预测所需的全部数据。
type upstreamForecastInput struct {
	Accounts   []*Account
	Snapshots  map[int64][]UpstreamBalancePoint // 近 7 天，按时间升序
	Cost24h    map[int64]float64
	Cost7d     map[int64]float64
	Thresholds UpstreamBalanceThresholds
	Now        time.Time
}

// buildUpstreamWallets 归并共享钱包、计算消耗速度/可用天数/状态并按充值优先级排序。
func buildUpstreamWallets(in upstreamForecastInput) []UpstreamWallet {
	byKey := make(map[string]*UpstreamWallet)
	order := make([]string, 0, len(in.Accounts))
	snaps := make(map[string]*UpstreamBalanceResult)
	for _, account := range in.Accounts {
		snap := DecodeUpstreamBalanceSnapshot(account.Extra)
		site := upstreamBalanceSite(account.GetCredential("base_url"))
		key := upstreamWalletKey(account, site, snap)
		wallet, ok := byKey[key]
		if !ok {
			wallet = &UpstreamWallet{Key: key, PrimaryAccountID: account.ID, Site: site}
			byKey[key] = wallet
			order = append(order, key)
		}
		if account.ID < wallet.PrimaryAccountID {
			wallet.PrimaryAccountID = account.ID
		}
		// 同一钱包取最新的快照。
		if snap != nil && (snaps[key] == nil || snap.FetchedAt.After(snaps[key].FetchedAt)) {
			snaps[key] = snap
		}
		wallet.Members = append(wallet.Members, UpstreamWalletMember{
			AccountID:   account.ID,
			Name:        account.Name,
			Platform:    account.Platform,
			GroupIDs:    account.GroupIDs,
			Schedulable: account.IsSchedulable(),
			PausedByUs:  upstreamBalancePausedByUs(account, in.Now),
			Cost24h:     in.Cost24h[account.ID],
			Cost7d:      in.Cost7d[account.ID],
			Override:    DecodeUpstreamBalanceAlertOverride(account.Extra),
		})
		// 共享钱包内各账号看到的是同一份余额下降，取最大值而不是相加。
		wallet.snapshotBurn24h = maxFloatPtr(wallet.snapshotBurn24h, upstreamBurnFromPoints(in.Snapshots[account.ID], in.Now.Add(-24*time.Hour)))
		wallet.snapshotBurn7d = maxFloatPtr(wallet.snapshotBurn7d, upstreamBurnFromPoints(in.Snapshots[account.ID], in.Now.Add(-7*24*time.Hour)))
	}

	var total7d float64
	for _, key := range order {
		for _, m := range byKey[key].Members {
			total7d += m.Cost7d
		}
	}
	wallets := make([]UpstreamWallet, 0, len(order))
	for _, key := range order {
		wallet := byKey[key]
		sort.Slice(wallet.Members, func(a, b int) bool { return wallet.Members[a].AccountID < wallet.Members[b].AccountID })
		finalizeUpstreamWallet(wallet, snaps[key], in, total7d)
		wallets = append(wallets, *wallet)
	}
	sortUpstreamWallets(wallets)
	return wallets
}

func finalizeUpstreamWallet(w *UpstreamWallet, snap *UpstreamBalanceResult, in upstreamForecastInput, total7d float64) {
	w.Thresholds = resolveUpstreamThresholds(in.Thresholds, w.Members)
	w.AlertDisabled = true
	var usage24h, usage7d float64
	for _, m := range w.Members {
		usage24h += m.Cost24h
		usage7d += m.Cost7d
		if m.Override == nil || !m.Override.Disabled {
			w.AlertDisabled = false
		}
	}
	if total7d > 0 {
		w.TrafficShare = usage7d / total7d
	}
	if snap == nil {
		w.Status = UpstreamWalletStatusUnknown
		w.FetchStatus = ""
		return
	}
	fetchedAt := snap.FetchedAt
	w.FetchedAt = &fetchedAt
	w.FetchStatus = snap.Status
	w.FetchError = snap.Error
	w.Source = snap.Source
	w.Stale = in.Now.Sub(snap.FetchedAt) > upstreamBalanceStaleAfter
	if snap.Status != UpstreamBalanceStatusOK {
		w.Status = UpstreamWalletStatusUnknown
		return
	}
	w.Balance, w.Used, w.Limit, w.Unlimited, w.Currency = snap.Balance, snap.Used, snap.Limit, snap.Unlimited, snap.Currency

	if w.snapshotBurn24h != nil {
		w.DailyBurn, w.DailyBurnSource = w.snapshotBurn24h, UpstreamBurnSourceSnapshot
	} else {
		v := roundUpstreamBalance(usage24h)
		w.DailyBurn, w.DailyBurnSource = &v, UpstreamBurnSourceUsage
	}
	if w.snapshotBurn7d != nil {
		w.Burn7d, w.Burn7dSource = w.snapshotBurn7d, UpstreamBurnSourceSnapshot
	} else {
		v := roundUpstreamBalance(usage7d / 7)
		w.Burn7d, w.Burn7dSource = &v, UpstreamBurnSourceUsage
	}

	if snap.Unlimited || snap.Balance == nil {
		if snap.Unlimited {
			w.Status = UpstreamWalletStatusUnlimited
		} else {
			w.Status = UpstreamWalletStatusUnknown
		}
		return
	}
	balance := *snap.Balance
	burn := *w.DailyBurn
	if balance <= 0 && burn <= 0 {
		zero := 0.0
		w.DaysLeft = &zero
	}
	if burn > 0 {
		days := roundUpstreamBalance(maxFloat(balance, 0) / burn)
		w.DaysLeft = &days
		exhaustAt := in.Now.Add(time.Duration(days * float64(24*time.Hour)))
		w.ExhaustAt = &exhaustAt
		suggest := roundUpstreamBalance(maxFloat(burn*w.Thresholds.TargetDays-balance, 0))
		w.SuggestRecharge = &suggest
	}
	w.Status = upstreamWalletStatus(balance, w.DaysLeft, w.Thresholds)
}

func upstreamWalletStatus(balance float64, daysLeft *float64, t UpstreamBalanceThresholds) string {
	switch {
	case balance <= 0:
		return UpstreamWalletStatusExhausted
	case (daysLeft != nil && *daysLeft < t.CriticalDays) || balance < t.MinBalance/2:
		return UpstreamWalletStatusCritical
	case (daysLeft != nil && *daysLeft < t.WarnDays) || balance < t.MinBalance:
		return UpstreamWalletStatusWarn
	default:
		return UpstreamWalletStatusOK
	}
}

// upstreamWalletStatusRank 越小越需要处理。
func upstreamWalletStatusRank(status string) int {
	switch status {
	case UpstreamWalletStatusExhausted:
		return 0
	case UpstreamWalletStatusCritical:
		return 1
	case UpstreamWalletStatusWarn:
		return 2
	case UpstreamWalletStatusUnknown:
		return 3
	case UpstreamWalletStatusOK:
		return 4
	default:
		return 5
	}
}

// upstreamAlertLevelRank 越大越严重；非告急状态为 0。
func upstreamAlertLevelRank(status string) int {
	switch status {
	case UpstreamWalletStatusWarn:
		return 1
	case UpstreamWalletStatusCritical:
		return 2
	case UpstreamWalletStatusExhausted:
		return 3
	default:
		return 0
	}
}

// sortUpstreamWallets 按充值优先级排序：状态严重度 → 可用天数升序 → 流量占比降序。
func sortUpstreamWallets(wallets []UpstreamWallet) {
	sort.SliceStable(wallets, func(a, b int) bool {
		wa, wb := wallets[a], wallets[b]
		if ra, rb := upstreamWalletStatusRank(wa.Status), upstreamWalletStatusRank(wb.Status); ra != rb {
			return ra < rb
		}
		if (wa.DaysLeft == nil) != (wb.DaysLeft == nil) {
			return wa.DaysLeft != nil
		}
		if wa.DaysLeft != nil && *wa.DaysLeft != *wb.DaysLeft {
			return *wa.DaysLeft < *wb.DaysLeft
		}
		if wa.TrafficShare != wb.TrafficShare {
			return wa.TrafficShare > wb.TrafficShare
		}
		return wa.PrimaryAccountID < wb.PrimaryAccountID
	})
}

func summarizeUpstreamWallets(wallets []UpstreamWallet) UpstreamBalanceOverviewSummary {
	summary := UpstreamBalanceOverviewSummary{Wallets: len(wallets)}
	for _, w := range wallets {
		switch w.Status {
		case UpstreamWalletStatusExhausted:
			summary.Exhausted++
		case UpstreamWalletStatusCritical:
			summary.Critical++
		case UpstreamWalletStatusWarn:
			summary.Warn++
		case UpstreamWalletStatusUnknown:
			summary.Unknown++
		case UpstreamWalletStatusUnlimited:
			summary.Unlimited++
		}
		if w.Unlimited || w.Balance == nil {
			continue
		}
		summary.TotalBalance += maxFloat(*w.Balance, 0)
		if w.DailyBurn != nil {
			summary.DailyBurn += *w.DailyBurn
		}
	}
	summary.TotalBalance = roundUpstreamBalance(summary.TotalBalance)
	summary.DailyBurn = roundUpstreamBalance(summary.DailyBurn)
	if summary.DailyBurn > 0 {
		days := roundUpstreamBalance(summary.TotalBalance / summary.DailyBurn)
		summary.DaysLeft = &days
	}
	return summary
}

// upstreamBurnFromPoints 用余额快照计算日消耗：只累加余额下降，上升视为充值忽略。
// 覆盖时长不足 upstreamBurnMinSpan 时返回 nil。
func upstreamBurnFromPoints(points []UpstreamBalancePoint, since time.Time) *float64 {
	var (
		first, last time.Time
		prev        *float64
		consumed    float64
		count       int
	)
	for _, p := range points {
		if p.At.Before(since) {
			continue
		}
		if count == 0 {
			first = p.At
		}
		last = p.At
		count++
		if prev != nil && *prev > p.Balance {
			consumed += *prev - p.Balance
		}
		balance := p.Balance
		prev = &balance
	}
	span := last.Sub(first)
	if count < 2 || span < upstreamBurnMinSpan {
		return nil
	}
	perDay := roundUpstreamBalance(consumed / span.Hours() * 24)
	return &perDay
}

// resolveUpstreamThresholds 合并全局阈值与账号覆盖；共享钱包内取最保守（最大）的覆盖值。
func resolveUpstreamThresholds(global UpstreamBalanceThresholds, members []UpstreamWalletMember) UpstreamBalanceThresholds {
	out := global
	pick := func(current float64, values []*float64) float64 {
		var best *float64
		for _, v := range values {
			best = maxFloatPtr(best, v)
		}
		if best == nil {
			return current
		}
		return *best
	}
	var warn, critical, minBalance []*float64
	for _, m := range members {
		if m.Override == nil {
			continue
		}
		warn = append(warn, m.Override.WarnDays)
		critical = append(critical, m.Override.CriticalDays)
		minBalance = append(minBalance, m.Override.MinBalance)
	}
	out.WarnDays = pick(out.WarnDays, warn)
	out.CriticalDays = pick(out.CriticalDays, critical)
	out.MinBalance = pick(out.MinBalance, minBalance)
	return out
}

// upstreamWalletKey 识别共享钱包：sub2api 余额模式（无 key 级限额）下，同站点同余额视为同一用户钱包。
func upstreamWalletKey(account *Account, site string, snap *UpstreamBalanceResult) string {
	if snap != nil && snap.Status == UpstreamBalanceStatusOK && snap.Source == UpstreamBalanceSourceSub2API &&
		snap.Limit == nil && snap.Balance != nil && !snap.Unlimited && site != "" {
		return fmt.Sprintf("wallet:%s:%s", site, strconv.FormatFloat(*snap.Balance, 'f', 6, 64))
	}
	return "account:" + strconv.FormatInt(account.ID, 10)
}

func upstreamBalanceSite(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Host)
}

func upstreamBalancePausedByUs(account *Account, now time.Time) bool {
	return account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) &&
		strings.HasPrefix(account.TempUnschedulableReason, upstreamBalanceExhaustedReasonPrefix)
}

func maxFloatPtr(a, b *float64) *float64 {
	if b == nil {
		return a
	}
	if a == nil || *b > *a {
		v := *b
		return &v
	}
	return a
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
