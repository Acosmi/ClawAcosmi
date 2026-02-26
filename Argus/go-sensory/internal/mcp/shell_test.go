package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────
// Tests: Blocklist
// ──────────────────────────────────────────────────────────────

func TestCheckBlocklist_DangerousCommands(t *testing.T) {
	dangerous := []string{
		"rm -rf /",
		"rm -rf /usr",
		"rm -rF /var",
		"mkfs.ext4 /dev/sda1",
		"dd if=/dev/zero of=/dev/sda",
		"shutdown -h now",
		"reboot",
		"halt",
		"init 0",
		"systemctl stop nginx",
		"systemctl disable sshd",
		"launchctl unload com.apple.something",
		"diskutil erase disk0",
		"diskutil partition disk1",
	}
	for _, cmd := range dangerous {
		reason := checkBlocklist(cmd)
		if reason == "" {
			t.Errorf("checkBlocklist(%q) should be blocked", cmd)
		}
	}
}

func TestCheckBlocklist_SafeCommands(t *testing.T) {
	safe := []string{
		"ls -la",
		"pwd",
		"echo hello",
		"git status",
		"cat /etc/hosts",
		"go test ./...",
		"npm run dev",
		"python3 script.py",
		"curl https://example.com",
		"find . -name '*.go'",
	}
	for _, cmd := range safe {
		reason := checkBlocklist(cmd)
		if reason != "" {
			t.Errorf("checkBlocklist(%q) blocked: %s (should be allowed)", cmd, reason)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Environment sanitization
// ──────────────────────────────────────────────────────────────

func TestSanitizeEnv_StripsSecrets(t *testing.T) {
	env := sanitizeEnv()
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		upper := strings.ToUpper(key)
		for _, prefix := range sensitiveEnvPrefixes {
			if strings.HasPrefix(upper, prefix) || strings.Contains(upper, prefix) {
				t.Errorf("Sensitive env var not stripped: %s", key)
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: LimitedBuffer
// ──────────────────────────────────────────────────────────────

func TestLimitedBuffer_UnderLimit(t *testing.T) {
	lb := &limitedBuffer{limit: 100}
	n, err := lb.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("Write error: n=%d err=%v", n, err)
	}
	if lb.String() != "hello" {
		t.Errorf("Got %q, want hello", lb.String())
	}
	if lb.truncated {
		t.Error("Should not be truncated")
	}
}

func TestLimitedBuffer_OverLimit(t *testing.T) {
	lb := &limitedBuffer{limit: 5}
	n, err := lb.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	// limitedBuffer pretend-accepts all bytes for exec.Cmd compatibility
	if n != 11 {
		t.Errorf("n = %d, want 11 (pretend-accept)", n)
	}
	// But only the first 5 bytes are buffered
	if lb.String() != "hello" {
		t.Errorf("Got %q, want 'hello' (truncated)", lb.String())
	}
	if !lb.truncated {
		t.Error("Should be truncated")
	}
}

func TestLimitedBuffer_ExactLimit(t *testing.T) {
	lb := &limitedBuffer{limit: 5}
	lb.Write([]byte("hello"))
	if lb.truncated {
		t.Error("Should not be truncated at exact limit")
	}
	// Next write should truncate
	lb.Write([]byte("!"))
	if !lb.truncated {
		t.Error("Should be truncated after exceeding limit")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: run_shell execution
// ──────────────────────────────────────────────────────────────

func TestRunShell_SimpleCommand(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	stdout := content["stdout"].(string)
	if !strings.Contains(stdout, "hello") {
		t.Errorf("stdout = %q, want 'hello'", stdout)
	}
	if content["exit_code"].(int) != 0 {
		t.Errorf("exit_code = %v, want 0", content["exit_code"])
	}
}

func TestRunShell_FailedCommand(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"false"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError for non-zero exit")
	}
	content := result.Content.(map[string]any)
	if content["exit_code"].(int) == 0 {
		t.Error("Expected non-zero exit code")
	}
}

func TestRunShell_Timeout(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"sleep 10","timeout_seconds":1}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	if content["timed_out"] != true {
		t.Error("Expected timed_out = true")
	}
}

func TestRunShell_BlockedCommand(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"rm -rf /"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for blocked command")
	}
	if !strings.Contains(result.Error, "安全策略") {
		t.Errorf("Error should mention security policy: %q", result.Error)
	}
}

func TestRunShell_Denied(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newTestGateway(&alwaysDeny{})}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo safe"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected denial")
	}
}

func TestRunShell_EmptyCommand(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty command")
	}
}

func TestRunShell_Stderr(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo error_msg >&2"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	stderr := content["stderr"].(string)
	if !strings.Contains(stderr, "error_msg") {
		t.Errorf("stderr = %q, want 'error_msg'", stderr)
	}
}

func TestRunShell_WorkingDir(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"pwd","working_dir":"/tmp"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	stdout := strings.TrimSpace(content["stdout"].(string))
	// macOS uses /private/tmp
	if stdout != "/tmp" && stdout != "/private/tmp" {
		t.Errorf("Working dir = %q, want /tmp or /private/tmp", stdout)
	}
}

func TestRunShell_HumanModifiedCommand(t *testing.T) {
	tool := &RunShellTool{deps: ShellDeps{
		Gateway: newTestGateway(&modifyNotifier{
			modified: json.RawMessage(`{"command":"echo modified"}`),
		}),
	}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo original"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	stdout := strings.TrimSpace(content["stdout"].(string))
	if stdout != "modified" {
		t.Errorf("stdout = %q, want 'modified' (human-modified)", stdout)
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Risk level + registration
// ──────────────────────────────────────────────────────────────

func TestRunShell_AlwaysHighRisk(t *testing.T) {
	tool := &RunShellTool{}
	if tool.Risk() != RiskHigh {
		t.Errorf("Risk = %v, want RiskHigh", tool.Risk())
	}
}

func TestRegisterShellTools(t *testing.T) {
	r := NewRegistry()
	RegisterShellTools(r, ShellDeps{Gateway: newAutoModeGateway()})

	if r.Get("run_shell") == nil {
		t.Error("run_shell not registered")
	}
}
