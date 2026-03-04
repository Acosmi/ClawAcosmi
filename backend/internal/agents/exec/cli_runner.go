package exec

// ============================================================================
// CLI Agent 运行器 — 完整实现
// 对应 TS: agents/cli-runner.ts (363L)
// 隐藏依赖审计: docs/renwu/phase4-cli-runner-audit.md
// ============================================================================

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/agents/helpers"
	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/internal/agents/workspace"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// CliRunnerParams CLI Agent 运行参数。
type CliRunnerParams struct {
	SessionID           string                     `json:"sessionId"`
	SessionKey          string                     `json:"sessionKey,omitempty"`
	AgentID             string                     `json:"agentId,omitempty"`
	SessionFile         string                     `json:"sessionFile"`
	WorkspaceDir        string                     `json:"workspaceDir"`
	Config              *types.OpenAcosmiConfig    `json:"-"`
	Prompt              string                     `json:"prompt"`
	Provider            string                     `json:"provider"`
	Model               string                     `json:"model,omitempty"`
	TimeoutMs           int64                      `json:"timeoutMs"`
	RunID               string                     `json:"runId"`
	ExtraSystemPrompt   string                     `json:"extraSystemPrompt,omitempty"`
	CliSessionID        string                     `json:"cliSessionId,omitempty"`
	SystemPromptBuilder CliSystemPromptBuilderFunc `json:"-"` // 可选 DI (P4-GA-CLI3)
}

// CliSystemPromptBuilderFunc 完整 system prompt 构建回调。
// TS 对应: cli-runner.ts 中的 bootstrap + heartbeat + doc paths 构建逻辑。
// 返回空字符串时回退到默认 ResolveSystemPromptUsage。
type CliSystemPromptBuilderFunc func(ctx CliSystemPromptContext) string

// CliSystemPromptContext system prompt 构建上下文。
type CliSystemPromptContext struct {
	WorkspaceDir string
	AgentDir     string
	Provider     string
	Model        string
	IsResume     bool
}

// CliRunResult CLI Agent 运行结果。
type CliRunResult struct {
	Payloads []CliRunPayload `json:"payloads,omitempty"`
	Meta     CliRunMeta      `json:"meta"`
}

// CliRunPayload CLI 运行输出负载。
type CliRunPayload struct {
	Text string `json:"text,omitempty"`
}

// CliRunMeta CLI 运行元数据。
type CliRunMeta struct {
	DurationMs int64         `json:"durationMs"`
	AgentMeta  *CliAgentMeta `json:"agentMeta,omitempty"`
}

// CliAgentMeta CLI Agent 元数据。
type CliAgentMeta struct {
	SessionID string `json:"sessionId"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
}

// RunCliAgent 通过外部 CLI 后端运行 Agent。
// TS 对应: cli-runner.ts → runCliAgent()
func RunCliAgent(params CliRunnerParams) (*CliRunResult, error) {
	started := time.Now()
	log := slog.Default().With("subsystem", "agent/cli-runner")

	// 1. 工作区解析
	wsResult := workspace.ResolveRunWorkspaceDir(
		params.WorkspaceDir, params.SessionKey, params.AgentID, params.Config,
	)
	workspaceDir := wsResult.WorkspaceDir
	if wsResult.UsedFallback {
		log.Warn("workspace-fallback",
			"caller", "runCliAgent",
			"reason", wsResult.FallbackReason,
			"runId", params.RunID,
		)
	}

	// 2. CLI backend 解析
	backendResolved := ResolveCliBackendConfig(params.Provider, params.Config)
	if backendResolved == nil {
		return nil, fmt.Errorf("unknown CLI backend: %s", params.Provider)
	}
	backend := backendResolved.Config
	modelID := strings.TrimSpace(params.Model)
	if modelID == "" {
		modelID = "default"
	}
	normalizedModel := NormalizeCliModel(modelID, backend)

	// 3. Session ID 解析
	cliSessionIDToSend, isNew := ResolveSessionIDToSend(backend, params.CliSessionID)
	useResume := params.CliSessionID != "" &&
		cliSessionIDToSend != "" &&
		len(backend.ResumeArgs) > 0

	// 4. 系统提示词 (P4-GA-CLI3: 支持完整 system prompt 构建)
	systemPromptArg := ""
	if params.SystemPromptBuilder != nil {
		systemPromptArg = params.SystemPromptBuilder(CliSystemPromptContext{
			WorkspaceDir: workspaceDir,
			Provider:     params.Provider,
			Model:        normalizedModel,
			IsResume:     useResume,
		})
	}
	if systemPromptArg == "" {
		systemPromptArg = ResolveSystemPromptUsage(backend, isNew, params.ExtraSystemPrompt)
	}

	// 5. prompt 输入模式
	argsPrompt, stdinPayload := ResolvePromptInput(backend, params.Prompt)

	// 6. 构建参数
	baseArgs := backend.Args
	if useResume && len(backend.ResumeArgs) > 0 {
		baseArgs = backend.ResumeArgs
	}
	if useResume {
		resolved := make([]string, len(baseArgs))
		for i, arg := range baseArgs {
			resolved[i] = strings.ReplaceAll(arg, "{sessionId}", cliSessionIDToSend)
		}
		baseArgs = resolved
	}
	args := BuildCliArgs(backend, baseArgs, normalizedModel,
		cliSessionIDToSend, systemPromptArg, nil, argsPrompt, useResume)

	// 7. 确定序列化 key
	serialize := backend.Serialize == nil || *backend.Serialize
	queueKey := backendResolved.ID
	if !serialize {
		queueKey = fmt.Sprintf("%s:%s", backendResolved.ID, params.RunID)
	}

	// 8. 确定 sessionId 用于结果
	sessionIDSent := ""
	if cliSessionIDToSend != "" {
		if useResume || backend.SessionArg != "" || len(backend.SessionArgs) > 0 {
			sessionIDSent = cliSessionIDToSend
		}
	}

	// 9. 执行
	output, err := EnqueueCliRun(queueKey, func() (*CliOutput, error) {
		log.Info("cli exec",
			"provider", params.Provider,
			"model", normalizedModel,
			"promptChars", len(params.Prompt),
		)

		// 构建环境变量 (隐藏依赖 #4)
		env := buildCliEnv(backend)

		// 执行子进程
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Duration(params.TimeoutMs)*time.Millisecond,
		)
		defer cancel()

		cmd := exec.CommandContext(ctx, backend.Command, args...)
		cmd.Dir = workspaceDir
		cmd.Env = env
		if stdinPayload != "" {
			cmd.Stdin = strings.NewReader(stdinPayload)
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		runErr := cmd.Run()
		stdoutStr := strings.TrimSpace(stdout.String())
		stderrStr := strings.TrimSpace(stderr.String())

		if runErr != nil {
			errText := stderrStr
			if errText == "" {
				errText = stdoutStr
			}
			if errText == "" {
				errText = "CLI failed."
			}
			reason := models.ClassifyFailoverReason(errText)
			if reason == "" {
				reason = "unknown"
			}
			return nil, &models.FailoverError{
				Message:  errText,
				Reason:   reason,
				Provider: params.Provider,
				Model:    modelID,
				Status:   models.ResolveFailoverStatus(reason),
			}
		}

		// 解析输出
		outputMode := backend.Output
		if useResume && backend.ResumeOutput != "" {
			outputMode = backend.ResumeOutput
		}

		switch outputMode {
		case "text":
			return &CliOutput{Text: stdoutStr}, nil
		case "jsonl":
			parsed := ParseCliJsonl(stdoutStr, backend)
			if parsed != nil {
				return parsed, nil
			}
			return &CliOutput{Text: stdoutStr}, nil
		default: // "json"
			parsed := ParseCliJson(stdoutStr, backend)
			if parsed != nil {
				return parsed, nil
			}
			return &CliOutput{Text: stdoutStr}, nil
		}
	})

	if err != nil {
		// Failover 错误分类 (隐藏依赖 #7)
		if _, ok := models.IsFailoverError(err); ok {
			return nil, err
		}
		msg := err.Error()
		if helpers.IsFailoverErrorMessage(msg) {
			reason := models.ClassifyFailoverReason(msg)
			if reason == "" {
				reason = "unknown"
			}
			return nil, &models.FailoverError{
				Message:  msg,
				Reason:   reason,
				Provider: params.Provider,
				Model:    modelID,
				Status:   models.ResolveFailoverStatus(reason),
			}
		}
		return nil, err
	}

	// 10. 构建结果
	text := strings.TrimSpace(output.Text)
	var payloads []CliRunPayload
	if text != "" {
		payloads = []CliRunPayload{{Text: text}}
	}

	resultSessionID := output.SessionID
	if resultSessionID == "" {
		resultSessionID = sessionIDSent
	}
	if resultSessionID == "" {
		resultSessionID = params.SessionID
	}

	return &CliRunResult{
		Payloads: payloads,
		Meta: CliRunMeta{
			DurationMs: time.Since(started).Milliseconds(),
			AgentMeta: &CliAgentMeta{
				SessionID: resultSessionID,
				Provider:  params.Provider,
				Model:     modelID,
			},
		},
	}, nil
}

// RunClaudeCliAgent 便利包装：使用 claude-cli 后端。
func RunClaudeCliAgent(params CliRunnerParams) (*CliRunResult, error) {
	if params.Provider == "" {
		params.Provider = "claude-cli"
	}
	if params.Model == "" {
		params.Model = "opus"
	}
	return RunCliAgent(params)
}

// buildCliEnv 构建子进程环境变量。
func buildCliEnv(backend *types.CliBackendConfig) []string {
	// 从当前进程继承
	env := make(map[string]string)
	for _, e := range currentEnv() {
		if k, v, ok := strings.Cut(e, "="); ok {
			env[k] = v
		}
	}
	// 添加 backend 自定义环境变量
	for k, v := range backend.Env {
		env[k] = v
	}
	// 清除指定的环境变量 (隐藏依赖 #4)
	for _, k := range backend.ClearEnv {
		delete(env, k)
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// currentEnv 返回当前进程环境变量（可测试替换）。
var currentEnv = os.Environ
