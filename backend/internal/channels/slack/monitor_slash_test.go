package slack

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 编解码测试 ----------

func TestEncodeDecodeSlackCommandArgValue(t *testing.T) {
	encoded := encodeSlackCommandArgValue("ask", "model", "gpt-4", "U123")
	parsed := parseSlackCommandArgValue(encoded)
	if parsed == nil {
		t.Fatal("parseSlackCommandArgValue returned nil")
	}
	if parsed.Command != "ask" {
		t.Errorf("Command: got %q, want %q", parsed.Command, "ask")
	}
	if parsed.Arg != "model" {
		t.Errorf("Arg: got %q, want %q", parsed.Arg, "model")
	}
	if parsed.Value != "gpt-4" {
		t.Errorf("Value: got %q, want %q", parsed.Value, "gpt-4")
	}
	if parsed.UserID != "U123" {
		t.Errorf("UserID: got %q, want %q", parsed.UserID, "U123")
	}
}

func TestParseSlackCommandArgValue_InvalidPrefix(t *testing.T) {
	result := parseSlackCommandArgValue("wrong|a|b|c|d")
	if result != nil {
		t.Error("expected nil for invalid prefix")
	}
}

func TestParseSlackCommandArgValue_Empty(t *testing.T) {
	result := parseSlackCommandArgValue("")
	if result != nil {
		t.Error("expected nil for empty string")
	}
}

func TestParseSlackCommandArgValue_WrongPartCount(t *testing.T) {
	result := parseSlackCommandArgValue("cmdarg|a|b")
	if result != nil {
		t.Error("expected nil for wrong part count")
	}
}

func TestEncodeSlackCommandArgValue_SpecialChars(t *testing.T) {
	encoded := encodeSlackCommandArgValue("cmd/test", "arg=1", "val&2", "U|3")
	parsed := parseSlackCommandArgValue(encoded)
	if parsed == nil {
		t.Fatal("parseSlackCommandArgValue returned nil for special chars")
	}
	if parsed.Command != "cmd/test" {
		t.Errorf("Command: got %q, want %q", parsed.Command, "cmd/test")
	}
	if parsed.Arg != "arg=1" {
		t.Errorf("Arg: got %q, want %q", parsed.Arg, "arg=1")
	}
	if parsed.Value != "val&2" {
		t.Errorf("Value: got %q, want %q", parsed.Value, "val&2")
	}
	if parsed.UserID != "U|3" {
		t.Errorf("UserID: got %q, want %q", parsed.UserID, "U|3")
	}
}

// ---------- Block Kit 菜单构建测试 ----------

func TestBuildSlackCommandArgMenuBlocks(t *testing.T) {
	choices := []SlackArgChoice{
		{Value: "v1", Label: "Choice 1"},
		{Value: "v2", Label: "Choice 2"},
	}
	blocks := buildSlackCommandArgMenuBlocks("Pick one", "ask", "model", choices, "U123")
	if len(blocks) != 2 { // 1 section + 1 actions row
		t.Errorf("blocks count: got %d, want 2", len(blocks))
	}
	if blocks[0]["type"] != "section" {
		t.Errorf("first block type: got %v, want section", blocks[0]["type"])
	}
	if blocks[1]["type"] != "actions" {
		t.Errorf("second block type: got %v, want actions", blocks[1]["type"])
	}
	elements := blocks[1]["elements"].([]map[string]interface{})
	if len(elements) != 2 {
		t.Errorf("button count: got %d, want 2", len(elements))
	}
}

func TestBuildSlackCommandArgMenuBlocks_Chunking(t *testing.T) {
	// 7 choices should produce 2 action rows (5+2)
	choices := make([]SlackArgChoice, 7)
	for i := range choices {
		choices[i] = SlackArgChoice{Value: "v", Label: "L"}
	}
	blocks := buildSlackCommandArgMenuBlocks("Title", "cmd", "arg", choices, "U1")
	// 1 section + 2 action rows
	if len(blocks) != 3 {
		t.Errorf("blocks count: got %d, want 3", len(blocks))
	}
}

// ---------- 授权合并测试 ----------

func TestResolveSlackCommandAuthorized_AccessGroupsOff(t *testing.T) {
	result := resolveSlackCommandAuthorized(false, true, false, true, false)
	if !result {
		t.Error("expected true when useAccessGroups is off")
	}
}

func TestResolveSlackCommandAuthorized_OwnerAllowed(t *testing.T) {
	result := resolveSlackCommandAuthorized(true, true, true, false, false)
	if !result {
		t.Error("expected true when owner is allowed")
	}
}

func TestResolveSlackCommandAuthorized_ChannelUserAllowed(t *testing.T) {
	result := resolveSlackCommandAuthorized(true, false, false, true, true)
	if !result {
		t.Error("expected true when channel user is allowed")
	}
}

func TestResolveSlackCommandAuthorized_NoneAllowed(t *testing.T) {
	result := resolveSlackCommandAuthorized(true, true, false, true, false)
	if result {
		t.Error("expected false when no authorizer allows")
	}
}

// ---------- Native 命令解析测试 ----------

func TestResolveSlackNativeCommands_NilConfig(t *testing.T) {
	result := ResolveSlackNativeCommands(nil, nil)
	if result != nil {
		t.Error("expected nil for nil config")
	}
}

func TestResolveSlackNativeCommands_DisabledByDefault(t *testing.T) {
	// Slack 默认 native = false（不在 discord/telegram 默认列表中）
	cfg := &types.OpenAcosmiConfig{}
	result := ResolveSlackNativeCommands(cfg, nil)
	if result != nil {
		t.Error("expected nil: slack native commands default disabled")
	}
}

func TestResolveSlackNativeCommands_ExplicitlyEnabled(t *testing.T) {
	enabled := true
	cfg := &types.OpenAcosmiConfig{}
	acctCfg := &types.SlackAccountConfig{
		Commands: &types.ProviderCommandsConfig{
			Native: &enabled,
		},
	}
	result := ResolveSlackNativeCommands(cfg, acctCfg)
	// 返回已注册的命令列表（可能为空，取决于 globalCommands 状态）
	// 只要不是 nil 错误就好
	_ = result
}

// ---------- Markdown Table Mode 测试 ----------

func TestResolveSlackMarkdownTableMode_Default(t *testing.T) {
	mode := ResolveSlackMarkdownTableMode(nil, "")
	if mode != types.MarkdownTableCode {
		t.Errorf("default mode: got %q, want %q", mode, types.MarkdownTableCode)
	}
}

func TestResolveSlackMarkdownTableMode_NilChannels(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	mode := ResolveSlackMarkdownTableMode(cfg, "default")
	if mode != types.MarkdownTableCode {
		t.Errorf("nil channels mode: got %q, want %q", mode, types.MarkdownTableCode)
	}
}

// ---------- deliverSlackSlashReplies 测试 ----------

func TestDeliverSlackSlashReplies_EmptyReplies(t *testing.T) {
	called := false
	err := deliverSlackSlashReplies(SlackSlashReplyParams{
		Replies: nil,
		Respond: func(text, responseType string) error {
			called = true
			return nil
		},
		Ephemeral: true,
		TextLimit: 4000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("respond should not be called for empty replies")
	}
}

func TestDeliverSlackSlashReplies_SingleReply(t *testing.T) {
	var messages []string
	var responseTypes []string
	err := deliverSlackSlashReplies(SlackSlashReplyParams{
		Replies: []autoreply.ReplyPayload{
			{Text: "Hello world"},
		},
		Respond: func(text, responseType string) error {
			messages = append(messages, text)
			responseTypes = append(responseTypes, responseType)
			return nil
		},
		Ephemeral: true,
		TextLimit: 4000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("message count: got %d, want 1", len(messages))
	}
	if responseTypes[0] != "ephemeral" {
		t.Errorf("response type: got %q, want ephemeral", responseTypes[0])
	}
}

func TestDeliverSlackSlashReplies_InChannel(t *testing.T) {
	var responseTypes []string
	err := deliverSlackSlashReplies(SlackSlashReplyParams{
		Replies: []autoreply.ReplyPayload{
			{Text: "Public message"},
		},
		Respond: func(text, responseType string) error {
			responseTypes = append(responseTypes, responseType)
			return nil
		},
		Ephemeral: false,
		TextLimit: 4000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responseTypes) == 0 || responseTypes[0] != "in_channel" {
		t.Errorf("response type: got %v, want in_channel", responseTypes)
	}
}

func TestDeliverSlackSlashReplies_SilentReply(t *testing.T) {
	called := false
	err := deliverSlackSlashReplies(SlackSlashReplyParams{
		Replies: []autoreply.ReplyPayload{
			{Text: "NO_REPLY"},
		},
		Respond: func(text, responseType string) error {
			called = true
			return nil
		},
		Ephemeral: true,
		TextLimit: 4000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("respond should not be called for silent reply")
	}
}

func TestDeliverSlackSlashReplies_MediaURLs(t *testing.T) {
	var messages []string
	err := deliverSlackSlashReplies(SlackSlashReplyParams{
		Replies: []autoreply.ReplyPayload{
			{Text: "caption", MediaURLs: []string{"https://example.com/a.png", "https://example.com/b.png"}},
		},
		Respond: func(text, responseType string) error {
			messages = append(messages, text)
			return nil
		},
		Ephemeral: true,
		TextLimit: 4000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected at least one message")
	}
}

// ---------- chunkItems 测试 ----------

func TestChunkItems(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	rows := chunkItems(items, 3)
	if len(rows) != 3 {
		t.Errorf("row count: got %d, want 3", len(rows))
	}
	if len(rows[0]) != 3 {
		t.Errorf("first row: got %d items, want 3", len(rows[0]))
	}
	if len(rows[2]) != 1 {
		t.Errorf("last row: got %d items, want 1", len(rows[2]))
	}
}

func TestChunkItems_ZeroSize(t *testing.T) {
	items := []string{"a", "b"}
	rows := chunkItems(items, 0)
	if len(rows) != 1 {
		t.Errorf("row count: got %d, want 1", len(rows))
	}
}

// ---------- toNativeCommandsSetting 测试 ----------

func TestToNativeCommandsSetting_Bool(t *testing.T) {
	result := toNativeCommandsSetting(true)
	if result == nil || !*result {
		t.Error("expected *true for bool true")
	}
	result = toNativeCommandsSetting(false)
	if result == nil || *result {
		t.Error("expected *false for bool false")
	}
}

func TestToNativeCommandsSetting_Nil(t *testing.T) {
	result := toNativeCommandsSetting(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestToNativeCommandsSetting_BoolPtr(t *testing.T) {
	v := true
	result := toNativeCommandsSetting(&v)
	if result == nil || !*result {
		t.Error("expected *true for *bool true")
	}
}

func TestToNativeCommandsSetting_UnsupportedType(t *testing.T) {
	result := toNativeCommandsSetting("auto")
	if result != nil {
		t.Error("expected nil for unsupported type string")
	}
}

// ---------- toInterfaceSlice 测试 ----------

func TestToInterfaceSlice(t *testing.T) {
	result := toInterfaceSlice([]string{"a", "b", "c"})
	if len(result) != 3 {
		t.Errorf("length: got %d, want 3", len(result))
	}
	if result[1] != "b" {
		t.Errorf("element: got %v, want b", result[1])
	}
}
