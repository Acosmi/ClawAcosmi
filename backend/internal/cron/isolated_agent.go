package cron

// ============================================================================
// 隔离 Agent 运行器 — Cron 独立会话 Agent 执行
// 对应 TS: cron/isolated-agent/run.ts (597L)
//
// 编排流程:
//   1. Agent 配置解析 (agent ID, defaults merge)
//   2. Session key + workspace 准备
//   3. 模型解析 (config → override → catalog)
//   4. Thinking level 解析 (job → agent → catalog)
//   5. 投递目标解析 (channel/to)
//   6. 安全包装 (外部 hook 内容边界)
//   7. Agent 运行 (PI-embedded 或 CLI, 含 fallback)
//   8. Session 持久化 (usage/model/tokens)
//   9. 投递 (structured → outbound; text → announce)
//
// 隐藏依赖审计: docs/renwu/phase4-global-audit.md §七
// ============================================================================

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/internal/agents/runner"
	"github.com/openacosmi/claw-acismi/internal/outbound"
	"github.com/openacosmi/claw-acismi/internal/routing"
	"github.com/openacosmi/claw-acismi/internal/security"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- DI 接口 ----------

// IsolatedAgentDeps 隔离 Agent 运行器外部依赖（构造时注入）。
type IsolatedAgentDeps struct {
	// Config 全局配置
	Config *types.OpenAcosmiConfig

	// --- Agent 作用域 ---

	// ResolveDefaultAgentID 获取默认 agent ID
	ResolveDefaultAgentID func(cfg *types.OpenAcosmiConfig) string
	// NormalizeAgentID 标准化 agent ID
	NormalizeAgentID func(raw string) string
	// ResolveAgentConfig 获取 agent 级别覆盖配置（非 nil 表示 agent 存在）
	ResolveAgentConfig func(cfg *types.OpenAcosmiConfig, agentID string) interface{}
	// ResolveAgentDir 获取 agent 目录路径
	ResolveAgentDir func(cfg *types.OpenAcosmiConfig, agentID string) string
	// ResolveAgentWorkspaceDir 获取 agent 工作区目录路径
	ResolveAgentWorkspaceDir func(cfg *types.OpenAcosmiConfig, agentID string) string

	// --- 模型解析 ---

	// ResolveConfiguredModelRef 解析配置中的模型引用
	ResolveConfiguredModelRef func(cfg *types.OpenAcosmiConfig) (provider, model string)
	// ResolveAgentTimeoutMs 获取 agent 超时（毫秒）
	ResolveAgentTimeoutMs func(cfg *types.OpenAcosmiConfig, overrideSeconds *int) int64

	// --- 工作区 ---

	// EnsureAgentWorkspace 确保工作区存在
	EnsureAgentWorkspace func(dir string, ensureBootstrap bool) (workspaceDir string, err error)

	// --- Agent 运行 ---

	// RunEmbeddedPiAgent 执行嵌入式 PI Agent
	RunEmbeddedPiAgent func(ctx context.Context, params runner.RunEmbeddedPiAgentParams, deps runner.EmbeddedRunDeps) (*runner.EmbeddedPiRunResult, error)
	// RunEmbeddedPiAgentDeps 嵌入式 PI Agent 依赖
	RunEmbeddedPiAgentDeps runner.EmbeddedRunDeps

	// --- 投递 ---

	// DeliverOutboundPayloads 批量投递出站负载
	DeliverOutboundPayloads func(params outbound.DeliverOutboundParams) ([]outbound.OutboundDeliveryResult, error)
	// RunSubagentAnnounceFlow 子 agent 公告流程
	RunSubagentAnnounceFlow func(params runner.RunSubagentAnnounceParams, deps runner.RunSubagentAnnounceDeps) (bool, error)
	// SubagentAnnounceDeps 子 agent 公告依赖
	SubagentAnnounceDeps runner.RunSubagentAnnounceDeps

	// --- Session 持久化 ---

	// PersistSessionEntry 持久化 session 条目到 store
	PersistSessionEntry func(storePath string, sessionKey string, entry CronSessionEntry) error

	// --- 投递目标 ---

	// ResolveDeliveryTarget 解析投递目标（channel, to, accountId, threadId）
	ResolveDeliveryTarget func(cfg *types.OpenAcosmiConfig, agentID string, channel, to string) (*ResolvedDeliveryTarget, error)

	// --- 日志 ---

	Logger CronLogger

	// --- 路径 ---

	// ResolveSessionTranscriptPath 生成 session 转录文件路径
	// TS 对照: config/sessions/paths.ts resolveSessionTranscriptPath()
	ResolveSessionTranscriptPath func(sessionID, agentID string) string

	// --- 模型验证 ---

	// ResolveAllowedModelRef 解析并验证模型是否在允许列表中。
	// 返回 (provider, model, error)。error 非 nil 表示模型不允许或无效。
	// TS 对照: model-selection.ts resolveAllowedModelRef()
	ResolveAllowedModelRef func(cfg *types.OpenAcosmiConfig, raw, defaultProvider string) (string, string, error)
}

// ---------- 投递目标 ----------

// ResolvedDeliveryTarget 解析后的投递目标。
// TS 对照: delivery-target.ts L23-30
type ResolvedDeliveryTarget struct {
	Channel   string
	To        string
	AccountID string
	ThreadID  string
	Mode      string // "explicit" | "implicit"
	Error     error
}

// ---------- 结果类型 ----------

// RunCronAgentTurnResult 单次 cron agent 运行结果。
// TS 对照: run.ts L100-108
type RunCronAgentTurnResult struct {
	Status     string `json:"status"` // "ok" | "error" | "skipped"
	Summary    string `json:"summary,omitempty"`
	OutputText string `json:"outputText,omitempty"`
	Error      string `json:"error,omitempty"`
	SessionID  string `json:"sessionId,omitempty"`
	SessionKey string `json:"sessionKey,omitempty"`
}

// ---------- 参数 ----------

// RunCronAgentTurnParams 运行参数。
type RunCronAgentTurnParams struct {
	Job        *CronJob
	Message    string
	SessionKey string
	AgentID    string // 可选: 覆盖 job.AgentID
	Lane       string // 可选: 默认 "cron"
}

// ---------- 主编排函数 ----------

// RunCronIsolatedAgentTurn 执行一次隔离 cron agent 运行。
// TS 对照: run.ts L110-596 runCronIsolatedAgentTurn()
//
// 这是 CronServiceDeps.RunIsolatedAgentJob 的完整实现。
// 调用方通过 IsolatedAgentDeps 注入所有外部依赖。
func RunCronIsolatedAgentTurn(ctx context.Context, params RunCronAgentTurnParams, deps IsolatedAgentDeps) RunCronAgentTurnResult {
	cfg := deps.Config
	job := params.Job

	// --- 1. Agent 配置解析 ---
	defaultAgentID := ""
	if deps.ResolveDefaultAgentID != nil {
		defaultAgentID = deps.ResolveDefaultAgentID(cfg)
	}

	requestedAgentID := strings.TrimSpace(params.AgentID)
	if requestedAgentID == "" {
		requestedAgentID = strings.TrimSpace(job.AgentID)
	}

	var agentID string
	if requestedAgentID != "" && deps.NormalizeAgentID != nil && deps.ResolveAgentConfig != nil {
		normalized := deps.NormalizeAgentID(requestedAgentID)
		override := deps.ResolveAgentConfig(cfg, normalized)
		if override != nil {
			agentID = normalized
		}
	}
	if agentID == "" {
		agentID = defaultAgentID
	}

	// --- 2. Session key + workspace ---
	baseSessionKey := strings.TrimSpace(params.SessionKey)
	if baseSessionKey == "" {
		baseSessionKey = "cron:" + job.ID
	}
	agentSessionKey := routing.BuildAgentMainSessionKey(agentID, baseSessionKey)

	workspaceDir := ""
	agentDir := ""
	if deps.ResolveAgentWorkspaceDir != nil {
		workspaceDirRaw := deps.ResolveAgentWorkspaceDir(cfg, agentID)
		if deps.EnsureAgentWorkspace != nil {
			dir, err := deps.EnsureAgentWorkspace(workspaceDirRaw, true)
			if err != nil {
				return withError("failed to ensure workspace: " + err.Error())
			}
			workspaceDir = dir
		} else {
			workspaceDir = workspaceDirRaw
		}
	}
	if deps.ResolveAgentDir != nil {
		agentDir = deps.ResolveAgentDir(cfg, agentID)
	}

	// --- 3. 模型解析 ---
	provider := runner.DefaultProvider
	model := runner.DefaultModel
	if deps.ResolveConfiguredModelRef != nil {
		provider, model = deps.ResolveConfiguredModelRef(cfg)
	}

	// Job 级 model override
	if job.Payload.Kind == PayloadKindAgentTurn {
		modelOverride := strings.TrimSpace(job.Payload.Model)
		if modelOverride != "" {
			if deps.ResolveAllowedModelRef != nil {
				// 通过 DI 接入完整的模型别名解析 + 白名单验证
				p, m, err := deps.ResolveAllowedModelRef(cfg, modelOverride, provider)
				if err != nil {
					logWarn(deps.Logger, "[cron:%s] model override rejected: %s, using default", job.ID, err.Error())
					// 回退到默认模型，不终止运行
				} else {
					provider = p
					model = m
				}
			} else {
				// 兆底: 无验证时直接切割
				parts := strings.SplitN(modelOverride, "/", 2)
				if len(parts) == 2 {
					provider = parts[0]
					model = parts[1]
				} else {
					model = modelOverride
				}
			}
		}
	}

	// --- 4. Session 创建 ---
	nowMs := time.Now().UnixMilli()
	cronSession := ResolveCronSession(agentSessionKey, nowMs)
	runSessionID := cronSession.SessionID
	runSessionKey := agentSessionKey
	if strings.HasPrefix(baseSessionKey, "cron:") {
		runSessionKey = fmt.Sprintf("%s:run:%s", agentSessionKey, runSessionID)
	}

	// 设置 label
	if strings.HasPrefix(baseSessionKey, "cron:") {
		label := strings.TrimSpace(job.Name)
		if label == "" {
			label = job.ID
		}
		cronSession.SessionEntry.Label = "Cron: " + label
	}

	// --- 5. Thinking level ---
	thinkLevel := ""
	if job.Payload.Kind == PayloadKindAgentTurn {
		thinkLevel = strings.TrimSpace(job.Payload.Thinking)
	}
	// 兜底: 未指定则由 runner 内部处理

	// --- 6. Timeout ---
	var timeoutMs int64 = 300000 // 默认 5 分钟
	if deps.ResolveAgentTimeoutMs != nil {
		timeoutMs = deps.ResolveAgentTimeoutMs(cfg, job.Payload.TimeoutSeconds)
	} else if job.Payload.TimeoutSeconds != nil {
		timeoutMs = int64(*job.Payload.TimeoutSeconds) * 1000
	}

	// --- 7. 投递计划 ---
	deliveryPlan := ResolveCronDeliveryPlan(job)
	deliveryRequested := deliveryPlan.Requested
	deliveryBestEffort := deliveryPlan.BestEffort

	var resolvedDelivery *ResolvedDeliveryTarget
	if deliveryRequested && deps.ResolveDeliveryTarget != nil {
		channel := deliveryPlan.Channel
		if channel == "" {
			channel = "last"
		}
		rd, err := deps.ResolveDeliveryTarget(cfg, agentID, channel, deliveryPlan.To)
		if err != nil {
			logWarn(deps.Logger, "[cron:%s] delivery target resolution failed: %s", job.ID, err.Error())
			if !deliveryBestEffort {
				return withRunSession(runSessionID, runSessionKey,
					RunCronAgentTurnResult{Status: "error", Error: err.Error()})
			}
		}
		resolvedDelivery = rd
	}

	// --- 8. 构造 prompt ---
	formattedTime := time.Now().Format(time.RFC3339)
	timeLine := "Current time: " + formattedTime

	base := fmt.Sprintf("[cron:%s %s] %s", job.ID, job.Name, params.Message)
	base = strings.TrimSpace(base)

	// 安全包装（外部 hook）
	commandBody := base + "\n" + timeLine
	isExternalHook := IsExternalHookSession(baseSessionKey)
	allowUnsafe := false
	if job.Payload.AllowUnsafeExternalContent != nil {
		allowUnsafe = *job.Payload.AllowUnsafeExternalContent
	}
	if isExternalHook && !allowUnsafe {
		hookSource := security.SourceFromHookType(GetHookType(baseSessionKey))
		commandBody = security.BuildSafeExternalPrompt(security.SafePromptParams{
			Content:   params.Message,
			Source:    hookSource,
			JobName:   job.Name,
			JobID:     job.ID,
			Timestamp: formattedTime,
		})
		commandBody += "\n" + timeLine
	}
	commandBody = strings.TrimSpace(commandBody)

	if deliveryRequested {
		commandBody += "\n\nReturn your summary as plain text; it will be delivered automatically. " +
			"If the task explicitly calls for messaging a specific external recipient, " +
			"note who/where it should go instead of sending it yourself."
		commandBody = strings.TrimSpace(commandBody)
	}

	// --- 9. 持久化 systemSent ---
	cronSession.SessionEntry.SystemSent = true
	if deps.PersistSessionEntry != nil {
		_ = deps.PersistSessionEntry(cronSession.StorePath, agentSessionKey, cronSession.SessionEntry)
	}

	// --- 10. 运行 Agent ---
	var runResult *runner.EmbeddedPiRunResult
	runStartedAt := time.Now().UnixMilli()
	var runEndedAt int64

	if deps.RunEmbeddedPiAgent != nil {
		agentParams := runner.RunEmbeddedPiAgentParams{
			SessionID:    runSessionID,
			SessionKey:   agentSessionKey,
			AgentID:      agentID,
			SessionFile:  resolveTranscriptPath(deps, runSessionID, agentID),
			WorkspaceDir: workspaceDir,
			AgentDir:     agentDir,
			Prompt:       commandBody,
			Provider:     provider,
			Model:        model,
			TimeoutMs:    timeoutMs,
			RunID:        runSessionID,
			ThinkLevel:   thinkLevel,
			Config:       cfg,
		}
		lane := params.Lane
		if lane == "" {
			lane = "cron"
		}

		result, err := deps.RunEmbeddedPiAgent(ctx, agentParams, deps.RunEmbeddedPiAgentDeps)
		runEndedAt = time.Now().UnixMilli()
		if err != nil {
			return withRunSession(runSessionID, runSessionKey,
				RunCronAgentTurnResult{Status: "error", Error: err.Error()})
		}
		runResult = result
	} else {
		return withRunSession(runSessionID, runSessionKey,
			RunCronAgentTurnResult{Status: "error", Error: "RunEmbeddedPiAgent not configured"})
	}

	// --- 11. 处理结果 ---
	payloads := toDeliveryPayloads(runResult.Payloads)

	// 更新 session store (usage/model)
	if runResult.Meta.AgentMeta != nil {
		meta := runResult.Meta.AgentMeta
		if meta.Provider != "" {
			cronSession.SessionEntry.ModelProvider = meta.Provider
		}
		if meta.Model != "" {
			cronSession.SessionEntry.Model = meta.Model
		}
		if meta.Usage != nil {
			cronSession.SessionEntry.InputTokens = meta.Usage.Input
			cronSession.SessionEntry.OutputTokens = meta.Usage.Output
			total := meta.Usage.Total
			if total == 0 {
				total = meta.Usage.Input + meta.Usage.Output
			}
			cronSession.SessionEntry.TotalTokens = total
		}
	}
	if deps.PersistSessionEntry != nil {
		_ = deps.PersistSessionEntry(cronSession.StorePath, agentSessionKey, cronSession.SessionEntry)
	}

	summary := PickSummaryFromPayloads(payloads)
	if summary == "" {
		if len(payloads) > 0 {
			summary = PickSummaryFromOutput(payloads[0].Text)
		}
	}
	outputText := PickLastNonEmptyText(payloads)
	synthesizedText := strings.TrimSpace(outputText)
	if synthesizedText == "" {
		synthesizedText = strings.TrimSpace(summary)
	}

	// --- 12. 投递 ---
	if deliveryRequested {
		ackMaxChars := ResolveHeartbeatAckMaxChars(nil)
		skipHeartbeat := IsHeartbeatOnlyResponse(payloads, ackMaxChars)
		if skipHeartbeat {
			return withRunSession(runSessionID, runSessionKey,
				RunCronAgentTurnResult{Status: "ok", Summary: summary, OutputText: outputText})
		}

		deliveryPayload := PickLastDeliverablePayload(payloads)
		hasStructured := HasDeliverableStructuredContent(deliveryPayload)

		if resolvedDelivery != nil {
			if resolvedDelivery.Error != nil {
				if !deliveryBestEffort {
					return withRunSession(runSessionID, runSessionKey,
						RunCronAgentTurnResult{Status: "error", Error: resolvedDelivery.Error.Error(), Summary: summary, OutputText: outputText})
				}
				logWarn(deps.Logger, "[cron:%s] %s", job.ID, resolvedDelivery.Error.Error())
				return withRunSession(runSessionID, runSessionKey,
					RunCronAgentTurnResult{Status: "ok", Summary: summary, OutputText: outputText})
			}

			if resolvedDelivery.To == "" {
				msg := "cron delivery target is missing"
				if !deliveryBestEffort {
					return withRunSession(runSessionID, runSessionKey,
						RunCronAgentTurnResult{Status: "error", Error: msg, Summary: summary, OutputText: outputText})
				}
				logWarn(deps.Logger, "[cron:%s] %s", job.ID, msg)
				return withRunSession(runSessionID, runSessionKey,
					RunCronAgentTurnResult{Status: "ok", Summary: summary, OutputText: outputText})
			}

			// 结构化内容 → 直接出站投递
			if hasStructured && deps.DeliverOutboundPayloads != nil {
				outPayloads := toOutboundPayloads(payloads)
				_, err := deps.DeliverOutboundPayloads(outbound.DeliverOutboundParams{
					Channel:    resolvedDelivery.Channel,
					To:         resolvedDelivery.To,
					AccountID:  resolvedDelivery.AccountID,
					ThreadID:   resolvedDelivery.ThreadID,
					Payloads:   outPayloads,
					BestEffort: deliveryBestEffort,
					AbortCtx:   ctx,
				})
				if err != nil && !deliveryBestEffort {
					return withRunSession(runSessionID, runSessionKey,
						RunCronAgentTurnResult{Status: "error", Error: err.Error(), Summary: summary, OutputText: outputText})
				}
			} else if synthesizedText != "" && deps.RunSubagentAnnounceFlow != nil {
				// 纯文本 → 子 agent 公告流程
				taskLabel := strings.TrimSpace(job.Name)
				if taskLabel == "" {
					taskLabel = "cron:" + job.ID
				}
				announceSessionKey := routing.BuildAgentMainSessionKey(agentID, "")
				didAnnounce, err := deps.RunSubagentAnnounceFlow(runner.RunSubagentAnnounceParams{
					ChildSessionKey:     runSessionKey,
					ChildRunID:          job.ID + ":" + runSessionID,
					RequesterSessionKey: announceSessionKey,
					RequesterOrigin: &runner.DeliveryContext{
						Channel:   resolvedDelivery.Channel,
						To:        resolvedDelivery.To,
						AccountID: resolvedDelivery.AccountID,
						ThreadID:  resolvedDelivery.ThreadID,
					},
					RequesterDisplayKey: announceSessionKey,
					Task:                taskLabel,
					TimeoutMs:           timeoutMs,
					Cleanup:             "keep",
					RoundOneReply:       synthesizedText,
					WaitForCompletion:   false,
					StartedAt:           runStartedAt,
					EndedAt:             runEndedAt,
					Outcome:             &runner.SubagentRunOutcome{Status: "ok"},
					AnnounceType:        runner.SubagentAnnounceCron,
				}, deps.SubagentAnnounceDeps)
				if err != nil && !deliveryBestEffort {
					return withRunSession(runSessionID, runSessionKey,
						RunCronAgentTurnResult{Status: "error", Error: err.Error(), Summary: summary, OutputText: outputText})
				}
				if !didAnnounce {
					msg := "cron announce delivery failed"
					if !deliveryBestEffort {
						return withRunSession(runSessionID, runSessionKey,
							RunCronAgentTurnResult{Status: "error", Error: msg, Summary: summary, OutputText: outputText})
					}
					logWarn(deps.Logger, "[cron:%s] %s", job.ID, msg)
				}
			}
		}
	}

	return withRunSession(runSessionID, runSessionKey,
		RunCronAgentTurnResult{Status: "ok", Summary: summary, OutputText: outputText})
}

// ---------- 桥接: CronServiceDeps.RunIsolatedAgentJob ----------

// NewRunIsolatedAgentJobFunc 创建 CronServiceDeps.RunIsolatedAgentJob 的实现。
// 调用方在服务启动时注入 IsolatedAgentDeps，返回闭包供 CronServiceDeps 使用。
func NewRunIsolatedAgentJobFunc(deps IsolatedAgentDeps) func(params IsolatedAgentJobParams) (*IsolatedAgentJobResult, error) {
	return func(params IsolatedAgentJobParams) (*IsolatedAgentJobResult, error) {
		// 从 CronPayload 中提取 message
		message := params.Payload.Message
		if message == "" {
			message = params.Payload.Text
		}

		// 构造完整的 job（只由 timer.go 传入关键字段）
		job := &CronJob{
			ID:      params.JobID,
			AgentID: params.AgentID,
			Payload: params.Payload,
		}

		result := RunCronIsolatedAgentTurn(context.Background(), RunCronAgentTurnParams{
			Job:     job,
			Message: message,
			AgentID: params.AgentID,
		}, deps)

		if result.Status == "error" {
			return nil, fmt.Errorf("cron agent turn failed: %s", result.Error)
		}

		return &IsolatedAgentJobResult{
			SessionID:  result.SessionID,
			SessionKey: result.SessionKey,
			Summary:    result.Summary,
		}, nil
	}
}

// ---------- 内部辅助 ----------

// withRunSession 附加 sessionID/sessionKey 到结果。
func withRunSession(sessionID, sessionKey string, result RunCronAgentTurnResult) RunCronAgentTurnResult {
	result.SessionID = sessionID
	result.SessionKey = sessionKey
	return result
}

// withError 构造错误结果。
func withError(msg string) RunCronAgentTurnResult {
	return RunCronAgentTurnResult{Status: "error", Error: msg}
}

// resolveTranscriptPath 生成 session 转录文件路径。
// 委托给 DI，未配置时返回空字符串。
// TS 对照: config/sessions/paths.ts resolveSessionTranscriptPath()
func resolveTranscriptPath(deps IsolatedAgentDeps, sessionID, agentID string) string {
	if deps.ResolveSessionTranscriptPath != nil {
		return deps.ResolveSessionTranscriptPath(sessionID, agentID)
	}
	return ""
}

// toDeliveryPayloads 将 runner.RunPayload 转换为 DeliveryPayload。
func toDeliveryPayloads(payloads []runner.RunPayload) []DeliveryPayload {
	result := make([]DeliveryPayload, len(payloads))
	for i, p := range payloads {
		result[i] = DeliveryPayload{
			Text:      p.Text,
			MediaURLs: p.MediaURLs,
		}
		if p.MediaURL != "" && len(result[i].MediaURLs) == 0 {
			result[i].MediaURLs = []string{p.MediaURL}
		}
	}
	return result
}

// toOutboundPayloads 将 DeliveryPayload 转换为 outbound.ReplyPayload。
func toOutboundPayloads(payloads []DeliveryPayload) []outbound.ReplyPayload {
	result := make([]outbound.ReplyPayload, len(payloads))
	for i, p := range payloads {
		result[i] = outbound.ReplyPayload{
			Text:        p.Text,
			MediaURLs:   p.MediaURLs,
			ChannelData: p.ChannelData,
		}
		if p.MediaURL != "" {
			result[i].MediaURL = p.MediaURL
		}
	}
	return result
}

// logWarn 带格式化的警告日志。
func logWarn(logger CronLogger, format string, args ...interface{}) {
	if logger != nil {
		logger.Warn(fmt.Sprintf(format, args...))
	}
}
