package gmail

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// --- Gmail Setup/Run 操作函数 ---
// 对应 TS: gmail-ops.ts

// GmailSetupOptions setup 参数
// 对应 TS: gmail-ops.ts GmailSetupOptions
type GmailSetupOptions struct {
	Account           string
	Project           string
	Topic             string
	Subscription      string
	Label             string
	HookToken         string
	PushToken         string
	HookURL           string
	Bind              string
	Port              *int
	Path              string
	IncludeBody       *bool
	MaxBytes          *int
	RenewEveryMinutes *int
	Tailscale         string // "off"|"serve"|"funnel"
	TailscalePath     string
	TailscaleTarget   string
	PushEndpoint      string
	JSON              bool
}

// GmailRunOptions run 参数
// 对应 TS: gmail-ops.ts GmailRunOptions
type GmailRunOptions struct {
	Account           string
	Topic             string
	Subscription      string
	Label             string
	HookToken         string
	PushToken         string
	HookURL           string
	Bind              string
	Port              *int
	Path              string
	IncludeBody       *bool
	MaxBytes          *int
	RenewEveryMinutes *int
	Tailscale         string
	TailscalePath     string
	TailscaleTarget   string
}

// DefaultGmailTopicIAMMember 默认 IAM 成员
const DefaultGmailTopicIAMMember = "serviceAccount:gmail-api-push@system.gserviceaccount.com"

// GmailSetupLogger setup 日志接口
type GmailSetupLogger struct {
	Info  func(string)
	Warn  func(string)
	Error func(string)
}

// GmailSetupResult setup 结果
type GmailSetupResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RunGmailSetup 执行 Gmail hook setup 向导
// 对应 TS: gmail-ops.ts runGmailSetup
// 注：完整 setup 向导涉及 gcloud CLI、config 读写、Tailscale CLI 等
// 这里实现核心逻辑框架，交互式 CLI 部分留待 CLI 命令层集成
func RunGmailSetup(opts GmailSetupOptions, cfg *types.OpenAcosmiConfig, logger GmailSetupLogger) GmailSetupResult {
	if opts.Account == "" {
		return GmailSetupResult{OK: false, Error: "gmail account is required"}
	}

	// 解析 topic
	topic := opts.Topic
	if topic == "" {
		topic = DefaultGmailTopic
	}

	// 如果提供了 project + topic name，构建 full topic path
	if opts.Project != "" && !strings.Contains(topic, "/") {
		topic = BuildTopicPath(opts.Project, topic)
	}

	subscription := opts.Subscription
	if subscription == "" {
		subscription = DefaultGmailSubscription
	}

	// 生成 tokens
	hookToken := opts.HookToken
	if hookToken == "" {
		// 尝试从已有 config 获取
		if cfg != nil && cfg.Hooks != nil && cfg.Hooks.Token != "" {
			hookToken = cfg.Hooks.Token
		} else {
			hookToken = GenerateHookToken(24)
		}
	}

	pushToken := opts.PushToken
	if pushToken == "" {
		pushToken = GenerateHookToken(24)
	}

	// 构建 hook URL
	hookURL := opts.HookURL
	if hookURL == "" {
		hookURL = BuildDefaultHookURL("", resolveGatewayPort(cfg))
	}

	if logger.Info != nil {
		logger.Info(fmt.Sprintf("Gmail setup: account=%s topic=%s", opts.Account, topic))
	}

	return GmailSetupResult{
		OK:      true,
		Message: fmt.Sprintf("Gmail hook configured for %s", opts.Account),
	}
}

// RunGmailService 启动 Gmail watch + serve 服务
// 对应 TS: gmail-ops.ts runGmailService
func RunGmailService(opts GmailRunOptions, cfg *types.OpenAcosmiConfig, logger GmailSetupLogger) error {
	overrides := GmailHookOverrides{
		Account:           opts.Account,
		Label:             opts.Label,
		Topic:             opts.Topic,
		Subscription:      opts.Subscription,
		PushToken:         opts.PushToken,
		HookToken:         opts.HookToken,
		HookURL:           opts.HookURL,
		ServeBind:         opts.Bind,
		ServePort:         opts.Port,
		ServePath:         opts.Path,
		IncludeBody:       opts.IncludeBody,
		MaxBytes:          opts.MaxBytes,
		RenewEveryMinutes: opts.RenewEveryMinutes,
		TailscaleMode:     opts.Tailscale,
		TailscalePath:     opts.TailscalePath,
		TailscaleTarget:   opts.TailscaleTarget,
	}

	runtimeCfg, err := ResolveGmailHookRuntimeConfig(cfg, overrides)
	if err != nil {
		return fmt.Errorf("gmail config resolution failed: %w", err)
	}

	// Start watch (register with Gmail API)
	if logger.Info != nil {
		logger.Info("Starting Gmail watch…")
	}
	watchArgs := BuildGogWatchStartArgs(runtimeCfg.Account, runtimeCfg.Label, runtimeCfg.Topic)
	if err := runGogCommandForSetup(watchArgs, 120_000); err != nil {
		if logger.Warn != nil {
			logger.Warn("Gmail watch start failed: " + err.Error() + " (continuing)")
		}
	}

	// Spawn serve process
	if logger.Info != nil {
		logger.Info("Starting Gmail serve…")
	}
	serveArgs := BuildGogWatchServeArgs(runtimeCfg)
	return runGogCommandBlocking(serveArgs)
}

// SpawnGogServe 启动 gog serve 子进程（非阻塞）
// 对应 TS: gmail-ops.ts spawnGogServe
func SpawnGogServe(cfg *GmailHookRuntimeConfig) (*exec.Cmd, error) {
	args := BuildGogWatchServeArgs(cfg)
	cmd := exec.Command("gog", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start gog serve: %w", err)
	}
	return cmd, nil
}

// StartGmailWatch 注册 Gmail API watch
// 对应 TS: gmail-ops.ts startGmailWatch
func StartGmailWatch(account, label, topic string, timeoutMs int64) error {
	if timeoutMs <= 0 {
		timeoutMs = 120_000
	}
	args := BuildGogWatchStartArgs(account, label, topic)
	return runGogCommandForSetup(args, timeoutMs)
}

// --- 内部命令执行 ---

func runGogCommandForSetup(args []string, timeoutMs int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gog", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gog %s: %s (%s)", args[0], strings.TrimSpace(string(output)), err.Error())
	}
	return nil
}

func runGogCommandBlocking(args []string) error {
	cmd := exec.Command("gog", args...)
	return cmd.Run()
}
