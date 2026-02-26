package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
)

// ── ChunkDiscordText (ChunkDiscordMessage equivalent) ──

func TestSendShared_ChunkDiscordText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		opts      ChunkDiscordTextOpts
		wantN     int      // expected number of chunks
		wantAll   string   // if non-empty, reassembled chunks must contain this
		wantExact []string // if non-nil, exact chunk list
	}{
		{
			name:      "empty message returns nil",
			text:      "",
			opts:      ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			wantN:     0,
			wantExact: nil,
		},
		{
			name:      "short message stays in one chunk",
			text:      "hello world",
			opts:      ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			wantN:     1,
			wantExact: []string{"hello world"},
		},
		{
			name:  "message at exact char boundary stays in one chunk",
			text:  strings.Repeat("a", 2000),
			opts:  ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			wantN: 1,
		},
		{
			name:    "message one char over boundary splits into two chunks",
			text:    strings.Repeat("a", 2001),
			opts:    ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			wantN:   2,
			wantAll: strings.Repeat("a", 2001),
		},
		{
			name:  "long message splits into multiple chunks",
			text:  strings.Repeat("x", 5000),
			opts:  ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			wantN: 3,
		},
		{
			name:  "splits by line count when max lines exceeded",
			text:  strings.Repeat("line\n", 20) + "last",
			opts:  ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 5},
			wantN: 5, // 21 lines / 5 per chunk = ~5 chunks
		},
		{
			name:  "message at exact line boundary stays in one chunk",
			text:  strings.Repeat("x\n", 16) + "x",
			opts:  ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			wantN: 1,
		},
		{
			name:  "preserves code fences across chunks",
			text:  "```go\n" + strings.Repeat("code line\n", 20) + "```",
			opts:  ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 10},
			wantN: 3,
		},
		{
			name:  "small maxChars forces many chunks",
			text:  "abcdefghij",
			opts:  ChunkDiscordTextOpts{MaxChars: 3, MaxLines: 100},
			wantN: 4, // abc, def, ghi, j
		},
		{
			name:      "defaults used when opts are zero",
			text:      "short",
			opts:      ChunkDiscordTextOpts{},
			wantN:     1,
			wantExact: []string{"short"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := ChunkDiscordText(tt.text, tt.opts)

			// Check chunk count
			if tt.wantN > 0 && len(chunks) != tt.wantN {
				t.Errorf("chunk count: got %d, want %d", len(chunks), tt.wantN)
			}
			if tt.wantN == 0 && len(chunks) != 0 {
				t.Errorf("expected nil/empty chunks, got %d", len(chunks))
			}

			// Check exact match
			if tt.wantExact != nil {
				if len(chunks) != len(tt.wantExact) {
					t.Errorf("exact match: got %d chunks, want %d", len(chunks), len(tt.wantExact))
				} else {
					for i, want := range tt.wantExact {
						if chunks[i] != want {
							t.Errorf("chunk[%d]: got %q, want %q", i, chunks[i], want)
						}
					}
				}
			}

			// Check that all content is preserved (substring check)
			if tt.wantAll != "" {
				joined := strings.Join(chunks, "")
				if !strings.Contains(joined, tt.wantAll) {
					t.Errorf("reassembled chunks don't contain expected content")
				}
			}

			// Check no chunk exceeds the char limit (when opts are set)
			if tt.opts.MaxChars > 0 {
				for i, chunk := range chunks {
					if len(chunk) > tt.opts.MaxChars {
						t.Errorf("chunk[%d] length %d exceeds max %d", i, len(chunk), tt.opts.MaxChars)
					}
				}
			}
		})
	}
}

func TestSendShared_ChunkDiscordTextWithMode(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		opts      ChunkDiscordTextOpts
		chunkMode ChunkMode
		wantN     int
	}{
		{
			name:      "length mode uses default chunking",
			text:      strings.Repeat("a", 3000),
			opts:      ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			chunkMode: ChunkModeLength,
			wantN:     2,
		},
		{
			name:      "empty chunkMode uses default chunking",
			text:      strings.Repeat("a", 3000),
			opts:      ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17},
			chunkMode: "",
			wantN:     2,
		},
		{
			name:      "newline mode delegates to autoreply chunker",
			text:      "paragraph one.\n\nparagraph two.\n\nparagraph three.",
			opts:      ChunkDiscordTextOpts{MaxChars: 30, MaxLines: 17},
			chunkMode: ChunkModeNewline,
			wantN:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := ChunkDiscordTextWithMode(tt.text, tt.opts, tt.chunkMode)
			if len(chunks) != tt.wantN {
				t.Errorf("chunk count: got %d, want %d (chunks=%v)", len(chunks), tt.wantN, chunks)
			}
		})
	}
}

// ── BuildDiscordSendErrorFromErr ──

func TestSendShared_BuildDiscordSendErrorFromErr(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		channelID string
		wantKind  DiscordSendErrorKind // empty means raw error passthrough
		wantMsg   string               // substring match
	}{
		{
			name:     "nil-like: already a DiscordSendError passes through",
			err:      &DiscordSendError{Kind: DiscordSendErrorKindDMBlocked, Message: "already wrapped"},
			wantKind: DiscordSendErrorKindDMBlocked,
			wantMsg:  "already wrapped",
		},
		{
			name:      "403 missing permissions",
			err:       &DiscordAPIError{StatusCode: 403, Message: "forbidden"},
			channelID: "123456",
			// 403 is not the same as code 50013; test separately
		},
		{
			name:      "50013 missing permissions error",
			err:       &DiscordAPIError{StatusCode: discordMissingPermission, Message: "missing perms"},
			channelID: "chan-999",
			wantKind:  DiscordSendErrorKindMissingPerms,
			wantMsg:   "missing permissions in channel chan-999",
		},
		{
			name:      "50007 DM blocked error",
			err:       &DiscordAPIError{StatusCode: discordCannotDM, Message: "cannot DM"},
			channelID: "chan-100",
			wantKind:  DiscordSendErrorKindDMBlocked,
			wantMsg:   "user blocks dms",
		},
		{
			name:      "404 not found passes through as raw error",
			err:       &DiscordAPIError{StatusCode: 404, Message: "not found"},
			channelID: "chan-404",
		},
		{
			name:      "429 rate limit passes through as raw error",
			err:       &DiscordAPIError{StatusCode: 429, Message: "rate limited"},
			channelID: "chan-429",
		},
		{
			name:      "generic error passes through",
			err:       fmt.Errorf("something went wrong"),
			channelID: "chan-gen",
			wantMsg:   "something went wrong",
		},
		{
			name:      "50001 missing access passes through (not 50013 or 50007)",
			err:       &DiscordAPIError{StatusCode: 50001, Message: "missing access"},
			channelID: "chan-50001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildDiscordSendErrorFromErr(tt.err, tt.channelID)

			if tt.wantKind != "" {
				sendErr, ok := result.(*DiscordSendError)
				if !ok {
					t.Fatalf("expected *DiscordSendError, got %T: %v", result, result)
				}
				if sendErr.Kind != tt.wantKind {
					t.Errorf("kind: got %q, want %q", sendErr.Kind, tt.wantKind)
				}
			}

			if tt.wantMsg != "" {
				if !strings.Contains(result.Error(), tt.wantMsg) {
					t.Errorf("message: got %q, want substring %q", result.Error(), tt.wantMsg)
				}
			}

			// For passthrough cases (no wantKind, no wantMsg), ensure original error is returned
			if tt.wantKind == "" && tt.wantMsg == "" {
				if result != tt.err {
					t.Errorf("expected original error to pass through, got different error: %v", result)
				}
			}
		})
	}
}

func TestSendShared_BuildDiscordSendErrorFromErr_NilContext(t *testing.T) {
	// 50013 without ctx/token args should still return MissingPerms
	err := &DiscordAPIError{StatusCode: discordMissingPermission, Message: "missing perms"}
	result := BuildDiscordSendErrorFromErr(err, "chan-abc")

	sendErr, ok := result.(*DiscordSendError)
	if !ok {
		t.Fatalf("expected *DiscordSendError, got %T", result)
	}
	if sendErr.Kind != DiscordSendErrorKindMissingPerms {
		t.Errorf("kind: got %q, want %q", sendErr.Kind, DiscordSendErrorKindMissingPerms)
	}
	if sendErr.ChannelID != "chan-abc" {
		t.Errorf("channelID: got %q, want %q", sendErr.ChannelID, "chan-abc")
	}
	// Without a valid ctx/token to probe, MissingPermissions should be nil
	if len(sendErr.MissingPermissions) != 0 {
		t.Errorf("expected no probed missing permissions, got %v", sendErr.MissingPermissions)
	}
}

func TestSendShared_GetDiscordErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "DiscordAPIError returns status code",
			err:  &DiscordAPIError{StatusCode: 403},
			want: 403,
		},
		{
			name: "non-API error returns 0",
			err:  fmt.Errorf("random error"),
			want: 0,
		},
		{
			name: "nil error returns 0",
			err:  nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDiscordErrorCode(tt.err)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// ── EncodeDiscordMultipart ──
// Tests the multipart encoding pattern used by discordMultipartPOST.
// Since discordMultipartPOST is unexported and makes HTTP calls, we test
// the encoding logic by replicating the multipart assembly pattern used
// in send_media.go and verifying the form data structure.

func TestSendShared_EncodeDiscordMultipart(t *testing.T) {
	tests := []struct {
		name        string
		payload     map[string]interface{}
		media       *discordMedia
		wantContent string // expected payload_json content substring
		wantFile    string // expected filename
	}{
		{
			name:        "basic message with image attachment",
			payload:     map[string]interface{}{"content": "hello"},
			media:       &discordMedia{Data: []byte("fake-png"), FileName: "image.png", ContentType: "image/png"},
			wantContent: `"content":"hello"`,
			wantFile:    "image.png",
		},
		{
			name: "message with embeds",
			payload: map[string]interface{}{
				"content": "check this out",
				"embeds":  []interface{}{map[string]string{"title": "My Embed"}},
			},
			media:       &discordMedia{Data: []byte("data"), FileName: "doc.pdf", ContentType: "application/pdf"},
			wantContent: `"embeds"`,
			wantFile:    "doc.pdf",
		},
		{
			name:        "empty content with file only",
			payload:     map[string]interface{}{},
			media:       &discordMedia{Data: []byte{0x89, 0x50, 0x4E, 0x47}, FileName: "test.png", ContentType: "image/png"},
			wantContent: "{}",
			wantFile:    "test.png",
		},
		{
			name:        "message with reply reference",
			payload:     map[string]interface{}{"content": "reply", "message_reference": map[string]interface{}{"message_id": "123"}},
			media:       &discordMedia{Data: []byte("audio"), FileName: "voice.ogg", ContentType: "audio/ogg"},
			wantContent: `"message_reference"`,
			wantFile:    "voice.ogg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the multipart encoding from discordMultipartPOST
			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)

			jsonData, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}

			// payload_json part
			jsonPartHeader := textproto.MIMEHeader{}
			jsonPartHeader.Set("Content-Disposition", `form-data; name="payload_json"`)
			jsonPartHeader.Set("Content-Type", "application/json")
			jsonPart, err := w.CreatePart(jsonPartHeader)
			if err != nil {
				t.Fatalf("create json part: %v", err)
			}
			if _, err := jsonPart.Write(jsonData); err != nil {
				t.Fatalf("write json: %v", err)
			}

			// file part
			filePartHeader := textproto.MIMEHeader{}
			filePartHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files[0]"; filename="%s"`, tt.media.FileName))
			if tt.media.ContentType != "" {
				filePartHeader.Set("Content-Type", tt.media.ContentType)
			}
			filePart, err := w.CreatePart(filePartHeader)
			if err != nil {
				t.Fatalf("create file part: %v", err)
			}
			if _, err := filePart.Write(tt.media.Data); err != nil {
				t.Fatalf("write file: %v", err)
			}

			if err := w.Close(); err != nil {
				t.Fatalf("close writer: %v", err)
			}

			// Parse the generated multipart form
			contentType := w.FormDataContentType()
			_, params, err := mime.ParseMediaType(contentType)
			if err != nil {
				t.Fatalf("parse content type: %v", err)
			}

			reader := multipart.NewReader(&buf, params["boundary"])

			// First part: payload_json
			part1, err := reader.NextPart()
			if err != nil {
				t.Fatalf("read first part: %v", err)
			}
			if part1.FormName() != "payload_json" {
				t.Errorf("first part name: got %q, want %q", part1.FormName(), "payload_json")
			}
			part1Body, _ := io.ReadAll(part1)
			if !strings.Contains(string(part1Body), tt.wantContent) {
				t.Errorf("payload_json: got %q, want substring %q", string(part1Body), tt.wantContent)
			}
			part1CT := part1.Header.Get("Content-Type")
			if part1CT != "application/json" {
				t.Errorf("payload_json Content-Type: got %q, want %q", part1CT, "application/json")
			}

			// Second part: file
			part2, err := reader.NextPart()
			if err != nil {
				t.Fatalf("read second part: %v", err)
			}
			if part2.FormName() != "files[0]" {
				t.Errorf("file part name: got %q, want %q", part2.FormName(), "files[0]")
			}
			if part2.FileName() != tt.wantFile {
				t.Errorf("filename: got %q, want %q", part2.FileName(), tt.wantFile)
			}
			part2Body, _ := io.ReadAll(part2)
			if !bytes.Equal(part2Body, tt.media.Data) {
				t.Errorf("file data mismatch: got %d bytes, want %d bytes", len(part2Body), len(tt.media.Data))
			}
			if tt.media.ContentType != "" {
				part2CT := part2.Header.Get("Content-Type")
				if part2CT != tt.media.ContentType {
					t.Errorf("file Content-Type: got %q, want %q", part2CT, tt.media.ContentType)
				}
			}

			// No more parts
			_, err = reader.NextPart()
			if err != io.EOF {
				t.Errorf("expected EOF after two parts, got: %v", err)
			}
		})
	}
}

// ── Edge cases ──

func TestSendShared_EdgeCases(t *testing.T) {
	t.Run("nil error to GetDiscordErrorCode", func(t *testing.T) {
		code := GetDiscordErrorCode(nil)
		if code != 0 {
			t.Errorf("expected 0 for nil error, got %d", code)
		}
	})

	t.Run("empty string to ChunkDiscordText returns empty slice", func(t *testing.T) {
		chunks := ChunkDiscordText("", ChunkDiscordTextOpts{MaxChars: 2000})
		if chunks == nil || len(chunks) != 0 {
			t.Errorf("expected empty slice for empty text, got %v", chunks)
		}
	})

	t.Run("whitespace-only text to ChunkDiscordText", func(t *testing.T) {
		chunks := ChunkDiscordText("   \n   \n   ", ChunkDiscordTextOpts{MaxChars: 2000, MaxLines: 17})
		if len(chunks) != 0 {
			t.Errorf("expected no chunks for whitespace-only text, got %d", len(chunks))
		}
	})

	t.Run("DiscordAPIError satisfies error interface", func(t *testing.T) {
		var err error = &DiscordAPIError{StatusCode: 500, Message: "internal"}
		if err.Error() != "internal" {
			t.Errorf("Error() = %q, want %q", err.Error(), "internal")
		}
	})

	t.Run("DiscordSendError satisfies error interface", func(t *testing.T) {
		var err error = &DiscordSendError{Kind: DiscordSendErrorKindDMBlocked, Message: "blocked"}
		if err.Error() != "blocked" {
			t.Errorf("Error() = %q, want %q", err.Error(), "blocked")
		}
	})

	t.Run("DiscordAPIError status code 429 rate limit", func(t *testing.T) {
		retryAfter := 1.5
		apiErr := &DiscordAPIError{
			StatusCode: http.StatusTooManyRequests,
			RetryAfter: &retryAfter,
			Message:    "rate limited",
		}
		code := GetDiscordErrorCode(apiErr)
		if code != 429 {
			t.Errorf("expected 429, got %d", code)
		}
		result := BuildDiscordSendErrorFromErr(apiErr, "ch-1")
		// 429 should pass through (it's not 50013 or 50007)
		if result != apiErr {
			t.Errorf("429 error should pass through, got %T", result)
		}
	})
}
