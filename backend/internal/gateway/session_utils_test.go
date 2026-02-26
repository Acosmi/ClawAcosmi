package gateway

import (
	"testing"

	"github.com/anthropic/open-acosmi/internal/session"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- ClassifySessionKey ----------

func TestClassifySessionKey_Global(t *testing.T) {
	if got := ClassifySessionKey("global", nil); got != "global" {
		t.Errorf("got %q, want global", got)
	}
}

func TestClassifySessionKey_Unknown(t *testing.T) {
	if got := ClassifySessionKey("unknown", nil); got != "unknown" {
		t.Errorf("got %q, want unknown", got)
	}
}

func TestClassifySessionKey_Direct(t *testing.T) {
	if got := ClassifySessionKey("some-session-id", nil); got != "direct" {
		t.Errorf("got %q, want direct", got)
	}
}

func TestClassifySessionKey_GroupByEntry(t *testing.T) {
	entry := &SessionEntry{ChatType: "group"}
	if got := ClassifySessionKey("some-key", entry); got != "group" {
		t.Errorf("got %q, want group", got)
	}
}

func TestClassifySessionKey_GroupByKey(t *testing.T) {
	if got := ClassifySessionKey("telegram:group:123", nil); got != "group" {
		t.Errorf("got %q, want group", got)
	}
	if got := ClassifySessionKey("slack:channel:456", nil); got != "group" {
		t.Errorf("got %q, want group", got)
	}
}

// ---------- ParseGroupKey ----------

func TestParseGroupKey_Valid(t *testing.T) {
	result := ParseGroupKey("telegram:group:12345")
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Channel != "telegram" || result.Kind != "group" || result.ID != "12345" {
		t.Errorf("got %+v", result)
	}
}

func TestParseGroupKey_Channel(t *testing.T) {
	result := ParseGroupKey("slack:channel:general")
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Channel != "slack" || result.Kind != "channel" || result.ID != "general" {
		t.Errorf("got %+v", result)
	}
}

func TestParseGroupKey_NotGroup(t *testing.T) {
	if ParseGroupKey("simple-key") != nil {
		t.Error("expected nil for non-group key")
	}
}

func TestParseGroupKey_AgentPrefix(t *testing.T) {
	result := ParseGroupKey("agent:mybot:telegram:group:999")
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Channel != "telegram" || result.Kind != "group" || result.ID != "999" {
		t.Errorf("got %+v", result)
	}
}

// ---------- IsCronRunSessionKey ----------

func TestIsCronRunSessionKey_True(t *testing.T) {
	if !IsCronRunSessionKey("cron:daily-report:run:abc123") {
		t.Error("expected true")
	}
}

func TestIsCronRunSessionKey_False(t *testing.T) {
	if IsCronRunSessionKey("session-abc") {
		t.Error("expected false for normal key")
	}
	if IsCronRunSessionKey("cron:daily-report") {
		t.Error("expected false for cron without run")
	}
}

func TestIsCronRunSessionKey_AgentPrefix(t *testing.T) {
	if !IsCronRunSessionKey("agent:bot:cron:daily:run:xyz") {
		t.Error("expected true with agent prefix")
	}
}

// ---------- CanonicalizeSessionKeyForAgent ----------

func TestCanonicalizeSessionKeyForAgent_Normal(t *testing.T) {
	got := CanonicalizeSessionKeyForAgent("myBot", "session-123")
	want := "agent:mybot:session-123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCanonicalizeSessionKeyForAgent_Global(t *testing.T) {
	if got := CanonicalizeSessionKeyForAgent("myBot", "global"); got != "global" {
		t.Errorf("got %q, want global", got)
	}
}

func TestCanonicalizeSessionKeyForAgent_AlreadyPrefixed(t *testing.T) {
	key := "agent:other:session-456"
	if got := CanonicalizeSessionKeyForAgent("myBot", key); got != key {
		t.Errorf("got %q, want %q", got, key)
	}
}

// ---------- DeriveSessionTitle ----------

func TestDeriveSessionTitle_DisplayName(t *testing.T) {
	entry := &SessionEntry{DisplayName: "My Chat"}
	if got := DeriveSessionTitle(entry, "hello"); got != "My Chat" {
		t.Errorf("got %q", got)
	}
}

func TestDeriveSessionTitle_Subject(t *testing.T) {
	entry := &SessionEntry{Subject: "Bug Report"}
	if got := DeriveSessionTitle(entry, ""); got != "Bug Report" {
		t.Errorf("got %q", got)
	}
}

func TestDeriveSessionTitle_FirstMsg(t *testing.T) {
	entry := &SessionEntry{}
	msg := "Hello, I need help with something important"
	got := DeriveSessionTitle(entry, msg)
	if got == "" {
		t.Error("expected non-empty")
	}
}

func TestDeriveSessionTitle_Nil(t *testing.T) {
	if got := DeriveSessionTitle(nil, "msg"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ---------- TruncateTitle ----------

func TestTruncateTitle_Short(t *testing.T) {
	if got := TruncateTitle("hello", 20); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateTitle_Truncate(t *testing.T) {
	long := "This is a very long title that should be truncated at some point"
	got := TruncateTitle(long, 30)
	if len([]rune(got)) > 30 {
		t.Errorf("got length %d, want <= 30", len([]rune(got)))
	}
}

// ---------- ResolveAvatarMime ----------

func TestResolveAvatarMime(t *testing.T) {
	tests := map[string]string{
		"avatar.png":  "image/png",
		"photo.jpg":   "image/jpeg",
		"photo.jpeg":  "image/jpeg",
		"icon.svg":    "image/svg+xml",
		"pic.webp":    "image/webp",
		"unknown.bin": "application/octet-stream",
	}
	for file, want := range tests {
		if got := ResolveAvatarMime(file); got != want {
			t.Errorf("ResolveAvatarMime(%q) = %q, want %q", file, got, want)
		}
	}
}

// ---------- IsWorkspaceRelativePath ----------

func TestIsWorkspaceRelativePath(t *testing.T) {
	if IsWorkspaceRelativePath("") {
		t.Error("empty should be false")
	}
	if IsWorkspaceRelativePath("~/.config") {
		t.Error("tilde should be false")
	}
	if IsWorkspaceRelativePath("http://example.com/img.png") {
		t.Error("http should be false")
	}
	if !IsWorkspaceRelativePath("assets/avatar.png") {
		t.Error("relative path should be true")
	}
}

// ---------- FormatSessionIdPrefix ----------

func TestFormatSessionIdPrefix_Short(t *testing.T) {
	got := FormatSessionIdPrefix("abc", 0)
	if got != "abc" {
		t.Errorf("got %q", got)
	}
}

func TestFormatSessionIdPrefix_Long(t *testing.T) {
	got := FormatSessionIdPrefix("abcdefgh-1234", 0)
	if got != "abcdefgh" {
		t.Errorf("got %q", got)
	}
}

func TestFormatSessionIdPrefix_WithTimestamp(t *testing.T) {
	got := FormatSessionIdPrefix("abcdefgh-1234", 1707840000000) // 2024-02-13
	if got == "" || got == "abcdefgh" {
		t.Errorf("expected date in result, got %q", got)
	}
}

// ---------- NormalizeAgentId ----------

func TestNormalizeAgentId(t *testing.T) {
	if got := NormalizeAgentId("  MyBot  "); got != "mybot" {
		t.Errorf("got %q", got)
	}
}

// ---------- ResolveSessionStoreKey ----------
// 对齐 TS: session-utils.test.ts L45-77

func TestResolveSessionStoreKey_MainAliases(t *testing.T) {
	isDefault := true
	cfg := &types.OpenAcosmiConfig{
		Session: &types.SessionConfig{MainKey: "work"},
		Agents:  &types.AgentsConfig{List: []types.AgentListItemConfig{{ID: "ops", Default: &isDefault}}},
	}
	tests := map[string]string{
		"main":           "agent:ops:work",
		"work":           "agent:ops:work",
		"agent:ops:main": "agent:ops:work",
	}
	for input, want := range tests {
		if got := ResolveSessionStoreKey(cfg, input); got != want {
			t.Errorf("ResolveSessionStoreKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveSessionStoreKey_BareKeys(t *testing.T) {
	isDefault := true
	cfg := &types.OpenAcosmiConfig{
		Session: &types.SessionConfig{MainKey: "main"},
		Agents:  &types.AgentsConfig{List: []types.AgentListItemConfig{{ID: "ops", Default: &isDefault}}},
	}
	if got := ResolveSessionStoreKey(cfg, "discord:group:123"); got != "agent:ops:discord:group:123" {
		t.Errorf("bare group key: got %q", got)
	}
	if got := ResolveSessionStoreKey(cfg, "agent:alpha:main"); got != "agent:alpha:main" {
		t.Errorf("agent-prefixed key: got %q", got)
	}
}

func TestResolveSessionStoreKey_GlobalScope(t *testing.T) {
	isDefault := true
	cfg := &types.OpenAcosmiConfig{
		Session: &types.SessionConfig{Scope: "global", MainKey: "work"},
		Agents:  &types.AgentsConfig{List: []types.AgentListItemConfig{{ID: "ops", Default: &isDefault}}},
	}
	if got := ResolveSessionStoreKey(cfg, "main"); got != "global" {
		t.Errorf("global scope main: got %q, want global", got)
	}
}

func TestResolveSessionStoreKey_Empty(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	if got := ResolveSessionStoreKey(cfg, ""); got != "" {
		t.Errorf("empty key: got %q", got)
	}
	if got := ResolveSessionStoreKey(cfg, "global"); got != "global" {
		t.Errorf("global: got %q", got)
	}
	if got := ResolveSessionStoreKey(cfg, "unknown"); got != "unknown" {
		t.Errorf("unknown: got %q", got)
	}
}

// ---------- LoadCombinedStore ----------

func TestLoadCombinedStore_Canonicalizes(t *testing.T) {
	store := NewSessionStore("")
	store.Save(&SessionEntry{SessionKey: "main", SessionId: "sess-1", UpdatedAt: 100})
	store.Save(&SessionEntry{SessionKey: "discord:group:dev", SessionId: "sess-2", UpdatedAt: 200})
	store.Save(&SessionEntry{SessionKey: "agent:ops:work", SessionId: "sess-3", UpdatedAt: 300})

	combined := store.LoadCombinedStore("ops")
	if _, ok := combined["agent:ops:main"]; !ok {
		t.Error("expected 'main' to be canonicalized to 'agent:ops:main'")
	}
	if _, ok := combined["agent:ops:discord:group:dev"]; !ok {
		t.Error("expected bare group key to be canonicalized")
	}
	if _, ok := combined["agent:ops:work"]; !ok {
		t.Error("expected already-prefixed key to remain")
	}
}

// ---------- F1: BuildGroupDisplayName ----------

func TestBuildGroupDisplayName_ProviderSubject(t *testing.T) {
	got := BuildGroupDisplayName("telegram", "Dev Chat", "", "", "", "")
	if got != "telegram:g-dev-chat" {
		t.Errorf("got %q, want telegram:g-dev-chat", got)
	}
}

func TestBuildGroupDisplayName_GroupChannel(t *testing.T) {
	got := BuildGroupDisplayName("slack", "", "#general", "Workspace", "", "")
	// gc="#general" already has #, so no extra # prefix: "Workspace#general"
	if got != "slack:workspace#general" {
		t.Errorf("got %q, want slack:workspace#general", got)
	}
}

func TestBuildGroupDisplayName_FallbackId(t *testing.T) {
	got := BuildGroupDisplayName("discord", "", "", "", "123456", "")
	if got != "discord:g-123456" {
		t.Errorf("got %q, want discord:g-123456", got)
	}
}

func TestBuildGroupDisplayName_EmptyProvider(t *testing.T) {
	got := BuildGroupDisplayName("", "Chat", "", "", "", "")
	if got != "group:g-chat" {
		t.Errorf("got %q, want group:g-chat", got)
	}
}

func TestBuildGroupDisplayName_LongId(t *testing.T) {
	longId := "abcdefghijklmnop1234567890"
	got := BuildGroupDisplayName("tg", "", "", "", longId, "")
	// should contain shortened form
	if got == "" {
		t.Error("expected non-empty")
	}
}

// ---------- F1: NormalizeSessionDeliveryFields ----------

func TestNormalizeSessionDeliveryFields_Nil(t *testing.T) {
	result := NormalizeSessionDeliveryFields(nil)
	if result.DeliveryContext != nil {
		t.Error("nil entry should return nil delivery context")
	}
}

func TestNormalizeSessionDeliveryFields_FullEntry(t *testing.T) {
	entry := &SessionEntry{
		Channel: "telegram",
		LastChannel: &session.SessionLastChannel{
			Channel:   "slack",
			AccountId: "acc-1",
			To:        "to-1",
		},
		LastTo:        "to-2",
		LastAccountId: "acc-2",
		DeliveryContext: &session.DeliveryContext{
			Channel:   "discord",
			To:        "to-fallback",
			AccountId: "acc-fallback",
		},
	}
	result := NormalizeSessionDeliveryFields(entry)
	if result.DeliveryContext == nil {
		t.Fatal("expected non-nil delivery context")
	}
	// LastChannel.Channel wins over entry.Channel and DeliveryContext.Channel
	if result.LastChannel != "slack" {
		t.Errorf("LastChannel = %q, want slack", result.LastChannel)
	}
	// LastTo wins over DeliveryContext.To
	if result.LastTo != "to-2" {
		t.Errorf("LastTo = %q, want to-2", result.LastTo)
	}
	// LastAccountId wins over DeliveryContext.AccountId
	if result.LastAccountId != "acc-2" {
		t.Errorf("LastAccountId = %q, want acc-2", result.LastAccountId)
	}
}

func TestNormalizeSessionDeliveryFields_FallbackOnly(t *testing.T) {
	entry := &SessionEntry{
		DeliveryContext: &session.DeliveryContext{
			Channel:   "telegram",
			To:        "12345",
			AccountId: "bot-acc",
		},
	}
	result := NormalizeSessionDeliveryFields(entry)
	if result.LastChannel != "telegram" {
		t.Errorf("got %q, want telegram", result.LastChannel)
	}
	if result.LastTo != "12345" {
		t.Errorf("got %q, want 12345", result.LastTo)
	}
}

func TestNormalizeSessionDeliveryFields_EmptyEntry(t *testing.T) {
	entry := &SessionEntry{}
	result := NormalizeSessionDeliveryFields(entry)
	if result.DeliveryContext != nil {
		t.Error("empty entry should return nil delivery context")
	}
}

// ---------- F1: ResolveSessionModelRef ----------

func TestResolveSessionModelRef_NoOverride(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	p, m := ResolveSessionModelRef(cfg, &SessionEntry{}, "")
	if p == "" || m == "" {
		t.Errorf("expected resolved model, got provider=%q model=%q", p, m)
	}
}

func TestResolveSessionModelRef_WithOverride(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	entry := &SessionEntry{
		ProviderOverride: "openai",
		ModelOverride:    "gpt-4o",
	}
	p, m := ResolveSessionModelRef(cfg, entry, "")
	if p != "openai" {
		t.Errorf("provider = %q, want openai", p)
	}
	if m != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", m)
	}
}

func TestResolveSessionModelRef_NilEntry(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	p, m := ResolveSessionModelRef(cfg, nil, "")
	if p == "" || m == "" {
		t.Errorf("expected resolved model even with nil entry, got provider=%q model=%q", p, m)
	}
}
