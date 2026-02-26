package autoreply

import "testing"

func TestFindCommand(t *testing.T) {
	cmd := FindCommand("status")
	if cmd == nil {
		t.Fatal("should find 'status' command")
	}
	if cmd.Key != "status" {
		t.Errorf("Key = %q, want %q", cmd.Key, "status")
	}
}

func TestFindCommand_ByAlias(t *testing.T) {
	cmd := FindCommand("/help")
	if cmd == nil {
		t.Fatal("should find 'help' command by '/help' alias")
	}
	if cmd.Key != "help" {
		t.Errorf("Key = %q, want %q", cmd.Key, "help")
	}
}

func TestIsCommandMessage(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"/status", true},
		{"/model gpt-4", true},
		{"/help", true},
		{"/unknown", false},
		{"hello", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsCommandMessage(tt.body, nil)
		if got != tt.want {
			t.Errorf("IsCommandMessage(%q) = %v, want %v", tt.body, got, tt.want)
		}
	}
}

func TestResolveTextCommand(t *testing.T) {
	result := ResolveTextCommand("/model gpt-4", nil)
	if result == nil {
		t.Fatal("should resolve /model command")
	}
	if result.Command.Key != "model" {
		t.Errorf("Key = %q, want %q", result.Command.Key, "model")
	}
	if result.Rest != "gpt-4" {
		t.Errorf("Rest = %q, want %q", result.Rest, "gpt-4")
	}
}

func TestResolveTextCommand_NoArgs(t *testing.T) {
	result := ResolveTextCommand("/stop", nil)
	if result == nil {
		t.Fatal("should resolve /stop command")
	}
	if result.Rest != "" {
		t.Errorf("Rest = %q, want empty", result.Rest)
	}
}

func TestNormalizeCommandBody_WithBot(t *testing.T) {
	result := NormalizeCommandBody("@mybot /status", &CommandNormalizeOptions{BotUsername: "mybot"})
	if result != "/status" {
		t.Errorf("NormalizeCommandBody = %q, want %q", result, "/status")
	}
}

func TestParseCommandArgs_Config(t *testing.T) {
	cmd := FindCommand("config")
	if cmd == nil {
		t.Fatal("config command not found")
	}
	// config 使用 ArgsParsingNone + FormatArgs (对齐 TS)，
	// 所以 raw 被整体放入第一个参数。
	args := ParseCommandArgs(cmd, "set agents.model gpt-4")
	if args.Values["action"] != "set agents.model gpt-4" {
		t.Errorf("action = %v, want 'set agents.model gpt-4'", args.Values["action"])
	}
}

func TestSerializeCommandArgs(t *testing.T) {
	cmd := FindCommand("config")
	if cmd == nil {
		t.Fatal("config command not found")
	}
	result := SerializeCommandArgs(cmd, CommandArgValues{
		"action": "set",
		"path":   "agents.model",
		"value":  "gpt-4",
	})
	if result != "set agents.model=gpt-4" {
		t.Errorf("SerializeCommandArgs = %q, want %q", result, "set agents.model=gpt-4")
	}
}

func TestListChatCommands(t *testing.T) {
	commands := ListChatCommands()
	if len(commands) == 0 {
		t.Error("should have registered commands")
	}
}

func TestGetCommandDetection(t *testing.T) {
	detection := GetCommandDetection()
	if detection == nil {
		t.Fatal("detection should not be nil")
	}
	if _, ok := detection.Exact["/status"]; !ok {
		t.Error("/status should be in exact set")
	}
	if _, ok := detection.Exact["/help"]; !ok {
		t.Error("/help should be in exact set")
	}
}

func TestNormalizeGroupActivation(t *testing.T) {
	mode, ok := NormalizeGroupActivation("mention")
	if !ok || mode != GroupActivationMention {
		t.Errorf("NormalizeGroupActivation('mention') = (%q, %v)", mode, ok)
	}
	mode, ok = NormalizeGroupActivation("always")
	if !ok || mode != GroupActivationAlways {
		t.Errorf("NormalizeGroupActivation('always') = (%q, %v)", mode, ok)
	}
	_, ok = NormalizeGroupActivation("invalid")
	if ok {
		t.Error("invalid should not match")
	}
}

func TestNormalizeSendPolicyOverride(t *testing.T) {
	policy, ok := NormalizeSendPolicyOverride("allow")
	if !ok || policy != SendPolicyAllow {
		t.Errorf("NormalizeSendPolicyOverride('allow') = (%q, %v)", policy, ok)
	}
	policy, ok = NormalizeSendPolicyOverride("deny")
	if !ok || policy != SendPolicyDeny {
		t.Errorf("NormalizeSendPolicyOverride('deny') = (%q, %v)", policy, ok)
	}
}

func TestNormalizeArgValue(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{"hello", "hello"},
		{" hello ", "hello"},
		{42, "42"},
		{true, "true"},
		{nil, ""},
	}
	for _, tt := range tests {
		got := NormalizeArgValue(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeArgValue(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------- E3 新增函数测试 ----------

func TestIsCommandEnabledForConfig(t *testing.T) {
	// nil config: config/debug/bash 默认禁用
	if IsCommandEnabledForConfig(nil, "config") {
		t.Error("config should be disabled with nil cfg")
	}
	if IsCommandEnabledForConfig(nil, "debug") {
		t.Error("debug should be disabled with nil cfg")
	}
	if IsCommandEnabledForConfig(nil, "bash") {
		t.Error("bash should be disabled with nil cfg")
	}
	if !IsCommandEnabledForConfig(nil, "help") {
		t.Error("help should be enabled with nil cfg")
	}

	// 显式启用
	trueVal := true
	cfg := &CommandsEnabledConfig{Config: &trueVal}
	if !IsCommandEnabledForConfig(cfg, "config") {
		t.Error("config should be enabled when set to true")
	}
	if IsCommandEnabledForConfig(cfg, "debug") {
		t.Error("debug should still be disabled")
	}
}

func TestListChatCommandsForConfig(t *testing.T) {
	// nil config → 过滤掉 config/debug/bash
	filtered := ListChatCommandsForConfig(nil)
	for _, cmd := range filtered {
		if cmd.Key == "config" || cmd.Key == "debug" || cmd.Key == "bash" {
			t.Errorf("command %q should be filtered out with nil config", cmd.Key)
		}
	}
	allCommands := ListChatCommands()
	if len(filtered) >= len(allCommands) {
		t.Errorf("filtered (%d) should be less than all (%d)", len(filtered), len(allCommands))
	}
}

func TestResolveCommandArgChoices(t *testing.T) {
	arg := &CommandArgDefinition{
		Name: "mode",
		Type: ArgTypeString,
		Choices: []CommandArgChoice{
			{Value: "on", Label: "On"},
			{Value: "off", Label: "Off"},
		},
	}
	choices := ResolveCommandArgChoices(arg)
	if len(choices) != 2 {
		t.Fatalf("got %d choices, want 2", len(choices))
	}
	if choices[0].Value != "on" || choices[0].Label != "On" {
		t.Errorf("first choice = %+v, want on/On", choices[0])
	}

	// nil arg
	if ResolveCommandArgChoices(nil) != nil {
		t.Error("nil arg should return nil")
	}
}

func TestGetTextAliasMap(t *testing.T) {
	InvalidateTextAliasCache()
	aliasMap := GetTextAliasMap()
	if len(aliasMap) == 0 {
		t.Fatal("alias map should not be empty")
	}
	// /help should map to help command
	spec, ok := aliasMap["/help"]
	if !ok {
		t.Fatal("'/help' should be in alias map")
	}
	if spec.Key != "help" {
		t.Errorf("Key = %q, want %q", spec.Key, "help")
	}

	// /t should map to think command
	spec, ok = aliasMap["/t"]
	if !ok {
		t.Fatal("'/t' should be in alias map")
	}
	if spec.Key != "think" {
		t.Errorf("Key = %q, want %q", spec.Key, "think")
	}
}

func TestMaybeResolveTextAlias(t *testing.T) {
	InvalidateTextAliasCache()

	tests := []struct {
		raw  string
		want string
	}{
		{"/help", "/help"},
		{"/status", "/status"},
		{"/t", "/t"},
		{"hello", ""},
		{"", ""},
		{"/nonexistent", ""},
	}
	for _, tt := range tests {
		got := MaybeResolveTextAlias(tt.raw)
		if got != tt.want {
			t.Errorf("MaybeResolveTextAlias(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestResolveNativeName(t *testing.T) {
	ttsCmd := FindCommand("tts")
	if ttsCmd == nil {
		t.Fatal("tts command not found")
	}
	if got := ResolveNativeName(ttsCmd, ""); got != "tts" {
		t.Errorf("no provider: got %q, want tts", got)
	}
	if got := ResolveNativeName(ttsCmd, "discord"); got != "voice" {
		t.Errorf("discord: got %q, want voice", got)
	}
	if got := ResolveNativeName(nil, ""); got != "" {
		t.Errorf("nil cmd: got %q, want empty", got)
	}
}

func TestBuildSkillCommandDefinitions(t *testing.T) {
	defs := BuildSkillCommandDefinitions([]SkillCommandSpec{
		{Name: "greet", Description: "Say hello"},
	})
	if len(defs) != 1 {
		t.Fatalf("got %d, want 1", len(defs))
	}
	if defs[0].Key != "skill:greet" {
		t.Errorf("Key=%q", defs[0].Key)
	}
	if BuildSkillCommandDefinitions(nil) != nil {
		t.Error("nil should return nil")
	}
}

func TestListNativeCommandSpecsForConfig(t *testing.T) {
	specs := ListNativeCommandSpecsForConfig(nil, "discord")
	found := false
	for _, s := range specs {
		if s.Name == "voice" {
			found = true
		}
	}
	if !found {
		t.Error("discord: tts should be overridden to voice")
	}
}

func TestResolveCommandArgMenu(t *testing.T) {
	cmd := FindCommand("verbose")
	if cmd == nil {
		t.Fatal("verbose not found")
	}
	menu := ResolveCommandArgMenu(cmd, nil)
	if menu == nil {
		t.Fatal("menu should not be nil")
	}
	if menu.Arg.Name != "mode" {
		t.Errorf("Arg.Name=%q", menu.Arg.Name)
	}
	// 已提供参数 → nil
	args := &CommandArgs{Values: CommandArgValues{"mode": "on"}}
	if ResolveCommandArgMenu(cmd, args) != nil {
		t.Error("should be nil when arg provided")
	}
}

func TestIsNativeCommandSurface(t *testing.T) {
	// 注入 DI provider（模拟 gateway 启动注入）
	origProvider := NativeCommandSurfaceProvider
	NativeCommandSurfaceProvider = func() []string {
		return []string{"discord", "telegram", "slack"}
	}
	defer func() { NativeCommandSurfaceProvider = origProvider }()

	if !IsNativeCommandSurface("discord") {
		t.Error("discord should be native")
	}
	if !IsNativeCommandSurface("Telegram") {
		t.Error("Telegram should be native")
	}
	if IsNativeCommandSurface("") {
		t.Error("empty should not be native")
	}
	if IsNativeCommandSurface("web") {
		t.Error("web should not be native")
	}
}

func TestIsNativeCommandSurface_NilProvider(t *testing.T) {
	origProvider := NativeCommandSurfaceProvider
	NativeCommandSurfaceProvider = nil
	defer func() { NativeCommandSurfaceProvider = origProvider }()

	if IsNativeCommandSurface("discord") {
		t.Error("nil provider should return false")
	}
}

func TestIsNativeCommandSurface_WithPlugin(t *testing.T) {
	origProvider := NativeCommandSurfaceProvider
	NativeCommandSurfaceProvider = func() []string {
		return []string{"discord", "telegram", "slack", "custom-native"}
	}
	defer func() { NativeCommandSurfaceProvider = origProvider }()

	if !IsNativeCommandSurface("custom-native") {
		t.Error("custom-native should be native with plugin")
	}
}
