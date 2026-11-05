package render

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"
)

// GroupRow is one public group.
type GroupRow struct {
	Name           string
	RateMultiplier float64
	PeakNote       string // optional, e.g. "高峰 ×1.5"
	Description    string
}

// GroupSection groups rows by platform (already ordered by the caller).
type GroupSection struct {
	Platform string // display name: Claude / OpenAI / Gemini ...
	Accent   string // optional hex "#d97706"; derived from Platform when empty
	Groups   []GroupRow
}

type GroupsInput struct {
	Title     string // default "公开分组"
	UpdatedAt time.Time
	Sections  []GroupSection
	Footer    string // default "详情见网站模型广场"
}

const (
	groupsW    = 760.0
	groupsPad  = 24.0
	sectionH   = 34.0
	groupRowH  = 38.0
	nameColW   = 190.0
	rateColW   = 70.0
	cardInsetX = 16.0
)

var platformAccents = map[string]uint32{
	"claude": 0xd97706, "anthropic": 0xd97706,
	"openai": 0x10a37f, "codex": 0x10a37f,
	"gemini": 0x4285f4, "antigravity": 0x7c3aed,
	"grok": 0x111827, "xai": 0x111827,
}

func accentFor(platform, hex string) color.RGBA {
	if v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32); hex != "" && err == nil {
		return rgb(uint32(v))
	}
	if v, ok := platformAccents[strings.ToLower(platform)]; ok {
		return rgb(v)
	}
	return rgb(0x64748b)
}

// mix blends the accent with white; frac is the accent share.
func mix(acc color.RGBA, frac float64) color.RGBA {
	m := func(v uint8) uint8 { return uint8(float64(v)*frac + 255*(1-frac) + 0.5) }
	return color.RGBA{R: m(acc.R), G: m(acc.G), B: m(acc.B), A: 0xff}
}

// tint mixes the accent with white (1/12 accent) for section header backgrounds.
func tint(acc color.RGBA) color.RGBA {
	m := func(v uint8) uint8 { return uint8(int(v)/12 + 0xff*11/12) }
	return color.RGBA{R: m(acc.R), G: m(acc.G), B: m(acc.B), A: 0xff}
}

func formatRate(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return "×" + s
}

// RenderGroups draws the public groups table (sectioned by platform) as PNG.
func RenderGroups(in GroupsInput) ([]byte, error) {
	if err := loadFonts(); err != nil {
		return nil, err
	}
	if in.Title == "" {
		in.Title = "公开分组"
	}
	if in.Footer == "" {
		in.Footer = "详情见网站模型广场"
	}
	total, rows := 0, 0
	for _, s := range in.Sections {
		total += len(s.Groups)
		rows += len(s.Groups)
	}
	headerH, footerH := 60.0, 36.0
	cardH := float64(len(in.Sections))*sectionH + float64(rows)*groupRowH + 8
	if total == 0 {
		cardH = 80
	}
	h := groupsPad + headerH + cardH + footerH + groupsPad/2
	c := newCanvas(groupsW, h, colPage)

	x, y := groupsPad, groupsPad
	tw := c.text(x, y+24, in.Title, true, 24, colText)
	c.text(x+tw+10, y+24, fmt.Sprintf("共 %d 个", total), false, 14, colMuted)
	c.text(x, y+46, "倍率越低越省，最终扣费 = 官方价 × 倍率", false, 12, colFaint)

	top := groupsPad + headerH
	cw := groupsW - 2*groupsPad
	c.borderRect(x, top, cw, cardH, 12, colCard, colBorder)
	if total == 0 {
		msg := "暂无公开分组"
		c.text((groupsW-c.measure(msg, false, 15))/2, c.baseline(top, cardH, false, 15), msg, false, 15, colMuted)
	}

	cy := top + 4
	ix := x + cardInsetX
	iw := cw - 2*cardInsetX
	for si, s := range in.Sections {
		if len(s.Groups) == 0 {
			continue
		}
		acc := accentFor(s.Platform, s.Accent)
		// Section header strip: accent bar + platform name + count.
		c.roundRect(x+8, cy+4, cw-16, sectionH-8, 6, tint(acc))
		c.roundRect(ix-2, cy+10, 3.5, sectionH-20, 1.75, acc)
		bl := c.baseline(cy+4, sectionH-8, true, 13)
		pw := c.text(ix+10, bl, s.Platform, true, 13, acc)
		c.text(ix+10+pw+8, bl, fmt.Sprintf("%d", len(s.Groups)), false, 12, colFaint)
		cy += sectionH
		for gi, g := range s.Groups {
			if gi > 0 {
				c.hline(ix, cy, iw, colBorder)
			}
			bl := c.baseline(cy, groupRowH, true, 14)
			c.text(ix+10, bl, c.truncate(g.Name, true, 14, nameColW-14), true, 14, colText)

			rate := formatRate(g.RateMultiplier)
			fg, rbg := colMuted, colUnkBg
			switch {
			case g.RateMultiplier < 1:
				fg, rbg = colOKText, colOKBg
			case g.RateMultiplier > 1:
				fg, rbg = colWarnText, colWarnBg
			}
			rx := ix + nameColW
			c.pill(rx, cy+(groupRowH-20)/2, rate, 12, fg, rbg)

			dx := rx + rateColW
			if g.PeakNote != "" {
				dx += c.text(dx, bl, g.PeakNote, false, 12, colWarnText) + 10
			}
			desc := strings.Join(strings.Fields(g.Description), " ")
			if desc == "" {
				desc = "—"
			}
			c.text(dx, c.baseline(cy, groupRowH, false, 13), c.truncate(desc, false, 13, ix+iw-dx-6), false, 13, colMuted)
			cy += groupRowH
		}
		_ = si
	}

	fy := top + cardH + 24
	c.text(groupsPad, fy, "更新于 "+in.UpdatedAt.Format("01-02 15:04"), false, 12, colFaint)
	c.textRight(groupsW-groupsPad, fy, in.Footer, false, 12, colFaint)
	return c.png()
}
