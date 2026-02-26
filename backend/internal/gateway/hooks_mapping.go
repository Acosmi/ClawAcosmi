package gateway

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------- Hook 映射配置 ----------

// HookMappingConfig 原始映射配置 (从 JSON/YAML 解析)。
type HookMappingConfig struct {
	ID    string                `json:"id,omitempty"`
	Match *HookMatchFieldConfig `json:"match,omitempty"` // 嵌套匹配条件（优先于 MatchPath/MatchSource）
	// 兼容旧配置的扁平字段（match.path/match.source 优先）
	MatchPath   string `json:"matchPath,omitempty"`
	MatchSource string `json:"matchSource,omitempty"`
	Action      string `json:"action"` // "wake" | "agent"
	// --- Wake 参数 ---
	WakeText     string `json:"wakeText,omitempty"`
	WakeMode     string `json:"wakeMode,omitempty"`     // "now" | "next-heartbeat"
	TextTemplate string `json:"textTemplate,omitempty"` // M-2: wake text template
	// --- Agent 参数 ---
	Message                    string `json:"message,omitempty"`
	MessageTemplate            string `json:"messageTemplate,omitempty"`
	Name                       string `json:"name,omitempty"`
	SessionKey                 string `json:"sessionKey,omitempty"`
	Channel                    string `json:"channel,omitempty"`
	To                         string `json:"to,omitempty"`
	Model                      string `json:"model,omitempty"`
	Deliver                    *bool  `json:"deliver,omitempty"`
	Thinking                   string `json:"thinking,omitempty"`
	TimeoutSeconds             int    `json:"timeoutSeconds,omitempty"`
	AllowUnsafeExternalContent *bool  `json:"allowUnsafeExternalContent,omitempty"` // M-3
	// --- Transform ---
	TransformModule string `json:"transformModule,omitempty"`
	TransformExport string `json:"transformExport,omitempty"`
}

// HookMatchFieldConfig 嵌套匹配条件（对齐 TS HookMappingMatch）。
type HookMatchFieldConfig struct {
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
}

// HookMappingResolved 解析后的映射规则。
type HookMappingResolved struct {
	ID                         string
	MatchPath                  string
	MatchSource                string
	Action                     string // "wake" | "agent"
	WakeText                   string
	WakeMode                   string
	TextTemplate               string // M-2: wake text template
	Message                    string
	MessageTemplate            string
	Name                       string
	SessionKey                 string
	Channel                    string
	To                         string
	Model                      string
	Deliver                    *bool
	Thinking                   string
	TimeoutSeconds             int
	AllowUnsafeExternalContent *bool // M-3
	TransformModule            string
	TransformExport            string
}

// HookMappingContext 映射匹配上下文。
type HookMappingContext struct {
	Path    string
	Source  string
	Method  string
	Headers map[string]string
	Body    interface{}
	Query   url.Values // URL query parameters
}

// HookMappingResult 映射匹配结果。
type HookMappingResult struct {
	MappingID string
	Action    string // "wake" | "agent"
	Payload   map[string]interface{}
}

// ---------- 解析映射 ----------

// ResolveHookMappings 从原始配置解析映射规则。
func ResolveHookMappings(raw *HooksRawConfig) []HookMappingResolved {
	var mappings []HookMappingResolved

	// 处理 presets
	for _, preset := range raw.Presets {
		switch strings.ToLower(preset) {
		case "github":
			mappings = append(mappings, HookMappingResolved{
				ID:              "preset-github",
				MatchSource:     "github",
				Action:          "agent",
				MessageTemplate: "GitHub event {{event}}: {{body.action}} on {{body.repository.full_name}}",
				Name:            "GitHub Hook",
			})
		case "gitlab":
			mappings = append(mappings, HookMappingResolved{
				ID:              "preset-gitlab",
				MatchSource:     "gitlab",
				Action:          "agent",
				MessageTemplate: "GitLab event: {{body.object_kind}} on {{body.project.path_with_namespace}}",
				Name:            "GitLab Hook",
			})
		case "slack":
			mappings = append(mappings, HookMappingResolved{
				ID:              "preset-slack",
				MatchSource:     "slack",
				Action:          "agent",
				MessageTemplate: "Slack message from {{body.event.user}}: {{body.event.text}}",
				Name:            "Slack Hook",
			})
		case "gmail":
			// M-R1: 对齐 TS gmail preset (matchPath, sessionKey, messageTemplate)
			allowUnsafe := true
			if raw.Gmail != nil && raw.Gmail.AllowUnsafeExternalContent != nil {
				allowUnsafe = *raw.Gmail.AllowUnsafeExternalContent
			}
			mappings = append(mappings, HookMappingResolved{
				ID:                         "gmail",
				MatchPath:                  "gmail",
				Action:                     "agent",
				WakeMode:                   "now",
				Name:                       "Gmail",
				SessionKey:                 "hook:gmail:{{messages[0].id}}",
				MessageTemplate:            "New email from {{messages[0].from}}\nSubject: {{messages[0].subject}}\n{{messages[0].snippet}}\n{{messages[0].body}}",
				AllowUnsafeExternalContent: &allowUnsafe,
			})
		}
	}

	// 处理自定义映射
	for i, cfg := range raw.Mappings {
		id := cfg.ID
		if id == "" {
			id = fmt.Sprintf("mapping-%d", i)
		}
		m := HookMappingResolved{
			ID: id,
			// P2-D2: 嵌套 match 优先于扁平 matchPath/matchSource
			MatchPath:                  resolveMatchField(cfg.Match, "path", cfg.MatchPath),
			MatchSource:                resolveMatchField(cfg.Match, "source", cfg.MatchSource),
			Action:                     strings.TrimSpace(cfg.Action),
			WakeText:                   strings.TrimSpace(cfg.WakeText),
			WakeMode:                   strings.TrimSpace(cfg.WakeMode),
			TextTemplate:               strings.TrimSpace(cfg.TextTemplate),
			Message:                    strings.TrimSpace(cfg.Message),
			MessageTemplate:            strings.TrimSpace(cfg.MessageTemplate),
			Name:                       strings.TrimSpace(cfg.Name),
			SessionKey:                 strings.TrimSpace(cfg.SessionKey),
			Channel:                    strings.TrimSpace(cfg.Channel),
			To:                         strings.TrimSpace(cfg.To),
			Model:                      strings.TrimSpace(cfg.Model),
			Deliver:                    cfg.Deliver,
			Thinking:                   strings.TrimSpace(cfg.Thinking),
			TimeoutSeconds:             cfg.TimeoutSeconds,
			AllowUnsafeExternalContent: cfg.AllowUnsafeExternalContent,
			TransformModule:            strings.TrimSpace(cfg.TransformModule),
			TransformExport:            strings.TrimSpace(cfg.TransformExport),
		}
		if m.Action == "" {
			m.Action = "agent"
		}
		if m.Name == "" && m.Action == "agent" {
			m.Name = "Hook"
		}
		if m.WakeMode == "" {
			m.WakeMode = "now"
		}
		mappings = append(mappings, m)
	}

	return mappings
}

// ---------- 映射匹配 ----------

// ApplyHookMappings 匹配并应用第一个匹配的映射规则。
func ApplyHookMappings(mappings []HookMappingResolved, ctx *HookMappingContext) (*HookMappingResult, error) {
	for _, m := range mappings {
		if !matchMapping(&m, ctx) {
			continue
		}
		result, err := buildMappingResult(&m, ctx)
		if err != nil {
			return nil, fmt.Errorf("mapping %s: %w", m.ID, err)
		}
		return result, nil
	}
	return nil, nil // 无匹配
}

func matchMapping(m *HookMappingResolved, ctx *HookMappingContext) bool {
	// 路径匹配 (M-11: 标准化路径)
	if m.MatchPath != "" {
		pattern := normalizeMatchPath(m.MatchPath)
		normPath := normalizeMatchPath(ctx.Path)
		if !matchPath(pattern, normPath) {
			return false
		}
	}
	// 来源匹配 (M-12: 直接比较 ctx.Source)
	if m.MatchSource != "" {
		if !matchSource(m.MatchSource, ctx) {
			return false
		}
	}
	return true
}

// normalizeMatchPath 去除前后斜线做标准化 (与 TS 对齐)。
func normalizeMatchPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return p
}

func matchPath(pattern, path string) bool {
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}
	return path == pattern
}

// matchSource M-12 修复: 从 ctx.Source 比较 (payload.source 优先, header fallback)。
func matchSource(source string, ctx *HookMappingContext) bool {
	return strings.EqualFold(ctx.Source, source)
}

func buildMappingResult(m *HookMappingResolved, ctx *HookMappingContext) (*HookMappingResult, error) {
	payload := make(map[string]interface{})

	if m.Action == "wake" {
		text := m.WakeText
		if text == "" && m.TextTemplate != "" {
			text = RenderTemplate(m.TextTemplate, ctx)
		}
		if text == "" && m.MessageTemplate != "" {
			text = RenderTemplate(m.MessageTemplate, ctx)
		}
		if text == "" {
			text = "Webhook received"
		}
		payload["text"] = text
		payload["mode"] = m.WakeMode
	} else {
		// agent
		message := m.Message
		if message == "" && m.MessageTemplate != "" {
			message = RenderTemplate(m.MessageTemplate, ctx)
		}
		if message == "" {
			return nil, fmt.Errorf("message required for agent action")
		}
		payload["message"] = message
		// R3-M7+R3-M8: 对齐 TS renderOptional — 渲染模板, 空→不设置
		if v := renderOptional(m.Name, ctx); v != "" {
			payload["name"] = v
		}
		if v := renderOptional(m.SessionKey, ctx); v != "" {
			payload["sessionKey"] = v
		}
		if m.Channel != "" {
			payload["channel"] = m.Channel
		}
		if v := renderOptional(m.To, ctx); v != "" {
			payload["to"] = v
		}
		if v := renderOptional(m.Model, ctx); v != "" {
			payload["model"] = v
		}
		if m.Deliver != nil {
			payload["deliver"] = *m.Deliver
		}
		if v := renderOptional(m.Thinking, ctx); v != "" {
			payload["thinking"] = v
		}
		if m.TimeoutSeconds > 0 {
			payload["timeoutSeconds"] = float64(m.TimeoutSeconds)
		}
		if m.AllowUnsafeExternalContent != nil {
			payload["allowUnsafeExternalContent"] = *m.AllowUnsafeExternalContent
		}
		payload["wakeMode"] = m.WakeMode
	}

	return &HookMappingResult{
		MappingID: m.ID,
		Action:    m.Action,
		Payload:   payload,
	}, nil
}

// ---------- Transform 管道 (P2-D1) ----------

// HookTransformResult transform 函数返回结果。
type HookTransformResult struct {
	Override map[string]interface{} // 覆盖 payload 字段
	Merge    map[string]interface{} // 合并到 payload (不覆盖已有字段)
	Skip     bool                   // true → 跳过此映射
}

// TransformFunc 用户自定义变换函数签名。
// 对齐 TS: hooks-mapping.ts L315-333 transform module/export 概念。
type TransformFunc func(ctx *HookMappingContext, payload map[string]interface{}) (*HookTransformResult, error)

// transformRegistry 全局变换函数注册表。
var transformRegistry = map[string]TransformFunc{}

// RegisterTransform 注册变换函数（供插件调用）。
func RegisterTransform(name string, fn TransformFunc) {
	transformRegistry[name] = fn
}

// ApplyTransform 应用已注册的 transform 到 payload。
// 对齐 TS: hooks-mapping.ts L315-350 中的 transform 逻辑。
func ApplyTransform(m *HookMappingResolved, ctx *HookMappingContext, payload map[string]interface{}) (map[string]interface{}, bool, error) {
	name := m.TransformModule
	if name == "" {
		return payload, false, nil
	}
	if m.TransformExport != "" {
		name = name + "." + m.TransformExport
	}
	fn, ok := transformRegistry[name]
	if !ok {
		// transform 未注册,跳过 (与 TS import() 失败时的 warn-and-skip 行为对齐)
		return payload, false, nil
	}
	result, err := fn(ctx, payload)
	if err != nil {
		return nil, false, fmt.Errorf("transform %s: %w", name, err)
	}
	if result == nil {
		return payload, false, nil
	}
	if result.Skip {
		return payload, true, nil
	}
	// 应用 Override
	for k, v := range result.Override {
		payload[k] = v
	}
	// 应用 Merge (不覆盖已有字段)
	for k, v := range result.Merge {
		if _, exists := payload[k]; !exists {
			payload[k] = v
		}
	}
	return payload, false, nil
}

// renderOptional 渲染可选模板字段 (对齐 TS renderOptional)。
// 空输入 → "", 渲染后空白 → ""。
func renderOptional(value string, ctx *HookMappingContext) string {
	if value == "" {
		return ""
	}
	rendered := strings.TrimSpace(RenderTemplate(value, ctx))
	return rendered
}

// ---------- 模板渲染 ----------

// templateRe 匹配 {{expr}} 模板变量。
var templateRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// RenderTemplate 渲染 {{expr}} 模板字符串。
// M-6: 支持 payload.xxx (等价 body.xxx)、query.xxx (M-4)、now (M-5)。
func RenderTemplate(template string, ctx *HookMappingContext) string {
	return templateRe.ReplaceAllStringFunc(template, func(match string) string {
		expr := strings.TrimSpace(match[2 : len(match)-2])
		val := resolveTemplateExpr(expr, ctx)
		return templateValueToString(val)
	})
}

// resolveTemplateExpr 解析模板表达式 (对齐 TS resolveTemplateExpr)。
func resolveTemplateExpr(expr string, ctx *HookMappingContext) interface{} {
	switch {
	case expr == "path":
		return ctx.Path
	case expr == "source":
		return ctx.Source
	case expr == "method":
		return ctx.Method
	case expr == "now":
		return time.Now().UTC().Format(time.RFC3339)
	case expr == "event":
		if v := ctx.Headers["x-github-event"]; v != "" {
			return v
		}
		if v := ctx.Headers["x-gitlab-event"]; v != "" {
			return v
		}
		return ""
	case strings.HasPrefix(expr, "headers."):
		key := strings.ToLower(strings.TrimPrefix(expr, "headers."))
		return ctx.Headers[key]
	case strings.HasPrefix(expr, "query."):
		key := strings.TrimPrefix(expr, "query.")
		if ctx.Query != nil {
			return ctx.Query.Get(key)
		}
		return ""
	case strings.HasPrefix(expr, "body."):
		path := strings.TrimPrefix(expr, "body.")
		return GetByPath(ctx.Body, path)
	case strings.HasPrefix(expr, "payload."):
		path := strings.TrimPrefix(expr, "payload.")
		return GetByPath(ctx.Body, path)
	default:
		// 裸变量 fallback 到 body lookup
		return GetByPath(ctx.Body, expr)
	}
}

// templateValueToString 将模板值转为字符串 (对齐 TS renderTemplate)。
// M-R7: nil → "" (不是 "<nil>").
// M-R8: 对象/数组 → JSON.stringify (不是 Go fmt.Sprint).
func templateValueToString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	default:
		// map/slice → JSON
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// ---------- 嵌套路径取值 ----------

// GetByPath 从嵌套对象中按路径取值。
// 支持点号分隔的嵌套路径和数组下标 (如 "items.0.name" 或 "items[0].name")。
func GetByPath(obj interface{}, path string) interface{} {
	if obj == nil || path == "" {
		return nil
	}
	// M-BUG-2: 将 [N] 转为 .N 统一处理
	path = bracketRe.ReplaceAllString(path, ".$1")
	parts := strings.Split(path, ".")
	current := obj
	for _, part := range parts {
		if current == nil || part == "" {
			return nil
		}
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		case []interface{}:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil
			}
			current = v[idx]
		default:
			return nil
		}
	}
	return current
}

// bracketRe 匹配 [N] 数组访问语法。
var bracketRe = regexp.MustCompile(`\[(\d+)\]`)

// resolveMatchField P2-D2: 嵌套 match 字段优先于扁平字段。
func resolveMatchField(match *HookMatchFieldConfig, field string, fallback string) string {
	if match != nil {
		switch field {
		case "path":
			if v := strings.TrimSpace(match.Path); v != "" {
				return v
			}
		case "source":
			if v := strings.TrimSpace(match.Source); v != "" {
				return v
			}
		}
	}
	return strings.TrimSpace(fallback)
}
