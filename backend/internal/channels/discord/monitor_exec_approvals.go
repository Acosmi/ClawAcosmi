package discord

// Discord 执行审批 UI — 继承自 src/discord/monitor/exec-approvals.ts (579L)
// Phase 9 实现：InteractionCreate 处理 + Button Components + Embed 更新。

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// DiscordApprovalState 审批状态
type DiscordApprovalState string

const (
	ApprovalStatePending  DiscordApprovalState = "pending"
	ApprovalStateApproved DiscordApprovalState = "approved"
	ApprovalStateDenied   DiscordApprovalState = "denied"
)

// DiscordApprovalRequest 审批请求
type DiscordApprovalRequest struct {
	Command    string
	Requester  string
	SessionKey string
	ChannelID  string
	MessageID  string // embed 消息 ID
	State      DiscordApprovalState
}

// 按钮 custom_id 前缀
const (
	approvalApprovePrefix = "exec_approve:"
	approvalDenyPrefix    = "exec_deny:"
)

// HandleDiscordInteraction 处理 Discord 交互事件。
func HandleDiscordInteraction(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := i.MessageComponentData()
	customID := data.CustomID

	switch {
	case strings.HasPrefix(customID, approvalApprovePrefix):
		handleApprovalButton(monCtx, i, true)
	case strings.HasPrefix(customID, approvalDenyPrefix):
		handleApprovalButton(monCtx, i, false)
	default:
		// 未知交互，忽略
		return
	}
}

// BuildExecApprovalEmbed 构建执行审批 Embed。
func BuildExecApprovalEmbed(command, requester, sessionKey string) *discordgo.MessageSend {
	embed := &discordgo.MessageEmbed{
		Title:       "⚠️ Execution Approval Required",
		Description: fmt.Sprintf("**Command:** `%s`\n**Requested by:** %s\n**Session:** `%s`", command, requester, sessionKey),
		Color:       0xFFA500, // orange
		Footer:      &discordgo.MessageEmbedFooter{Text: "Click a button below to approve or deny."},
	}

	approveBtn := discordgo.Button{
		Label:    "✅ Approve",
		Style:    discordgo.SuccessButton,
		CustomID: approvalApprovePrefix + sessionKey,
	}
	denyBtn := discordgo.Button{
		Label:    "❌ Deny",
		Style:    discordgo.DangerButton,
		CustomID: approvalDenyPrefix + sessionKey,
	}
	row := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{approveBtn, denyBtn},
	}

	return &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{row},
	}
}

// handleApprovalButton 处理审批按钮点击。
func handleApprovalButton(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate, approved bool) {
	logger := monCtx.Logger.With("action", "exec-approval", "approved", approved)

	// ACK 交互
	action := "approved"
	emoji := "✅"
	color := 0x00FF00 // green
	if !approved {
		action = "denied"
		emoji = "❌"
		color = 0xFF0000 // red
	}

	// 更新 Embed
	respEmbed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("%s Execution %s", emoji, capitalizeFirst(action)),
		Description: fmt.Sprintf("Action was **%s** by <@%s>.", action, func() string {
			if u := resolveInteractionUser(i); u != nil {
				return u.ID
			}
			return "unknown"
		}()),
		Color: color,
	}

	err := monCtx.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{respEmbed},
			Components: []discordgo.MessageComponent{}, // 移除按钮
		},
	})
	if err != nil {
		logger.Error("interaction response failed", "error", err)
	}

	logger.Info("exec approval handled", "action", action)
}

// capitalizeFirst uppercases the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// 确保 slog 被引用
var _ = slog.Default

// ── Full Exec Approval Handler ──
// TS ref: exec-approvals.ts L15-579

// ExecApprovalDecision 审批决策类型
type ExecApprovalDecision string

const (
	ExecApprovalAllowOnce   ExecApprovalDecision = "allow-once"
	ExecApprovalAllowAlways ExecApprovalDecision = "allow-always"
	ExecApprovalDeny        ExecApprovalDecision = "deny"
)

// ExecApprovalRequest 执行审批请求
// TS ref: ExecApprovalRequest
type ExecApprovalRequest struct {
	ID          string                    `json:"id"`
	Request     ExecApprovalRequestDetail `json:"request"`
	CreatedAtMs int64                     `json:"createdAtMs"`
	ExpiresAtMs int64                     `json:"expiresAtMs"`
}

// ExecApprovalRequestDetail 审批请求详情
type ExecApprovalRequestDetail struct {
	Command      string `json:"command"`
	Cwd          string `json:"cwd,omitempty"`
	Host         string `json:"host,omitempty"`
	Security     string `json:"security,omitempty"`
	Ask          string `json:"ask,omitempty"`
	AgentID      string `json:"agentId,omitempty"`
	ResolvedPath string `json:"resolvedPath,omitempty"`
	SessionKey   string `json:"sessionKey,omitempty"`
}

// ExecApprovalResolved 审批结果
// TS ref: ExecApprovalResolved
type ExecApprovalResolved struct {
	ID         string               `json:"id"`
	Decision   ExecApprovalDecision `json:"decision"`
	ResolvedBy string               `json:"resolvedBy,omitempty"`
	Ts         int64                `json:"ts"`
}

// ExecApprovalConfig 审批配置
type ExecApprovalConfig struct {
	Enabled       bool     `json:"enabled"`
	Approvers     []string `json:"approvers,omitempty"`
	AgentFilter   []string `json:"agentFilter,omitempty"`
	SessionFilter []string `json:"sessionFilter,omitempty"`
}

// pendingApproval 待处理审批
type pendingApproval struct {
	DiscordMessageID string
	DiscordChannelID string
	CancelTimeout    context.CancelFunc
}

// ── GatewayClient interface ──
// TS ref: GatewayClient (gateway/client.ts) — start / stop / request.
// Full WebSocket implementation lives in Phase 7; this interface allows
// the approval handler to resolve decisions via the gateway.

// ExecApprovalGatewayClient abstracts the gateway operations needed by
// the exec approval handler (start, stop, resolveApproval).
// TODO(phase-7): Provide a concrete implementation backed by a real WebSocket
// gateway client once the gateway package is available in Go.
type ExecApprovalGatewayClient interface {
	// Start connects to the gateway and begins listening for events.
	Start() error
	// Stop disconnects from the gateway.
	Stop()
	// ResolveApproval sends a resolution decision to the gateway.
	// TS ref: GatewayClient.request("exec.approval.resolve", { id, decision })
	ResolveApproval(approvalID string, decision ExecApprovalDecision) error
}

// DiscordExecApprovalHandlerOpts 审批处理器选项
type DiscordExecApprovalHandlerOpts struct {
	Token     string
	AccountID string
	Config    ExecApprovalConfig
	Session   *discordgo.Session
	// GatewayClient optional gateway client for resolving approvals.
	// When nil, resolveApproval will delegate to the OnResolve callback instead.
	// TODO(phase-7): Wire a real GatewayClient implementation here.
	GatewayClient ExecApprovalGatewayClient
	// OnResolve callback when an approval is resolved (optional)
	OnResolve func(id string, decision ExecApprovalDecision) error
}

// DiscordExecApprovalHandler 执行审批处理器
// TS ref: DiscordExecApprovalHandler class
type DiscordExecApprovalHandler struct {
	mu           sync.Mutex
	opts         DiscordExecApprovalHandlerOpts
	pending      map[string]*pendingApproval
	requestCache map[string]*ExecApprovalRequest
	started      bool
	logger       *slog.Logger
}

// NewDiscordExecApprovalHandler creates a new approval handler.
func NewDiscordExecApprovalHandler(opts DiscordExecApprovalHandlerOpts) *DiscordExecApprovalHandler {
	return &DiscordExecApprovalHandler{
		opts:         opts,
		pending:      make(map[string]*pendingApproval),
		requestCache: make(map[string]*ExecApprovalRequest),
		logger:       slog.Default().With("module", "discord-exec-approvals"),
	}
}

// Start initialises the handler and connects to the gateway (if available).
// TS ref: start()
func (h *DiscordExecApprovalHandler) Start() error {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return nil
	}
	h.started = true
	h.mu.Unlock()

	config := h.opts.Config
	if !config.Enabled {
		h.logger.Debug("exec approvals disabled")
		return nil
	}
	if len(config.Approvers) == 0 {
		h.logger.Debug("exec approvals: no approvers configured")
		return nil
	}

	h.logger.Debug("exec approvals: starting handler")

	if h.opts.GatewayClient != nil {
		if err := h.opts.GatewayClient.Start(); err != nil {
			h.logger.Error("gateway client start failed", "error", err)
			return err
		}
	}
	return nil
}

// ResolveApproval sends a resolution decision via the gateway client.
// TS ref: resolveApproval()
func (h *DiscordExecApprovalHandler) ResolveApproval(approvalID string, decision ExecApprovalDecision) error {
	if h.opts.GatewayClient != nil {
		return h.opts.GatewayClient.ResolveApproval(approvalID, decision)
	}
	// Fallback: delegate to OnResolve callback
	if h.opts.OnResolve != nil {
		return h.opts.OnResolve(approvalID, decision)
	}
	return fmt.Errorf("exec approvals: no gateway client or OnResolve callback configured")
}

// ShouldHandle checks if a request should be handled by this handler.
// TS ref: shouldHandle()
func (h *DiscordExecApprovalHandler) ShouldHandle(request *ExecApprovalRequest) bool {
	config := h.opts.Config
	if !config.Enabled {
		return false
	}
	if len(config.Approvers) == 0 {
		return false
	}
	// Agent filter
	if len(config.AgentFilter) > 0 {
		if request.Request.AgentID == "" {
			return false
		}
		found := false
		for _, id := range config.AgentFilter {
			if id == request.Request.AgentID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	// Session filter — TS ref: session.includes(p) || new RegExp(p).test(session)
	// Try substring match first, then fall back to regex match (matching TS behaviour).
	if len(config.SessionFilter) > 0 {
		session := request.Request.SessionKey
		if session == "" {
			return false
		}
		matches := false
		for _, pattern := range config.SessionFilter {
			if strings.Contains(session, pattern) {
				matches = true
				break
			}
			// Regex fallback: compile the pattern; ignore if invalid (TS catch block).
			if re, err := regexp.Compile(pattern); err == nil {
				if re.MatchString(session) {
					matches = true
					break
				}
			}
		}
		if !matches {
			return false
		}
	}
	return true
}

// HandleApprovalRequested processes a new approval request.
// TS ref: handleApprovalRequested()
func (h *DiscordExecApprovalHandler) HandleApprovalRequested(request *ExecApprovalRequest) {
	if !h.ShouldHandle(request) {
		return
	}

	h.logger.Debug("received approval request", "id", request.ID)

	h.mu.Lock()
	h.requestCache[request.ID] = request
	h.mu.Unlock()

	session := h.opts.Session
	if session == nil {
		h.logger.Error("session not available for approval notification")
		return
	}

	// Build embed
	embed := formatExecApprovalRequestEmbed(request)

	// Build action buttons
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Allow once",
					Style:    discordgo.SuccessButton,
					CustomID: BuildExecApprovalCustomID(request.ID, ExecApprovalAllowOnce),
				},
				discordgo.Button{
					Label:    "Always allow",
					Style:    discordgo.PrimaryButton,
					CustomID: BuildExecApprovalCustomID(request.ID, ExecApprovalAllowAlways),
				},
				discordgo.Button{
					Label:    "Deny",
					Style:    discordgo.DangerButton,
					CustomID: BuildExecApprovalCustomID(request.ID, ExecApprovalDeny),
				},
			},
		},
	}

	for _, approverID := range h.opts.Config.Approvers {
		// Create DM channel
		dmCh, err := session.UserChannelCreate(approverID)
		if err != nil {
			h.logger.Error("failed to create DM channel", "user", approverID, "error", err)
			continue
		}

		// Send message with embed and buttons
		msg, err := session.ChannelMessageSendComplex(dmCh.ID, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		})
		if err != nil {
			h.logger.Error("failed to send approval message", "user", approverID, "error", err)
			continue
		}

		// Set up timeout
		timeoutMs := request.ExpiresAtMs - time.Now().UnixMilli()
		if timeoutMs < 0 {
			timeoutMs = 0
		}

		ctx, cancel := context.WithCancel(context.Background())
		h.mu.Lock()
		h.pending[request.ID] = &pendingApproval{
			DiscordMessageID: msg.ID,
			DiscordChannelID: dmCh.ID,
			CancelTimeout:    cancel,
		}
		h.mu.Unlock()

		// Start timeout goroutine
		go func(approvalID string, timeoutDuration time.Duration) {
			select {
			case <-time.After(timeoutDuration):
				h.handleApprovalTimeout(approvalID)
			case <-ctx.Done():
				// Cancelled (resolved before timeout)
			}
		}(request.ID, time.Duration(timeoutMs)*time.Millisecond)

		h.logger.Debug("sent approval to user", "user", approverID, "id", request.ID)
		// TS sends to ALL approvers (no break) — W-039 fix
	}
}

// HandleApprovalResolved processes an approval resolution event.
// TS ref: handleApprovalResolved()
func (h *DiscordExecApprovalHandler) HandleApprovalResolved(resolved *ExecApprovalResolved) {
	h.mu.Lock()
	pending, ok := h.pending[resolved.ID]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.pending, resolved.ID)
	pending.CancelTimeout()

	request, hasRequest := h.requestCache[resolved.ID]
	delete(h.requestCache, resolved.ID)
	h.mu.Unlock()

	if !hasRequest || request == nil {
		return
	}

	h.logger.Debug("approval resolved", "id", resolved.ID, "decision", resolved.Decision)

	h.updateApprovalMessage(
		pending.DiscordChannelID,
		pending.DiscordMessageID,
		formatExecApprovalResolvedEmbed(request, resolved.Decision, resolved.ResolvedBy),
	)
}

// handleApprovalTimeout handles approval timeout.
// TS ref: handleApprovalTimeout()
func (h *DiscordExecApprovalHandler) handleApprovalTimeout(approvalID string) {
	h.mu.Lock()
	pending, ok := h.pending[approvalID]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.pending, approvalID)

	request, hasRequest := h.requestCache[approvalID]
	delete(h.requestCache, approvalID)
	h.mu.Unlock()

	if !hasRequest || request == nil {
		return
	}

	h.logger.Debug("approval timeout", "id", approvalID)

	h.updateApprovalMessage(
		pending.DiscordChannelID,
		pending.DiscordMessageID,
		formatExecApprovalExpiredEmbed(request),
	)
}

// updateApprovalMessage updates the approval message with new embed and removes buttons.
func (h *DiscordExecApprovalHandler) updateApprovalMessage(channelID, messageID string, embed *discordgo.MessageEmbed) {
	session := h.opts.Session
	if session == nil {
		return
	}
	emptyComponents := []discordgo.MessageComponent{}
	_, err := session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    channelID,
		ID:         messageID,
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &emptyComponents,
	})
	if err != nil {
		h.logger.Error("failed to update approval message", "error", err)
	}
}

// Stop stops the handler and clears pending approvals.
// TS ref: stop()
func (h *DiscordExecApprovalHandler) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, pending := range h.pending {
		pending.CancelTimeout()
	}
	h.pending = make(map[string]*pendingApproval)
	h.requestCache = make(map[string]*ExecApprovalRequest)
	h.started = false

	// W-040: Stop gateway client if present (TS ref: this.gatewayClient?.stop())
	if h.opts.GatewayClient != nil {
		h.opts.GatewayClient.Stop()
	}

	h.logger.Debug("exec approvals handler stopped")
}

// HandleExecApprovalButtonInteraction handles button interactions for exec approvals.
// TS ref: ExecApprovalButton.run()
func (h *DiscordExecApprovalHandler) HandleExecApprovalButtonInteraction(session *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}
	data := i.MessageComponentData()
	parsed := ParseExecApprovalCustomID(data.CustomID)
	if parsed == nil {
		_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "This approval is no longer valid.",
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	decisionLabel := "Denied"
	switch parsed.Action {
	case ExecApprovalAllowOnce:
		decisionLabel = "Allowed (once)"
	case ExecApprovalAllowAlways:
		decisionLabel = "Allowed (always)"
	}

	// Update message immediately
	_ = session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("Submitting decision: **%s**...", decisionLabel),
			Components: []discordgo.MessageComponent{},
		},
	})

	// Resolve via callback
	// W-041 fix: 当 OnResolve 为 nil 时记录警告日志并向用户反馈
	if h.opts.OnResolve != nil {
		if err := h.opts.OnResolve(parsed.ApprovalID, parsed.Action); err != nil {
			h.logger.Error("resolve callback failed", "error", err)
			_, _ = session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "Failed to submit approval decision.",
				Flags:   discordgo.MessageFlagsEphemeral,
			})
		}
	} else {
		h.logger.Warn("OnResolve callback is nil, cannot resolve approval",
			"approvalId", parsed.ApprovalID,
			"action", string(parsed.Action),
		)
		_, _ = session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No approval resolver configured. Decision was not submitted.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
	}
}

// BuildExecApprovalCustomID builds a custom ID for exec approval buttons.
// TS ref: buildExecApprovalCustomId
func BuildExecApprovalCustomID(approvalID string, action ExecApprovalDecision) string {
	return fmt.Sprintf("execapproval:id=%s;action=%s",
		url.QueryEscape(approvalID),
		string(action),
	)
}

// ParsedExecApproval parsed exec approval button data
type ParsedExecApproval struct {
	ApprovalID string
	Action     ExecApprovalDecision
}

// ParseExecApprovalCustomID parses an exec approval custom ID.
// TS ref: parseExecApprovalData
func ParseExecApprovalCustomID(customID string) *ParsedExecApproval {
	if !strings.HasPrefix(customID, "execapproval:") {
		return nil
	}
	rest := customID[len("execapproval:"):]
	params := make(map[string]string)
	for _, part := range strings.Split(rest, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			decoded, err := url.QueryUnescape(kv[1])
			if err != nil {
				decoded = kv[1]
			}
			params[kv[0]] = decoded
		}
	}
	rawID := params["id"]
	rawAction := params["action"]
	if rawID == "" || rawAction == "" {
		return nil
	}
	action := ExecApprovalDecision(rawAction)
	if action != ExecApprovalAllowOnce && action != ExecApprovalAllowAlways && action != ExecApprovalDeny {
		return nil
	}
	return &ParsedExecApproval{ApprovalID: rawID, Action: action}
}

// formatExecApprovalRequestEmbed builds the embed for an approval request.
// TS ref: formatExecApprovalEmbed
func formatExecApprovalRequestEmbed(request *ExecApprovalRequest) *discordgo.MessageEmbed {
	commandText := request.Request.Command
	if len(commandText) > 1000 {
		commandText = commandText[:1000] + "..."
	}
	expiresIn := (request.ExpiresAtMs - time.Now().UnixMilli()) / 1000
	if expiresIn < 0 {
		expiresIn = 0
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Command", Value: "```\n" + commandText + "\n```", Inline: false},
	}
	if request.Request.Cwd != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Working Directory", Value: request.Request.Cwd, Inline: true})
	}
	if request.Request.Host != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Host", Value: request.Request.Host, Inline: true})
	}
	if request.Request.AgentID != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Agent", Value: request.Request.AgentID, Inline: true})
	}

	return &discordgo.MessageEmbed{
		Title:       "Exec Approval Required",
		Description: "A command needs your approval.",
		Color:       0xFFA500,
		Fields:      fields,
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Expires in %ds | ID: %s", expiresIn, request.ID)},
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// formatExecApprovalResolvedEmbed builds the embed for a resolved approval.
// TS ref: formatResolvedEmbed
func formatExecApprovalResolvedEmbed(request *ExecApprovalRequest, decision ExecApprovalDecision, resolvedBy string) *discordgo.MessageEmbed {
	commandText := request.Request.Command
	if len(commandText) > 500 {
		commandText = commandText[:500] + "..."
	}

	decisionLabel := "Denied"
	color := 0xED4245 // red
	switch decision {
	case ExecApprovalAllowOnce:
		decisionLabel = "Allowed (once)"
		color = 0x57F287 // green
	case ExecApprovalAllowAlways:
		decisionLabel = "Allowed (always)"
		color = 0x5865F2 // blurple
	}

	description := "Resolved"
	if resolvedBy != "" {
		description = fmt.Sprintf("Resolved by %s", resolvedBy)
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Exec Approval: %s", decisionLabel),
		Description: description,
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Command", Value: "```\n" + commandText + "\n```", Inline: false},
		},
		Footer:    &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("ID: %s", request.ID)},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// formatExecApprovalExpiredEmbed builds the embed for an expired approval.
// TS ref: formatExpiredEmbed
func formatExecApprovalExpiredEmbed(request *ExecApprovalRequest) *discordgo.MessageEmbed {
	commandText := request.Request.Command
	if len(commandText) > 500 {
		commandText = commandText[:500] + "..."
	}

	return &discordgo.MessageEmbed{
		Title:       "Exec Approval: Expired",
		Description: "This approval request has expired.",
		Color:       0x99AAB5,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Command", Value: "```\n" + commandText + "\n```", Inline: false},
		},
		Footer:    &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("ID: %s", request.ID)},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
