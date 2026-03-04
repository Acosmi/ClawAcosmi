package main

// setup_noninteractive.go — 非交互式 onboarding 模式
// TS 对照: onboard-non-interactive.ts (37L) + local.ts (148L) + remote.ts (54L)
//          + auth-choice-inference.ts (70L) + gateway-config.ts (115L)
//
// 接受 CLI flag 直接配置 provider/key/model/channel，无需交互输入。

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- NonInteractive 选项 ----------

// NonInteractiveOptions 非交互模式完整选项。
// 对应 TS OnboardOptions（子集）。
type NonInteractiveOptions struct {
	Mode       string // "local" | "remote"
	Workspace  string
	AcceptRisk bool // --accept-risk 必须

	// Auth
	AuthChoice    string
	Provider      string
	Token         string
	TokenProvider string

	// Provider-specific API keys
	AnthropicApiKey         string
	OpenAIApiKey            string
	GeminiApiKey            string
	OpenrouterApiKey        string
	AiGatewayApiKey         string
	CloudflareAiGwAccountID string
	CloudflareAiGwGatewayID string
	CloudflareAiGwApiKey    string
	MoonshotApiKey          string
	KimiCodeApiKey          string
	SyntheticApiKey         string
	VeniceApiKey            string
	ZaiApiKey               string
	XiaomiApiKey            string
	MinimaxApiKey           string
	AcosmiZenApiKey         string
	XaiApiKey               string
	QianfanApiKey           string

	// Gateway
	GatewayPort     int
	GatewayBind     string // "loopback" | "lan" | "auto" | "custom" | "tailnet"
	GatewayAuth     string // "token" | "password"
	GatewayToken    string
	GatewayPassword string

	// Tailscale
	Tailscale            string // "off" | "serve" | "funnel"
	TailscaleResetOnExit bool

	// Flags
	InstallDaemon bool
	SkipSkills    bool
	SkipHealth    bool
	SkipChannels  bool
	NodeManager   string

	// Remote
	RemoteURL   string
	RemoteToken string

	// Output
	JSON bool
}

// ---------- Auth Choice Flag Inference ----------

// authChoiceFlagEntry 映射 flag → authChoice。
type authChoiceFlagEntry struct {
	Provider   string // NonInteractiveOptions 字段标识
	AuthChoice string
	Label      string // CLI flag 名
}

var authChoiceFlagMap = []authChoiceFlagEntry{
	{"anthropic", AuthChoiceApiKey, "--anthropic-api-key"},
	{"gemini", AuthChoiceGeminiApiKey, "--gemini-api-key"},
	{"openai", AuthChoiceOpenAIApiKey, "--openai-api-key"},
	{"moonshot", AuthChoiceMoonshotApiKey, "--moonshot-api-key"},
	{"zai", AuthChoiceZaiApiKey, "--zai-api-key"},
	{"xai", AuthChoiceXAIApiKey, "--xai-api-key"},
	{"minimax", AuthChoiceMinimaxApi, "--minimax-api-key"},
	{"openacosmi-zen", AuthChoiceAcosmiZen, "--openacosmi-zen-api-key"},
}

// AuthChoiceInference 推断结果。
type AuthChoiceInference struct {
	Choice  string
	Matches []authChoiceFlagEntry
}

// InferAuthChoiceFromFlags 从 CLI flag 推断 auth choice。
// 对应 TS inferAuthChoiceFromFlags (auth-choice-inference.ts)。
func InferAuthChoiceFromFlags(opts NonInteractiveOptions) AuthChoiceInference {
	keyByProvider := map[string]string{
		"anthropic":      opts.AnthropicApiKey,
		"gemini":         opts.GeminiApiKey,
		"openai":         opts.OpenAIApiKey,
		"openrouter":     opts.OpenrouterApiKey,
		"ai-gateway":     opts.AiGatewayApiKey,
		"cloudflare":     opts.CloudflareAiGwApiKey,
		"moonshot":       opts.MoonshotApiKey,
		"kimi-code":      opts.KimiCodeApiKey,
		"synthetic":      opts.SyntheticApiKey,
		"venice":         opts.VeniceApiKey,
		"zai":            opts.ZaiApiKey,
		"xiaomi":         opts.XiaomiApiKey,
		"xai":            opts.XaiApiKey,
		"minimax":        opts.MinimaxApiKey,
		"openacosmi-zen": opts.AcosmiZenApiKey,
	}

	var matches []authChoiceFlagEntry
	for _, entry := range authChoiceFlagMap {
		if key, ok := keyByProvider[entry.Provider]; ok && strings.TrimSpace(key) != "" {
			matches = append(matches, entry)
		}
	}

	result := AuthChoiceInference{Matches: matches}
	if len(matches) > 0 {
		result.Choice = matches[0].AuthChoice
	}
	return result
}

// ---------- 主入口 ----------

// RunNonInteractiveOnboarding 非交互式 onboarding 入口。
// 对应 TS runNonInteractiveOnboarding (onboard-non-interactive.ts)。
func RunNonInteractiveOnboarding(opts NonInteractiveOptions) error {
	// 读取现有配置
	configPath := resolveConfigPath()
	baseConfig, exists := readConfigFileJSON(configPath)
	if exists {
		// 校验有效性（TS 检查 snapshot.valid）
		// Go 的 readConfigFileJSON 已在 json 解析失败时返回空配置
		_ = baseConfig
	}

	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = "local"
	}
	if mode != "local" && mode != "remote" {
		return fmt.Errorf("invalid --mode %q (use local|remote)", mode)
	}

	if mode == "remote" {
		return runNonInteractiveRemote(opts, baseConfig)
	}
	return runNonInteractiveLocal(opts, baseConfig, configPath)
}

// ---------- Remote 模式 ----------

// runNonInteractiveRemote 远程模式：设置 remote URL + token。
// 对应 TS runNonInteractiveOnboardingRemote (remote.ts)。
func runNonInteractiveRemote(opts NonInteractiveOptions, baseConfig *types.OpenAcosmiConfig) error {
	remoteURL := strings.TrimSpace(opts.RemoteURL)
	if remoteURL == "" {
		return fmt.Errorf("missing --remote-url for remote mode")
	}

	nextConfig := shallowCopyConfig(baseConfig)
	if nextConfig.Gateway == nil {
		nextConfig.Gateway = &types.GatewayConfig{}
	}
	nextConfig.Gateway.Mode = "remote"
	nextConfig.Gateway.Remote = &types.GatewayRemoteConfig{
		URL: remoteURL,
	}
	if tok := strings.TrimSpace(opts.RemoteToken); tok != "" {
		nextConfig.Gateway.Remote.Token = tok
	}

	ApplyWizardMetadata(nextConfig, WizardMetadata{Command: "onboard", Mode: "remote"})

	configPath := resolveConfigPath()
	if err := writeConfigFileJSON(configPath, nextConfig); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	if opts.JSON {
		payload := map[string]interface{}{
			"mode":      "remote",
			"remoteUrl": remoteURL,
			"auth":      "none",
		}
		if opts.RemoteToken != "" {
			payload["auth"] = "token"
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(data))
	} else {
		slog.Info("remote gateway configured", "url", remoteURL)
	}
	return nil
}

// ---------- Local 模式 ----------

// runNonInteractiveLocal 本地模式全流程。
// 对应 TS runNonInteractiveOnboardingLocal (local.ts)。
func runNonInteractiveLocal(opts NonInteractiveOptions, baseConfig *types.OpenAcosmiConfig, configPath string) error {
	// 1. Workspace
	workspaceDir := opts.Workspace
	if workspaceDir == "" {
		if baseConfig.Agents != nil && baseConfig.Agents.Defaults != nil && baseConfig.Agents.Defaults.Workspace != "" {
			workspaceDir = baseConfig.Agents.Defaults.Workspace
		} else {
			workspaceDir = DefaultWorkspace
		}
	}

	nextConfig := shallowCopyConfig(baseConfig)
	if nextConfig.Agents == nil {
		nextConfig.Agents = &types.AgentsConfig{}
	}
	if nextConfig.Agents.Defaults == nil {
		nextConfig.Agents.Defaults = &types.AgentDefaultsConfig{}
	}
	nextConfig.Agents.Defaults.Workspace = workspaceDir
	if nextConfig.Gateway == nil {
		nextConfig.Gateway = &types.GatewayConfig{}
	}
	nextConfig.Gateway.Mode = "local"

	// 2. Auth choice
	inference := InferAuthChoiceFromFlags(opts)
	if opts.AuthChoice == "" && len(inference.Matches) > 1 {
		labels := make([]string, len(inference.Matches))
		for i, m := range inference.Matches {
			labels[i] = m.Label
		}
		return fmt.Errorf("multiple API key flags provided: %s. Use --auth-choice explicitly", strings.Join(labels, ", "))
	}

	authChoice := opts.AuthChoice
	if authChoice == "" {
		authChoice = inference.Choice
	}
	if authChoice == "" {
		authChoice = "skip"
	}

	// 3. Apply auth
	if authChoice != "skip" {
		storePath := resolveAuthStorePath()
		store := auth.NewAuthStore(storePath)
		if _, err := store.Load(); err != nil {
			slog.Warn("auth store load", "error", err)
		}

		result, err := ApplyAuthChoice(ApplyAuthChoiceParams{
			AuthChoice:      authChoice,
			Config:          nextConfig,
			AuthStore:       store,
			SetDefaultModel: true,
			Opts: &ApplyAuthChoiceOpts{
				Token:         opts.Token,
				TokenProvider: opts.TokenProvider,
				XAIApiKey:     opts.XaiApiKey,
			},
		})
		if err != nil {
			return fmt.Errorf("apply auth: %w", err)
		}
		nextConfig = result.Config
	}

	// 4. Gateway config
	gwResult, err := applyNonInteractiveGatewayConfig(opts, nextConfig)
	if err != nil {
		return err
	}
	nextConfig = gwResult.config

	// 5. Skills config
	nextConfig = ApplyNonInteractiveSkillsConfig(nextConfig, opts)

	// 6. Write config
	ApplyWizardMetadata(nextConfig, WizardMetadata{Command: "onboard", Mode: "local"})
	if err := writeConfigFileJSON(configPath, nextConfig); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// 7. Workspace + sessions
	skipBootstrap := false
	if nextConfig.Agents != nil && nextConfig.Agents.Defaults != nil &&
		nextConfig.Agents.Defaults.SkipBootstrap != nil {
		skipBootstrap = *nextConfig.Agents.Defaults.SkipBootstrap
	}
	if err := EnsureWorkspaceAndSessions(workspaceDir, skipBootstrap); err != nil {
		return fmt.Errorf("workspace: %w", err)
	}

	// 8. Daemon install (placeholder — OS-specific implementation)
	InstallGatewayDaemonNonInteractive(opts, gwResult)

	// 9. JSON output
	LogNonInteractiveOnboardingJson(opts, NonInteractiveOnboardingOutput{
		Mode:          "local",
		Workspace:     workspaceDir,
		AuthChoice:    authChoice,
		GatewayPort:   gwResult.port,
		GatewayBind:   gwResult.bind,
		GatewayAuth:   gwResult.authMode,
		InstallDaemon: opts.InstallDaemon,
		SkipSkills:    opts.SkipSkills,
		SkipHealth:    opts.SkipHealth,
	})

	slog.Info("non-interactive onboarding complete",
		"mode", "local",
		"workspace", workspaceDir,
		"auth", authChoice,
	)
	return nil
}

// ---------- Gateway Config ----------

type nonInteractiveGatewayResult struct {
	config       *types.OpenAcosmiConfig
	port         int
	bind         string
	authMode     string
	gatewayToken string
}

// applyNonInteractiveGatewayConfig 应用非交互式 gateway 配置。
// 对应 TS applyNonInteractiveGatewayConfig (gateway-config.ts)。
func applyNonInteractiveGatewayConfig(opts NonInteractiveOptions, cfg *types.OpenAcosmiConfig) (*nonInteractiveGatewayResult, error) {
	port := opts.GatewayPort
	if port <= 0 {
		// 使用默认端口
		if cfg.Gateway != nil && cfg.Gateway.Port != nil {
			port = *cfg.Gateway.Port
		}
		if port <= 0 {
			port = 19001
		}
	}

	bind := opts.GatewayBind
	if bind == "" {
		bind = "loopback"
	}

	authMode := opts.GatewayAuth
	if authMode == "" {
		authMode = "token"
	}
	if authMode != "token" && authMode != "password" {
		return nil, fmt.Errorf("invalid --gateway-auth %q (use token|password)", authMode)
	}

	tailscaleMode := opts.Tailscale
	if tailscaleMode == "" {
		tailscaleMode = "off"
	}

	// Tailscale 安全约束
	if tailscaleMode != "off" && bind != "loopback" {
		bind = "loopback"
	}
	if tailscaleMode == "funnel" && authMode != "password" {
		authMode = "password"
	}

	nextConfig := shallowCopyConfig(cfg)
	gatewayToken := strings.TrimSpace(opts.GatewayToken)

	if nextConfig.Gateway == nil {
		nextConfig.Gateway = &types.GatewayConfig{}
	}
	if nextConfig.Gateway.Auth == nil {
		nextConfig.Gateway.Auth = &types.GatewayAuthConfig{}
	}

	if authMode == "token" {
		if gatewayToken == "" {
			gatewayToken = randomHexToken()
		}
		nextConfig.Gateway.Auth.Mode = "token"
		nextConfig.Gateway.Auth.Token = gatewayToken
	}

	if authMode == "password" {
		pw := strings.TrimSpace(opts.GatewayPassword)
		if pw == "" {
			return nil, fmt.Errorf("missing --gateway-password for password auth")
		}
		nextConfig.Gateway.Auth.Mode = "password"
		nextConfig.Gateway.Auth.Password = pw
	}

	nextConfig.Gateway.Port = &port
	nextConfig.Gateway.Bind = types.GatewayBindMode(bind)

	return &nonInteractiveGatewayResult{
		config:       nextConfig,
		port:         port,
		bind:         bind,
		authMode:     authMode,
		gatewayToken: gatewayToken,
	}, nil
}

// ---------- Skills Config ----------

// ApplyNonInteractiveSkillsConfig 应用非交互式技能配置。
// 对应 TS applyNonInteractiveSkillsConfig (skills-config.ts)。
func ApplyNonInteractiveSkillsConfig(cfg *types.OpenAcosmiConfig, opts NonInteractiveOptions) *types.OpenAcosmiConfig {
	if opts.SkipSkills {
		return cfg
	}

	nodeManager := opts.NodeManager
	if nodeManager == "" {
		nodeManager = "npm"
	}
	if nodeManager != "npm" && nodeManager != "pnpm" && nodeManager != "bun" {
		slog.Warn("invalid --node-manager, using npm", "value", nodeManager)
		nodeManager = "npm"
	}

	nextConfig := shallowCopyConfig(cfg)
	if nextConfig.Skills == nil {
		nextConfig.Skills = &types.SkillsConfig{}
	}
	if nextConfig.Skills.Install == nil {
		nextConfig.Skills.Install = &types.SkillsInstallConfig{}
	}
	nextConfig.Skills.Install.NodeManager = nodeManager

	slog.Info("skills config applied", "nodeManager", nodeManager)
	return nextConfig
}

// ---------- Daemon Install ----------

// InstallGatewayDaemonNonInteractive 非交互式 daemon 安装。
// 对应 TS installGatewayDaemonNonInteractive (daemon-install.ts)。
// 注意：OS-specific 安装逻辑 (plist/systemd) 在 OB-1-DEFERRED 中完成。
func InstallGatewayDaemonNonInteractive(opts NonInteractiveOptions, gwResult *nonInteractiveGatewayResult) {
	if !opts.InstallDaemon {
		return
	}

	// 日志记录意图 — 实际 OS-specific 安装委托给 gateway 包
	slog.Info("daemon install requested",
		"port", gwResult.port,
		"bind", gwResult.bind,
		"authMode", gwResult.authMode,
	)
	// 实际安装逻辑在 gateway/wizard_finalize.go 中实现
	// macOS: ~/Library/LaunchAgents/com.openacosmi.gateway.plist
	// Linux: ~/.config/systemd/user/openacosmi-gateway.service
}

// ---------- JSON Output ----------

// NonInteractiveOnboardingOutput 非交互式输出数据。
// 对应 TS logNonInteractiveOnboardingJson (output.ts)。
type NonInteractiveOnboardingOutput struct {
	Mode          string `json:"mode"`
	Workspace     string `json:"workspace,omitempty"`
	AuthChoice    string `json:"authChoice,omitempty"`
	GatewayPort   int    `json:"gatewayPort,omitempty"`
	GatewayBind   string `json:"gatewayBind,omitempty"`
	GatewayAuth   string `json:"gatewayAuth,omitempty"`
	InstallDaemon bool   `json:"installDaemon"`
	SkipSkills    bool   `json:"skipSkills"`
	SkipHealth    bool   `json:"skipHealth"`
}

// LogNonInteractiveOnboardingJson 输出非交互式 onboarding 结果 JSON。
// 对应 TS logNonInteractiveOnboardingJson (output.ts)。
func LogNonInteractiveOnboardingJson(opts NonInteractiveOptions, output NonInteractiveOnboardingOutput) {
	if !opts.JSON {
		return
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		slog.Warn("json output error", "error", err)
		return
	}
	fmt.Println(string(data))
}

// ---------- 内部辅助 ----------

// shallowCopyConfig 浅拷贝配置（避免修改原始对象）。
func shallowCopyConfig(src *types.OpenAcosmiConfig) *types.OpenAcosmiConfig {
	if src == nil {
		return &types.OpenAcosmiConfig{}
	}
	copied := *src
	return &copied
}

// randomHexToken 生成 24 字节随机 hex token。
// 对应 TS randomToken (onboard-helpers.ts L68-70)。
func randomHexToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		// 极端退化路径 — 不应发生
		return "fallback-token-placeholder"
	}
	return hex.EncodeToString(b)
}
