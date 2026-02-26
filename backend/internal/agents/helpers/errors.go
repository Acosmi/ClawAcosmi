package helpers

import (
	"regexp"
	"strings"
)

// ---------- 错误分类 ----------

// TS 参考: src/agents/pi-embedded-helpers/errors.ts (669 行)

// BillingErrorUserMessage 账单错误用户消息。
const BillingErrorUserMessage = "⚠️ API provider returned a billing error — your API key has run out of credits or has an insufficient balance. Check your provider's billing dashboard and top up or switch to a different API key."

// ---------- 上下文溢出 ----------

var (
	contextWindowSmallRE  = regexp.MustCompile(`(?i)context window.*(too small|minimum is)`)
	contextOverflowHintRE = regexp.MustCompile(`(?i)context.*overflow|context window.*(too (?:large|long)|exceed|over|limit|max(?:imum)?|requested|sent|tokens)|(?:prompt|request|input).*(too (?:large|long)|exceed|over|limit|max(?:imum)?)`)
	contextOverflowHeadRE = regexp.MustCompile(`(?i)^(?:context overflow:|request_too_large\b|request size exceeds\b|request exceeds the maximum size\b|context length exceeded\b|maximum context length\b|prompt is too long\b|exceeds model context window\b)`)
)

// IsContextOverflowError 检查是否上下文溢出错误。
// TS 参考: errors.ts L9-30 — 使用精确 includes 检查
func IsContextOverflowError(errorMessage string) bool {
	if errorMessage == "" {
		return false
	}
	lower := strings.ToLower(errorMessage)
	if strings.Contains(lower, "request_too_large") {
		return true
	}
	if strings.Contains(lower, "request exceeds the maximum size") {
		return true
	}
	if strings.Contains(lower, "context length exceeded") {
		return true
	}
	if strings.Contains(lower, "maximum context length") {
		return true
	}
	if strings.Contains(lower, "prompt is too long") {
		return true
	}
	if strings.Contains(lower, "exceeds model context window") {
		return true
	}
	if strings.Contains(lower, "request size exceeds") &&
		(strings.Contains(lower, "context window") || strings.Contains(lower, "context length")) {
		return true
	}
	if strings.Contains(lower, "context overflow:") {
		return true
	}
	if strings.Contains(lower, "413") && strings.Contains(lower, "too large") {
		return true
	}
	return false
}

// IsLikelyContextOverflowError 检查是否可能是上下文溢出。
// TS 参考: errors.ts L36-47
func IsLikelyContextOverflowError(errorMessage string) bool {
	if errorMessage == "" {
		return false
	}
	// TS: 排除 "context window too small" — 不是溢出
	if contextWindowSmallRE.MatchString(errorMessage) {
		return false
	}
	if IsContextOverflowError(errorMessage) {
		return true
	}
	return contextOverflowHintRE.MatchString(errorMessage)
}

// ---------- 压缩失败 ----------

// IsCompactionFailureError 检查是否压缩失败。
// TS 参考: errors.ts L49-63 — 需要同时是上下文溢出错误
func IsCompactionFailureError(errorMessage string) bool {
	if errorMessage == "" {
		return false
	}
	// TS: 前置条件 — 必须先是上下文溢出错误
	if !IsContextOverflowError(errorMessage) {
		return false
	}
	lower := strings.ToLower(errorMessage)
	return strings.Contains(lower, "summarization failed") ||
		strings.Contains(lower, "auto-compaction") ||
		strings.Contains(lower, "compaction failed") ||
		strings.Contains(lower, "compaction")
}

// ---------- 错误模式匹配 ----------

// ErrorPattern 错误模式（正则或字符串）。
type ErrorPattern struct {
	RE     *regexp.Regexp
	Substr string
}

func re(pattern string) ErrorPattern {
	return ErrorPattern{RE: regexp.MustCompile("(?i)" + pattern)}
}

func substr(s string) ErrorPattern {
	return ErrorPattern{Substr: strings.ToLower(s)}
}

func matchesPatterns(raw string, patterns []ErrorPattern) bool {
	lower := strings.ToLower(raw)
	for _, p := range patterns {
		if p.RE != nil && p.RE.MatchString(raw) {
			return true
		}
		if p.Substr != "" && strings.Contains(lower, p.Substr) {
			return true
		}
	}
	return false
}

// 错误模式集合
var (
	rateLimitPatterns = []ErrorPattern{
		re(`rate[_ ]limit|too many requests|429`),
		substr("exceeded your current quota"),
		substr("resource has been exhausted"),
		substr("quota exceeded"),
		substr("resource_exhausted"),
		substr("usage limit"),
	}
	timeoutPatterns = []ErrorPattern{
		re(`timeout|timed out|deadline exceeded|context deadline exceeded`),
		substr("ETIMEDOUT"),
		substr("ESOCKETTIMEDOUT"),
	}
	billingPatterns = []ErrorPattern{
		re(`\b402\b`),
		substr("payment required"),
		substr("insufficient credits"),
		substr("credit balance"),
		substr("plans & billing"),
	}
	authPatterns = []ErrorPattern{
		re(`invalid[_ ]?api[_ ]?key`),
		substr("incorrect api key"),
		substr("invalid token"),
		substr("authentication"),
		substr("re-authenticate"),
		substr("oauth token refresh failed"),
		substr("unauthorized"),
		substr("forbidden"),
		substr("access denied"),
		substr("expired"),
		substr("token has expired"),
		re(`\b401\b`),
		re(`\b403\b`),
		substr("no credentials found"),
		substr("no api key found"),
	}
	overloadPatterns = []ErrorPattern{
		re(`overloaded_error|"type"\s*:\s*"overloaded_error"`),
		substr("overloaded"),
	}
	formatPatterns = []ErrorPattern{
		substr("string should match pattern"),
		substr("tool_use.id"),
		substr("tool_use_id"),
		substr("messages.1.content.1.tool_use.id"),
		substr("invalid request format"),
	}
)

// IsRateLimitErrorMessage 检查是否限流错误。
func IsRateLimitErrorMessage(raw string) bool {
	return matchesPatterns(raw, rateLimitPatterns)
}

// IsTimeoutErrorMessage 检查是否超时错误。
func IsTimeoutErrorMessage(raw string) bool {
	return matchesPatterns(raw, timeoutPatterns)
}

// IsBillingErrorMessage 检查是否账单错误。
// TS 参考: errors.ts L529-544
func IsBillingErrorMessage(raw string) bool {
	if raw == "" {
		return false
	}
	if matchesPatterns(raw, billingPatterns) {
		return true
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "billing") {
		return strings.Contains(lower, "upgrade") ||
			strings.Contains(lower, "credits") ||
			strings.Contains(lower, "payment") ||
			strings.Contains(lower, "plan")
	}
	return false
}

// IsAuthErrorMessage 检查是否认证错误。
func IsAuthErrorMessage(raw string) bool {
	return matchesPatterns(raw, authPatterns)
}

// IsOverloadedErrorMessage 检查是否过载错误。
func IsOverloadedErrorMessage(raw string) bool {
	return matchesPatterns(raw, overloadPatterns)
}

// IsFailoverErrorMessage 检查是否可切换错误。
func IsFailoverErrorMessage(raw string) bool {
	return IsRateLimitErrorMessage(raw) ||
		IsBillingErrorMessage(raw) ||
		IsTimeoutErrorMessage(raw) ||
		IsAuthErrorMessage(raw) ||
		IsOverloadedErrorMessage(raw)
}

// IsCloudCodeAssistFormatError 检查是否 API 格式错误。
// TS 参考: errors.ts L620-622 — 排除 image 维度错误
func IsCloudCodeAssistFormatError(raw string) bool {
	if IsImageDimensionErrorMessage(raw) {
		return false
	}
	return matchesPatterns(raw, formatPatterns)
}

// ---------- classifyFailoverReason ----------

// TS 参考: errors.ts → 被 failover-error.ts 引用

// ClassifyFailoverReason 从错误消息分类失败切换原因。
// TS 参考: errors.ts L631-657 — 优先级: image排除 → rate_limit → overloaded → format → billing → timeout → auth
func ClassifyFailoverReason(message string) string {
	if message == "" {
		return ""
	}
	// 图像错误不触发 failover
	if IsImageDimensionErrorMessage(message) || IsImageSizeError(message) {
		return ""
	}
	switch {
	case IsRateLimitErrorMessage(message):
		return "rate_limit"
	case IsOverloadedErrorMessage(message):
		return "rate_limit" // TS 将 overloaded 归类为 rate_limit
	case IsCloudCodeAssistFormatError(message):
		return "format"
	case IsBillingErrorMessage(message):
		return "billing"
	case IsTimeoutErrorMessage(message):
		return "timeout"
	case IsAuthErrorMessage(message):
		return "auth"
	default:
		return ""
	}
}

// ---------- 连续重复块折叠 ----------

var multiNewlineRE = regexp.MustCompile(`\n{2,}`)
var multiSpaceRE = regexp.MustCompile(`\s+`)

// CollapseConsecutiveDuplicateBlocks 折叠连续重复的文本块。
// TS 参考: errors.ts L98-125
func CollapseConsecutiveDuplicateBlocks(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text
	}
	blocks := multiNewlineRE.Split(trimmed, -1)
	if len(blocks) < 2 {
		return text
	}
	normalizeBlock := func(s string) string {
		return multiSpaceRE.ReplaceAllString(strings.TrimSpace(s), " ")
	}
	var result []string
	lastNormalized := ""
	for _, block := range blocks {
		normalized := normalizeBlock(block)
		if lastNormalized != "" && normalized == lastNormalized {
			continue
		}
		result = append(result, strings.TrimSpace(block))
		lastNormalized = normalized
	}
	if len(result) == len(blocks) {
		return text
	}
	return strings.Join(result, "\n\n")
}

// ---------- 用户可见文本清洗 ----------

var (
	errorPayloadPrefixRE = regexp.MustCompile(`(?i)^(?:error|api\s*error|apierror|openai\s*error|anthropic\s*error|gateway\s*error)[:\s-]+`)
	finalTagRE           = regexp.MustCompile(`(?i)<\s*\/?\s*final\s*>`)
	controlCharRE        = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`)
	httpStatusPrefixRE   = regexp.MustCompile(`^(?:http\s*)?(\d{3})\s+(.+)$`)
	errorPrefixRE        = regexp.MustCompile(`(?i)^(?:error|api\s*error|openai\s*error|anthropic\s*error|gateway\s*error|request failed|failed|exception)[:\s-]+`)
)

var httpErrorHints = []string{
	"error", "bad request", "not found", "unauthorized", "forbidden",
	"internal server", "service unavailable", "gateway", "rate limit",
	"overloaded", "timeout", "timed out", "invalid", "too many requests", "permission",
}

// ApiErrorInfo API 错误解析结果。
type ApiErrorInfo struct {
	HttpCode  string
	Type      string
	Message   string
	RequestId string
}

// ParseApiErrorInfo 解析 API 错误 JSON 信息。
// TS 参考: errors.ts L244-298
func ParseApiErrorInfo(raw string) *ApiErrorInfo {
	if raw == "" {
		return nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var httpCode string
	candidate := trimmed

	// 提取 HTTP 状态码前缀
	if m := httpStatusPrefixRE.FindStringSubmatch(candidate); len(m) > 2 {
		httpCode = m[1]
		candidate = strings.TrimSpace(m[2])
	}

	// 尝试解析 JSON payload
	payload := parseApiErrorPayload(candidate)
	if payload == nil {
		return nil
	}

	return &ApiErrorInfo{
		HttpCode:  httpCode,
		Type:      payload.errType,
		Message:   payload.errMessage,
		RequestId: payload.requestId,
	}
}

type errorPayload struct {
	errType    string
	errMessage string
	requestId  string
}

func parseApiErrorPayload(raw string) *errorPayload {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	candidates := []string{trimmed}
	if errorPayloadPrefixRE.MatchString(trimmed) {
		stripped := strings.TrimSpace(errorPayloadPrefixRE.ReplaceAllString(trimmed, ""))
		if stripped != "" {
			candidates = append(candidates, stripped)
		}
	}
	for _, c := range candidates {
		if !strings.HasPrefix(c, "{") || !strings.HasSuffix(c, "}") {
			continue
		}
		// 简单 JSON 字段提取（避免引入 encoding/json 依赖复杂度）
		p := extractErrorFields(c)
		if p != nil {
			return p
		}
	}
	return nil
}

func extractErrorFields(jsonStr string) *errorPayload {
	// 检查是否看起来像 error payload
	hasTypeError := strings.Contains(jsonStr, `"type":"error"`) || strings.Contains(jsonStr, `"type": "error"`)
	hasRequestId := strings.Contains(jsonStr, `"request_id"`) || strings.Contains(jsonStr, `"requestId"`)
	hasErrorField := strings.Contains(jsonStr, `"error"`)

	if !hasTypeError && !hasRequestId && !hasErrorField {
		return nil
	}

	result := &errorPayload{}
	result.requestId = extractJSONStringField(jsonStr, "request_id")
	if result.requestId == "" {
		result.requestId = extractJSONStringField(jsonStr, "requestId")
	}

	// 提取顶层 type 和 message
	topType := extractJSONStringField(jsonStr, "type")
	topMessage := extractJSONStringField(jsonStr, "message")

	// 提取嵌套 error 对象的 type/code/message
	errType := ""
	errMessage := ""
	if idx := strings.Index(jsonStr, `"error"`); idx >= 0 {
		// 查找 error 对象范围
		rest := jsonStr[idx:]
		if braceIdx := strings.Index(rest, "{"); braceIdx >= 0 {
			inner := extractInnerObject(rest[braceIdx:])
			if inner != "" {
				if t := extractJSONStringField(inner, "type"); t != "" {
					errType = t
				}
				if errType == "" {
					if c := extractJSONStringField(inner, "code"); c != "" {
						errType = c
					}
				}
				if m := extractJSONStringField(inner, "message"); m != "" {
					errMessage = m
				}
			}
		}
	}

	if errType != "" {
		result.errType = errType
	} else {
		result.errType = topType
	}
	if errMessage != "" {
		result.errMessage = errMessage
	} else {
		result.errMessage = topMessage
	}

	if result.errType == "" && result.errMessage == "" && result.requestId == "" {
		return nil
	}
	return result
}

func extractJSONStringField(json, field string) string {
	key := `"` + field + `"`
	idx := strings.Index(json, key)
	if idx < 0 {
		return ""
	}
	rest := json[idx+len(key):]
	// Skip : and whitespace
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 || rest[0] != ':' {
		return ""
	}
	rest = strings.TrimSpace(rest[1:])
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	// Extract string value
	end := 1
	for end < len(rest) {
		if rest[end] == '\\' {
			end += 2
			continue
		}
		if rest[end] == '"' {
			return rest[1:end]
		}
		end++
	}
	return ""
}

func extractInnerObject(s string) string {
	if len(s) == 0 || s[0] != '{' {
		return ""
	}
	depth := 0
	for i, c := range s {
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

// IsRawApiErrorPayload 检查原始文本是否为 API 错误 JSON payload。
func IsRawApiErrorPayload(raw string) bool {
	return parseApiErrorPayload(strings.TrimSpace(raw)) != nil
}

// GetApiErrorPayloadFingerprint 生成 API 错误 payload 的稳定指纹。
// TS 参考: errors.ts L222-231 (stableStringify + getApiErrorPayloadFingerprint)
func GetApiErrorPayloadFingerprint(raw string) string {
	if raw == "" {
		return ""
	}
	payload := parseApiErrorPayload(strings.TrimSpace(raw))
	if payload == nil {
		return ""
	}
	// 构建确定性指纹: sorted fields
	var parts []string
	if payload.errMessage != "" {
		parts = append(parts, "m:"+payload.errMessage)
	}
	if payload.requestId != "" {
		parts = append(parts, "r:"+payload.requestId)
	}
	if payload.errType != "" {
		parts = append(parts, "t:"+payload.errType)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "|")
}

func isLikelyHttpErrorText(raw string) bool {
	m := httpStatusPrefixRE.FindStringSubmatch(raw)
	if m == nil {
		return false
	}
	code := parseInt(m[1])
	if code < 400 {
		return false
	}
	msg := strings.ToLower(m[2])
	for _, hint := range httpErrorHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}
	return false
}

// SanitizeUserFacingText 清洗面向用户的文本。
// TS 参考: errors.ts L403-446
func SanitizeUserFacingText(text string) string {
	if text == "" {
		return text
	}
	// 移除 <final> 标签
	result := finalTagRE.ReplaceAllString(text, "")
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return result
	}

	// 角色排序冲突
	if roleConflictRE.MatchString(trimmed) {
		return "Message ordering conflict - please try again. " +
			"If this persists, use /new to start a fresh session."
	}

	// 上下文溢出重写
	// TS: shouldRewriteContextOverflowText() — 4 个条件
	if IsContextOverflowError(trimmed) &&
		(IsRawApiErrorPayload(trimmed) || isLikelyHttpErrorText(trimmed) ||
			errorPrefixRE.MatchString(trimmed) || contextOverflowHeadRE.MatchString(trimmed)) {
		return "Context overflow: prompt too large for the model. " +
			"Try again with less input or a larger-context model."
	}

	// 账单错误
	if IsBillingErrorMessage(trimmed) {
		return BillingErrorUserMessage
	}

	// API 错误 payload
	if IsRawApiErrorPayload(trimmed) || isLikelyHttpErrorText(trimmed) {
		return FormatRawAssistantErrorForUi(trimmed)
	}

	// 错误前缀
	if errorPrefixRE.MatchString(trimmed) {
		if IsOverloadedErrorMessage(trimmed) || IsRateLimitErrorMessage(trimmed) {
			return "The AI service is temporarily overloaded. Please try again in a moment."
		}
		if IsTimeoutErrorMessage(trimmed) {
			return "LLM request timed out."
		}
		return FormatRawAssistantErrorForUi(trimmed)
	}
	// TS L445: collapseConsecutiveDuplicateBlocks(stripped)
	return CollapseConsecutiveDuplicateBlocks(result)
}

var roleConflictRE = regexp.MustCompile(`(?i)incorrect role information|roles must alternate`)

// FormatRawAssistantErrorForUi 格式化原始助手错误。
// TS 参考: errors.ts L300-323
func FormatRawAssistantErrorForUi(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "LLM request failed with an unknown error."
	}

	// HTTP 状态码 + 非 JSON 文本
	if m := httpStatusPrefixRE.FindStringSubmatch(trimmed); len(m) > 2 {
		rest := strings.TrimSpace(m[2])
		if !strings.HasPrefix(rest, "{") {
			return "HTTP " + m[1] + ": " + rest
		}
	}

	// 结构化 API 错误
	info := ParseApiErrorInfo(trimmed)
	if info != nil && info.Message != "" {
		prefix := "LLM error"
		if info.HttpCode != "" {
			prefix = "HTTP " + info.HttpCode
		}
		typ := ""
		if info.Type != "" {
			typ = " " + info.Type
		}
		reqId := ""
		if info.RequestId != "" {
			reqId = " (request_id: " + info.RequestId + ")"
		}
		return prefix + typ + ": " + info.Message + reqId
	}

	// 截断过长错误
	if len(trimmed) > 600 {
		return trimmed[:600] + "…"
	}
	return trimmed
}

// ---------- Tool Call Input 缺失 ----------

var (
	toolCallInputMissingRE = regexp.MustCompile(`(?i)tool_(?:use|call)\.(?:input|arguments).*?(?:field required|required)`)
	toolCallInputPathRE    = regexp.MustCompile(`(?i)messages\.\d+\.content\.\d+\.tool_(?:use|call)\.(?:input|arguments)`)
)

// IsMissingToolCallInputError 检查是否 tool call input 缺失错误。
func IsMissingToolCallInputError(raw string) bool {
	if raw == "" {
		return false
	}
	return toolCallInputMissingRE.MatchString(raw) || toolCallInputPathRE.MatchString(raw)
}

// ---------- 图像错误解析 ----------

var (
	imageDimensionErrorRE = regexp.MustCompile(`(?i)image dimensions exceed max allowed size.*?(\d+)\s*pixels`)
	imageDimensionPathRE  = regexp.MustCompile(`(?i)messages\.(\d+)\.content\.(\d+)\.image`)
	imageSizeErrorRE      = regexp.MustCompile(`(?i)image exceeds\s*(\d+(?:\.\d+)?)\s*mb`)
)

// ImageDimensionError 图像维度错误解析结果。
type ImageDimensionError struct {
	MaxDimensionPx int
	MessageIndex   int
	ContentIndex   int
	Raw            string
}

// ParseImageDimensionError 解析图像维度错误。
func ParseImageDimensionError(raw string) *ImageDimensionError {
	if raw == "" {
		return nil
	}
	if !strings.Contains(strings.ToLower(raw), "image dimensions exceed max allowed size") {
		return nil
	}
	result := &ImageDimensionError{Raw: raw, MessageIndex: -1, ContentIndex: -1}
	if m := imageDimensionErrorRE.FindStringSubmatch(raw); len(m) > 1 {
		result.MaxDimensionPx = parseInt(m[1])
	}
	if m := imageDimensionPathRE.FindStringSubmatch(raw); len(m) > 2 {
		result.MessageIndex = parseInt(m[1])
		result.ContentIndex = parseInt(m[2])
	}
	return result
}

// IsImageDimensionErrorMessage 检查是否图像维度错误。
func IsImageDimensionErrorMessage(raw string) bool {
	return ParseImageDimensionError(raw) != nil
}

// ImageSizeError 图像大小错误解析结果。
type ImageSizeError struct {
	MaxMb float64
	Raw   string
}

// ParseImageSizeError 解析图像大小错误。
func ParseImageSizeError(raw string) *ImageSizeError {
	if raw == "" {
		return nil
	}
	lower := strings.ToLower(raw)
	if !strings.Contains(lower, "image exceeds") || !strings.Contains(lower, "mb") {
		return nil
	}
	result := &ImageSizeError{Raw: raw}
	if m := imageSizeErrorRE.FindStringSubmatch(raw); len(m) > 1 {
		result.MaxMb = parseFloat(m[1])
	}
	return result
}

// IsImageSizeError 检查是否图像大小错误。
func IsImageSizeError(raw string) bool {
	return ParseImageSizeError(raw) != nil
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func parseFloat(s string) float64 {
	// 简单浮点解析
	parts := strings.SplitN(s, ".", 2)
	whole := parseInt(parts[0])
	if len(parts) == 1 {
		return float64(whole)
	}
	frac := parseInt(parts[1])
	div := 1.0
	for i := 0; i < len(parts[1]); i++ {
		div *= 10
	}
	return float64(whole) + float64(frac)/div
}
