package api

import (
	"bytes"
	"image"
	"image/jpeg"

	"Argus-compound/go-sensory/internal/capture"
)

// --- Image conversion utilities ---

// frameToJPEG converts a BGRA frame to JPEG bytes.
func frameToJPEG(frame *capture.Frame, quality int) ([]byte, error) {
	w, h := frame.Width, frame.Height
	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	srcStride := frame.Stride
	if srcStride == 0 {
		srcStride = w * 4 // fallback if Stride not set
	}

	// Single-pass BGRA → RGBA conversion, stride-aware.
	// Each row: copy srcStride bytes, swap B↔R per pixel.
	for y := 0; y < h; y++ {
		srcRow := frame.Pixels[y*srcStride : y*srcStride+w*4]
		dstRow := img.Pix[y*img.Stride : y*img.Stride+w*4]

		// Bulk copy + in-place swap is faster than per-pixel assign
		copy(dstRow, srcRow)
		for i := 0; i < len(dstRow); i += 4 {
			dstRow[i+0], dstRow[i+2] = dstRow[i+2], dstRow[i+0] // B↔R swap
		}
	}

	// Pre-size buffer to avoid reallocation (estimate: ~10% of raw)
	var buf bytes.Buffer
	buf.Grow(w * h / 10)
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
