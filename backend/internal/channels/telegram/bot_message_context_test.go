package telegram

import (
	"strings"
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------------------------------------------------------------------------
// Test helpers for BuildTelegramMessageContext
// ---------------------------------------------------------------------------

// boolPtr is defined in network_test.go (same package)

func makeDMMsg(chatID int64, userID int64, username, text string) *TelegramMessage {
	return &TelegramMessage{
		MessageID: 100,
		Date:      1700000000,
		Chat:      TelegramChat{ID: chatID, Type: "private"},
		Text:      text,
		From: &TelegramUser{
			ID:        userID,
			FirstName: "Alice",
			LastName:  "Smith",
			Username:  username,
		},
	}
}

func makeGroupMsg(chatID int64, userID int64, username, text string) *TelegramMessage {
	return &TelegramMessage{
		MessageID: 200,
		Date:      1700000000,
		Chat: TelegramChat{
			ID:    chatID,
			Type:  "supergroup",
			Title: "TestGroup",
		},
		Text: text,
		From: &TelegramUser{
			ID:        userID,
			FirstName: "Bob",
			Username:  username,
		},
	}
}

func makeGroupMsgWithMention(chatID int64, userID int64, username, botUsername, text string) *TelegramMessage {
	mentionText := "@" + botUsername
	fullText := text + " " + mentionText
	mentionOffset := len([]rune(text)) + 1
	return &TelegramMessage{
		MessageID: 200,
		Date:      1700000000,
		Chat: TelegramChat{
			ID:    chatID,
			Type:  "supergroup",
			Title: "TestGroup",
		},
		Text: fullText,
		From: &TelegramUser{
			ID:        userID,
			FirstName: "Bob",
			Username:  username,
		},
		Entities: []TelegramEntity{
			{Type: "mention", Offset: mentionOffset, Length: len([]rune(mentionText))},
		},
	}
}

func defaultBuildParams(msg *TelegramMessage) BuildTelegramMessageContextParams {
	return BuildTelegramMessageContextParams{
		Msg:            msg,
		BotID:          777,
		BotUsername:    "testbot",
		Config:         &types.OpenAcosmiConfig{},
		AccountID:      "acc-1",
		AllowFrom:      NormalizeAllowFrom([]string{"*"}),
		GroupAllowFrom: NormalizeAllowFrom([]string{"*"}),
		DMPolicy:       "open",
		RequireMention: false,
	}
}

// ---------------------------------------------------------------------------
// TestBuildTelegramMessageContext
// ---------------------------------------------------------------------------

func TestBuildTelegramMessageContext(t *testing.T) {
	t.Run("nil message returns nil", func(t *testing.T) {
		params := defaultBuildParams(nil)
		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context for nil message")
		}
	})

	t.Run("DM with open policy creates context", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "hello bot")
		params := defaultBuildParams(msg)
		params.DMPolicy = "open"

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context for DM with open policy")
		}
		if ctx.ChatID != 12345 {
			t.Errorf("ChatID = %d, want 12345", ctx.ChatID)
		}
		if ctx.IsGroup {
			t.Error("expected IsGroup=false for DM")
		}
		if ctx.SenderUsername != "alice" {
			t.Errorf("SenderUsername = %q, want %q", ctx.SenderUsername, "alice")
		}
		if !ctx.WasMentioned {
			t.Error("DM messages should always be marked as mentioned")
		}
		if ctx.AccountID != "acc-1" {
			t.Errorf("AccountID = %q, want %q", ctx.AccountID, "acc-1")
		}
	})

	t.Run("DM with disabled policy returns nil", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "hello")
		params := defaultBuildParams(msg)
		params.DMPolicy = "disabled"

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context for DM with disabled policy")
		}
	})

	t.Run("DM with allowlist policy and unlisted user returns nil", func(t *testing.T) {
		msg := makeDMMsg(99999, 99999, "stranger", "hello")
		params := defaultBuildParams(msg)
		params.DMPolicy = "allowlist"
		params.AllowFrom = NormalizeAllowFrom([]string{"12345"})

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context for DM with allowlist policy and unlisted user")
		}
	})

	t.Run("DM with allowlist policy and listed user creates context", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "hello")
		params := defaultBuildParams(msg)
		params.DMPolicy = "allowlist"
		params.AllowFrom = NormalizeAllowFrom([]string{"12345"})

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context for allowed user")
		}
		if ctx.SenderID != "12345" {
			t.Errorf("SenderID = %q, want %q", ctx.SenderID, "12345")
		}
	})

	t.Run("group message without mention when requireMention=true returns nil", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello everyone")
		params := defaultBuildParams(msg)
		params.RequireMention = true

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context for group message without mention when requireMention=true")
		}
	})

	t.Run("group message with bot mention creates context", func(t *testing.T) {
		msg := makeGroupMsgWithMention(-100999, 555, "bob", "testbot", "hello")
		params := defaultBuildParams(msg)
		params.RequireMention = true

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context for group message with bot mention")
		}
		if !ctx.IsGroup {
			t.Error("expected IsGroup=true for group message")
		}
		if !ctx.WasMentioned {
			t.Error("expected WasMentioned=true when bot is mentioned")
		}
		if ctx.ChatID != -100999 {
			t.Errorf("ChatID = %d, want -100999", ctx.ChatID)
		}
	})

	t.Run("group message with requireMention=false creates context without mention", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello everyone")
		params := defaultBuildParams(msg)
		params.RequireMention = false

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context when requireMention=false")
		}
	})

	t.Run("group message with disabled group config returns nil", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello")
		params := defaultBuildParams(msg)
		params.GroupConfig = &types.TelegramGroupConfig{
			Enabled: boolPtr(false),
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context when group config is disabled")
		}
	})

	t.Run("group message with disabled topic config returns nil", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello")
		params := defaultBuildParams(msg)
		params.TopicConfig = &types.TelegramTopicConfig{
			Enabled: boolPtr(false),
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context when topic config is disabled")
		}
	})

	t.Run("group message with enabled group config creates context", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello")
		params := defaultBuildParams(msg)
		params.GroupConfig = &types.TelegramGroupConfig{
			Enabled: boolPtr(true),
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context when group config is explicitly enabled")
		}
	})

	t.Run("empty text with no media returns nil", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "")
		params := defaultBuildParams(msg)

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context for empty message with no media")
		}
	})

	t.Run("empty text with media creates context", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "")
		msg.Photo = []interface{}{"photo_data"}
		params := defaultBuildParams(msg)
		params.AllMedia = []TelegramMediaRef{
			{Path: "/tmp/photo.jpg", ContentType: "image/jpeg"},
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context for message with media")
		}
		if ctx.MediaPath != "/tmp/photo.jpg" {
			t.Errorf("MediaPath = %q, want %q", ctx.MediaPath, "/tmp/photo.jpg")
		}
		if ctx.MediaType != "image/jpeg" {
			t.Errorf("MediaType = %q, want %q", ctx.MediaType, "image/jpeg")
		}
	})

	t.Run("reply to bot sets implicit mention", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "response here")
		msg.ReplyToMessage = &TelegramMessage{
			MessageID: 99,
			Chat:      msg.Chat,
			Text:      "original message",
			From:      &TelegramUser{ID: 777, FirstName: "TestBot", Username: "testbot", IsBot: true},
		}
		params := defaultBuildParams(msg)
		params.RequireMention = true

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context when replying to bot (implicit mention)")
		}
		if !ctx.IsReply {
			t.Error("expected IsReply=true when message has ReplyToMessage")
		}
		if ctx.ReplyToMessageID != 99 {
			t.Errorf("ReplyToMessageID = %d, want 99", ctx.ReplyToMessageID)
		}
	})

	t.Run("group allowFrom override blocks unlisted user", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello")
		params := defaultBuildParams(msg)
		params.GroupConfig = &types.TelegramGroupConfig{
			AllowFrom: []interface{}{"12345"},
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context for user not in group allowFrom override")
		}
	})

	t.Run("topic config overrides requireMention", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello everyone")
		params := defaultBuildParams(msg)
		params.RequireMention = true
		params.TopicConfig = &types.TelegramTopicConfig{
			RequireMention: boolPtr(false),
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context when topic config overrides requireMention to false")
		}
	})

	t.Run("group config overrides requireMention", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello everyone")
		params := defaultBuildParams(msg)
		params.RequireMention = false
		params.GroupConfig = &types.TelegramGroupConfig{
			RequireMention: boolPtr(true),
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx != nil {
			t.Error("expected nil context when group config overrides requireMention to true and no mention")
		}
	})

	t.Run("message timestamp is converted to milliseconds", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "hello")
		msg.Date = 1700000000
		params := defaultBuildParams(msg)

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if ctx.Timestamp != 1700000000000 {
			t.Errorf("Timestamp = %d, want %d", ctx.Timestamp, 1700000000000)
		}
	})

	t.Run("sender label includes name and username", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "hello")
		params := defaultBuildParams(msg)

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if !strings.Contains(ctx.SenderLabel, "Alice") {
			t.Errorf("SenderLabel %q should contain first name", ctx.SenderLabel)
		}
		if !strings.Contains(ctx.SenderLabel, "@alice") {
			t.Errorf("SenderLabel %q should contain @username", ctx.SenderLabel)
		}
	})

	t.Run("message ID override is respected", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "hello")
		params := defaultBuildParams(msg)
		params.Options = &TelegramMessageContextOptions{
			MessageIDOverride: "custom-id-99",
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if ctx.MessageSid != "custom-id-99" {
			t.Errorf("MessageSid = %q, want %q", ctx.MessageSid, "custom-id-99")
		}
	})

	t.Run("skill filter from topic config", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello")
		params := defaultBuildParams(msg)
		params.TopicConfig = &types.TelegramTopicConfig{
			Skills: []string{"skill-a", "skill-b"},
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if len(ctx.SkillFilter) != 2 || ctx.SkillFilter[0] != "skill-a" {
			t.Errorf("SkillFilter = %v, want [skill-a skill-b]", ctx.SkillFilter)
		}
	})

	t.Run("group system prompt combines group and topic prompts", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello")
		params := defaultBuildParams(msg)
		params.GroupConfig = &types.TelegramGroupConfig{
			SystemPrompt: "You are helpful.",
		}
		params.TopicConfig = &types.TelegramTopicConfig{
			SystemPrompt: "Be concise.",
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if !strings.Contains(ctx.GroupSystemPrompt, "You are helpful.") {
			t.Errorf("GroupSystemPrompt missing group prompt: %q", ctx.GroupSystemPrompt)
		}
		if !strings.Contains(ctx.GroupSystemPrompt, "Be concise.") {
			t.Errorf("GroupSystemPrompt missing topic prompt: %q", ctx.GroupSystemPrompt)
		}
	})

	t.Run("group subject from chat title", func(t *testing.T) {
		msg := makeGroupMsg(-100999, 555, "bob", "hello")
		msg.Chat.Title = "My Cool Group"
		params := defaultBuildParams(msg)

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if ctx.GroupSubject != "My Cool Group" {
			t.Errorf("GroupSubject = %q, want %q", ctx.GroupSubject, "My Cool Group")
		}
	})
}

// ---------------------------------------------------------------------------
// TestFormatTelegramInboundEnvelope
// ---------------------------------------------------------------------------

func TestFormatTelegramInboundEnvelope(t *testing.T) {
	tests := []struct {
		name   string
		params formatTelegramInboundEnvelopeParams
		checks []string // substrings that must appear
	}{
		{
			name: "DM envelope has channel and sender label",
			params: formatTelegramInboundEnvelopeParams{
				channel:     "Telegram",
				from:        "Alice",
				body:        "Hello!",
				chatType:    "direct",
				senderLabel: "Alice (@alice) id:12345",
			},
			checks: []string{"[Telegram]", "From: Alice (@alice) id:12345", "Hello!"},
		},
		{
			name: "group envelope includes from (group label)",
			params: formatTelegramInboundEnvelopeParams{
				channel:     "Telegram",
				from:        "TestGroup id:-100999",
				body:        "Hey group",
				chatType:    "group",
				senderLabel: "Bob (@bob) id:555",
			},
			checks: []string{"[Telegram]", "From: Bob (@bob) id:555", "in TestGroup id:-100999", "Hey group"},
		},
		{
			name: "timestamp is included when present",
			params: formatTelegramInboundEnvelopeParams{
				channel:     "Telegram",
				body:        "timestamped",
				chatType:    "direct",
				senderLabel: "Alice",
				timestamp:   1700000000000,
			},
			checks: []string{"[2023-11-14", "UTC]", "timestamped"},
		},
		{
			name: "empty body produces envelope without body line",
			params: formatTelegramInboundEnvelopeParams{
				channel:     "Telegram",
				chatType:    "direct",
				senderLabel: "Alice",
			},
			checks: []string{"[Telegram]", "From: Alice"},
		},
		{
			name: "no sender label in direct shows just channel",
			params: formatTelegramInboundEnvelopeParams{
				channel:  "Telegram",
				body:     "test",
				chatType: "direct",
			},
			checks: []string{"[Telegram]", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTelegramInboundEnvelope(tt.params)
			for _, check := range tt.checks {
				if !strings.Contains(result, check) {
					t.Errorf("result %q does not contain %q", result, check)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestResolveMediaPlaceholder
// ---------------------------------------------------------------------------

func TestResolveMediaPlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		msg      *TelegramMessage
		allMedia []TelegramMediaRef
		want     string
	}{
		{
			name:     "photo returns image placeholder",
			msg:      &TelegramMessage{Photo: []interface{}{"p1"}},
			allMedia: nil,
			want:     "<media:image>",
		},
		{
			name:     "video returns video placeholder",
			msg:      &TelegramMessage{Video: "v1"},
			allMedia: nil,
			want:     "<media:video>",
		},
		{
			name:     "audio returns audio placeholder",
			msg:      &TelegramMessage{Audio: "a1"},
			allMedia: nil,
			want:     "<media:audio>",
		},
		{
			name:     "voice returns audio placeholder",
			msg:      &TelegramMessage{Voice: "v1"},
			allMedia: nil,
			want:     "<media:audio>",
		},
		{
			name:     "document returns document placeholder",
			msg:      &TelegramMessage{Document: "d1"},
			allMedia: nil,
			want:     "<media:document>",
		},
		{
			name: "sticker without metadata returns sticker placeholder",
			msg: &TelegramMessage{
				Sticker: &TelegramSticker{FileID: "f1", Emoji: "😀"},
			},
			allMedia: nil,
			want:     "<media:sticker>",
		},
		{
			name: "sticker with cached description returns rich placeholder",
			msg: &TelegramMessage{
				Sticker: &TelegramSticker{FileID: "f1", Emoji: "😀", SetName: "FunPack"},
			},
			allMedia: []TelegramMediaRef{
				{
					StickerMetadata: &StickerMetadata{
						Emoji:             "😀",
						SetName:           "FunPack",
						CachedDescription: "a smiling face",
					},
				},
			},
			want: "[Sticker 😀 from \"FunPack\"] a smiling face",
		},
		{
			name:     "plain text message returns empty",
			msg:      &TelegramMessage{Text: "just text"},
			allMedia: nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveMediaPlaceholder(tt.msg, tt.allMedia)
			if got != tt.want {
				t.Errorf("resolveMediaPlaceholder() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDescribeReplyTarget
// ---------------------------------------------------------------------------

func TestDescribeReplyTarget(t *testing.T) {
	t.Run("no reply returns nil", func(t *testing.T) {
		msg := &TelegramMessage{
			MessageID: 1,
			Chat:      TelegramChat{ID: 12345, Type: "private"},
			Text:      "hello",
		}
		target := DescribeReplyTarget(msg)
		if target != nil {
			t.Error("expected nil target when no reply")
		}
	})

	t.Run("reply with text uses reply kind", func(t *testing.T) {
		msg := &TelegramMessage{
			MessageID: 2,
			Chat:      TelegramChat{ID: 12345, Type: "private"},
			Text:      "my response",
			ReplyToMessage: &TelegramMessage{
				MessageID: 1,
				Text:      "original message",
				From:      &TelegramUser{ID: 100, FirstName: "Alice"},
			},
		}
		target := DescribeReplyTarget(msg)
		if target == nil {
			t.Fatal("expected non-nil target")
		}
		if target.Kind != "reply" {
			t.Errorf("Kind = %q, want %q", target.Kind, "reply")
		}
		if target.Body != "original message" {
			t.Errorf("Body = %q, want %q", target.Body, "original message")
		}
		if target.Sender != "Alice" {
			t.Errorf("Sender = %q, want %q", target.Sender, "Alice")
		}
		if target.ID != "1" {
			t.Errorf("ID = %q, want %q", target.ID, "1")
		}
	})

	t.Run("quote overrides reply body and sets quote kind", func(t *testing.T) {
		msg := &TelegramMessage{
			MessageID: 3,
			Chat:      TelegramChat{ID: 12345, Type: "private"},
			Text:      "my response",
			Quote:     &TelegramQuote{Text: "quoted portion"},
			ReplyToMessage: &TelegramMessage{
				MessageID: 1,
				Text:      "full original message",
				From:      &TelegramUser{ID: 100, FirstName: "Alice"},
			},
		}
		target := DescribeReplyTarget(msg)
		if target == nil {
			t.Fatal("expected non-nil target")
		}
		if target.Kind != "quote" {
			t.Errorf("Kind = %q, want %q", target.Kind, "quote")
		}
		if target.Body != "quoted portion" {
			t.Errorf("Body = %q, want %q", target.Body, "quoted portion")
		}
	})

	t.Run("reply to photo message uses media placeholder", func(t *testing.T) {
		msg := &TelegramMessage{
			MessageID: 4,
			Chat:      TelegramChat{ID: 12345, Type: "private"},
			Text:      "nice photo",
			ReplyToMessage: &TelegramMessage{
				MessageID: 1,
				Photo:     []interface{}{"photo_data"},
				From:      &TelegramUser{ID: 100, FirstName: "Alice"},
			},
		}
		target := DescribeReplyTarget(msg)
		if target == nil {
			t.Fatal("expected non-nil target")
		}
		if target.Body != "<media:image>" {
			t.Errorf("Body = %q, want %q", target.Body, "<media:image>")
		}
	})

	t.Run("reply to video message uses video placeholder", func(t *testing.T) {
		msg := &TelegramMessage{
			MessageID: 5,
			Chat:      TelegramChat{ID: 12345, Type: "private"},
			Text:      "nice video",
			ReplyToMessage: &TelegramMessage{
				MessageID: 1,
				Video:     "video_data",
				From:      &TelegramUser{ID: 100, FirstName: "Alice"},
			},
		}
		target := DescribeReplyTarget(msg)
		if target == nil {
			t.Fatal("expected non-nil target")
		}
		if target.Body != "<media:video>" {
			t.Errorf("Body = %q, want %q", target.Body, "<media:video>")
		}
	})

	t.Run("quoteText returns body for quote kind", func(t *testing.T) {
		target := &TelegramReplyTarget{Kind: "quote", Body: "some quote"}
		if target.quoteText() != "some quote" {
			t.Errorf("quoteText() = %q, want %q", target.quoteText(), "some quote")
		}
	})

	t.Run("quoteText returns empty for reply kind", func(t *testing.T) {
		target := &TelegramReplyTarget{Kind: "reply", Body: "some text"}
		if target.quoteText() != "" {
			t.Errorf("quoteText() = %q, want empty", target.quoteText())
		}
	})

	t.Run("quoteText on nil returns empty", func(t *testing.T) {
		var target *TelegramReplyTarget
		if target.quoteText() != "" {
			t.Errorf("quoteText() on nil = %q, want empty", target.quoteText())
		}
	})
}

// ---------------------------------------------------------------------------
// TestBuildTelegramMessageContext_ReplyFormatting
// ---------------------------------------------------------------------------

func TestBuildTelegramMessageContext_ReplyFormatting(t *testing.T) {
	t.Run("reply suffix with reply kind", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "my reply")
		msg.ReplyToMessage = &TelegramMessage{
			MessageID: 50,
			Text:      "original text",
			From:      &TelegramUser{ID: 100, FirstName: "Bob"},
		}
		params := defaultBuildParams(msg)

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if !ctx.IsReply {
			t.Error("expected IsReply=true")
		}
		if !strings.Contains(ctx.Body, "[Replying to Bob") {
			t.Errorf("Body should contain reply suffix, got: %s", ctx.Body)
		}
		if ctx.ReplyToIsQuote {
			t.Error("expected ReplyToIsQuote=false for non-quote reply")
		}
	})

	t.Run("reply suffix with quote kind", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "my reply")
		msg.ReplyToMessage = &TelegramMessage{
			MessageID: 50,
			Text:      "full original text that is long",
			From:      &TelegramUser{ID: 100, FirstName: "Bob"},
		}
		msg.Quote = &TelegramQuote{Text: "partial quote"}
		params := defaultBuildParams(msg)

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if !strings.Contains(ctx.Body, "[Quoting Bob") {
			t.Errorf("Body should contain quote suffix, got: %s", ctx.Body)
		}
		if !ctx.ReplyToIsQuote {
			t.Error("expected ReplyToIsQuote=true for quote reply")
		}
		if ctx.ReplyQuoteText != "partial quote" {
			t.Errorf("ReplyQuoteText = %q, want %q", ctx.ReplyQuoteText, "partial quote")
		}
	})
}

// ---------------------------------------------------------------------------
// TestBuildTelegramMessageContext_ForwardContext
// ---------------------------------------------------------------------------

func TestBuildTelegramMessageContext_ForwardContext(t *testing.T) {
	t.Run("forwarded message includes forward prefix in body", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "check this out")
		msg.ForwardOrigin = &MessageOrigin{
			Type: "user",
			Date: 1700000000,
			SenderUser: &TelegramUser{
				ID:        999,
				FirstName: "Charlie",
				Username:  "charlie",
			},
		}
		params := defaultBuildParams(msg)

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if ctx.ForwardContext == "" {
			t.Error("expected non-empty ForwardContext for forwarded message")
		}
		if !strings.Contains(ctx.Body, "[Forwarded from Charlie") {
			t.Errorf("Body should contain forward prefix, got: %s", ctx.Body)
		}
		if ctx.ForwardedFrom == "" {
			t.Error("expected non-empty ForwardedFrom")
		}
		if ctx.ForwardedFromType != "user" {
			t.Errorf("ForwardedFromType = %q, want %q", ctx.ForwardedFromType, "user")
		}
	})
}

// ---------------------------------------------------------------------------
// TestBuildTelegramMessageContext_MediaFields
// ---------------------------------------------------------------------------

func TestBuildTelegramMessageContext_MediaFields(t *testing.T) {
	t.Run("multiple media fills multi-media fields", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "")
		msg.Photo = []interface{}{"p1"}
		params := defaultBuildParams(msg)
		params.AllMedia = []TelegramMediaRef{
			{Path: "/tmp/a.jpg", ContentType: "image/jpeg"},
			{Path: "/tmp/b.png", ContentType: "image/png"},
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if len(ctx.MediaPaths) != 2 {
			t.Errorf("MediaPaths length = %d, want 2", len(ctx.MediaPaths))
		}
		if len(ctx.MediaTypes) != 2 {
			t.Errorf("MediaTypes length = %d, want 2", len(ctx.MediaTypes))
		}
		if ctx.MediaPath != "/tmp/a.jpg" {
			t.Errorf("MediaPath = %q, want first media path", ctx.MediaPath)
		}
	})

	t.Run("sticker metadata populates sticker fields", func(t *testing.T) {
		msg := makeDMMsg(12345, 12345, "alice", "")
		msg.Sticker = &TelegramSticker{FileID: "f1", Emoji: "😀", SetName: "FunPack"}
		params := defaultBuildParams(msg)
		params.AllMedia = []TelegramMediaRef{
			{
				Path:        "/tmp/sticker.webp",
				ContentType: "image/webp",
				StickerMetadata: &StickerMetadata{
					Emoji:   "😀",
					SetName: "FunPack",
				},
			},
		}

		ctx := BuildTelegramMessageContext(params)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		if ctx.StickerEmoji != "😀" {
			t.Errorf("StickerEmoji = %q, want %q", ctx.StickerEmoji, "😀")
		}
		if ctx.StickerSetName != "FunPack" {
			t.Errorf("StickerSetName = %q, want %q", ctx.StickerSetName, "FunPack")
		}
	})
}
