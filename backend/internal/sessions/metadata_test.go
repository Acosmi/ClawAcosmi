package sessions

import (
	"testing"
)

func TestMergeOrigin(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		got := MergeOrigin(nil, nil)
		if got != nil {
			t.Error("expected nil")
		}
	})
	t.Run("merge non-nil", func(t *testing.T) {
		existing := &SessionOrigin{Provider: "telegram", From: "alice"}
		next := &SessionOrigin{Provider: "slack", Surface: "api"}
		got := MergeOrigin(existing, next)
		if got.Provider != "slack" {
			t.Errorf("Provider = %q, want slack", got.Provider)
		}
		if got.From != "alice" {
			t.Errorf("From = %q, want alice", got.From)
		}
		if got.Surface != "api" {
			t.Errorf("Surface = %q, want api", got.Surface)
		}
	})
}

func TestDeriveSessionOrigin(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		ctx := MetadataContext{
			Provider:  "telegram",
			From:      "+1234567890",
			ChatType:  "dm",
			AccountID: "acc1",
		}
		got := DeriveSessionOrigin(ctx)
		if got == nil {
			t.Fatal("expected non-nil")
		}
		if got.Provider != "telegram" {
			t.Errorf("Provider = %q, want telegram", got.Provider)
		}
		if got.From != "+1234567890" {
			t.Errorf("From = %q", got.From)
		}
	})
	t.Run("all empty", func(t *testing.T) {
		got := DeriveSessionOrigin(MetadataContext{})
		if got != nil {
			t.Error("expected nil for empty context")
		}
	})
	t.Run("originating channel overrides provider", func(t *testing.T) {
		ctx := MetadataContext{
			Provider:           "old",
			OriginatingChannel: "whatsapp",
		}
		got := DeriveSessionOrigin(ctx)
		if got == nil || got.Provider != "whatsapp" {
			t.Errorf("expected provider=whatsapp, got %+v", got)
		}
	})
}

func TestDeriveGroupSessionPatch(t *testing.T) {
	t.Run("non-group returns nil", func(t *testing.T) {
		ctx := MetadataContext{From: "+1234567890"}
		got := DeriveGroupSessionPatch(ctx, "key1", nil, nil)
		if got != nil {
			t.Error("expected nil for non-group")
		}
	})
	t.Run("group returns patch", func(t *testing.T) {
		ctx := MetadataContext{
			From:         "telegram:group:test123",
			ChatType:     "group",
			GroupSubject: "My Group",
		}
		got := DeriveGroupSessionPatch(ctx, "key1", nil, nil)
		if got == nil {
			t.Fatal("expected non-nil")
		}
		if got["chatType"] != "group" {
			t.Errorf("chatType = %v, want group", got["chatType"])
		}
		if got["channel"] != "telegram" {
			t.Errorf("channel = %v, want telegram", got["channel"])
		}
	})
}

func TestResolveMirroredTranscriptText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		mediaURLs []string
		want      string
	}{
		{"text only", "hello world", nil, "hello world"},
		{"empty text", "", nil, ""},
		{"whitespace text", "  ", nil, ""},
		{"media urls", "", []string{"https://example.com/photo.jpg"}, "photo.jpg"},
		{"multiple media", "", []string{"https://a.com/img.png", "https://b.com/vid.mp4"}, "img.png, vid.mp4"},
		{"media without filename", "", []string{"https://example.com/"}, "media"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveMirroredTranscriptText(tt.text, tt.mediaURLs)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
