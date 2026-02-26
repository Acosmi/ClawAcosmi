package cron

// ============================================================================
// 隔离 Agent 辅助函数
// 对应 TS: cron/isolated-agent/helpers.ts (83L)
//          cron/isolated-agent/session.ts (37L)
//          cron/isolated-agent/delivery-target.ts (108L, 部分)
// ============================================================================

import (
	"strings"

	"github.com/google/uuid"
)

// ---------- DeliveryPayload (helpers 内部类型) ----------

// DeliveryPayload 投递负载（用于 payload 选择和心跳判断）。
type DeliveryPayload struct {
	Text        string                 `json:"text,omitempty"`
	MediaURL    string                 `json:"mediaUrl,omitempty"`
	MediaURLs   []string               `json:"mediaUrls,omitempty"`
	ChannelData map[string]interface{} `json:"channelData,omitempty"`
}

// ---------- Summary / Payload 选择 ----------

const summaryMaxChars = 2000

// PickSummaryFromOutput 截断文本到 summaryMaxChars 字符（UTF-16 安全）。
// TS 对照: helpers.ts L14-21
func PickSummaryFromOutput(text string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	if len([]rune(clean)) > summaryMaxChars {
		return truncateUTF16Safe(clean, summaryMaxChars) + "…"
	}
	return clean
}

// PickSummaryFromPayloads 从 payloads 倒序取第一个非空 summary。
// TS 对照: helpers.ts L23-31
func PickSummaryFromPayloads(payloads []DeliveryPayload) string {
	for i := len(payloads) - 1; i >= 0; i-- {
		s := PickSummaryFromOutput(payloads[i].Text)
		if s != "" {
			return s
		}
	}
	return ""
}

// PickLastNonEmptyText 从 payloads 倒序取第一个非空 text。
// TS 对照: helpers.ts L33-41
func PickLastNonEmptyText(payloads []DeliveryPayload) string {
	for i := len(payloads) - 1; i >= 0; i-- {
		clean := strings.TrimSpace(payloads[i].Text)
		if clean != "" {
			return clean
		}
	}
	return ""
}

// PickLastDeliverablePayload 从 payloads 倒序取第一个有内容的负载。
// TS 对照: helpers.ts L43-54
func PickLastDeliverablePayload(payloads []DeliveryPayload) *DeliveryPayload {
	for i := len(payloads) - 1; i >= 0; i-- {
		p := &payloads[i]
		text := strings.TrimSpace(p.Text)
		hasMedia := p.MediaURL != "" || len(p.MediaURLs) > 0
		hasChannelData := len(p.ChannelData) > 0
		if text != "" || hasMedia || hasChannelData {
			return p
		}
	}
	return nil
}

// ---------- 心跳过滤 ----------

const defaultHeartbeatAckMaxChars = 120

// IsHeartbeatOnlyResponse 判断所有 payloads 是否只含心跳应答。
// TS 对照: helpers.ts L60-77
//
// 简化实现: 无媒体且文本长度 ≤ ackMaxChars 视为心跳应答。
// 完整实现需 stripHeartbeatToken (auto-reply/heartbeat.ts)，当前以长度判断近似。
func IsHeartbeatOnlyResponse(payloads []DeliveryPayload, ackMaxChars int) bool {
	if len(payloads) == 0 {
		return true
	}
	for _, p := range payloads {
		hasMedia := len(p.MediaURLs) > 0 || p.MediaURL != ""
		if hasMedia {
			return false
		}
		text := strings.TrimSpace(p.Text)
		if len([]rune(text)) > ackMaxChars {
			return false
		}
	}
	return true
}

// ResolveHeartbeatAckMaxChars 解析心跳 ACK 最大字符数。
// TS 对照: helpers.ts L79-82
func ResolveHeartbeatAckMaxChars(configuredValue *int) int {
	if configuredValue != nil && *configuredValue >= 0 {
		return *configuredValue
	}
	return defaultHeartbeatAckMaxChars
}

// ---------- Session 创建 ----------

// CronSessionResult resolveCronSession 的返回值。
type CronSessionResult struct {
	StorePath    string
	SessionID    string
	SessionEntry CronSessionEntry
	IsNewSession bool
}

// CronSessionEntry 隔离 cron session 条目（传递给 session store）。
type CronSessionEntry struct {
	SessionID     string `json:"sessionId"`
	UpdatedAt     int64  `json:"updatedAt"`
	SystemSent    bool   `json:"systemSent"`
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
	VerboseLevel  string `json:"verboseLevel,omitempty"`
	Model         string `json:"model,omitempty"`
	ModelProvider string `json:"modelProvider,omitempty"`
	ContextTokens int    `json:"contextTokens,omitempty"`
	SendPolicy    string `json:"sendPolicy,omitempty"`
	LastChannel   string `json:"lastChannel,omitempty"`
	LastTo        string `json:"lastTo,omitempty"`
	LastAccountID string `json:"lastAccountId,omitempty"`
	Label         string `json:"label,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
	InputTokens   int    `json:"inputTokens,omitempty"`
	OutputTokens  int    `json:"outputTokens,omitempty"`
	TotalTokens   int    `json:"totalTokens,omitempty"`
}

// ResolveCronSession 创建新的 cron session 条目。
// TS 对照: session.ts L5-36
func ResolveCronSession(sessionKey string, nowMs int64) CronSessionResult {
	sessionID := uuid.New().String()
	return CronSessionResult{
		SessionID: sessionID,
		SessionEntry: CronSessionEntry{
			SessionID:  sessionID,
			UpdatedAt:  nowMs,
			SystemSent: false,
		},
		IsNewSession: true,
	}
}

// ResolveCronDeliveryBestEffort 判断投递是否为 best-effort 模式。
// TS 对照: run.ts L90-98
func ResolveCronDeliveryBestEffort(job *CronJob) bool {
	if job.Delivery != nil && job.Delivery.BestEffort != nil {
		return *job.Delivery.BestEffort
	}
	if job.Payload.BestEffortDeliver != nil {
		return *job.Payload.BestEffortDeliver
	}
	return false
}

// ---------- 内部工具 ----------

// truncateUTF16Safe 截断字符串到 maxChars 个 UTF-16 code unit。
// TS 对照: utils.ts → truncateUtf16Safe()
func truncateUTF16Safe(s string, maxChars int) string {
	runes := []rune(s)
	count := 0
	for i, r := range runes {
		// UTF-16 surrogate pair 占 2 个 code unit
		if r >= 0x10000 {
			count += 2
		} else {
			count++
		}
		if count > maxChars {
			return string(runes[:i])
		}
	}
	return s
}

// HasDeliverableStructuredContent 判断负载是否含结构化内容（媒体/channelData）。
func HasDeliverableStructuredContent(p *DeliveryPayload) bool {
	if p == nil {
		return false
	}
	return p.MediaURL != "" || len(p.MediaURLs) > 0 || len(p.ChannelData) > 0
}

// IsExternalHookSession 判断 session key 是否为外部 hook 会话。
// TS 对照: security/external-content.ts L247-253
func IsExternalHookSession(sessionKey string) bool {
	return strings.HasPrefix(sessionKey, "hook:gmail:") ||
		strings.HasPrefix(sessionKey, "hook:webhook:") ||
		strings.HasPrefix(sessionKey, "hook:")
}

// GetHookType 从 session key 提取 hook 来源类型。
// TS 对照: security/external-content.ts L258-269
func GetHookType(sessionKey string) string {
	if strings.HasPrefix(sessionKey, "hook:gmail:") {
		return "email"
	}
	if strings.HasPrefix(sessionKey, "hook:webhook:") {
		return "webhook"
	}
	if strings.HasPrefix(sessionKey, "hook:") {
		return "webhook"
	}
	return "unknown"
}
