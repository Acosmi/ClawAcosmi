package agent

// Helper functions for the ReAct loop: parameter extraction, JPEG
// encoding, markdown fence stripping.  Split from react_loop.go to
// keep each file under 300 lines.

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"strings"

	"Argus-compound/go-sensory/internal/capture"
	"Argus-compound/go-sensory/internal/input"
)

// ──────────────────────────────────────────────────────────────
// Image helpers
// ──────────────────────────────────────────────────────────────

// frameToJPEGLocal encodes a capture.Frame to JPEG bytes.
// Handles BGRA→RGBA conversion and variable stride.
func frameToJPEGLocal(frame *capture.Frame, quality int) ([]byte, error) {
	w, h := frame.Width, frame.Height
	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	srcStride := frame.Stride
	if srcStride == 0 {
		srcStride = w * 4
	}

	for y := 0; y < h; y++ {
		srcRow := frame.Pixels[y*srcStride : y*srcStride+w*4]
		dstRow := img.Pix[y*img.Stride : y*img.Stride+w*4]
		copy(dstRow, srcRow)
		for i := 0; i < len(dstRow); i += 4 {
			dstRow[i+0], dstRow[i+2] = dstRow[i+2], dstRow[i+0] // BGRA→RGBA
		}
	}

	var buf bytes.Buffer
	buf.Grow(w * h / 10)
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ──────────────────────────────────────────────────────────────
// VLM output parsing
// ──────────────────────────────────────────────────────────────

// stripMarkdownFences removes ```json ... ``` wrappers from VLM output.
func stripMarkdownFences(raw string) string {
	raw = strings.TrimSpace(raw)

	if idx := strings.Index(raw, "```json"); idx >= 0 {
		raw = raw[idx+7:]
		if end := strings.Index(raw, "```"); end >= 0 {
			raw = raw[:end]
		}
	} else if idx := strings.Index(raw, "```"); idx >= 0 {
		raw = raw[idx+3:]
		if end := strings.Index(raw, "```"); end >= 0 {
			raw = raw[:end]
		}
	}

	return strings.TrimSpace(raw)
}

// ──────────────────────────────────────────────────────────────
// Parameter extraction (JSON-safe type coercion)
// ──────────────────────────────────────────────────────────────

// getIntParam safely extracts an int from params map.
func getIntParam(params map[string]any, key string, defaultVal int) int {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return defaultVal
}

// getFloatParam safely extracts a float from params map.
func getFloatParam(params map[string]any, key string, defaultVal float64) float64 {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return defaultVal
}

// getStringParam safely extracts a string from params map.
func getStringParam(params map[string]any, key, defaultVal string) string {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}

// getKeysParam extracts key codes from params.
// Handles both list of ints and list of floats (from JSON deserialization).
func getKeysParam(params map[string]any) []input.Key {
	v, ok := params["keys"]
	if !ok {
		return nil
	}
	switch keys := v.(type) {
	case []any:
		result := make([]input.Key, 0, len(keys))
		for _, k := range keys {
			switch n := k.(type) {
			case float64:
				result = append(result, input.Key(int(n)))
			case int:
				result = append(result, input.Key(n))
			}
		}
		return result
	}
	return nil
}
