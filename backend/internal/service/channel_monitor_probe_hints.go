package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// fork 扩展：渠道监控探测提示协议。
//
// 监控探测请求走自家网关时，带上签名头告诉网关「本次排除哪些账号 / 优先用哪个账号」，
// 网关在响应头回传实际服务的账号 ID，监控据此换账号重试并记住最佳账号。
//
// 安全性：签名 = HMAC(key, ts|sha256(apiKey)|exclude|prefer)，key 由 JWT secret 派生。
// 签名绑定了请求里的 API Key，即使把头发给外部渠道，对方也无法用自己的 key 重放到本站。
// 验签失败一律静默忽略（当作普通请求），且网关会剥离这些头，不会转发给上游。
const (
	ProbeHeaderSignature     = "X-Sub2API-Probe"
	ProbeHeaderExclude       = "X-Sub2API-Probe-Exclude"
	ProbeHeaderPrefer        = "X-Sub2API-Probe-Prefer"
	ProbeHeaderServedAccount = "X-Sub2API-Probe-Account"

	probeHeaderPrefix   = "x-sub2api-probe"
	probeSignatureTTL   = 60 * time.Second
	probeMaxExcludedIDs = 32
	probeKeyDerivation  = "sub2api-channel-monitor-probe-v1"
)

var probeSigningKey atomic.Pointer[[]byte]

// InitChannelMonitorProbeSigner 用 JWT secret 派生探测签名密钥；进程启动时调用一次。
// 多实例部署共享同一 JWT secret，因此监控与网关不在同一实例也能互认。
func InitChannelMonitorProbeSigner(secret string) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		probeSigningKey.Store(nil)
		return
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(probeKeyDerivation))
	key := mac.Sum(nil)
	probeSigningKey.Store(&key)
}

func currentProbeSigningKey() []byte {
	if p := probeSigningKey.Load(); p != nil {
		return *p
	}
	return nil
}

// isProbeHeaderName 探测协议头不允许出现在用户自定义 headers 中。
func isProbeHeaderName(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), probeHeaderPrefix)
}

// ProbeHints 监控对网关选号的提示。
type ProbeHints struct {
	Exclude []int64 // 软排除：剩余无可用账号时网关会忽略它们
	Prefer  int64   // 优选：可调度时直接选中
}

func joinProbeIDs(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

func parseProbeIDs(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make([]int64, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		out = append(out, id)
		if len(out) >= probeMaxExcludedIDs {
			break
		}
	}
	return out
}

func probeAPIKeyDigest(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

func computeProbeSignature(key []byte, ts int64, apiKey string, h ProbeHints) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("|"))
	mac.Write([]byte(probeAPIKeyDigest(apiKey)))
	mac.Write([]byte("|"))
	mac.Write([]byte(joinProbeIDs(h.Exclude)))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(h.Prefer, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// BuildProbeHeaders 生成探测请求头；签名密钥未初始化时返回 nil（退化为普通重试）。
func BuildProbeHeaders(apiKey string, h ProbeHints, now time.Time) map[string]string {
	key := currentProbeSigningKey()
	if len(key) == 0 || strings.TrimSpace(apiKey) == "" {
		return nil
	}
	if len(h.Exclude) > probeMaxExcludedIDs {
		h.Exclude = h.Exclude[len(h.Exclude)-probeMaxExcludedIDs:]
	}
	ts := now.Unix()
	headers := map[string]string{
		ProbeHeaderSignature: strconv.FormatInt(ts, 10) + "." + computeProbeSignature(key, ts, apiKey, h),
	}
	if len(h.Exclude) > 0 {
		headers[ProbeHeaderExclude] = joinProbeIDs(h.Exclude)
	}
	if h.Prefer > 0 {
		headers[ProbeHeaderPrefer] = strconv.FormatInt(h.Prefer, 10)
	}
	return headers
}

// VerifyProbeHeaders 校验签名头。apiKeys 是从请求里提取到的候选 API Key（任一匹配即可）。
func VerifyProbeHeaders(signature, exclude, prefer string, apiKeys []string, now time.Time) (ProbeHints, bool) {
	key := currentProbeSigningKey()
	if len(key) == 0 {
		return ProbeHints{}, false
	}
	tsRaw, sig, ok := strings.Cut(strings.TrimSpace(signature), ".")
	if !ok || sig == "" {
		return ProbeHints{}, false
	}
	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return ProbeHints{}, false
	}
	age := now.Sub(time.Unix(ts, 0))
	if age > probeSignatureTTL || age < -probeSignatureTTL {
		return ProbeHints{}, false
	}
	hints := ProbeHints{Exclude: parseProbeIDs(exclude)}
	if p, err := strconv.ParseInt(strings.TrimSpace(prefer), 10, 64); err == nil && p > 0 {
		hints.Prefer = p
	}
	for _, apiKey := range apiKeys {
		if apiKey == "" {
			continue
		}
		expected := computeProbeSignature(key, ts, apiKey, hints)
		if hmac.Equal([]byte(expected), []byte(sig)) {
			return hints, true
		}
	}
	return ProbeHints{}, false
}

// ProbeSelectionRecord 记录网关为本次探测请求最终选中的账号（failover 换号会覆盖）。
type ProbeSelectionRecord struct {
	served atomic.Int64
}

// ServedAccountID 最近一次选中的账号 ID；0 表示尚未选号。
func (r *ProbeSelectionRecord) ServedAccountID() int64 {
	if r == nil {
		return 0
	}
	return r.served.Load()
}

type probeSelectionState struct {
	hints  ProbeHints
	record *ProbeSelectionRecord
}

type probeSelectionStateKey struct{}
type probeSelectionConsumedKey struct{}

// WithProbeSelectionHints 把已验签的提示放进请求 ctx，返回用于回传账号的记录。
func WithProbeSelectionHints(ctx context.Context, hints ProbeHints) (context.Context, *ProbeSelectionRecord) {
	rec := &ProbeSelectionRecord{}
	return context.WithValue(ctx, probeSelectionStateKey{}, &probeSelectionState{hints: hints, record: rec}), rec
}

// probeSelectionStateFrom 返回待应用的探测提示；已在本次选号调用链中应用过时返回 nil，避免递归。
func probeSelectionStateFrom(ctx context.Context) *probeSelectionState {
	if ctx == nil {
		return nil
	}
	if consumed, _ := ctx.Value(probeSelectionConsumedKey{}).(bool); consumed {
		return nil
	}
	st, _ := ctx.Value(probeSelectionStateKey{}).(*probeSelectionState)
	return st
}

func markProbeSelectionConsumed(ctx context.Context) context.Context {
	return context.WithValue(ctx, probeSelectionConsumedKey{}, true)
}
