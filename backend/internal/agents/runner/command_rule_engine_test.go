package runner

// command_rule_engine_test.go — P3 规则引擎单元测试
// 测试 EvaluateCommand, matchPattern, MergeRulesWithPresets

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/infra"
)

// ============================================================================
// EvaluateCommand 测试
// ============================================================================

func TestEvaluateCommand_DenyMatch(t *testing.T) {
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "rm -rf /", Action: infra.RuleActionDeny, Description: "Block root delete"},
	}
	result := EvaluateCommand("rm -rf /", rules)
	if !result.Matched {
		t.Fatal("expected match")
	}
	if result.Action != infra.RuleActionDeny {
		t.Fatalf("expected deny, got %s", result.Action)
	}
	if result.Rule.ID != "r1" {
		t.Fatalf("expected rule r1, got %s", result.Rule.ID)
	}
}

func TestEvaluateCommand_AskMatch(t *testing.T) {
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "sudo *", Action: infra.RuleActionAsk, Description: "Confirm sudo"},
	}
	result := EvaluateCommand("sudo apt install vim", rules)
	if !result.Matched {
		t.Fatal("expected match")
	}
	if result.Action != infra.RuleActionAsk {
		t.Fatalf("expected ask, got %s", result.Action)
	}
}

func TestEvaluateCommand_AllowMatch(t *testing.T) {
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "npm install*", Action: infra.RuleActionAllow, Description: "Allow npm"},
	}
	result := EvaluateCommand("npm install express", rules)
	if !result.Matched {
		t.Fatal("expected match")
	}
	if result.Action != infra.RuleActionAllow {
		t.Fatalf("expected allow, got %s", result.Action)
	}
}

func TestEvaluateCommand_NoMatch(t *testing.T) {
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "rm -rf /", Action: infra.RuleActionDeny},
	}
	result := EvaluateCommand("echo hello", rules)
	if result.Matched {
		t.Fatal("expected no match")
	}
}

func TestEvaluateCommand_PriorityOrder_DenyOverAllow(t *testing.T) {
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "rm -rf /tmp", Action: infra.RuleActionAllow, Priority: 20},
		{ID: "r2", Pattern: "rm -rf *", Action: infra.RuleActionDeny, Priority: 0},
	}
	result := EvaluateCommand("rm -rf /tmp", rules)
	if !result.Matched {
		t.Fatal("expected match")
	}
	if result.Action != infra.RuleActionDeny {
		t.Fatalf("expected deny (priority over allow), got %s", result.Action)
	}
}

func TestEvaluateCommand_PriorityOrder_AskOverAllow(t *testing.T) {
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "curl*", Action: infra.RuleActionAllow, Priority: 20},
		{ID: "r2", Pattern: "curl*", Action: infra.RuleActionAsk, Priority: 10},
	}
	result := EvaluateCommand("curl https://example.com", rules)
	if result.Action != infra.RuleActionAsk {
		t.Fatalf("expected ask (priority over allow), got %s", result.Action)
	}
}

func TestEvaluateCommand_EmptyRules(t *testing.T) {
	result := EvaluateCommand("echo hello", nil)
	if result.Matched {
		t.Fatal("expected no match with empty rules")
	}
}

func TestEvaluateCommand_EmptyCommand(t *testing.T) {
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "*", Action: infra.RuleActionDeny},
	}
	result := EvaluateCommand("", rules)
	if result.Matched {
		t.Fatal("expected no match with empty command")
	}
}

func TestEvaluateCommand_SamePriorityUsesID(t *testing.T) {
	// 同一 action + 同一 priority，取 priority 值更小的
	rules := []infra.CommandRule{
		{ID: "r1", Pattern: "echo*", Action: infra.RuleActionDeny, Priority: 5},
		{ID: "r2", Pattern: "echo*", Action: infra.RuleActionDeny, Priority: 3},
	}
	result := EvaluateCommand("echo hello", rules)
	if result.Rule.ID != "r2" {
		t.Fatalf("expected r2 (lower priority value), got %s", result.Rule.ID)
	}
}

// ============================================================================
// matchPattern 测试
// ============================================================================

func TestMatchPattern_Exact(t *testing.T) {
	if !matchPattern("ls -la", "ls -la") {
		t.Fatal("expected exact match")
	}
}

func TestMatchPattern_CaseInsensitive(t *testing.T) {
	if !matchPattern("NPM install", "npm install") {
		t.Fatal("expected case-insensitive match")
	}
}

func TestMatchPattern_GlobWildcard(t *testing.T) {
	// path.Match: pattern "*" matches any non-separator character sequence
	if !matchPattern("rm -rf /tmp", "rm -rf *") {
		t.Fatal("expected glob match")
	}
}

func TestMatchPattern_Prefix(t *testing.T) {
	if !matchPattern("npm install express lodash", "npm install") {
		t.Fatal("expected prefix match")
	}
}

func TestMatchPattern_PrefixWithStar(t *testing.T) {
	if !matchPattern("go test ./...", "go test*") {
		t.Fatal("expected prefix-with-star match")
	}
}

func TestMatchPattern_Substring(t *testing.T) {
	if !matchPattern("echo foo | sudo tee /tmp/bar", "*sudo*") {
		t.Fatal("expected substring match")
	}
}

func TestMatchPattern_SubstringDD(t *testing.T) {
	if !matchPattern("dd if=/dev/zero of=/dev/sda", "*dd *of=/dev/*") {
		t.Fatal("expected substring match for dd")
	}
}

func TestMatchPattern_NoMatch(t *testing.T) {
	if matchPattern("echo hello", "rm -rf /") {
		t.Fatal("expected no match")
	}
}

func TestMatchPattern_EmptyPattern(t *testing.T) {
	if matchPattern("echo hello", "") {
		t.Fatal("expected no match for empty pattern")
	}
}

func TestMatchPattern_ForkBomb(t *testing.T) {
	if !matchPattern(":(){ :|:& };:", "*(){*|*&*};*") {
		t.Fatal("expected fork bomb match")
	}
}

// ============================================================================
// MergeRulesWithPresets 测试
// ============================================================================

func TestMergeRulesWithPresets_Empty(t *testing.T) {
	merged := MergeRulesWithPresets(nil)
	if len(merged) != len(PresetCommandRules) {
		t.Fatalf("expected %d preset rules, got %d", len(PresetCommandRules), len(merged))
	}
}

func TestMergeRulesWithPresets_WithUser(t *testing.T) {
	userRules := []infra.CommandRule{
		{ID: "user1", Pattern: "docker run*", Action: infra.RuleActionAsk},
	}
	merged := MergeRulesWithPresets(userRules)
	expected := len(PresetCommandRules) + 1
	if len(merged) != expected {
		t.Fatalf("expected %d rules, got %d", expected, len(merged))
	}
	// 预设在前
	if merged[0].ID != PresetCommandRules[0].ID {
		t.Fatal("expected preset rules first")
	}
	// 用户规则在后
	if merged[len(merged)-1].ID != "user1" {
		t.Fatal("expected user rule last")
	}
}

// ============================================================================
// Preset Rules 完整性测试
// ============================================================================

func TestPresetRules_DangerousCommands(t *testing.T) {
	tests := []struct {
		command        string
		expectedAction infra.CommandRuleAction
	}{
		{"rm -rf /", infra.RuleActionDeny},
		{"rm -rf /home/user", infra.RuleActionDeny},
		{"shutdown -h now", infra.RuleActionDeny},
		{"reboot", infra.RuleActionDeny},
		{"halt", infra.RuleActionDeny},
		{"mkfs.ext4 /dev/sda1", infra.RuleActionDeny},
		{"sudo apt install vim", infra.RuleActionAsk},
		{"npm install express", infra.RuleActionAllow},
		{"go build ./...", infra.RuleActionAllow},
		{"go test -v ./...", infra.RuleActionAllow},
	}

	for _, tc := range tests {
		result := EvaluateCommand(tc.command, PresetCommandRules)
		if !result.Matched {
			t.Errorf("command %q: expected match, got no match", tc.command)
			continue
		}
		if result.Action != tc.expectedAction {
			t.Errorf("command %q: expected %s, got %s (rule: %s)",
				tc.command, tc.expectedAction, result.Action, result.Rule.Pattern)
		}
	}
}

func TestPresetRules_SafeCommandsUnmatched(t *testing.T) {
	safeCmds := []string{
		"echo hello",
		"ls -la",
		"cat /etc/hosts",
		"pwd",
		"whoami",
	}
	for _, cmd := range safeCmds {
		result := EvaluateCommand(cmd, PresetCommandRules)
		if result.Matched {
			t.Errorf("command %q: expected no match, got %s (rule: %s)",
				cmd, result.Action, result.Rule.Pattern)
		}
	}
}
