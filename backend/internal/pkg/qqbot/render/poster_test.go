package render

import (
	"image"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestRenderPosterEdgeCases(t *testing.T) {
	cases := []PosterInput{
		{SiteName: "S", Link: "https://example.com", Seed: 1},
		{SiteName: "一个特别特别特别特别特别特别特别特别长的站点名称用于测试截断", Logo: []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"),
			Headline: "邀请好友", Rules: []string{"a", "b", "c", "d"}, Link: "https://example.com/register?aff=ABC", Code: "ABC", Seed: 2},
		{SiteName: "X", Logo: []byte("data:image/png;base64,not-base64"), Link: "https://x.y", Seed: 3,
			Art: image.NewRGBA(image.Rect(0, 0, 10, 40))},
	}
	for i, in := range cases {
		b, err := RenderPoster(in)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		assertPNG(t, b)
	}
}

func TestQRMatrix(t *testing.T) {
	m, err := qrMatrix("https://stapi.example.com/register?aff=ABCD1234")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(m); n < 21 || n != len(m[0]) {
		t.Fatalf("unexpected QR size %d", n)
	}
}

func loadDemoImage(t *testing.T, path string) image.Image {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	img := DecodeImage(data)
	if img == nil {
		t.Fatalf("decode %s failed", path)
	}
	return img
}

// TestRenderPosterDemo writes demo posters when QQBOT_RENDER_DEMO=1.
// Optional: QQBOT_POSTER_LOGO (png path), QQBOT_POSTER_ART_DIR (jpg/png illustrations).
func TestRenderPosterDemo(t *testing.T) {
	if os.Getenv("QQBOT_RENDER_DEMO") != "1" {
		t.Skip("set QQBOT_RENDER_DEMO=1 to write demo posters")
	}
	var logo []byte
	if p := os.Getenv("QQBOT_POSTER_LOGO"); p != "" {
		logo, _ = os.ReadFile(p)
	}
	user := PosterInput{
		SiteName: "STApi", Logo: logo,
		Headline: "邀请好友 · 一起用 AI",
		Subline:  "Claude / GPT / Gemini 一站直连，按量计费",
		Rules:    []string{"好友充值消费，你得 10% 返利", "二级好友消费，你再得 3%", "返利可提现或抵扣余额"},
		Domain:   "stapi.example.com",
		Link:     "https://stapi.example.com/register?aff=K7Q2XM",
		Code:     "K7Q2XM",
	}
	admin := user
	admin.Headline = "STApi · 稳定好用的 AI 中转站"
	admin.Subline = "官方渠道直连 · 按量计费 · 7×24 监控"
	admin.Rules = []string{"注册即送体验额度", "邀请好友消费，享 10% 返利"}
	admin.Link = "https://stapi.example.com"
	admin.Code = ""

	type demo struct {
		name string
		in   PosterInput
	}
	// one fallback per procedural style: hills (user), orbs (admin), night (user)
	seedFor := func(style int, skip int64) int64 {
		for s := int64(1); ; s++ {
			if s != skip && fallbackStyle(s) == style {
				return s
			}
		}
	}
	u1, a1, u2 := user, admin, user
	u1.Seed, a1.Seed, u2.Seed = seedFor(0, 0), seedFor(3, 0), seedFor(2, 0)
	demos := []demo{{"poster_1_fallback_user.png", u1}, {"poster_2_fallback_admin.png", a1}, {"poster_6_fallback_night.png", u2}}

	if dir := os.Getenv("QQBOT_POSTER_ART_DIR"); dir != "" {
		files, _ := filepath.Glob(filepath.Join(dir, "art*.jpg"))
		sort.Strings(files)
		for i, f := range files {
			in := user
			if i == 1 {
				in = admin
			}
			in.Art = loadDemoImage(t, f)
			in.Seed = int64(100 + i)
			demos = append(demos, demo{name: filepathBase(i), in: in})
		}
	}

	dir := filepath.Join("..", "..", "..", "..", "tmp", "qqbot-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range demos {
		b, err := RenderPoster(d.in)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, d.name), b, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d KB)", d.name, len(b)/1024)
	}
}

func filepathBase(i int) string {
	return []string{"poster_3_art_user.png", "poster_4_art_admin.png", "poster_5_art_user.png", "poster_6_art.png"}[min(i, 3)]
}
