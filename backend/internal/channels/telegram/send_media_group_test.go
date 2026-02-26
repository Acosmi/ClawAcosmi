package telegram

import (
	"context"
	"net/http"
	"testing"
)

func TestSendMediaGroup_EmptyMedia(t *testing.T) {
	_, err := SendMediaGroup(context.Background(), http.DefaultClient, "123", nil, SendMediaGroupOpts{Token: "tok"})
	if err == nil {
		t.Fatal("expected error for empty media")
	}
	if err.Error() != "media group must contain at least one item" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMediaGroup_TooManyItems(t *testing.T) {
	media := make([]InputMedia, 11)
	for i := range media {
		media[i] = InputMedia{Type: InputMediaPhoto, Media: "https://example.com/img.jpg"}
	}
	_, err := SendMediaGroup(context.Background(), http.DefaultClient, "123", media, SendMediaGroupOpts{Token: "tok"})
	if err == nil {
		t.Fatal("expected error for >10 items")
	}
}

func TestInputMediaTypes(t *testing.T) {
	// Verify type constants are correct Telegram API values
	if InputMediaPhoto != "photo" {
		t.Error("InputMediaPhoto should be 'photo'")
	}
	if InputMediaVideo != "video" {
		t.Error("InputMediaVideo should be 'video'")
	}
	if InputMediaDocument != "document" {
		t.Error("InputMediaDocument should be 'document'")
	}
	if InputMediaAudio != "audio" {
		t.Error("InputMediaAudio should be 'audio'")
	}
}
