package acp

import (
	"fmt"
)

// ---------- GatewayRequester 接口 ----------

// GatewayRequester 网关请求接口，用于 ACP 与 Gateway 交互。
// 由调用方注入具体实现（隔离对 gateway 包的直接依赖）。
type GatewayRequester interface {
	// Request 向网关发送 RPC 请求。method 为方法名，params 为请求参数，result 为响应载荷指针。
	Request(method string, params interface{}, result interface{}) error
}

// ---------- Session Meta 解析 ----------

// AcpSessionMeta ACP 会话元数据（从 _meta 中提取的 session 选项）。
type AcpSessionMeta struct {
	SessionKey             string
	SessionLabel           string
	RequireExistingSession *bool
	ResetSession           *bool
	PrefixCwd              *bool
}

// ParseSessionMeta 解析 _meta 中的会话选项。
// 对应 TS: acp/session-mapper.ts parseSessionMeta()
// P1-7: 键名 alias 对齐 TS
func ParseSessionMeta(meta map[string]interface{}) AcpSessionMeta {
	return AcpSessionMeta{
		SessionKey:             ReadString(meta, []string{"sessionKey", "session", "key"}),
		SessionLabel:           ReadString(meta, []string{"sessionLabel", "label"}),
		RequireExistingSession: ReadBool(meta, []string{"requireExistingSession", "requireExisting"}),
		ResetSession:           ReadBool(meta, []string{"resetSession", "reset"}),
		PrefixCwd:              ReadBool(meta, []string{"prefixCwd"}),
	}
}

// ---------- Session Key 解析 ----------

// ResolveSessionKeyParams 解析 session key 的参数。
type ResolveSessionKeyParams struct {
	Meta        AcpSessionMeta
	FallbackKey string
	Gateway     GatewayRequester
	Opts        *AcpServerOptions
}

// resolveResult sessions.resolve 响应。
type resolveResult struct {
	OK  bool   `json:"ok"`
	Key string `json:"key"`
}

// ResolveSessionKey 按优先级解析 session key。
// 优先级: sessionLabel > sessionKey > defaultSessionLabel > defaultSessionKey > fallbackKey
// 对应 TS: acp/session-mapper.ts resolveSessionKey()
// P0-6: 补全 requireExistingSession 验证逻辑。
func ResolveSessionKey(params ResolveSessionKeyParams) (string, error) {
	meta := params.Meta
	opts := params.Opts
	if opts == nil {
		opts = &AcpServerOptions{}
	}

	requestedLabel := meta.SessionLabel
	if requestedLabel == "" {
		requestedLabel = opts.DefaultSessionLabel
	}
	requestedKey := meta.SessionKey
	if requestedKey == "" {
		requestedKey = opts.DefaultSessionKey
	}
	requireExisting := false
	if meta.RequireExistingSession != nil {
		requireExisting = *meta.RequireExistingSession
	} else if opts.RequireExistingSession {
		requireExisting = true
	}

	// 1. 如果有 meta.sessionLabel，从网关解析
	if meta.SessionLabel != "" {
		var result resolveResult
		err := params.Gateway.Request("sessions.resolve", map[string]interface{}{
			"label": meta.SessionLabel,
		}, &result)
		if err != nil {
			return "", fmt.Errorf("resolve session label %q: %w", meta.SessionLabel, err)
		}
		if result.Key == "" {
			return "", fmt.Errorf("unable to resolve session label: %s", meta.SessionLabel)
		}
		return result.Key, nil
	}

	// 2. 如果有显式 session key
	if meta.SessionKey != "" {
		if !requireExisting {
			return meta.SessionKey, nil
		}
		// 验证存在性
		var result resolveResult
		err := params.Gateway.Request("sessions.resolve", map[string]interface{}{
			"key": meta.SessionKey,
		}, &result)
		if err != nil || result.Key == "" {
			return "", fmt.Errorf("session key not found: %s", meta.SessionKey)
		}
		return result.Key, nil
	}

	// 3. 如果有 requestedLabel（来自 opts），从网关解析
	if requestedLabel != "" {
		var result resolveResult
		err := params.Gateway.Request("sessions.resolve", map[string]interface{}{
			"label": requestedLabel,
		}, &result)
		if err != nil {
			return "", fmt.Errorf("resolve session label %q: %w", requestedLabel, err)
		}
		if result.Key == "" {
			return "", fmt.Errorf("unable to resolve session label: %s", requestedLabel)
		}
		return result.Key, nil
	}

	// 4. 如果有 requestedKey（来自 opts）
	if requestedKey != "" {
		if !requireExisting {
			return requestedKey, nil
		}
		var result resolveResult
		err := params.Gateway.Request("sessions.resolve", map[string]interface{}{
			"key": requestedKey,
		}, &result)
		if err != nil || result.Key == "" {
			return "", fmt.Errorf("session key not found: %s", requestedKey)
		}
		return result.Key, nil
	}

	// 5. Fallback
	return params.FallbackKey, nil
}

// ---------- Session Reset ----------

// ResetSessionIfNeeded 根据选项条件重置 session。
// 对应 TS: acp/session-mapper.ts resetSessionIfNeeded()
func ResetSessionIfNeeded(gateway GatewayRequester, sessionKey string, meta AcpSessionMeta, opts *AcpServerOptions) error {
	shouldReset := false
	if meta.ResetSession != nil && *meta.ResetSession {
		shouldReset = true
	} else if opts != nil && opts.ResetSession {
		shouldReset = true
	}

	if !shouldReset || sessionKey == "" {
		return nil
	}

	var result map[string]interface{}
	return gateway.Request("sessions.reset", map[string]interface{}{
		"key": sessionKey,
	}, &result)
}
