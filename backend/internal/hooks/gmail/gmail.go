package gmail

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// --- 常量 ---
// 对应 TS: gmail.ts 常量

const (
	DefaultGmailLabel        = "INBOX"
	DefaultGmailTopic        = "gog-gmail-watch"
	DefaultGmailSubscription = "gog-gmail-watch-push"
	DefaultGmailServeBind    = "127.0.0.1"
	DefaultGmailServePort    = 8788
	DefaultGmailServePath    = "/gmail-pubsub"
	DefaultGmailMaxBytes     = 20_000
	DefaultGmailRenewMinutes = 12 * 60
	DefaultHooksPath         = "/hooks"
	DefaultGatewayPort       = 3000
)

// --- 配置类型 ---

// GmailHookOverrides Gmail Hook 配置覆盖
// 对应 TS: gmail.ts GmailHookOverrides
type GmailHookOverrides struct {
	Account           string
	Label             string
	Topic             string
	Subscription      string
	PushToken         string
	HookToken         string
	HookURL           string
	IncludeBody       *bool
	MaxBytes          *int
	RenewEveryMinutes *int
	ServeBind         string
	ServePort         *int
	ServePath         string
	TailscaleMode     string // "off"|"serve"|"funnel"
	TailscalePath     string
	TailscaleTarget   string
}

// GmailHookRuntimeConfig 完全解析后的运行时配置
// 对应 TS: gmail.ts GmailHookRuntimeConfig
type GmailHookRuntimeConfig struct {
	Account           string
	Label             string
	Topic             string
	Subscription      string
	PushToken         string
	HookToken         string
	HookURL           string
	IncludeBody       bool
	MaxBytes          int
	RenewEveryMinutes int
	Serve             GmailServeConfig
	Tailscale         GmailTailscaleConfig
}

// GmailServeConfig 服务配置
type GmailServeConfig struct {
	Bind string
	Port int
	Path string
}

// GmailTailscaleConfig Tailscale 配置
type GmailTailscaleConfig struct {
	Mode   string // "off"|"serve"|"funnel"
	Path   string
	Target string
}

// --- 核心函数 ---

// GenerateHookToken 生成随机 hook token
// 对应 TS: gmail.ts generateHookToken
func GenerateHookToken(byteLen int) string {
	if byteLen <= 0 {
		byteLen = 24
	}
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// MergeHookPresets 合并 hook presets
// 对应 TS: gmail.ts mergeHookPresets
func MergeHookPresets(existing []string, preset string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(existing)+1)
	for _, item := range existing {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	if !seen[preset] {
		result = append(result, preset)
	}
	return result
}

// NormalizeHooksPath 规范化 hooks 路径
// 对应 TS: gmail.ts normalizeHooksPath
func NormalizeHooksPath(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		base = DefaultHooksPath
	}
	if base == "/" {
		return DefaultHooksPath
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return strings.TrimRight(base, "/")
}

// NormalizeServePath 规范化 serve 路径
// 对应 TS: gmail.ts normalizeServePath
func NormalizeServePath(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		base = DefaultGmailServePath
	}
	if base == "/" {
		return "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return strings.TrimRight(base, "/")
}

// BuildDefaultHookURL 构建默认 hook URL
// 对应 TS: gmail.ts buildDefaultHookUrl
func BuildDefaultHookURL(hooksPath string, port int) string {
	if port <= 0 {
		port = DefaultGatewayPort
	}
	basePath := NormalizeHooksPath(hooksPath)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	return joinURL(baseURL, basePath+"/gmail")
}

// ResolveGmailHookRuntimeConfig 解析 Gmail Hook 运行时配置
// 对应 TS: gmail.ts resolveGmailHookRuntimeConfig
func ResolveGmailHookRuntimeConfig(
	cfg *types.OpenAcosmiConfig,
	overrides GmailHookOverrides,
) (*GmailHookRuntimeConfig, error) {
	var hooks *types.HooksConfig
	if cfg != nil {
		hooks = cfg.Hooks
	}
	var gmail *types.HooksGmailConfig
	if hooks != nil {
		gmail = hooks.Gmail
	}

	// hookToken
	hookToken := firstNonEmpty(overrides.HookToken, hooksToken(hooks))
	if hookToken == "" {
		return nil, fmt.Errorf("hooks.token missing (needed for gmail hook)")
	}

	// account
	account := firstNonEmpty(overrides.Account, gmailAccount(gmail))
	if account == "" {
		return nil, fmt.Errorf("gmail account required")
	}

	// topic
	topic := firstNonEmpty(overrides.Topic, gmailTopic(gmail))
	if topic == "" {
		return nil, fmt.Errorf("gmail topic required")
	}

	subscription := firstNonEmpty(overrides.Subscription, gmailSubscription(gmail), DefaultGmailSubscription)
	pushToken := firstNonEmpty(overrides.PushToken, gmailPushToken(gmail))
	if pushToken == "" {
		return nil, fmt.Errorf("gmail push token required")
	}

	// hookURL
	hookURL := firstNonEmpty(overrides.HookURL, gmailHookURL(gmail))
	if hookURL == "" {
		hookURL = BuildDefaultHookURL(hooksPath(hooks), resolveGatewayPort(cfg))
	}

	// includeBody
	includeBody := true
	if overrides.IncludeBody != nil {
		includeBody = *overrides.IncludeBody
	} else if gmail != nil && gmail.IncludeBody != nil {
		includeBody = *gmail.IncludeBody
	}

	// maxBytes
	maxBytes := DefaultGmailMaxBytes
	if overrides.MaxBytes != nil && *overrides.MaxBytes > 0 {
		maxBytes = *overrides.MaxBytes
	} else if gmail != nil && gmail.MaxBytes != nil && *gmail.MaxBytes > 0 {
		maxBytes = *gmail.MaxBytes
	}

	// renewEveryMinutes
	renewEveryMinutes := DefaultGmailRenewMinutes
	if overrides.RenewEveryMinutes != nil && *overrides.RenewEveryMinutes > 0 {
		renewEveryMinutes = *overrides.RenewEveryMinutes
	} else if gmail != nil && gmail.RenewEveryMinutes != nil && *gmail.RenewEveryMinutes > 0 {
		renewEveryMinutes = *gmail.RenewEveryMinutes
	}

	// serve
	serveBind := firstNonEmpty(overrides.ServeBind, gmailServeBind(gmail), DefaultGmailServeBind)
	servePort := DefaultGmailServePort
	if overrides.ServePort != nil && *overrides.ServePort > 0 {
		servePort = *overrides.ServePort
	} else if gmail != nil && gmail.Serve != nil && gmail.Serve.Port != nil && *gmail.Serve.Port > 0 {
		servePort = *gmail.Serve.Port
	}

	servePathRaw := firstNonEmpty(overrides.ServePath, gmailServePathCfg(gmail))
	normalizedServePath := DefaultGmailServePath
	if servePathRaw != "" {
		normalizedServePath = NormalizeServePath(servePathRaw)
	}

	// tailscale
	tailscaleMode := firstNonEmpty(overrides.TailscaleMode, gmailTailscaleMode(gmail), "off")
	tailscaleTarget := ""
	tailscaleTargetRaw := firstNonEmpty(overrides.TailscaleTarget, gmailTailscaleTarget(gmail))
	if tailscaleMode != "off" && tailscaleTargetRaw != "" {
		tailscaleTarget = strings.TrimSpace(tailscaleTargetRaw)
	}

	servePath := NormalizeServePath(normalizedServePath)
	if tailscaleMode != "off" && tailscaleTarget == "" {
		servePath = NormalizeServePath("/")
	}

	tailscalePathRaw := firstNonEmpty(overrides.TailscalePath, gmailTailscalePath(gmail))
	var tailscalePath string
	if tailscaleMode != "off" {
		tailscalePath = NormalizeServePath(firstNonEmpty(tailscalePathRaw, normalizedServePath))
	} else {
		tailscalePath = NormalizeServePath(firstNonEmpty(tailscalePathRaw, servePath))
	}

	return &GmailHookRuntimeConfig{
		Account:           account,
		Label:             firstNonEmpty(overrides.Label, gmailLabel(gmail), DefaultGmailLabel),
		Topic:             topic,
		Subscription:      subscription,
		PushToken:         pushToken,
		HookToken:         hookToken,
		HookURL:           hookURL,
		IncludeBody:       includeBody,
		MaxBytes:          maxBytes,
		RenewEveryMinutes: renewEveryMinutes,
		Serve: GmailServeConfig{
			Bind: serveBind,
			Port: servePort,
			Path: servePath,
		},
		Tailscale: GmailTailscaleConfig{
			Mode:   tailscaleMode,
			Path:   tailscalePath,
			Target: tailscaleTarget,
		},
	}, nil
}

// BuildGogWatchStartArgs 构建 gog gmail watch start 参数
// 对应 TS: gmail.ts buildGogWatchStartArgs
func BuildGogWatchStartArgs(account, label, topic string) []string {
	return []string{
		"gmail", "watch", "start",
		"--account", account,
		"--label", label,
		"--topic", topic,
	}
}

// BuildGogWatchServeArgs 构建 gog gmail watch serve 参数
// 对应 TS: gmail.ts buildGogWatchServeArgs
func BuildGogWatchServeArgs(cfg *GmailHookRuntimeConfig) []string {
	args := []string{
		"gmail", "watch", "serve",
		"--account", cfg.Account,
		"--bind", cfg.Serve.Bind,
		"--port", fmt.Sprintf("%d", cfg.Serve.Port),
		"--path", cfg.Serve.Path,
		"--token", cfg.PushToken,
		"--hook-url", cfg.HookURL,
		"--hook-token", cfg.HookToken,
	}
	if cfg.IncludeBody {
		args = append(args, "--include-body")
	}
	if cfg.MaxBytes > 0 {
		args = append(args, "--max-bytes", fmt.Sprintf("%d", cfg.MaxBytes))
	}
	return args
}

// BuildTopicPath 构建 Pub/Sub topic 路径
func BuildTopicPath(projectID, topicName string) string {
	return fmt.Sprintf("projects/%s/topics/%s", projectID, topicName)
}

var topicPathRe = regexp.MustCompile(`(?i)^projects/([^/]+)/topics/([^/]+)$`)

// ParseTopicPath 解析 Pub/Sub topic 路径
func ParseTopicPath(topic string) (projectID, topicName string, ok bool) {
	m := topicPathRe.FindStringSubmatch(strings.TrimSpace(topic))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// --- 辅助函数 ---

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func joinURL(base, path string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base + path
	}
	basePath := strings.TrimRight(u.Path, "/")
	extra := path
	if !strings.HasPrefix(extra, "/") {
		extra = "/" + extra
	}
	u.Path = basePath + extra
	return u.String()
}

func resolveGatewayPort(cfg *types.OpenAcosmiConfig) int {
	if cfg != nil && cfg.Gateway != nil && cfg.Gateway.Port != nil && *cfg.Gateway.Port > 0 {
		return *cfg.Gateway.Port
	}
	return DefaultGatewayPort
}

// 类型安全的 config 字段访问
func hooksToken(h *types.HooksConfig) string {
	if h == nil {
		return ""
	}
	return h.Token
}
func hooksPath(h *types.HooksConfig) string {
	if h == nil {
		return ""
	}
	return h.Path
}
func gmailAccount(g *types.HooksGmailConfig) string {
	if g == nil {
		return ""
	}
	return g.Account
}
func gmailLabel(g *types.HooksGmailConfig) string {
	if g == nil {
		return ""
	}
	return g.Label
}
func gmailTopic(g *types.HooksGmailConfig) string {
	if g == nil {
		return ""
	}
	return g.Topic
}
func gmailSubscription(g *types.HooksGmailConfig) string {
	if g == nil {
		return ""
	}
	return g.Subscription
}
func gmailPushToken(g *types.HooksGmailConfig) string {
	if g == nil {
		return ""
	}
	return g.PushToken
}
func gmailHookURL(g *types.HooksGmailConfig) string {
	if g == nil {
		return ""
	}
	return g.HookURL
}
func gmailServeBind(g *types.HooksGmailConfig) string {
	if g == nil || g.Serve == nil {
		return ""
	}
	return g.Serve.Bind
}
func gmailServePathCfg(g *types.HooksGmailConfig) string {
	if g == nil || g.Serve == nil {
		return ""
	}
	return g.Serve.Path
}
func gmailTailscaleMode(g *types.HooksGmailConfig) string {
	if g == nil || g.Tailscale == nil {
		return ""
	}
	return string(g.Tailscale.Mode)
}
func gmailTailscalePath(g *types.HooksGmailConfig) string {
	if g == nil || g.Tailscale == nil {
		return ""
	}
	return g.Tailscale.Path
}
func gmailTailscaleTarget(g *types.HooksGmailConfig) string {
	if g == nil || g.Tailscale == nil {
		return ""
	}
	return g.Tailscale.Target
}
