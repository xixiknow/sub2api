package service

import (
	"context"
	"log/slog"
)

// fork 扩展：渠道监控探测请求的选号提示（优选账号 / 软排除），以及回传实际选中账号。
// 只在请求 ctx 带有已验签的探测提示时生效；普通流量零开销（一次 ctx.Value 查找）。

type probeSelectFunc func(ctx context.Context, excluded map[int64]struct{}) (*AccountSelectionResult, error)

func mergeProbeExclusions(base map[int64]struct{}, extra []int64) (map[int64]struct{}, int) {
	out := make(map[int64]struct{}, len(base)+len(extra))
	for id := range base {
		out[id] = struct{}{}
	}
	added := 0
	for _, id := range extra {
		if _, ok := out[id]; ok {
			continue
		}
		out[id] = struct{}{}
		added++
	}
	return out, added
}

// probePreferExclusions 返回「除优选账号外，分组内其他可调度账号」的排除集；
// 优选账号不在分组内 / 分组只有它一个 / 查询失败时返回 nil。
func probePreferExclusions(ctx context.Context, repo AccountRepository, groupID *int64, prefer int64, base map[int64]struct{}) map[int64]struct{} {
	if repo == nil || groupID == nil || prefer <= 0 {
		return nil
	}
	if _, excluded := base[prefer]; excluded {
		return nil
	}
	accounts, err := repo.ListSchedulableByGroupID(ctx, *groupID)
	if err != nil || len(accounts) < 2 {
		return nil
	}
	found := false
	others := make([]int64, 0, len(accounts))
	for i := range accounts {
		if accounts[i].ID == prefer {
			found = true
			continue
		}
		others = append(others, accounts[i].ID)
	}
	if !found {
		return nil
	}
	ex, _ := mergeProbeExclusions(base, others)
	return ex
}

// selectWithProbeHints 依次尝试：优选账号 → 软排除后的其余账号 → 原始排除集（兜底，单账号分组走这里）。
// 任一步出错都回退到下一步，最终结果与不带提示时一致，不会因提示导致「无可用账号」。
func selectWithProbeHints(ctx context.Context, st *probeSelectionState, repo AccountRepository, groupID *int64, base map[int64]struct{}, selectFn probeSelectFunc) (*AccountSelectionResult, error) {
	ctx = markProbeSelectionConsumed(ctx)
	record := func(res *AccountSelectionResult) {
		if res != nil && res.Account != nil && st.record != nil {
			st.record.served.Store(res.Account.ID)
		}
	}

	if ex := probePreferExclusions(ctx, repo, groupID, st.hints.Prefer, base); ex != nil {
		if res, err := selectFn(ctx, ex); err == nil && res != nil && res.Account != nil {
			record(res)
			return res, nil
		} else if err != nil {
			slog.Debug("channel_monitor_probe.prefer_unavailable", "prefer", st.hints.Prefer, "error", err)
		}
	}

	if len(st.hints.Exclude) > 0 {
		if ex, added := mergeProbeExclusions(base, st.hints.Exclude); added > 0 {
			if res, err := selectFn(ctx, ex); err == nil && res != nil && res.Account != nil {
				record(res)
				return res, nil
			} else if err != nil {
				slog.Debug("channel_monitor_probe.soft_exclude_fallback", "excluded", st.hints.Exclude, "error", err)
			}
		}
	}

	res, err := selectFn(ctx, base)
	if err == nil {
		record(res)
	}
	return res, err
}

// selectAccountWithLoadAwarenessProbe Anthropic / Gemini 网关入口的探测钩子。
func (s *GatewayService) selectAccountWithLoadAwarenessProbe(ctx context.Context, st *probeSelectionState, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}, metadataUserID string, sub2apiUserID int64) (*AccountSelectionResult, error) {
	return selectWithProbeHints(ctx, st, s.accountRepo, groupID, excludedIDs, func(ctx context.Context, excluded map[int64]struct{}) (*AccountSelectionResult, error) {
		return s.SelectAccountWithLoadAwareness(ctx, groupID, sessionHash, requestedModel, excluded, metadataUserID, sub2apiUserID)
	})
}

// selectAccountWithSchedulerProbe OpenAI 兼容网关调度入口的探测钩子。
func (s *OpenAIGatewayService) selectAccountWithSchedulerProbe(
	ctx context.Context,
	st *probeSelectionState,
	groupID *int64,
	excludedIDs map[int64]struct{},
	call func(ctx context.Context, excluded map[int64]struct{}) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error),
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	var decision OpenAIAccountScheduleDecision
	res, err := selectWithProbeHints(ctx, st, s.accountRepo, groupID, excludedIDs, func(ctx context.Context, excluded map[int64]struct{}) (*AccountSelectionResult, error) {
		r, d, e := call(ctx, excluded)
		decision = d
		return r, e
	})
	return res, decision, err
}

// ProbeServedAccountForTest 供跨包测试使用：若 ctx 带有已验签的探测提示，
// 用一个总是选中 accountID 的 selector 走一遍装饰器，返回是否生效。
func ProbeServedAccountForTest(ctx context.Context, accountID int64) bool {
	st := probeSelectionStateFrom(ctx)
	if st == nil {
		return false
	}
	_, err := selectWithProbeHints(ctx, st, nil, nil, nil, func(context.Context, map[int64]struct{}) (*AccountSelectionResult, error) {
		return &AccountSelectionResult{Account: &Account{ID: accountID}}, nil
	})
	return err == nil
}
