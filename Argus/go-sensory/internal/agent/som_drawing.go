package agent

// SoM (Set-of-Mark) drawing primitives for UI element annotation.
// Split from ui_parser.go for single-responsibility compliance.
// These replace PIL ImageDraw functionality with Go standard library.

import (
	"fmt"
	"image"
	"image/color"
)

// drawRect draws a rectangle outline with the given thickness.
func drawRect(img *image.RGBA, x1, y1, x2, y2 int, col color.RGBA, thickness int) {
	for t := 0; t < thickness; t++ {
		// Top and bottom edges
		for x := x1; x <= x2; x++ {
			img.SetRGBA(x, y1+t, col)
			img.SetRGBA(x, y2-t, col)
		}
		// Left and right edges
		for y := y1; y <= y2; y++ {
			img.SetRGBA(x1+t, y, col)
			img.SetRGBA(x2-t, y, col)
		}
	}
}

// fillRect fills a solid rectangle.
func fillRect(img *image.RGBA, x1, y1, x2, y2 int, col color.RGBA) {
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			img.SetRGBA(x, y, col)
		}
	}
}

// drawText renders simple text at the given position using a basic pixel font.
// This is a minimal implementation — for production, use golang.org/x/image/font.
func drawText(img *image.RGBA, x, y int, text string, col color.Color) {
	// Simple 5x7 digit + bracket font for SoM labels [0]-[99]
	r, g, b, a := col.RGBA()
	rgba := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}

	cx := x
	for _, ch := range text {
		glyph := getGlyph(ch)
		if glyph == nil {
			cx += 6
			continue
		}
		for row, bits := range glyph {
			for col := 0; col < 5; col++ {
				if bits&(1<<(4-col)) != 0 {
					img.SetRGBA(cx+col, y+row, rgba)
				}
			}
		}
		cx += 6
	}
}

// getGlyph returns a 7-row bitmap for the given character (5 bits wide).
// Only covers digits 0-9 and brackets [] needed for SoM labels.
func getGlyph(ch rune) []byte {
	switch ch {
	case '0':
		return []byte{0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E}
	case '1':
		return []byte{0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E}
	case '2':
		return []byte{0x0E, 0x11, 0x01, 0x06, 0x08, 0x10, 0x1F}
	case '3':
		return []byte{0x0E, 0x11, 0x01, 0x06, 0x01, 0x11, 0x0E}
	case '4':
		return []byte{0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02}
	case '5':
		return []byte{0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E}
	case '6':
		return []byte{0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E}
	case '7':
		return []byte{0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08}
	case '8':
		return []byte{0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E}
	case '9':
		return []byte{0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C}
	case '[':
		return []byte{0x0E, 0x08, 0x08, 0x08, 0x08, 0x08, 0x0E}
	case ']':
		return []byte{0x0E, 0x02, 0x02, 0x02, 0x02, 0x02, 0x0E}
	}
	return nil
}

// parseSoMColor converts a SoM color index to an RGBA color.
func parseSoMColor(id int) color.RGBA {
	hex := SoMColors[id%len(SoMColors)]
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{r, g, b, 255}
}
