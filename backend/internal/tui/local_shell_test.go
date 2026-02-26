package tui

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestResolveShellDefault(t *testing.T) {
	shell := resolveShell()
	if shell == "" {
		t.Fatal("resolveShell returned empty string")
	}

	if runtime.GOOS == "windows" {
		// Windows 应返回 COMSPEC 或 cmd
		if shell != os.Getenv("COMSPEC") && shell != "cmd" {
			t.Errorf("Windows shell: got %q", shell)
		}
	} else {
		// Unix 应返回 SHELL 或 /bin/sh
		if shell != os.Getenv("SHELL") && shell != "/bin/sh" {
			t.Errorf("Unix shell: got %q", shell)
		}
	}
}

func TestResolveShellEnvOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test for Unix only")
	}

	original := os.Getenv("SHELL")
	os.Setenv("SHELL", "/usr/bin/zsh")
	defer os.Setenv("SHELL", original)

	shell := resolveShell()
	if shell != "/usr/bin/zsh" {
		t.Errorf("got %q, want /usr/bin/zsh", shell)
	}
}

func TestLocalShellResultMsg(t *testing.T) {
	// 验证 LocalShellResultMsg 结构字段
	msg := LocalShellResultMsg{
		Command:     "echo hello",
		OutputLines: []string{"hello"},
		ExitCode:    0,
		Signal:      "",
		Err:         nil,
	}

	if msg.Command != "echo hello" {
		t.Errorf("Command: got %q", msg.Command)
	}
	if len(msg.OutputLines) != 1 || msg.OutputLines[0] != "hello" {
		t.Errorf("OutputLines: got %v", msg.OutputLines)
	}
	if msg.ExitCode != 0 {
		t.Errorf("ExitCode: got %d", msg.ExitCode)
	}
	if msg.Err != nil {
		t.Errorf("Err: got %v", msg.Err)
	}
}

func TestLocalShellPermissionMsg(t *testing.T) {
	msgAllowed := LocalShellPermissionMsg{Allowed: true}
	msgDenied := LocalShellPermissionMsg{Allowed: false}

	if !msgAllowed.Allowed {
		t.Error("expected Allowed=true")
	}
	if msgDenied.Allowed {
		t.Error("expected Allowed=false")
	}
}

func TestMaxLocalOutputChars(t *testing.T) {
	if maxLocalOutputChars != 40_000 {
		t.Errorf("maxLocalOutputChars: got %d, want 40000", maxLocalOutputChars)
	}
}

func TestOutputTruncation(t *testing.T) {
	// 模拟 executeLocalShell 中的截断逻辑
	combined := strings.Repeat("A", 50_000)
	if len(combined) > maxLocalOutputChars {
		combined = combined[:maxLocalOutputChars]
	}
	if len(combined) != maxLocalOutputChars {
		t.Errorf("truncated length: got %d, want %d", len(combined), maxLocalOutputChars)
	}
}
