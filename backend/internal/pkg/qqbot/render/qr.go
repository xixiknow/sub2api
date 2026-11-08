package render

import (
	"image/color"
	"math"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

// qrMatrix encodes content with high error correction (a logo covers the center).
func qrMatrix(content string) ([][]bool, error) {
	code, err := qr.Encode(content, qr.H, qr.Auto)
	if err != nil {
		return nil, err
	}
	return barcodeMatrix(code), nil
}

func barcodeMatrix(code barcode.Barcode) [][]bool {
	b := code.Bounds()
	m := make([][]bool, b.Dy())
	for y := range m {
		m[y] = make([]bool, b.Dx())
		for x := range m[y] {
			r, _, _, _ := code.At(b.Min.X+x, b.Min.Y+y).RGBA()
			m[y][x] = r < 0x8000
		}
	}
	return m
}

// drawQR draws the matrix inside the logical square (x,y,size) with crisp,
// device-pixel-aligned modules. Returns the actual drawn square (logical).
func (c *canvas) drawQR(x, y, size float64, m [][]bool, fg color.RGBA) (float64, float64, float64) {
	n := len(m)
	mod := math.Floor(size * scale / float64(n)) // device pixels per module
	if mod < 1 {
		mod = 1
	}
	total := mod * float64(n)
	ox := math.Round(x*scale + (size*scale-total)/2)
	oy := math.Round(y*scale + (size*scale-total)/2)
	for row := 0; row < n; row++ {
		for col := 0; col < n; col++ {
			if !m[row][col] {
				continue
			}
			x0, y0 := int(ox+float64(col)*mod), int(oy+float64(row)*mod)
			for py := y0; py < y0+int(mod); py++ {
				for px := x0; px < x0+int(mod); px++ {
					c.blend(px, py, fg, 1)
				}
			}
		}
	}
	return ox / scale, oy / scale, total / scale
}
