package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"Argus-compound/go-sensory/internal/input"
)

// ──────────────────────────────────────────────────────────────
// Security: command blocklist (defense-in-depth layer 1)
//
// These patterns are checked BEFORE the command reaches ApprovalGateway.
// Even in AutoMode, blocklisted commands are rejected.
// ──────────────────────────────────────────────────────────────

// blockedPatterns are regex patterns that will be rejected outright.
// These represent commands with very high potential for irreversible damage.
var blockedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-[rfRF]*\s+/\s*$`),          // rm -rf /
	regexp.MustCompile(`(?i)\brm\s+-[rfRF]*\s+/[a-z]+\s*$`),    // rm -rf /usr etc.
	regexp.MustCompile(`(?i)\bmkfs\b`),                         // format disk
	regexp.MustCompile(`(?i)\bdd\s+.*of=/dev/`),                // dd to device
	regexp.MustCompile(`(?i)\b:(){ :\|:& };:`),                 // fork bomb
	regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`),                 // write to raw device
	regexp.MustCompile(`(?i)\bshutdown\s`),                     // shutdown
	regexp.MustCompile(`(?i)\breboot\b`),                       // reboot
	regexp.MustCompile(`(?i)\bhalt\b`),                         // halt
	regexp.MustCompile(`(?i)\binit\s+0`),                       // init 0
	regexp.MustCompile(`(?i)\bsystemctl\s+(stop|disable)\s`),   // stop services
	regexp.MustCompile(`(?i)\blaunchctl\s+unload\s`),           // unload macOS daemons
	regexp.MustCompile(`(?i)\bdiskutil\s+(erase|partition)\b`), // macOS disk ops
}

// sensitiveEnvPrefixes are env var prefixes to strip from the child process.
var sensitiveEnvPrefixes = []string{
	"AWS_", "AZURE_", "GCP_",
	"OPENAI_", "ANTHROPIC_", "GOOGLE_API_",
	"API_KEY", "API_SECRET", "SECRET_",
	"TOKEN", "PRIVATE_KEY",
	"DATABASE_URL", "DB_PASSWORD",
	"SSH_AUTH_SOCK",
}

// ──────────────────────────────────────────────────────────────
// Sandbox configuration
// ──────────────────────────────────────────────────────────────

const (
	defaultTimeoutSec = 30
	maxTimeoutSec     = 300       // 5 minutes max
	maxOutputBytes    = 64 * 1024 // 64 KB per stream
	shellPath         = "/bin/sh"
)

// ──────────────────────────────────────────────────────────────
// Shell tool dependencies
// ──────────────────────────────────────────────────────────────

// ShellDeps bundles dependencies for the shell tool.
type ShellDeps struct {
	Gateway *input.ApprovalGateway
}

// RegisterShellTools batch-registers shell execution MCP tools.
func RegisterShellTools(r *Registry, deps ShellDeps) {
	r.Register(&RunShellTool{deps: deps})
}

// ══════════════════════════════════════════════════════════════
// Tool: run_shell
//
// Security layers:
//   1. Pattern blocklist (hardcoded, bypass-proof)
//   2. ApprovalGateway (always RiskHigh → requires human confirmation)
//   3. Execution sandbox (timeout, output cap, env sanitization)
//   4. Audit trail (via ApprovalGateway logging)
// ══════════════════════════════════════════════════════════════

type RunShellTool struct{ deps ShellDeps }

func (t *RunShellTool) Name() string { return "run_shell" }
func (t *RunShellTool) Description() string {
	return "执行 shell 命令（高危操作，始终需要人工确认）"
}
func (t *RunShellTool) Category() ToolCategory { return CategoryAction }
func (t *RunShellTool) Risk() RiskLevel        { return RiskHigh }
func (t *RunShellTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"command": {
				Type:        "string",
				Description: "Shell command to execute (runs via /bin/sh -c)",
			},
			"working_dir": {
				Type:        "string",
				Description: "Working directory (default: user home)",
			},
			"timeout_seconds": {
				Type:        "integer",
				Description: "Execution timeout in seconds (default 30, max 300)",
				Default:     defaultTimeoutSec,
			},
		},
		Required: []string{"command"},
	}
}

func (t *RunShellTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Command        string `json:"command"`
		WorkingDir     string `json:"working_dir"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	p.TimeoutSeconds = defaultTimeoutSec
	if err := json.Unmarshal(params, &p); err != nil || p.Command == "" {
		return &ToolResult{IsError: true, Error: "command is required"}, nil
	}

	// ── Layer 1: Pattern blocklist ──
	if reason := checkBlocklist(p.Command); reason != "" {
		log.Printf("[MCP:run_shell] BLOCKED by pattern: %s ← %q", reason, p.Command)
		return &ToolResult{
			IsError: true,
			Error:   fmt.Sprintf("命令被安全策略阻止: %s", reason),
		}, nil
	}

	// ── Layer 2: ApprovalGateway (always RiskHigh) ──
	if t.deps.Gateway != nil {
		approved, modifiedParams, err := t.deps.Gateway.CheckAndApprove(
			ctx, "run_shell", params, "mcp", nil,
		)
		if err != nil {
			return nil, err
		}
		if !approved {
			return &ToolResult{IsError: true, Error: "命令被人类审核员拒绝"}, nil
		}
		// Human may have modified the command for safety
		if modifiedParams != nil {
			var mp struct {
				Command string `json:"command"`
			}
			if json.Unmarshal(modifiedParams, &mp) == nil && mp.Command != "" {
				log.Printf("[MCP:run_shell] Command modified by human: %q → %q", p.Command, mp.Command)
				p.Command = mp.Command
			}
		}
	}

	// ── Layer 3: Execution sandbox ──
	// Validate timeout
	if p.TimeoutSeconds <= 0 || p.TimeoutSeconds > maxTimeoutSec {
		p.TimeoutSeconds = defaultTimeoutSec
	}

	// Resolve working directory
	workDir := p.WorkingDir
	if workDir == "" {
		workDir, _ = os.UserHomeDir()
	}

	// Execute with timeout
	result, err := executeShellSandboxed(ctx, p.Command, workDir, p.TimeoutSeconds)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ──────────────────────────────────────────────────────────────
// Security helpers
// ──────────────────────────────────────────────────────────────

// checkBlocklist tests a command against the hardcoded blocklist.
// Returns a human-readable reason if blocked, empty string if allowed.
func checkBlocklist(cmd string) string {
	for _, pat := range blockedPatterns {
		if pat.MatchString(cmd) {
			return fmt.Sprintf("匹配危险模式: %s", pat.String())
		}
	}
	return ""
}

// sanitizeEnv returns the current environment with sensitive vars removed.
func sanitizeEnv() []string {
	env := os.Environ()
	clean := make([]string, 0, len(env))
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		keyUpper := strings.ToUpper(key)
		skip := false
		for _, prefix := range sensitiveEnvPrefixes {
			if strings.HasPrefix(keyUpper, prefix) || strings.Contains(keyUpper, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			clean = append(clean, e)
		}
	}
	return clean
}

// executeShellSandboxed runs a command in a sandboxed environment.
func executeShellSandboxed(ctx context.Context, command, workDir string, timeoutSec int) (*ToolResult, error) {
	timeout := time.Duration(timeoutSec) * time.Second
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, shellPath, "-c", command)
	cmd.Dir = workDir
	cmd.Env = sanitizeEnv()

	// Capture output with size limits
	var stdout, stderr limitedBuffer
	stdout.limit = maxOutputBytes
	stderr.limit = maxOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	elapsed := time.Since(startTime)

	exitCode := 0
	timedOut := false

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			timedOut = true
			exitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("exec error: %w", err)
		}
	}

	content := map[string]any{
		"exit_code":   exitCode,
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
		"elapsed_ms":  elapsed.Milliseconds(),
		"timed_out":   timedOut,
		"working_dir": workDir,
	}

	if stdout.truncated {
		content["stdout_truncated"] = true
	}
	if stderr.truncated {
		content["stderr_truncated"] = true
	}

	return &ToolResult{
		Content: content,
		IsError: exitCode != 0,
		Error:   formatExitError(exitCode, timedOut),
	}, nil
}

// formatExitError produces a human-readable error message for non-zero exits.
func formatExitError(code int, timedOut bool) string {
	if timedOut {
		return "命令执行超时"
	}
	if code != 0 {
		return fmt.Sprintf("命令退出码: %d", code)
	}
	return ""
}

// ──────────────────────────────────────────────────────────────
// limitedBuffer: io.Writer that caps output size
// ──────────────────────────────────────────────────────────────

// limitedBuffer is a bytes.Buffer that stops accepting writes after
// reaching a byte limit.  This prevents a runaway command from
// consuming all available memory.
type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p) // always report full acceptance to avoid exec.Cmd short-write errors
	remaining := lb.limit - lb.buf.Len()
	if remaining <= 0 {
		lb.truncated = true
		return n, nil
	}
	if len(p) > remaining {
		lb.truncated = true
		p = p[:remaining]
	}
	lb.buf.Write(p)
	return n, nil
}

func (lb *limitedBuffer) String() string {
	return lb.buf.String()
}
