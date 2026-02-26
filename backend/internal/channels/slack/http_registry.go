package slack

import (
	"net/http"
	"strings"
	"sync"
)

// Slack HTTP 路由注册 — 继承自 src/slack/http/registry.ts (49L)

// SlackHttpRequestHandler HTTP 请求处理函数
type SlackHttpRequestHandler func(w http.ResponseWriter, r *http.Request)

// slackHttpRoutes 全局 Slack HTTP 路由表（模块级单例，与 TS 端一致）
var (
	slackHttpRoutes   = make(map[string]SlackHttpRequestHandler)
	slackHttpRoutesMu sync.RWMutex
)

// NormalizeSlackWebhookPath 规范化 webhook 路径。
func NormalizeSlackWebhookPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/slack/events"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

// RegisterSlackHttpHandler 注册 Slack HTTP 处理器，返回注销函数。
func RegisterSlackHttpHandler(path string, handler SlackHttpRequestHandler, log func(string), accountID string) func() {
	normalizedPath := NormalizeSlackWebhookPath(path)

	slackHttpRoutesMu.Lock()
	defer slackHttpRoutesMu.Unlock()

	if _, exists := slackHttpRoutes[normalizedPath]; exists {
		suffix := ""
		if accountID != "" {
			suffix = ` for account "` + accountID + `"`
		}
		if log != nil {
			log("slack: webhook path " + normalizedPath + " already registered" + suffix)
		}
		return func() {}
	}

	slackHttpRoutes[normalizedPath] = handler
	return func() {
		slackHttpRoutesMu.Lock()
		defer slackHttpRoutesMu.Unlock()
		delete(slackHttpRoutes, normalizedPath)
	}
}

// HandleSlackHttpRequest 分发 Slack HTTP 请求。
func HandleSlackHttpRequest(w http.ResponseWriter, r *http.Request) bool {
	slackHttpRoutesMu.RLock()
	handler, ok := slackHttpRoutes[r.URL.Path]
	slackHttpRoutesMu.RUnlock()

	if !ok {
		return false
	}
	handler(w, r)
	return true
}
