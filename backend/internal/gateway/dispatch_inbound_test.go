package gateway

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

func TestExtractMediaListFromReplies_PrefersMediaItems(t *testing.T) {
	replies := []autoreply.ReplyPayload{
		{
			MediaItems: []autoreply.ReplyMediaItem{
				{MediaBase64: "a1", MediaMimeType: "image/png"},
				{MediaBase64: "a2", MediaMimeType: "image/jpeg"},
			},
			MediaBase64:   "legacy",
			MediaMimeType: "image/gif",
		},
	}

	items := ExtractMediaListFromReplies(replies)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Base64Data != "a1" || items[0].MimeType != "image/png" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].Base64Data != "a2" || items[1].MimeType != "image/jpeg" {
		t.Fatalf("unexpected second item: %+v", items[1])
	}
}

func TestExtractMediaListFromReplies_FallsBackToLegacyField(t *testing.T) {
	replies := []autoreply.ReplyPayload{
		{MediaBase64: "legacy", MediaMimeType: "image/png"},
	}

	items := ExtractMediaListFromReplies(replies)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Base64Data != "legacy" || items[0].MimeType != "image/png" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}

func TestExtractMediaListFromReplies_MediaItemMimeFallback(t *testing.T) {
	replies := []autoreply.ReplyPayload{
		{
			MediaMimeType: "image/webp",
			MediaItems: []autoreply.ReplyMediaItem{
				{MediaBase64: "img", MediaMimeType: ""},
				{MediaBase64: "", MediaMimeType: "image/png"},
			},
		},
	}

	items := ExtractMediaListFromReplies(replies)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].MimeType != "image/webp" {
		t.Fatalf("expected mime fallback image/webp, got %q", items[0].MimeType)
	}
}

func TestExtractMediaListFromReplies_EmptyMediaItemsFallbackToLegacy(t *testing.T) {
	replies := []autoreply.ReplyPayload{
		{
			MediaItems: []autoreply.ReplyMediaItem{
				{MediaBase64: "", MediaMimeType: "image/png"},
			},
			MediaBase64:   "legacy",
			MediaMimeType: "image/jpeg",
		},
	}

	items := ExtractMediaListFromReplies(replies)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Base64Data != "legacy" || items[0].MimeType != "image/jpeg" {
		t.Fatalf("unexpected fallback item: %+v", items[0])
	}
}
