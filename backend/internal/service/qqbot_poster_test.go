package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot"
	"github.com/Wei-Shaw/sub2api/internal/pkg/qqbot/render"
)

type posterRecorder struct {
	mu      sync.Mutex
	replies map[string][]botReply // by msg id
}

func (r *posterRecorder) send(_ context.Context, _ QQBotConfig, m *qqbot.Message, rep botReply) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies[m.MsgID] = append(r.replies[m.MsgID], rep)
}

func (r *posterRecorder) get(id string) []botReply {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]botReply(nil), r.replies[id]...)
}

func tinyPNG() []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 12)))
	return buf.Bytes()
}

// newPosterTestService stubs every external dependency of /邀请.
func newPosterTestService(genDelay time.Duration) (*QQBotService, *posterRecorder, *int32) {
	s, _ := newQQBotTestService(true)
	rec := &posterRecorder{replies: map[string][]botReply{}}
	s.send = rec.send
	var running, peak int32
	s.generateArt = func(ctx context.Context, _ QQBotConfig, _ string) ([]byte, error) {
		n := atomic.AddInt32(&running, 1)
		defer atomic.AddInt32(&running, -1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		select {
		case <-time.After(genDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return tinyPNG(), nil
	}
	s.invitePlanner = func(_ context.Context, _ QQBotConfig, m *qqbot.Message) (*invitePlan, string) {
		return &invitePlan{
			poster:   render.PosterInput{SiteName: "T", Link: "https://t.example/register?aff=" + m.UserOpenID, Code: m.UserOpenID},
			after:    "stats",
			fallback: "fallback",
		}, ""
	}
	s.renderPoster = func(in render.PosterInput) ([]byte, error) {
		time.Sleep(5 * time.Millisecond)
		return []byte("jpg:" + in.Code), nil
	}
	return s, rec, &peak
}

var posterTestCfg = QQBotConfig{Enabled: true, AppID: "a", AppSecret: "b", PosterAPIKey: "sk-test", PosterModel: "m", PosterDailyLimit: 3}

func posterMsg(user, id string) *qqbot.Message {
	return &qqbot.Message{Scene: qqbot.SceneGroup, MsgID: id, GroupOpenID: "G", UserOpenID: user, Content: "/邀请"}
}

func hasImage(reps []botReply) bool {
	for _, r := range reps {
		if len(r.Image) > 0 {
			return true
		}
	}
	return false
}

// 100 users in one group ask at the same time: everyone gets a poster, AI
// generation never exceeds its concurrency limit, and the long queue degrades
// to pool/procedural art instead of making people wait.
func TestQQBotInviteConcurrentBurst(t *testing.T) {
	s, rec, peak := newPosterTestService(300 * time.Millisecond)
	const n = 100
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.handleInvite(posterTestCfg, posterMsg(fmt.Sprintf("U%03d", i), fmt.Sprintf("M%03d", i)))
		}(i)
	}
	wg.Wait()
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("burst took too long: %s", elapsed)
	}
	for i := 0; i < n; i++ {
		if !hasImage(rec.get(fmt.Sprintf("M%03d", i))) {
			t.Fatalf("user %d got no poster: %+v", i, rec.get(fmt.Sprintf("M%03d", i)))
		}
	}
	if p := atomic.LoadInt32(peak); p > posterGenSlots {
		t.Fatalf("AI generation concurrency %d exceeds limit %d", p, posterGenSlots)
	}
	if d := s.poster.depth(); d != 0 {
		t.Fatalf("queue not drained: %d", d)
	}
}

func TestQQBotInviteDuplicateAndCache(t *testing.T) {
	s, rec, _ := newPosterTestService(300 * time.Millisecond)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); s.handleInvite(posterTestCfg, posterMsg("U1", "A")) }()
	time.Sleep(50 * time.Millisecond)
	go func() { defer wg.Done(); s.handleInvite(posterTestCfg, posterMsg("U1", "B")) }()
	wg.Wait()
	if !hasImage(rec.get("A")) {
		t.Fatal("first request got no poster")
	}
	if b := rec.get("B"); len(b) != 1 || !strings.Contains(b[0].Text, "正在生成") {
		t.Fatalf("duplicate request should be told to wait, got %+v", b)
	}
	// Within the cache TTL the poster is resent without generating again.
	s.handleInvite(posterTestCfg, posterMsg("U1", "C"))
	c := rec.get("C")
	if !hasImage(c) || strings.Contains(c[0].Text, "生成中") {
		t.Fatalf("cached poster expected, got %+v", c)
	}
}

func TestQQBotInviteDailyLimitAndPool(t *testing.T) {
	p := newPosterPipeline()
	for i := 0; i < 3; i++ {
		if !p.takeDaily("U", "2026-11-08", 3) {
			t.Fatalf("take %d should succeed", i)
		}
	}
	if p.takeDaily("U", "2026-11-08", 3) {
		t.Fatal("4th generation on the same day must be refused")
	}
	p.refundDaily("U", "2026-11-08")
	if !p.takeDaily("U", "2026-11-08", 3) {
		t.Fatal("refunded slot should be usable")
	}
	if !p.takeDaily("U", "2026-11-09", 3) {
		t.Fatal("counter must reset on a new day")
	}
	if p.randomPool() != nil {
		t.Fatal("empty pool must return nil")
	}
	for i := 0; i < posterPoolMax+5; i++ {
		p.addPool([]byte{byte(i)})
	}
	if len(p.pool) != posterPoolMax || p.randomPool() == nil {
		t.Fatalf("pool size %d", len(p.pool))
	}
}

func TestQQBotInviteFallsBackWhenGenerationFails(t *testing.T) {
	s, rec, _ := newPosterTestService(0)
	s.generateArt = func(context.Context, QQBotConfig, string) ([]byte, error) { return nil, fmt.Errorf("boom") }
	s.handleInvite(posterTestCfg, posterMsg("U9", "X"))
	if !hasImage(rec.get("X")) {
		t.Fatalf("poster must still be sent with a procedural background: %+v", rec.get("X"))
	}
	// failed generation is refunded
	if !s.poster.takeDaily("U9", s.now().In(checkinZone).Format("2006-01-02"), 1) {
		t.Fatal("failed generation should not consume the daily quota")
	}
}

func TestUserInviteRules(t *testing.T) {
	d := &AffiliateDetail{EffectiveLevelRates: []AffiliateLevelRateRule{
		{Level: 1, RatePercent: 10, Unlocked: true},
		{Level: 2, RatePercent: 3, Unlocked: true},
		{Level: 3, RatePercent: 0, Unlocked: false, UnlockInviteCount: 10},
	}}
	got := userInviteRules(d, []float64{10, 3, 1.5})
	want := []string{"好友充值消费，你得 10% 返利", "二级好友消费，你再得 3%", "有效邀请满 10 人，解锁三级返利 1.5%"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("rules = %q", got)
	}
	if got := userInviteRules(&AffiliateDetail{}, nil); len(got) != 1 {
		t.Fatalf("empty rules should still have the balance hint: %q", got)
	}
	if p := randomPosterPrompt(1); !strings.Contains(p, "no text") {
		t.Fatalf("prompt must forbid text: %s", p)
	}
}
