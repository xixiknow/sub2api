package render

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	_ "image/gif"  // site logo formats
	_ "image/jpeg" // site logo / illustration formats
	_ "image/png"
	"math"
	"strings"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// DecodeImage accepts raw bytes or a data URI (data:image/png;base64,...).
// SVG and unknown formats return nil.
func DecodeImage(data []byte) image.Image {
	if len(data) == 0 {
		return nil
	}
	if s := string(data[:min(len(data), 64)]); strings.HasPrefix(s, "data:") {
		raw := string(data)
		i := strings.Index(raw, ",")
		if i < 0 || !strings.Contains(raw[:i], ";base64") {
			return nil
		}
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw[i+1:]))
		if err != nil {
			return nil
		}
		data = dec
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return img
}

// drawCover scales src to fully cover the logical rect, cropping the overflow (centered).
func (c *canvas) drawCover(x, y, w, h float64, src image.Image) {
	dst := image.Rect(int(x*scale), int(y*scale), int(math.Ceil((x+w)*scale)), int(math.Ceil((y+h)*scale)))
	sb := src.Bounds()
	sw, sh := float64(sb.Dx()), float64(sb.Dy())
	dw, dh := float64(dst.Dx()), float64(dst.Dy())
	k := math.Max(dw/sw, dh/sh)
	cw, ch := dw/k, dh/k
	ox, oy := (sw-cw)/2, (sh-ch)/2
	srcRect := image.Rect(sb.Min.X+int(ox), sb.Min.Y+int(oy), sb.Min.X+int(ox+cw), sb.Min.Y+int(oy+ch))
	xdraw.CatmullRom.Scale(c.img, dst, src, srcRect, xdraw.Over, nil)
}

// drawRoundedImage draws src (center-cropped square) into the square centered at
// (cx,cy) with half-size r, clipped to rounded corners of radius rad (rad=r → circle).
func (c *canvas) drawRoundedImage(cx, cy, r, rad float64, src image.Image) {
	size := int(math.Ceil(2 * r * scale))
	tmp := image.NewRGBA(image.Rect(0, 0, size, size))
	sb := src.Bounds()
	side := min(sb.Dx(), sb.Dy())
	sq := image.Rect(sb.Min.X+(sb.Dx()-side)/2, sb.Min.Y+(sb.Dy()-side)/2, 0, 0)
	sq.Max = sq.Min.Add(image.Pt(side, side))
	xdraw.CatmullRom.Scale(tmp, tmp.Bounds(), src, sq, xdraw.Src, nil)
	X0, Y0 := (cx-r)*scale, (cy-r)*scale
	R, RR := r*scale, rad*scale
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			qx := math.Abs(float64(px)+0.5-R) - (R - RR)
			qy := math.Abs(float64(py)+0.5-R) - (R - RR)
			d := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - RR
			cov := math.Min(math.Max(0.5-d, 0), 1)
			if cov == 0 {
				continue
			}
			p := tmp.RGBAAt(px, py)
			if p.A == 0 {
				continue
			}
			a := float64(p.A) / 255
			col := color.RGBA{R: unpremul(p.R, p.A), G: unpremul(p.G, p.A), B: unpremul(p.B, p.A), A: 255}
			c.blend(int(X0)+px, int(Y0)+py, col, cov*a)
		}
	}
}

func unpremul(v, a uint8) uint8 {
	if a == 0 {
		return 0
	}
	return uint8(min(255, int(v)*255/int(a)))
}

// averageColor samples a region of the canvas (logical coords).
func (c *canvas) averageColor(x, y, w, h float64) color.RGBA {
	var r, g, b, n float64
	step := 6
	for py := int(y * scale); py < int((y+h)*scale); py += step {
		for px := int(x * scale); px < int((x+w)*scale); px += step {
			if !(image.Point{px, py}.In(c.img.Rect)) {
				continue
			}
			p := c.img.RGBAAt(px, py)
			r, g, b, n = r+float64(p.R), g+float64(p.G), b+float64(p.B), n+1
		}
	}
	if n == 0 {
		return colText
	}
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 255}
}

func rgbToHSL(c color.RGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func hslToRGB(h, s, l float64) color.RGBA {
	if s == 0 {
		v := uint8(l * 255)
		return color.RGBA{v, v, v, 255}
	}
	hue := func(p, q, t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	return color.RGBA{uint8(hue(p, q, h+1.0/3) * 255), uint8(hue(p, q, h) * 255), uint8(hue(p, q, h-1.0/3) * 255), 255}
}

// accentFrom turns a sampled average into a readable, saturated accent color.
func accentFrom(avg color.RGBA) color.RGBA {
	h, s, _ := rgbToHSL(avg)
	if s < 0.08 { // near-grey artwork: fall back to the Terra forest green hue
		h, s = 0.38, 0.3
	}
	return hslToRGB(h, math.Min(math.Max(s, 0.45), 0.75), 0.36)
}
