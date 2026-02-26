package agent

// VLM communication helpers for ReActLoop.
// Split from react_loop.go for single-responsibility compliance.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	"Argus-compound/go-sensory/internal/vlm"
)

// captureJPEG grabs the latest frame, downscales for VLM, and encodes to JPEG.
// Uses Scaler.ForVLM to reduce token consumption (~87% savings at 1024px).
func (r *ReActLoop) captureJPEG() []byte {
	frame := r.capturer.LatestFrame()
	if frame == nil {
		log.Printf("[ReAct] No frame available")
		return nil
	}

	jpegBytes, err := r.scaler.ForVLM(frame)
	if err != nil {
		log.Printf("[ReAct] VLM image encode error: %v", err)
		return nil
	}
	return jpegBytes
}

// callVLMWithImage sends a screenshot + text prompt to the VLM (in-process).
// This replaces GoBridge.describe_screenshot() — zero HTTP overhead.
func (r *ReActLoop) callVLMWithImage(ctx context.Context, jpegData []byte, prompt string) (string, error) {
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

	return r.callVLM(ctx, chatReq)
}

// callVLM sends a chat request to the active VLM provider (in-process).
func (r *ReActLoop) callVLM(ctx context.Context, req vlm.ChatRequest) (string, error) {
	if r.vlmRouter == nil {
		return "", fmt.Errorf("no VLM router configured")
	}

	resp, err := r.vlmRouter.ActiveProvider().ChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty VLM response")
	}

	// Extract text content from Message.Content
	msg := resp.Choices[0].Message
	switch c := msg.Content.(type) {
	case string:
		return c, nil
	default:
		// If content is a complex type, marshal and return
		data, _ := json.Marshal(c)
		return string(data), nil
	}
}
