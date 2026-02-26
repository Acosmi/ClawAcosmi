package gateway

import (
	"net/http"
	"regexp"
	"strings"
)

// ---------- 新增 HTTP 响应辅助 ----------
// 注意: SendJSON / SendText / SendUnauthorized / SendInvalidRequest /
// SendMethodNotAllowed / SetSSEHeaders / WriteSSEDone / GetHeader /
// GetBearerToken / ReadJSONBody 已定义在 net.go 中。

// SendNotFound 写入 404 标准 JSON 响应。
func SendNotFound(w http.ResponseWriter) {
	SendJSON(w, http.StatusNotFound, map[string]interface{}{
		"error": map[string]string{
			"message": "Not found",
			"type":    "not_found",
		},
	})
}

// WriteSSEData 写入一条 SSE data 行并 flush。
func WriteSSEData(w http.ResponseWriter, data string) {
	w.Write([]byte("data: " + data + "\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// ---------- Agent ID 解析 ----------

// agentFromModelRe 匹配 "openacosmi:agentId" 或 "openacosmi/agentId" 或 "openclaw:agentId" 或 "agent:agentId"
var agentFromModelRe = regexp.MustCompile(`(?i)^(?:(?:openacosmi|openclaw)[:/]|agent:)([a-z0-9][a-z0-9_-]{0,63})$`)

// ResolveAgentIDFromHeader 从请求头解析 Agent ID。
func ResolveAgentIDFromHeader(r *http.Request) string {
	raw := strings.TrimSpace(GetHeader(r, "X-OpenAcosmi-Agent-Id"))
	if raw == "" {
		raw = strings.TrimSpace(GetHeader(r, "X-OpenAcosmi-Agent"))
	}
	if raw == "" {
		return ""
	}
	return normalizeAgentID(raw)
}

// ResolveAgentIDFromModel 从模型名解析 Agent ID。
// 支持 "openacosmi:agentId"、"openacosmi/agentId"、"agent:agentId"。
func ResolveAgentIDFromModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	match := agentFromModelRe.FindStringSubmatch(model)
	if match == nil {
		return ""
	}
	return normalizeAgentID(match[1])
}

// ResolveAgentIDForRequest 综合 header 和模型名解析 Agent ID。
func ResolveAgentIDForRequest(r *http.Request, model string) string {
	if id := ResolveAgentIDFromHeader(r); id != "" {
		return id
	}
	if id := ResolveAgentIDFromModel(model); id != "" {
		return id
	}
	return "main"
}

// normalizeAgentID 标准化 Agent ID (小写)。
func normalizeAgentID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

// ---------- Session Key 辅助 (U-1) ----------

// ResolveSessionKey 从 agent payload 派生 session key。
// 优先使用显式 sessionKey，否则用 channel 或 "default"。
func ResolveSessionKey(sessionKey, channel string) string {
	if sessionKey != "" {
		return sessionKey
	}
	if channel != "" {
		return "channel:" + channel
	}
	return "default"
}
