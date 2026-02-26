package tui

import (
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantArgs string
	}{
		{"simple", "/help", "help", ""},
		{"with args", "/agent main", "agent", "main"},
		{"with extra spaces", "/model   anthropic/claude  ", "model", "anthropic/claude"},
		{"alias", "/elev on", "elevated", "on"},
		{"leading slash only", "/", "", ""},
		{"no slash", "hello", "hello", ""},
		{"multi arg", "/think high", "think", "high"},
		{"empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCommand(tt.input)
			if got.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", got.Name, tt.wantName)
			}
			if got.Args != tt.wantArgs {
				t.Errorf("Args: got %q, want %q", got.Args, tt.wantArgs)
			}
		})
	}
}

func TestGetSlashCommands(t *testing.T) {
	cmds := GetSlashCommands(SlashCommandOptions{})

	if len(cmds) < 15 {
		t.Errorf("expected at least 15 commands, got %d", len(cmds))
	}

	// 检查核心命令存在
	expectedNames := []string{
		"help", "status", "agent", "agents", "session", "sessions",
		"model", "models", "think", "verbose", "reasoning",
		"usage", "elevated", "abort", "new", "exit",
	}
	nameSet := make(map[string]bool)
	for _, cmd := range cmds {
		nameSet[cmd.Name] = true
	}
	for _, name := range expectedNames {
		if !nameSet[name] {
			t.Errorf("missing command: %q", name)
		}
	}
}

func TestGetSlashCommandsCompletions(t *testing.T) {
	cmds := GetSlashCommands(SlashCommandOptions{})

	// think 命令应有补全
	var thinkCmd *SlashCommand
	for i := range cmds {
		if cmds[i].Name == "think" {
			thinkCmd = &cmds[i]
			break
		}
	}
	if thinkCmd == nil {
		t.Fatal("think command not found")
	}
	if thinkCmd.GetArgumentCompletions == nil {
		t.Fatal("think command has no completions")
	}

	// 空前缀 — 应返回所有 thinking levels
	completions := thinkCmd.GetArgumentCompletions("")
	if len(completions) == 0 {
		t.Error("expected completions for empty prefix")
	}

	// "o" 前缀 — 应过滤
	filtered := thinkCmd.GetArgumentCompletions("o")
	for _, c := range filtered {
		if !strings.HasPrefix(c.Value, "o") {
			t.Errorf("completion %q does not start with 'o'", c.Value)
		}
	}
}

func TestHelpText(t *testing.T) {
	text := HelpText(SlashCommandOptions{})

	if text == "" {
		t.Fatal("help text is empty")
	}

	// 帮助文本应包含核心命令
	for _, cmd := range []string{"/help", "/status", "/agent", "/model", "/abort", "/exit"} {
		if !strings.Contains(text, cmd) {
			t.Errorf("help text missing %q", cmd)
		}
	}
}

func TestFilterCompletions(t *testing.T) {
	levels := []string{"on", "off", "optional"}

	// 空前缀 — 全部返回
	all := filterCompletions(levels, "")
	if len(all) != 3 {
		t.Errorf("empty prefix: got %d, want 3", len(all))
	}

	// "o" 前缀 — 全部匹配
	withO := filterCompletions(levels, "o")
	if len(withO) != 3 {
		t.Errorf("'o' prefix: got %d, want 3", len(withO))
	}

	// "on" 前缀
	withOn := filterCompletions(levels, "on")
	if len(withOn) != 1 {
		t.Errorf("'on' prefix: got %d, want 1", len(withOn))
	}

	// "x" 前缀 — 无匹配
	noMatch := filterCompletions(levels, "x")
	if len(noMatch) != 0 {
		t.Errorf("'x' prefix: got %d, want 0", len(noMatch))
	}
}
