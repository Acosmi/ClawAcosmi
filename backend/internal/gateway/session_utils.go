package gateway

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/agents/models"
	scope "github.com/anthropic/open-acosmi/internal/agents/scope"
	"github.com/anthropic/open-acosmi/internal/session"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- 常量 ----------

const derivedTitleMaxLen = 60

// avatarMaxBytes 头像文件最大 2MB。
const avatarMaxBytes = 2 * 1024 * 1024

// MIME 映射表。
var avatarMimeByExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".tif":  "image/tiff",
	".tiff": "image/tiff",
}

// 正则预编译。
var (
	avatarDataRE   = regexp.MustCompile(`(?i)^data:`)
	avatarHTTPRE   = regexp.MustCompile(`(?i)^https?://`)
	avatarSchemeRE = regexp.MustCompile(`^[a-z][a-z0-9+.\-]*:`)
	windowsAbsRE   = regexp.MustCompile(`^[a-zA-Z]:[/\\]`)
	cronRunRE      = regexp.MustCompile(`^cron:[^:]+:run:[^:]+$`)
	groupLabelRE   = regexp.MustCompile(`[^a-z0-9#@._+\-]+`)
	groupDashesRE  = regexp.MustCompile(`-{2,}`)
	groupEdgesRE   = regexp.MustCompile(`^[-.]+|[-.]+$`)
)

// ---------- A. Session Key 工具函数 ----------

// ClassifySessionKey 对会话 key 进行分类。
func ClassifySessionKey(key string, entry *SessionEntry) string {
	if key == "global" {
		return "global"
	}
	if key == "unknown" {
		return "unknown"
	}
	if entry != nil && (entry.ChatType == "group" || entry.ChatType == "channel") {
		return "group"
	}
	if strings.Contains(key, ":group:") || strings.Contains(key, ":channel:") {
		return "group"
	}
	return "direct"
}

// ParseGroupKey 解析群组 key 结构 (channel:group:id)。
// 返回 nil 表示不是群组 key。
func ParseGroupKey(key string) *GroupKeyParts {
	rawKey := key
	if parsed := parseAgentSessionKeySimple(key); parsed != nil {
		rawKey = parsed.rest
	}
	parts := splitNonEmpty(rawKey, ":")
	if len(parts) >= 3 {
		kind := parts[1]
		if kind == "group" || kind == "channel" {
			id := strings.Join(parts[2:], ":")
			return &GroupKeyParts{
				Channel: parts[0],
				Kind:    kind,
				ID:      id,
			}
		}
	}
	return nil
}

// IsCronRunSessionKey 判断是否为 cron 运行会话 key。
func IsCronRunSessionKey(key string) bool {
	rawKey := key
	if parsed := parseAgentSessionKeySimple(key); parsed != nil {
		rawKey = parsed.rest
	}
	return cronRunRE.MatchString(rawKey)
}

// CanonicalizeSessionKeyForAgent 为 key 添加 agent: 前缀。
func CanonicalizeSessionKeyForAgent(agentId, key string) string {
	if key == "global" || key == "unknown" {
		return key
	}
	if strings.HasPrefix(key, "agent:") {
		return key
	}
	return fmt.Sprintf("agent:%s:%s", NormalizeAgentId(agentId), key)
}

// CanonicalizeSpawnedByForAgent 规范化 spawnedBy 字段。
func CanonicalizeSpawnedByForAgent(agentId, spawnedBy string) string {
	raw := strings.TrimSpace(spawnedBy)
	if raw == "" {
		return ""
	}
	if raw == "global" || raw == "unknown" {
		return raw
	}
	if strings.HasPrefix(raw, "agent:") {
		return raw
	}
	return fmt.Sprintf("agent:%s:%s", NormalizeAgentId(agentId), raw)
}

// ResolveDefaultStoreAgentId 解析默认 store agent ID。
// 对齐 TS: session-utils.ts resolveDefaultStoreAgentId()
func ResolveDefaultStoreAgentId(cfg *types.OpenAcosmiConfig) string {
	return NormalizeAgentId(scope.ResolveDefaultAgentId(cfg))
}

// ResolveSessionStoreKey 规范化 session key 为 store 使用的标准形式。
// 对齐 TS: session-utils.ts resolveSessionStoreKey()
func ResolveSessionStoreKey(cfg *types.OpenAcosmiConfig, sessionKey string) string {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return raw
	}
	if raw == "global" || raw == "unknown" {
		return raw
	}

	// 已带 agent: 前缀 → 尝试别名规范化
	if parsed := parseAgentSessionKeySimple(raw); parsed != nil {
		agentId := NormalizeAgentId(parsed.agentId)
		canonical := canonicalizeMainSessionAlias(cfg, agentId, raw)
		if canonical != raw {
			return canonical
		}
		return raw
	}

	// 裸键: 检查是否为 main 别名
	mainKey := resolveConfigMainKey(cfg)
	if raw == "main" || raw == mainKey {
		return resolveMainSessionKeyForGateway(cfg)
	}

	// 裸键: 添加默认 agent 前缀
	agentId := ResolveDefaultStoreAgentId(cfg)
	return CanonicalizeSessionKeyForAgent(agentId, raw)
}

// ResolveSessionStoreAgentId 从 canonical key 解析 agent ID。
// 对齐 TS: session-utils.ts resolveSessionStoreAgentId()
func ResolveSessionStoreAgentId(cfg *types.OpenAcosmiConfig, canonicalKey string) string {
	if canonicalKey == "global" || canonicalKey == "unknown" {
		return ResolveDefaultStoreAgentId(cfg)
	}
	if parsed := parseAgentSessionKeySimple(canonicalKey); parsed != nil && parsed.agentId != "" {
		return NormalizeAgentId(parsed.agentId)
	}
	return ResolveDefaultStoreAgentId(cfg)
}

// canonicalizeMainSessionAlias 将 main 别名规范化为 agent 主会话键。
// 对齐 TS: main-session.ts canonicalizeMainSessionAlias()
func canonicalizeMainSessionAlias(cfg *types.OpenAcosmiConfig, agentId, sessionKey string) string {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return raw
	}

	aid := NormalizeAgentId(agentId)
	mainKey := resolveConfigMainKey(cfg)
	agentMainSessionKey := fmt.Sprintf("agent:%s:%s", aid, mainKey)
	agentMainAliasKey := fmt.Sprintf("agent:%s:main", aid)

	isMainAlias := raw == "main" || raw == mainKey ||
		raw == agentMainSessionKey || raw == agentMainAliasKey

	if cfg != nil && cfg.Session != nil && string(cfg.Session.Scope) == "global" && isMainAlias {
		return "global"
	}
	if isMainAlias {
		return agentMainSessionKey
	}
	return raw
}

// resolveMainSessionKeyForGateway 从配置解析主会话键。
// 对齐 TS: config/sessions.ts resolveMainSessionKey()
func resolveMainSessionKeyForGateway(cfg *types.OpenAcosmiConfig) string {
	if cfg != nil && cfg.Session != nil && string(cfg.Session.Scope) == "global" {
		return "global"
	}
	agentId := ResolveDefaultStoreAgentId(cfg)
	mainKey := resolveConfigMainKey(cfg)
	return fmt.Sprintf("agent:%s:%s", agentId, mainKey)
}

// resolveConfigMainKey 从配置提取 mainKey（默认 "main"）。
func resolveConfigMainKey(cfg *types.OpenAcosmiConfig) string {
	if cfg != nil && cfg.Session != nil && cfg.Session.MainKey != "" {
		return NormalizeMainKey(cfg.Session.MainKey)
	}
	return "main"
}

// IsStorePathTemplate 检查 store 路径是否含 {agentId} 占位符。
func IsStorePathTemplate(store string) bool {
	return strings.Contains(store, "{agentId}")
}

// NormalizeAgentId 标准化 agent ID（小写 + trim）。
func NormalizeAgentId(id string) string {
	return strings.TrimSpace(strings.ToLower(id))
}

// NormalizeMainKey 标准化 mainKey。
func NormalizeMainKey(mainKey string) string {
	return strings.TrimSpace(mainKey)
}

// ---------- B. Title / Avatar 函数 ----------

// FormatSessionIdPrefix 生成会话 ID 前缀显示文本。
func FormatSessionIdPrefix(sessionId string, updatedAt int64) string {
	prefix := sessionId
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	if updatedAt > 0 {
		t := time.UnixMilli(updatedAt)
		date := t.Format("2006-01-02")
		return fmt.Sprintf("%s (%s)", prefix, date)
	}
	return prefix
}

// TruncateTitle 截断标题文本，保留词边界。
func TruncateTitle(text string, maxLen int) string {
	if len([]rune(text)) <= maxLen {
		return text
	}
	runes := []rune(text)
	cut := string(runes[:maxLen-1])
	lastSpace := strings.LastIndex(cut, " ")
	threshold := int(float64(maxLen) * 0.6)
	if lastSpace > threshold {
		return cut[:lastSpace] + "…"
	}
	return cut + "…"
}

// DeriveSessionTitle 推导会话标题（优先级：displayName > subject > firstUserMsg > sessionId）。
func DeriveSessionTitle(entry *SessionEntry, firstUserMsg string) string {
	if entry == nil {
		return ""
	}
	if dn := strings.TrimSpace(entry.DisplayName); dn != "" {
		return dn
	}
	if subj := strings.TrimSpace(entry.Subject); subj != "" {
		return subj
	}
	if msg := strings.TrimSpace(firstUserMsg); msg != "" {
		normalized := collapseWhitespace(msg)
		return TruncateTitle(normalized, derivedTitleMaxLen)
	}
	if entry.SessionId != "" {
		return FormatSessionIdPrefix(entry.SessionId, entry.UpdatedAt)
	}
	return ""
}

// ResolveAvatarMime 根据文件扩展名解析 MIME 类型。
func ResolveAvatarMime(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if mime, ok := avatarMimeByExt[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// IsWorkspaceRelativePath 判断路径是否为工作空间相对路径。
func IsWorkspaceRelativePath(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "~") {
		return false
	}
	if avatarSchemeRE.MatchString(value) && !windowsAbsRE.MatchString(value) {
		return false
	}
	return true
}

// ResolveIdentityAvatarUrl 解析身份头像 URL，支持 data URI、HTTP URL 和本地文件。
func ResolveIdentityAvatarUrl(workspaceDir, agentId, avatar string) string {
	trimmed := strings.TrimSpace(avatar)
	if trimmed == "" {
		return ""
	}
	if avatarDataRE.MatchString(trimmed) || avatarHTTPRE.MatchString(trimmed) {
		return trimmed
	}
	if !IsWorkspaceRelativePath(trimmed) {
		return ""
	}
	workspaceRoot, _ := filepath.Abs(workspaceDir)
	resolved, _ := filepath.Abs(filepath.Join(workspaceRoot, trimmed))
	rel, err := filepath.Rel(workspaceRoot, resolved)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return ""
	}
	stat, err := os.Stat(resolved)
	if err != nil || stat.IsDir() || stat.Size() > avatarMaxBytes {
		return ""
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ""
	}
	mime := ResolveAvatarMime(resolved)
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
}

// ---------- C. groups / delivery / model 函数 ----------

// DeliveryFieldsResult normalizeSessionDeliveryFields 返回结果。
type DeliveryFieldsResult struct {
	DeliveryContext *session.DeliveryContext
	LastChannel     string
	LastTo          string
	LastAccountId   string
}

// BuildGroupDisplayName 根据 provider/subject/groupChannel/space/id 构建群组显示名。
// 对齐 TS: config/sessions/group.ts buildGroupDisplayName
func BuildGroupDisplayName(provider, subject, groupChannel, space, id, key string) string {
	providerKey := strings.TrimSpace(strings.ToLower(provider))
	if providerKey == "" {
		providerKey = "group"
	}
	gc := strings.TrimSpace(groupChannel)
	sp := strings.TrimSpace(space)
	subj := strings.TrimSpace(subject)

	// detail = space#groupChannel | groupChannel | subject | space
	var detail string
	if gc != "" && sp != "" {
		prefix := ""
		if !strings.HasPrefix(gc, "#") {
			prefix = "#"
		}
		detail = sp + prefix + gc
	} else if gc != "" {
		detail = gc
	} else if subj != "" {
		detail = subj
	} else if sp != "" {
		detail = sp
	}

	fallbackID := strings.TrimSpace(id)
	if fallbackID == "" {
		fallbackID = key
	}
	rawLabel := detail
	if rawLabel == "" {
		rawLabel = fallbackID
	}

	token := normalizeGroupLabel(rawLabel)
	if token == "" {
		token = normalizeGroupLabel(shortenGroupId(rawLabel))
	}
	if groupChannel == "" && strings.HasPrefix(token, "#") {
		token = strings.TrimLeft(token, "#")
	}
	if token != "" && !strings.HasPrefix(token, "@") && !strings.HasPrefix(token, "#") &&
		!strings.HasPrefix(token, "g-") && !strings.Contains(token, "#") {
		token = "g-" + token
	}
	if token != "" {
		return providerKey + ":" + token
	}
	return providerKey
}

// normalizeGroupLabel 对齐 TS normalizeGroupLabel — 标准化群组标签。
func normalizeGroupLabel(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}
	dashed := strings.ReplaceAll(trimmed, " ", "-")
	cleaned := groupLabelRE.ReplaceAllString(dashed, "-")
	cleaned = groupDashesRE.ReplaceAllString(cleaned, "-")
	cleaned = groupEdgesRE.ReplaceAllString(cleaned, "")
	return cleaned
}

// shortenGroupId 对齐 TS shortenGroupId — 缩短过长的群组 ID。
func shortenGroupId(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 14 {
		return trimmed
	}
	return trimmed[:6] + "..." + trimmed[len(trimmed)-4:]
}

// NormalizeSessionDeliveryFields 合并 session entry 的投递字段。
// 对齐 TS: utils/delivery-context.ts normalizeSessionDeliveryFields
func NormalizeSessionDeliveryFields(entry *SessionEntry) DeliveryFieldsResult {
	if entry == nil {
		return DeliveryFieldsResult{}
	}

	// Primary: lastChannel/lastTo/lastAccountId 字段
	primaryChannel := ""
	if entry.LastChannel != nil {
		primaryChannel = entry.LastChannel.Channel
	}
	if primaryChannel == "" {
		primaryChannel = entry.Channel
	}
	primaryTo := entry.LastTo
	primaryAccountId := entry.LastAccountId

	// Fallback: deliveryContext
	var fallbackChannel, fallbackTo, fallbackAccountId string
	if entry.DeliveryContext != nil {
		fallbackChannel = entry.DeliveryContext.Channel
		fallbackTo = entry.DeliveryContext.To
		fallbackAccountId = entry.DeliveryContext.AccountId
	}

	// Merge: primary wins, fallback fills gaps
	mergedChannel := primaryChannel
	if mergedChannel == "" {
		mergedChannel = fallbackChannel
	}
	mergedTo := primaryTo
	if mergedTo == "" {
		mergedTo = fallbackTo
	}
	mergedAccountId := primaryAccountId
	if mergedAccountId == "" {
		mergedAccountId = fallbackAccountId
	}

	// If all empty, return zero
	if mergedChannel == "" && mergedTo == "" && mergedAccountId == "" {
		return DeliveryFieldsResult{}
	}

	var dc *session.DeliveryContext
	if mergedChannel != "" || mergedTo != "" || mergedAccountId != "" {
		dc = &session.DeliveryContext{
			Channel:   mergedChannel,
			To:        mergedTo,
			AccountId: mergedAccountId,
		}
	}

	return DeliveryFieldsResult{
		DeliveryContext: dc,
		LastChannel:     mergedChannel,
		LastTo:          mergedTo,
		LastAccountId:   mergedAccountId,
	}
}

// ResolveSessionModelRef 解析 session 的模型引用（含 entry override）。
// 对齐 TS: session-utils.ts resolveSessionModelRef
func ResolveSessionModelRef(cfg *types.OpenAcosmiConfig, entry *SessionEntry, agentId string) (provider, model string) {
	var ref models.ModelRef
	if agentId != "" {
		ref = models.ResolveDefaultModelForAgent(cfg, agentId)
	} else {
		ref = models.ResolveConfiguredModelRef(cfg, models.DefaultProvider, models.DefaultModel)
	}
	provider = ref.Provider
	model = ref.Model

	// Apply session-level overrides
	if entry != nil {
		if mo := strings.TrimSpace(entry.ModelOverride); mo != "" {
			if po := strings.TrimSpace(entry.ProviderOverride); po != "" {
				provider = po
			}
			model = mo
		}
	}
	return provider, model
}

// ---------- 内部辅助 ----------

// agentKeyParsed 是 parseAgentSessionKey 的简化返回。
type agentKeyParsed struct {
	agentId string
	rest    string
}

// parseAgentSessionKeySimple 简化版解析 "agent:<id>:<rest>"。
func parseAgentSessionKeySimple(key string) *agentKeyParsed {
	if !strings.HasPrefix(key, "agent:") {
		return nil
	}
	after := key[6:] // 去掉 "agent:"
	idx := strings.Index(after, ":")
	if idx < 0 {
		return &agentKeyParsed{agentId: after, rest: ""}
	}
	return &agentKeyParsed{
		agentId: after[:idx],
		rest:    after[idx+1:],
	}
}

// splitNonEmpty 按分隔符拆分，过滤空串。
func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// collapseWhitespace 合并连续空白为单个空格。
func collapseWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
