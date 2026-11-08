// Package render draws compact PNG cards (channel status, public groups) for the QQ bot.
// Pure Go: embedded CJK fonts + golang.org/x/image, no headless browser.
package render

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/NotoSansSC-Regular.ttf
var regularTTF []byte

//go:embed fonts/NotoSansSC-Bold.ttf
var boldTTF []byte

// scale renders at 2x pixel density so text stays sharp after QQ recompression.
const scale = 2.0

var (
	fontOnce    sync.Once
	regularFont *opentype.Font
	boldFont    *opentype.Font
	fontErr     error
)

func loadFonts() error {
	fontOnce.Do(func() {
		if regularFont, fontErr = opentype.Parse(regularTTF); fontErr != nil {
			return
		}
		boldFont, fontErr = opentype.Parse(boldTTF)
	})
	return fontErr
}

// Palette (light theme, aligned with the site's status colors).
var (
	colPage     = rgb(0xf3f4f6)
	colCard     = rgb(0xffffff)
	colBorder   = rgb(0xe5e7eb)
	colText     = rgb(0x111827)
	colMuted    = rgb(0x6b7280)
	colFaint    = rgb(0x9ca3af)
	colOK       = rgb(0x22c55e)
	colWarn     = rgb(0xf59e0b)
	colDown     = rgb(0xef4444)
	colUnknown  = rgb(0xcbd5e1)
	colOKBg     = rgb(0xdcfce7)
	colWarnBg   = rgb(0xfef3c7)
	colDownBg   = rgb(0xfee2e2)
	colUnkBg    = rgb(0xf1f5f9)
	colOKText   = rgb(0x15803d)
	colWarnText = rgb(0xb45309)
	colDownText = rgb(0xb91c1c)
)

func rgb(v uint32) color.RGBA {
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

// canvas works in logical (1x) coordinates; everything is multiplied by scale internally.
type canvas struct {
	img   *image.RGBA
	faces map[faceKey]font.Face
}

type faceKey struct {
	bold bool
	size float64
}

func newCanvas(w, h float64, bg color.RGBA) *canvas {
	img := image.NewRGBA(image.Rect(0, 0, int(math.Ceil(w*scale)), int(math.Ceil(h*scale))))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bg.R, bg.G, bg.B, 0xff
	}
	return &canvas{img: img, faces: map[faceKey]font.Face{}}
}

func (c *canvas) jpeg(quality int) ([]byte, error) {
	for _, f := range c.faces {
		_ = f.Close()
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, c.img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *canvas) png() ([]byte, error) {
	for _, f := range c.faces {
		_ = f.Close()
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, c.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *canvas) face(bold bool, size float64) font.Face {
	k := faceKey{bold, size}
	if f, ok := c.faces[k]; ok {
		return f
	}
	src := regularFont
	if bold {
		src = boldFont
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: size * scale, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err) // only fails on invalid size
	}
	c.faces[k] = f
	return f
}

// blend paints col over pixel (x,y) with coverage a in [0,1].
func (c *canvas) blend(x, y int, col color.RGBA, a float64) {
	if a <= 0 || !(image.Point{x, y}.In(c.img.Rect)) {
		return
	}
	if a > 1 {
		a = 1
	}
	i := c.img.PixOffset(x, y)
	p := c.img.Pix[i : i+3 : i+3]
	p[0] = uint8(float64(col.R)*a + float64(p[0])*(1-a) + 0.5)
	p[1] = uint8(float64(col.G)*a + float64(p[1])*(1-a) + 0.5)
	p[2] = uint8(float64(col.B)*a + float64(p[2])*(1-a) + 0.5)
}

// roundRect fills an anti-aliased rounded rectangle (logical coords).
func (c *canvas) roundRect(x, y, w, h, r float64, col color.RGBA) {
	c.roundRectA(x, y, w, h, r, col, 1)
}

// roundRectA is roundRect with an extra opacity factor in [0,1].
func (c *canvas) roundRectA(x, y, w, h, r float64, col color.RGBA, alpha float64) {
	X, Y, W, H, R := x*scale, y*scale, w*scale, h*scale, r*scale
	if R > W/2 {
		R = W / 2
	}
	if R > H/2 {
		R = H / 2
	}
	for py := int(math.Floor(Y)); py < int(math.Ceil(Y+H)); py++ {
		for px := int(math.Floor(X)); px < int(math.Ceil(X+W)); px++ {
			cx, cy := float64(px)+0.5, float64(py)+0.5
			// signed distance to rounded box
			qx := math.Abs(cx-(X+W/2)) - (W/2 - R)
			qy := math.Abs(cy-(Y+H/2)) - (H/2 - R)
			d := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - R
			c.blend(px, py, col, math.Min(math.Max(0.5-d, 0), 1)*alpha)
		}
	}
}

// borderRect draws a rounded card with a 1px (logical) border.
func (c *canvas) borderRect(x, y, w, h, r float64, fill, border color.RGBA) {
	c.roundRect(x, y, w, h, r, border)
	c.roundRect(x+0.5, y+0.5, w-1, h-1, r-0.5, fill)
}

func (c *canvas) circle(cx, cy, r float64, col color.RGBA) {
	c.roundRect(cx-r, cy-r, 2*r, 2*r, r, col)
}

func (c *canvas) hline(x, y, w float64, col color.RGBA) {
	c.roundRect(x, y, w, 0.5, 0, col)
}

// text draws s with its baseline at y and returns the advance width (logical).
func (c *canvas) text(x, y float64, s string, bold bool, size float64, col color.RGBA) float64 {
	f := c.face(bold, size)
	d := font.Drawer{Dst: c.img, Src: image.NewUniform(col), Face: f, Dot: fixed.Point26_6{X: fixed.Int26_6(x * scale * 64), Y: fixed.Int26_6(y * scale * 64)}}
	d.DrawString(s)
	return float64(d.MeasureString(s)) / 64 / scale
}

func (c *canvas) measure(s string, bold bool, size float64) float64 {
	return float64(font.MeasureString(c.face(bold, size), s)) / 64 / scale
}

// textRight draws s right-aligned to x.
func (c *canvas) textRight(x, y float64, s string, bold bool, size float64, col color.RGBA) {
	c.text(x-c.measure(s, bold, size), y, s, bold, size, col)
}

// baseline returns the baseline that vertically centers text of size within [top, top+h].
func (c *canvas) baseline(top, h float64, bold bool, size float64) float64 {
	m := c.face(bold, size).Metrics()
	asc, desc := float64(m.Ascent)/64/scale, float64(m.Descent)/64/scale
	return top + (h+asc-desc)/2
}

// truncate shortens s with "…" so it fits into maxW.
func (c *canvas) truncate(s string, bold bool, size, maxW float64) string {
	if c.measure(s, bold, size) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if t := string(r) + "…"; c.measure(t, bold, size) <= maxW {
			return t
		}
	}
	return "…"
}

// pill draws a rounded label and returns its width.
func (c *canvas) pill(x, top float64, s string, size float64, fg, bg color.RGBA) float64 {
	padX, h := size*0.6, size+8
	w := c.measure(s, true, size) + 2*padX
	c.roundRect(x, top, w, h, h/2, bg)
	c.text(x+padX, c.baseline(top, h, true, size), s, true, size, fg)
	return w
}
