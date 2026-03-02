// bash/exec_security.go — exec.go 安全验证、PATH 管理和辅助函数。
// TS 参考：src/agents/bash-tools.exec.ts L59-419
//
// 包含环境安全黑名单、PATH 前缀管理、审批 slug、
// 主机/安全/ask 模式规范化、系统事件发射、类型定义。
//
// GAP-9 统一: ExecSecurity/ExecAsk/ExecHost 类型统一定义于 infra 包，
// 此处通过类型别名保持 bash 包 API 不变。
package bash

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/infra"
)

// ========== 安全黑名单 ==========

// DANGEROUS_HOST_ENV_VARS 在非沙箱主机（Gateway/Node）上执行时
// 被禁止的环境变量。这些变量可能改变执行流程或注入代码。
// TS 参考: bash-tools.exec.ts L61-78
var dangerousHostEnvVars = map[string]bool{
	"LD_PRELOAD":            true,
	"LD_LIBRARY_PATH":       true,
	"LD_AUDIT":              true,
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH":     true,
	"NODE_OPTIONS":          true,
	"NODE_PATH":             true,
	"PYTHONPATH":            true,
	"PYTHONHOME":            true,
	"RUBYLIB":               true,
	"PERL5LIB":              true,
	"BASH_ENV":              true,
	"ENV":                   true,
	"GCONV_PATH":            true,
	"IFS":                   true,
	"SSLKEYLOGFILE":         true,
}

// dangerousHostEnvPrefixes 危险环境变量前缀。
// TS 参考: bash-tools.exec.ts L79
var dangerousHostEnvPrefixes = []string{"DYLD_", "LD_"}

// ValidateHostEnv 验证主机环境变量安全性。
// 非沙箱执行时必须调用。如果发现危险变量或 PATH 修改，返回错误。
// TS 参考: bash-tools.exec.ts validateHostEnv L83-107
func ValidateHostEnv(env map[string]string) error {
	for key := range env {
		upperKey := strings.ToUpper(key)

		// 1. 阻止已知危险前缀
		for _, prefix := range dangerousHostEnvPrefixes {
			if strings.HasPrefix(upperKey, prefix) {
				return fmt.Errorf(
					"Security Violation: Environment variable '%s' is forbidden during host execution",
					key,
				)
			}
		}

		// 2. 阻止已知危险变量
		if dangerousHostEnvVars[upperKey] {
			return fmt.Errorf(
				"Security Violation: Environment variable '%s' is forbidden during host execution",
				key,
			)
		}

		// 3. 严格阻止 PATH 修改
		if upperKey == "PATH" {
			return fmt.Errorf(
				"Security Violation: Custom 'PATH' variable is forbidden during host execution",
			)
		}
	}
	return nil
}

// ========== 默认常量 ==========

const (
	defaultMaxOutput                = 200_000
	defaultPendingMaxOutput         = 200_000
	defaultNotifyTailChars          = 400
	defaultApprovalTimeoutMs        = 120_000
	defaultApprovalRequestTimeoutMs = 130_000
	defaultApprovalRunningNoticeMs  = 10_000
	approvalSlugLength              = 8
)

// ExecDefaultPath 默认 PATH。
var ExecDefaultPath = func() string {
	p := os.Getenv("PATH")
	if p != "" {
		return p
	}
	return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
}()

// ========== 执行主机/安全/Ask 类型 (GAP-9 统一别名) ==========

// ExecHost 执行主机类型（别名 → infra.ExecHost）。
type ExecHost = infra.ExecHost

const (
	ExecHostSandbox = infra.ExecHostSandbox
	ExecHostGateway = infra.ExecHostGateway
	ExecHostNode    = infra.ExecHostNode
)

// ExecSecurity 安全模式（别名 → infra.ExecSecurity）。
type ExecSecurity = infra.ExecSecurity

const (
	ExecSecurityDeny      = infra.ExecSecurityDeny
	ExecSecurityAllowlist = infra.ExecSecurityAllowlist
	ExecSecuritySandboxed = infra.ExecSecuritySandboxed
	ExecSecurityFull      = infra.ExecSecurityFull
)

// ExecAsk Ask 模式（别名 → infra.ExecAsk）。
type ExecAsk = infra.ExecAsk

const (
	ExecAskOff    = infra.ExecAskOff
	ExecAskOnMiss = infra.ExecAskOnMiss
	ExecAskAlways = infra.ExecAskAlways
)

// ========== 类型定义 ==========

// ExecToolDefaults createExecTool 的默认参数。
// TS 参考: bash-tools.exec.ts ExecToolDefaults L166-185
type ExecToolDefaults struct {
	Host                    ExecHost              `json:"host,omitempty"`
	Security                ExecSecurity          `json:"security,omitempty"`
	Ask                     ExecAsk               `json:"ask,omitempty"`
	Node                    string                `json:"node,omitempty"`
	PathPrepend             []string              `json:"pathPrepend,omitempty"`
	SafeBins                []string              `json:"safeBins,omitempty"`
	AgentID                 string                `json:"agentId,omitempty"`
	BackgroundMs            int                   `json:"backgroundMs,omitempty"`
	TimeoutSec              int                   `json:"timeoutSec,omitempty"`
	ApprovalRunningNoticeMs int                   `json:"approvalRunningNoticeMs,omitempty"`
	Sandbox                 *BashSandboxConfig    `json:"sandbox,omitempty"`
	Elevated                *ExecElevatedDefaults `json:"elevated,omitempty"`
	AllowBackground         bool                  `json:"allowBackground,omitempty"`
	ScopeKey                string                `json:"scopeKey,omitempty"`
	SessionKey              string                `json:"sessionKey,omitempty"`
	MessageProvider         string                `json:"messageProvider,omitempty"`
	NotifyOnExit            bool                  `json:"notifyOnExit,omitempty"`
	Cwd                     string                `json:"cwd,omitempty"`
}

// ExecElevatedDefaults 提权默认参数。
// TS 参考: bash-tools.exec.ts ExecElevatedDefaults L189-193
type ExecElevatedDefaults struct {
	Enabled      bool   `json:"enabled"`
	Allowed      bool   `json:"allowed"`
	DefaultLevel string `json:"defaultLevel"` // "on" | "off" | "ask" | "full"
}

// ExecToolDetails 工具执行状态详情（联合类型）。
// TS 参考: bash-tools.exec.ts ExecToolDetails L243-268
type ExecToolDetails struct {
	Status       string   `json:"status"` // "running" | "completed" | "failed" | "approval-pending"
	SessionID    string   `json:"sessionId,omitempty"`
	PID          int      `json:"pid,omitempty"`
	StartedAt    int64    `json:"startedAt,omitempty"`
	Cwd          string   `json:"cwd,omitempty"`
	Tail         string   `json:"tail,omitempty"`
	ExitCode     *int     `json:"exitCode,omitempty"`
	DurationMs   int64    `json:"durationMs,omitempty"`
	Aggregated   string   `json:"aggregated,omitempty"`
	ApprovalID   string   `json:"approvalId,omitempty"`
	ApprovalSlug string   `json:"approvalSlug,omitempty"`
	ExpiresAtMs  int64    `json:"expiresAtMs,omitempty"`
	Host         ExecHost `json:"host,omitempty"`
	Command      string   `json:"command,omitempty"`
	NodeID       string   `json:"nodeId,omitempty"`
}

// ExecProcessOutcome 进程执行结果。
// TS 参考: bash-tools.exec.ts ExecProcessOutcome L148-156
type ExecProcessOutcome struct {
	Status     string `json:"status"` // "completed" | "failed"
	ExitCode   *int   `json:"exitCode"`
	ExitSignal string `json:"exitSignal,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Aggregated string `json:"aggregated"`
	TimedOut   bool   `json:"timedOut"`
	Reason     string `json:"reason,omitempty"`
}

// ========== 规范化函数 ==========

// NormalizeExecHost 规范化执行主机值。
// TS 参考: bash-tools.exec.ts normalizeExecHost L270-276
func NormalizeExecHost(value string) ExecHost {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case "sandbox":
		return ExecHostSandbox
	case "gateway":
		return ExecHostGateway
	case "node":
		return ExecHostNode
	default:
		return ""
	}
}

// NormalizeExecSecurity 规范化安全模式值。
// TS 参考: bash-tools.exec.ts normalizeExecSecurity L278-284
func NormalizeExecSecurity(value string) ExecSecurity {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case "deny":
		return ExecSecurityDeny
	case "allowlist":
		return ExecSecurityAllowlist
	case "sandboxed":
		return ExecSecuritySandboxed
	case "full":
		return ExecSecurityFull
	default:
		return ""
	}
}

// NormalizeExecAsk 规范化 Ask 模式值。
// TS 参考: bash-tools.exec.ts normalizeExecAsk L286-292
func NormalizeExecAsk(value string) ExecAsk {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case "off":
		return ExecAskOff
	case "on-miss":
		return ExecAskOnMiss
	case "always":
		return ExecAskAlways
	default:
		return ""
	}
}

// RenderExecHostLabel 渲染主机标签。
// TS 参考: bash-tools.exec.ts renderExecHostLabel L294-296
func RenderExecHostLabel(host ExecHost) string {
	switch host {
	case ExecHostSandbox:
		return "sandbox"
	case ExecHostGateway:
		return "gateway"
	case ExecHostNode:
		return "node"
	default:
		return string(host)
	}
}

// MinSecurity 取安全级别的最低值（委托 infra 包）。
func MinSecurity(a, b ExecSecurity) ExecSecurity {
	return infra.MinSecurity(a, b)
}

// MaxAsk 取 Ask 级别的最高值（委托 infra 包）。
func MaxAsk(a, b ExecAsk) ExecAsk {
	return infra.MaxAsk(a, b)
}

// ========== PATH 管理 ==========

// NormalizePathPrepend 规范化 PATH 前缀列表（去重去空）。
// TS 参考: bash-tools.exec.ts normalizePathPrepend L302-320
func NormalizePathPrepend(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var normalized []string
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		normalized = append(normalized, trimmed)
	}
	return normalized
}

// MergePathPrepend 将前缀合并到现有 PATH，去重保序。
// TS 参考: bash-tools.exec.ts mergePathPrepend L322-340
func MergePathPrepend(existing string, prepend []string) string {
	if len(prepend) == 0 {
		return existing
	}
	existingParts := splitPath(existing)
	merged := make([]string, 0, len(prepend)+len(existingParts))
	seen := make(map[string]bool)

	// prepend 优先
	for _, part := range append(prepend, existingParts...) {
		if seen[part] {
			continue
		}
		seen[part] = true
		merged = append(merged, part)
	}

	return strings.Join(merged, string(os.PathListSeparator))
}

// ApplyPathPrepend 将前缀应用到 env 的 PATH。
// TS 参考: bash-tools.exec.ts applyPathPrepend L342-357
func ApplyPathPrepend(env map[string]string, prepend []string, requireExisting bool) {
	if len(prepend) == 0 {
		return
	}
	if requireExisting {
		if _, ok := env["PATH"]; !ok {
			return
		}
	}
	merged := MergePathPrepend(env["PATH"], prepend)
	if merged != "" {
		env["PATH"] = merged
	}
}

// ApplyShellPath 从登录 shell PATH 中合并条目。
// TS 参考: bash-tools.exec.ts applyShellPath L359-374
func ApplyShellPath(env map[string]string, shellPath string) {
	if shellPath == "" {
		return
	}
	entries := splitPath(shellPath)
	if len(entries) == 0 {
		return
	}
	merged := MergePathPrepend(env["PATH"], entries)
	if merged != "" {
		env["PATH"] = merged
	}
}

// splitPath 将 PATH 字符串分割为去空白的非空段。
func splitPath(pathStr string) []string {
	if pathStr == "" {
		return nil
	}
	parts := strings.Split(pathStr, string(os.PathListSeparator))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ========== 审批辅助 ==========

// CreateApprovalSlug 生成审批 slug（截取前 N 个字符）。
// TS 参考: bash-tools.exec.ts createApprovalSlug L398-400
func CreateApprovalSlug(id string) string {
	if len(id) <= approvalSlugLength {
		return id
	}
	return id[:approvalSlugLength]
}

// GenerateApprovalID 生成随机审批 ID。
func GenerateApprovalID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ResolveApprovalRunningNoticeMs 解析审批运行通知延迟。
// TS 参考: bash-tools.exec.ts resolveApprovalRunningNoticeMs L402-410
func ResolveApprovalRunningNoticeMs(value int) int {
	if value <= 0 || math.IsInf(float64(value), 0) {
		return defaultApprovalRunningNoticeMs
	}
	return value
}

// ========== 通知辅助 ==========

// NormalizeNotifyOutput 规范化通知输出文本。
// TS 参考: bash-tools.exec.ts normalizeNotifyOutput L298-300
func NormalizeNotifyOutput(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// MaybeNotifyOnExit 在后台进程退出时发送系统事件通知。
// TS 参考: bash-tools.exec.ts maybeNotifyOnExit L376-396
func MaybeNotifyOnExit(session *ProcessSession, status string) {
	if session == nil {
		return
	}
	if !session.Backgrounded || !session.NotifyOnExit || session.ExitNotified {
		return
	}
	sessionKey := strings.TrimSpace(session.SessionKey)
	if sessionKey == "" {
		return
	}
	session.ExitNotified = true

	exitLabel := ""
	if session.ExitSignal != "" {
		exitLabel = "signal " + session.ExitSignal
	} else {
		exitCode := 0
		if session.ExitCode != nil {
			exitCode = *session.ExitCode
		}
		exitLabel = fmt.Sprintf("code %d", exitCode)
	}

	output := NormalizeNotifyOutput(
		TailString(session.Aggregated, defaultNotifyTailChars),
	)

	var summary string
	shortID := session.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if output != "" {
		summary = fmt.Sprintf("Exec %s (%s, %s) :: %s", status, shortID, exitLabel, output)
	} else {
		summary = fmt.Sprintf("Exec %s (%s, %s)", status, shortID, exitLabel)
	}

	EnqueueSystemEvent(summary, SystemEventOptions{SessionKey: sessionKey})
	// Note: requestHeartbeatNow would be called here in production
}

// EmitExecSystemEvent 发射执行系统事件。
// TS 参考: bash-tools.exec.ts emitExecSystemEvent L412-419
func EmitExecSystemEvent(text string, sessionKey, contextKey string) {
	sk := strings.TrimSpace(sessionKey)
	if sk == "" {
		return
	}
	EnqueueSystemEvent(text, SystemEventOptions{
		SessionKey: sk,
		ContextKey: contextKey,
	})
	// Note: requestHeartbeatNow would be called here in production
}

// TailString 取字符串末尾 N 个字符。
func TailString(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
