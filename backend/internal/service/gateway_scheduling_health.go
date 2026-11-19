package service

import (
	"math"
	mathrand "math/rand"
)

// fork 扩展：健康分加权调度（Anthropic / Gemini 网关路径）。
// 独立成文件，尽量减少对上游 gateway_scheduling.go 的改动面。

const healthScoreEqualEpsilon = 1e-6

func accountIDsAndLoads(accounts []accountWithLoad) ([]int64, []int) {
	ids := make([]int64, len(accounts))
	loads := make([]int, len(accounts))
	for i, acc := range accounts {
		ids[i] = acc.account.ID
		if acc.loadInfo != nil {
			loads[i] = acc.loadInfo.LoadRate
		}
	}
	return ids, loads
}

func healthScoresAllEqual(scores []float64) bool {
	for i := 1; i < len(scores); i++ {
		if math.Abs(scores[i]-scores[0]) > healthScoreEqualEpsilon {
			return false
		}
	}
	return true
}

// selectByHealthWeighted 在同一 priority 档候选中按「健康分 × 空闲度」加权随机选择。
// 所有候选健康分相同（例如都没有历史数据）时返回 nil，由调用方走原有负载率/LRU 逻辑。
func selectByHealthWeighted(accounts []accountWithLoad) *accountWithLoad {
	if len(accounts) <= 1 {
		return nil
	}
	tracker := DefaultAccountHealthTracker()
	ids, loads := accountIDsAndLoads(accounts)
	if healthScoresAllEqual(tracker.Scores(ids)) {
		return nil
	}
	idx := tracker.PickWeighted(ids, loads)
	if idx < 0 {
		return nil
	}
	return &accounts[idx]
}

// reorderByHealthWithinPriority 对已按 priority 排序的列表，在每个 priority 段内
// 按健康分加权做无放回抽样重排（健康分无差异的段保持原顺序）。
func reorderByHealthWithinPriority(accounts []accountWithLoad) {
	tracker := DefaultAccountHealthTracker()
	start := 0
	for start < len(accounts) {
		end := start + 1
		for end < len(accounts) && accounts[end].account.Priority == accounts[start].account.Priority {
			end++
		}
		if end-start > 1 {
			reorderSegmentByHealth(tracker, accounts[start:end])
		}
		start = end
	}
}

func reorderSegmentByHealth(tracker *AccountHealthTracker, seg []accountWithLoad) {
	ids, loads := accountIDsAndLoads(seg)
	scores := tracker.Scores(ids)
	if healthScoresAllEqual(scores) {
		return
	}
	weights := make([]float64, len(seg))
	for i := range seg {
		idle := 1 - float64(loads[i])/100
		if idle < 0.05 {
			idle = 0.05
		}
		w := scores[i] * idle
		if w < healthMinWeight {
			w = healthMinWeight
		}
		weights[i] = w
	}
	// 加权无放回抽样：逐位选出，交换到前面
	for pos := 0; pos < len(seg)-1; pos++ {
		total := 0.0
		for i := pos; i < len(seg); i++ {
			total += weights[i]
		}
		r := mathrand.Float64() * total
		pick := len(seg) - 1
		for i := pos; i < len(seg); i++ {
			r -= weights[i]
			if r < 0 {
				pick = i
				break
			}
		}
		seg[pos], seg[pick] = seg[pick], seg[pos]
		weights[pos], weights[pick] = weights[pick], weights[pos]
	}
}
