package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"strings"

	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// UIParser — VLM-based UI element detection + SoM grounding.
//
// Key design decisions:
// - Uses Go image/draw for SoM annotation (replaces PIL ImageDraw)
// - Uses basic font rendering (replaces PIL ImageFont)
// - VLM calls are in-process via vlm.Provider (zero HTTP)
// ──────────────────────────────────────────────────────────────

const detectPrompt = `Analyze this screenshot and identify ALL interactive UI elements. For each element, provide: type (button, text_field, checkbox, link, menu_item, tab, dropdown, icon, etc.), label (visible text or description), and approximate bounding box.

Return a JSON array:
[{"type": "button", "label": "Search", "x1": 100, "y1": 50, "x2": 200, "y2": 80}, ...]`

const somGroundPromptTpl = `This screenshot has numbered labels [0], [1], [2], etc. on UI elements.
Which numbered element best matches this description: %s

Respond with ONLY a JSON object: {"id": 3, "confidence": 0.9}`

// UIParser detects UI elements using AX API (primary) or VLM (fallback),
// and performs SoM-based grounding.
type UIParser struct {
	vlmProvider vlm.Provider
	scaler      *imaging.Scaler
	axClient    *RustAccessibility // nil = AX disabled, VLM only
}

// NewUIParser creates a parser with an optional AX client (primary)
// and VLM provider (fallback). Pass nil for axClient to disable AX detection.
func NewUIParser(provider vlm.Provider, scaler *imaging.Scaler, axClient *RustAccessibility) *UIParser {
	return &UIParser{vlmProvider: provider, scaler: scaler, axClient: axClient}
}

// DetectElements finds all UI elements in the screenshot.
// Strategy: AX API first (zero-latency, pixel-perfect), VLM fallback.
func (p *UIParser) DetectElements(ctx context.Context, screenshotJPEG []byte) ([]UIElement, error) {
	// Primary path: macOS Accessibility API via Rust FFI
	if p.axClient != nil {
		elements, err := p.axClient.FocusedAppElements()
		if err != nil {
			log.Printf("[UIParser] AX detection failed, falling back to VLM: %v", err)
		} else if len(elements) > 0 {
			log.Printf("[UIParser] [AX] Detected %d elements (zero-latency)", len(elements))
			return elements, nil
		} else {
			log.Printf("[UIParser] AX returned 0 elements, falling back to VLM")
		}
	}

	// Fallback path: VLM-based detection (original logic)
	return p.detectElementsVLM(ctx, screenshotJPEG)
}

// detectElementsVLM uses VLM to detect UI elements (fallback path).
func (p *UIParser) detectElementsVLM(ctx context.Context, screenshotJPEG []byte) ([]UIElement, error) {
	// Downscale input for VLM token savings
	scaledJPEG, err := p.scaler.ScaleJPEG(screenshotJPEG)
	if err != nil {
		log.Printf("[UIParser] Scale for VLM failed, using original: %v", err)
		scaledJPEG = screenshotJPEG
	}

	raw, err := p.callVLMWithImage(ctx, scaledJPEG, detectPrompt)
	if err != nil {
		return nil, fmt.Errorf("detect elements VLM call: %w", err)
	}

	// Parse JSON from response (handle markdown code blocks)
	cleaned := stripMarkdownFences(raw)

	// Find JSON array in response
	start := strings.Index(cleaned, "[")
	end := strings.LastIndex(cleaned, "]")
	if start >= 0 && end > start {
		cleaned = cleaned[start : end+1]
	}

	var rawElems []struct {
		Type         string  `json:"type"`
		Label        string  `json:"label"`
		X1           int     `json:"x1"`
		Y1           int     `json:"y1"`
		X2           int     `json:"x2"`
		Y2           int     `json:"y2"`
		Confidence   float64 `json:"confidence"`
		Interactable *bool   `json:"interactable"`
	}

	if err := json.Unmarshal([]byte(cleaned), &rawElems); err != nil {
		return nil, fmt.Errorf("parse element JSON: %w (raw: %.300s)", err, raw)
	}

	elements := make([]UIElement, 0, len(rawElems))
	for i, el := range rawElems {
		conf := el.Confidence
		if conf == 0 {
			conf = 0.8 // default confidence if not provided
		}
		interact := true
		if el.Interactable != nil {
			interact = *el.Interactable
		}

		elements = append(elements, UIElement{
			ID:           i,
			Type:         ParseElementType(el.Type),
			Label:        el.Label,
			X1:           el.X1,
			Y1:           el.Y1,
			X2:           el.X2,
			Y2:           el.Y2,
			Confidence:   conf,
			Interactable: interact,
		})
	}

	log.Printf("[UIParser] [VLM] Detected %d elements", len(elements))
	return elements, nil
}

// AnnotateSoM draws Set-of-Marks (SoM) annotations on the screenshot.
// Each element gets a colored bounding box and a numbered label.
// Returns JPEG bytes of the annotated image.
func (p *UIParser) AnnotateSoM(screenshotJPEG []byte, elements []UIElement) ([]byte, error) {
	// Decode JPEG to image
	img, err := jpeg.Decode(bytes.NewReader(screenshotJPEG))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}

	// Create mutable RGBA copy
	bounds := img.Bounds()
	annotated := image.NewRGBA(bounds)
	draw.Draw(annotated, bounds, img, bounds.Min, draw.Src)

	for _, elem := range elements {
		col := parseSoMColor(elem.ID)

		// Draw bounding box (2px border)
		drawRect(annotated, elem.X1, elem.Y1, elem.X2, elem.Y2, col, 2)

		// Draw label badge: [N]
		label := fmt.Sprintf("[%d]", elem.ID)
		labelW := len(label)*8 + 6 // approximate width: 8px per char + padding
		labelH := 16

		badgeX := elem.X1
		badgeY := elem.Y1 - labelH - 2
		if badgeY < 0 {
			badgeY = 0
		}

		// Fill badge background
		fillRect(annotated, badgeX, badgeY, badgeX+labelW, badgeY+labelH, col)

		// Draw label text (simple pixel font)
		drawText(annotated, badgeX+3, badgeY+2, label, color.White)
	}

	// Encode back to JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, annotated, &jpeg.Options{Quality: 80}); err != nil {
		return nil, fmt.Errorf("encode annotated image: %w", err)
	}
	return buf.Bytes(), nil
}

// GroundWithSoM performs the full SoM grounding pipeline:
// 1. Detect UI elements
// 2. Annotate screenshot with numbered labels
// 3. Ask VLM to pick the best matching label
// 4. Return the corresponding UIElement
//
// This converts coordinate regression (error-prone) into
// classification (much more reliable for VLMs).
func (p *UIParser) GroundWithSoM(ctx context.Context, screenshotJPEG []byte, prompt string) (*UIElement, error) {
	// 1. Detect elements
	elements, err := p.DetectElements(ctx, screenshotJPEG)
	if err != nil {
		return nil, err
	}
	if len(elements) == 0 {
		log.Printf("[UIParser] No elements detected for SoM grounding")
		return nil, nil
	}

	// 2. Annotate screenshot
	annotatedJPEG, err := p.AnnotateSoM(screenshotJPEG, elements)
	if err != nil {
		return nil, fmt.Errorf("annotate SoM: %w", err)
	}

	// 3. Downscale annotated image for VLM token savings
	scaledJPEG, err := p.scaler.ScaleJPEG(annotatedJPEG)
	if err != nil {
		log.Printf("[UIParser] Scale annotated image failed, using original: %v", err)
		scaledJPEG = annotatedJPEG
	}

	// 4. Ask VLM to classify
	groundPrompt := fmt.Sprintf(somGroundPromptTpl, prompt)
	raw, err := p.callVLMWithImage(ctx, scaledJPEG, groundPrompt)
	if err != nil {
		return nil, fmt.Errorf("SoM VLM call: %w", err)
	}

	// 4. Parse response
	cleaned := stripMarkdownFences(raw)
	var result struct {
		ID         *int    `json:"id"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		log.Printf("[UIParser] SoM response parse failed: %v", err)
		return nil, nil
	}

	if result.ID != nil {
		for _, el := range elements {
			if el.ID == *result.ID {
				log.Printf("[UIParser] SoM grounding matched: [%d] %s '%s'",
					el.ID, el.Type, el.Label)
				return &el, nil
			}
		}
	}

	log.Printf("[UIParser] SoM grounding found no match for: %s", prompt)
	return nil, nil
}

// ──────────────────────────────────────────────────────────────
// VLM helper (shared with react_loop.go via same pattern)
// ──────────────────────────────────────────────────────────────

func (p *UIParser) callVLMWithImage(ctx context.Context, jpegData []byte, prompt string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(jpegData)

	chatReq := vlm.ChatRequest{
		Messages: []vlm.Message{
			{
				Role: "user",
				Content: []vlm.ContentPart{
					{Type: "text", Text: prompt},
					{
						Type: "image_url",
						ImageURL: &vlm.ImageURL{
							URL: "data:image/jpeg;base64," + b64,
						},
					},
				},
			},
		},
		MaxTokens: 2048,
	}

	temp := 0.2
	chatReq.Temperature = &temp

	resp, err := p.vlmProvider.ChatCompletion(ctx, chatReq)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty VLM response")
	}

	msg := resp.Choices[0].Message
	switch c := msg.Content.(type) {
	case string:
		return c, nil
	default:
		data, _ := json.Marshal(c)
		return string(data), nil
	}
}
