package models

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anthropic/open-acosmi/internal/agents/helpers"
)

// ---------- 失败切换错误 ----------

// TS 参考: src/agents/failover-error.ts (235 行) + pi-embedded-helpers (classifyFailoverReason)

// FailoverReason 失败切换原因。
type FailoverReason string

const (
	FailoverBilling   FailoverReason = "billing"
	FailoverRateLimit FailoverReason = "rate_limit"
	FailoverAuth      FailoverReason = "auth"
	FailoverTimeout   FailoverReason = "timeout"
	FailoverFormat    FailoverReason = "format"
	FailoverOverload  FailoverReason = "overload"
	FailoverServer    FailoverReason = "server"
)

// FailoverError 失败切换错误。
type FailoverError struct {
	Message   string         `json:"message"`
	Reason    FailoverReason `json:"reason"`
	Provider  string         `json:"provider,omitempty"`
	Model     string         `json:"model,omitempty"`
	ProfileID string         `json:"profileId,omitempty"`
	Status    int            `json:"status,omitempty"`
	Code      string         `json:"code,omitempty"`
	Cause     error          `json:"-"`
}

func (e *FailoverError) Error() string {
	return fmt.Sprintf("FailoverError[%s]: %s", e.Reason, e.Message)
}

func (e *FailoverError) Unwrap() error {
	return e.Cause
}

var (
	timeoutHintRE  = regexp.MustCompile(`(?i)timeout|timed out|deadline exceeded|context deadline exceeded`)
	abortTimeoutRE = regexp.MustCompile(`(?i)request was aborted|request aborted`)
)

// IsFailoverError 类型断言。
func IsFailoverError(err error) (*FailoverError, bool) {
	if err == nil {
		return nil, false
	}
	fe, ok := err.(*FailoverError)
	return fe, ok
}

// ShouldFailover 判断错误是否应触发失败切换。
// 兼容旧的 pattern-matching 方式 + 新的 FailoverError 类型。
func ShouldFailover(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := IsFailoverError(err); ok {
		return true
	}
	reason := ClassifyFailoverReason(err.Error())
	return reason != ""
}

// ResolveFailoverStatus 根据原因推导 HTTP 状态码。
func ResolveFailoverStatus(reason FailoverReason) int {
	switch reason {
	case FailoverBilling:
		return 402
	case FailoverRateLimit:
		return 429
	case FailoverAuth:
		return 401
	case FailoverTimeout:
		return 408
	case FailoverFormat:
		return 400
	case FailoverOverload:
		return 503
	case FailoverServer:
		return 500
	default:
		return 0
	}
}

// HasTimeoutHint 检查错误消息是否包含超时提示。
func HasTimeoutHint(err error) bool {
	if err == nil {
		return false
	}
	return timeoutHintRE.MatchString(err.Error())
}

// IsTimeoutError 检查是否为超时错误。
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if HasTimeoutHint(err) {
		return true
	}
	msg := err.Error()
	if abortTimeoutRE.MatchString(msg) {
		return true
	}
	return false
}

// ClassifyFailoverReason 从错误消息分类失败切换原因。
// 委托给 helpers.ClassifyFailoverReason（使用精确的 Is*ErrorMessage 函数）
// TS 参考: errors.ts L631-657
func ClassifyFailoverReason(message string) FailoverReason {
	return FailoverReason(helpers.ClassifyFailoverReason(message))
}

// ---------- 错误属性提取 ----------
// TS 参考: failover-error.ts L58-91

// StatusCoder 实现此接口的错误可提供 HTTP 状态码。
type StatusCoder interface {
	StatusCode() int
}

// ErrorCoder 实现此接口的错误可提供错误码。
type ErrorCoder interface {
	ErrorCode() string
}

// ErrorNamer 实现此接口的错误可提供错误名。
type ErrorNamer interface {
	ErrorName() string
}

// GetStatusCode 从错误提取 HTTP 状态码。
func GetStatusCode(err error) int {
	if err == nil {
		return 0
	}
	if fe, ok := IsFailoverError(err); ok && fe.Status > 0 {
		return fe.Status
	}
	if sc, ok := err.(StatusCoder); ok {
		return sc.StatusCode()
	}
	return 0
}

// GetErrorCode 从错误提取错误码。
func GetErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if fe, ok := IsFailoverError(err); ok && fe.Code != "" {
		return fe.Code
	}
	if ec, ok := err.(ErrorCoder); ok {
		return ec.ErrorCode()
	}
	return ""
}

// GetErrorName 从错误提取错误名。
func GetErrorName(err error) string {
	if err == nil {
		return ""
	}
	if en, ok := err.(ErrorNamer); ok {
		return en.ErrorName()
	}
	return ""
}

// ResolveFailoverReasonFromError 从错误推导失败切换原因。
// TS 参考: failover-error.ts L145-180
func ResolveFailoverReasonFromError(err error) FailoverReason {
	if err == nil {
		return ""
	}
	if fe, ok := IsFailoverError(err); ok {
		return fe.Reason
	}

	// HTTP 状态码分派
	status := GetStatusCode(err)
	switch status {
	case 402:
		return FailoverBilling
	case 429:
		return FailoverRateLimit
	case 401, 403:
		return FailoverAuth
	case 408:
		return FailoverTimeout
	case 400:
		return FailoverFormat
	}

	// 错误码分派
	code := strings.ToUpper(GetErrorCode(err))
	switch code {
	case "ETIMEDOUT", "ESOCKETTIMEDOUT", "ECONNRESET", "ECONNABORTED":
		return FailoverTimeout
	}

	// 超时检测
	if IsTimeoutError(err) {
		return FailoverTimeout
	}

	// 消息模式匹配
	return ClassifyFailoverReason(err.Error())
}

// CoerceToFailoverError 将普通错误转换为 FailoverError。
func CoerceToFailoverError(err error, provider, model, profileId string) *FailoverError {
	if err == nil {
		return nil
	}
	if fe, ok := IsFailoverError(err); ok {
		return fe
	}
	reason := ResolveFailoverReasonFromError(err)
	if reason == "" {
		return nil
	}
	status := ResolveFailoverStatus(reason)
	return &FailoverError{
		Message:   err.Error(),
		Reason:    reason,
		Provider:  provider,
		Model:     model,
		ProfileID: profileId,
		Status:    status,
		Cause:     err,
	}
}

// DescribeFailoverError 描述任意错误的失败切换信息。
// TS 参考: failover-error.ts L182-203 → { message, reason, status, code }
func DescribeFailoverError(err error) (message string, reason FailoverReason, status int, code string) {
	if err == nil {
		return "", "", 0, ""
	}
	if fe, ok := IsFailoverError(err); ok {
		return fe.Message, fe.Reason, fe.Status, fe.Code
	}
	message = err.Error()
	reason = ResolveFailoverReasonFromError(err)
	if reason != "" {
		status = ResolveFailoverStatus(reason)
	}
	code = GetErrorCode(err)
	return
}
