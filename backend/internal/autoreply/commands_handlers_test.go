package autoreply

import (
	"context"
	"testing"
)

// ---------- 核心分发器测试 ----------

func TestHandleCommands_ResetCommand(t *testing.T) {
	params := &HandleCommandsParams{
		MsgCtx: &MsgContext{CommandSource: "text"},
		Command: &CommandContext{
			CommandBodyNormalized: "/reset",
			IsAuthorizedSender:    true,
		},
		SessionKey: "test-session",
	}

	result, err := HandleCommands(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ShouldContinue {
		t.Error("ShouldContinue should be false for /reset")
	}
	if result.Reply == nil || result.Reply.Text == "" {
		t.Error("expected reply text for /reset")
	}
}

func TestHandleCommands_ResetCommand_Unauthorized(t *testing.T) {
	params := &HandleCommandsParams{
		MsgCtx: &MsgContext{CommandSource: "text"},
		Command: &CommandContext{
			CommandBodyNormalized: "/reset",
			IsAuthorizedSender:    false,
		},
	}

	result, err := HandleCommands(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ShouldContinue {
		t.Error("ShouldContinue should be false for unauthorized /reset")
	}
}

func TestHandleCommands_NilParams(t *testing.T) {
	result, err := HandleCommands(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for nil params")
	}
	if !result.ShouldContinue {
		t.Error("ShouldContinue should be true for nil params")
	}
}

func TestShouldHandleTextCommands(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected bool
	}{
		{"native source", "native", true},
		{"text source", "text", true},
		{"empty source", "", false},
		{"other source", "other", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldHandleTextCommands(&ShouldHandleTextCommandsParams{
				CommandSource: tt.source,
			})
			if got != tt.expected {
				t.Errorf("ShouldHandleTextCommands(%q) = %v, want %v", tt.source, got, tt.expected)
			}
		})
	}
}

// ---------- Approve 命令解析测试 ----------

func TestParseApproveCommand(t *testing.T) {
	tests := []struct {
		body         string
		wantNil      bool
		wantDecision ApproveDecision
		wantReqID    string
	}{
		{"/approve", false, ApproveAllowOnce, ""},
		{"/approve allow", false, ApproveAllowOnce, ""},
		{"/approve always", false, ApproveAllowAlways, ""},
		{"/approve deny req-123", false, ApproveDeny, "req-123"},
		{"/approve reject", false, ApproveDeny, ""},
		{"/help", true, "", ""},
		{"not a command", true, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseApproveCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q, got %+v", tt.body, result)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Decision != tt.wantDecision {
				t.Errorf("Decision = %q, want %q", result.Decision, tt.wantDecision)
			}
			if result.RequestID != tt.wantReqID {
				t.Errorf("RequestID = %q, want %q", result.RequestID, tt.wantReqID)
			}
		})
	}
}

// ---------- PTT 命令解析测试 ----------

func TestParsePTTArgs(t *testing.T) {
	tests := []struct {
		body     string
		wantSub  string
		wantHint string
	}{
		{"/ptt", "status", ""},
		{"/ptt start", "start", ""},
		{"/ptt on", "start", ""}, // alias
		{"/ptt off", "stop", ""}, // alias
		{"/ptt once node1", "once", "node1"},
		{"/ptt toggle my device", "toggle", "my device"},
		{"/ptt invalid", "", ""},
		{"/help", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			sub, hint := parsePTTArgs(tt.body)
			if sub != tt.wantSub {
				t.Errorf("sub = %q, want %q", sub, tt.wantSub)
			}
			if hint != tt.wantHint {
				t.Errorf("hint = %q, want %q", hint, tt.wantHint)
			}
		})
	}
}

// ---------- Config 命令解析测试 ----------

func TestParseConfigCommand(t *testing.T) {
	tests := []struct {
		body       string
		wantNil    bool
		wantAction string
		wantPath   string
		wantValue  string
		wantTemp   bool
	}{
		{"/config", false, "list", "", "", false},
		{"/config get ai.model", false, "get", "ai.model", "", false},
		{"/config set ai.model gpt-4", false, "set", "ai.model", "gpt-4", false},
		{"/config unset ai.model", false, "unset", "ai.model", "", false},
		{"/config reset", false, "reset", "", "", false},
		{"/config temp set key val", false, "set", "key", "val", true},
		{"/config validate", false, "validate", "", "", false},
		{"/help", true, "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseConfigCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q, got %+v", tt.body, result)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
			if result.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", result.Path, tt.wantPath)
			}
			if result.IsTemp != tt.wantTemp {
				t.Errorf("IsTemp = %v, want %v", result.IsTemp, tt.wantTemp)
			}
		})
	}
}

// ---------- TTS 命令解析测试 ----------

func TestParseTtsCommand(t *testing.T) {
	tests := []struct {
		body       string
		wantNil    bool
		wantAction string
	}{
		{"/tts", false, "toggle"},
		{"/tts on", false, "on"},
		{"/tts off", false, "off"},
		{"/tts say hello world", false, "say"},
		{"/tts provider openai", false, "provider"},
		{"/tts status", false, "status"},
		{"/tts providers", false, "providers"},
		{"/help", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseTtsCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q", tt.body)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
		})
	}
}

// ---------- Models 命令解析测试 ----------

func TestParseModelsCommand(t *testing.T) {
	tests := []struct {
		body         string
		wantNil      bool
		wantAction   string
		wantModel    string
		wantProvider string
	}{
		{"/model", false, "list", "", ""},
		{"/models", false, "list", "", ""},
		{"/model list", false, "list", "", ""},
		{"/model set gpt-4", false, "set", "gpt-4", ""},
		{"/model set openai/gpt-4", false, "set", "gpt-4", "openai"},
		{"/model info gpt-4", false, "info", "gpt-4", ""},
		{"/model providers", false, "providers", "", ""},
		{"/model search fast", false, "search", "", ""},
		{"/help", true, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseModelsCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q", tt.body)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
			if result.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", result.Model, tt.wantModel)
			}
			if result.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", result.Provider, tt.wantProvider)
			}
		})
	}
}

// ---------- Session 命令解析测试 ----------

func TestParseSessionCommand(t *testing.T) {
	tests := []struct {
		body       string
		wantNil    bool
		wantAction string
	}{
		{"/session", false, "list"},
		{"/sessions", false, "list"},
		{"/session list", false, "list"},
		{"/session switch my-session", false, "switch"},
		{"/session new test session", false, "new"},
		{"/session delete old-key", false, "delete"},
		{"/session stop", false, "stop"},
		{"/session restart", false, "restart"},
		{"/help", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseSessionCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q", tt.body)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
		})
	}
}

// ---------- Context Report 命令解析测试 ----------

func TestParseContextReportCommand(t *testing.T) {
	tests := []struct {
		body        string
		wantNil     bool
		wantAction  string
		wantVerbose bool
	}{
		{"/context-report", false, "full", false},
		{"/cr", false, "full", false},
		{"/cr summary", false, "summary", false},
		{"/cr tokens", false, "tokens", false},
		{"/cr full -v", false, "full", true},
		{"/cr --verbose", false, "full", true},
		{"/help", true, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseContextReportCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q", tt.body)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
			if result.Verbose != tt.wantVerbose {
				t.Errorf("Verbose = %v, want %v", result.Verbose, tt.wantVerbose)
			}
		})
	}
}

// ---------- Subagents 命令解析测试 ----------

func TestParseSubagentsCommand(t *testing.T) {
	tests := []struct {
		body       string
		wantNil    bool
		wantAction string
	}{
		{"/subagent", false, "list"},
		{"/subagents", false, "list"},
		{"/subagent list", false, "list"},
		{"/subagent create my-agent", false, "create"},
		{"/subagent delete agent-1", false, "delete"},
		{"/subagent send agent-1 hello", false, "send"},
		{"/subagent switch agent-2", false, "switch"},
		{"/help", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseSubagentsCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q", tt.body)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
		})
	}
}

// ---------- Allowlist 命令解析测试 ----------

func TestParseAllowlistCommand(t *testing.T) {
	tests := []struct {
		body       string
		wantNil    bool
		wantAction string
	}{
		{"/allowlist", false, "list"},
		{"/whitelist", false, "list"},
		{"/acl", false, "list"},
		{"/allowlist add user123", false, "add"},
		{"/allowlist remove user123", false, "remove"},
		{"/allowlist check user123", false, "check"},
		{"/allowlist clear", false, "clear"},
		{"/allowlist pair ch-123", false, "pair"},
		{"/allowlist unpair ch-123", false, "unpair"},
		{"/allowlist pairs", false, "pairs"},
		{"/help", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			result := parseAllowlistCommand(tt.body)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil for %q", tt.body)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil for %q", tt.body)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
		})
	}
}

// ---------- BuildCommandContext 测试 ----------

func TestBuildCommandContext(t *testing.T) {
	ctx := &MsgContext{
		Surface:     "chat",
		ChannelType: "telegram",
		ChannelID:   "ch-123",
		SenderID:    "user-1",
		From:        "+1234567890",
		To:          "bot",
	}

	cmdCtx := BuildCommandContext(&BuildCommandContextParams{
		Ctx:                   ctx,
		AgentID:               "default",
		SessionKey:            "sess-123",
		IsGroup:               false,
		TriggerBodyNormalized: "/help",
		CommandAuthorized:     true,
		OwnerList:             []string{"user-1"},
	})

	if cmdCtx == nil {
		t.Fatal("expected non-nil CommandContext")
	}
	if cmdCtx.Surface != "chat" {
		t.Errorf("Surface = %q, want %q", cmdCtx.Surface, "chat")
	}
	if cmdCtx.SenderIsOwner != true {
		t.Error("SenderIsOwner should be true")
	}
	if cmdCtx.CommandBodyNormalized == "" {
		t.Error("CommandBodyNormalized should not be empty")
	}
}

// ---------- 处理器返回 nil 测试 ----------

func TestHandleBashCommand_NoMatch(t *testing.T) {
	params := &HandleCommandsParams{
		MsgCtx:  &MsgContext{},
		Command: &CommandContext{CommandBodyNormalized: "/help"},
	}
	result, err := HandleBashCommand(context.Background(), params, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for non-bash command")
	}
}

func TestHandlePluginCommand_NilMatcher(t *testing.T) {
	params := &HandleCommandsParams{
		MsgCtx:  &MsgContext{},
		Command: &CommandContext{CommandBodyNormalized: "/custom"},
	}
	result, err := HandlePluginCommand(context.Background(), params, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil when PluginMatcher is nil")
	}
}

func TestTruncateForDisplay(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a …"},
		{"", 10, ""},
	}
	for _, tt := range tests {
		got := truncateForDisplay(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateForDisplay(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
