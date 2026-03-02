package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	"github.com/openacosmi/claw-acismi/internal/agents/llmclient"
	"github.com/openacosmi/claw-acismi/internal/agents/models"
	"github.com/openacosmi/claw-acismi/internal/agents/runner"
	"github.com/openacosmi/claw-acismi/internal/agents/scope"
	"github.com/openacosmi/claw-acismi/internal/agents/skills"
	"github.com/openacosmi/claw-acismi/internal/agents/tools"
	"github.com/openacosmi/claw-acismi/internal/argus"
	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/internal/autoreply/reply"
	"github.com/openacosmi/claw-acismi/internal/browser"
	"github.com/openacosmi/claw-acismi/internal/channels"
	"github.com/openacosmi/claw-acismi/internal/channels/dingtalk"
	"github.com/openacosmi/claw-acismi/internal/channels/feishu"
	"github.com/openacosmi/claw-acismi/internal/channels/website"
	"github.com/openacosmi/claw-acismi/internal/channels/wechat_mp"
	"github.com/openacosmi/claw-acismi/internal/channels/wecom"
	"github.com/openacosmi/claw-acismi/internal/channels/xiaohongshu"
	"github.com/openacosmi/claw-acismi/internal/cli"
	"github.com/openacosmi/claw-acismi/internal/config"
	"github.com/openacosmi/claw-acismi/internal/cron"
	"github.com/openacosmi/claw-acismi/internal/infra"
	"github.com/openacosmi/claw-acismi/internal/media"
	"github.com/openacosmi/claw-acismi/internal/memory/uhms"
	"github.com/openacosmi/claw-acismi/internal/memory/uhms/vectoradapter"
	"github.com/openacosmi/claw-acismi/internal/sandbox"
	applog "github.com/openacosmi/claw-acismi/pkg/log"
	"github.com/openacosmi/claw-acismi/pkg/mcpremote"
	types "github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- 网关启动编排 ----------
// 对齐 TS server.impl.ts: startGatewayServer()

// GatewayServerOptions 网关启动选项。
type GatewayServerOptions struct {
	ControlUIDir string
	BindMode     BindMode
	BindHost     string
}

// GatewayRuntime 网关运行时，持有 server/state 引用及关闭方法。
type GatewayRuntime struct {
	State             *GatewayState
	HTTPServer        *GatewayHTTPServer
	MaintenanceTimers *MaintenanceTimers
	mu                sync.Mutex
	closed            bool
}

// Close 优雅关闭网关。
func (rt *GatewayRuntime) Close(reason string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed {
		return nil
	}
	rt.closed = true

	slog.Info("gateway: shutting down", "reason", reason)

	// 停止维护计时器（tick 广播）
	if rt.MaintenanceTimers != nil {
		rt.MaintenanceTimers.Stop()
	}

	rt.State.SetPhase(BootPhaseStopping)

	// 广播 shutdown 事件
	rt.State.Broadcaster().Broadcast("gateway.shutdown", ShutdownEvent{
		Reason: reason,
	}, nil)

	// 停止沙箱子系统（容器池 + Worker）
	rt.State.StopSandbox()

	// 停止原生沙箱 Worker
	rt.State.StopNativeSandbox()

	// 停止 Argus 视觉子智能体
	rt.State.StopArgus()

	// (Phase 2A: Coder Bridge 已删除 — oa-coder 升级为 spawn_coder_agent)

	// 停止 MCP 远程工具 Bridge
	rt.State.StopRemoteMCP()

	// 停止合约 TTL 清理 goroutine
	if rt.State.contractCleanupDone != nil {
		close(rt.State.contractCleanupDone)
	}

	// 停止方案确认 TTL 清理 goroutine
	if rt.State.planConfirmMgr != nil {
		rt.State.planConfirmMgr.Close()
	}

	// 停止结果签收 TTL 清理 goroutine
	if rt.State.resultApprovalMgr != nil {
		rt.State.resultApprovalMgr.Close()
	}

	// 停止 UHMS 记忆系统
	rt.State.StopUHMS()

	// 优雅关闭 HTTP 服务器
	if err := rt.HTTPServer.Shutdown(); err != nil {
		slog.Error("gateway: http shutdown error", "error", err)
	}

	rt.State.SetPhase(BootPhaseStopped)
	slog.Info("gateway: shutdown complete")
	return nil
}

// ---------- Argus Bridge → Agent 适配器 ----------

// argusBridgeAdapter 将 *argus.Bridge 适配为 runner.ArgusBridgeForAgent 接口。
// 转换 mcpclient 类型为 runner 本地类型，避免 runner→mcpclient 依赖。
type argusBridgeAdapter struct {
	bridge *argus.Bridge
}

func (a *argusBridgeAdapter) AgentTools() []runner.ArgusToolDef {
	tools := a.bridge.Tools()
	result := make([]runner.ArgusToolDef, len(tools))
	for i, t := range tools {
		result[i] = runner.ArgusToolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return result
}

func (a *argusBridgeAdapter) AgentCallTool(ctx context.Context, name string, args json.RawMessage, timeout time.Duration) (string, error) {
	result, err := a.bridge.CallTool(ctx, name, args, timeout)
	if err != nil {
		return "", err
	}
	// 提取 MCP content → 纯文本（image 标注大小）
	var sb strings.Builder
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(c.Text)
		case "image":
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("[image: %s, %d bytes base64]", c.MIME, len(c.Data)))
		}
	}
	if result.IsError {
		return fmt.Sprintf("[Argus error] %s", sb.String()), nil
	}
	return sb.String(), nil
}

// maxImageBase64Size Anthropic API 限制 tool_result 中 image 最大 5MB。
const maxImageBase64Size = 5 * 1024 * 1024

func (a *argusBridgeAdapter) AgentCallToolMultimodal(ctx context.Context, name string, args json.RawMessage, timeout time.Duration) ([]llmclient.ContentBlock, error) {
	result, err := a.bridge.CallTool(ctx, name, args, timeout)
	if err != nil {
		return nil, err
	}
	var blocks []llmclient.ContentBlock
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			// Argus capture_screen 返回 JSON 文本内含 image_b64 字段，
			// 需要从中提取图片数据转为 image block。
			if extracted := extractEmbeddedImage(c.Text); extracted != nil {
				blocks = append(blocks, *extracted)
			} else {
				blocks = append(blocks, llmclient.ContentBlock{Type: "text", Text: c.Text})
			}
		case "image":
			if len(c.Data) > maxImageBase64Size {
				blocks = append(blocks, llmclient.ContentBlock{
					Type: "text",
					Text: fmt.Sprintf("[image too large for vision: %s, %d bytes — exceeds 5MB limit]", c.MIME, len(c.Data)),
				})
			} else {
				mediaType := c.MIME
				if mediaType == "" {
					mediaType = "image/png"
				}
				blocks = append(blocks, llmclient.ContentBlock{
					Type: "image",
					Source: &llmclient.ImageSource{
						Type:      "base64",
						MediaType: mediaType,
						Data:      c.Data,
					},
				})
			}
		}
	}
	if result.IsError && len(blocks) > 0 {
		blocks = append([]llmclient.ContentBlock{{Type: "text", Text: "[Argus error]"}}, blocks...)
	}
	return blocks, nil
}

// extractEmbeddedImage 从 Argus capture_screen 返回的 JSON 文本中提取嵌入图片。
// Argus 的 capture_screen 返回 {"image_b64": "<base64>", "width": N, "height": N, ...}
// 作为 type="text" 的 MCP 内容块。此函数检测并转换为 image ContentBlock。
func extractEmbeddedImage(text string) *llmclient.ContentBlock {
	// 快速检测：文本必须是包含 image_b64 的 JSON
	if !strings.Contains(text, "image_b64") {
		return nil
	}
	var payload struct {
		ImageB64 string `json:"image_b64"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil || payload.ImageB64 == "" {
		return nil
	}
	slog.Info("argus: extracted embedded image from text block",
		"width", payload.Width, "height", payload.Height, "base64Len", len(payload.ImageB64))
	return &llmclient.ContentBlock{
		Type: "image",
		Source: &llmclient.ImageSource{
			Type:      "base64",
			MediaType: "image/jpeg",
			Data:      payload.ImageB64,
		},
	}
}

// (Phase 2A: coderBridgeAdapter 已删除 — oa-coder 升级为 spawn_coder_agent)

// ---------- Native Sandbox Bridge → Agent 适配器 ----------

// nativeSandboxRouterAdapter 将 *sandbox.NativeSandboxRouter 适配为 runner.NativeSandboxForAgent 接口。
// Router 根据 securityLevel 动态路由: L1(allowlist)→持久Worker IPC, L2(sandboxed)→一次性CLI。
// adapter 负责从 runner.SandboxExecOptions 解包参数并转换 mount 类型。
type nativeSandboxRouterAdapter struct {
	router *sandbox.NativeSandboxRouter
}

func (a *nativeSandboxRouterAdapter) ExecuteSandboxed(ctx context.Context, opts runner.SandboxExecOptions) (stdout, stderr string, exitCode int, err error) {
	var mounts []sandbox.SandboxMountParam
	for _, m := range opts.MountRequests {
		mounts = append(mounts, sandbox.SandboxMountParam{HostPath: m.HostPath, MountMode: m.MountMode})
	}
	return a.router.ExecuteSandboxed(ctx, opts.Cmd, opts.Args, opts.Env, opts.TimeoutMs, opts.SecurityLevel, opts.Workspace, mounts)
}

// ---------- Remote MCP Bridge → Agent 适配器 ----------

// remoteMCPBridgeAdapter 将 *mcpremote.RemoteBridge 适配为 runner.RemoteMCPBridgeForAgent 接口。
type remoteMCPBridgeAdapter struct {
	bridge *mcpremote.RemoteBridge
}

func (a *remoteMCPBridgeAdapter) AgentRemoteTools() []runner.RemoteToolDef {
	tools := a.bridge.Tools()
	result := make([]runner.RemoteToolDef, len(tools))
	for i, t := range tools {
		result[i] = runner.RemoteToolDef{
			Name:        t.Name,
			Title:       t.Title,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return result
}

func (a *remoteMCPBridgeAdapter) AgentCallRemoteTool(ctx context.Context, name string, args json.RawMessage, timeout time.Duration) (string, error) {
	result, err := a.bridge.CallTool(ctx, name, args, timeout)
	if err != nil {
		return "", err
	}
	return mcpremote.ToolCallResultToText(result), nil
}

// ---------- Media Sender → Agent 适配器 ----------

// mediaSenderAdapter 将 ChannelMgr 适配为 runner 包的 MediaSender 接口。
type mediaSenderAdapter struct {
	channelMgr *channels.Manager
}

func (a *mediaSenderAdapter) SendMedia(ctx context.Context, channelID, to string, data []byte, mimeType, message string) error {
	_, err := a.channelMgr.SendMessage(channels.ChannelID(channelID), channels.OutboundSendParams{
		Ctx:           ctx,
		To:            to,
		Text:          message,
		MediaData:     data,
		MediaMimeType: mimeType,
	})
	return err
}

// ---------- Bocha Search → Agent 适配器 ----------

// bochaSearchAdapter 将 tools.BochaSearchProvider 适配为 runner.WebSearchProvider 接口。
// 转换 tools.WebSearchResult → runner.WebSearchResult，避免 runner→tools 循环依赖。
type bochaSearchAdapter struct {
	provider *tools.BochaSearchProvider
}

func (a *bochaSearchAdapter) Search(ctx context.Context, query string, maxResults int) ([]runner.WebSearchResult, error) {
	results, err := a.provider.Search(ctx, query, maxResults)
	if err != nil {
		return nil, err
	}
	out := make([]runner.WebSearchResult, len(results))
	for i, r := range results {
		out[i] = runner.WebSearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Snippet,
		}
	}
	return out, nil
}

// ---------- Browser CDP URL 解析 ----------

// resolveBrowserCdpURL 从配置解析 CDP WebSocket URL。
func resolveBrowserCdpURL(cfg *types.BrowserConfig) string {
	if cfg.CdpURL != "" {
		return cfg.CdpURL
	}
	if cfg.DefaultProfile != "" && cfg.Profiles != nil {
		if p := cfg.Profiles[cfg.DefaultProfile]; p != nil {
			if p.CdpURL != "" {
				return p.CdpURL
			}
			if p.CdpPort != nil && *p.CdpPort > 0 {
				return fmt.Sprintf("ws://127.0.0.1:%d", *p.CdpPort)
			}
		}
	}
	return ""
}

// ---------- UHMS Bridge → Agent 适配器 ----------

// uhmsBridgeAdapter 将 *uhms.DefaultManager 适配为 runner.UHMSBridgeForAgent 接口。
// 转换 llmclient.ChatMessage ↔ uhms.Message，避免 runner→uhms 直接依赖。
type uhmsBridgeAdapter struct {
	mgr           *uhms.DefaultManager
	broadcaster   *Broadcaster      // 可选, 用于 WS 事件广播
	bootMgr       *uhms.BootManager // 可选, 用于更新 boot.json LastSession
	coldStartInfo func() string     // 可选, lastSummary 为空时生成系统简报
}

func (a *uhmsBridgeAdapter) CompressChatMessages(ctx context.Context, messages []llmclient.ChatMessage, tokenBudget int) ([]llmclient.ChatMessage, error) {
	// 快速路径: 粗估 token 量，低于阈值跳过双向转换 (参考 gRPC-Go PreparedMsg / Letta 门控)
	// 估算方式: 累加字节数 / 3.5 (英文+代码混合场景 ~90-96% 准确)
	threshold := a.mgr.CompressThreshold()
	if threshold > 0 {
		var totalBytes int
		for i := range messages {
			totalBytes += len(messages[i].Content)
		}
		estimatedTokens := int(float64(totalBytes) / 3.5)
		if estimatedTokens < threshold {
			return messages, nil // 直通: 避免 chatMessagesToUHMS + uhmsToChatMessages 开销
		}
	}

	beforeCount := len(messages)

	// 1. llmclient.ChatMessage → uhms.Message
	uhmsMessages := chatMessagesToUHMS(messages)

	// 2. 调用 UHMS 压缩
	compressed, err := a.mgr.CompressIfNeeded(ctx, uhmsMessages, tokenBudget)
	if err != nil {
		return nil, err
	}

	// 3. 广播压缩事件 (仅在实际压缩时)
	afterCount := len(compressed)
	if a.broadcaster != nil && afterCount < beforeCount {
		a.broadcaster.Broadcast("memory.compressed", map[string]interface{}{
			"before_messages": beforeCount,
			"after_messages":  afterCount,
			"ts":              time.Now().UnixMilli(),
		}, nil)
	}

	// 4. uhms.Message → llmclient.ChatMessage
	return uhmsToChatMessages(compressed), nil
}

func (a *uhmsBridgeAdapter) CommitChatSession(ctx context.Context, userID, sessionKey string, messages []llmclient.ChatMessage) error {
	uhmsMessages := chatMessagesToUHMS(messages)
	result, err := a.mgr.CommitSession(ctx, userID, sessionKey, uhmsMessages)
	if err != nil {
		return err
	}

	// 广播提交事件
	if a.broadcaster != nil && result != nil {
		a.broadcaster.Broadcast("memory.committed", map[string]interface{}{
			"session_key":      result.SessionKey,
			"memories_created": result.MemoriesCreated,
			"tokens_saved":     result.TokensSaved,
			"ts":               time.Now().UnixMilli(),
		}, nil)
	}

	// 更新 boot.json LastSession（同步执行——CommitChatSession 已在 goroutine 中被调用，
	// 此处同步确保关机时 WaitGroup 能覆盖完整的 commit + boot 更新链路）
	if a.bootMgr != nil {
		brief := a.mgr.BuildContextBrief(ctx, userID)
		if brief != "" {
			if err := a.bootMgr.UpdateLastSession(brief, nil); err != nil {
				slog.Warn("boot: update last session failed (non-fatal)", "error", err)
			}
		}
	}
	return nil
}

func (a *uhmsBridgeAdapter) BuildContextBrief(ctx context.Context) string {
	// "default" matches the single-user desktop app convention used by
	// CompressChatMessages/Status/etc. Multi-user would require per-session userID.
	brief := a.mgr.BuildContextBrief(ctx, "default")
	if brief != "" {
		return brief
	}
	// Cold start: lastSummary 为空（首次对话或重启后），生成系统简报
	if a.coldStartInfo != nil {
		return a.coldStartInfo()
	}
	return ""
}

// --- Boot 模式扩展 ---

func (a *uhmsBridgeAdapter) IsSkillsIndexed() bool {
	if a.bootMgr == nil {
		return false
	}
	// 注意: 分发中不降级 IsSkillsIndexed — 上一轮 VFS 数据仍有效可搜索，
	// 降级会导致 isBootMode=false → search_skills 工具消失 + 文件扫描 token 浪费。
	// 分发状态仅通过 IsSkillsDistributing() 在搜索结果中追加提示。
	return a.bootMgr.IsSkillsIndexed()
}

func (a *uhmsBridgeAdapter) IsSkillsDistributing() bool {
	return a.mgr.IsSkillsDistributing()
}

func (a *uhmsBridgeAdapter) SearchSkillsVFS(ctx context.Context, query string, topK int) ([]runner.SkillSearchHit, error) {
	hits, err := a.mgr.SearchSystemEntries(ctx, "sys_skills", query, topK)
	if err != nil {
		return nil, err
	}

	results := make([]runner.SkillSearchHit, 0, len(hits))
	for _, h := range hits {
		name, _ := h.Payload["name"].(string)
		cat, _ := h.Payload["category"].(string)

		// 读 L0 摘要
		abstract := ""
		if name != "" && cat != "" {
			if l0, readErr := a.mgr.VFS().ReadSystemL0("skills", cat, name); readErr == nil {
				abstract = l0
			}
		}

		results = append(results, runner.SkillSearchHit{
			Name:     name,
			Category: cat,
			Abstract: abstract,
			VFSPath:  h.VFSPath,
		})
	}
	return results, nil
}

func (a *uhmsBridgeAdapter) ReadSkillVFS(_ context.Context, category, name string) (string, error) {
	return a.mgr.VFS().ReadSystemL2("skills", category, name)
}

// chatMessagesToUHMS converts llmclient.ChatMessage slice to uhms.Message slice.
// Extracts text content from ContentBlock arrays.
func chatMessagesToUHMS(messages []llmclient.ChatMessage) []uhms.Message {
	result := make([]uhms.Message, 0, len(messages))
	for _, msg := range messages {
		var sb strings.Builder
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(block.Text)
			case "tool_use":
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(fmt.Sprintf("[tool_use: %s]", block.Name))
			case "tool_result":
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				text := block.ResultText
				if runes := []rune(text); len(runes) > 2000 {
					text = string(runes[:2000]) + "... (truncated)"
				}
				sb.WriteString(text)
			}
		}
		if content := sb.String(); content != "" {
			result = append(result, uhms.Message{
				Role:    msg.Role,
				Content: content,
			})
		}
	}
	return result
}

// uhmsToChatMessages converts uhms.Message slice back to llmclient.ChatMessage slice.
func uhmsToChatMessages(messages []uhms.Message) []llmclient.ChatMessage {
	result := make([]llmclient.ChatMessage, len(messages))
	for i, msg := range messages {
		result[i] = llmclient.TextMessage(msg.Role, msg.Content)
	}
	return result
}

// resolveOpenCoderConfig 解析 open-coder 子智能体的 provider/model/apiKey/baseURL。
// 三级 fallback: 1) subAgents.openCoder 显式配置 → 2) agents.defaults.model.primary → 3) 硬编码默认值
func resolveOpenCoderConfig(cfg *types.OpenAcosmiConfig) (provider, model, apiKey, baseURL string) {
	// Level 3: 硬编码默认值
	provider = runner.DefaultProvider // "anthropic"
	model = runner.DefaultModel       // "claude-sonnet-4-20250514"

	// Level 2: 主 agent 默认配置
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil &&
		cfg.Agents.Defaults.Model != nil && cfg.Agents.Defaults.Model.Primary != "" {
		primary := cfg.Agents.Defaults.Model.Primary
		if parts := strings.SplitN(primary, "/", 2); len(parts) == 2 {
			provider, model = parts[0], parts[1]
		} else {
			model = primary
		}
	}

	// Level 1: open-coder 显式配置（最高优先级）
	if cfg != nil && cfg.SubAgents != nil && cfg.SubAgents.OpenCoder != nil {
		oc := cfg.SubAgents.OpenCoder
		if oc.Provider != "" {
			provider = oc.Provider
		}
		if oc.Model != "" {
			model = oc.Model
		}
		apiKey = oc.APIKey
		baseURL = oc.BaseURL
	}
	return
}

// resolveArgusConfig 解析灵瞳子智能体的 provider/model/apiKey/baseURL。
// 三级 fallback: 1) subAgents.screenObserver 显式配置 → 2) agents.defaults.model.primary → 3) 硬编码默认值
// Phase 5: 灵瞳完全子智能体化 — 与 resolveOpenCoderConfig 对称。
func resolveArgusConfig(cfg *types.OpenAcosmiConfig) (provider, model, apiKey, baseURL string) {
	// Level 3: 硬编码默认值
	provider = runner.DefaultProvider // "anthropic"
	model = runner.DefaultModel       // "claude-sonnet-4-20250514"

	// Level 2: 主 agent 默认配置
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil &&
		cfg.Agents.Defaults.Model != nil && cfg.Agents.Defaults.Model.Primary != "" {
		primary := cfg.Agents.Defaults.Model.Primary
		if parts := strings.SplitN(primary, "/", 2); len(parts) == 2 {
			provider, model = parts[0], parts[1]
		} else {
			model = primary
		}
	}

	// Level 1: screenObserver 显式配置（最高优先级）
	if cfg != nil && cfg.SubAgents != nil && cfg.SubAgents.ScreenObserver != nil {
		so := cfg.SubAgents.ScreenObserver
		if so.Provider != "" {
			provider = so.Provider
		}
		if so.Model != "" {
			model = so.Model
		}
		apiKey = so.APIKey
		baseURL = so.BaseURL
	}
	return
}

// configToUHMSConfig converts types.MemoryUHMSConfig → uhms.UHMSConfig.
func configToUHMSConfig(c *types.MemoryUHMSConfig) uhms.UHMSConfig {
	cfg := uhms.DefaultUHMSConfig()
	cfg.Enabled = c.Enabled
	if c.DBPath != "" {
		cfg.DBPath = c.DBPath
	}
	if c.VFSPath != "" {
		cfg.VFSPath = c.VFSPath
	}
	if c.VectorMode != "" {
		cfg.VectorMode = uhms.VectorMode(c.VectorMode)
	}
	if c.CompressionThreshold > 0 {
		cfg.CompressionThreshold = c.CompressionThreshold
	}
	if c.DecayEnabled != nil {
		cfg.DecayEnabled = c.DecayEnabled
	}
	if c.DecayIntervalHours > 0 {
		cfg.DecayIntervalHours = c.DecayIntervalHours
	}
	if c.MaxMemories > 0 {
		cfg.MaxMemories = c.MaxMemories
	}
	if c.TieredLoadingEnabled != nil {
		cfg.TieredLoadingEnabled = c.TieredLoadingEnabled
	}
	if c.EmbeddingProvider != "" {
		cfg.EmbeddingProvider = c.EmbeddingProvider
	}
	if c.EmbeddingModel != "" {
		cfg.EmbeddingModel = c.EmbeddingModel
	}
	if c.EmbeddingBaseURL != "" {
		cfg.EmbeddingBaseURL = c.EmbeddingBaseURL
	}
	if c.CompressionTriggerPercent > 0 {
		cfg.CompressionTriggerPercent = c.CompressionTriggerPercent
	}
	if c.ObservationMaskTurns > 0 {
		cfg.ObservationMaskTurns = c.ObservationMaskTurns
	}
	if c.KeepRecentMessages > 0 {
		cfg.KeepRecentMessages = c.KeepRecentMessages
	}
	return cfg
}

// initSearchEngine initializes the core Segment search engine (system collections: skills, plugins, sessions).
// ALWAYS called regardless of vectorMode. Failures are non-fatal (graceful degradation to VFS meta.json scan).
func initSearchEngine(mgr *uhms.DefaultManager, uhmsCfg uhms.UHMSConfig) {
	vectorDataDir := filepath.Join(uhmsCfg.ResolvedVFSPath(), "segment-vectors")
	searchIdx, err := vectoradapter.NewSearchEngine(vectorDataDir)
	if err != nil {
		slog.Warn("gateway: search engine init failed (non-fatal, fallback to VFS scan)", "error", err)
		return
	}
	mgr.SetSearchEngine(searchIdx)
	slog.Info("gateway: core search engine activated (Segment)",
		"collections", len(vectoradapter.SystemCollections))
}

// initUHMSVectorBackend initializes embedding provider + full vector index for memory semantic search.
// Called only when VectorMode != off. Upgrades the search engine with embedding capability.
// Failures are non-fatal (search engine remains active for system collections).
func initUHMSVectorBackend(mgr *uhms.DefaultManager, uhmsCfg uhms.UHMSConfig, fullCfg *types.OpenAcosmiConfig) {
	// 1. Build embedding provider.
	embProvider := uhmsCfg.EmbeddingProvider
	if embProvider == "" || embProvider == "auto" {
		embProvider = "openai" // default
	}

	// Resolve API key from provider config.
	apiKey := ""
	if fullCfg != nil && fullCfg.Models != nil && fullCfg.Models.Providers != nil {
		if pc := fullCfg.Models.Providers[embProvider]; pc != nil {
			apiKey = pc.APIKey
		}
		// Fallback: provider name aliases.
		if apiKey == "" {
			aliases := map[string][]string{
				"gemini": {"google"},
				"qwen":   {"qwen-portal"},
			}
			for _, alias := range aliases[embProvider] {
				if pc := fullCfg.Models.Providers[alias]; pc != nil && pc.APIKey != "" {
					apiKey = pc.APIKey
					break
				}
			}
		}
	}

	embedder, err := vectoradapter.NewHTTPEmbeddingProvider(embProvider, uhmsCfg.EmbeddingModel, uhmsCfg.EmbeddingBaseURL, apiKey)
	if err != nil {
		slog.Warn("gateway: UHMS embedding provider init failed (non-fatal, search engine unaffected)", "error", err)
		return
	}

	// 1b. Probe connectivity (5s timeout, non-fatal).
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer probeCancel()
	if probeErr := embedder.Probe(probeCtx); probeErr != nil {
		slog.Warn("gateway: UHMS embedding probe failed (non-fatal, search engine unaffected)",
			"provider", embedder.Provider(), "model", embedder.Model(), "error", probeErr)
		return
	}
	slog.Info("gateway: UHMS embedding probe OK", "provider", embedder.Provider(), "model", embedder.Model())

	// 2. Build full vector index (memory + system collections with embedding dimension).
	vectorDataDir := filepath.Join(uhmsCfg.ResolvedVFSPath(), "segment-vectors")
	vecIdx, err := vectoradapter.NewSegmentVectorIndex(vectorDataDir, embedder.Dimension())
	if err != nil {
		slog.Warn("gateway: UHMS vector index init failed (non-fatal, search engine unaffected)", "error", err)
		return
	}

	// 3. Upgrade: replace search-only index with full vector backend.
	mgr.SetVectorBackend(vecIdx, embedder)
	slog.Info("gateway: UHMS vector backend activated (search engine upgraded)",
		"mode", uhmsCfg.VectorMode,
		"dimension", embedder.Dimension(),
		"embeddingProvider", embProvider,
	)
}

// autoDistributeSkills 在启动后异步索引技能到搜索引擎。
// 幂等：boot.json 已标记时跳过；DistributeSkills 内部有 content_hash 增量检查。
// 全程 non-fatal：失败仅打日志，不影响启动。
func autoDistributeSkills(mgr *uhms.DefaultManager, bootMgr *uhms.BootManager, cfgLoader *config.ConfigLoader) {
	mgr.SetSkillsDistributing(true)
	defer mgr.SetSkillsDistributing(false)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("autoDistributeSkills: panic (recovered)", "panic", r)
		}
	}()

	vfs := mgr.VFS()
	if vfs == nil {
		slog.Warn("autoDistributeSkills: VFS not available, skipping")
		return
	}

	cfg, err := cfgLoader.LoadConfig()
	if err != nil {
		slog.Warn("autoDistributeSkills: failed to load config", "error", err)
		return
	}

	agentID := scope.ResolveDefaultAgentId(cfg)
	workspaceDir := scope.ResolveAgentWorkspaceDir(cfg, agentID)
	bundledDir := skills.ResolveBundledSkillsDir("")
	entries := skills.LoadSkillEntries(workspaceDir, "", bundledDir, cfg)

	if len(entries) == 0 {
		slog.Info("autoDistributeSkills: no skills found, skipping")
		return
	}

	// 已索引则跳过 — 验证集合中数据量与磁盘技能总数匹配
	if bootMgr.IsSkillsIndexed() {
		vi := mgr.VectorIndex()
		if vi != nil {
			type pointCounter interface {
				PointCount(collection string) (int, error)
			}
			if pc, ok := vi.(pointCounter); ok {
				count, pcErr := pc.PointCount("sys_skills")
				if pcErr == nil && count >= len(entries) {
					slog.Debug("autoDistributeSkills: skills already indexed, skipping",
						"pointCount", count, "diskCount", len(entries))
					return
				}
				if pcErr != nil {
					slog.Info("autoDistributeSkills: PointCount error, re-indexing", "error", pcErr)
				} else {
					slog.Info("autoDistributeSkills: count mismatch, incremental re-index",
						"indexed", count, "available", len(entries))
				}
			} else {
				slog.Debug("autoDistributeSkills: skills already indexed, skipping")
				return
			}
		} else {
			slog.Debug("autoDistributeSkills: skills already indexed, skipping")
			return
		}
	}

	vi := mgr.VectorIndex()
	result, err := skills.DistributeSkills(context.Background(), vfs, vi, entries)
	if err != nil {
		slog.Warn("autoDistributeSkills: distribute failed (non-fatal)", "error", err)
		return
	}

	totalCount := result.Indexed + result.Skipped
	if markErr := bootMgr.MarkSkillsIndexed(totalCount); markErr != nil {
		slog.Warn("autoDistributeSkills: MarkSkillsIndexed failed (non-fatal)", "error", markErr)
	} else {
		slog.Info("autoDistributeSkills: boot mode activated",
			"vfs_written", result.Indexed, "vfs_skipped", result.Skipped,
			"search_upserted", result.SearchUpserted, "total", totalCount)
	}
}

// buildUHMSLLMAdapter constructs an LLM adapter for UHMS from explicit config.
// Returns nil if no provider is configured — UHMS LLM features degrade gracefully.
// Used at boot and by memory.uhms.llm.set RPC for hot-swap.
func buildUHMSLLMAdapter(uhmsCfg *types.MemoryUHMSConfig, fullCfg *types.OpenAcosmiConfig) uhms.LLMProvider {
	if uhmsCfg == nil || uhmsCfg.LLMProvider == "" {
		return nil
	}

	provider := uhmsCfg.LLMProvider
	model := uhmsCfg.LLMModel
	if model == "" {
		model = defaultModelForProvider(provider)
	}
	baseURL := uhmsCfg.LLMBaseURL

	// UHMS 独立 API key 优先, 否则从 agent providers 查找
	apiKey := uhmsCfg.LLMApiKey
	if apiKey == "" && fullCfg != nil && fullCfg.Models != nil && fullCfg.Models.Providers != nil {
		if pc := fullCfg.Models.Providers[provider]; pc != nil {
			apiKey = pc.APIKey
		}
	}

	return &uhms.LLMClientAdapter{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	}
}

// defaultModelForProvider returns a sensible default model for common LLM providers.
func defaultModelForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case "anthropic":
		return "claude-sonnet-4-5-20250514"
	case "openai":
		return "gpt-4o-mini"
	case "ollama":
		return "llama3.2"
	case "deepseek":
		return "deepseek-chat"
	case "google":
		return "gemini-2.0-flash"
	case "groq":
		return "llama-3.3-70b-versatile"
	case "mistral":
		return "mistral-small-latest"
	case "together":
		return "meta-llama/Llama-3.3-70B-Instruct-Turbo"
	case "openrouter":
		return "anthropic/claude-sonnet-4"
	default:
		return "default"
	}
}

// coldStartIdentity 冷启动时注入 system prompt 的身份提示。
// 首次对话（冷启动）时如果用户问候则做完整自我介绍，后续对话不再介绍。
const coldStartIdentity = `
[Cold Start — 首次对话]
这是系统重启后的第一次对话，无历史上下文。
如果用户发来问候（你好/hi/hello），请做一次完整自我介绍（3-5 句话）：
  - 你的名字：创宇太虚（Claw Acismi）
  - 你是什么：运行于 OpenAcosmi 的多模态 AI 代理
  - 你能做什么：截屏分析、执行命令、搜索网页、读写文件、发送消息、管理记忆等
  - 你的接入渠道：飞书、网页、API
  - 用一句话邀请用户告诉你需要做什么
如果用户发来的是任务请求（非问候），直接执行任务，不要自我介绍。`

// buildColdStartInfoFunc 构建冷启动系统简报回调。
// 当 lastSummary 为空（首次对话/重启）时，生成系统状态快照 + 首次问候指令注入 system prompt。
func buildColdStartInfoFunc(state *GatewayState) func() string {
	return func() string {
		var parts []string

		// 1. UHMS 状态
		if mgr := state.UHMSManager(); mgr != nil {
			st := mgr.Status()
			parts = append(parts, fmt.Sprintf("Memory: %d entries, vector=%s (ready=%v)",
				st.MemoryCount, st.VectorMode, st.VectorReady))
		}

		// 2. 活跃频道
		if cm := state.ChannelMgr(); cm != nil {
			snap := cm.GetSnapshot()
			var activeChans []string
			for chID, s := range snap.Channels {
				if s.Status == "running" {
					activeChans = append(activeChans, string(chID))
				}
			}
			if len(activeChans) > 0 {
				parts = append(parts, fmt.Sprintf("Channels: %s", strings.Join(activeChans, ", ")))
			}
		}

		// 3. Argus 可用性
		if state.ArgusBridge() != nil {
			parts = append(parts, "Argus: available")
		}

		// 4. Skills VFS 索引
		if bm := state.UHMSBootMgr(); bm != nil && bm.IsSkillsIndexed() {
			parts = append(parts, "Skills: VFS indexed (boot mode)")
		}

		if len(parts) == 0 {
			return coldStartIdentity
		}
		return fmt.Sprintf("[System Boot Context]\n%s\n%s", strings.Join(parts, "\n"), coldStartIdentity)
	}
}

// StartGatewayServer 启动网关服务。
// 这是核心启动函数，将所有 Phase 0-9 实现的子系统组装起来。
func StartGatewayServer(port int, opts GatewayServerOptions) (*GatewayRuntime, error) {
	slog.Info("gateway: starting", "port", port)

	// ---------- 1. 创建运行时状态 ----------
	state := NewGatewayState()
	state.SetPhase(BootPhaseStarting)

	// ---------- 1b. 启用文件日志 ----------
	applog.EnableFileLogging("")
	logFilePath := applog.DefaultRollingPath()

	// ---------- 2. 解析认证配置 ----------
	auth := ResolveGatewayAuth(nil, "")

	// 校验配置
	bootCfg := BootConfig{
		Server: ServerConfig{
			Host: ResolveGatewayBindHost(opts.BindMode, opts.BindHost),
			Port: port,
		},
		Auth:   auth,
		Reload: DefaultReloadSettings,
	}
	if err := ValidateBootConfig(bootCfg); err != nil {
		// 认证未配置时，允许本地连接（开发模式）
		slog.Warn("gateway: auth config issue (local access still allowed)", "error", err)
	}

	// ---------- 2b. DI 注入: channel dock → autoreply ----------
	// 将 channels 包函数注入到 autoreply/reply DI 变量，
	// 避免 autoreply → channels 循环依赖。
	autoreply.NativeCommandSurfaceProvider = func() []string {
		ids := channels.ListNativeCommandChannels()
		result := make([]string, len(ids))
		for i, id := range ids {
			result[i] = strings.ToLower(string(id))
		}
		return result
	}
	reply.PluginDebounceProvider = func(channelKey string) *int {
		return channels.GetPluginDebounce(channels.ChannelID(channelKey))
	}
	reply.BlockStreamingCoalesceDefaultsProvider = func(channelKey string) (int, int) {
		return channels.GetBlockStreamingCoalesceDefaults(channels.ChannelID(channelKey))
	}

	// ---------- 3. 创建方法注册表 ----------
	registry := NewMethodRegistry()
	storePath := resolveDefaultStorePath()
	sessionStore := NewSessionStore(storePath)

	// 注册会话方法 (对齐 TS server-methods-list.ts)
	registry.RegisterAll(map[string]GatewayMethodHandler{
		"sessions.list":    handleSessionsList,
		"sessions.preview": handleSessionsPreview,
		"sessions.resolve": handleSessionsResolve,
		"sessions.patch":   handleSessionsPatch,
		"sessions.reset":   handleSessionsReset,
		"sessions.delete":  handleSessionsDelete,
		"sessions.compact": handleSessionsCompact,
	})

	// 注册 Batch A 方法 (config/models/agents/agent)
	registry.RegisterAll(ConfigHandlers())
	registry.RegisterAll(ModelsHandlers())
	registry.RegisterAll(AgentsHandlers())
	registry.RegisterAll(AgentHandlers())

	// 注册 Batch C 方法 (channels/logs/system)
	registry.RegisterAll(ChannelsHandlers())
	registry.RegisterAll(LogsHandlers())
	registry.RegisterAll(SystemHandlers())
	registry.RegisterAll(SystemResetHandlers())

	// 注册 Batch D-W1 方法 (cron/tts/skills/node/device/voicewake/update/browser/talk/web)
	registry.RegisterAll(CronHandlers())
	registry.RegisterAll(TtsHandlers())
	registry.RegisterAll(SkillsHandlers())
	registry.RegisterAll(NodeHandlers())
	registry.RegisterAll(DeviceHandlers())
	registry.RegisterAll(VoiceWakeHandlers())
	registry.RegisterAll(UpdateHandlers())
	registry.RegisterAll(BrowserHandlers())
	registry.RegisterAll(TalkHandlers())
	registry.RegisterAll(WebHandlers())

	// 注册 Batch FE-C 方法 (agents.files/sessions.usage/exec.approvals)
	registry.RegisterAll(AgentFilesHandlers())
	registry.RegisterAll(UsageHandlers())
	registry.RegisterAll(ExecApprovalsHandlers())
	registry.RegisterAll(SecurityHandlers())
	registry.RegisterAll(EscalationHandlers())
	registry.RegisterAll(RulesHandlers())          // P3: Allow/Ask/Deny 命令规则 CRUD
	registry.RegisterAll(RemoteApprovalHandlers()) // P4: 远程审批
	registry.RegisterAll(TaskPresetHandlers())     // P5: 任务预设权限
	RegisterSandboxMethods(registry)               // 沙箱配置 + 状态 + 测试
	registry.RegisterAll(ArgusHandlers())          // Argus 视觉子智能体静态方法
	registry.RegisterAll(SubagentHandlers())       // 子智能体状态/控制方法
	registry.RegisterAll(ContractHandlers())       // Phase 8: 合约生命周期仪表盘
	registry.RegisterAll(MCPRemoteHandlers())      // P2: MCP 远程工具方法
	registry.RegisterAll(UHMSHandlers())           // P3: UHMS 记忆系统方法
	registry.RegisterAll(MemoryHandlers())         // memory.* 直接操作方法
	registry.RegisterAll(STTHandlers())            // Phase C: STT 配置方法
	registry.RegisterAll(DocConvHandlers())        // Phase D: 文档转换方法
	registry.RegisterAll(ImageHandlers())          // Phase E: 图片理解 Fallback 方法
	registry.RegisterAll(MediaHandlers())          // Phase 5+6: 媒体子系统方法
	registry.RegisterAll(PluginsHandlers())        // 插件中心
	if state.ArgusBridge() != nil {
		RegisterArgusDynamicMethods(registry, state.ArgusBridge()) // Argus 动态工具方法
	}

	// Fix 2: 兼容旧版前端 exec.approval.resolve → 路由到 EscalationManager.ResolveEscalation
	registry.Register("exec.approval.resolve", func(ctx *MethodHandlerContext) {
		mgr := ctx.Context.EscalationMgr
		if mgr == nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "escalation manager not available"))
			return
		}

		id, _ := ctx.Params["id"].(string)
		decision, _ := ctx.Params["decision"].(string)
		if decision == "" {
			// 兼容 approve bool 参数
			if approve, ok := ctx.Params["approve"].(bool); ok {
				if approve {
					decision = "allow-once"
				} else {
					decision = "deny"
				}
			}
		}

		approved := decision == "allow-once" || decision == "allow-always"
		ttl := 30
		if decision == "allow-always" {
			ttl = 60
		}
		if ttlRaw, ok := ctx.Params["ttlMinutes"].(float64); ok && ttlRaw > 0 {
			ttl = int(ttlRaw)
		}

		// 验证 ID 匹配（如果提供了 ID）
		if id != "" {
			pendingID := mgr.GetPendingID()
			if pendingID != "" && pendingID != id {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "escalation ID mismatch"))
				return
			}
		}

		if err := mgr.ResolveEscalation(approved, ttl); err != nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, err.Error()))
			return
		}
		ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
	})

	// Coder 确认流 RPC
	registry.Register("coder.confirm.resolve", func(ctx *MethodHandlerContext) {
		id, _ := ctx.Params["id"].(string)
		decision, _ := ctx.Params["decision"].(string)
		if id == "" || decision == "" {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing id or decision"))
			return
		}
		if ctx.Context.CoderConfirmMgr == nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "coder confirmation not enabled"))
			return
		}
		if err := ctx.Context.CoderConfirmMgr.ResolveConfirmation(id, decision); err != nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, err.Error()))
			return
		}
		ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
	})

	// Phase 1: 方案确认 RPC
	registry.RegisterAll(PlanConfirmHandlers())

	// Phase 3: 结果签收 RPC
	registry.RegisterAll(ResultApprovalHandlers())

	// Phase 4: 子智能体求助 RPC
	registry.RegisterAll(SubagentHelpHandlers())

	// 注册 Batch B 方法 (chat/send/agent)
	registry.RegisterAll(ChatHandlers())
	registry.RegisterAll(SendHandlers())
	registry.RegisterAll(AgentRPCHandlers())

	// 注册 health 方法
	registry.Register("health", func(ctx *MethodHandlerContext) {
		ctx.Respond(true, GetHealthStatus(state, cli.Version), nil)
	})

	// 注册 status 方法 (精简版)
	registry.Register("status", func(ctx *MethodHandlerContext) {
		ctx.Respond(true, map[string]interface{}{
			"phase":   string(state.Phase()),
			"version": cli.Version,
			"clients": state.Broadcaster().ClientCount(),
		}, nil)
	})

	// ---------- 4. 创建配置加载器和模型目录 ----------
	cfgLoader := config.NewConfigLoader()
	modelCatalog := models.NewModelCatalog()

	// 尝试加载配置以填充模型目录
	var loadedCfg *types.OpenAcosmiConfig
	if cfg, err := cfgLoader.LoadConfig(); err == nil {
		loadedCfg = cfg
	}

	// ---------- 4a-wizard. 注册 Wizard 方法（替代 stub） ----------
	wizardTracker := NewWizardSessionTracker()
	registry.RegisterAll(WizardHandlers(WizardHandlerDeps{
		Tracker:      wizardTracker,
		ConfigLoader: cfgLoader,
		ModelCatalog: modelCatalog,
		State:        state,
	}))

	// ---------- 4b. Batch C 基础设施 ----------
	presenceStore := NewSystemPresenceStore()
	heartbeatState := NewHeartbeatState()
	eventQueue := NewSystemEventQueue()

	// ---------- 4d. 创建 AttemptRunner ----------
	// 注意: attemptRunner 的 Config 字段在 dispatcher 中动态刷新
	// 构建 Argus Bridge 适配器（nil-safe: bridge 不可用时 adapter 也为 nil）
	var argusBridgeForAgent runner.ArgusBridgeForAgent
	if ab := state.ArgusBridge(); ab != nil {
		argusBridgeForAgent = &argusBridgeAdapter{bridge: ab}
	}

	// (Phase 2A: Coder Bridge adapter 已删除 — oa-coder 升级为 spawn_coder_agent)

	// 构建 NativeSandbox 适配器（NativeSandboxRouter: L1→Worker IPC, L2→一次性 CLI）
	// workspace 由 AttemptRunner 的 buildToolExecParams 动态设置 (AttemptParams.WorkspaceDir),
	// Router 层使用 /tmp 作为默认（与 boot.go 一致）。
	var nativeSandboxForAgent runner.NativeSandboxForAgent
	if nsb := state.NativeSandboxBridge(); nsb != nil {
		binaryPath := resolveNativeSandboxBinaryPath()
		router := sandbox.NewNativeSandboxRouter(nsb, binaryPath, os.TempDir())
		nativeSandboxForAgent = &nativeSandboxRouterAdapter{router: router}
	}

	// 构建 UHMS Bridge 适配器（nil-safe: manager 不可用时 adapter 也为 nil）
	// 如果 UHMS 配置启用但 boot 时未传 LLM，在此处注入 LLM adapter
	var uhmsBridgeForAgent runner.UHMSBridgeForAgent
	var uhmsBootMgr *uhms.BootManager
	if mgr := state.UHMSManager(); mgr != nil {
		uhmsBootMgr = uhms.NewBootManager(mgr.Config().ResolvedBootFilePath())
		uhmsBootMgr.Load()
		uhmsBridgeForAgent = &uhmsBridgeAdapter{mgr: mgr, broadcaster: state.Broadcaster(), bootMgr: uhmsBootMgr, coldStartInfo: buildColdStartInfoFunc(state)}

		// Ensure search engine is always available (even if boot.go didn't init vector).
		if mgr.VectorIndex() == nil {
			initSearchEngine(mgr, *mgr.Config())
		}
	} else if loadedCfg != nil && loadedCfg.Memory != nil && loadedCfg.Memory.UHMS != nil && loadedCfg.Memory.UHMS.Enabled {
		// boot.go 使用 DefaultUHMSConfig 初始化（默认 disabled），
		// 这里从真实配置读取并重新初始化
		uhmsCfg := configToUHMSConfig(loadedCfg.Memory.UHMS)

		// 构建 LLM adapter: 优先使用 UHMS 独立配置，fallback 到 agent provider
		llmProvider := buildUHMSLLMAdapter(loadedCfg.Memory.UHMS, loadedCfg)

		mgr, err := uhms.NewManager(uhmsCfg, llmProvider)
		if err != nil {
			slog.Warn("gateway: UHMS init from config failed (non-fatal)", "error", err)
		} else {
			// 如果 LLM provider 是 Anthropic，注入 Compaction API client
			if llmProvider != nil {
				if adapter, ok := llmProvider.(*uhms.LLMClientAdapter); ok && strings.ToLower(adapter.Provider) == "anthropic" {
					mgr.SetCompactionClient(&uhms.AnthropicCompactionClient{
						APIKey: adapter.APIKey,
					})
					slog.Info("gateway: UHMS Anthropic Compaction API enabled")
				}
			}
			state.SetUHMSManager(mgr)
			uhmsBootMgr = uhms.NewBootManager(uhmsCfg.ResolvedBootFilePath())
			uhmsBootMgr.Load()
			uhmsBridgeForAgent = &uhmsBridgeAdapter{mgr: mgr, broadcaster: state.Broadcaster(), bootMgr: uhmsBootMgr, coldStartInfo: buildColdStartInfoFunc(state)}

			// ALWAYS: Init core search engine (system collections).
			initSearchEngine(mgr, uhmsCfg)

			// OPTIONAL: Init memory vector backend (requires embedding API).
			if uhmsCfg.VectorMode != uhms.VectorOff {
				initUHMSVectorBackend(mgr, uhmsCfg, loadedCfg)
			}

			slog.Info("gateway: UHMS memory system initialized from config",
				"vectorMode", uhmsCfg.VectorMode,
				"vfsPath", uhmsCfg.ResolvedVFSPath(),
			)
		}
	}

	// 持久化 BootManager 到 GatewayState，供 skills.distribute 使用
	if uhmsBootMgr != nil {
		state.SetUHMSBootMgr(uhmsBootMgr)
	}

	// Phase 5: 合约 VFS 持久化初始化（依赖 UHMS VFS）
	if vfs := state.UHMSVFS(); vfs != nil {
		state.contractStore = NewVFSContractPersistence(vfs)
		slog.Info("gateway: contract VFS persistence initialized")

		// TTL 清理 goroutine: 每小时清理 >24h 的已完成合约
		state.contractCleanupDone = make(chan struct{})
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-state.contractCleanupDone:
					return
				case <-ticker.C:
					if n, err := state.contractStore.CleanupCompleted(24 * time.Hour); err != nil {
						slog.Warn("contract: TTL cleanup error", "error", err)
					} else if n > 0 {
						slog.Info("contract: TTL cleanup done", "deleted", n)
					}
				}
			}
		}()
	}

	// 启动后台 goroutine 自动索引技能到搜索引擎（首次启动或 boot.json 缺失时）
	if mgr := state.UHMSManager(); mgr != nil && uhmsBootMgr != nil {
		go autoDistributeSkills(mgr, uhmsBootMgr, cfgLoader)
	}

	// 浏览器控制器初始化（CDP 直连模式）
	var browserControllerForAgent tools.BrowserController
	browserEvaluateEnabled := true // 默认允许 JS 执行
	if loadedCfg != nil && loadedCfg.Browser != nil {
		enabled := loadedCfg.Browser.Enabled == nil || *loadedCfg.Browser.Enabled
		if enabled {
			cdpURL := resolveBrowserCdpURL(loadedCfg.Browser)
			if cdpURL != "" {
				cdpTools := browser.NewCDPPlaywrightTools(cdpURL, slog.Default())
				browserControllerForAgent = browser.NewPlaywrightBrowserController(cdpTools, cdpURL)
				slog.Info("browser controller configured for agent", "cdpURL", cdpURL)
			}
		}
		if loadedCfg.Browser.EvaluateEnabled != nil {
			browserEvaluateEnabled = *loadedCfg.Browser.EvaluateEnabled
		}
	}

	// 从配置读取 Argus 审批模式
	var argusApprovalMode string
	if loadedCfg != nil && loadedCfg.SubAgents != nil && loadedCfg.SubAgents.ScreenObserver != nil {
		argusApprovalMode = loadedCfg.SubAgents.ScreenObserver.ApprovalMode
	}

	attemptRunner := &runner.EmbeddedAttemptRunner{
		Config:      loadedCfg,           // 初始值，dispatch 时会被热更新
		AuthStore:   nil,                 // Phase 10 暂不集成，回退到环境变量
		ArgusBridge: argusBridgeForAgent, // Argus 视觉工具注入
		// (Phase 2A: CoderBridge 已删除 — oa-coder 升级为 spawn_coder_agent)
		NativeSandbox:          nativeSandboxForAgent,     // 原生沙箱 Worker 注入
		UHMSBridge:             uhmsBridgeForAgent,        // UHMS 记忆系统注入
		CoderConfirmation:      state.CoderConfirmMgr(),   // Coder 确认流注入
		PlanConfirmation:       state.PlanConfirmMgr(),    // Phase 1: 方案确认门控注入
		ResultApprovalMgr:      state.ResultApprovalMgr(), // Phase 3: 结果签收门控注入
		BrowserController:      browserControllerForAgent, // 浏览器工具注入
		BrowserEvaluateEnabled: browserEvaluateEnabled,    // JS 执行开关
		ArgusApprovalMode:      argusApprovalMode,         // Argus 审批模式
		ContractStore:          state.contractStore,       // Phase 6: 合约持久化注入
		// RemoteMCPBridge 在 4h 节之后注入
	}

	// MediaSender 注入 — 有 ChannelMgr 时自动启用 send_media 工具
	if channelMgr := state.ChannelMgr(); channelMgr != nil {
		attemptRunner.MediaSender = &mediaSenderAdapter{channelMgr: channelMgr}
		slog.Info("media sender configured for agent")
	}

	// MediaSubsystem 注入 — 媒体工具子系统（可选，初始化失败不影响主流程）
	var mediaSub *media.MediaSubsystem
	{
		mediaWorkspace := config.ResolveStateDir()
		var mErr error
		mediaSub, mErr = media.NewMediaSubsystem(media.MediaSubsystemConfig{
			Workspace:      mediaWorkspace,
			EnablePublish:  true,
			EnableInteract: true,
		})
		if mErr != nil {
			slog.Warn("media subsystem init failed (non-fatal)", "error", mErr)
			mediaSub = nil
		} else {
			attemptRunner.MediaSubsystem = mediaSub
			slog.Info("media subsystem configured for agent", "tools", mediaSub.ToolNames())
		}
	}

	// 注入联网搜索 Provider（博查搜索）
	if loadedCfg != nil && loadedCfg.Tools != nil && loadedCfg.Tools.Web != nil && loadedCfg.Tools.Web.Search != nil {
		searchCfg := loadedCfg.Tools.Web.Search
		if searchCfg.Bocha != nil && searchCfg.Bocha.APIKey != "" {
			enabled := searchCfg.Bocha.Enabled == nil || *searchCfg.Bocha.Enabled // default true
			if enabled {
				attemptRunner.WebSearchProvider = &bochaSearchAdapter{
					provider: tools.NewBochaSearchProvider(
						searchCfg.Bocha.APIKey,
						searchCfg.Bocha.BaseURL,
					),
				}
				slog.Info("web search provider configured", "provider", "bocha")
			}
		}
	}

	// SpawnSubagent 回调注入 — spawn_coder_agent 工具通过此回调启动子 LLM session。
	// 闭包捕获 cfgLoader/modelCatalog/attemptRunner，与 pipelineDispatcher 共享依赖。
	attemptRunner.SpawnSubagent = func(ctx context.Context, sp runner.SpawnSubagentParams) (*runner.SubagentRunOutcome, error) {
		childSessionID := fmt.Sprintf("spawn-%d", time.Now().UnixNano())
		// 子智能体对话频道 session key: coder:<contractID>
		childSessionKey := fmt.Sprintf("coder:%s", sp.Contract.ContractID)

		// 获取广播器
		coderBc := state.Broadcaster()

		// 注册子智能体对话频道 session
		if sessionStore != nil {
			entry := sessionStore.LoadSessionEntry(childSessionKey)
			if entry == nil {
				taskBrief := sp.Label
				if taskBrief == "" && len(sp.Task) > 60 {
					taskBrief = sp.Task[:60] + "..."
				} else if taskBrief == "" {
					taskBrief = sp.Task
				}
				entry = &SessionEntry{
					SessionKey: childSessionKey,
					SessionId:  childSessionID,
					Label:      taskBrief,
					Channel:    "coder",
					ChatType:   "",
					CreatedAt:  time.Now().UnixMilli(),
					UpdatedAt:  time.Now().UnixMilli(),
					SpawnedBy:  sp.Contract.IssuedBy,
				}
				sessionStore.Save(entry)
				slog.Info("coder channel: created session", "sessionKey", childSessionKey, "label", taskBrief)
			}
		}

		// 广播子智能体频道开启通知
		coderBc.Broadcast("channel.message.incoming", map[string]interface{}{
			"sessionKey": childSessionKey,
			"channel":    "coder",
			"text":       fmt.Sprintf("[Open Coder] 任务开始: %s", sp.Label),
			"from":       "system",
			"ts":         time.Now().UnixMilli(),
		}, nil)

		// Phase 5: 启动前持久化合约到 active/
		if state.contractStore != nil {
			sp.Contract.Status = runner.ContractActive
			if err := state.contractStore.SaveContract(sp.Contract, nil); err != nil {
				slog.Warn("contract: failed to persist active contract (non-fatal)", "contractID", sp.Contract.ContractID, "error", err)
			}
		}

		// Phase 5: 根据 AgentType 确定子智能体标签（在 goroutine 之前声明供其捕获）
		subDisplayName := "Open Coder"
		subLogPrefix := "spawn_coder_agent"
		subChannelLabel := "coder"
		if sp.AgentType == "argus" {
			subDisplayName = "灵瞳"
			subLogPrefix = "spawn_argus_agent"
			subChannelLabel = "argus"
		} else if sp.AgentType == "media" {
			subDisplayName = "媒体运营"
			subLogPrefix = "spawn_media_agent"
			subChannelLabel = "media"
		}

		// Phase 4: 创建异步消息通道
		helpChannel := runner.NewAgentChannel()
		defer helpChannel.Close()

		// Phase 4: 启动 toParent 监听 goroutine — 处理子智能体求助请求
		go func() {
			for msg := range helpChannel.ToParentChan() {
				switch msg.Type {
				case runner.MsgHelpRequest:
					slog.Info("subagent help request received",
						"contractID", sp.Contract.ContractID,
						"msgID", msg.ID,
						"urgency", msg.Urgency,
					)
					// 注册到 help channel 查找表（供 subagent.help.resolve RPC 使用）
					state.RegisterHelpChannel(msg.ID, helpChannel, sp.Contract.ContractID, sp.Label)
					// 广播到前端
					coderBc.Broadcast("subagent.help.requested", map[string]interface{}{
						"id":         msg.ID,
						"contractId": sp.Contract.ContractID,
						"question":   msg.Content,
						"context":    msg.Context,
						"options":    msg.Options,
						"urgency":    msg.Urgency,
						"label":      sp.Label,
						"ts":         msg.Timestamp,
					}, nil)
				case runner.MsgStatusUpdate:
					slog.Debug("subagent status update",
						"contractID", sp.Contract.ContractID,
						"content", msg.Content,
					)
					// 状态更新广播到频道
					coderBc.Broadcast("channel.message.incoming", map[string]interface{}{
						"sessionKey": childSessionKey,
						"channel":    subChannelLabel,
						"text":       fmt.Sprintf("[%s] %s", subDisplayName, msg.Content),
						"from":       subDisplayName,
						"ts":         msg.Timestamp,
					}, nil)
				}
			}
		}()

		// 热加载最新配置
		currentCfg := loadedCfg
		if freshCfg, err := cfgLoader.LoadConfig(); err == nil {
			currentCfg = freshCfg
		}

		// 超时 context
		timeoutMs := sp.TimeoutMs
		if timeoutMs <= 0 {
			timeoutMs = 60000
		}
		childCtx, childCancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer childCancel()

		slog.Info(subLogPrefix+": launching sub-agent session",
			"contractID", sp.Contract.ContractID,
			"childSessionID", childSessionID,
			"timeoutMs", timeoutMs,
			"label", sp.Label,
			"agentType", sp.AgentType,
		)

		// Phase 5: 按 AgentType 分流模型配置解析
		var subProvider, subModel, subAPIKey, subBaseURL string
		if sp.AgentType == "argus" {
			subProvider, subModel, subAPIKey, subBaseURL = resolveArgusConfig(currentCfg)
		} else {
			// coder / media / 其他 — 统一使用 Open Coder 配置
			subProvider, subModel, subAPIKey, subBaseURL = resolveOpenCoderConfig(currentCfg)
		}

		// 如果子智能体有独立 API key/baseURL，注入到配置副本
		if subAPIKey != "" || subBaseURL != "" {
			cfgCopy := *currentCfg
			if cfgCopy.Models == nil {
				cfgCopy.Models = &types.ModelsConfig{}
			}
			if cfgCopy.Models.Providers == nil {
				cfgCopy.Models.Providers = make(map[string]*types.ModelProviderConfig)
			}
			provCfg := cfgCopy.Models.Providers[subProvider]
			if provCfg == nil {
				provCfg = &types.ModelProviderConfig{}
			} else {
				// 浅拷贝避免修改原始配置
				cp := *provCfg
				provCfg = &cp
			}
			if subAPIKey != "" {
				provCfg.APIKey = subAPIKey
			}
			if subBaseURL != "" {
				provCfg.BaseURL = subBaseURL
			}
			cfgCopy.Models.Providers[subProvider] = provCfg
			currentCfg = &cfgCopy
		}

		slog.Info(subLogPrefix+": resolved config",
			"provider", subProvider,
			"model", subModel,
			"hasAPIKey", subAPIKey != "",
			"hasBaseURL", subBaseURL != "",
		)

		result, err := runner.RunEmbeddedPiAgent(childCtx, runner.RunEmbeddedPiAgentParams{
			SessionID:          childSessionID,
			SessionKey:         childSessionKey,
			Prompt:             sp.Task,
			Provider:           subProvider,
			Model:              subModel,
			TimeoutMs:          timeoutMs,
			ExtraSystemPrompt:  sp.SystemPrompt,
			Config:             currentCfg,
			DelegationContract: sp.Contract,
			AgentChannel:       helpChannel,  // Phase 4: 异步消息通道
			AgentType:          sp.AgentType, // 传递子智能体类型（media/coder/argus）
			PromptMode:         "minimal",    // 子智能体不需要 Self-Update/Messaging/Voice 等段落
			// 子智能体对话频道广播回调
			OnCoderEvent: func(event string, data map[string]interface{}) {
				text := ""
				switch event {
				case "task_received":
					if p, ok := data["prompt"].(string); ok {
						text = fmt.Sprintf("[%s] 收到任务: %s", subDisplayName, truncateForLog(p, 100))
					}
				case "turn_complete":
					if t, ok := data["text"].(string); ok {
						text = truncateForLog(t, 200)
					}
				}
				if text != "" {
					coderBc.Broadcast("channel.message.incoming", map[string]interface{}{
						"sessionKey": childSessionKey,
						"channel":    subChannelLabel,
						"text":       text,
						"from":       subDisplayName,
						"ts":         time.Now().UnixMilli(),
					}, nil)
				}
			},
			// 子智能体频道结构化工具事件广播
			OnToolEvent: func(event runner.ToolEvent) {
				var text string
				switch event.Phase {
				case "start":
					text = fmt.Sprintf("[工具] %s: %s", event.ToolName, event.Args)
				case "end":
					if event.IsError {
						text = fmt.Sprintf("[错误] %s (%dms)", event.Result, event.Duration)
					} else {
						text = fmt.Sprintf("[结果] %s (%dms)", event.Result, event.Duration)
					}
				}
				if text != "" {
					coderBc.Broadcast("channel.message.incoming", map[string]interface{}{
						"sessionKey": childSessionKey,
						"channel":    subChannelLabel,
						"text":       truncateForLog(text, 300),
						"from":       subDisplayName,
						"ts":         time.Now().UnixMilli(),
					}, nil)
				}
			},
		}, runner.EmbeddedRunDeps{
			AttemptRunner: attemptRunner,
			ModelResolver: &runner.EnvModelResolver{Catalog: modelCatalog},
		})
		if err != nil {
			return &runner.SubagentRunOutcome{
				Status: "error",
				Error:  err.Error(),
			}, nil
		}

		// 提取最后一条文本回复
		var lastReply string
		for i := len(result.Payloads) - 1; i >= 0; i-- {
			if result.Payloads[i].Text != "" {
				lastReply = result.Payloads[i].Text
				break
			}
		}

		// 解析 ThoughtResult（子智能体结构化返回）
		tr := runner.ParseThoughtResult(lastReply)

		outcome := &runner.SubagentRunOutcome{
			Status:        "ok",
			ThoughtResult: tr,
		}
		if result.Meta.Aborted {
			outcome.Status = "timeout"
		}
		if result.Meta.Error != nil {
			outcome.Status = "error"
			outcome.Error = result.Meta.Error.Message
		}

		slog.Info(subLogPrefix+": sub-agent session completed",
			"contractID", sp.Contract.ContractID,
			"status", outcome.Status,
			"hasThoughtResult", tr != nil,
		)

		// 广播子智能体频道完成通知
		{
			statusText := outcome.Status
			if tr != nil && tr.Status != "" {
				statusText = string(tr.Status)
			}
			completionText := fmt.Sprintf("[%s] 任务完成 (%s)", subDisplayName, statusText)
			if outcome.Status == "error" && outcome.Error != "" {
				completionText = fmt.Sprintf("[%s] 任务失败: %s", subDisplayName, truncateForLog(outcome.Error, 100))
			}
			coderBc.Broadcast("channel.message.incoming", map[string]interface{}{
				"sessionKey": childSessionKey,
				"channel":    subChannelLabel,
				"text":       completionText,
				"from":       "system",
				"ts":         time.Now().UnixMilli(),
			}, nil)
			// 更新 session 时间戳
			if sessionStore != nil {
				if entry := sessionStore.LoadSessionEntry(childSessionKey); entry != nil {
					entry.UpdatedAt = time.Now().UnixMilli()
					sessionStore.Save(entry)
				}
			}
		}

		// Phase 5: 完成后转换合约状态
		if state.contractStore != nil {
			var targetStatus runner.ContractStatus
			switch {
			case tr != nil && tr.Status == runner.ThoughtNeedsAuth:
				targetStatus = runner.ContractSuspended
			case outcome.Status == "ok":
				targetStatus = runner.ContractCompleted
			default: // error, timeout
				targetStatus = runner.ContractFailed
			}

			// 暂停时保存 ThoughtResult 到 l2（供恢复时读取）
			if targetStatus == runner.ContractSuspended && tr != nil {
				sp.Contract.Status = targetStatus
				if err := state.contractStore.SaveContract(sp.Contract, tr); err != nil {
					slog.Warn("contract: failed to persist suspended contract", "contractID", sp.Contract.ContractID, "error", err)
				}
				// 删除旧 active 条目
				_ = state.contractStore.DeleteEntry(runner.ContractActive, sp.Contract.ContractID)
			} else {
				if err := state.contractStore.TransitionStatus(sp.Contract.ContractID, runner.ContractActive, targetStatus); err != nil {
					slog.Warn("contract: status transition failed (non-fatal)", "contractID", sp.Contract.ContractID, "to", targetStatus, "error", err)
				}
			}
		}

		// Phase 4: 清理该合约的所有 help channel 映射 + 通知前端清除弹窗
		removedHelpIDs := state.CleanupAgentChannels(sp.Contract.ContractID)
		for _, helpID := range removedHelpIDs {
			coderBc.Broadcast("subagent.help.resolved", map[string]interface{}{
				"id":       helpID,
				"response": "[sub-agent completed]",
				"ts":       time.Now().UnixMilli(),
			}, nil)
		}

		return outcome, nil
	}

	// ---------- 4e. 创建真实 PipelineDispatcher（内联实现，避免循环导入） ----------
	// 这是 DI 回调函数，`chat.send` → `DispatchInboundMessage` → `GetReplyFromConfig`
	// 每次调用时从 cfgLoader 热加载最新配置，确保向导保存的配置立即生效。
	pipelineDispatcher := func(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions) ([]autoreply.ReplyPayload, error) {
		// 0. 热加载最新配置（向导保存后立即生效）
		currentCfg := loadedCfg
		if freshCfg, err := cfgLoader.LoadConfig(); err == nil {
			currentCfg = freshCfg
			// 同步更新 attemptRunner 的配置引用
			attemptRunner.Config = currentCfg
		} else {
			slog.Warn("pipeline: config reload failed, using cached", "error", err)
		}

		// 1. 创建 AgentExecutor: ModelFallbackExecutor 需要 RunnerDeps + Config
		executor := &reply.ModelFallbackExecutor{
			RunnerDeps: runner.EmbeddedRunDeps{
				AttemptRunner: attemptRunner,
				ModelResolver: &runner.EnvModelResolver{Catalog: modelCatalog},
				// AuthStore, CompactionRunner 暂留 nil — fallback 到默认行为
			},
			Config: currentCfg,
			OnPermissionDenied: func(tool, detail string) {
				bc := state.Broadcaster()
				if bc != nil {
					// 读取安全等级（与前端 PermissionDeniedEvent 契约一致）
					level := "deny"
					if currentCfg != nil && currentCfg.Tools != nil && currentCfg.Tools.Exec != nil && currentCfg.Tools.Exec.Security != "" {
						level = currentCfg.Tools.Exec.Security
					}
					bc.Broadcast("permission_denied", map[string]interface{}{
						"tool":   tool,
						"detail": detail,
						"level":  level,
						"ts":     time.Now().UnixMilli(),
					}, nil)
				}

				// 自动触发提权请求（幂等，重复调用安全）
				// 四级安全模型: bash/write/edit 需要 L2(sandboxed)，其他工具 L1(allowlist)
				escMgr := state.EscalationMgr()
				if escMgr != nil {
					escLevel := "allowlist"
					if tool == "bash" || tool == "write_file" || tool == "write" || tool == "edit" {
						escLevel = "sandboxed"
					}
					// Design Fix 2: effective level 已满足则跳过提权
					effectiveLevel := escMgr.GetEffectiveLevel()
					if infra.LevelOrder(infra.ExecSecurity(effectiveLevel)) >= infra.LevelOrder(infra.ExecSecurity(escLevel)) {
						slog.Debug("auto-escalation skipped: effective level already sufficient",
							"effective", effectiveLevel, "needed", escLevel)
					} else {
						reason := fmt.Sprintf("工具 %s 需要权限: %s", tool, truncateStr(detail, 200))
						escId := fmt.Sprintf("esc_auto_%d", time.Now().UnixNano())
						if err := escMgr.RequestEscalation(escId, escLevel, reason, "", "", msgCtx.ChannelID, msgCtx.SenderID, 30); err != nil {
							slog.Debug("auto-escalation skipped (expected if already pending)", "error", err)
						}
					}
				}
			},
			// WaitForApproval 阻塞等待提权审批。
			// 轮询 EscalationManager 状态，直到出现 active grant（审批通过）、
			// pending 清除（被拒绝/超时），或 ctx 取消。
			WaitForApproval: func(ctx context.Context) bool {
				escMgr := state.EscalationMgr()
				if escMgr == nil {
					return false
				}
				const pollInterval = 2 * time.Second
				const maxWait = 10 * time.Minute
				deadline := time.After(maxWait)
				ticker := time.NewTicker(pollInterval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return false
					case <-deadline:
						slog.Warn("WaitForApproval: max wait exceeded")
						return false
					case <-ticker.C:
						status := escMgr.GetStatus()
						if status.HasActive {
							// 审批已通过
							return true
						}
						if !status.HasPending && !status.HasActive {
							// 既无 pending 也无 active — 被拒绝或超时
							return false
						}
						// 仍有 pending — 继续等待
					}
				}
			},
			// SecurityLevelFunc 动态返回有效安全级别（含临时提权）
			SecurityLevelFunc: func() string {
				escMgr := state.EscalationMgr()
				if escMgr != nil {
					return escMgr.GetEffectiveLevel()
				}
				// fallback 到静态配置（需规范化，防止 "Sandbox"/"OFF" 等非规范值）
				if currentCfg != nil && currentCfg.Tools != nil && currentCfg.Tools.Exec != nil {
					sec := strings.ToLower(strings.TrimSpace(currentCfg.Tools.Exec.Security))
					switch sec {
					case "full":
						return "full"
					case "sandboxed":
						return "sandboxed"
					case "allowlist", "sandbox":
						return "allowlist"
					default:
						return "deny"
					}
				}
				return "deny"
			},
			// MountRequestsFunc 动态返回活跃 grant 的临时挂载请求（Phase 3.4）
			MountRequestsFunc: func() []runner.MountRequestForSandbox {
				escMgr := state.EscalationMgr()
				if escMgr == nil {
					return nil
				}
				mounts := escMgr.GetActiveMountRequests()
				if len(mounts) == 0 {
					return nil
				}
				result := make([]runner.MountRequestForSandbox, len(mounts))
				for i, m := range mounts {
					result[i] = runner.MountRequestForSandbox{HostPath: m.HostPath, MountMode: m.MountMode}
				}
				return result
			},
		}

		// 1b. 注入 OnToolEvent（per-request，通过 opts 传递避免循环导入）
		if opts != nil && opts.OnToolEvent != nil {
			if fn, ok := opts.OnToolEvent.(func(runner.ToolEvent)); ok {
				executor.OnToolEvent = fn
			} else {
				slog.Warn("pipeline: OnToolEvent type assertion failed, tool events will not be broadcast")
			}
		}

		// 2. 构建 reply 层选项
		agentId := scope.ResolveDefaultAgentId(currentCfg)
		wsDir := scope.ResolveAgentWorkspaceDir(currentCfg, agentId)
		replyOpts := &reply.GetReplyOptions{
			AgentExecutor: executor,
			WorkspaceDir:  wsDir,
			SessionID:     msgCtx.SessionID,
			SessionKey:    msgCtx.SessionKey,
			StorePath:     storePath,
		}
		return reply.GetReplyFromConfig(ctx, msgCtx, opts, replyOpts)
	}

	// ---------- 4c. 初始化中国频道插件 ----------
	// Phase 5: 从 config 读取频道配置，初始化并启动已配置的频道插件
	channelMgr := state.ChannelMgr()
	if !config.SkipChannels && loadedCfg != nil && loadedCfg.Channels != nil {
		// 注册插件
		if loadedCfg.Channels.Feishu != nil {
			feishuPlugin := feishu.NewFeishuPlugin(loadedCfg)

			// feishuDispatchResult 飞书分发结果（文本 + 可选媒体）。
			type feishuDispatchResult struct {
				Text          string
				MediaBase64   string
				MediaMimeType string
			}

			// 公共飞书消息分发逻辑（DispatchFunc 和 DispatchMultimodalFunc 共用）
			// imageBase64/imageMimeType: 附件图片数据（仅 DispatchMultimodalFunc 传入）
			feishuDispatch := func(channel, accountID, chatID, userID, text, imageBase64, imageMimeType string) feishuDispatchResult {
				// 持久化飞书目标 ID，确保重启后审批通知仍可送达
				if rn := state.RemoteApprovalNotifier(); rn != nil && chatID != "" {
					rn.UpdateLastKnownFeishuTarget(chatID, userID)
				}

				sessionKey := fmt.Sprintf("feishu:%s", chatID)

				// ===== 步骤 1: 确保 session 注册到 SessionStore =====
				var resolvedSessionId string
				if sessionStore != nil {
					entry := sessionStore.LoadSessionEntry(sessionKey)
					if entry == nil {
						newId := fmt.Sprintf("session_%d", time.Now().UnixNano())
						entry = &SessionEntry{
							SessionKey: sessionKey,
							SessionId:  newId,
							Label:      fmt.Sprintf("飞书:%s", chatID),
							Channel:    "feishu",
						}
						sessionStore.Save(entry)
						slog.Info("feishu: auto-created session", "sessionKey", sessionKey, "sessionId", newId)
					}
					resolvedSessionId = entry.SessionId
					sessionStore.RecordSessionMeta(sessionKey, InboundMeta{
						Channel:     "feishu",
						DisplayName: userID,
					})
				}

				// 用户消息 transcript 由 attempt_runner.persistToTranscript 写入，此处不再双写

				msgCtx := &autoreply.MsgContext{
					Body:        text,
					ChannelType: channel,
					ChannelID:   chatID,
					SenderID:    userID,
					AccountID:   accountID,
					SessionID:   resolvedSessionId,
					SessionKey:  sessionKey,
				}

				// 广播用户消息到前端（让聊天页面能看到飞书会话）
				bc := state.Broadcaster()
				if bc != nil {
					ts := time.Now().UnixMilli()
					userPayload := map[string]interface{}{
						"sessionKey": sessionKey,
						"channel":    "feishu",
						"role":       "user",
						"text":       text,
						"from":       userID,
						"chatId":     chatID,
						"ts":         ts,
					}
					// 附加图片数据（飞书发来的图片，前端可直接显示）
					if imageBase64 != "" {
						userPayload["mediaBase64"] = imageBase64
						userPayload["mediaMimeType"] = imageMimeType
					}
					bc.Broadcast("chat.message", userPayload, nil)

					// 跨会话通知：让所有前端客户端感知到飞书消息
					bc.Broadcast("channel.message.incoming", map[string]interface{}{
						"sessionKey": sessionKey,
						"channel":    "feishu",
						"text":       truncateStr(text, 100),
						"from":       userID,
						"label":      fmt.Sprintf("飞书:%s", chatID),
						"ts":         ts,
					}, nil)
				}

				result := DispatchInboundMessage(context.Background(), DispatchInboundParams{
					MsgCtx:     msgCtx,
					SessionKey: sessionKey,
					Dispatcher: pipelineDispatcher,
				})

				var replyText string
				var mediaB64, mediaMime string
				if result.Error != nil {
					slog.Error("feishu dispatch error", "error", result.Error, "chatID", chatID)
					replyText = fmt.Sprintf("⚠️ 处理失败: %s", result.Error.Error())
				} else {
					replyText = CombineReplyPayloads(result.Replies)
					mediaB64, mediaMime = ExtractMediaFromReplies(result.Replies)
					if mediaB64 != "" {
						slog.Info("feishuDispatch: media extracted from replies",
							"mimeType", mediaMime, "base64Len", len(mediaB64))
					} else {
						slog.Debug("feishuDispatch: no media in replies", "replyCount", len(result.Replies))
					}
				}

				// AI 回复 transcript 由 attempt_runner.persistToTranscript 写入，此处不再双写

				// 广播 AI 回复到前端
				if bc != nil && (replyText != "" || mediaB64 != "") {
					chatPayload := map[string]interface{}{
						"sessionKey": sessionKey,
						"channel":    "feishu",
						"role":       "assistant",
						"text":       replyText,
						"chatId":     chatID,
						"ts":         time.Now().UnixMilli(),
					}
					if mediaB64 != "" {
						chatPayload["mediaBase64"] = mediaB64
						chatPayload["mediaMimeType"] = mediaMime
					}
					bc.Broadcast("chat.message", chatPayload, nil)
				}

				// 广播任务完成信号：触发前端 loadChatHistory 显示完整对话历史。
				// 若前端当前在飞书 session → handleChatEvent 匹配 → loadChatHistory。
				// 若前端在其他 session → fix 1a (app-gateway.ts) 自动切换回飞书 session。
				if bc != nil {
					bc.Broadcast("chat", map[string]interface{}{
						"sessionKey": sessionKey,
						"state":      "final",
					}, nil)
				}

				return feishuDispatchResult{
					Text:          replyText,
					MediaBase64:   mediaB64,
					MediaMimeType: mediaMime,
				}
			}

			feishuPlugin.DispatchFunc = func(ctx context.Context, channel, accountID, chatID, userID, text string) string {
				return feishuDispatch(channel, accountID, chatID, userID, text, "", "").Text
			}

			// 多模态分发：预处理附件（STT 转录 + 文档转换 + 图片下载），然后走公共分发
			feishuPreprocessor := &MultimodalPreprocessor{}
			if loadedCfg.STT != nil && loadedCfg.STT.Provider != "" {
				if prov, err := media.NewSTTProvider(loadedCfg.STT); err == nil {
					feishuPreprocessor.STTProvider = prov
					slog.Info("multimodal: STT provider loaded", "provider", prov.Name())
				} else {
					slog.Warn("multimodal: STT provider init failed (non-fatal)", "error", err)
				}
			}
			if loadedCfg.DocConv != nil && loadedCfg.DocConv.Provider != "" {
				if conv, err := media.NewDocConverter(loadedCfg.DocConv); err == nil {
					feishuPreprocessor.DocConverter = conv
					slog.Info("multimodal: DocConv provider loaded", "provider", conv.Name())
				} else {
					slog.Warn("multimodal: DocConv provider init failed (non-fatal)", "error", err)
				}
			}
			if loadedCfg.ImageUnderstanding != nil && loadedCfg.ImageUnderstanding.Provider != "" {
				if desc, err := media.NewImageDescriber(loadedCfg.ImageUnderstanding); err == nil {
					feishuPreprocessor.ImageDescriber = desc
					slog.Info("multimodal: ImageDescriber provider loaded", "provider", desc.Name())
				} else {
					slog.Warn("multimodal: ImageDescriber provider init failed (non-fatal)", "error", err)
				}
			}

			feishuPlugin.DispatchMultimodalFunc = func(channel, accountID, chatID, userID string, msg *channels.ChannelMessage) *channels.DispatchReply {
				// M-01: 添加超时，防止 STT/DocConv 无限挂起
				preprocessCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()
				client := feishuPlugin.GetClient(accountID)
				preprocessResult := feishuPreprocessor.ProcessFeishuMessage(preprocessCtx, client, msg)
				dr := feishuDispatch(channel, accountID, chatID, userID, preprocessResult.Text, preprocessResult.ImageBase64, preprocessResult.ImageMimeType)
				if dr.Text == "" && dr.MediaBase64 == "" {
					return nil
				}
				reply := &channels.DispatchReply{Text: dr.Text}
				if dr.MediaBase64 != "" {
					data, err := base64.StdEncoding.DecodeString(dr.MediaBase64)
					if err == nil {
						reply.MediaData = data
						reply.MediaMimeType = dr.MediaMimeType
					} else {
						slog.Warn("feishu: media base64 decode failed", "error", err)
					}
				}
				return reply
			}

			// 注入卡片回传交互回调（审批按钮点击，走 WebSocket 长连接）
			feishuPlugin.CardActionFunc = buildFeishuCardActionHandler(state)

			channelMgr.RegisterPlugin(feishuPlugin)
			slog.Info("channel: feishu plugin registered")

			// Bug A fix: 自动注入飞书频道凭据到审批系统
			if rn := state.RemoteApprovalNotifier(); rn != nil {
				feishuCfg := loadedCfg.Channels.Feishu
				if feishuCfg.AppID != "" && feishuCfg.AppSecret != "" {
					rn.InjectChannelFeishuConfig(feishuCfg.AppID, feishuCfg.AppSecret, feishuCfg.ApprovalChatID)
				}
			}
		}
		if loadedCfg.Channels.DingTalk != nil {
			dingtalkPlugin := dingtalk.NewDingTalkPlugin(loadedCfg)
			dingtalkPlugin.DispatchFunc = func(ctx context.Context, channel, accountID, chatID, userID, text string) string {
				sessionKey := fmt.Sprintf("dingtalk:%s", chatID)

				// 步骤 1: session 注册
				var resolvedSessionId string
				if sessionStore != nil {
					entry := sessionStore.LoadSessionEntry(sessionKey)
					if entry == nil {
						newId := fmt.Sprintf("session_%d", time.Now().UnixNano())
						entry = &SessionEntry{
							SessionKey: sessionKey,
							SessionId:  newId,
							Label:      fmt.Sprintf("钉钉:%s", chatID),
							Channel:    "dingtalk",
						}
						sessionStore.Save(entry)
						slog.Info("dingtalk: auto-created session", "sessionKey", sessionKey, "sessionId", newId)
					}
					resolvedSessionId = entry.SessionId
					sessionStore.RecordSessionMeta(sessionKey, InboundMeta{
						Channel:     "dingtalk",
						DisplayName: userID,
					})
				}

				// 步骤 2: 持久化用户消息
				if resolvedSessionId != "" {
					AppendUserTranscriptMessage(AppendTranscriptParams{
						Message:         text,
						SessionID:       resolvedSessionId,
						StorePath:       storePath,
						CreateIfMissing: true,
					})
				}

				msgCtx := &autoreply.MsgContext{
					Body:        text,
					ChannelType: channel,
					ChannelID:   chatID,
					SenderID:    userID,
					AccountID:   accountID,
					SessionID:   resolvedSessionId,
					SessionKey:  sessionKey,
				}

				// 广播用户消息到前端
				bc := state.Broadcaster()
				if bc != nil {
					bc.Broadcast("chat.message", map[string]interface{}{
						"sessionKey": sessionKey,
						"channel":    "dingtalk",
						"role":       "user",
						"text":       text,
						"from":       userID,
						"chatId":     chatID,
						"ts":         time.Now().UnixMilli(),
					}, nil)
				}

				result := DispatchInboundMessage(ctx, DispatchInboundParams{
					MsgCtx:     msgCtx,
					SessionKey: sessionKey,
					Dispatcher: pipelineDispatcher,
				})

				var reply string
				var dtMediaB64, dtMediaMime string
				if result.Error != nil {
					slog.Error("dingtalk dispatch error", "error", result.Error, "chatID", chatID)
					reply = fmt.Sprintf("⚠️ 处理失败: %s", result.Error.Error())
				} else {
					reply = CombineReplyPayloads(result.Replies)
					dtMediaB64, dtMediaMime = ExtractMediaFromReplies(result.Replies)
				}

				// 步骤 4: 持久化 AI 回复
				if resolvedSessionId != "" && reply != "" {
					AppendAssistantTranscriptMessage(AppendTranscriptParams{
						Message:         reply,
						SessionID:       resolvedSessionId,
						StorePath:       storePath,
						CreateIfMissing: true,
					})
				}

				// 广播 AI 回复到前端
				if bc != nil && (reply != "" || dtMediaB64 != "") {
					dtPayload := map[string]interface{}{
						"sessionKey": sessionKey,
						"channel":    "dingtalk",
						"role":       "assistant",
						"text":       reply,
						"chatId":     chatID,
						"ts":         time.Now().UnixMilli(),
					}
					if dtMediaB64 != "" {
						dtPayload["mediaBase64"] = dtMediaB64
						dtPayload["mediaMimeType"] = dtMediaMime
					}
					bc.Broadcast("chat.message", dtPayload, nil)
				}

				return reply
			}
			channelMgr.RegisterPlugin(dingtalkPlugin)
			slog.Info("channel: dingtalk plugin registered")
		}
		if loadedCfg.Channels.WeCom != nil {
			wecomPlugin := wecom.NewWeComPlugin(loadedCfg)
			wecomPlugin.DispatchFunc = func(ctx context.Context, channel, accountID, chatID, userID, text string) string {
				sessionKey := fmt.Sprintf("wecom:%s", chatID)

				// 步骤 1: session 注册
				var resolvedSessionId string
				if sessionStore != nil {
					entry := sessionStore.LoadSessionEntry(sessionKey)
					if entry == nil {
						newId := fmt.Sprintf("session_%d", time.Now().UnixNano())
						entry = &SessionEntry{
							SessionKey: sessionKey,
							SessionId:  newId,
							Label:      fmt.Sprintf("企微:%s", chatID),
							Channel:    "wecom",
						}
						sessionStore.Save(entry)
						slog.Info("wecom: auto-created session", "sessionKey", sessionKey, "sessionId", newId)
					}
					resolvedSessionId = entry.SessionId
					sessionStore.RecordSessionMeta(sessionKey, InboundMeta{
						Channel:     "wecom",
						DisplayName: userID,
					})
				}

				// 步骤 2: 持久化用户消息
				if resolvedSessionId != "" {
					AppendUserTranscriptMessage(AppendTranscriptParams{
						Message:         text,
						SessionID:       resolvedSessionId,
						StorePath:       storePath,
						CreateIfMissing: true,
					})
				}

				msgCtx := &autoreply.MsgContext{
					Body:        text,
					ChannelType: channel,
					ChannelID:   chatID,
					SenderID:    userID,
					AccountID:   accountID,
					SessionID:   resolvedSessionId,
					SessionKey:  sessionKey,
				}

				// 广播用户消息到前端
				bc := state.Broadcaster()
				if bc != nil {
					bc.Broadcast("chat.message", map[string]interface{}{
						"sessionKey": sessionKey,
						"channel":    "wecom",
						"role":       "user",
						"text":       text,
						"from":       userID,
						"chatId":     chatID,
						"ts":         time.Now().UnixMilli(),
					}, nil)
				}

				result := DispatchInboundMessage(ctx, DispatchInboundParams{
					MsgCtx:     msgCtx,
					SessionKey: sessionKey,
					Dispatcher: pipelineDispatcher,
				})

				var reply string
				var wcMediaB64, wcMediaMime string
				if result.Error != nil {
					slog.Error("wecom dispatch error", "error", result.Error, "chatID", chatID)
					reply = fmt.Sprintf("⚠️ 处理失败: %s", result.Error.Error())
				} else {
					reply = CombineReplyPayloads(result.Replies)
					wcMediaB64, wcMediaMime = ExtractMediaFromReplies(result.Replies)
				}

				// 步骤 4: 持久化 AI 回复
				if resolvedSessionId != "" && reply != "" {
					AppendAssistantTranscriptMessage(AppendTranscriptParams{
						Message:         reply,
						SessionID:       resolvedSessionId,
						StorePath:       storePath,
						CreateIfMissing: true,
					})
				}

				// 广播 AI 回复到前端
				if bc != nil && (reply != "" || wcMediaB64 != "") {
					wcPayload := map[string]interface{}{
						"sessionKey": sessionKey,
						"channel":    "wecom",
						"role":       "assistant",
						"text":       reply,
						"chatId":     chatID,
						"ts":         time.Now().UnixMilli(),
					}
					if wcMediaB64 != "" {
						wcPayload["mediaBase64"] = wcMediaB64
						wcPayload["mediaMimeType"] = wcMediaMime
					}
					bc.Broadcast("chat.message", wcPayload, nil)
				}

				return reply
			}
			channelMgr.RegisterPlugin(wecomPlugin)
			slog.Info("channel: wecom plugin registered")
		}

		// ---------- 4c-1b. 媒体频道插件注册 (WeChatMP/Xiaohongshu/Website) ----------
		if loadedCfg.Channels.WeChatMP != nil && loadedCfg.Channels.WeChatMP.Enabled {
			wmpPlugin := wechat_mp.NewWeChatMPPlugin()
			wmpCfg := loadedCfg.Channels.WeChatMP
			if err := wmpPlugin.ConfigureAccount(channels.DefaultAccountID, &wechat_mp.WeChatMPConfig{
				Enabled:        wmpCfg.Enabled,
				AppID:          wmpCfg.AppID,
				AppSecret:      wmpCfg.AppSecret,
				TokenCachePath: wmpCfg.TokenCachePath,
			}); err != nil {
				slog.Warn("channel: wechat_mp configure failed", "error", err)
			} else {
				channelMgr.RegisterPlugin(wmpPlugin)
				slog.Info("channel: wechat_mp plugin registered")
			}
		}
		if loadedCfg.Channels.Xiaohongshu != nil && loadedCfg.Channels.Xiaohongshu.Enabled {
			xhsPlugin := xiaohongshu.NewXiaohongshuPlugin()
			xhsCfg := loadedCfg.Channels.Xiaohongshu
			if err := xhsPlugin.ConfigureAccount(channels.DefaultAccountID, &xiaohongshu.XiaohongshuConfig{
				Enabled:              xhsCfg.Enabled,
				CookiePath:           xhsCfg.CookiePath,
				AutoInteractInterval: xhsCfg.AutoInteractInterval,
				RateLimitSeconds:     xhsCfg.RateLimitSeconds,
				ErrorScreenshotDir:   xhsCfg.ErrorScreenshotDir,
			}); err != nil {
				slog.Warn("channel: xiaohongshu configure failed", "error", err)
			} else {
				channelMgr.RegisterPlugin(xhsPlugin)
				slog.Info("channel: xiaohongshu plugin registered")
			}
		}
		if loadedCfg.Channels.Website != nil && loadedCfg.Channels.Website.Enabled {
			wsPlugin := website.NewWebsitePlugin()
			wsCfg := loadedCfg.Channels.Website
			if err := wsPlugin.ConfigureAccount(channels.DefaultAccountID, &website.WebsiteConfig{
				Enabled:        wsCfg.Enabled,
				APIURL:         wsCfg.APIURL,
				AuthType_:      website.AuthType(wsCfg.AuthType),
				AuthToken:      wsCfg.AuthToken,
				ImageUploadURL: wsCfg.ImageUploadURL,
				TimeoutSeconds: wsCfg.TimeoutSeconds,
				MaxRetries:     wsCfg.MaxRetries,
			}); err != nil {
				slog.Warn("channel: website configure failed", "error", err)
			} else {
				channelMgr.RegisterPlugin(wsPlugin)
				slog.Info("channel: website plugin registered")
			}
		}

		// 启动已配置的频道
		pluginChannels := []channels.ChannelID{channels.ChannelFeishu, channels.ChannelDingTalk, channels.ChannelWeCom}
		if loadedCfg.Channels.WeChatMP != nil && loadedCfg.Channels.WeChatMP.Enabled {
			pluginChannels = append(pluginChannels, media.ChannelWeChatMP)
		}
		if loadedCfg.Channels.Xiaohongshu != nil && loadedCfg.Channels.Xiaohongshu.Enabled {
			pluginChannels = append(pluginChannels, media.ChannelXiaohongshu)
		}
		if loadedCfg.Channels.Website != nil && loadedCfg.Channels.Website.Enabled {
			pluginChannels = append(pluginChannels, website.ChannelWebsite)
		}
		for _, chID := range pluginChannels {
			if err := channelMgr.StartChannel(chID, channels.DefaultAccountID); err != nil {
				slog.Warn("channel: start failed (non-fatal)", "channel", chID, "error", err)
			}
		}

		// ---------- 4c-2. 启动 Monitor 模式渠道 (Discord/Telegram/Slack) ----------
		monitorDepsCtx := &ChannelDepsContext{
			StorePath:    storePath,
			State:        state,
			EventQueue:   eventQueue,
			Dispatcher:   pipelineDispatcher,
			SessionStore: sessionStore,
		}
		monitorCtx, monitorCancel := context.WithCancel(context.Background())
		_ = monitorCancel // 优雅关闭时调用
		startMonitorChannels(monitorCtx, monitorDepsCtx, loadedCfg, nil)
	}

	// ---------- 4f. 创建 CronService ----------
	cronStorePath := filepath.Join(storePath, "cron", "jobs.json")
	cronSvc := cron.NewCronService(cron.CronServiceDeps{
		StorePath: cronStorePath,
		Logger:    &slogCronLogger{},
		OnEvent: func(event cron.CronEvent) {
			bc := state.Broadcaster()
			if bc != nil {
				bc.Broadcast("cron.event", event, nil)
			}
		},
		EnqueueSystemEvent: func(text string) error {
			eventQueue.Enqueue(text, "main", "cron")
			return nil
		},
		RequestHeartbeatNow: func() {
			// 心跳唤醒 — 目前仅标记为可用
			heartbeatState.SetEnabled(true)
		},
	})
	if !config.SkipCron {
		if err := cronSvc.Start(); err != nil {
			slog.Warn("gateway: cron service start failed", "error", err)
		} else {
			slog.Info("gateway: cron service started")
		}
	}

	// ---------- 4g. 创建技能商店客户端 ----------
	var skillStoreClient *skills.SkillStoreClient
	if loadedCfg != nil && loadedCfg.Skills != nil && loadedCfg.Skills.Store != nil {
		store := loadedCfg.Skills.Store
		if store.URL != "" && store.Token != "" {
			skillStoreClient = skills.NewSkillStoreClient(store.URL, store.Token)
			slog.Info("gateway: skill store client configured", "url", store.URL)
		}
	}

	// ---------- 4h. 创建 MCP 远程工具 Bridge (P2) ----------
	var remoteMCPBridge *mcpremote.RemoteBridge
	if loadedCfg != nil && loadedCfg.Skills != nil && loadedCfg.Skills.Store != nil {
		store := loadedCfg.Skills.Store
		if store.MCP != nil && store.MCP.Enabled && store.URL != "" {
			// 计算 MCP 端点
			mcpEndpoint := store.MCP.Endpoint
			if mcpEndpoint == "" {
				mcpEndpoint = strings.TrimRight(store.URL, "/") + "/api/v4/mcp"
			}

			// 创建 Token Manager（P1 JWT 兼容模式）
			tokenMgr := mcpremote.NewOAuthTokenManager(mcpremote.OAuthConfig{
				StaticToken:  store.Token,
				OAuthEnabled: store.MCP.OAuthEnabled,
				IssuerURL:    store.URL,
			}, nil)

			remoteMCPBridge = mcpremote.NewRemoteBridge(mcpremote.RemoteBridgeConfig{
				Endpoint:     mcpEndpoint,
				TokenManager: tokenMgr,
			})

			// 异步启动连接（不阻塞网关启动）
			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := remoteMCPBridge.Start(bgCtx); err != nil {
					slog.Warn("gateway: MCP remote bridge start failed (non-fatal)", "error", err, "endpoint", mcpEndpoint)
				}
			}()
			state.SetRemoteMCPBridge(remoteMCPBridge)
			slog.Info("gateway: MCP remote bridge initializing", "endpoint", mcpEndpoint)
		}
	}

	// 4h-b. 注入 Remote MCP Bridge 到 AttemptRunner（需在 4h 后）
	if remoteMCPBridge != nil {
		attemptRunner.RemoteMCPBridge = &remoteMCPBridgeAdapter{bridge: remoteMCPBridge}
	}

	// ---------- 5. 创建 HTTP 路由 ----------
	wsConfig := WsServerConfig{
		Auth:               auth,
		TrustedProxies:     bootCfg.TrustedProxies,
		State:              state,
		Registry:           registry,
		SessionStore:       sessionStore,
		StorePath:          storePath,
		Version:            cli.Version,
		LogFilePath:        logFilePath,
		ConfigLoader:       cfgLoader,
		ModelCatalog:       modelCatalog,
		PresenceStore:      presenceStore,
		HeartbeatState:     heartbeatState,
		EventQueue:         eventQueue,
		PipelineDispatcher: pipelineDispatcher,
		CronService:        cronSvc,
		CronStorePath:      filepath.Dir(cronStorePath),
		ChannelMgr:         channelMgr,
		SkillStoreClient:   skillStoreClient,
		RemoteMCPBridge:    remoteMCPBridge, // P2: MCP 远程工具
		BootedAt:           time.Now(),
		MediaSubsystem:     mediaSub, // Phase 5+6: 媒体子系统
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		SendJSON(w, http.StatusOK, GetHealthStatus(state, cli.Version))
	})

	// WebSocket 端点
	mux.HandleFunc("/ws", HandleWebSocketUpgrade(wsConfig))

	// Issue 4 fix: Register hooks/openai/tools routes directly on top-level mux
	// (Previously these were nested under /hooks/ via CreateGatewayHTTPHandler, causing
	// double-prefix like /hooks/hooks/ and nil callback panics.)
	httpCfg := GatewayHTTPHandlerConfig{
		ControlUIDir:   opts.ControlUIDir,
		TrustedProxies: bootCfg.TrustedProxies,
		GetHooksConfig: func() *HooksConfig {
			// Hooks require explicit enabled+token in config; return nil if not configured.
			// The hooksHandler already returns 404 when config is nil.
			if loadedCfg != nil && loadedCfg.Hooks != nil && loadedCfg.Hooks.Enabled != nil && *loadedCfg.Hooks.Enabled {
				raw := &HooksRawConfig{
					Enabled:  loadedCfg.Hooks.Enabled,
					Token:    loadedCfg.Hooks.Token,
					Path:     loadedCfg.Hooks.Path,
					Presets:  loadedCfg.Hooks.Presets,
					Mappings: convertHookMappings(loadedCfg.Hooks.Mappings),
				}
				if loadedCfg.Hooks.MaxBodyBytes != nil {
					raw.MaxBodyBytes = int64(*loadedCfg.Hooks.MaxBodyBytes)
				}
				cfg, err := ResolveHooksConfig(raw)
				if err != nil {
					slog.Warn("gateway: hooks config error", "error", err)
					return nil
				}
				return cfg
			}
			return nil
		},
		GetAuth: func() ResolvedGatewayAuth {
			return auth
		},
		PipelineDispatcher: pipelineDispatcher,
	}

	// Hooks handler (直接注册到顶层)
	hooksHandler := createHooksHTTPHandler(httpCfg)
	mux.HandleFunc("/hooks/", hooksHandler)

	// OpenAI Chat Completions API (直接注册到顶层)
	openaiCfg := OpenAIChatHandlerConfig{
		GetAuth:        httpCfg.GetAuth,
		Dispatcher:     httpCfg.PipelineDispatcher,
		TrustedProxies: httpCfg.TrustedProxies,
	}
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		HandleOpenAIChatCompletions(w, r, openaiCfg)
	})
	mux.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		HandleOpenAIResponses(w, r, openaiCfg)
	})

	// Tools invoke (直接注册到顶层)
	toolsCfg := ToolsInvokeHandlerConfig{
		GetAuth:   httpCfg.GetAuth,
		Invoker:   httpCfg.ToolInvoker,
		ToolNames: httpCfg.ToolNames,
	}
	mux.HandleFunc("/tools/invoke/", func(w http.ResponseWriter, r *http.Request) {
		HandleToolsInvoke(w, r, toolsCfg)
	})

	// Control UI 静态文件
	if opts.ControlUIDir != "" {
		fs := http.FileServer(http.Dir(opts.ControlUIDir))
		mux.Handle("/ui/", http.StripPrefix("/ui/", fs))
	}

	// Phase 5: 频道 webhook HTTP 路由
	mux.HandleFunc("/channels/feishu/webhook", ChannelWebhookFeishu(channelMgr))
	mux.HandleFunc("/channels/wecom/callback", ChannelWebhookWeCom(channelMgr))

	// Issue 6: Root path handler — prevents 404 for "/"
	hasControlUI := opts.ControlUIDir != ""
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only handle exact "/" path; net/http routes unmatched paths to "/"
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if hasControlUI {
			http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
			return
		}
		SendJSON(w, http.StatusOK, map[string]interface{}{
			"name":    "OpenAcosmi Gateway",
			"version": cli.Version,
			"status":  "ok",
		})
	})

	// ---------- 5. 创建并启动 HTTP 服务器 ----------
	serverCfg := ServerConfig{
		Host:           bootCfg.Server.Host,
		Port:           port,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   0, // SSE + WS 需要无限写超时
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	httpServer := NewGatewayHTTPServer(serverCfg, mux)

	// ---------- 5a. 功能开关（SKIP_* 系列）----------
	// 对应 TS server.impl.ts 启动顺序第 4-7 步的 OPENACOSMI_SKIP_* 控制逻辑。
	// 各子系统在未来接入时须先检查对应 flag 再执行 Start()。
	if config.SkipCron {
		slog.Info("gateway: OPENACOSMI_SKIP_CRON set — cron scheduler will not start")
	}
	if config.SkipChannels {
		slog.Info("gateway: OPENACOSMI_SKIP_CHANNELS set — channel subsystem will not start")
	}
	if config.SkipBrowserControl {
		slog.Info("gateway: OPENACOSMI_SKIP_BROWSER_CONTROL_SERVER set — browser control server will not start")
	}
	if config.SkipCanvasHost {
		slog.Info("gateway: OPENACOSMI_SKIP_CANVAS_HOST set — canvas host will not start")
	}
	if config.SkipProviders {
		slog.Info("gateway: OPENACOSMI_SKIP_PROVIDERS set — provider initialization skipped")
	}

	// ---------- 5b. 启动维护计时器（gateway.tick 广播） ----------
	// 对齐 TS server-maintenance.ts: 每 30s 广播 tick 事件
	maintenanceTimers := StartMaintenanceTick(state.Broadcaster())

	runtime := &GatewayRuntime{
		State:             state,
		HTTPServer:        httpServer,
		MaintenanceTimers: maintenanceTimers,
	}

	// 启动 HTTP 监听
	go func() {
		listenAddr := fmt.Sprintf("%s:%d", serverCfg.Host, serverCfg.Port)
		slog.Info("🦜 Gateway listening", "addr", listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("gateway: http server error", "error", err)
			os.Exit(1)
		}
	}()

	// 等待服务器就绪
	time.Sleep(100 * time.Millisecond)
	state.SetPhase(BootPhaseReady)
	slog.Info("gateway: ready", "port", port)

	// 打印 Dashboard URL（Jupyter 模式：终端可点击直接打开）
	if auth.Token != "" {
		dashURL := fmt.Sprintf("http://localhost:%d/?token=%s", port, auth.Token)
		slog.Info("🔑 Dashboard URL (copy to browser)", "url", dashURL)
	}

	return runtime, nil
}

// RunGatewayBlocking 启动网关并阻塞直到收到终止信号。
func RunGatewayBlocking(port int, opts GatewayServerOptions) error {
	runtime, err := StartGatewayServer(port, opts)
	if err != nil {
		return err
	}

	// 等待终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	slog.Info("gateway: received signal", "signal", sig)

	return runtime.Close(fmt.Sprintf("signal: %s", sig))
}

// resolveDefaultStorePath 解析默认存储路径。
// 顺序: $OPENACOSMI_STORE_PATH → ~/.openacosmi/store
func resolveDefaultStorePath() string {
	if v := os.Getenv("OPENACOSMI_STORE_PATH"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("gateway: cannot resolve home dir, using ./store", "error", err)
		return "./store"
	}
	return filepath.Join(home, ".openacosmi", "store")
}

// convertHookMappings 将 types.HookMappingConfig 列表转换为 gateway.HookMappingConfig 列表。
func convertHookMappings(mappings []types.HookMappingConfig) []HookMappingConfig {
	if len(mappings) == 0 {
		return nil
	}
	result := make([]HookMappingConfig, len(mappings))
	for i, m := range mappings {
		r := HookMappingConfig{
			ID:                         m.ID,
			Action:                     m.Action,
			WakeMode:                   m.WakeMode,
			MessageTemplate:            m.MessageTemplate,
			TextTemplate:               m.TextTemplate,
			Name:                       m.Name,
			SessionKey:                 m.SessionKey,
			Channel:                    m.Channel,
			To:                         m.To,
			Model:                      m.Model,
			Deliver:                    m.Deliver,
			Thinking:                   m.Thinking,
			AllowUnsafeExternalContent: m.AllowUnsafeExternalContent,
		}
		if m.Match != nil {
			r.Match = &HookMatchFieldConfig{
				Path:   m.Match.Path,
				Source: m.Match.Source,
			}
		}
		if m.TimeoutSeconds != nil {
			r.TimeoutSeconds = *m.TimeoutSeconds
		}
		if m.Transform != nil {
			r.TransformModule = m.Transform.Module
			r.TransformExport = m.Transform.Export
		}
		result[i] = r
	}
	return result
}

// slogCronLogger 将 cron.CronLogger 接口适配到标准 slog。
type slogCronLogger struct{}

func (l *slogCronLogger) Info(msg string, fields ...interface{}) { slog.Info("cron: "+msg, fields...) }
func (l *slogCronLogger) Warn(msg string, fields ...interface{}) { slog.Warn("cron: "+msg, fields...) }
func (l *slogCronLogger) Error(msg string, fields ...interface{}) {
	slog.Error("cron: "+msg, fields...)
}
func (l *slogCronLogger) Debug(msg string, fields ...interface{}) {
	slog.Debug("cron: "+msg, fields...)
}

// buildFeishuCardActionHandler 构建飞书卡片回传交互处理器。
// 处理审批/操作确认卡片按钮点击，通过 WebSocket 长连接接收，无需公网地址。
// 通过 value["type"] 区分卡片类型：
//   - 无 type 或空 → 权限提升审批（escalation，向后兼容）
//   - "coder_confirm" → 操作确认（CoderConfirmation）
func buildFeishuCardActionHandler(state *GatewayState) feishu.CardActionHandler {
	return func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
		if event == nil || event.Event == nil || event.Event.Action == nil {
			slog.Warn("feishu card action: missing event data")
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{Type: "error", Content: "回调数据异常"},
			}, nil
		}

		value := event.Event.Action.Value
		actionStr, _ := value["action"].(string)
		cardID, _ := value["id"].(string)
		cardType, _ := value["type"].(string)

		if cardID == "" {
			slog.Warn("feishu card action: missing ID")
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{Type: "error", Content: "回调 ID 缺失"},
			}, nil
		}

		// 根据卡片类型路由
		switch cardType {
		case "coder_confirm":
			return handleFeishuCoderConfirmAction(state, cardID, actionStr)
		default:
			// 向后兼容：无 type 字段视为 escalation 审批
			return handleFeishuEscalationAction(state, cardID, actionStr, value)
		}
	}
}

// handleFeishuEscalationAction 处理权限提升审批卡片回调。
func handleFeishuEscalationAction(state *GatewayState, escalationID, actionStr string, value map[string]interface{}) (*callback.CardActionTriggerResponse, error) {
	escMgr := state.EscalationMgr()
	if escMgr == nil {
		slog.Warn("feishu card action: escalation manager not available")
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "error", Content: "审批系统未初始化"},
		}, nil
	}

	switch actionStr {
	case "approve":
		ttl := 30
		if ttlFloat, ok := value["ttl"].(float64); ok && ttlFloat > 0 {
			ttl = int(ttlFloat)
		}
		if err := escMgr.ResolveEscalation(true, ttl); err != nil {
			slog.Warn("feishu card action: approve failed", "id", escalationID, "error", err)
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{Type: "warning", Content: "审批失败: " + err.Error()},
			}, nil
		}
		slog.Info("feishu card action: approved", "id", escalationID, "ttl", ttl)
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "success", Content: "✅ 审批通过，权限已生效"},
		}, nil

	case "deny":
		if err := escMgr.ResolveEscalation(false, 0); err != nil {
			slog.Warn("feishu card action: deny failed", "id", escalationID, "error", err)
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{Type: "warning", Content: "拒绝失败: " + err.Error()},
			}, nil
		}
		slog.Info("feishu card action: denied", "id", escalationID)
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "info", Content: "❌ 已拒绝权限提升请求"},
		}, nil

	default:
		slog.Warn("feishu card action: unknown action", "action", actionStr)
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "error", Content: "未知操作: " + actionStr},
		}, nil
	}
}

// handleFeishuCoderConfirmAction 处理操作确认卡片回调。
func handleFeishuCoderConfirmAction(state *GatewayState, confirmID, actionStr string) (*callback.CardActionTriggerResponse, error) {
	confirmMgr := state.CoderConfirmMgr()
	if confirmMgr == nil {
		slog.Warn("feishu card action: coder confirm manager not available")
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "error", Content: "确认系统未初始化"},
		}, nil
	}

	// 映射 action → decision
	var decision string
	switch actionStr {
	case "allow":
		decision = "allow"
	case "deny":
		decision = "deny"
	default:
		slog.Warn("feishu card action: unknown coder_confirm action", "action", actionStr)
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "error", Content: "未知操作: " + actionStr},
		}, nil
	}

	if err := confirmMgr.ResolveConfirmation(confirmID, decision); err != nil {
		slog.Warn("feishu card action: coder confirm resolve failed", "id", confirmID, "error", err)
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "warning", Content: "确认失败: " + err.Error()},
		}, nil
	}

	approved := decision == "allow"
	slog.Info("feishu card action: coder confirm resolved", "id", confirmID, "decision", decision)

	// 推送结果卡片
	if notifier := state.RemoteApprovalNotifier(); notifier != nil {
		notifier.NotifyCoderConfirmResult(confirmID, approved)
	}

	if approved {
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "success", Content: "✅ 操作已批准"},
		}, nil
	}
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{Type: "info", Content: "❌ 操作已拒绝"},
	}, nil
}
