package gateway

// tools_invoke_http.go — 工具调用 HTTP 端点
// 对应 TS src/gateway/tools-invoke-http.ts (328L)
//
// POST /tools/invoke — 接受 {tool, action, args, sessionKey, dryRun}
// 验证认证 → 查找工具 → 执行调用 → 返回结果。
//
// 当前以 DI 接口 (ToolInvoker) 预留完整实现，
// 框架层返回工具列表元信息以便前端 introspection。

import (
	"log/slog"
	"net/http"
)

// ToolsInvokeBody 工具调用请求体。
type ToolsInvokeBody struct {
	Tool       string                 `json:"tool"`
	Action     string                 `json:"action,omitempty"`
	Args       map[string]interface{} `json:"args"`
	SessionKey string                 `json:"sessionKey,omitempty"`
	DryRun     bool                   `json:"dryRun,omitempty"`
}

// ToolInvokeResult 工具调用结果。
type ToolInvokeResult struct {
	Tool   string      `json:"tool"`
	Action string      `json:"action,omitempty"`
	Output interface{} `json:"output"`
	Error  string      `json:"error,omitempty"`
	DryRun bool        `json:"dryRun,omitempty"`
}

// ToolInvoker 工具调用接口（DI 注入）。
type ToolInvoker func(tool, action string, args map[string]interface{}, sessionKey string) (interface{}, error)

// ToolsInvokeHandlerConfig 工具调用处理器配置。
type ToolsInvokeHandlerConfig struct {
	GetAuth      func() ResolvedGatewayAuth
	Invoker      ToolInvoker
	ToolNames    []string // 已注册工具名称列表
	MaxBodyBytes int64
	Logger       *slog.Logger
}

// HandleToolsInvoke 处理 POST /tools/invoke/。
func HandleToolsInvoke(w http.ResponseWriter, r *http.Request, cfg ToolsInvokeHandlerConfig) {
	if r.Method != http.MethodPost {
		SendMethodNotAllowed(w, "POST")
		return
	}

	// 认证
	auth := cfg.GetAuth()
	token := GetBearerToken(r)
	if token == "" {
		token = GetHeader(r, "X-OpenAcosmi-Token")
	}
	if !authorizeOpenAI(auth, token) {
		SendUnauthorized(w)
		return
	}

	// 读取 body
	maxBytes := cfg.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	body, err := ReadJSONBody(r, maxBytes)
	if err != nil {
		SendInvalidRequest(w, err.Error())
		return
	}

	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		SendInvalidRequest(w, "invalid JSON body")
		return
	}

	toolName, _ := bodyMap["tool"].(string)
	action, _ := bodyMap["action"].(string)
	sessionKey, _ := bodyMap["sessionKey"].(string)
	dryRun := coerceBool(bodyMap["dryRun"])

	if toolName == "" {
		SendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": map[string]string{
				"message": "Missing `tool` field.",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 解析 args
	args := make(map[string]interface{})
	if argsRaw, ok := bodyMap["args"]; ok {
		if argsMap, ok := argsRaw.(map[string]interface{}); ok {
			args = argsMap
		}
	}

	slog.Info("tools.invoke",
		"tool", toolName,
		"action", action,
		"sessionKey", sessionKey,
		"dryRun", dryRun,
		"argsKeys", len(args),
	)

	// Dry run: 返回工具信息
	if dryRun {
		found := false
		for _, name := range cfg.ToolNames {
			if name == toolName {
				found = true
				break
			}
		}
		status := "not_found"
		if found {
			status = "available"
		}
		SendJSON(w, http.StatusOK, map[string]interface{}{
			"tool":           toolName,
			"dryRun":         true,
			"status":         status,
			"availableTools": cfg.ToolNames,
		})
		return
	}

	// 调用工具
	if cfg.Invoker == nil {
		// 无注入 invoker — 返回可用工具列表
		SendJSON(w, http.StatusOK, map[string]interface{}{
			"tool":           toolName,
			"action":         action,
			"output":         nil,
			"error":          "tool invoker not configured - available tools listed",
			"availableTools": cfg.ToolNames,
		})
		return
	}

	output, invokeErr := cfg.Invoker(toolName, action, args, sessionKey)
	if invokeErr != nil {
		slog.Error("tools.invoke: error",
			"tool", toolName,
			"error", invokeErr,
		)
		SendJSON(w, http.StatusOK, ToolInvokeResult{
			Tool:   toolName,
			Action: action,
			Error:  invokeErr.Error(),
		})
		return
	}

	SendJSON(w, http.StatusOK, ToolInvokeResult{
		Tool:   toolName,
		Action: action,
		Output: output,
	})
}
