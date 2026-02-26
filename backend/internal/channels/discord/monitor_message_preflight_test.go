package discord

import (
	"log/slog"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestSession creates a minimal discordgo.Session with an initialized State.
// Channels and guilds can be added via addChannel / addGuild helpers.
func newTestSession() *discordgo.Session {
	s, _ := discordgo.New("")
	return s
}

// addChannel adds a channel to the session state so that
// Session.State.Channel(id) succeeds.
func addChannel(s *discordgo.Session, ch *discordgo.Channel) {
	_ = s.State.ChannelAdd(ch)
}

// addGuild adds a guild to the session state so that
// Session.State.Guild(id) succeeds.
func addGuild(s *discordgo.Session, g *discordgo.Guild) {
	_ = s.State.GuildAdd(g)
}

// newMonCtx creates a DiscordMonitorContext suitable for testing.
func newMonCtx(opts ...func(*DiscordMonitorContext)) *DiscordMonitorContext {
	session := newTestSession()
	mc := &DiscordMonitorContext{
		BotUserID:      "BOT123",
		AccountID:      "acct-1",
		Token:          "tok",
		Session:        session,
		DMPolicy:       "open",
		GroupPolicy:    "open",
		RequireMention: false,
		GuildConfigs:   make(map[string]DiscordGuildEntryResolved),
		Logger:         slog.Default(),
	}
	for _, fn := range opts {
		fn(mc)
	}
	return mc
}

// msgCreate builds a *discordgo.MessageCreate with sensible defaults.
// Use option functions to customise fields.
func msgCreate(opts ...func(*discordgo.MessageCreate)) *discordgo.MessageCreate {
	m := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "msg-1",
			ChannelID: "ch-1",
			Content:   "hello world",
			Type:      discordgo.MessageTypeDefault,
			Author: &discordgo.User{
				ID:       "user-1",
				Username: "testuser",
				Bot:      false,
			},
		},
	}
	for _, fn := range opts {
		fn(m)
	}
	return m
}

// --- common option helpers for msgCreate ---

func withAuthor(id, username string, bot bool) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) {
		m.Author = &discordgo.User{ID: id, Username: username, Bot: bot}
	}
}

func withNilAuthor() func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.Author = nil }
}

func withContent(c string) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.Content = c }
}

func withGuildID(g string) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.GuildID = g }
}

func withMessageType(mt discordgo.MessageType) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.Type = mt }
}

func withAttachments(atts ...*discordgo.MessageAttachment) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.Attachments = atts }
}

func withMessageReference(ref *discordgo.MessageReference) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.MessageReference = ref }
}

func withReferencedMessage(rm *discordgo.Message) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.ReferencedMessage = rm }
}

func withMentions(users ...*discordgo.User) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) { m.Mentions = users }
}

func withMember(nick string) func(*discordgo.MessageCreate) {
	return func(m *discordgo.MessageCreate) {
		m.Member = &discordgo.Member{Nick: nick}
	}
}

func boolPtr(v bool) *bool { return &v }

// ===========================================================================
// Tests for PrepareDiscordInboundMessage
// ===========================================================================

func TestPreflightPrepare_NilAuthor(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withNilAuthor())
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "no-author" {
		t.Fatalf("expected (nil, no-author), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_SelfMessage(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withAuthor("BOT123", "bot", false))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "self-message" {
		t.Fatalf("expected (nil, self-message), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_BotMessage(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withAuthor("other-bot", "otherbot", true))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "bot-message" {
		t.Fatalf("expected (nil, bot-message), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_SystemMessage(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withMessageType(discordgo.MessageTypeGuildMemberJoin))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "system-message" {
		t.Fatalf("expected (nil, system-message), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_ReplyMessageTypeAllowed(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "general"})
	m := msgCreate(withMessageType(discordgo.MessageTypeReply), withContent("reply text"))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("reply message type should pass, got skip=%q", reason)
	}
	if msg.Text != "reply text" {
		t.Fatalf("expected text 'reply text', got %q", msg.Text)
	}
}

func TestPreflightPrepare_DMOpenPolicy(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "open"
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm-chan"})
	m := msgCreate() // no GuildID => DM
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("DM with open policy should pass, got skip=%q", reason)
	}
	if !msg.IsDM {
		t.Fatal("expected IsDM=true")
	}
}

func TestPreflightPrepare_DMAllowlistPolicy_Allowed(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "allowlist"
		mc.AllowFrom = []string{"user-1"}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm-chan"})
	m := msgCreate()
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("user-1 is on allowlist, should pass, got skip=%q", reason)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
}

func TestPreflightPrepare_DMAllowlistPolicy_Denied(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "allowlist"
		mc.AllowFrom = []string{"someone-else"}
	})
	m := msgCreate() // user-1 not in allowlist
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "dm-blocked:user-1" {
		t.Fatalf("expected (nil, dm-blocked:user-1), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_DMAllowlistPolicy_ByUsername(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "allowlist"
		mc.AllowFrom = []string{"testuser"}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm-chan"})
	m := msgCreate()
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("username match should pass, got skip=%q", reason)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
}

func TestPreflightPrepare_DMAllowlistPolicy_DynamicStore(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "allowlist"
		mc.AllowFrom = []string{} // static list empty
		mc.Deps = &DiscordMonitorDeps{
			ReadAllowFromStore: func(channel string) ([]string, error) {
				return []string{"user-1"}, nil
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate()
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("dynamic allowlist should pass, got skip=%q", reason)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
}

func TestPreflightPrepare_GuildAllowlistPolicy_GuildDenied(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "allowlist"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"guild-99": {ID: "guild-99"},
		}
	})
	m := msgCreate(withGuildID("guild-unknown"))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "guild-denied:guild-unknown" {
		t.Fatalf("expected (nil, guild-denied:guild-unknown), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_GuildAllowlistPolicy_GuildAllowed(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "allowlist"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"guild-1": {ID: "guild-1"},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "guild-1", Name: "general"})
	m := msgCreate(withGuildID("guild-1"))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("allowed guild should pass, got skip=%q", reason)
	}
	if msg == nil || msg.GuildID != "guild-1" {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}

// ---------------------------------------------------------------------------
// Mention detection and RequireMention gate
// ---------------------------------------------------------------------------

func TestPreflightPrepare_MentionDetection_TagStyle(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("<@BOT123> do something"),
	)
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("should pass, got skip=%q", reason)
	}
	if !msg.WasMentioned {
		t.Fatal("expected WasMentioned=true")
	}
	if msg.Text != "do something" {
		t.Fatalf("mention tag should be stripped; got %q", msg.Text)
	}
}

func TestPreflightPrepare_MentionDetection_NickStyle(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("<@!BOT123> nick mention"),
	)
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("should pass, got skip=%q", reason)
	}
	if !msg.WasMentioned {
		t.Fatal("expected WasMentioned=true for nick-style mention")
	}
	if msg.Text != "nick mention" {
		t.Fatalf("mention tag should be stripped; got %q", msg.Text)
	}
}

func TestPreflightPrepare_RequireMention_NoMention_NoReply(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = true
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("hello without mention"),
	)
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "mention-required" {
		t.Fatalf("expected (nil, mention-required), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_RequireMention_NoMention_WithReply(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = true
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("reply without mention"),
		withMessageReference(&discordgo.MessageReference{MessageID: "ref-1"}),
	)
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("reply should bypass mention requirement, got skip=%q", reason)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if msg.ReplyToID != "ref-1" {
		t.Fatalf("expected ReplyToID=ref-1, got %q", msg.ReplyToID)
	}
}

func TestPreflightPrepare_RequireMention_WithMention(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = true
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("<@BOT123> hello"),
	)
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("mentioned message should pass, got skip=%q", reason)
	}
	if !msg.WasMentioned {
		t.Fatal("expected WasMentioned=true")
	}
}

func TestPreflightPrepare_RequireMention_DM_IgnoresMentionRequirement(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = true
		mc.DMPolicy = "open"
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate(withContent("DM without mention"))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("DM should not require mention, got skip=%q", reason)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
}

// ---------------------------------------------------------------------------
// Attachments and empty message
// ---------------------------------------------------------------------------

func TestPreflightPrepare_EmptyMessage_NoAttachments(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withContent(""))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "empty-message" {
		t.Fatalf("expected (nil, empty-message), got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_EmptyContent_WithAttachment(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "gen"})
	att := &discordgo.MessageAttachment{ID: "att-1", URL: "https://cdn.discord.com/img.png", Filename: "img.png"}
	m := msgCreate(withContent(""), withAttachments(att))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("attachment-only message should pass, got skip=%q", reason)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].ID != "att-1" {
		t.Fatalf("expected 1 attachment with ID att-1, got %+v", msg.Attachments)
	}
}

func TestPreflightPrepare_WhitespaceOnly_NoAttachments(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withContent("   \t  \n  "))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "empty-message" {
		t.Fatalf("whitespace-only should be empty, got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_MentionOnly_BecomesEmpty(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withContent("<@BOT123>"))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if msg != nil || reason != "empty-message" {
		t.Fatalf("mention-only should become empty after stripping, got (%v, %q)", msg, reason)
	}
}

func TestPreflightPrepare_MentionOnly_WithAttachment(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "gen"})
	att := &discordgo.MessageAttachment{ID: "att-1", URL: "https://cdn.discord.com/f.txt", Filename: "f.txt"}
	m := msgCreate(withContent("<@BOT123>"), withAttachments(att))
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("mention-only with attachment should pass, got skip=%q", reason)
	}
	if msg.Text != "" {
		t.Fatalf("expected empty text after stripping mention, got %q", msg.Text)
	}
	if len(msg.Attachments) != 1 {
		t.Fatal("expected 1 attachment")
	}
}

// ---------------------------------------------------------------------------
// Thread detection
// ---------------------------------------------------------------------------

func TestPreflightPrepare_ThreadDetection(t *testing.T) {
	monCtx := newMonCtx()
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	addChannel(monCtx.Session, &discordgo.Channel{
		ID:       "thread-1",
		GuildID:  "g1",
		Name:     "my-thread",
		ParentID: "ch-parent",
		Type:     discordgo.ChannelType(channelTypeGuildPublicThread),
	})
	m := msgCreate(func(mc *discordgo.MessageCreate) {
		mc.ChannelID = "thread-1"
		mc.GuildID = "g1"
	})
	msg, reason := PrepareDiscordInboundMessage(monCtx, m)
	if reason != "" {
		t.Fatalf("thread message should pass, got skip=%q", reason)
	}
	if !msg.IsThread {
		t.Fatal("expected IsThread=true")
	}
	if msg.ThreadID != "thread-1" {
		t.Fatalf("expected ThreadID=thread-1, got %q", msg.ThreadID)
	}
}

// ---------------------------------------------------------------------------
// Output field correctness (table-driven)
// ---------------------------------------------------------------------------

func TestPreflightPrepare_OutputFields(t *testing.T) {
	tests := []struct {
		name       string
		monCtxOpts []func(*DiscordMonitorContext)
		msgOpts    []func(*discordgo.MessageCreate)
		setupChan  *discordgo.Channel
		wantIsDM   bool
		wantGuild  string
		wantSender string
	}{
		{
			name:       "DM message populates IsDM and SenderID",
			monCtxOpts: nil,
			msgOpts:    []func(*discordgo.MessageCreate){withContent("hi")},
			setupChan:  &discordgo.Channel{ID: "ch-1", Name: "dm"},
			wantIsDM:   true,
			wantGuild:  "",
			wantSender: "user-1",
		},
		{
			name:       "Guild message populates GuildID",
			monCtxOpts: nil,
			msgOpts: []func(*discordgo.MessageCreate){
				withGuildID("g1"),
				withContent("guild hi"),
			},
			setupChan:  &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "general"},
			wantIsDM:   false,
			wantGuild:  "g1",
			wantSender: "user-1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			monCtx := newMonCtx(tc.monCtxOpts...)
			if tc.setupChan != nil {
				addChannel(monCtx.Session, tc.setupChan)
			}
			m := msgCreate(tc.msgOpts...)
			msg, reason := PrepareDiscordInboundMessage(monCtx, m)
			if reason != "" {
				t.Fatalf("should pass, got skip=%q", reason)
			}
			if msg.IsDM != tc.wantIsDM {
				t.Errorf("IsDM: got %v, want %v", msg.IsDM, tc.wantIsDM)
			}
			if msg.GuildID != tc.wantGuild {
				t.Errorf("GuildID: got %q, want %q", msg.GuildID, tc.wantGuild)
			}
			if msg.SenderID != tc.wantSender {
				t.Errorf("SenderID: got %q, want %q", msg.SenderID, tc.wantSender)
			}
		})
	}
}

// ===========================================================================
// Tests for PreflightDiscordMessage (extended preflight)
// ===========================================================================

func TestPreflightFull_NilAuthor(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withNilAuthor())
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("nil author should return nil context")
	}
}

func TestPreflightFull_SelfMessage(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withAuthor("BOT123", "bot", false))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("self message should return nil")
	}
}

func TestPreflightFull_BotUserIDFromParams(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.BotUserID = "" // not set on context
	})
	params := &DiscordMessagePreflightParams{BotUserID: "PARAM-BOT", DMEnabled: true, GroupDmEnabled: true}
	m := msgCreate(withAuthor("PARAM-BOT", "thebot", false))
	ctx := PreflightDiscordMessage(monCtx, m, params)
	if ctx != nil {
		t.Fatal("message from params.BotUserID should be filtered as self")
	}
}

func TestPreflightFull_BotMessageFiltered(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withAuthor("other-bot", "otherbot", true))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("bot message should return nil")
	}
}

func TestPreflightFull_BotMessageAllowed(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "gen"})
	params := &DiscordMessagePreflightParams{AllowBots: true, DMEnabled: true, GroupDmEnabled: true}
	m := msgCreate(withAuthor("other-bot", "otherbot", true))
	ctx := PreflightDiscordMessage(monCtx, m, params)
	if ctx == nil {
		t.Fatal("allowBots=true should let bot message through")
	}
}

func TestPreflightFull_SystemMessage(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withMessageType(discordgo.MessageTypeGuildMemberJoin))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("system message should return nil")
	}
}

func TestPreflightFull_SystemEventEnqueued(t *testing.T) {
	var enqueuedText string
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.Deps = &DiscordMonitorDeps{
			EnqueueSystemEvent: func(text, sessionKey, contextKey string) error {
				enqueuedText = text
				return nil
			},
		}
	})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(
		withMessageType(discordgo.MessageTypeGuildMemberJoin),
		withGuildID("g1"),
		withAuthor("joiner", "joiner", false),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("system message should still return nil")
	}
	if enqueuedText == "" {
		t.Fatal("expected system event to be enqueued")
	}
}

// ---------------------------------------------------------------------------
// DM policy enforcement (PreflightDiscordMessage)
// ---------------------------------------------------------------------------

func TestPreflightFull_DMPolicy_Disabled(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "disabled"
	})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("DM disabled should return nil")
	}
}

func TestPreflightFull_DMPolicy_Open(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "open"
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("DM open should pass")
	}
	if !ctx.IsDirectMessage {
		t.Fatal("expected IsDirectMessage=true")
	}
}

func TestPreflightFull_DMPolicy_Allowlist_Allowed(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "allowlist"
		mc.AllowFrom = []string{"user-1"}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("user on allowlist should pass")
	}
}

func TestPreflightFull_DMPolicy_Allowlist_Denied(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "allowlist"
		mc.AllowFrom = []string{"someone-else"}
	})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("user not on allowlist should return nil")
	}
}

func TestPreflightFull_DMPolicy_Pairing_CreatesPairingRequest(t *testing.T) {
	var pairingCreated bool
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "pairing"
		mc.AllowFrom = []string{}
		mc.Deps = &DiscordMonitorDeps{
			UpsertPairingRequest: func(params DiscordPairingRequestParams) (*DiscordPairingResult, error) {
				pairingCreated = true
				if params.Channel != "discord" {
					t.Errorf("expected channel=discord, got %q", params.Channel)
				}
				if params.ID != "user-1" {
					t.Errorf("expected ID=user-1, got %q", params.ID)
				}
				return &DiscordPairingResult{Created: true, Code: "XYZ"}, nil
			},
		}
	})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("pairing-blocked user should return nil")
	}
	if !pairingCreated {
		t.Fatal("pairing request should have been created")
	}
}

func TestPreflightFull_DMPolicy_Pairing_AllowedUser(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "pairing"
		mc.AllowFrom = []string{"user-1"}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("user on allowlist in pairing mode should pass")
	}
}

func TestPreflightFull_DMPolicy_OverrideFromParams(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.DMPolicy = "open"
	})
	params := &DiscordMessagePreflightParams{
		DMPolicy:       "disabled",
		DMEnabled:      true,
		GroupDmEnabled: true,
	}
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, params)
	if ctx != nil {
		t.Fatal("params.DMPolicy=disabled should override monCtx.DMPolicy=open")
	}
}

// ---------------------------------------------------------------------------
// DMEnabled / GroupDmEnabled gate
// ---------------------------------------------------------------------------

func TestPreflightFull_DMEnabled_False(t *testing.T) {
	monCtx := newMonCtx()
	params := &DiscordMessagePreflightParams{
		DMEnabled:      false,
		GroupDmEnabled: true,
	}
	m := msgCreate() // DM
	ctx := PreflightDiscordMessage(monCtx, m, params)
	if ctx != nil {
		t.Fatal("DMEnabled=false should block DM")
	}
}

// ---------------------------------------------------------------------------
// Group policy enforcement (PreflightDiscordMessage)
// ---------------------------------------------------------------------------

func TestPreflightFull_GroupPolicy_Disabled(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "disabled"
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(withGuildID("g1"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("group disabled should return nil for guild messages")
	}
}

func TestPreflightFull_GroupPolicy_Open(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "open"
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(withGuildID("g1"), withContent("hello"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("group open should allow guild message")
	}
	if !ctx.IsGuildMessage {
		t.Fatal("expected IsGuildMessage=true")
	}
}

func TestPreflightFull_GroupPolicy_Allowlist_GuildNotConfigured(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "allowlist"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"other-guild": {ID: "other-guild"},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(withGuildID("g1"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("guild not in allowlist should return nil")
	}
}

func TestPreflightFull_GroupPolicy_Allowlist_GuildConfigured(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "allowlist"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {ID: "g1"},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(withGuildID("g1"), withContent("allowed"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("guild in allowlist should pass")
	}
}

func TestPreflightFull_GroupPolicy_OverrideFromParams(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "open"
	})
	params := &DiscordMessagePreflightParams{
		GroupPolicy:    "disabled",
		DMEnabled:      true,
		GroupDmEnabled: true,
	}
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(withGuildID("g1"))
	ctx := PreflightDiscordMessage(monCtx, m, params)
	if ctx != nil {
		t.Fatal("params.GroupPolicy=disabled should override monCtx.GroupPolicy=open")
	}
}

// ---------------------------------------------------------------------------
// Mention detection (PreflightDiscordMessage)
// ---------------------------------------------------------------------------

func TestPreflightFull_MentionDetection_ContentTag(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = false
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(withGuildID("g1"), withContent("<@BOT123> hi"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if !ctx.WasMentioned {
		t.Fatal("expected WasMentioned=true")
	}
	if ctx.BaseText != "hi" {
		t.Fatalf("expected stripped text 'hi', got %q", ctx.BaseText)
	}
}

func TestPreflightFull_MentionDetection_ExplicitMentionsList(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = false
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("hello"), // no mention in content
		withMentions(&discordgo.User{ID: "BOT123"}),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if !ctx.WasMentioned {
		t.Fatal("expected WasMentioned=true via explicit mention list")
	}
}

func TestPreflightFull_MentionDetection_NoBotMention(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = false
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("hello everyone"),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass (requireMention=false)")
	}
	if ctx.WasMentioned {
		t.Fatal("expected WasMentioned=false")
	}
}

func TestPreflightFull_HasAnyMention_GuildOnly(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("hello"),
		withMentions(&discordgo.User{ID: "someone-else"}),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if !ctx.HasAnyMention {
		t.Fatal("expected HasAnyMention=true when Mentions is non-empty in guild")
	}
}

func TestPreflightFull_HasAnyMention_DMIgnored(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate(
		withContent("hello"),
		withMentions(&discordgo.User{ID: "someone-else"}),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if ctx.HasAnyMention {
		t.Fatal("expected HasAnyMention=false in DM")
	}
}

// ---------------------------------------------------------------------------
// RequireMention with implicit mention (reply-to-bot)
// ---------------------------------------------------------------------------

func TestPreflightFull_RequireMention_ImplicitMention_ReplyToBot(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = true
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {ID: "g1", RequireMention: boolPtr(true)},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("replying to bot"),
		withMessageReference(&discordgo.MessageReference{MessageID: "bot-msg-1"}),
		withReferencedMessage(&discordgo.Message{
			ID:     "bot-msg-1",
			Author: &discordgo.User{ID: "BOT123"},
		}),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("reply to bot should be implicit mention and pass")
	}
	if !ctx.EffectiveWasMentioned {
		t.Fatal("expected EffectiveWasMentioned=true")
	}
}

func TestPreflightFull_RequireMention_NoMention_NoReply_Blocked(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = true
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {ID: "g1", RequireMention: boolPtr(true)},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("hello"),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("no mention and no reply should be blocked")
	}
}

func TestPreflightFull_RequireMention_ReplyToOtherUser_Blocked(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.RequireMention = true
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {ID: "g1", RequireMention: boolPtr(true)},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("replying"),
		withMessageReference(&discordgo.MessageReference{MessageID: "other-msg"}),
		withReferencedMessage(&discordgo.Message{
			ID:     "other-msg",
			Author: &discordgo.User{ID: "other-user"},
		}),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("reply to non-bot user should not count as implicit mention")
	}
}

// ---------------------------------------------------------------------------
// User allowlist in guild (PreflightDiscordMessage)
// ---------------------------------------------------------------------------

func TestPreflightFull_GuildUserAllowlist_Blocked(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "open"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {
				ID:    "g1",
				Users: []string{"allowed-user-only"},
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(withGuildID("g1"), withContent("hello"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("user not on guild user allowlist should be blocked")
	}
}

func TestPreflightFull_GuildUserAllowlist_Allowed(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "open"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {
				ID:    "g1",
				Users: []string{"user-1"},
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(withGuildID("g1"), withContent("hello"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("user on guild user allowlist should pass")
	}
}

// ---------------------------------------------------------------------------
// Empty message (PreflightDiscordMessage)
// ---------------------------------------------------------------------------

func TestPreflightFull_EmptyMessage(t *testing.T) {
	monCtx := newMonCtx()
	m := msgCreate(withContent(""))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("empty message should return nil")
	}
}

func TestPreflightFull_EmptyContent_WithAttachment(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	att := &discordgo.MessageAttachment{ID: "att-1", URL: "https://example.com/f.png", Filename: "f.png"}
	m := msgCreate(withContent(""), withAttachments(att))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("empty content with attachment should pass")
	}
}

// ---------------------------------------------------------------------------
// Agent routing (PreflightDiscordMessage)
// ---------------------------------------------------------------------------

func TestPreflightFull_AgentRouting_DM(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.Deps = &DiscordMonitorDeps{
			ResolveAgentRoute: func(params DiscordAgentRouteParams) (*DiscordAgentRoute, error) {
				if params.PeerKind != "direct" {
					t.Errorf("expected peerKind=direct for DM, got %q", params.PeerKind)
				}
				if params.PeerID != "user-1" {
					t.Errorf("expected peerID=user-1 for DM, got %q", params.PeerID)
				}
				return &DiscordAgentRoute{SessionKey: "sess-dm"}, nil
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if ctx.Route == nil || ctx.Route.SessionKey != "sess-dm" {
		t.Fatalf("expected route with session key sess-dm, got %+v", ctx.Route)
	}
	if ctx.BaseSessionKey != "sess-dm" {
		t.Fatalf("expected BaseSessionKey=sess-dm, got %q", ctx.BaseSessionKey)
	}
}

func TestPreflightFull_AgentRouting_Guild(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.Deps = &DiscordMonitorDeps{
			ResolveAgentRoute: func(params DiscordAgentRouteParams) (*DiscordAgentRoute, error) {
				if params.PeerKind != "channel" {
					t.Errorf("expected peerKind=channel for guild, got %q", params.PeerKind)
				}
				if params.PeerID != "ch-1" {
					t.Errorf("expected peerID=ch-1, got %q", params.PeerID)
				}
				return &DiscordAgentRoute{SessionKey: "sess-guild"}, nil
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(withGuildID("g1"), withContent("hello"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if ctx.Route == nil || ctx.Route.SessionKey != "sess-guild" {
		t.Fatalf("expected route with session key sess-guild, got %+v", ctx.Route)
	}
}

// ---------------------------------------------------------------------------
// Params overrides for history/media/text
// ---------------------------------------------------------------------------

func TestPreflightFull_ParamsOverrides(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	params := &DiscordMessagePreflightParams{
		DMEnabled:        true,
		GroupDmEnabled:   true,
		HistoryLimit:     50,
		MediaMaxBytes:    1048576,
		TextLimit:        4000,
		ReplyToMode:      "always",
		AckReactionScope: "direct",
	}
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, params)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if ctx.HistoryLimit != 50 {
		t.Errorf("HistoryLimit: got %d, want 50", ctx.HistoryLimit)
	}
	if ctx.MediaMaxBytes != 1048576 {
		t.Errorf("MediaMaxBytes: got %d, want 1048576", ctx.MediaMaxBytes)
	}
	if ctx.TextLimit != 4000 {
		t.Errorf("TextLimit: got %d, want 4000", ctx.TextLimit)
	}
	if ctx.ReplyToMode != "always" {
		t.Errorf("ReplyToMode: got %q, want always", ctx.ReplyToMode)
	}
	if ctx.AckReactionScope != "direct" {
		t.Errorf("AckReactionScope: got %q, want direct", ctx.AckReactionScope)
	}
}

// ---------------------------------------------------------------------------
// CanDetectMention
// ---------------------------------------------------------------------------

func TestPreflightFull_CanDetectMention(t *testing.T) {
	tests := []struct {
		name      string
		botUserID string
		want      bool
	}{
		{"with bot ID", "BOT123", true},
		{"without bot ID", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
				mc.BotUserID = tc.botUserID
			})
			addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
			m := msgCreate()
			ctx := PreflightDiscordMessage(monCtx, m, nil)
			if ctx == nil {
				t.Fatal("should pass")
			}
			if ctx.CanDetectMention != tc.want {
				t.Errorf("CanDetectMention: got %v, want %v", ctx.CanDetectMention, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Guild slug and guild info resolution
// ---------------------------------------------------------------------------

func TestPreflightFull_GuildSlug(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {ID: "g1"},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "My Test Guild"})
	m := msgCreate(withGuildID("g1"), withContent("hello"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if ctx.GuildSlug != "my-test-guild" {
		t.Fatalf("expected guild slug 'my-test-guild', got %q", ctx.GuildSlug)
	}
	if ctx.GuildInfo == nil {
		t.Fatal("expected GuildInfo to be set")
	}
}

// ---------------------------------------------------------------------------
// Channel config resolution
// ---------------------------------------------------------------------------

func TestPreflightFull_ChannelConfigResolved(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "open"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {
				ID: "g1",
				Channels: map[string]DiscordChannelEntryResolved{
					"ch-1": {
						Allow:   boolPtr(true),
						Skills:  []string{"code-review"},
						Enabled: boolPtr(true),
					},
				},
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "dev"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(withGuildID("g1"), withContent("hello"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if ctx.ChannelConfig == nil {
		t.Fatal("expected ChannelConfig to be resolved")
	}
	if !ctx.ChannelConfig.Allowed {
		t.Fatal("expected ChannelConfig.Allowed=true")
	}
	if len(ctx.ChannelConfig.Skills) != 1 || ctx.ChannelConfig.Skills[0] != "code-review" {
		t.Fatalf("expected skills=[code-review], got %v", ctx.ChannelConfig.Skills)
	}
}

func TestPreflightFull_ChannelDisabled(t *testing.T) {
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.GroupPolicy = "open"
		mc.GuildConfigs = map[string]DiscordGuildEntryResolved{
			"g1": {
				ID: "g1",
				Channels: map[string]DiscordChannelEntryResolved{
					"ch-1": {
						Allow:   boolPtr(true),
						Enabled: boolPtr(false),
					},
				},
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "disabled-chan"})
	addGuild(monCtx.Session, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
	m := msgCreate(withGuildID("g1"), withContent("hello"))
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx != nil {
		t.Fatal("channel with Enabled=false should return nil")
	}
}

// ---------------------------------------------------------------------------
// Sender identity
// ---------------------------------------------------------------------------

func TestPreflightFull_SenderIdentity(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate(func(mc *discordgo.MessageCreate) {
		mc.Author = &discordgo.User{
			ID:            "user-1",
			Username:      "alice",
			GlobalName:    "Alice W",
			Discriminator: "0",
		}
	})
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if ctx.Sender.ID != "user-1" {
		t.Errorf("Sender.ID: got %q, want user-1", ctx.Sender.ID)
	}
	if ctx.Sender.Name != "alice" {
		t.Errorf("Sender.Name: got %q, want alice", ctx.Sender.Name)
	}
}

func TestPreflightFull_SenderIdentity_WithNickname(t *testing.T) {
	monCtx := newMonCtx()
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", GuildID: "g1", Name: "gen"})
	m := msgCreate(
		withGuildID("g1"),
		withContent("hello"),
		withMember("NickAlice"),
	)
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	// Nickname should influence Label via ResolveDiscordSenderIdentity
	if ctx.Sender.Label == "" {
		t.Fatal("expected non-empty sender label")
	}
}

// ---------------------------------------------------------------------------
// RecordChannelActivity callback
// ---------------------------------------------------------------------------

func TestPreflightFull_RecordChannelActivity(t *testing.T) {
	var recorded bool
	monCtx := newMonCtx(func(mc *DiscordMonitorContext) {
		mc.Deps = &DiscordMonitorDeps{
			RecordChannelActivity: func(channel, accountID, direction string) {
				recorded = true
				if channel != "discord" {
					t.Errorf("expected channel=discord, got %q", channel)
				}
				if direction != "inbound" {
					t.Errorf("expected direction=inbound, got %q", direction)
				}
			},
		}
	})
	addChannel(monCtx.Session, &discordgo.Channel{ID: "ch-1", Name: "dm"})
	m := msgCreate()
	ctx := PreflightDiscordMessage(monCtx, m, nil)
	if ctx == nil {
		t.Fatal("should pass")
	}
	if !recorded {
		t.Fatal("expected RecordChannelActivity to be called")
	}
}

// ===========================================================================
// Table-driven tests for checkDiscordDMSenderAllowed
// ===========================================================================

func TestPreflightCheckDMSenderAllowed(t *testing.T) {
	tests := []struct {
		name      string
		dmPolicy  string
		allowFrom []string
		deps      *DiscordMonitorDeps
		userID    string
		userName  string
		want      bool
	}{
		{
			name:     "open policy always allows",
			dmPolicy: "open",
			userID:   "anyone",
			userName: "anyone",
			want:     true,
		},
		{
			name:      "allowlist by ID",
			dmPolicy:  "allowlist",
			allowFrom: []string{"user-42"},
			userID:    "user-42",
			userName:  "bob",
			want:      true,
		},
		{
			name:      "allowlist by username case-insensitive",
			dmPolicy:  "allowlist",
			allowFrom: []string{"Bob"},
			userID:    "user-42",
			userName:  "bob",
			want:      true,
		},
		{
			name:      "allowlist miss",
			dmPolicy:  "allowlist",
			allowFrom: []string{"charlie"},
			userID:    "user-42",
			userName:  "bob",
			want:      false,
		},
		{
			name:      "dynamic store match",
			dmPolicy:  "allowlist",
			allowFrom: []string{},
			deps: &DiscordMonitorDeps{
				ReadAllowFromStore: func(channel string) ([]string, error) {
					return []string{"user-42"}, nil
				},
			},
			userID:   "user-42",
			userName: "bob",
			want:     true,
		},
		{
			name:      "dynamic store miss",
			dmPolicy:  "allowlist",
			allowFrom: []string{},
			deps: &DiscordMonitorDeps{
				ReadAllowFromStore: func(channel string) ([]string, error) {
					return []string{"other"}, nil
				},
			},
			userID:   "user-42",
			userName: "bob",
			want:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mc := &DiscordMonitorContext{
				DMPolicy:  tc.dmPolicy,
				AllowFrom: tc.allowFrom,
				Deps:      tc.deps,
			}
			got := checkDiscordDMSenderAllowed(mc, tc.userID, tc.userName)
			if got != tc.want {
				t.Errorf("checkDiscordDMSenderAllowed() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// Table-driven tests for isGuildAllowed
// ===========================================================================

func TestPreflightIsGuildAllowed(t *testing.T) {
	tests := []struct {
		name    string
		configs map[string]DiscordGuildEntryResolved
		guildID string
		want    bool
	}{
		{
			name:    "empty configs rejects all",
			configs: nil,
			guildID: "g1",
			want:    false,
		},
		{
			name: "guild present is allowed",
			configs: map[string]DiscordGuildEntryResolved{
				"g1": {ID: "g1"},
			},
			guildID: "g1",
			want:    true,
		},
		{
			name: "guild absent is denied",
			configs: map[string]DiscordGuildEntryResolved{
				"g2": {ID: "g2"},
			},
			guildID: "g1",
			want:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mc := &DiscordMonitorContext{
				GuildConfigs: tc.configs,
			}
			got := isGuildAllowed(mc, tc.guildID)
			if got != tc.want {
				t.Errorf("isGuildAllowed() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// Concurrent access to isGuildAllowed (tests RWMutex usage)
// ===========================================================================

func TestPreflightIsGuildAllowed_Concurrent(t *testing.T) {
	mc := &DiscordMonitorContext{
		GuildConfigs: map[string]DiscordGuildEntryResolved{
			"g1": {ID: "g1"},
		},
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = isGuildAllowed(mc, "g1")
		}()
	}
	wg.Wait()
}

// ===========================================================================
// resolveGuildName
// ===========================================================================

func TestPreflightResolveGuildName(t *testing.T) {
	tests := []struct {
		name    string
		session *discordgo.Session
		guildID string
		want    string
	}{
		{
			name:    "nil session",
			session: nil,
			guildID: "g1",
			want:    "",
		},
		{
			name:    "empty guildID",
			session: newTestSession(),
			guildID: "",
			want:    "",
		},
		{
			name: "guild found",
			session: func() *discordgo.Session {
				s := newTestSession()
				addGuild(s, &discordgo.Guild{ID: "g1", Name: "TestGuild"})
				return s
			}(),
			guildID: "g1",
			want:    "TestGuild",
		},
		{
			name:    "guild not found",
			session: newTestSession(),
			guildID: "unknown",
			want:    "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveGuildName(tc.session, tc.guildID)
			if got != tc.want {
				t.Errorf("resolveGuildName() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// ResolveDiscordSystemEvent
// ===========================================================================

func TestPreflightResolveDiscordSystemEvent(t *testing.T) {
	tests := []struct {
		name     string
		msg      *discordgo.MessageCreate
		location string
		want     string
	}{
		{
			name: "nil message",
			msg:  nil,
			want: "",
		},
		{
			name: "member join",
			msg: &discordgo.MessageCreate{Message: &discordgo.Message{
				Type:   discordgo.MessageTypeGuildMemberJoin,
				Author: &discordgo.User{Username: "alice"},
			}},
			location: "TestGuild #general",
			want:     "Member joined: alice in TestGuild #general",
		},
		{
			name: "boost",
			msg: &discordgo.MessageCreate{Message: &discordgo.Message{
				Type:   discordgo.MessageTypeUserPremiumGuildSubscription,
				Author: &discordgo.User{Username: "bob"},
			}},
			location: "TestGuild",
			want:     "Boost: bob boosted TestGuild",
		},
		{
			name: "pin",
			msg: &discordgo.MessageCreate{Message: &discordgo.Message{
				Type:   discordgo.MessageTypeChannelPinnedMessage,
				Author: &discordgo.User{Username: "charlie"},
			}},
			location: "#general",
			want:     "Pin: message pinned in #general",
		},
		{
			name: "unknown type returns empty",
			msg: &discordgo.MessageCreate{Message: &discordgo.Message{
				Type:   discordgo.MessageTypeDefault,
				Author: &discordgo.User{Username: "dave"},
			}},
			location: "loc",
			want:     "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveDiscordSystemEvent(tc.msg, tc.location)
			if got != tc.want {
				t.Errorf("ResolveDiscordSystemEvent() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// Full end-to-end table-driven test for PrepareDiscordInboundMessage
// ===========================================================================

func TestPreflightPrepare_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		monCtxOpts []func(*DiscordMonitorContext)
		msgOpts    []func(*discordgo.MessageCreate)
		setupChans []*discordgo.Channel
		wantSkip   string
		wantNil    bool
		validate   func(t *testing.T, msg *DiscordInboundMessage)
	}{
		{
			name:     "nil author",
			msgOpts:  []func(*discordgo.MessageCreate){withNilAuthor()},
			wantSkip: "no-author",
			wantNil:  true,
		},
		{
			name:     "self message",
			msgOpts:  []func(*discordgo.MessageCreate){withAuthor("BOT123", "bot", false)},
			wantSkip: "self-message",
			wantNil:  true,
		},
		{
			name:     "bot message",
			msgOpts:  []func(*discordgo.MessageCreate){withAuthor("x", "xbot", true)},
			wantSkip: "bot-message",
			wantNil:  true,
		},
		{
			name:     "system message type",
			msgOpts:  []func(*discordgo.MessageCreate){withMessageType(discordgo.MessageTypeGuildMemberJoin)},
			wantSkip: "system-message",
			wantNil:  true,
		},
		{
			name:     "empty message no attachments",
			msgOpts:  []func(*discordgo.MessageCreate){withContent("")},
			wantSkip: "empty-message",
			wantNil:  true,
		},
		{
			name: "DM open policy passes",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.DMPolicy = "open"
			}},
			msgOpts:    []func(*discordgo.MessageCreate){withContent("hi")},
			setupChans: []*discordgo.Channel{{ID: "ch-1", Name: "dm"}},
			wantSkip:   "",
			validate: func(t *testing.T, msg *DiscordInboundMessage) {
				if !msg.IsDM {
					t.Error("expected IsDM=true")
				}
			},
		},
		{
			name: "guild mention required but missing",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.RequireMention = true
			}},
			msgOpts: []func(*discordgo.MessageCreate){
				withGuildID("g1"),
				withContent("no mention here"),
			},
			setupChans: []*discordgo.Channel{{ID: "ch-1", GuildID: "g1", Name: "gen"}},
			wantSkip:   "mention-required",
			wantNil:    true,
		},
		{
			name: "guild mention present passes",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.RequireMention = true
			}},
			msgOpts: []func(*discordgo.MessageCreate){
				withGuildID("g1"),
				withContent("<@BOT123> hello"),
			},
			setupChans: []*discordgo.Channel{{ID: "ch-1", GuildID: "g1", Name: "gen"}},
			wantSkip:   "",
			validate: func(t *testing.T, msg *DiscordInboundMessage) {
				if msg.Text != "hello" {
					t.Errorf("expected stripped text 'hello', got %q", msg.Text)
				}
				if !msg.WasMentioned {
					t.Error("expected WasMentioned=true")
				}
			},
		},
		{
			name: "multiple attachments",
			msgOpts: []func(*discordgo.MessageCreate){
				withContent("look at these"),
				withAttachments(
					&discordgo.MessageAttachment{ID: "a1", Filename: "a.png"},
					&discordgo.MessageAttachment{ID: "a2", Filename: "b.jpg"},
				),
			},
			setupChans: []*discordgo.Channel{{ID: "ch-1", Name: "dm"}},
			wantSkip:   "",
			validate: func(t *testing.T, msg *DiscordInboundMessage) {
				if len(msg.Attachments) != 2 {
					t.Errorf("expected 2 attachments, got %d", len(msg.Attachments))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			monCtx := newMonCtx(tc.monCtxOpts...)
			for _, ch := range tc.setupChans {
				addChannel(monCtx.Session, ch)
			}
			m := msgCreate(tc.msgOpts...)
			msg, reason := PrepareDiscordInboundMessage(monCtx, m)

			if tc.wantNil && msg != nil {
				t.Fatalf("expected nil message, got %+v", msg)
			}
			if !tc.wantNil && msg == nil {
				t.Fatalf("expected non-nil message, got nil (skip=%q)", reason)
			}
			if reason != tc.wantSkip {
				t.Fatalf("skip reason: got %q, want %q", reason, tc.wantSkip)
			}
			if tc.validate != nil && msg != nil {
				tc.validate(t, msg)
			}
		})
	}
}

// ===========================================================================
// Full end-to-end table-driven test for PreflightDiscordMessage
// ===========================================================================

func TestPreflightFull_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		monCtxOpts  []func(*DiscordMonitorContext)
		params      *DiscordMessagePreflightParams
		msgOpts     []func(*discordgo.MessageCreate)
		setupChans  []*discordgo.Channel
		setupGuilds []*discordgo.Guild
		wantNil     bool
		validate    func(t *testing.T, ctx *DiscordMessagePreflightContext)
	}{
		{
			name:    "nil author returns nil",
			msgOpts: []func(*discordgo.MessageCreate){withNilAuthor()},
			wantNil: true,
		},
		{
			name:    "self message returns nil",
			msgOpts: []func(*discordgo.MessageCreate){withAuthor("BOT123", "bot", false)},
			wantNil: true,
		},
		{
			name:    "bot message returns nil",
			msgOpts: []func(*discordgo.MessageCreate){withAuthor("x", "xbot", true)},
			wantNil: true,
		},
		{
			name: "DM disabled returns nil",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.DMPolicy = "disabled"
			}},
			msgOpts: []func(*discordgo.MessageCreate){withContent("hi")},
			wantNil: true,
		},
		{
			name: "DM open passes with correct fields",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.DMPolicy = "open"
			}},
			msgOpts:    []func(*discordgo.MessageCreate){withContent("hello dm")},
			setupChans: []*discordgo.Channel{{ID: "ch-1", Name: "dm-chan"}},
			validate: func(t *testing.T, ctx *DiscordMessagePreflightContext) {
				if !ctx.IsDirectMessage {
					t.Error("expected IsDirectMessage=true")
				}
				if ctx.IsGuildMessage {
					t.Error("expected IsGuildMessage=false")
				}
				if ctx.BaseText != "hello dm" {
					t.Errorf("BaseText: got %q, want 'hello dm'", ctx.BaseText)
				}
				if ctx.DMPolicy != "open" {
					t.Errorf("DMPolicy: got %q, want 'open'", ctx.DMPolicy)
				}
			},
		},
		{
			name: "guild message with group policy open",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.GroupPolicy = "open"
			}},
			msgOpts: []func(*discordgo.MessageCreate){
				withGuildID("g1"),
				withContent("guild msg"),
			},
			setupChans:  []*discordgo.Channel{{ID: "ch-1", GuildID: "g1", Name: "general"}},
			setupGuilds: []*discordgo.Guild{{ID: "g1", Name: "My Guild"}},
			validate: func(t *testing.T, ctx *DiscordMessagePreflightContext) {
				if !ctx.IsGuildMessage {
					t.Error("expected IsGuildMessage=true")
				}
				if ctx.IsDirectMessage {
					t.Error("expected IsDirectMessage=false")
				}
				if ctx.GuildSlug != "my-guild" {
					t.Errorf("GuildSlug: got %q, want 'my-guild'", ctx.GuildSlug)
				}
			},
		},
		{
			name: "guild disabled returns nil",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.GroupPolicy = "disabled"
			}},
			msgOpts: []func(*discordgo.MessageCreate){
				withGuildID("g1"),
				withContent("hello"),
			},
			setupChans: []*discordgo.Channel{{ID: "ch-1", GuildID: "g1", Name: "gen"}},
			wantNil:    true,
		},
		{
			name: "empty text + no attachments returns nil",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.DMPolicy = "open"
			}},
			msgOpts: []func(*discordgo.MessageCreate){withContent("")},
			wantNil: true,
		},
		{
			name: "mention stripped from text",
			monCtxOpts: []func(*DiscordMonitorContext){func(mc *DiscordMonitorContext) {
				mc.DMPolicy = "open"
			}},
			msgOpts: []func(*discordgo.MessageCreate){
				withContent("<@BOT123> do the thing"),
			},
			setupChans: []*discordgo.Channel{{ID: "ch-1", Name: "dm"}},
			validate: func(t *testing.T, ctx *DiscordMessagePreflightContext) {
				if ctx.BaseText != "do the thing" {
					t.Errorf("expected stripped text 'do the thing', got %q", ctx.BaseText)
				}
				if !ctx.WasMentioned {
					t.Error("expected WasMentioned=true")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			monCtx := newMonCtx(tc.monCtxOpts...)
			for _, ch := range tc.setupChans {
				addChannel(monCtx.Session, ch)
			}
			for _, g := range tc.setupGuilds {
				addGuild(monCtx.Session, g)
			}
			m := msgCreate(tc.msgOpts...)
			ctx := PreflightDiscordMessage(monCtx, m, tc.params)

			if tc.wantNil {
				if ctx != nil {
					t.Fatalf("expected nil context, got %+v", ctx)
				}
				return
			}
			if ctx == nil {
				t.Fatal("expected non-nil context")
			}
			if tc.validate != nil {
				tc.validate(t, ctx)
			}
		})
	}
}
