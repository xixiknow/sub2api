package render

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ms(v int) *int { return &v }

func tl(pattern string) []string {
	m := map[rune]string{'o': LevelOK, 'w': LevelWarn, 'd': LevelDown, 'u': LevelUnknown}
	out := make([]string, 0, len(pattern))
	for _, r := range pattern {
		out = append(out, m[r])
	}
	return out
}

func demoStatus() StatusInput {
	return StatusInput{
		UpdatedAt: time.Date(2026, 11, 5, 14, 32, 0, 0, time.Local),
		Channels: []StatusChannel{
			{Platform: "Claude", Name: "Claude-主池", GroupName: "claude-稳定", Model: "claude-sonnet-5-5", Level: LevelOK, LatencyMs: ms(812), Availability7d: 99.8, Timeline: tl("oooooooooooooooooooo")},
			{Platform: "Claude", Name: "Claude-备用", GroupName: "claude-特价", Model: "claude-opus-5-5", Level: LevelOK, LatencyMs: ms(1240), Availability7d: 98.1, Timeline: tl("ooooooooooowoooooooo")},
			{Platform: "OpenAI", Name: "GPT-5 Pro", GroupName: "codex-pro", Model: "gpt-5.5", Level: LevelOK, LatencyMs: ms(640), Availability7d: 99.9, Timeline: tl("oooooooooooooooooooo")},
			{Platform: "OpenAI", Name: "Codex Plus", GroupName: "codex-plus", Model: "gpt-5.5-codex", Level: LevelOK, LatencyMs: ms(905), Availability7d: 99.2, Timeline: tl("ooooooooooooooooo")},
			{Platform: "Gemini", Name: "Gemini CLI", GroupName: "gemini-cli", Model: "gemini-3-pro", Level: LevelWarn, LatencyMs: ms(2810), Availability7d: 93.4, Timeline: tl("ooowooodoowoooowooww")},
			{Platform: "Grok", Name: "Grok 号池（超长名称用于测试截断效果）", GroupName: "grok", Model: "grok-5", Level: LevelDown, Availability7d: 71.0, Timeline: tl("oowddoooddddoddddddd")},
			{Platform: "Antigravity", Name: "Antigravity", GroupName: "antigravity-稳定", Model: "claude-sonnet-5-5", Level: LevelOK, LatencyMs: ms(1530), Availability7d: 99.5, Timeline: tl("oooooooooooooooooooo")},
			{Platform: "Kimi", Name: "Kimi Coding", GroupName: "kimi", Model: "kimi-k3", Level: LevelOK, LatencyMs: ms(420), Availability7d: 100, Timeline: tl("oooooooooooo")},
			{Platform: "OpenAI", Name: "新接入渠道", GroupName: "测试", Model: "", Level: LevelUnknown, Availability7d: -1},
		},
	}
}

func demoGroups() GroupsInput {
	return GroupsInput{
		UpdatedAt: time.Date(2026, 11, 5, 14, 32, 0, 0, time.Local),
		Sections: []GroupSection{
			{Platform: "Claude", Groups: []GroupRow{
				{Name: "claude-稳定", RateMultiplier: 1, Description: "Max 号池，支持 1M 上下文，适合 Claude Code 长任务"},
				{Name: "claude-特价", RateMultiplier: 0.6, PeakNote: "高峰 ×0.9", Description: "高峰时段可能排队"},
			}},
			{Platform: "OpenAI", Groups: []GroupRow{
				{Name: "codex-pro", RateMultiplier: 1.2, Description: "Pro 号池，含图片生成"},
				{Name: "codex-plus", RateMultiplier: 0.8, Description: "Plus 号池"},
			}},
			{Platform: "Gemini", Groups: []GroupRow{
				{Name: "gemini-cli", RateMultiplier: 0.5, Description: ""},
				{Name: "gemini-一个很长很长很长的分组名称", RateMultiplier: 0.75, Description: "这是一段非常非常长的简介，用来验证超出宽度后会被省略号截断，不会溢出到卡片外面去"},
			}},
		},
	}
}

func assertPNG(t *testing.T, b []byte) {
	t.Helper()
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatalf("invalid png: %v", err)
	}
}

func TestRenderEdgeCases(t *testing.T) {
	for _, in := range []StatusInput{{}, {Channels: make([]StatusChannel, 40)}} {
		b, err := RenderStatus(in)
		if err != nil {
			t.Fatal(err)
		}
		assertPNG(t, b)
	}
	b, err := RenderGroups(GroupsInput{})
	if err != nil {
		t.Fatal(err)
	}
	assertPNG(t, b)
	if got := formatRate(1); got != "×1.0" {
		t.Fatalf("formatRate(1)=%s", got)
	}
	if got := formatRate(0.75); got != "×0.75" {
		t.Fatalf("formatRate(0.75)=%s", got)
	}
}

// TestRenderDemo writes demo images when QQBOT_RENDER_DEMO=1 (to backend/tmp/qqbot-demo).
func TestRenderDemo(t *testing.T) {
	status, err := RenderStatus(demoStatus())
	if err != nil {
		t.Fatal(err)
	}
	groups, err := RenderGroups(demoGroups())
	if err != nil {
		t.Fatal(err)
	}
	assertPNG(t, status)
	assertPNG(t, groups)
	if !strings.EqualFold(os.Getenv("QQBOT_RENDER_DEMO"), "1") {
		return
	}
	dir := filepath.Join("..", "..", "..", "..", "tmp", "qqbot-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{"status_demo.png": status, "groups_demo.png": groups} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d KB)", filepath.Join(dir, name), len(b)/1024)
	}
}
