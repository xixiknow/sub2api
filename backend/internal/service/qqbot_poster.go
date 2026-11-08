package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot/render"
)

const (
	posterGenSlots    = 4               // concurrent AI illustrations
	posterQueueMax    = 200             // queued + running invite requests
	posterDeadline    = 3 * time.Minute // QQ passive replies expire after 5 min
	posterTimeout     = 180 * time.Second
	posterGenTimeout  = 120 * time.Second
	posterCacheTTL    = 10 * time.Minute
	posterPoolMax     = 30
	posterArtMaxBytes = 20 << 20
	posterJPEGQuality = 88
)

func isInviteCommand(cmd string) bool {
	switch cmd {
	case "邀请", "invite", "海报", "邀请海报":
		return true
	}
	return false
}

// posterPipeline holds the in-memory state of /邀请: per-user dedup, the
// queue size, daily generation counters, the illustration reuse pool and a
// short per-user cache of finished posters. Nothing here affects correctness
// across restarts; it only bounds cost and latency.
type posterPipeline struct {
	mu        sync.Mutex
	inflight  map[string]struct{}
	daily     map[string]int // openid -> AI generations on dailyDate
	dailyDate string
	pool      [][]byte // JPEG illustrations, ring buffer
	poolNext  int
	recent    map[string]recentPoster
	rng       *rand.Rand

	genSlots    chan struct{}
	renderSlots chan struct{}
}

type recentPoster struct {
	jpg []byte
	at  time.Time
}

func newPosterPipeline() *posterPipeline {
	return &posterPipeline{
		inflight:    map[string]struct{}{},
		daily:       map[string]int{},
		recent:      map[string]recentPoster{},
		rng:         rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x51ed2701)),
		genSlots:    make(chan struct{}, posterGenSlots),
		renderSlots: make(chan struct{}, max(1, runtime.NumCPU())),
	}
}

type posterAdmission int

const (
	posterAdmitted posterAdmission = iota
	posterDuplicate
	posterQueueFull
)

func (p *posterPipeline) begin(key string) posterAdmission {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.inflight[key]; ok {
		return posterDuplicate
	}
	if len(p.inflight) >= posterQueueMax {
		return posterQueueFull
	}
	p.inflight[key] = struct{}{}
	return posterAdmitted
}

func (p *posterPipeline) end(key string) {
	p.mu.Lock()
	delete(p.inflight, key)
	p.mu.Unlock()
}

func (p *posterPipeline) depth() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.inflight)
}

// takeDaily reserves one AI generation for key on day; refundDaily returns it.
func (p *posterPipeline) takeDaily(key, day string, limit int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dailyDate != day {
		p.dailyDate, p.daily = day, map[string]int{}
	}
	if p.daily[key] >= limit {
		return false
	}
	p.daily[key]++
	return true
}

func (p *posterPipeline) refundDaily(key, day string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dailyDate == day && p.daily[key] > 0 {
		p.daily[key]--
	}
}

func (p *posterPipeline) addPool(jpg []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pool) < posterPoolMax {
		p.pool = append(p.pool, jpg)
		return
	}
	p.pool[p.poolNext] = jpg
	p.poolNext = (p.poolNext + 1) % posterPoolMax
}

func (p *posterPipeline) randomPool() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pool) == 0 {
		return nil
	}
	return p.pool[p.rng.IntN(len(p.pool))]
}

func (p *posterPipeline) seed() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rng.Int64()
}

func (p *posterPipeline) cached(key string) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r, ok := p.recent[key]; ok && time.Since(r.at) < posterCacheTTL {
		return r.jpg
	}
	return nil
}

func (p *posterPipeline) store(key string, jpg []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.recent) > 2000 {
		for k, r := range p.recent {
			if time.Since(r.at) >= posterCacheTTL {
				delete(p.recent, k)
			}
		}
	}
	p.recent[key] = recentPoster{jpg: jpg, at: time.Now()}
}

// invitePlan is everything needed to render and send one invite poster.
type invitePlan struct {
	poster   render.PosterInput // Art and Seed are filled later
	after    string             // text sent after the poster (may be empty)
	fallback string             // text sent when the image cannot be sent
}

// handleInvite runs the whole /邀请 flow for one message (called on its own goroutine).
func (s *QQBotService) handleInvite(cfg QQBotConfig, msg *qqbot.Message) {
	start := s.now()
	ctx, cancel := context.WithTimeout(context.Background(), posterTimeout)
	defer cancel()

	plan, text := s.invitePlanner(ctx, cfg, msg)
	if plan == nil {
		s.send(ctx, cfg, msg, botReply{Text: text})
		return
	}
	key := msg.UserOpenID
	if jpg := s.poster.cached(key); jpg != nil {
		s.sendPoster(ctx, cfg, msg, plan, jpg)
		return
	}
	switch s.poster.begin(key) {
	case posterDuplicate:
		s.send(ctx, cfg, msg, botReply{Text: "你的海报正在生成，请稍候～"})
		return
	case posterQueueFull:
		s.send(ctx, cfg, msg, botReply{Text: "请求太多，请稍后再试"})
		return
	}
	defer s.poster.end(key)

	art := s.pickPosterArt(ctx, cfg, msg, start)
	in := plan.poster
	in.Art, in.Seed = art, s.poster.seed()

	select {
	case s.poster.renderSlots <- struct{}{}:
	case <-ctx.Done():
		return
	}
	jpg, err := s.renderPoster(in)
	<-s.poster.renderSlots
	if err != nil {
		logger.LegacyPrintf("service.qqbot", "[QQBot] render poster failed: %v", err)
		s.send(ctx, cfg, msg, botReply{Text: plan.fallback})
		return
	}
	s.poster.store(key, jpg)
	s.sendPoster(ctx, cfg, msg, plan, jpg)
}

func (s *QQBotService) sendPoster(ctx context.Context, cfg QQBotConfig, msg *qqbot.Message, plan *invitePlan, jpg []byte) {
	s.send(ctx, cfg, msg, botReply{Image: jpg, Text: plan.fallback})
	if plan.after != "" {
		s.send(ctx, cfg, msg, botReply{Text: plan.after})
	}
}

// pickPosterArt returns a fresh AI illustration when allowed, otherwise one
// from the reuse pool, otherwise nil (procedural background).
func (s *QQBotService) pickPosterArt(ctx context.Context, cfg QQBotConfig, msg *qqbot.Message, start time.Time) image.Image {
	day := s.now().In(checkinZone).Format("2006-01-02")
	canGenerate := cfg.PosterAPIKey != "" && cfg.PosterDailyLimit > 0 &&
		s.poster.depth() <= 2*posterGenSlots && // long queue → reuse instead of generating
		s.poster.takeDaily(msg.UserOpenID, day, cfg.PosterDailyLimit)
	if canGenerate {
		s.send(ctx, cfg, msg, botReply{Text: "海报生成中，约半分钟…"})
		if art := s.generatePosterArt(ctx, cfg, start); art != nil {
			return art
		}
		s.poster.refundDaily(msg.UserOpenID, day)
	}
	if b := s.poster.randomPool(); b != nil {
		return render.DecodeImage(b)
	}
	return nil
}

func (s *QQBotService) generatePosterArt(ctx context.Context, cfg QQBotConfig, start time.Time) image.Image {
	remaining := posterDeadline - s.now().Sub(start)
	if remaining <= 0 {
		return nil
	}
	wait := time.NewTimer(remaining)
	defer wait.Stop()
	select {
	case s.poster.genSlots <- struct{}{}:
	case <-wait.C: // waited too long for a slot: fall back so the reply window is not missed
		return nil
	case <-ctx.Done():
		return nil
	}
	defer func() { <-s.poster.genSlots }()

	genCtx, cancel := context.WithTimeout(ctx, min(posterGenTimeout, posterDeadline-s.now().Sub(start)+30*time.Second))
	defer cancel()
	data, err := s.generateArt(genCtx, cfg, randomPosterPrompt(s.poster.seed()))
	if err != nil {
		logger.LegacyPrintf("service.qqbot", "[QQBot] poster art generation failed: %v", err)
		return nil
	}
	img := render.DecodeImage(data)
	if img == nil {
		logger.LegacyPrintf("service.qqbot", "[QQBot] poster art is not a decodable image (%d bytes)", len(data))
		return nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err == nil {
		s.poster.addPool(buf.Bytes())
	}
	return img
}

// buildInvitePlan gathers user/site data. When plan is nil, text is the reply.
func (s *QQBotService) buildInvitePlan(ctx context.Context, cfg QQBotConfig, msg *qqbot.Message) (*invitePlan, string) {
	uid, err := s.bindingUser(ctx, cfg.AppID, msg.UserOpenID)
	if errors.Is(err, ErrQQBotBindingMissing) {
		return nil, "你还没有绑定账号，请先发送：/绑定 你的注册邮箱"
	}
	if err != nil {
		return nil, s.internalError("invite lookup", err)
	}
	user, err := s.userRepo.GetByID(ctx, uid)
	if err != nil || user == nil || !user.IsActive() {
		return nil, "绑定的账号不可用（可能已被禁用），请联系管理员"
	}
	frontend := strings.TrimRight(strings.TrimSpace(s.settings.GetFrontendURL(ctx)), "/")
	if frontend == "" {
		return nil, "管理员尚未配置站点地址，暂时无法生成邀请海报"
	}
	affEnabled := s.affiliate != nil && s.affiliate.IsEnabled(ctx)
	if !user.IsAdmin() && !affEnabled {
		return nil, "邀请返利暂未开启"
	}

	siteName := s.settings.GetSiteName(ctx)
	subtitle, _ := s.settingRepo.GetValue(ctx, SettingKeySiteSubtitle)
	subtitle = strings.TrimSpace(subtitle)
	logo, _ := s.settingRepo.GetValue(ctx, SettingKeySiteLogo)
	domain := frontend
	if u, err := url.Parse(frontend); err == nil && u.Host != "" {
		domain = u.Host
	}
	plan := &invitePlan{poster: render.PosterInput{SiteName: siteName, Logo: []byte(strings.TrimSpace(logo)), Domain: domain}}
	group := msg.Scene == qqbot.SceneGroup

	if user.IsAdmin() {
		plan.poster.Headline = "欢迎加入 " + siteName
		plan.poster.Subline = firstNonEmpty(subtitle, "主流 AI 模型一站直连，按量计费")
		plan.poster.Link = frontend
		plan.poster.Rules = s.siteInviteRules(ctx, affEnabled)
		plan.fallback = "海报发送失败了，可以稍后再试一次"
		if !group {
			plan.after = "这是站点推广海报（管理员），可直接转发到群里～"
		}
		return plan, ""
	}

	detail, err := s.affiliate.GetAffiliateDetail(ctx, uid)
	if err != nil || detail == nil || detail.AffCode == "" {
		return nil, s.internalError("invite detail", err)
	}
	plan.poster.Headline = "邀请好友 · 一起用 AI"
	plan.poster.Subline = firstNonEmpty(subtitle, "主流 AI 模型一站直连，按量计费")
	plan.poster.Link = frontend + "/register?aff=" + url.QueryEscape(detail.AffCode)
	plan.poster.Code = detail.AffCode
	plan.poster.Rules = userInviteRules(detail, s.settings.GetAffiliateLevelRates(ctx))
	plan.fallback = "海报发送失败了，可以稍后再试一次。你的邀请码：" + detail.AffCode
	if group {
		plan.after = "返利明细请私聊我发送 /邀请 查看"
	} else {
		plan.after = fmt.Sprintf("你的邀请码：%s\n已邀请 %d 人（有效 %d 人）\n累计返利 %s · 可用 %s · 冻结 %s\n长按海报识别二维码即可注册",
			detail.AffCode, detail.AffCount, detail.QualifiedAffCount,
			formatUSD(detail.AffHistoryQuota), formatUSD(detail.AffQuota), formatUSD(detail.AffFrozenQuota))
	}
	return plan, ""
}

var levelNames = []string{"", "一", "二", "三", "四", "五"}

func levelName(level int) string {
	if level > 0 && level < len(levelNames) {
		return levelNames[level]
	}
	return strconv.Itoa(level)
}

func formatPercent(v float64) string {
	return strconv.FormatFloat(float64(int64(v*100+0.5))/100, 'f', -1, 64)
}

func formatUSD(v float64) string { return fmt.Sprintf("$%.2f", v) }

// userInviteRules describes the user's effective rebate rates (max 3 lines).
func userInviteRules(d *AffiliateDetail, globalLevels []float64) []string {
	var rules []string
	for _, r := range d.EffectiveLevelRates {
		switch {
		case r.Level == 1:
			if r.RatePercent > 0 {
				rules = append(rules, fmt.Sprintf("好友充值消费，你得 %s%% 返利", formatPercent(r.RatePercent)))
			}
		case r.Unlocked && r.RatePercent > 0:
			rules = append(rules, fmt.Sprintf("%s级好友消费，你再得 %s%%", levelName(r.Level), formatPercent(r.RatePercent)))
		case !r.Unlocked && r.Level-1 < len(globalLevels) && globalLevels[r.Level-1] > 0:
			rules = append(rules, fmt.Sprintf("有效邀请满 %d 人，解锁%s级返利 %s%%", r.UnlockInviteCount, levelName(r.Level), formatPercent(globalLevels[r.Level-1])))
		}
	}
	if len(rules) < 3 {
		rules = append(rules, "返利可转入账户余额使用")
	}
	if len(rules) > 3 {
		rules = rules[:3]
	}
	return rules
}

// siteInviteRules describes the public rebate program for the admin's site poster.
func (s *QQBotService) siteInviteRules(ctx context.Context, affEnabled bool) []string {
	rules := []string{"注册即可使用，按量计费"}
	if !affEnabled {
		return append(rules, "Claude / GPT / Gemini 等主流模型")
	}
	if base := s.settings.GetAffiliateRebateRatePercent(ctx); base > 0 {
		rules = append(rules, fmt.Sprintf("邀请好友消费，享 %s%% 返利", formatPercent(base)))
	}
	var parts []string
	for i, v := range s.settings.GetAffiliateLevelRates(ctx) {
		if level := i + 1; level >= 2 && v > 0 {
			parts = append(parts, fmt.Sprintf("%s级 %s%%", levelName(level), formatPercent(v)))
		}
	}
	if len(parts) > 0 {
		rules = append(rules, "多级返利："+strings.Join(parts, " · "))
	}
	return rules
}

// ---- AI illustration via this site's own OpenAI-compatible gateway ----

var (
	posterSubjects = []string{
		"a red fox curled up among autumn leaves", "a humpback whale gliding through glowing water",
		"a snowy owl perched on a pine branch", "a deer standing in a misty birch forest",
		"a snow mountain reflected in a calm alpine lake", "a quiet bamboo forest with a stone path",
		"a cherry blossom street with paper lanterns", "a starry grassland with a lone tree",
		"a cat napping on a sunny windowsill full of plants", "a lighthouse on a cliff above the sea",
		"a hot air balloon drifting over rolling hills", "a red panda in a mossy forest",
		"koi fish in a lotus pond", "a small cabin beside a waterfall", "a fennec fox in golden desert dunes",
	}
	posterStyles = []string{
		"soft watercolor illustration", "flat vector illustration with gentle gradients", "layered paper-cut art",
		"ukiyo-e woodblock print style", "low-poly 3D render", "cute clay 3D render", "impressionist oil painting",
		"Studio-style anime background painting", "minimalist gouache illustration",
	}
	posterMoments = []string{"at golden sunrise", "at dusk", "under a starry night sky", "on a misty morning", "in warm afternoon light", "in early spring"}
)

func randomPosterPrompt(seed int64) string {
	r := rand.New(rand.NewPCG(uint64(seed), 0x7f4a7c15))
	return fmt.Sprintf("%s, %s, %s. Vertical poster illustration, rich but harmonious colors, "+
		"the main subject arranged around the edges and lower third, leave the center and the bottom area calm and uncluttered. "+
		"Absolutely no text, no letters, no numbers, no logos, no watermark.",
		posterSubjects[r.IntN(len(posterSubjects))], posterStyles[r.IntN(len(posterStyles))], posterMoments[r.IntN(len(posterMoments))])
}

// posterGatewayBase is this server's own address (loopback avoids CDN timeouts).
func (s *QQBotService) posterGatewayBase() string {
	host, port := "127.0.0.1", 8080
	if s.appCfg != nil {
		if h := strings.TrimSpace(s.appCfg.Server.Host); h != "" && h != "0.0.0.0" && h != "::" {
			host = h
		}
		if s.appCfg.Server.Port > 0 {
			port = s.appCfg.Server.Port
		}
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}

var posterHTTPClient = &http.Client{Timeout: posterGenTimeout + 30*time.Second}

func (s *QQBotService) generateArtViaGateway(ctx context.Context, cfg QQBotConfig, prompt string) ([]byte, error) {
	body, _ := json.Marshal(map[string]any{"model": cfg.PosterModel, "prompt": prompt, "size": "1024x1536", "n": 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.posterGatewayBase()+"/v1/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.PosterAPIKey)
	resp, err := posterHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, posterArtMaxBytes*2))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("image generation: http %d: %s", resp.StatusCode, truncateForLog(raw, 300))
	}
	var out struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) == 0 {
		return nil, fmt.Errorf("image generation: unexpected response: %s", truncateForLog(raw, 300))
	}
	if b64 := out.Data[0].B64JSON; b64 != "" {
		return base64.StdEncoding.DecodeString(b64)
	}
	if u := out.Data[0].URL; strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		return downloadPosterArt(ctx, u)
	}
	return nil, errors.New("image generation: response has no image")
}

func downloadPosterArt(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := posterHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("download art: http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, posterArtMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > posterArtMaxBytes {
		return nil, errors.New("download art: image too large")
	}
	return data, nil
}
