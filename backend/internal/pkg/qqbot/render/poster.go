package render

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"strings"
)

// PosterInput describes one invite poster. Link is encoded in the centered QR code.
type PosterInput struct {
	SiteName string
	Logo     []byte      // raw image bytes or data URI; SVG/unknown → initial badge
	Art      image.Image // AI illustration; nil → seeded procedural background
	Headline string
	Subline  string
	Rules    []string // up to 3 lines
	Domain   string   // footer text (no query string)
	Link     string   // QR content
	Code     string   // invite code; empty for the site poster
	Seed     int64
}

const (
	posterW = 540.0 // logical; 1080x1620 px at 2x
	posterH = 810.0
)

// RenderPoster composes the invite poster as PNG.
func RenderPoster(in PosterInput) ([]byte, error) {
	c, err := renderPoster(in)
	if err != nil {
		return nil, err
	}
	return c.png()
}

// RenderPosterJPEG composes the poster as JPEG (much smaller with photo-like art).
func RenderPosterJPEG(in PosterInput, quality int) ([]byte, error) {
	c, err := renderPoster(in)
	if err != nil {
		return nil, err
	}
	return c.jpeg(quality)
}

func renderPoster(in PosterInput) (*canvas, error) {
	if err := loadFonts(); err != nil {
		return nil, err
	}
	m, err := qrMatrix(in.Link)
	if err != nil {
		return nil, err
	}
	rng := posterRNG(in.Seed)
	c := newCanvas(posterW, posterH, colPage)
	if in.Art != nil {
		c.drawCover(0, 0, posterW, posterH, in.Art)
	} else {
		drawProceduralBackground(c, rng)
	}
	accent := accentFrom(c.averageColor(0, 0, posterW, posterH))
	logo := DecodeImage(in.Logo)

	drawSiteBadge(c, in.SiteName, logo, accent)
	drawCenterQR(c, m, logo, in.SiteName, in.Code, accent)
	drawInfoCard(c, in, accent)
	return c, nil
}

func drawSiteBadge(c *canvas, name string, logo image.Image, accent color.RGBA) {
	const x, y, h = 24.0, 24.0, 46.0
	name = c.truncate(strings.TrimSpace(name), true, 18, 300)
	w := 12 + 32 + 10 + c.measure(name, true, 18) + 18
	c.roundRectA(x, y+3, w, h, h/2, color.RGBA{0, 0, 0, 255}, 0.10)
	c.roundRectA(x, y, w, h, h/2, colCard, 0.9)
	drawLogoMark(c, x+12+16, y+h/2, 16, logo, name, accent)
	c.text(x+12+32+10, c.baseline(y, h, true, 18), name, true, 18, colText)
}

// drawLogoMark draws the site logo in a circle, or the first letter on the accent color.
func drawLogoMark(c *canvas, cx, cy, r float64, logo image.Image, name string, accent color.RGBA) {
	if logo != nil {
		c.drawRoundedImage(cx, cy, r, r*0.28, logo)
		return
	}
	c.circle(cx, cy, r, accent)
	initial := "S"
	if rs := []rune(strings.TrimSpace(name)); len(rs) > 0 {
		initial = strings.ToUpper(string(rs[0]))
	}
	size := r * 1.05
	c.text(cx-c.measure(initial, true, size)/2, c.baseline(cy-r, 2*r, true, size), initial, true, size, colCard)
}

func drawCenterQR(c *canvas, m [][]bool, logo image.Image, siteName, code string, accent color.RGBA) {
	const card = 236.0
	x, y := (posterW-card)/2, 196.0
	// soft shadow
	for i := 8.0; i >= 1; i-- {
		c.roundRectA(x-i, y-i+6, card+2*i, card+2*i, 22+i, color.RGBA{0, 0, 0, 255}, 0.018)
	}
	c.roundRect(x-3, y-3, card+6, card+6, 24, accent) // accent ring
	c.roundRect(x, y, card, card, 21, colCard)
	pad := 18.0
	qx, qy, qs := c.drawQR(x+pad, y+pad, card-2*pad, m, colText)
	// logo in the middle (≈20% width, well within level-H recovery)
	lr := qs * 0.105
	cx, cy := qx+qs/2, qy+qs/2
	c.roundRect(cx-lr-5, cy-lr-5, 2*lr+10, 2*lr+10, 9, colCard)
	drawLogoMark(c, cx, cy, lr, logo, siteName, accent)

	label := "扫码注册"
	if code != "" {
		label += " · 邀请码 " + code
	}
	pw := c.measure(label, true, 14) + 32
	py := y + card + 18
	c.roundRectA((posterW-pw)/2, py+3, pw, 34, 17, color.RGBA{0, 0, 0, 255}, 0.12)
	c.roundRect((posterW-pw)/2, py, pw, 34, 17, accent)
	c.text((posterW-pw)/2+16, c.baseline(py, 34, true, 14), label, true, 14, colCard)
}

func drawInfoCard(c *canvas, in PosterInput, accent color.RGBA) {
	rules := in.Rules
	if len(rules) > 3 {
		rules = rules[:3]
	}
	const x, w, pad = 24.0, posterW - 48, 22.0
	h := pad + 30 + 8 + float64(len(rules))*26 + 30 + pad
	if in.Subline != "" {
		h += 22
	}
	y := posterH - 24 - h
	c.roundRectA(x, y+4, w, h, 20, color.RGBA{0, 0, 0, 255}, 0.10)
	c.roundRect(x, y, w, h, 20, mix(accent, 0.55))
	c.roundRectA(x+1.5, y+1.5, w-3, h-3, 18.5, colCard, 0.94)

	ty := y + pad + 24
	c.roundRect(x+pad, ty-20, 4, 22, 2, accent)
	c.text(x+pad+12, ty, c.truncate(in.Headline, true, 24, w-2*pad-12), true, 24, colText)
	ty += 8
	if in.Subline != "" {
		ty += 22
		c.text(x+pad, ty, c.truncate(in.Subline, false, 14, w-2*pad), false, 14, colMuted)
	}
	for _, r := range rules {
		ty += 26
		c.circle(x+pad+4, ty-5, 3.5, accent)
		c.text(x+pad+14, ty, c.truncate(r, false, 15, w-2*pad-14), false, 15, colText)
	}
	fy := y + h - pad
	if in.Domain != "" {
		c.text(x+pad, fy, c.truncate(in.Domain, false, 12, w*0.6), false, 12, colFaint)
	}
	c.textRight(x+w-pad, fy, "长按识别二维码", false, 12, colFaint)
}

// ---- procedural backgrounds (no AI art) ----

type bgPalette struct {
	skyTop, skyBottom, sun color.RGBA
	layers                 []color.RGBA
}

var bgPalettes = []bgPalette{
	{rgb(0xe4eddd), rgb(0xf8f1e4), rgb(0xd6b574), []color.RGBA{rgb(0xabc595), rgb(0x7fa36f), rgb(0x4a7c59), rgb(0x2a4635)}}, // forest dawn
	{rgb(0xf4ead6), rgb(0xfbf7ee), rgb(0xbb9146), []color.RGBA{rgb(0xe7d2a9), rgb(0xd6b574), rgb(0x705c30), rgb(0x3d321f)}}, // amber dusk
	{rgb(0xdfeae6), rgb(0xf3f7ef), rgb(0xf4ead6), []color.RGBA{rgb(0xb9d2c6), rgb(0x86ab9a), rgb(0x4f7d6c), rgb(0x2b4a40)}}, // misty teal
}

func posterRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed)*0x9e3779b97f4a7c15+1))
}

// fallbackStyle reports which procedural style a seed produces (0/1 hills, 2 night, 3 orbs).
func fallbackStyle(seed int64) int { return posterRNG(seed).IntN(4) }

func drawProceduralBackground(c *canvas, rng *rand.Rand) {
	switch rng.IntN(4) {
	case 0, 1:
		drawHills(c, rng, bgPalettes[rng.IntN(len(bgPalettes))])
	case 2:
		drawNight(c, rng)
	default:
		drawOrbs(c, rng)
	}
}

func (c *canvas) vgradient(top, bottom color.RGBA) {
	b := c.img.Rect
	for py := b.Min.Y; py < b.Max.Y; py++ {
		t := float64(py-b.Min.Y) / float64(b.Dy())
		col := color.RGBA{lerp8(top.R, bottom.R, t), lerp8(top.G, bottom.G, t), lerp8(top.B, bottom.B, t), 255}
		for px := b.Min.X; px < b.Max.X; px++ {
			i := c.img.PixOffset(px, py)
			c.img.Pix[i], c.img.Pix[i+1], c.img.Pix[i+2], c.img.Pix[i+3] = col.R, col.G, col.B, 255
		}
	}
}

// softBlob paints a radial blob whose opacity fades smoothly to zero at radius r.
func (c *canvas) softBlob(cx, cy, r float64, col color.RGBA, alpha float64) {
	X, Y, R := cx*scale, cy*scale, r*scale
	for py := int(Y - R); py < int(Y+R); py++ {
		for px := int(X - R); px < int(X+R); px++ {
			d := math.Hypot(float64(px)+0.5-X, float64(py)+0.5-Y) / R
			if d >= 1 {
				continue
			}
			t := 1 - d
			c.blend(px, py, col, alpha*t*t*(3-2*t))
		}
	}
}

func lerp8(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }

// fillBelow fills everything under curve y=f(x) (logical) with anti-aliased top edge.
func (c *canvas) fillBelow(f func(x float64) float64, col color.RGBA, alpha float64) {
	b := c.img.Rect
	for px := b.Min.X; px < b.Max.X; px++ {
		edge := f((float64(px)+0.5)/scale) * scale
		start := int(math.Floor(edge))
		for py := max(start, b.Min.Y); py < b.Max.Y; py++ {
			cov := 1.0
			if py == start {
				cov = 1 - (edge - float64(start))
			}
			c.blend(px, py, col, cov*alpha)
		}
	}
}

func waveFunc(rng *rand.Rand, base, amp float64) func(float64) float64 {
	f1, f2 := 0.004+rng.Float64()*0.008, 0.015+rng.Float64()*0.02
	p1, p2 := rng.Float64()*6.28, rng.Float64()*6.28
	a2 := amp * (0.2 + rng.Float64()*0.3)
	return func(x float64) float64 { return base + amp*math.Sin(x*f1+p1) + a2*math.Sin(x*f2+p2) }
}

func drawHills(c *canvas, rng *rand.Rand, p bgPalette) {
	c.vgradient(p.skyTop, p.skyBottom)
	sx, sy := 80+rng.Float64()*380, 110+rng.Float64()*120
	sr := 46 + rng.Float64()*40
	for i := 3.0; i >= 1; i-- {
		c.roundRectA(sx-sr*(1+i*0.25), sy-sr*(1+i*0.25), 2*sr*(1+i*0.25), 2*sr*(1+i*0.25), sr*(1+i*0.25), p.sun, 0.08)
	}
	c.circle(sx, sy, sr, p.sun)
	for i := 0; i < 6; i++ { // a few birds-like dashes / clouds
		cx, cy := rng.Float64()*posterW, 60+rng.Float64()*220
		c.roundRectA(cx, cy, 40+rng.Float64()*60, 10, 5, colCard, 0.55)
	}
	for i, col := range p.layers {
		base := 470 + float64(i)*85 + rng.Float64()*20
		c.fillBelow(waveFunc(rng, base, 22+rng.Float64()*26), col, 1)
	}
}

func drawNight(c *canvas, rng *rand.Rand) {
	c.vgradient(rgb(0x101511), rgb(0x2a4635))
	for i := 0; i < 160; i++ {
		x, y := rng.Float64()*posterW, rng.Float64()*posterH*0.75
		r := 0.6 + rng.Float64()*1.4
		c.roundRectA(x-r, y-r, 2*r, 2*r, r, rgb(0xfbf7ee), 0.35+rng.Float64()*0.6)
	}
	mx, my, mr := 360+rng.Float64()*120, 90+rng.Float64()*60, 34.0
	c.roundRectA(mx-mr*1.8, my-mr*1.8, mr*3.6, mr*3.6, mr*1.8, rgb(0xf4ead6), 0.07)
	c.circle(mx, my, mr, rgb(0xf4ead6))
	c.circle(mx+14, my-8, mr*0.9, c.averageColor(mx+30, my-30, 6, 6)) // crescent cut
	for i, col := range []color.RGBA{rgb(0x273027), rgb(0x1a211b), rgb(0x101511)} {
		c.fillBelow(waveFunc(rng, 560+float64(i)*80, 26), col, 1)
	}
}

func drawOrbs(c *canvas, rng *rand.Rand) {
	c.vgradient(rgb(0xfaf6f0), rgb(0xf0ece4))
	cols := []color.RGBA{rgb(0x7fa36f), rgb(0x4a7c59), rgb(0xd6b574), rgb(0xbb9146), rgb(0xabc595)}
	for i := 0; i < 9; i++ {
		r := 60 + rng.Float64()*150
		x, y := rng.Float64()*posterW, rng.Float64()*posterH
		c.softBlob(x, y, r, cols[rng.IntN(len(cols))], 0.32)
	}
	for i := 0; i < 40; i++ {
		x, y := rng.Float64()*posterW, rng.Float64()*posterH
		c.circle(x, y, 1.5+rng.Float64()*2.5, cols[rng.IntN(len(cols))])
	}
}
