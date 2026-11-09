package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot/render"
)

const qqbotHelpText = `可用指令：
/签到 —— 每日签到抽奖（需先绑定）
/绑定 绑定码 —— 绑定站内账号（绑定码在网站个人资料页生成）
/解绑 —— 解除 QQ 绑定
/状态 —— 渠道监控状态
/分组 —— 公开分组与倍率
/邀请 —— 生成专属邀请海报
/帮助 —— 查看本帮助
群聊中请 @我 后发送指令（已开通全量消息时可直接发 /指令）；建议私聊完成绑定`

const qqbotBindUsage = "请先登录网站，在「个人资料 → QQ 机器人」生成绑定码，然后私聊我发送：/绑定 绑定码\n例如：/绑定 ABCD2345"
const qqbotNeedBind = "你还没有绑定账号。请登录网站，在「个人资料 → QQ 机器人」生成绑定码，然后发送：/绑定 绑定码"

const qqbotBindLimitPerMinute = 5

// parseQQBotCommand splits "/绑定 ABCD2345" into ("绑定", "ABCD2345"). The slash is optional.
func parseQQBotCommand(content string) (cmd, arg string) {
	content = strings.TrimSpace(content)
	content = strings.TrimLeft(content, "/／")
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return "", ""
	}
	return strings.ToLower(fields[0]), strings.Join(fields[1:], " ")
}

// HandleCommand runs one chat command and returns the reply.
func (s *QQBotService) HandleCommand(ctx context.Context, cfg QQBotConfig, m *qqbot.Message) botReply {
	cmd, arg := parseQQBotCommand(m.Content)
	switch cmd {
	case "", "帮助", "help", "菜单", "指令":
		return botReply{Text: qqbotHelpText}
	case "签到", "checkin":
		return botReply{Text: s.cmdCheckin(ctx, cfg, m)}
	case "绑定", "bind":
		return botReply{Text: s.cmdBind(ctx, cfg, m, arg)}
	case "解绑", "unbind":
		return botReply{Text: s.cmdUnbind(ctx, cfg, m)}
	case "状态", "status":
		return s.cmdStatus(ctx)
	case "分组", "groups":
		return s.cmdGroups(ctx)
	default:
		if !m.Addressed {
			return botReply{} // "/xxx" not meant for us in full-message mode: stay quiet
		}
		return botReply{Text: "未知指令，发送 /帮助 查看可用指令"}
	}
}

func formatCents(cents int) string { return fmt.Sprintf("¥%d.%02d", cents/100, cents%100) }

func (s *QQBotService) cmdCheckin(ctx context.Context, cfg QQBotConfig, m *qqbot.Message) string {
	uid, err := s.bindingUser(ctx, cfg.AppID, m.UserOpenID)
	if errors.Is(err, ErrQQBotBindingMissing) {
		return qqbotNeedBind
	}
	if err != nil {
		return s.internalError("checkin lookup", err)
	}
	before, err := s.checkin.Status(ctx, uid)
	if errors.Is(err, ErrCheckinDisabled) {
		return "签到功能暂未开启"
	}
	if err != nil {
		return s.internalError("checkin status", err)
	}
	if before.CheckedIn {
		return fmt.Sprintf("今天已经签到过啦～今日奖励 %s，已连续签到 %d 天，明天再来吧", formatCents(before.RewardCents), before.Snapshot.StreakDays)
	}
	after, err := s.checkin.Claim(ctx, uid, qqbotCheckinHash(uid, m.UserOpenID, checkinDay(s.now())))
	switch {
	case errors.Is(err, ErrCheckinDisabled):
		return "签到功能暂未开启"
	case errors.Is(err, ErrUserNotFound):
		return "绑定的账号不可用（可能已被禁用），请联系管理员"
	case err != nil:
		return s.internalError("checkin claim", err)
	}
	return fmt.Sprintf("签到成功！抽中 %s\n已连续签到 %d 天", formatCents(after.RewardCents), after.Snapshot.StreakDays)
}

func (s *QQBotService) cmdBind(ctx context.Context, cfg QQBotConfig, m *qqbot.Message, arg string) string {
	code := normalizeQQBotBindCode(arg)
	if !validQQBotBindCode(code) {
		return qqbotBindUsage
	}
	// Per-QQ limit keeps code guessing impractical (8 chars from a 31-symbol alphabet).
	if s.limiter.incr(m.UserOpenID) > qqbotBindLimitPerMinute {
		return "操作太频繁，请稍后再试"
	}
	if _, err := s.bindingUser(ctx, cfg.AppID, m.UserOpenID); err == nil {
		return "当前 QQ 已绑定账号，如需更换请先发送 /解绑"
	} else if !errors.Is(err, ErrQQBotBindingMissing) {
		return s.internalError("bind lookup", err)
	}
	uid, err := s.consumeBindCode(ctx, cfg, code, m.UserOpenID, m.Scene)
	switch {
	case errors.Is(err, errQQBotBindCodeInvalid):
		return "绑定码无效或已过期，请在网站「个人资料 → QQ 机器人」重新生成"
	case errors.Is(err, errQQBotBindConflict):
		return "该账号已绑定其他 QQ，请先在网站「个人资料」中解除后重试"
	case err != nil:
		return s.internalError("bind consume", err)
	}
	// Never echo account details into a group chat.
	if m.Scene == qqbot.SceneGroup {
		return "绑定成功！现在可以发送 /签到 了"
	}
	if u, uerr := s.userRepo.GetByID(ctx, uid); uerr == nil && u != nil {
		return "绑定成功：" + maskEmail(u.Email) + "\n现在可以发送 /签到 了"
	}
	return "绑定成功！现在可以发送 /签到 了"
}

func (s *QQBotService) cmdUnbind(ctx context.Context, cfg QQBotConfig, m *qqbot.Message) string {
	res, err := s.db.ExecContext(ctx, `DELETE FROM qqbot_bindings WHERE app_id=$1 AND openid=$2`, cfg.AppID, m.UserOpenID)
	if err != nil {
		return s.internalError("unbind", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "当前 QQ 没有绑定账号"
	}
	return "已解除绑定"
}

func (s *QQBotService) internalError(op string, err error) string {
	logger.LegacyPrintf("service.qqbot", "[QQBot] %s failed: %v", op, err)
	return "服务暂时不可用，请稍后再试"
}

// ---- images ----

var qqbotPlatformNames = map[string]string{
	"anthropic": "Claude", "openai": "OpenAI", "gemini": "Gemini", "antigravity": "Antigravity",
	"grok": "Grok", "kimi": "Kimi", "zhipu": "智谱", "deepseek": "DeepSeek", "minimax": "MiniMax",
	"kiro": "Kiro", "typesafe": "TypeSafe", "opencode_go": "OpenCode", "command_code": "CommandCode",
	"cline": "Cline", "composite": "组合",
}

var qqbotPlatformOrder = []string{"anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax"}

func qqbotPlatformName(p string) string {
	if n, ok := qqbotPlatformNames[strings.ToLower(p)]; ok {
		return n
	}
	if p == "" {
		return "其他"
	}
	return p
}

func qqbotPlatformRank(p string) int {
	for i, v := range qqbotPlatformOrder {
		if v == strings.ToLower(p) {
			return i
		}
	}
	return len(qqbotPlatformOrder)
}

func qqbotMonitorLevel(status string) string {
	switch status {
	case MonitorStatusOperational:
		return render.LevelOK
	case MonitorStatusDegraded:
		return render.LevelWarn
	case MonitorStatusFailed, MonitorStatusError:
		return render.LevelDown
	default:
		return render.LevelUnknown
	}
}

func (s *QQBotService) cmdStatus(ctx context.Context) botReply {
	if rt := s.settings.GetChannelMonitorRuntime(ctx); !rt.Enabled {
		return botReply{Text: "渠道监控暂未开启"}
	}
	views, err := s.monitor.ListUserView(ctx)
	if err != nil {
		return botReply{Text: s.internalError("list monitors", err)}
	}
	sort.SliceStable(views, func(i, j int) bool {
		return qqbotPlatformRank(views[i].Provider) < qqbotPlatformRank(views[j].Provider)
	})
	channels := make([]render.StatusChannel, 0, len(views))
	counts := map[string]int{}
	var down []string
	for _, v := range views {
		ch := render.StatusChannel{
			Platform: qqbotPlatformName(v.Provider), Name: v.Name, GroupName: v.GroupName,
			Model: v.PrimaryModel, Level: qqbotMonitorLevel(v.PrimaryStatus),
			LatencyMs: v.PrimaryLatencyMs, Availability7d: v.Availability7d,
		}
		if v.PrimaryStatus == "" && len(v.Timeline) == 0 {
			ch.Availability7d = -1
		}
		for i := len(v.Timeline) - 1; i >= 0; i-- { // timeline is newest-first
			ch.Timeline = append(ch.Timeline, qqbotMonitorLevel(v.Timeline[i].Status))
		}
		counts[ch.Level]++
		if ch.Level == render.LevelDown {
			down = append(down, v.Name)
		}
		channels = append(channels, ch)
	}
	text := fmt.Sprintf("渠道状态：正常 %d / 降级 %d / 故障 %d", counts[render.LevelOK], counts[render.LevelWarn], counts[render.LevelDown])
	if len(down) > 0 {
		text += "\n故障：" + strings.Join(down, "、")
	}
	png, err := s.images.get("status", channels, func() ([]byte, error) {
		return render.RenderStatus(render.StatusInput{UpdatedAt: s.now().In(checkinZone), Channels: channels})
	})
	if err != nil {
		logger.LegacyPrintf("service.qqbot", "[QQBot] render status failed: %v", err)
	}
	return botReply{Text: text, Image: png}
}

func (s *QQBotService) cmdGroups(ctx context.Context) botReply {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return botReply{Text: s.internalError("list groups", err)}
	}
	byPlatform := map[string]*render.GroupSection{}
	var platforms []string
	var lines []string
	for _, g := range groups {
		if g.IsExclusive {
			continue
		}
		sec, ok := byPlatform[g.Platform]
		if !ok {
			sec = &render.GroupSection{Platform: qqbotPlatformName(g.Platform)}
			byPlatform[g.Platform] = sec
			platforms = append(platforms, g.Platform)
		}
		row := render.GroupRow{Name: g.Name, RateMultiplier: g.RateMultiplier, Description: g.Description}
		if g.PeakRateEnabled && g.PeakRateMultiplier > 0 {
			row.PeakNote = fmt.Sprintf("高峰 ×%g", g.PeakRateMultiplier)
		}
		sec.Groups = append(sec.Groups, row)
		lines = append(lines, fmt.Sprintf("%s ×%g", g.Name, g.RateMultiplier))
	}
	if len(lines) == 0 {
		return botReply{Text: "暂无公开分组"}
	}
	sort.SliceStable(platforms, func(i, j int) bool { return qqbotPlatformRank(platforms[i]) < qqbotPlatformRank(platforms[j]) })
	sections := make([]render.GroupSection, 0, len(platforms))
	for _, p := range platforms {
		sections = append(sections, *byPlatform[p])
	}
	png, err := s.images.get("groups", sections, func() ([]byte, error) {
		return render.RenderGroups(render.GroupsInput{UpdatedAt: s.now().In(checkinZone), Sections: sections})
	})
	if err != nil {
		logger.LegacyPrintf("service.qqbot", "[QQBot] render groups failed: %v", err)
	}
	return botReply{Text: "公开分组：\n" + strings.Join(lines, "\n"), Image: png}
}

// ---- small in-memory helpers (single instance; correctness never depends on them) ----

// imageCache reuses a rendered image while its input data is unchanged (≤5 min).
type imageCache struct {
	mu      sync.Mutex
	entries map[string]imageCacheEntry
}

type imageCacheEntry struct {
	fp  [32]byte
	png []byte
	at  time.Time
}

func (c *imageCache) get(name string, data any, build func() ([]byte, error)) ([]byte, error) {
	raw, _ := json.Marshal(data)
	fp := sha256.Sum256(raw)
	c.mu.Lock()
	if e, ok := c.entries[name]; ok && e.fp == fp && time.Since(e.at) < 5*time.Minute {
		c.mu.Unlock()
		return e.png, nil
	}
	c.mu.Unlock()
	png, err := build()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]imageCacheEntry{}
	}
	c.entries[name] = imageCacheEntry{fp: fp, png: png, at: time.Now()}
	c.mu.Unlock()
	return png, nil
}

// ttlSet remembers keys for ttl; add reports whether the key was new.
type ttlSet struct {
	mu   sync.Mutex
	ttl  time.Duration
	seen map[string]time.Time
}

func newTTLSet(ttl time.Duration) *ttlSet { return &ttlSet{ttl: ttl, seen: map[string]time.Time{}} }

func (t *ttlSet) add(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if len(t.seen) > 10000 {
		for k, at := range t.seen {
			if now.Sub(at) > t.ttl {
				delete(t.seen, k)
			}
		}
	}
	if at, ok := t.seen[key]; ok && now.Sub(at) <= t.ttl {
		return false
	}
	t.seen[key] = now
	return true
}

// ttlCounter is a fixed-window counter per key.
type ttlCounter struct {
	mu     sync.Mutex
	window time.Duration
	m      map[string]*windowCount
}

type windowCount struct {
	start time.Time
	n     int
}

func newTTLCounter(window time.Duration) *ttlCounter {
	return &ttlCounter{window: window, m: map[string]*windowCount{}}
}

func (c *ttlCounter) incr(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if len(c.m) > 10000 {
		for k, w := range c.m {
			if now.Sub(w.start) > c.window {
				delete(c.m, k)
			}
		}
	}
	w, ok := c.m[key]
	if !ok || now.Sub(w.start) > c.window {
		w = &windowCount{start: now}
		c.m[key] = w
	}
	w.n++
	return w.n
}

// maskMiddle keeps the first and last 4 characters of an opaque id.
func maskMiddle(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:4] + "****" + s[len(s)-4:]
}
