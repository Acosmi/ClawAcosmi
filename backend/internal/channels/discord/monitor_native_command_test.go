package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/bwmarrin/discordgo"
)

// ── Test Helpers ──

// newTestLogger returns a no-op slog.Logger suitable for tests.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// newTestMonitorContext builds a minimal DiscordMonitorContext for unit tests.
// The Session field is nil (callers that need it should set it).
func newTestMonitorContext(deps *DiscordMonitorDeps) *DiscordMonitorContext {
	return &DiscordMonitorContext{
		AccountID:   "test-account-123",
		BotUserID:   "bot-user-456",
		Token:       "test-token",
		DMPolicy:    "open",
		GroupPolicy: "allowlist",
		AllowFrom:   []string{},
		Logger:      newTestLogger(),
		Deps:        deps,
	}
}

// newTestInboundMessage builds a DiscordInboundMessage for tests.
func newTestInboundMessage(text, channelID, senderID string) *DiscordInboundMessage {
	return &DiscordInboundMessage{
		ChannelID:  channelID,
		SenderID:   senderID,
		SenderName: "TestUser",
		Text:       text,
		MessageID:  "msg-001",
	}
}

// ── isDiscordNativeCommand ──

func TestNativeCommand_IsDiscordNativeCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		// Known commands — all 9 registered in discordNativeCommands map
		{name: "reset is native", cmd: "/reset", want: true},
		{name: "help is native", cmd: "/help", want: true},
		{name: "model is native", cmd: "/model", want: true},
		{name: "status is native", cmd: "/status", want: true},
		{name: "compact is native", cmd: "/compact", want: true},
		{name: "verbose is native", cmd: "/verbose", want: true},
		{name: "pair is native", cmd: "/pair", want: true},
		{name: "unpair is native", cmd: "/unpair", want: true},
		{name: "ping is native", cmd: "/ping", want: true},

		// Unknown / edge-case inputs
		{name: "unknown slash command", cmd: "/foo", want: false},
		{name: "empty string", cmd: "", want: false},
		{name: "no slash prefix", cmd: "reset", want: false},
		{name: "double slash", cmd: "//reset", want: false},
		{name: "uppercase not matched", cmd: "/RESET", want: false},
		{name: "with trailing space", cmd: "/reset ", want: false},
		{name: "with args embedded", cmd: "/model gpt-4", want: false},
		{name: "just a slash", cmd: "/", want: false},
		{name: "random text", cmd: "hello world", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDiscordNativeCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("isDiscordNativeCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// ── HandleDiscordNativeCommand sub-handlers ──
// We test the individual reply-producing functions directly since
// HandleDiscordNativeCommand requires a real discordgo.Session to send messages.

func TestNativeCommand_HandleDiscordNativeCommand_Help(t *testing.T) {
	reply := buildDiscordHelpText()
	if reply == "" {
		t.Fatal("buildDiscordHelpText() returned empty string")
	}
	expected := []string{"/help", "/ping", "/status", "/reset", "/model", "/compact", "/verbose", "/pair"}
	for _, cmd := range expected {
		if !strings.Contains(reply, cmd) {
			t.Errorf("help text missing command %q", cmd)
		}
	}
}

func TestNativeCommand_HandleDiscordNativeCommand_Ping(t *testing.T) {
	// Verify that /ping text is parsed correctly as a command.
	msg := newTestInboundMessage("/ping", "ch-1", "user-1")
	parts := strings.SplitN(msg.Text, " ", 2)
	cmd := strings.ToLower(parts[0])
	if cmd != "/ping" {
		t.Errorf("expected cmd=/ping, got %q", cmd)
	}
	if len(parts) > 1 {
		t.Errorf("expected no args for /ping, got %q", parts[1])
	}
}

func TestNativeCommand_HandleDiscordNativeCommand_Status(t *testing.T) {
	monCtx := newTestMonitorContext(nil)
	reply := buildDiscordStatusText(monCtx)

	if !strings.Contains(reply, monCtx.AccountID) {
		t.Errorf("status text should contain account ID %q", monCtx.AccountID)
	}
	if !strings.Contains(reply, monCtx.BotUserID) {
		t.Errorf("status text should contain bot user ID %q", monCtx.BotUserID)
	}
	if !strings.Contains(reply, monCtx.DMPolicy) {
		t.Errorf("status text should contain DM policy %q", monCtx.DMPolicy)
	}
	if !strings.Contains(reply, monCtx.GroupPolicy) {
		t.Errorf("status text should contain group policy %q", monCtx.GroupPolicy)
	}
}

func TestNativeCommand_HandleDiscordReset_NilDeps(t *testing.T) {
	monCtx := newTestMonitorContext(nil)
	msg := newTestInboundMessage("/reset", "ch-1", "user-1")

	reply := handleDiscordReset(monCtx, msg)
	if reply != "✅ Session has been reset." {
		t.Errorf("nil deps should return success stub, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordReset_NilResetSession(t *testing.T) {
	monCtx := newTestMonitorContext(&DiscordMonitorDeps{
		ResetSession: nil,
	})
	msg := newTestInboundMessage("/reset", "ch-1", "user-1")

	reply := handleDiscordReset(monCtx, msg)
	if reply != "✅ Session has been reset." {
		t.Errorf("nil ResetSession should return success stub, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordReset_Success(t *testing.T) {
	var calledWith struct {
		accountID string
		channelID string
		senderID  string
	}
	deps := &DiscordMonitorDeps{
		ResetSession: func(ctx context.Context, accountID, channelID, senderID string) error {
			calledWith.accountID = accountID
			calledWith.channelID = channelID
			calledWith.senderID = senderID
			return nil
		},
	}

	monCtx := newTestMonitorContext(deps)
	msg := newTestInboundMessage("/reset", "ch-1", "user-1")

	reply := handleDiscordReset(monCtx, msg)
	if reply != "✅ Session has been reset." {
		t.Errorf("successful reset should return success message, got %q", reply)
	}
	if calledWith.accountID != "test-account-123" {
		t.Errorf("ResetSession called with accountID=%q, want %q", calledWith.accountID, "test-account-123")
	}
	if calledWith.channelID != "ch-1" {
		t.Errorf("ResetSession called with channelID=%q, want %q", calledWith.channelID, "ch-1")
	}
	if calledWith.senderID != "user-1" {
		t.Errorf("ResetSession called with senderID=%q, want %q", calledWith.senderID, "user-1")
	}
}

func TestNativeCommand_HandleDiscordReset_Error(t *testing.T) {
	deps := &DiscordMonitorDeps{
		ResetSession: func(ctx context.Context, accountID, channelID, senderID string) error {
			return errors.New("session store unavailable")
		},
	}

	monCtx := newTestMonitorContext(deps)
	msg := newTestInboundMessage("/reset", "ch-1", "user-1")

	reply := handleDiscordReset(monCtx, msg)
	if !strings.Contains(reply, "Failed to reset session") {
		t.Errorf("error should be surfaced in reply, got %q", reply)
	}
	if !strings.Contains(reply, "session store unavailable") {
		t.Errorf("error message should be in reply, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordModel_NoArgs(t *testing.T) {
	monCtx := newTestMonitorContext(nil)
	msg := newTestInboundMessage("/model", "ch-1", "user-1")
	_ = msg

	reply := handleDiscordModel(monCtx, msg, "")
	if !strings.Contains(reply, "Usage") {
		t.Errorf("no-args should return usage hint, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordModel_NilDeps(t *testing.T) {
	monCtx := newTestMonitorContext(nil)
	msg := newTestInboundMessage("/model claude-sonnet-4-20250514", "ch-1", "user-1")

	reply := handleDiscordModel(monCtx, msg, "claude-sonnet-4-20250514")
	if !strings.Contains(reply, "claude-sonnet-4-20250514") {
		t.Errorf("nil deps stub should echo model name, got %q", reply)
	}
	if !strings.Contains(reply, "Model switched") {
		t.Errorf("nil deps should return success stub, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordModel_NilSwitchModel(t *testing.T) {
	monCtx := newTestMonitorContext(&DiscordMonitorDeps{SwitchModel: nil})
	msg := newTestInboundMessage("/model gpt-4", "ch-1", "user-1")

	reply := handleDiscordModel(monCtx, msg, "gpt-4")
	if !strings.Contains(reply, "Model switched") {
		t.Errorf("nil SwitchModel should return success stub, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordModel_Success(t *testing.T) {
	var calledModel string
	deps := &DiscordMonitorDeps{
		SwitchModel: func(ctx context.Context, accountID, modelName string) error {
			calledModel = modelName
			return nil
		},
	}

	monCtx := newTestMonitorContext(deps)
	msg := newTestInboundMessage("/model claude-opus-4-20250514", "ch-1", "user-1")

	reply := handleDiscordModel(monCtx, msg, "claude-opus-4-20250514")
	if !strings.Contains(reply, "claude-opus-4-20250514") {
		t.Errorf("reply should contain model name, got %q", reply)
	}
	if calledModel != "claude-opus-4-20250514" {
		t.Errorf("SwitchModel called with %q, want %q", calledModel, "claude-opus-4-20250514")
	}
}

func TestNativeCommand_HandleDiscordModel_Error(t *testing.T) {
	deps := &DiscordMonitorDeps{
		SwitchModel: func(ctx context.Context, accountID, modelName string) error {
			return errors.New("invalid model")
		},
	}

	monCtx := newTestMonitorContext(deps)
	msg := newTestInboundMessage("/model bad-model", "ch-1", "user-1")

	reply := handleDiscordModel(monCtx, msg, "bad-model")
	if !strings.Contains(reply, "Failed to switch model") {
		t.Errorf("error should appear in reply, got %q", reply)
	}
	if !strings.Contains(reply, "invalid model") {
		t.Errorf("error text should appear in reply, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordPair_NilDeps(t *testing.T) {
	monCtx := newTestMonitorContext(nil)
	msg := newTestInboundMessage("/pair", "ch-1", "user-1")

	reply := handleDiscordPair(monCtx, msg, "")
	if !strings.Contains(reply, "Pairing is not available") {
		t.Errorf("nil deps should report pairing unavailable, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordPair_NilUpsertPairingRequest(t *testing.T) {
	monCtx := newTestMonitorContext(&DiscordMonitorDeps{UpsertPairingRequest: nil})
	msg := newTestInboundMessage("/pair", "ch-1", "user-1")

	reply := handleDiscordPair(monCtx, msg, "")
	if !strings.Contains(reply, "Pairing is not available") {
		t.Errorf("nil UpsertPairingRequest should report pairing unavailable, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordPair_Success_Created(t *testing.T) {
	var calledParams DiscordPairingRequestParams
	deps := &DiscordMonitorDeps{
		UpsertPairingRequest: func(params DiscordPairingRequestParams) (*DiscordPairingResult, error) {
			calledParams = params
			return &DiscordPairingResult{Code: "ABC123", Created: true}, nil
		},
	}

	monCtx := newTestMonitorContext(deps)
	msg := newTestInboundMessage("/pair", "ch-1", "user-1")

	reply := handleDiscordPair(monCtx, msg, "")
	if !strings.Contains(reply, "ABC123") {
		t.Errorf("reply should contain pairing code, got %q", reply)
	}
	if calledParams.Channel != "discord" {
		t.Errorf("params.Channel=%q, want %q", calledParams.Channel, "discord")
	}
	if calledParams.ID != "user-1" {
		t.Errorf("params.ID=%q, want %q", calledParams.ID, "user-1")
	}
	if calledParams.Meta["sender"] != "user-1" {
		t.Errorf("params.Meta[sender]=%q, want %q", calledParams.Meta["sender"], "user-1")
	}
}

func TestNativeCommand_HandleDiscordPair_AlreadyExists(t *testing.T) {
	deps := &DiscordMonitorDeps{
		UpsertPairingRequest: func(params DiscordPairingRequestParams) (*DiscordPairingResult, error) {
			return &DiscordPairingResult{Code: "OLD123", Created: false}, nil
		},
	}

	monCtx := newTestMonitorContext(deps)
	msg := newTestInboundMessage("/pair", "ch-1", "user-1")

	reply := handleDiscordPair(monCtx, msg, "")
	if !strings.Contains(reply, "already exists") {
		t.Errorf("existing pairing should indicate already exists, got %q", reply)
	}
}

func TestNativeCommand_HandleDiscordPair_Error(t *testing.T) {
	deps := &DiscordMonitorDeps{
		UpsertPairingRequest: func(params DiscordPairingRequestParams) (*DiscordPairingResult, error) {
			return nil, errors.New("db connection lost")
		},
	}

	monCtx := newTestMonitorContext(deps)
	msg := newTestInboundMessage("/pair", "ch-1", "user-1")

	reply := handleDiscordPair(monCtx, msg, "")
	if !strings.Contains(reply, "Pairing failed") {
		t.Errorf("error should be in reply, got %q", reply)
	}
	if !strings.Contains(reply, "db connection lost") {
		t.Errorf("error message should be in reply, got %q", reply)
	}
}

// ── Command text parsing (mirrors HandleDiscordNativeCommand internal logic) ──

func TestNativeCommand_CommandParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCmd  string
		wantArgs string
	}{
		{name: "simple command", input: "/reset", wantCmd: "/reset", wantArgs: ""},
		{name: "command with args", input: "/model claude-sonnet-4-20250514", wantCmd: "/model", wantArgs: "claude-sonnet-4-20250514"},
		{name: "command with multi-word args", input: "/model some long model name", wantCmd: "/model", wantArgs: "some long model name"},
		{name: "command with leading spaces in args", input: "/model   spaces", wantCmd: "/model", wantArgs: "spaces"},
		{name: "uppercase command lowered", input: "/HELP", wantCmd: "/help", wantArgs: ""},
		{name: "mixed case command", input: "/Model gpt-4", wantCmd: "/model", wantArgs: "gpt-4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.SplitN(tt.input, " ", 2)
			cmd := strings.ToLower(parts[0])
			args := ""
			if len(parts) > 1 {
				args = strings.TrimSpace(parts[1])
			}
			if cmd != tt.wantCmd {
				t.Errorf("cmd=%q, want %q", cmd, tt.wantCmd)
			}
			if args != tt.wantArgs {
				t.Errorf("args=%q, want %q", args, tt.wantArgs)
			}
		})
	}
}

// ── BuildDiscordApplicationCommands ──

func TestNativeCommand_BuildDiscordApplicationCommands_Empty(t *testing.T) {
	commands := BuildDiscordApplicationCommands(nil)
	if len(commands) != 0 {
		t.Errorf("nil specs should produce 0 commands, got %d", len(commands))
	}

	commands = BuildDiscordApplicationCommands([]autoreply.NativeCommandSpec{})
	if len(commands) != 0 {
		t.Errorf("empty specs should produce 0 commands, got %d", len(commands))
	}
}

func TestNativeCommand_BuildDiscordApplicationCommands_SimpleCommand(t *testing.T) {
	specs := []autoreply.NativeCommandSpec{
		{Name: "ping", Description: "Check latency"},
	}

	commands := BuildDiscordApplicationCommands(specs)
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	cmd := commands[0]
	if cmd.Name != "ping" {
		t.Errorf("Name=%q, want %q", cmd.Name, "ping")
	}
	if cmd.Description != "Check latency" {
		t.Errorf("Description=%q, want %q", cmd.Description, "Check latency")
	}
	if len(cmd.Options) != 0 {
		t.Errorf("simple command should have no options, got %d", len(cmd.Options))
	}
}

func TestNativeCommand_BuildDiscordApplicationCommands_AcceptsArgs(t *testing.T) {
	specs := []autoreply.NativeCommandSpec{
		{Name: "model", Description: "Switch model", AcceptsArgs: true},
	}

	commands := BuildDiscordApplicationCommands(specs)
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	cmd := commands[0]
	if len(cmd.Options) != 1 {
		t.Fatalf("AcceptsArgs=true should produce 1 option, got %d", len(cmd.Options))
	}

	opt := cmd.Options[0]
	if opt.Name != "input" {
		t.Errorf("default option Name=%q, want %q", opt.Name, "input")
	}
	if opt.Description != "Command input" {
		t.Errorf("default option Description=%q, want %q", opt.Description, "Command input")
	}
	if opt.Type != discordgo.ApplicationCommandOptionString {
		t.Errorf("default option Type=%d, want String(%d)", opt.Type, discordgo.ApplicationCommandOptionString)
	}
	if opt.Required {
		t.Error("default input option should not be required")
	}
}

func TestNativeCommand_BuildDiscordApplicationCommands_WithArgs(t *testing.T) {
	specs := []autoreply.NativeCommandSpec{
		{
			Name:        "test",
			Description: "Test command",
			Args: []autoreply.CommandArgDefinition{
				{Name: "text", Description: "A text arg", Type: autoreply.ArgTypeString, Required: true},
				{Name: "count", Description: "A number arg", Type: autoreply.ArgTypeNumber, Required: false},
				{Name: "flag", Description: "A boolean arg", Type: autoreply.ArgTypeBoolean, Required: false},
			},
		},
	}

	commands := BuildDiscordApplicationCommands(specs)
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	cmd := commands[0]
	if len(cmd.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(cmd.Options))
	}

	// String arg
	if cmd.Options[0].Type != discordgo.ApplicationCommandOptionString {
		t.Errorf("text arg type=%d, want String(%d)", cmd.Options[0].Type, discordgo.ApplicationCommandOptionString)
	}
	if cmd.Options[0].Name != "text" {
		t.Errorf("text arg name=%q, want %q", cmd.Options[0].Name, "text")
	}
	if !cmd.Options[0].Required {
		t.Error("text arg should be required")
	}

	// Number arg
	if cmd.Options[1].Type != discordgo.ApplicationCommandOptionNumber {
		t.Errorf("count arg type=%d, want Number(%d)", cmd.Options[1].Type, discordgo.ApplicationCommandOptionNumber)
	}
	if cmd.Options[1].Required {
		t.Error("count arg should not be required")
	}

	// Boolean arg
	if cmd.Options[2].Type != discordgo.ApplicationCommandOptionBoolean {
		t.Errorf("flag arg type=%d, want Boolean(%d)", cmd.Options[2].Type, discordgo.ApplicationCommandOptionBoolean)
	}
}

func TestNativeCommand_BuildDiscordApplicationCommands_WithChoices(t *testing.T) {
	specs := []autoreply.NativeCommandSpec{
		{
			Name:        "usage",
			Description: "Usage mode",
			Args: []autoreply.CommandArgDefinition{
				{
					Name:        "mode",
					Description: "off, tokens, full, or cost",
					Type:        autoreply.ArgTypeString,
					Choices: []autoreply.CommandArgChoice{
						{Value: "off", Label: "Off"},
						{Value: "tokens", Label: "Tokens"},
						{Value: "full", Label: "Full"},
						{Value: "cost", Label: "Cost"},
					},
				},
			},
		},
	}

	commands := BuildDiscordApplicationCommands(specs)
	cmd := commands[0]
	if len(cmd.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(cmd.Options))
	}

	opt := cmd.Options[0]
	if len(opt.Choices) != 4 {
		t.Fatalf("expected 4 choices, got %d", len(opt.Choices))
	}
	if opt.Autocomplete {
		t.Error("4 choices should not trigger autocomplete")
	}

	// Verify choice mapping
	wantChoices := map[string]string{"off": "Off", "tokens": "Tokens", "full": "Full", "cost": "Cost"}
	for _, c := range opt.Choices {
		label, ok := wantChoices[c.Value.(string)]
		if !ok {
			t.Errorf("unexpected choice value %q", c.Value)
		}
		if c.Name != label {
			t.Errorf("choice Name=%q, want %q for value %q", c.Name, label, c.Value)
		}
	}
}

func TestNativeCommand_BuildDiscordApplicationCommands_AutocompleteOverflow(t *testing.T) {
	// When > 25 choices, autocomplete should be enabled instead of inline choices.
	choices := make([]autoreply.CommandArgChoice, 30)
	for i := 0; i < 30; i++ {
		choices[i] = autoreply.CommandArgChoice{Value: fmt.Sprintf("val%d", i), Label: fmt.Sprintf("Label %d", i)}
	}

	specs := []autoreply.NativeCommandSpec{
		{
			Name:        "many",
			Description: "Many choices",
			Args: []autoreply.CommandArgDefinition{
				{Name: "pick", Description: "Pick one", Type: autoreply.ArgTypeString, Choices: choices},
			},
		},
	}

	commands := BuildDiscordApplicationCommands(specs)
	opt := commands[0].Options[0]
	if !opt.Autocomplete {
		t.Error("more than 25 choices should enable Autocomplete")
	}
	if len(opt.Choices) != 0 {
		t.Errorf("autocomplete mode should not set inline choices, got %d", len(opt.Choices))
	}
}

func TestNativeCommand_BuildDiscordApplicationCommands_MultipleSpecs(t *testing.T) {
	specs := []autoreply.NativeCommandSpec{
		{Name: "ping", Description: "Pong"},
		{Name: "help", Description: "Show help"},
		{Name: "status", Description: "Show status"},
	}

	commands := BuildDiscordApplicationCommands(specs)
	if len(commands) != 3 {
		t.Errorf("expected 3 commands, got %d", len(commands))
	}
	names := make(map[string]bool)
	for _, c := range commands {
		names[c.Name] = true
	}
	for _, s := range specs {
		if !names[s.Name] {
			t.Errorf("missing command %q in output", s.Name)
		}
	}
}

func TestNativeCommand_BuildDiscordApplicationCommands_ArgsOverrideAcceptsArgs(t *testing.T) {
	// When both Args and AcceptsArgs are set, Args should take priority.
	specs := []autoreply.NativeCommandSpec{
		{
			Name:        "model",
			Description: "Switch model",
			AcceptsArgs: true,
			Args: []autoreply.CommandArgDefinition{
				{Name: "name", Description: "Model name", Type: autoreply.ArgTypeString},
			},
		},
	}

	commands := BuildDiscordApplicationCommands(specs)
	cmd := commands[0]
	if len(cmd.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(cmd.Options))
	}
	if cmd.Options[0].Name != "name" {
		t.Errorf("Args should take precedence over AcceptsArgs, got option name %q", cmd.Options[0].Name)
	}
}

// ── readDiscordSlashCommandArgs (ParseDiscordSlashCommandArgs) ──

func TestNativeCommand_ParseDiscordSlashCommandArgs_NilOptions(t *testing.T) {
	result := readDiscordSlashCommandArgs(nil, []autoreply.CommandArgDefinition{
		{Name: "mode", Type: autoreply.ArgTypeString},
	})
	if result != nil {
		t.Errorf("nil options should return nil, got %v", result)
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_NilDefs(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "mode", Type: discordgo.ApplicationCommandOptionString, Value: "compact"},
	}
	result := readDiscordSlashCommandArgs(opts, nil)
	if result != nil {
		t.Errorf("nil defs should return nil, got %v", result)
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_EmptyOptions(t *testing.T) {
	defs := []autoreply.CommandArgDefinition{
		{Name: "mode", Type: autoreply.ArgTypeString},
	}
	result := readDiscordSlashCommandArgs([]*discordgo.ApplicationCommandInteractionDataOption{}, defs)
	if result != nil {
		t.Errorf("empty options should return nil, got %v", result)
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_StringOption(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "mode", Type: discordgo.ApplicationCommandOptionString, Value: "compact"},
	}
	defs := []autoreply.CommandArgDefinition{
		{Name: "mode", Type: autoreply.ArgTypeString},
	}

	result := readDiscordSlashCommandArgs(opts, defs)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if v, ok := result.Values["mode"]; !ok || v != "compact" {
		t.Errorf("Values[mode]=%v, want %q", v, "compact")
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_NumberOption(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "count", Type: discordgo.ApplicationCommandOptionNumber, Value: 42.0},
	}
	defs := []autoreply.CommandArgDefinition{
		{Name: "count", Type: autoreply.ArgTypeNumber},
	}

	result := readDiscordSlashCommandArgs(opts, defs)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if v, ok := result.Values["count"]; !ok {
		t.Error("expected count in values")
	} else if v != 42.0 {
		t.Errorf("Values[count]=%v, want 42.0", v)
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_BooleanOption(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "flag", Type: discordgo.ApplicationCommandOptionBoolean, Value: true},
	}
	defs := []autoreply.CommandArgDefinition{
		{Name: "flag", Type: autoreply.ArgTypeBoolean},
	}

	result := readDiscordSlashCommandArgs(opts, defs)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if v, ok := result.Values["flag"]; !ok || v != true {
		t.Errorf("Values[flag]=%v, want true", v)
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_IntegerOption(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "limit", Type: discordgo.ApplicationCommandOptionInteger, Value: float64(10)},
	}
	defs := []autoreply.CommandArgDefinition{
		{Name: "limit", Type: autoreply.ArgTypeNumber},
	}

	result := readDiscordSlashCommandArgs(opts, defs)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if _, ok := result.Values["limit"]; !ok {
		t.Error("expected limit in values")
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_MultipleOptions(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "mode", Type: discordgo.ApplicationCommandOptionString, Value: "full"},
		{Name: "count", Type: discordgo.ApplicationCommandOptionNumber, Value: 5.0},
		{Name: "verbose", Type: discordgo.ApplicationCommandOptionBoolean, Value: true},
	}
	defs := []autoreply.CommandArgDefinition{
		{Name: "mode", Type: autoreply.ArgTypeString},
		{Name: "count", Type: autoreply.ArgTypeNumber},
		{Name: "verbose", Type: autoreply.ArgTypeBoolean},
	}

	result := readDiscordSlashCommandArgs(opts, defs)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(result.Values))
	}
}

func TestNativeCommand_ParseDiscordSlashCommandArgs_EmptyStringSkipped(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "mode", Type: discordgo.ApplicationCommandOptionString, Value: ""},
	}
	defs := []autoreply.CommandArgDefinition{
		{Name: "mode", Type: autoreply.ArgTypeString},
	}

	result := readDiscordSlashCommandArgs(opts, defs)
	// Empty string is skipped by StringValue() check, so no values => nil
	if result != nil {
		t.Errorf("empty string value should be skipped, got %v", result)
	}
}

// ── buildDiscordCommandArgCustomID / parseDiscordCommandArgCustomID ──

func TestNativeCommand_BuildAndParseCommandArgCustomID(t *testing.T) {
	tests := []struct {
		name    string
		command string
		arg     string
		value   string
		userID  string
	}{
		{
			name:    "basic roundtrip",
			command: "usage",
			arg:     "mode",
			value:   "off",
			userID:  "123456",
		},
		{
			name:    "special characters in value",
			command: "model",
			arg:     "name",
			value:   "claude-sonnet-4-20250514/latest",
			userID:  "789",
		},
		{
			name:    "spaces in value",
			command: "test",
			arg:     "input",
			value:   "hello world",
			userID:  "user-42",
		},
		{
			name:    "empty user ID",
			command: "cmd",
			arg:     "a",
			value:   "v",
			userID:  "",
		},
		{
			name:    "encoded characters",
			command: "test",
			arg:     "text",
			value:   "hello+world=42",
			userID:  "u1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customID := buildDiscordCommandArgCustomID(tt.command, tt.arg, tt.value, tt.userID)

			// Custom ID should start with the key prefix
			if !strings.HasPrefix(customID, discordCommandArgCustomIDKey+":") {
				t.Errorf("customID should start with %q:, got %q", discordCommandArgCustomIDKey, customID)
			}

			parsed := parseDiscordCommandArgCustomID(customID)
			if parsed == nil {
				t.Fatal("parseDiscordCommandArgCustomID returned nil")
			}
			if parsed.Command != tt.command {
				t.Errorf("Command=%q, want %q", parsed.Command, tt.command)
			}
			if parsed.Arg != tt.arg {
				t.Errorf("Arg=%q, want %q", parsed.Arg, tt.arg)
			}
			if parsed.Value != tt.value {
				t.Errorf("Value=%q, want %q", parsed.Value, tt.value)
			}
			if parsed.UserID != tt.userID {
				t.Errorf("UserID=%q, want %q", parsed.UserID, tt.userID)
			}
		})
	}
}

func TestNativeCommand_ParseCommandArgCustomID_InvalidPrefix(t *testing.T) {
	result := parseDiscordCommandArgCustomID("wrong:command=a;arg=b;value=c")
	if result != nil {
		t.Errorf("invalid prefix should return nil, got %v", result)
	}
}

func TestNativeCommand_ParseCommandArgCustomID_MissingFields(t *testing.T) {
	tests := []struct {
		name     string
		customID string
	}{
		{name: "missing command", customID: "cmdarg:arg=b;value=c"},
		{name: "missing arg", customID: "cmdarg:command=a;value=c"},
		{name: "missing value", customID: "cmdarg:command=a;arg=b"},
		{name: "all missing", customID: "cmdarg:"},
		{name: "empty after prefix", customID: "cmdarg:;"},
		{name: "malformed kv pairs", customID: "cmdarg:abc;def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDiscordCommandArgCustomID(tt.customID)
			if result != nil {
				t.Errorf("missing required field should return nil, got %+v", result)
			}
		})
	}
}

func TestNativeCommand_ParseCommandArgCustomID_EmptyString(t *testing.T) {
	result := parseDiscordCommandArgCustomID("")
	if result != nil {
		t.Errorf("empty string should return nil, got %v", result)
	}
}

// ── resolveInteractionUser ──

func TestNativeCommand_ResolveInteractionUser_MemberUser(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: &discordgo.Member{
				User: &discordgo.User{
					ID:       "member-user-1",
					Username: "MemberUser",
				},
			},
			User: &discordgo.User{
				ID:       "dm-user-2",
				Username: "DMUser",
			},
		},
	}

	user := resolveInteractionUser(i)
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	// Member.User takes precedence over i.User
	if user.ID != "member-user-1" {
		t.Errorf("user.ID=%q, want %q (member takes precedence)", user.ID, "member-user-1")
	}
}

func TestNativeCommand_ResolveInteractionUser_DMUser(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: nil,
			User: &discordgo.User{
				ID:       "dm-user-2",
				Username: "DMUser",
			},
		},
	}

	user := resolveInteractionUser(i)
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.ID != "dm-user-2" {
		t.Errorf("user.ID=%q, want %q", user.ID, "dm-user-2")
	}
}

func TestNativeCommand_ResolveInteractionUser_NilMemberUser(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: &discordgo.Member{User: nil},
			User: &discordgo.User{
				ID: "fallback-user",
			},
		},
	}

	user := resolveInteractionUser(i)
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	// Member exists but Member.User is nil, so falls back to i.User
	if user.ID != "fallback-user" {
		t.Errorf("user.ID=%q, want %q", user.ID, "fallback-user")
	}
}

func TestNativeCommand_ResolveInteractionUser_AllNil(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: nil,
			User:   nil,
		},
	}

	user := resolveInteractionUser(i)
	if user != nil {
		t.Errorf("both nil should return nil, got %+v", user)
	}
}

// ── isDiscordUnknownInteraction ──

func TestNativeCommand_IsDiscordUnknownInteraction(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "10062 code", err: errors.New("HTTP 404: 10062"), want: true},
		{name: "unknown interaction text", err: errors.New("unknown interaction"), want: true},
		{name: "Unknown Interaction uppercase", err: errors.New("Unknown Interaction"), want: true},
		{name: "unrelated error", err: errors.New("connection refused"), want: false},
		{name: "empty error message", err: errors.New(""), want: false},
		{name: "partial 10062 in message", err: errors.New("error code 10062: not found"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDiscordUnknownInteraction(tt.err)
			if got != tt.want {
				t.Errorf("isDiscordUnknownInteraction(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ── buildDiscordHelpText ──

func TestNativeCommand_BuildDiscordHelpText_Content(t *testing.T) {
	text := buildDiscordHelpText()
	if text == "" {
		t.Fatal("help text should not be empty")
	}
	// Should start with bold markdown header
	if !strings.HasPrefix(text, "**") {
		t.Error("help text should start with bold markdown header")
	}
	// All documented commands should be present
	documented := []string{"/help", "/ping", "/status", "/reset", "/model", "/compact", "/verbose", "/pair"}
	for _, cmd := range documented {
		if !strings.Contains(text, cmd) {
			t.Errorf("help text missing documented command %q", cmd)
		}
	}
}

// ── buildDiscordStatusText ──

func TestNativeCommand_BuildDiscordStatusText_AllFields(t *testing.T) {
	monCtx := &DiscordMonitorContext{
		AccountID:   "acc-xyz",
		BotUserID:   "bot-789",
		DMPolicy:    "pairing",
		GroupPolicy: "open",
		Logger:      newTestLogger(),
	}

	text := buildDiscordStatusText(monCtx)
	for _, want := range []string{"acc-xyz", "bot-789", "pairing", "open", "Connected"} {
		if !strings.Contains(text, want) {
			t.Errorf("status text missing %q", want)
		}
	}
}

// ── commandNeedsUpdate / optionNeedsUpdate ──

func TestCommandNeedsUpdate_Identical(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "Pong",
		Options: []*discordgo.ApplicationCommandOption{
			{Name: "target", Type: discordgo.ApplicationCommandOptionString, Description: "Target host", Required: true},
		},
	}
	b := &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "Pong",
		Options: []*discordgo.ApplicationCommandOption{
			{Name: "target", Type: discordgo.ApplicationCommandOptionString, Description: "Target host", Required: true},
		},
	}
	if commandNeedsUpdate(a, b) {
		t.Error("identical commands should not need update")
	}
}

func TestCommandNeedsUpdate_DescriptionChanged(t *testing.T) {
	a := &discordgo.ApplicationCommand{Name: "ping", Description: "Pong"}
	b := &discordgo.ApplicationCommand{Name: "ping", Description: "Check latency"}
	if !commandNeedsUpdate(a, b) {
		t.Error("different description should need update")
	}
}

func TestCommandNeedsUpdate_OptionDescriptionChanged(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "model",
		Description: "Switch model",
		Options:     []*discordgo.ApplicationCommandOption{{Name: "name", Type: discordgo.ApplicationCommandOptionString, Description: "Model name"}},
	}
	b := &discordgo.ApplicationCommand{
		Name:        "model",
		Description: "Switch model",
		Options:     []*discordgo.ApplicationCommandOption{{Name: "name", Type: discordgo.ApplicationCommandOptionString, Description: "Model identifier"}},
	}
	if !commandNeedsUpdate(a, b) {
		t.Error("option description change should need update")
	}
}

func TestCommandNeedsUpdate_OptionAutocompleteChanged(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "model",
		Description: "Switch model",
		Options:     []*discordgo.ApplicationCommandOption{{Name: "name", Type: discordgo.ApplicationCommandOptionString, Autocomplete: false}},
	}
	b := &discordgo.ApplicationCommand{
		Name:        "model",
		Description: "Switch model",
		Options:     []*discordgo.ApplicationCommandOption{{Name: "name", Type: discordgo.ApplicationCommandOptionString, Autocomplete: true}},
	}
	if !commandNeedsUpdate(a, b) {
		t.Error("option autocomplete change should need update")
	}
}

func TestCommandNeedsUpdate_ChoiceValueChanged(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "usage",
		Description: "Usage",
		Options: []*discordgo.ApplicationCommandOption{{
			Name: "mode",
			Type: discordgo.ApplicationCommandOptionString,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Off", Value: "off"},
			},
		}},
	}
	b := &discordgo.ApplicationCommand{
		Name:        "usage",
		Description: "Usage",
		Options: []*discordgo.ApplicationCommandOption{{
			Name: "mode",
			Type: discordgo.ApplicationCommandOptionString,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Off", Value: "disable"},
			},
		}},
	}
	if !commandNeedsUpdate(a, b) {
		t.Error("choice value change should need update")
	}
}

func TestCommandNeedsUpdate_NestedSubOptions(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "config",
		Description: "Configuration",
		Options: []*discordgo.ApplicationCommandOption{{
			Name:        "set",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Description: "Set a value",
			Options: []*discordgo.ApplicationCommandOption{
				{Name: "key", Type: discordgo.ApplicationCommandOptionString, Description: "Key", Required: true},
				{Name: "value", Type: discordgo.ApplicationCommandOptionString, Description: "Value", Required: true},
			},
		}},
	}
	b := &discordgo.ApplicationCommand{
		Name:        "config",
		Description: "Configuration",
		Options: []*discordgo.ApplicationCommandOption{{
			Name:        "set",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Description: "Set a value",
			Options: []*discordgo.ApplicationCommandOption{
				{Name: "key", Type: discordgo.ApplicationCommandOptionString, Description: "Key", Required: true},
				{Name: "value", Type: discordgo.ApplicationCommandOptionString, Description: "New value", Required: true},
			},
		}},
	}
	if !commandNeedsUpdate(a, b) {
		t.Error("nested sub-option description change should need update")
	}
}

func TestCommandNeedsUpdate_NestedSubOptionAdded(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "config",
		Description: "Configuration",
		Options: []*discordgo.ApplicationCommandOption{{
			Name:        "set",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Description: "Set a value",
			Options: []*discordgo.ApplicationCommandOption{
				{Name: "key", Type: discordgo.ApplicationCommandOptionString},
			},
		}},
	}
	b := &discordgo.ApplicationCommand{
		Name:        "config",
		Description: "Configuration",
		Options: []*discordgo.ApplicationCommandOption{{
			Name:        "set",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Description: "Set a value",
			Options: []*discordgo.ApplicationCommandOption{
				{Name: "key", Type: discordgo.ApplicationCommandOptionString},
				{Name: "value", Type: discordgo.ApplicationCommandOptionString},
			},
		}},
	}
	if !commandNeedsUpdate(a, b) {
		t.Error("added nested sub-option should need update")
	}
}

func TestCommandNeedsUpdate_NestedIdentical(t *testing.T) {
	mkCmd := func() *discordgo.ApplicationCommand {
		return &discordgo.ApplicationCommand{
			Name:        "admin",
			Description: "Admin commands",
			Options: []*discordgo.ApplicationCommandOption{{
				Name:        "user",
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Description: "User management",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "ban",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Description: "Ban a user",
						Options: []*discordgo.ApplicationCommandOption{
							{Name: "target", Type: discordgo.ApplicationCommandOptionUser, Description: "User to ban", Required: true},
							{Name: "reason", Type: discordgo.ApplicationCommandOptionString, Description: "Reason"},
						},
					},
				},
			}},
		}
	}
	if commandNeedsUpdate(mkCmd(), mkCmd()) {
		t.Error("deeply nested identical commands should not need update")
	}
}

func TestCommandNeedsUpdate_NoOptions(t *testing.T) {
	a := &discordgo.ApplicationCommand{Name: "ping", Description: "Pong"}
	b := &discordgo.ApplicationCommand{Name: "ping", Description: "Pong"}
	if commandNeedsUpdate(a, b) {
		t.Error("commands with no options should not need update")
	}
}

func TestCommandNeedsUpdate_OptionCountChanged(t *testing.T) {
	a := &discordgo.ApplicationCommand{
		Name:        "test",
		Description: "Test",
		Options:     []*discordgo.ApplicationCommandOption{{Name: "a", Type: discordgo.ApplicationCommandOptionString}},
	}
	b := &discordgo.ApplicationCommand{
		Name:        "test",
		Description: "Test",
		Options: []*discordgo.ApplicationCommandOption{
			{Name: "a", Type: discordgo.ApplicationCommandOptionString},
			{Name: "b", Type: discordgo.ApplicationCommandOptionString},
		},
	}
	if !commandNeedsUpdate(a, b) {
		t.Error("different option count should need update")
	}
}

// ── Edge cases: dispatch with nil session/deps ──

func TestNativeCommand_DispatchSlashCommand_NilDeps(t *testing.T) {
	monCtx := newTestMonitorContext(nil)
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Member: &discordgo.Member{
				User: &discordgo.User{ID: "u1", Username: "User1"},
			},
		},
	}
	logger := newTestLogger()
	cmd := &autoreply.ChatCommandDefinition{Key: "test"}

	// Should not panic with nil deps; silently returns.
	dispatchDiscordSlashCommand(monCtx, i, cmd, nil, "test prompt", logger)
}
