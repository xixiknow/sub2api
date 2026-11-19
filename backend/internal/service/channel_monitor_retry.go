package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// fork 扩展：渠道监控「尽量全绿」重试循环。
//
// 网关同请求内的 failover 只覆盖上游 4xx/5xx；慢响应（degraded）、200 但 challenge 答错（failed）、
// 客户端超时等网关视为成功的情况，由这里在监控侧换账号重试：
//   - 每次尝试带签名探测头：软排除本轮表现差的账号 + 优选上一轮最佳账号；
//   - 分组只有一个账号 / 外部渠道（网关不回传账号）时退化为同目标多次重试；
//   - 拿到 operational 立即停止；否则取所有尝试中最好的一次写历史；
//   - challenge 答错的账号回灌健康追踪器（真实流量也会避开）。

const (
	defaultMonitorProbeMaxRetries    = 3
	defaultMonitorProbeBudgetSeconds = 90
	minMonitorProbeMaxRetries        = 0
	maxMonitorProbeMaxRetries        = 5
	minMonitorProbeBudgetSeconds     = 30
	maxMonitorProbeBudgetSeconds     = 300
	monitorProbeMinAttemptTimeout    = 5 * time.Second
)

// monitorProbeRetryBackoffs 第 n 次重试前的等待（超出长度取最后一个）。
var monitorProbeRetryBackoffs = []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second}

// MonitorProbeRetryPolicy 重试预算（来自全局设置）。
type MonitorProbeRetryPolicy struct {
	MaxRetries int           // 首次之外的最多重试次数
	Budget     time.Duration // 单个模型所有尝试的总耗时上限
}

func normalizeMonitorProbeRetryPolicy(maxRetries, budgetSeconds int) MonitorProbeRetryPolicy {
	if maxRetries < minMonitorProbeMaxRetries {
		maxRetries = minMonitorProbeMaxRetries
	}
	if maxRetries > maxMonitorProbeMaxRetries {
		maxRetries = maxMonitorProbeMaxRetries
	}
	if budgetSeconds < minMonitorProbeBudgetSeconds || budgetSeconds > maxMonitorProbeBudgetSeconds {
		budgetSeconds = defaultMonitorProbeBudgetSeconds
	}
	return MonitorProbeRetryPolicy{MaxRetries: maxRetries, Budget: time.Duration(budgetSeconds) * time.Second}
}

// parseMonitorProbeMaxRetries 解析设置值；未配置 / 非法时取默认 3，越界钳制到 [0,5]。
func parseMonitorProbeMaxRetries(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return defaultMonitorProbeMaxRetries
	}
	return normalizeMonitorProbeRetryPolicy(v, defaultMonitorProbeBudgetSeconds).MaxRetries
}

// parseMonitorProbeBudgetSeconds 解析设置值；未配置 / 非法时取默认 90，越界钳制到 [30,300]。
func parseMonitorProbeBudgetSeconds(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return defaultMonitorProbeBudgetSeconds
	}
	if v < minMonitorProbeBudgetSeconds {
		return minMonitorProbeBudgetSeconds
	}
	if v > maxMonitorProbeBudgetSeconds {
		return maxMonitorProbeBudgetSeconds
	}
	return v
}

// monitorBestAccounts 记住每个 (monitor, model) 上一次表现最好的账号，下一轮优先探测。
// 进程内即可：重启后第一轮按正常调度，随后立刻恢复。
var monitorBestAccounts sync.Map // key string -> int64

func monitorBestAccountKey(monitorID int64, model string) string {
	return fmt.Sprintf("%d|%s", monitorID, model)
}

func loadMonitorBestAccount(monitorID int64, model string) int64 {
	if v, ok := monitorBestAccounts.Load(monitorBestAccountKey(monitorID, model)); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

func storeMonitorBestAccount(monitorID int64, model string, accountID int64) {
	key := monitorBestAccountKey(monitorID, model)
	if accountID > 0 {
		monitorBestAccounts.Store(key, accountID)
		return
	}
	monitorBestAccounts.Delete(key)
}

// monitorStatusRank 状态优劣：数值越大越好。
func monitorStatusRank(status string) int {
	switch status {
	case MonitorStatusOperational:
		return 3
	case MonitorStatusDegraded:
		return 2
	case MonitorStatusFailed:
		return 1
	default:
		return 0
	}
}

func latencyOf(r *CheckResult) int {
	if r == nil || r.LatencyMs == nil {
		return int(^uint(0) >> 1)
	}
	return *r.LatencyMs
}

// betterMonitorResult a 是否优于 b：先比状态，同状态比延迟。
func betterMonitorResult(a, b *CheckResult) bool {
	if b == nil {
		return a != nil
	}
	if a == nil {
		return false
	}
	ra, rb := monitorStatusRank(a.Status), monitorStatusRank(b.Status)
	if ra != rb {
		return ra > rb
	}
	return latencyOf(a) < latencyOf(b)
}

type monitorProbeAttempt struct {
	result    *CheckResult
	accountID int64
}

// monitorProbeCheckFunc 单次尝试；测试中可替换。
type monitorProbeCheckFunc func(ctx context.Context, opts *CheckOptions) *CheckResult

func monitorAttemptLabel(a monitorProbeAttempt) string {
	who := "?"
	if a.accountID > 0 {
		who = fmt.Sprintf("#%d", a.accountID)
	}
	lat := ""
	if a.result.LatencyMs != nil {
		lat = fmt.Sprintf(" %.1fs", float64(*a.result.LatencyMs)/1000)
	}
	return who + " " + a.result.Status + lat
}

// summarizeMonitorAttempts 在最终结果 message 前加上重试轨迹，便于管理员看到真实过程。
func summarizeMonitorAttempts(best *CheckResult, attempts []monitorProbeAttempt) {
	if len(attempts) <= 1 {
		return
	}
	labels := make([]string, 0, len(attempts))
	for _, a := range attempts {
		labels = append(labels, monitorAttemptLabel(a))
	}
	trail := fmt.Sprintf("retried %d: %s", len(attempts)-1, strings.Join(labels, " -> "))
	if strings.TrimSpace(best.Message) == "" {
		best.Message = truncateMessage(trail)
		return
	}
	best.Message = truncateMessage(trail + " | " + best.Message)
}

// reportMonitorAttemptHealth 只回灌网关视为成功、但探测判定为账号问题的情况（challenge 答错）。
// 成功请求的延迟已由计费链路（gateway_usage_billing）上报，上游错误已由 failover 上报，避免双计。
func reportMonitorAttemptHealth(a monitorProbeAttempt) {
	if a.accountID <= 0 || a.result == nil {
		return
	}
	if a.result.Status == MonitorStatusFailed {
		ReportAccountHealth(a.accountID, false, nil)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// runCheckWithRetry 对单个模型执行带换号重试的检测。
func runCheckWithRetry(ctx context.Context, monitorID int64, model, apiKey string, baseOpts *CheckOptions, policy MonitorProbeRetryPolicy, check monitorProbeCheckFunc) *CheckResult {
	deadline := time.Now().Add(policy.Budget)
	excluded := make([]int64, 0, policy.MaxRetries+1)
	prefer := loadMonitorBestAccount(monitorID, model)
	attempts := make([]monitorProbeAttempt, 0, policy.MaxRetries+1)
	var best *CheckResult
	bestAccount := int64(0)

	for i := 0; i <= policy.MaxRetries; i++ {
		if i > 0 {
			backoff := monitorProbeRetryBackoffs[min(i-1, len(monitorProbeRetryBackoffs)-1)]
			if time.Until(deadline) < backoff+monitorProbeMinAttemptTimeout {
				break
			}
			if !sleepCtx(ctx, backoff) {
				break
			}
		}
		remaining := time.Until(deadline)
		if remaining < monitorProbeMinAttemptTimeout && i > 0 {
			break
		}

		opts := &CheckOptions{}
		if baseOpts != nil {
			*opts = *baseOpts
		}
		opts.servedAccountID = 0
		opts.probeHeaders = BuildProbeHeaders(apiKey, ProbeHints{Exclude: excluded, Prefer: prefer}, time.Now())

		attemptCtx, cancel := context.WithTimeout(ctx, min(monitorRequestTimeout, remaining))
		res := check(attemptCtx, opts)
		cancel()
		if res == nil {
			continue
		}
		attempt := monitorProbeAttempt{result: res, accountID: opts.servedAccountID}
		attempts = append(attempts, attempt)
		reportMonitorAttemptHealth(attempt)

		if betterMonitorResult(res, best) {
			best = res
			bestAccount = attempt.accountID
		}
		if res.Status == MonitorStatusOperational || ctx.Err() != nil {
			break
		}
		// 表现差的账号本轮软排除；优选只在第一次尝试生效。
		if attempt.accountID > 0 {
			excluded = append(excluded, attempt.accountID)
		}
		prefer = 0
	}

	if best == nil {
		return &CheckResult{Model: model, Status: MonitorStatusError, CheckedAt: time.Now(), Message: "probe aborted"}
	}
	if monitorStatusRank(best.Status) >= monitorStatusRank(MonitorStatusDegraded) {
		storeMonitorBestAccount(monitorID, model, bestAccount)
	} else {
		storeMonitorBestAccount(monitorID, model, 0)
	}
	summarizeMonitorAttempts(best, attempts)
	return best
}
