package slack

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// Slack OAuth 权限范围查询 — 继承自 src/slack/scopes.ts (119L)

// SlackScopesResult 权限范围查询结果
type SlackScopesResult struct {
	OK     bool     `json:"ok"`
	Scopes []string `json:"scopes,omitempty"`
	Source string   `json:"source,omitempty"`
	Error  string   `json:"error,omitempty"`
}

var scopeSplitRe = regexp.MustCompile(`[,\s]+`)

// collectScopes 从嵌套数据结构中收集 scope 字符串。
func collectScopes(value interface{}, into *[]string) {
	if value == nil {
		return
	}
	switch v := value.(type) {
	case []interface{}:
		for _, entry := range v {
			if s, ok := entry.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					*into = append(*into, trimmed)
				}
			}
		}
	case string:
		raw := strings.TrimSpace(v)
		if raw == "" {
			return
		}
		parts := scopeSplitRe.Split(raw, -1)
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				*into = append(*into, part)
			}
		}
	case map[string]interface{}:
		for _, entry := range v {
			switch e := entry.(type) {
			case []interface{}, string:
				collectScopes(e, into)
			}
		}
	}
}

// normalizeScopes 去重并排序 scope 列表。
func normalizeScopes(scopes []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range scopes {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	sort.Strings(result)
	return result
}

// extractScopes 从 Slack API 响应中提取 scope。
func extractScopes(payload map[string]interface{}) []string {
	if payload == nil {
		return nil
	}
	var scopes []string
	collectScopes(payload["scopes"], &scopes)
	collectScopes(payload["scope"], &scopes)

	if info, ok := payload["info"].(map[string]interface{}); ok {
		collectScopes(info["scopes"], &scopes)
		collectScopes(info["scope"], &scopes)
		collectScopes(info["user_scopes"], &scopes)
		collectScopes(info["bot_scopes"], &scopes)
	}

	return normalizeScopes(scopes)
}

// FetchSlackScopes 获取 Slack 应用的 OAuth 权限范围。
// 尝试 auth.scopes → apps.permissions.info 两个端点。
func FetchSlackScopes(ctx context.Context, token string) SlackScopesResult {
	client := NewSlackWebClient(token)
	attempts := []string{"auth.scopes", "apps.permissions.info"}
	var errors []string

	for _, method := range attempts {
		raw, err := client.APICall(ctx, method, nil)
		if err != nil {
			errors = append(errors, method+": "+err.Error())
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err != nil {
			errors = append(errors, method+": unmarshal: "+err.Error())
			continue
		}

		scopes := extractScopes(payload)
		if len(scopes) > 0 {
			return SlackScopesResult{OK: true, Scopes: scopes, Source: method}
		}

		if errStr, ok := payload["error"].(string); ok && strings.TrimSpace(errStr) != "" {
			errors = append(errors, method+": "+strings.TrimSpace(errStr))
		}
	}

	errMsg := "no scopes returned"
	if len(errors) > 0 {
		errMsg = strings.Join(errors, " | ")
	}
	return SlackScopesResult{OK: false, Error: errMsg}
}
