// Package analysis implements temporal reasoning over keyframe sequences
// using the VLM system.
package analysis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// Data types (from temporal_analyzer.py)
// ──────────────────────────────────────────────────────────────

// TimelineEvent represents a significant event detected in the timeline.
type TimelineEvent struct {
	Timestamp     float64 `json:"timestamp"` // seconds since epoch or relative
	FrameNo       int     `json:"frame_no"`
	EventType     string  `json:"event_type"` // "change", "error", "action", "idle"
	Description   string  `json:"description"`
	Severity      string  `json:"severity"` // "info", "warning", "error", "critical"
	RelatedRegion *BBox   `json:"related_region,omitempty"`
}

// BBox represents a bounding box.
type BBox struct {
	X1 int `json:"x1"`
	Y1 int `json:"y1"`
	X2 int `json:"x2"`
	Y2 int `json:"y2"`
}

// TemporalAnalysis holds the result of analyzing a keyframe sequence.
type TemporalAnalysis struct {
	Summary        string          `json:"summary"`
	Events         []TimelineEvent `json:"events,omitempty"`
	RootCause      string          `json:"root_cause,omitempty"`
	Recommendation string          `json:"recommendation,omitempty"`
	Confidence     float64         `json:"confidence"`
	RawResponse    string          `json:"raw_response,omitempty"`
}

// FrameInput represents a single frame in a temporal sequence.
type FrameInput struct {
	JPEG      []byte  // JPEG image bytes
	Timestamp float64 // seconds (epoch or relative)
	FrameNo   int
}

// ──────────────────────────────────────────────────────────────
// TemporalAnalyzer
// ──────────────────────────────────────────────────────────────

// TemporalAnalyzer uses a VLM provider to perform causal reasoning
// over historical keyframe sequences.
//
// Key improvement over Python version:
// - Direct in-process VLM call (no HTTP round-trip via GoBridge)
// - No PIL dependency for base64 encoding
type TemporalAnalyzer struct {
	provider vlm.Provider
	model    string
	scaler   *imaging.Scaler
}

// NewTemporalAnalyzer creates an analyzer with the given VLM provider and image scaler.
func NewTemporalAnalyzer(provider vlm.Provider, model string, scaler *imaging.Scaler) *TemporalAnalyzer {
	if model == "" {
		model = "gemini-1.5-pro"
	}
	return &TemporalAnalyzer{
		provider: provider,
		model:    model,
		scaler:   scaler,
	}
}

// AnalyzeSequence performs temporal analysis over a sequence of keyframes.
func (a *TemporalAnalyzer) AnalyzeSequence(
	ctx context.Context,
	frames []FrameInput,
	query string,
	extraContext string,
) (*TemporalAnalysis, error) {
	if len(frames) == 0 {
		return &TemporalAnalysis{Summary: "No frames provided", Confidence: 0}, nil
	}

	chatReq := a.buildAnalysisPrompt(frames, query, extraContext)

	resp, err := a.provider.ChatCompletion(ctx, chatReq)
	if err != nil {
		log.Printf("[TemporalAnalyzer] VLM call failed: %v", err)
		return &TemporalAnalysis{
			Summary:     fmt.Sprintf("Analysis failed: %v", err),
			Confidence:  0,
			RawResponse: err.Error(),
		}, nil
	}

	if len(resp.Choices) == 0 {
		return &TemporalAnalysis{Summary: "Empty VLM response", Confidence: 0}, nil
	}

	return a.parseAnalysisResponse(resp)
}

// buildAnalysisPrompt constructs a multi-image VLM chat request with:
// 1. System context (frame count, time span)
// 2. Each frame as a labeled image with timestamp
// 3. The user's query + JSON response format
func (a *TemporalAnalyzer) buildAnalysisPrompt(
	frames []FrameInput,
	query string,
	extraContext string,
) vlm.ChatRequest {
	var contentParts []vlm.ContentPart

	// System text
	timeSpan := 0.0
	if len(frames) > 1 {
		timeSpan = frames[len(frames)-1].Timestamp - frames[0].Timestamp
	}
	systemText := fmt.Sprintf(
		"You are an expert visual analyst for a desktop monitoring system. "+
			"You are shown a sequence of screenshots captured over time. "+
			"Analyze the visual changes between frames to answer questions "+
			"about what happened and why.\n\n"+
			"Total frames: %d\n"+
			"Time span: %.1f seconds\n",
		len(frames), timeSpan,
	)
	if extraContext != "" {
		systemText += fmt.Sprintf("Context: %s\n", extraContext)
	}
	contentParts = append(contentParts, vlm.ContentPart{Type: "text", Text: systemText})

	// Frame images with labels (downscaled for VLM token savings)
	for _, f := range frames {
		contentParts = append(contentParts, vlm.ContentPart{
			Type: "text",
			Text: fmt.Sprintf("\n--- Frame #%d (t=%.1fs) ---", f.FrameNo, f.Timestamp),
		})

		// Downscale frame for VLM
		frameJPEG := f.JPEG
		if a.scaler != nil {
			if scaled, err := a.scaler.ScaleJPEG(f.JPEG); err == nil {
				frameJPEG = scaled
			}
		}

		b64 := base64.StdEncoding.EncodeToString(frameJPEG)
		contentParts = append(contentParts, vlm.ContentPart{
			Type: "image_url",
			ImageURL: &vlm.ImageURL{
				URL: "data:image/jpeg;base64," + b64,
			},
		})
	}

	// Query + response format
	responseFormat := fmt.Sprintf(
		"\n\nQuestion: %s\n\n"+
			"Respond in JSON format:\n"+
			"{\n"+
			"  \"summary\": \"brief summary of findings\",\n"+
			"  \"events\": [\n"+
			"    {\"timestamp\": float, \"frame_no\": int, "+
			"\"event_type\": \"change|error|action|idle\", "+
			"\"description\": \"...\", \"severity\": \"info|warning|error|critical\"}\n"+
			"  ],\n"+
			"  \"root_cause\": \"analysis of why this happened (or null)\",\n"+
			"  \"recommendation\": \"suggested action (or null)\",\n"+
			"  \"confidence\": float (0-1)\n"+
			"}", query,
	)
	contentParts = append(contentParts, vlm.ContentPart{Type: "text", Text: responseFormat})

	temp := 0.2
	return vlm.ChatRequest{
		Model: a.model,
		Messages: []vlm.Message{
			{Role: "user", Content: contentParts},
		},
		MaxTokens:   2048,
		Temperature: &temp,
	}
}

// parseAnalysisResponse extracts structured TemporalAnalysis from VLM response.
func (a *TemporalAnalyzer) parseAnalysisResponse(resp *vlm.ChatResponse) (*TemporalAnalysis, error) {
	rawText := ""
	switch c := resp.Choices[0].Message.Content.(type) {
	case string:
		rawText = c
	default:
		data, _ := json.Marshal(c)
		rawText = string(data)
	}

	var parsed struct {
		Summary string `json:"summary"`
		Events  []struct {
			Timestamp   float64 `json:"timestamp"`
			FrameNo     int     `json:"frame_no"`
			EventType   string  `json:"event_type"`
			Description string  `json:"description"`
			Severity    string  `json:"severity"`
		} `json:"events"`
		RootCause      *string `json:"root_cause"`
		Recommendation *string `json:"recommendation"`
		Confidence     float64 `json:"confidence"`
	}

	cleanedJSON := stripJSONFences(rawText)
	if err := json.Unmarshal([]byte(cleanedJSON), &parsed); err != nil {
		log.Printf("[TemporalAnalyzer] JSON parse failed: %v (raw: %.300s)", err, rawText)
		return &TemporalAnalysis{
			Summary:     rawText,
			Confidence:  0,
			RawResponse: rawText,
		}, nil
	}

	events := make([]TimelineEvent, 0, len(parsed.Events))
	for _, e := range parsed.Events {
		events = append(events, TimelineEvent{
			Timestamp:   e.Timestamp,
			FrameNo:     e.FrameNo,
			EventType:   e.EventType,
			Description: e.Description,
			Severity:    e.Severity,
		})
	}

	result := &TemporalAnalysis{
		Summary:     parsed.Summary,
		Events:      events,
		Confidence:  parsed.Confidence,
		RawResponse: rawText,
	}
	if parsed.RootCause != nil {
		result.RootCause = *parsed.RootCause
	}
	if parsed.Recommendation != nil {
		result.Recommendation = *parsed.Recommendation
	}

	log.Printf("[TemporalAnalyzer] Analysis complete: %d events, confidence=%.2f",
		len(events), parsed.Confidence)
	return result, nil
}

// ExplainChange is a shorthand for comparing two frames.
func (a *TemporalAnalyzer) ExplainChange(
	ctx context.Context,
	beforeJPEG, afterJPEG []byte,
	question string,
) (*TemporalAnalysis, error) {
	if question == "" {
		question = "What changed between these two screenshots?"
	}
	return a.AnalyzeSequence(ctx, []FrameInput{
		{JPEG: beforeJPEG, Timestamp: 0, FrameNo: 0},
		{JPEG: afterJPEG, Timestamp: 1, FrameNo: 1},
	}, question, "")
}

// stripJSONFences removes ```json...``` markdown wrappers.
func stripJSONFences(raw string) string {
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		raw = raw[idx+7:]
		if end := strings.Index(raw, "```"); end >= 0 {
			raw = raw[:end]
		}
		return strings.TrimSpace(raw)
	}
	if idx := strings.Index(raw, "```"); idx >= 0 {
		raw = raw[idx+3:]
		if end := strings.Index(raw, "```"); end >= 0 {
			raw = raw[:end]
		}
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(raw)
}
