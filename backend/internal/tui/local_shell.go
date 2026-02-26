// local_shell.go — TUI 本地 shell 执行
//
// 对齐 TS: src/tui/tui-local-shell.ts(146L) — 差异 LS-01 (P0)
// 会话级权限控制 + exec.Command 执行 + 输出截断。
//
// W4 产出文件 #3。
package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------- 常量 ----------

const maxLocalOutputChars = 40_000

// ---------- 消息类型 ----------

// LocalShellResultMsg 本地 shell 执行结果。
type LocalShellResultMsg struct {
	Command     string
	OutputLines []string
	ExitCode    int
	Signal      string
	Err         error
}

// LocalShellPermissionMsg 本地 shell 权限确认结果。
type LocalShellPermissionMsg struct {
	Allowed bool
}

// ---------- Model 方法 ----------

// runLocalShellLine 执行本地 shell 命令。
// TS 参考: tui-local-shell.ts L81-141
func (m *Model) runLocalShellLine(line string) tea.Cmd {
	cmd := strings.TrimPrefix(line, "!")
	if cmd == "" {
		return nil
	}

	// 已询问但被拒绝
	if m.localExecAsked && !m.localExecAllowed {
		m.chatLog.AddSystem("local shell: not enabled for this session")
		return nil
	}

	// 尚未请求权限 → 请求确认
	if !m.localExecAsked {
		m.localExecAsked = true
		m.chatLog.AddSystem("Allow local shell commands for this session?")
		m.chatLog.AddSystem(
			"This runs commands on YOUR machine (not the gateway) and may delete files or reveal secrets.",
		)
		m.chatLog.AddSystem("Type /yes to allow or /no to deny.")
		// 保存待执行命令，等待用户确认
		m.pendingShellCommand = cmd
		return nil
	}

	// 已授权 → 直接执行
	return m.executeLocalShell(cmd)
}

// handleLocalShellPermission 处理本地 shell 权限确认。
func (m *Model) handleLocalShellPermission(allowed bool) tea.Cmd {
	m.localExecAllowed = allowed
	if allowed {
		m.chatLog.AddSystem("local shell: enabled for this session")
		if m.pendingShellCommand != "" {
			cmd := m.pendingShellCommand
			m.pendingShellCommand = ""
			return m.executeLocalShell(cmd)
		}
	} else {
		m.chatLog.AddSystem("local shell: not enabled")
		m.pendingShellCommand = ""
	}
	return nil
}

// executeLocalShell 执行本地 shell 命令（异步）。
func (m *Model) executeLocalShell(cmd string) tea.Cmd {
	m.chatLog.AddSystem(fmt.Sprintf("[local] $ %s", cmd))

	return func() tea.Msg {
		shell := resolveShell()

		cwd, err := os.Getwd()
		if err != nil {
			return LocalShellResultMsg{
				Command: cmd,
				Err:     fmt.Errorf("getwd: %w", err),
			}
		}

		c := exec.Command(shell, "-c", cmd)
		c.Dir = cwd
		c.Env = os.Environ()

		var stdout, stderr bytes.Buffer
		c.Stdout = &stdout
		c.Stderr = &stderr

		runErr := c.Run()

		// 合并输出
		combined := stdout.String()
		stderrStr := stderr.String()
		if stderrStr != "" {
			if combined != "" {
				combined += "\n"
			}
			combined += stderrStr
		}

		// 截断
		if len(combined) > maxLocalOutputChars {
			combined = combined[:maxLocalOutputChars]
		}
		combined = strings.TrimRight(combined, "\n\r\t ")

		// 解析退出码
		exitCode := 0
		signal := ""
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				// 命令无法执行
				return LocalShellResultMsg{
					Command: cmd,
					Err:     runErr,
				}
			}
		}

		// 逐行拆分输出
		var lines []string
		if combined != "" {
			lines = strings.Split(combined, "\n")
		}

		return LocalShellResultMsg{
			Command:     cmd,
			OutputLines: lines,
			ExitCode:    exitCode,
			Signal:      signal,
		}
	}
}

// resolveShell 获取当前平台的 shell。
func resolveShell() string {
	if runtime.GOOS == "windows" {
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return comspec
		}
		return "cmd"
	}
	if shellEnv := os.Getenv("SHELL"); shellEnv != "" {
		return shellEnv
	}
	return "/bin/sh"
}
