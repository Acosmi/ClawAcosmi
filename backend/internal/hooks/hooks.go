// Package hooks 实现 webhook 钩子系统：配置解析、映射匹配、模板渲染。
package hooks

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------- 常量 ----------

const (
	DefaultHooksPath         = "/hooks"
	DefaultHooksMaxBodyBytes = 256 * 1024
)

// ---------- 类型定义 ----------

// HooksConfig 钩子系统配置（对应 config 层）。
type HooksConfig struct {
	Enabled       bool                `json:"enabled"`
	Token         string              `json:"token,omitempty"`
	Path          string              `json:"path,omitempty"`
	MaxBodyBytes  int                 `json:"maxBodyBytes,omitempty"`
	Presets       []string            `json:"presets,omitempty"`
	Mappings      []HookMappingConfig `json:"mappings,omitempty"`
	TransformsDir string              `json:"transformsDir,omitempty"`
	Gmail         *GmailHooksConfig   `json:"gmail,omitempty"`
}

// GmailHooksConfig Gmail 钩子配置。
type GmailHooksConfig struct {
	AllowUnsafeExternalContent *bool `json:"allowUnsafeExternalContent,omitempty"`
}

// HookMappingConfig 单条映射配置。
type HookMappingConfig struct {
	ID                         string            `json:"id,omitempty"`
	Match                      *HookMatchConfig  `json:"match,omitempty"`
	Action                     string            `json:"action,omitempty"` // "wake" | "agent"
	WakeMode                   string            `json:"wakeMode,omitempty"`
	Name                       string            `json:"name,omitempty"`
	SessionKey                 string            `json:"sessionKey,omitempty"`
	MessageTemplate            string            `json:"messageTemplate,omitempty"`
	TextTemplate               string            `json:"textTemplate,omitempty"`
	Deliver                    *bool             `json:"deliver,omitempty"`
	AllowUnsafeExternalContent *bool             `json:"allowUnsafeExternalContent,omitempty"`
	Channel                    string            `json:"channel,omitempty"`
	To                         string            `json:"to,omitempty"`
	Model                      string            `json:"model,omitempty"`
	Thinking                   string            `json:"thinking,omitempty"`
	TimeoutSeconds             *int              `json:"timeoutSeconds,omitempty"`
	Transform                  *HookTransformRef `json:"transform,omitempty"`
}

// HookMatchConfig 匹配条件。
type HookMatchConfig struct {
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
}

// HookTransformRef 变换模块引用。
type HookTransformRef struct {
	Module string `json:"module,omitempty"`
	Export string `json:"export,omitempty"`
}

// HookMappingResolved 解析后的映射规则。
type HookMappingResolved struct {
	ID                         string
	MatchPath                  string
	MatchSource                string
	Action                     string // "wake" | "agent"
	WakeMode                   string // "now" | "next-heartbeat"
	Name                       string
	SessionKey                 string
	MessageTemplate            string
	TextTemplate               string
	Deliver                    *bool
	AllowUnsafeExternalContent *bool
	Channel                    string
	To                         string
	Model                      string
	Thinking                   string
	TimeoutSeconds             *int
}

// HooksConfigResolved 解析后的钩子配置。
type HooksConfigResolved struct {
	BasePath     string
	Token        string
	MaxBodyBytes int
	Mappings     []HookMappingResolved
}

// ---------- 钩子动作类型 ----------

// HookAction 钩子触发的动作。
type HookAction struct {
	Kind                       string `json:"kind"` // "wake" | "agent"
	Text                       string `json:"text,omitempty"`
	Mode                       string `json:"mode,omitempty"`
	Message                    string `json:"message,omitempty"`
	Name                       string `json:"name,omitempty"`
	WakeMode                   string `json:"wakeMode,omitempty"`
	SessionKey                 string `json:"sessionKey,omitempty"`
	Deliver                    *bool  `json:"deliver,omitempty"`
	AllowUnsafeExternalContent *bool  `json:"allowUnsafeExternalContent,omitempty"`
	Channel                    string `json:"channel,omitempty"`
	To                         string `json:"to,omitempty"`
	Model                      string `json:"model,omitempty"`
	Thinking                   string `json:"thinking,omitempty"`
	TimeoutSeconds             *int   `json:"timeoutSeconds,omitempty"`
}

// HookMappingResult 映射匹配结果。
type HookMappingResult struct {
	OK      bool
	Action  *HookAction
	Skipped bool
	Error   string
}

// HookMappingContext 映射上下文。
type HookMappingContext struct {
	Payload map[string]interface{}
	Headers map[string]string
	URL     *url.URL
	Path    string
}

// HookAgentPayload 代理钩子 payload。
type HookAgentPayload struct {
	Message        string `json:"message"`
	Name           string `json:"name"`
	WakeMode       string `json:"wakeMode"`
	SessionKey     string `json:"sessionKey"`
	Deliver        bool   `json:"deliver"`
	Channel        string `json:"channel"`
	To             string `json:"to,omitempty"`
	Model          string `json:"model,omitempty"`
	Thinking       string `json:"thinking,omitempty"`
	TimeoutSeconds *int   `json:"timeoutSeconds,omitempty"`
}

// ---------- Gmail 预设 ----------

var gmailPresetMappings = []HookMappingConfig{
	{
		ID:              "gmail",
		Match:           &HookMatchConfig{Path: "gmail"},
		Action:          "agent",
		WakeMode:        "now",
		Name:            "Gmail",
		SessionKey:      "hook:gmail:{{messages[0].id}}",
		MessageTemplate: "New email from {{messages[0].from}}\nSubject: {{messages[0].subject}}\n{{messages[0].snippet}}\n{{messages[0].body}}",
	},
}

// ---------- 配置解析 ----------

// ResolveHooksConfig 从配置解析钩子系统设置。
func ResolveHooksConfig(cfg *HooksConfig) (*HooksConfigResolved, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, fmt.Errorf("hooks.enabled requires hooks.token")
	}

	rawPath := strings.TrimSpace(cfg.Path)
	if rawPath == "" {
		rawPath = DefaultHooksPath
	}
	if !strings.HasPrefix(rawPath, "/") {
		rawPath = "/" + rawPath
	}
	if len(rawPath) > 1 {
		rawPath = strings.TrimRight(rawPath, "/")
	}
	if rawPath == "/" {
		return nil, fmt.Errorf("hooks.path may not be '/'")
	}

	maxBodyBytes := DefaultHooksMaxBodyBytes
	if cfg.MaxBodyBytes > 0 {
		maxBodyBytes = cfg.MaxBodyBytes
	}

	mappings := resolveHookMappings(cfg)
	return &HooksConfigResolved{
		BasePath:     rawPath,
		Token:        token,
		MaxBodyBytes: maxBodyBytes,
		Mappings:     mappings,
	}, nil
}

func resolveHookMappings(cfg *HooksConfig) []HookMappingResolved {
	var raw []HookMappingConfig
	raw = append(raw, cfg.Mappings...)

	for _, preset := range cfg.Presets {
		if preset == "gmail" {
			presets := make([]HookMappingConfig, len(gmailPresetMappings))
			copy(presets, gmailPresetMappings)
			if cfg.Gmail != nil && cfg.Gmail.AllowUnsafeExternalContent != nil {
				for i := range presets {
					presets[i].AllowUnsafeExternalContent = cfg.Gmail.AllowUnsafeExternalContent
				}
			}
			raw = append(raw, presets...)
		}
	}

	if len(raw) == 0 {
		return nil
	}

	resolved := make([]HookMappingResolved, len(raw))
	for i, m := range raw {
		resolved[i] = normalizeHookMapping(m, i)
	}
	return resolved
}

func normalizeHookMapping(m HookMappingConfig, index int) HookMappingResolved {
	id := strings.TrimSpace(m.ID)
	if id == "" {
		id = fmt.Sprintf("mapping-%d", index+1)
	}
	action := m.Action
	if action == "" {
		action = "agent"
	}
	wakeMode := m.WakeMode
	if wakeMode == "" {
		wakeMode = "now"
	}

	var matchPath, matchSource string
	if m.Match != nil {
		matchPath = normalizeMatchPath(m.Match.Path)
		matchSource = strings.TrimSpace(m.Match.Source)
	}

	return HookMappingResolved{
		ID:                         id,
		MatchPath:                  matchPath,
		MatchSource:                matchSource,
		Action:                     action,
		WakeMode:                   wakeMode,
		Name:                       m.Name,
		SessionKey:                 m.SessionKey,
		MessageTemplate:            m.MessageTemplate,
		TextTemplate:               m.TextTemplate,
		Deliver:                    m.Deliver,
		AllowUnsafeExternalContent: m.AllowUnsafeExternalContent,
		Channel:                    m.Channel,
		To:                         m.To,
		Model:                      m.Model,
		Thinking:                   m.Thinking,
		TimeoutSeconds:             m.TimeoutSeconds,
	}
}

// ---------- 映射匹配与动作构建 ----------

// ApplyHookMappings 对上下文依次检查映射规则，返回第一个匹配的动作。
func ApplyHookMappings(mappings []HookMappingResolved, ctx *HookMappingContext) *HookMappingResult {
	if len(mappings) == 0 {
		return nil
	}
	for _, m := range mappings {
		if !mappingMatches(m, ctx) {
			continue
		}
		return buildActionFromMapping(m, ctx)
	}
	return nil
}

func mappingMatches(m HookMappingResolved, ctx *HookMappingContext) bool {
	if m.MatchPath != "" {
		if m.MatchPath != normalizeMatchPath(ctx.Path) {
			return false
		}
	}
	if m.MatchSource != "" {
		source, _ := ctx.Payload["source"].(string)
		if source != m.MatchSource {
			return false
		}
	}
	return true
}

func buildActionFromMapping(m HookMappingResolved, ctx *HookMappingContext) *HookMappingResult {
	if m.Action == "wake" {
		text := RenderTemplate(m.TextTemplate, ctx)
		if strings.TrimSpace(text) == "" {
			return &HookMappingResult{OK: false, Error: "hook mapping requires text"}
		}
		wm := m.WakeMode
		if wm == "" {
			wm = "now"
		}
		return &HookMappingResult{
			OK:     true,
			Action: &HookAction{Kind: "wake", Text: text, Mode: wm},
		}
	}
	// agent action
	message := RenderTemplate(m.MessageTemplate, ctx)
	if strings.TrimSpace(message) == "" {
		return &HookMappingResult{OK: false, Error: "hook mapping requires message"}
	}
	wm := m.WakeMode
	if wm == "" {
		wm = "now"
	}
	return &HookMappingResult{
		OK: true,
		Action: &HookAction{
			Kind:                       "agent",
			Message:                    message,
			Name:                       renderOptional(m.Name, ctx),
			WakeMode:                   wm,
			SessionKey:                 renderOptional(m.SessionKey, ctx),
			Deliver:                    m.Deliver,
			AllowUnsafeExternalContent: m.AllowUnsafeExternalContent,
			Channel:                    m.Channel,
			To:                         renderOptional(m.To, ctx),
			Model:                      renderOptional(m.Model, ctx),
			Thinking:                   renderOptional(m.Thinking, ctx),
			TimeoutSeconds:             m.TimeoutSeconds,
		},
	}
}

// ---------- Payload 规范化 ----------

// NormalizeWakePayload 规范化 wake payload。
func NormalizeWakePayload(payload map[string]interface{}) (text string, mode string, err error) {
	raw, _ := payload["text"].(string)
	text = strings.TrimSpace(raw)
	if text == "" {
		return "", "", fmt.Errorf("text required")
	}
	m, _ := payload["mode"].(string)
	if m == "next-heartbeat" {
		mode = "next-heartbeat"
	} else {
		mode = "now"
	}
	return text, mode, nil
}

// NormalizeAgentPayload 规范化 agent hook payload。
func NormalizeAgentPayload(payload map[string]interface{}) (*HookAgentPayload, error) {
	raw, _ := payload["message"].(string)
	message := strings.TrimSpace(raw)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	nameRaw, _ := payload["name"].(string)
	name := strings.TrimSpace(nameRaw)
	if name == "" {
		name = "Hook"
	}

	wakeMode := "now"
	if wm, _ := payload["wakeMode"].(string); wm == "next-heartbeat" {
		wakeMode = "next-heartbeat"
	}

	sessionKeyRaw, _ := payload["sessionKey"].(string)
	sessionKey := strings.TrimSpace(sessionKeyRaw)
	if sessionKey == "" {
		sessionKey = "hook:" + uuid.New().String()
	}

	channelRaw, ok := payload["channel"]
	channel := "last"
	if ok {
		ch, _ := channelRaw.(string)
		ch = strings.TrimSpace(strings.ToLower(ch))
		if ch == "" {
			return nil, fmt.Errorf("invalid channel")
		}
		channel = ch
	}

	deliver := true
	if d, ok := payload["deliver"].(bool); ok {
		deliver = d
	}

	toRaw, _ := payload["to"].(string)
	to := strings.TrimSpace(toRaw)

	modelRaw, hasModel := payload["model"]
	model := ""
	if hasModel {
		m, _ := modelRaw.(string)
		model = strings.TrimSpace(m)
		if model == "" {
			return nil, fmt.Errorf("model required")
		}
	}

	thinkingRaw, _ := payload["thinking"].(string)
	thinking := strings.TrimSpace(thinkingRaw)

	var timeoutSeconds *int
	if ts, ok := payload["timeoutSeconds"].(float64); ok && !math.IsInf(ts, 0) && !math.IsNaN(ts) && ts > 0 {
		v := int(ts)
		timeoutSeconds = &v
	}

	return &HookAgentPayload{
		Message:        message,
		Name:           name,
		WakeMode:       wakeMode,
		SessionKey:     sessionKey,
		Deliver:        deliver,
		Channel:        channel,
		To:             to,
		Model:          model,
		Thinking:       thinking,
		TimeoutSeconds: timeoutSeconds,
	}, nil
}

// ---------- 模板渲染 ----------

var templateRegex = regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)

// RenderTemplate 渲染 `{{expr}}` 模板。
func RenderTemplate(tmpl string, ctx *HookMappingContext) string {
	if tmpl == "" {
		return ""
	}
	return templateRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		sub := templateRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		expr := strings.TrimSpace(sub[1])
		val := resolveTemplateExpr(expr, ctx)
		return formatTemplateValue(val)
	})
}

func resolveTemplateExpr(expr string, ctx *HookMappingContext) interface{} {
	if expr == "path" {
		return ctx.Path
	}
	if expr == "now" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	if strings.HasPrefix(expr, "headers.") {
		key := expr[len("headers."):]
		return ctx.Headers[key]
	}
	if strings.HasPrefix(expr, "query.") && ctx.URL != nil {
		key := expr[len("query."):]
		return ctx.URL.Query().Get(key)
	}
	if strings.HasPrefix(expr, "payload.") {
		return getByPath(ctx.Payload, expr[len("payload."):])
	}
	return getByPath(ctx.Payload, expr)
}

func formatTemplateValue(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

// ---------- 路径遍历 ----------

var pathPartRegex = regexp.MustCompile(`([^.\[\]]+)|\[(\d+)\]`)

// getByPath 按点号路径 + 数组索引访问嵌套数据。
// 支持: "a.b.c", "messages[0].id", "a[0][1].name"
func getByPath(data map[string]interface{}, pathExpr string) interface{} {
	if pathExpr == "" || data == nil {
		return nil
	}
	matches := pathPartRegex.FindAllStringSubmatch(pathExpr, -1)
	if len(matches) == 0 {
		return nil
	}

	var current interface{} = data
	for _, m := range matches {
		if current == nil {
			return nil
		}
		if m[2] != "" {
			// 数组索引 [N]
			arr, ok := current.([]interface{})
			if !ok {
				return nil
			}
			idx, _ := strconv.Atoi(m[2])
			if idx < 0 || idx >= len(arr) {
				return nil
			}
			current = arr[idx]
		} else {
			// 对象键
			obj, ok := current.(map[string]interface{})
			if !ok {
				return nil
			}
			current = obj[m[1]]
		}
	}
	return current
}

// ---------- 工具函数 ----------

func normalizeMatchPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimLeft(trimmed, "/")
	trimmed = strings.TrimRight(trimmed, "/")
	return trimmed
}

func renderOptional(value string, ctx *HookMappingContext) string {
	if value == "" {
		return ""
	}
	rendered := strings.TrimSpace(RenderTemplate(value, ctx))
	return rendered
}
