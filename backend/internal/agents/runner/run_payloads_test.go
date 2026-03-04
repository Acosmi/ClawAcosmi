package runner

import "testing"

func TestBuildPayloads_AttachesMediaItemsAndLegacyFallback(t *testing.T) {
	attempt := &AttemptResult{
		AssistantTexts: []string{"done"},
		MediaBlocks: []MediaBlock{
			{MimeType: "image/png", Base64: "img1"},
			{MimeType: "image/jpeg", Base64: "img2"},
		},
	}

	payloads := buildPayloads(attempt)
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}

	p := payloads[0]
	if p.Text != "done" {
		t.Fatalf("expected text 'done', got %q", p.Text)
	}
	if len(p.MediaItems) != 2 {
		t.Fatalf("expected 2 media items, got %d", len(p.MediaItems))
	}
	if p.MediaItems[0].Base64 != "img1" || p.MediaItems[0].MimeType != "image/png" {
		t.Fatalf("unexpected first media item: %+v", p.MediaItems[0])
	}
	if p.MediaItems[1].Base64 != "img2" || p.MediaItems[1].MimeType != "image/jpeg" {
		t.Fatalf("unexpected second media item: %+v", p.MediaItems[1])
	}
	// 兼容旧字段：保留最后一项。
	if p.MediaBase64 != "img2" || p.MediaMimeType != "image/jpeg" {
		t.Fatalf("legacy media fallback mismatch: base64=%q mime=%q", p.MediaBase64, p.MediaMimeType)
	}
}

func TestBuildPayloads_MediaOnlyStillReturnsPayload(t *testing.T) {
	attempt := &AttemptResult{
		MediaBlocks: []MediaBlock{
			{MimeType: "image/png", Base64: "img1"},
		},
	}

	payloads := buildPayloads(attempt)
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].Text != "" {
		t.Fatalf("expected empty text, got %q", payloads[0].Text)
	}
	if len(payloads[0].MediaItems) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(payloads[0].MediaItems))
	}
	if payloads[0].MediaBase64 != "img1" || payloads[0].MediaMimeType != "image/png" {
		t.Fatalf("legacy fallback mismatch: base64=%q mime=%q", payloads[0].MediaBase64, payloads[0].MediaMimeType)
	}
}

func TestBuildPayloads_EmptyBase64BlocksAreIgnored(t *testing.T) {
	attempt := &AttemptResult{
		AssistantTexts: []string{"ok"},
		MediaBlocks: []MediaBlock{
			{MimeType: "image/png", Base64: ""},
		},
	}

	payloads := buildPayloads(attempt)
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if len(payloads[0].MediaItems) != 0 {
		t.Fatalf("expected empty media items, got %d", len(payloads[0].MediaItems))
	}
	if payloads[0].MediaBase64 != "" || payloads[0].MediaMimeType != "" {
		t.Fatalf("expected legacy media fields to stay empty, got base64=%q mime=%q", payloads[0].MediaBase64, payloads[0].MediaMimeType)
	}
}
