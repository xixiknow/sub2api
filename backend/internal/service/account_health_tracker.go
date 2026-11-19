package service

import (
	"math"
	mathrand "math/rand"
	"sync"
	"time"
)

// AccountHealthTracker 记录每个账号的实时表现（进程内），用于在同一 priority 档内
// 让表现好的账号（低错误率、低首字延迟、未被智力测试判定降智）获得更多流量。
//
// 设计要点：
//   - 错误率与首字延迟(TTFT)使用 EWMA（α=0.2），与 OpenAI 高级调度器一致；
//   - 错误率随时间向 0 衰减（半衰期 healthErrorHalfLife），避免流量被压低的账号永远翻不了身；
//   - 选择采用加权随机而不是硬排序，且每个账号保留最低权重，持续获得少量探测流量；
//   - 惩罚（如智力测试失败）以系数形式叠加，可设置过期时间。
type AccountHealthTracker struct {
	mu        sync.RWMutex
	stats     map[int64]*accountHealthStat
	penalties map[int64]accountHealthPenalty
	now       func() time.Time
}

type accountHealthStat struct {
	errorEWMA  float64
	ttftEWMA   float64
	hasTTFT    bool
	samples    int64
	lastUpdate time.Time
}

type accountHealthPenalty struct {
	Factor float64
	Until  time.Time // 零值表示不过期
	Reason string
}

// AccountHealthSnapshot 是对外展示用的只读快照。
type AccountHealthSnapshot struct {
	AccountID     int64      `json:"account_id"`
	Score         float64    `json:"score"`
	ErrorRate     float64    `json:"error_rate"`
	TTFTMs        *float64   `json:"ttft_ms,omitempty"`
	Samples       int64      `json:"samples"`
	PenaltyFactor float64    `json:"penalty_factor"`
	PenaltyReason string     `json:"penalty_reason,omitempty"`
	PenaltyUntil  *time.Time `json:"penalty_until,omitempty"`
}

const (
	healthEWMAAlpha       = 0.2
	healthErrorHalfLife   = 10 * time.Minute
	healthMinWeight       = 0.03
	healthErrorExponent   = 2.0
	healthTTFTMinFactor   = 0.35
	healthMinSamplesTTFT  = 3
	healthStatsMaxEntries = 100000
)

// defaultAccountHealthTracker 是进程级单例：Anthropic/Gemini 网关与 OpenAI 网关共用。
var defaultAccountHealthTracker = NewAccountHealthTracker()

// DefaultAccountHealthTracker 返回进程级共享的健康追踪器。
func DefaultAccountHealthTracker() *AccountHealthTracker { return defaultAccountHealthTracker }

func NewAccountHealthTracker() *AccountHealthTracker {
	return &AccountHealthTracker{
		stats:     make(map[int64]*accountHealthStat),
		penalties: make(map[int64]accountHealthPenalty),
		now:       time.Now,
	}
}

// ReportAccountHealth 是 handler 层的便捷入口。
func ReportAccountHealth(accountID int64, success bool, firstTokenMs *int) {
	defaultAccountHealthTracker.Report(accountID, success, firstTokenMs)
}

// Report 记录一次请求结果。只应对「账号原因」的失败调用 success=false。
func (t *AccountHealthTracker) Report(accountID int64, success bool, firstTokenMs *int) {
	if t == nil || accountID <= 0 {
		return
	}
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.stats[accountID]
	if st == nil {
		if len(t.stats) >= healthStatsMaxEntries {
			return
		}
		st = &accountHealthStat{lastUpdate: now}
		t.stats[accountID] = st
	}
	st.errorEWMA = decayedError(st.errorEWMA, now.Sub(st.lastUpdate))
	sample := 0.0
	if !success {
		sample = 1.0
	}
	st.errorEWMA = healthEWMAAlpha*sample + (1-healthEWMAAlpha)*st.errorEWMA
	if success && firstTokenMs != nil && *firstTokenMs > 0 {
		v := float64(*firstTokenMs)
		if !st.hasTTFT {
			st.ttftEWMA = v
			st.hasTTFT = true
		} else {
			st.ttftEWMA = healthEWMAAlpha*v + (1-healthEWMAAlpha)*st.ttftEWMA
		}
	}
	st.samples++
	st.lastUpdate = now
}

// SetPenalty 为账号设置惩罚系数（0~1，越小越少被选中）。factor>=1 等价于清除。
func (t *AccountHealthTracker) SetPenalty(accountID int64, factor float64, until time.Time, reason string) {
	if t == nil || accountID <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if factor >= 1 || math.IsNaN(factor) {
		delete(t.penalties, accountID)
		return
	}
	if factor < 0 {
		factor = 0
	}
	t.penalties[accountID] = accountHealthPenalty{Factor: factor, Until: until, Reason: reason}
}

// ClearPenalty 清除账号惩罚。
func (t *AccountHealthTracker) ClearPenalty(accountID int64) {
	t.SetPenalty(accountID, 1, time.Time{}, "")
}

func decayedError(errorEWMA float64, elapsed time.Duration) float64 {
	if elapsed <= 0 || errorEWMA <= 0 {
		return errorEWMA
	}
	return errorEWMA * math.Pow(0.5, float64(elapsed)/float64(healthErrorHalfLife))
}

type healthView struct {
	errorRate float64
	ttft      float64
	hasTTFT   bool
	samples   int64
	penalty   accountHealthPenalty
	penalized bool
}

func (t *AccountHealthTracker) viewLocked(accountID int64, now time.Time) healthView {
	var v healthView
	if st := t.stats[accountID]; st != nil {
		v.errorRate = clamp01(decayedError(st.errorEWMA, now.Sub(st.lastUpdate)))
		v.ttft = st.ttftEWMA
		v.hasTTFT = st.hasTTFT && st.samples >= healthMinSamplesTTFT
		v.samples = st.samples
	}
	if p, ok := t.penalties[accountID]; ok {
		if p.Until.IsZero() || now.Before(p.Until) {
			v.penalty = p
			v.penalized = true
		}
	}
	return v
}

func healthScoreFromView(v healthView, medianTTFT float64) float64 {
	score := math.Pow(1-v.errorRate, healthErrorExponent)
	if v.hasTTFT && medianTTFT > 0 && v.ttft > medianTTFT {
		// 比同档中位数慢的账号按比例降权，下限 healthTTFTMinFactor
		f := medianTTFT / v.ttft
		if f < healthTTFTMinFactor {
			f = healthTTFTMinFactor
		}
		score *= f
	}
	if v.penalized {
		score *= v.penalty.Factor
	}
	return score
}

// Scores 计算一组候选账号的健康分（0~1）。TTFT 以候选集内中位数为基准做相对比较。
func (t *AccountHealthTracker) Scores(accountIDs []int64) []float64 {
	out := make([]float64, len(accountIDs))
	if t == nil {
		for i := range out {
			out[i] = 1
		}
		return out
	}
	now := t.now()
	t.mu.RLock()
	views := make([]healthView, len(accountIDs))
	for i, id := range accountIDs {
		views[i] = t.viewLocked(id, now)
	}
	t.mu.RUnlock()
	median := medianTTFTOf(views)
	for i, v := range views {
		out[i] = healthScoreFromView(v, median)
	}
	return out
}

func medianTTFTOf(views []healthView) float64 {
	vals := make([]float64, 0, len(views))
	for _, v := range views {
		if v.hasTTFT {
			vals = append(vals, v.ttft)
		}
	}
	if len(vals) == 0 {
		return 0
	}
	// 插入排序：候选集很小
	for i := 1; i < len(vals); i++ {
		for j := i; j > 0 && vals[j] < vals[j-1]; j-- {
			vals[j], vals[j-1] = vals[j-1], vals[j]
		}
	}
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

// Snapshot 返回账号的健康快照（用于后台展示）。
func (t *AccountHealthTracker) Snapshot(accountID int64) AccountHealthSnapshot {
	snap := AccountHealthSnapshot{AccountID: accountID, Score: 1, PenaltyFactor: 1}
	if t == nil {
		return snap
	}
	now := t.now()
	t.mu.RLock()
	v := t.viewLocked(accountID, now)
	t.mu.RUnlock()
	snap.ErrorRate = v.errorRate
	snap.Samples = v.samples
	if v.hasTTFT {
		ttft := v.ttft
		snap.TTFTMs = &ttft
	}
	if v.penalized {
		snap.PenaltyFactor = v.penalty.Factor
		snap.PenaltyReason = v.penalty.Reason
		if !v.penalty.Until.IsZero() {
			until := v.penalty.Until
			snap.PenaltyUntil = &until
		}
	}
	snap.Score = healthScoreFromView(v, 0)
	return snap
}

// PickWeighted 按 健康分 × 空闲度 做加权随机，返回选中下标；n==0 返回 -1。
// loadRates 为 0~100 的负载率（可为 nil）。
func (t *AccountHealthTracker) PickWeighted(accountIDs []int64, loadRates []int) int {
	n := len(accountIDs)
	if n == 0 {
		return -1
	}
	if n == 1 {
		return 0
	}
	scores := t.Scores(accountIDs)
	weights := make([]float64, n)
	total := 0.0
	for i := range accountIDs {
		idle := 1.0
		if loadRates != nil && i < len(loadRates) {
			idle = 1 - float64(loadRates[i])/100
			if idle < 0.05 {
				idle = 0.05
			}
		}
		w := scores[i] * idle
		if w < healthMinWeight {
			w = healthMinWeight
		}
		weights[i] = w
		total += w
	}
	r := mathrand.Float64() * total
	for i, w := range weights {
		r -= w
		if r < 0 {
			return i
		}
	}
	return n - 1
}
