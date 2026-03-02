package gateway

// wizard_gateway_config.go — Gateway 网络配置向导
// TS 对照：src/wizard/onboarding.gateway-config.ts (287L)
//
// 提供 bind mode/port/auth/Tailscale 的交互式配置。
// quickstart 模式下使用默认值跳过交互。

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/i18n"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- 常量 ----------

// DefaultGatewayPort 默认 gateway 端口。
const DefaultGatewayPort = 19001

// DefaultDangerousNodeDenyCommands 高风险节点命令默认拒绝列表。
// 这些命令涉及隐私写入/录制，需要用户通过 /phone arm 显式启用。
var DefaultDangerousNodeDenyCommands = []string{
	"camera.snap",
	"camera.clip",
	"screen.record",
	"calendar.add",
	"contacts.add",
	"reminders.add",
}

// ---------- 类型定义 ----------

// GatewayWizardSettings 向导产出的 gateway 配置。
type GatewayWizardSettings struct {
	Port                 int
	Bind                 types.GatewayBindMode
	CustomBindHost       string
	AuthMode             string // "token" | "password"
	GatewayToken         string
	TailscaleMode        types.GatewayTailscaleMode
	TailscaleResetOnExit bool
}

// QuickstartGatewayDefaults quickstart 模式的默认值。
type QuickstartGatewayDefaults struct {
	Port                 int
	Bind                 types.GatewayBindMode
	CustomBindHost       string
	AuthMode             string // "token" | "password"
	Token                string
	Password             string
	TailscaleMode        types.GatewayTailscaleMode
	TailscaleResetOnExit bool
	HasExisting          bool
}

// ConfigureGatewayOptions 配置函数入参。
type ConfigureGatewayOptions struct {
	Flow              string // "quickstart" | "guided"
	BaseConfig        *types.OpenAcosmiConfig
	NextConfig        *types.OpenAcosmiConfig
	LocalPort         int
	QuickstartGateway QuickstartGatewayDefaults
	Prompter          WizardPrompter
}

// ConfigureGatewayResult 配置函数返回值。
type ConfigureGatewayResult struct {
	NextConfig *types.OpenAcosmiConfig
	Settings   GatewayWizardSettings
}

// ---------- 主函数 ----------

// ConfigureGatewayForOnboarding 运行 gateway 网络配置向导。
// 对应 TS configureGatewayForOnboarding (onboarding.gateway-config.ts L42-286)。
func ConfigureGatewayForOnboarding(opts ConfigureGatewayOptions) (*ConfigureGatewayResult, error) {
	flow := opts.Flow
	localPort := opts.LocalPort
	if localPort <= 0 {
		localPort = DefaultGatewayPort
	}
	qs := opts.QuickstartGateway
	prompter := opts.Prompter
	nextConfig := opts.NextConfig
	if nextConfig == nil {
		nextConfig = &types.OpenAcosmiConfig{}
	}

	// --- 1. Port ---
	var port int
	if flow == "quickstart" {
		port = qs.Port
		if port <= 0 {
			port = DefaultGatewayPort
		}
	} else {
		portStr, err := prompter.Text(
			"Gateway port",
			"",
			strconv.Itoa(localPort),
			false,
		)
		if err != nil {
			return nil, fmt.Errorf("port input: %w", err)
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil || parsed <= 0 || parsed > 65535 {
			return nil, fmt.Errorf("invalid port: %s", portStr)
		}
		port = parsed
	}

	// --- 2. Bind mode ---
	var bind types.GatewayBindMode
	if flow == "quickstart" {
		bind = qs.Bind
		if bind == "" {
			bind = types.GatewayBindLoopback
		}
	} else {
		bindOptions := []WizardStepOption{
			{Value: string(types.GatewayBindLoopback), Label: "Loopback (127.0.0.1)"},
			{Value: string(types.GatewayBindLAN), Label: "LAN (0.0.0.0)"},
			{Value: string(types.GatewayBindTailnet), Label: "Tailnet (Tailscale IP)"},
			{Value: string(types.GatewayBindAuto), Label: "Auto (Loopback → LAN)"},
			{Value: string(types.GatewayBindCustom), Label: "Custom IP"},
		}
		selected, err := prompter.Select(i18n.Tp("onboard.gw.bind_title"), bindOptions, string(types.GatewayBindLoopback))
		if err != nil {
			return nil, fmt.Errorf("bind select: %w", err)
		}
		bind = types.GatewayBindMode(fmt.Sprint(selected))
	}

	// --- 3. Custom IP ---
	customBindHost := qs.CustomBindHost
	if bind == types.GatewayBindCustom {
		needsPrompt := flow != "quickstart" || customBindHost == ""
		if needsPrompt {
			input, err := prompter.Text(
				"Custom IP address",
				"192.168.1.100",
				customBindHost,
				false,
			)
			if err != nil {
				return nil, fmt.Errorf("custom IP input: %w", err)
			}
			trimmed := strings.TrimSpace(input)
			if trimmed == "" {
				return nil, fmt.Errorf("IP address is required for custom bind mode")
			}
			if !IsValidIPv4(trimmed) {
				return nil, fmt.Errorf("invalid IPv4 address: %s", trimmed)
			}
			customBindHost = trimmed
		}
	}

	// --- 4. Auth mode ---
	var authMode string
	if flow == "quickstart" {
		authMode = qs.AuthMode
		if authMode == "" {
			authMode = "token"
		}
	} else {
		authOptions := []WizardStepOption{
			{Value: "token", Label: "Token", Hint: "Recommended default (local + remote)"},
			{Value: "password", Label: "Password"},
		}
		selected, err := prompter.Select(i18n.Tp("onboard.gw.auth_title"), authOptions, "token")
		if err != nil {
			return nil, fmt.Errorf("auth select: %w", err)
		}
		authMode = fmt.Sprint(selected)
	}

	// --- 5. Tailscale mode ---
	var tailscaleMode types.GatewayTailscaleMode
	if flow == "quickstart" {
		tailscaleMode = qs.TailscaleMode
		if tailscaleMode == "" {
			tailscaleMode = types.TailscaleOff
		}
	} else {
		tsOptions := []WizardStepOption{
			{Value: string(types.TailscaleOff), Label: "Off", Hint: "No Tailscale exposure"},
			{Value: string(types.TailscaleServe), Label: "Serve", Hint: "Private HTTPS for your tailnet"},
			{Value: string(types.TailscaleFunnel), Label: "Funnel", Hint: "Public HTTPS via Tailscale Funnel"},
		}
		selected, err := prompter.Select(i18n.Tp("onboard.gw.ts_title"), tsOptions, string(types.TailscaleOff))
		if err != nil {
			return nil, fmt.Errorf("tailscale select: %w", err)
		}
		tailscaleMode = types.GatewayTailscaleMode(fmt.Sprint(selected))
	}

	// --- 5a. Tailscale binary 检测 ---
	if tailscaleMode != types.TailscaleOff {
		if !findTailscaleBinary() {
			_ = prompter.Note(
				"Tailscale binary not found in PATH or /Applications.\n"+
					"Ensure Tailscale is installed from:\n"+
					"  https://tailscale.com/download/mac\n\n"+
					"You can continue setup, but serve/funnel will fail at runtime.",
				"Tailscale Warning",
			)
		}
	}

	// --- 5b. Tailscale resetOnExit ---
	tailscaleResetOnExit := qs.TailscaleResetOnExit
	if tailscaleMode != types.TailscaleOff && flow != "quickstart" {
		_ = prompter.Note(
			"Docs:\ndocs/skills/gateway/tailscale/SKILL.md\ndocs/skills/web/control-ui/SKILL.md",
			"Tailscale",
		)
		reset, err := prompter.Confirm(i18n.Tp("onboard.gw.ts_reset_confirm"), false)
		if err == nil {
			tailscaleResetOnExit = reset
		}
	}

	// --- 5c. Tailscale 安全约束 ---
	if tailscaleMode != types.TailscaleOff && bind != types.GatewayBindLoopback {
		_ = prompter.Note(i18n.Tp("onboard.gw.ts_bind_note"), "Note")
		bind = types.GatewayBindLoopback
		customBindHost = ""
	}
	if tailscaleMode == types.TailscaleFunnel && authMode != "password" {
		_ = prompter.Note(i18n.Tp("onboard.gw.ts_funnel_auth_note"), "Note")
		authMode = "password"
	}

	// --- 6. Token 生成 ---
	var gatewayToken string
	if authMode == "token" {
		if flow == "quickstart" {
			gatewayToken = qs.Token
			if gatewayToken == "" {
				gatewayToken = RandomToken()
			}
		} else {
			tokenInput, err := prompter.Text(
				"Gateway token (blank to generate)",
				"Needed for multi-machine or non-loopback access",
				"",
				false,
			)
			if err != nil {
				return nil, fmt.Errorf("token input: %w", err)
			}
			gatewayToken = NormalizeGatewayTokenInput(tokenInput)
			if gatewayToken == "" {
				gatewayToken = RandomToken()
			}
		}
	}

	// --- 7. Password ---
	if authMode == "password" {
		var password string
		if flow == "quickstart" && qs.Password != "" {
			password = qs.Password
		} else {
			pw, err := prompter.Text("Gateway password", "", "", true)
			if err != nil {
				return nil, fmt.Errorf("password input: %w", err)
			}
			password = strings.TrimSpace(pw)
			if password == "" {
				return nil, fmt.Errorf("password is required")
			}
		}
		ensureGatewayConfig(nextConfig)
		nextConfig.Gateway.Auth = &types.GatewayAuthConfig{
			Mode:     types.GatewayAuthPassword,
			Password: password,
		}
	} else if authMode == "token" {
		ensureGatewayConfig(nextConfig)
		nextConfig.Gateway.Auth = &types.GatewayAuthConfig{
			Mode:  types.GatewayAuthToken,
			Token: gatewayToken,
		}
	}

	// --- 8. 写入 config ---
	ensureGatewayConfig(nextConfig)
	portVal := port
	nextConfig.Gateway.Port = &portVal
	nextConfig.Gateway.Bind = bind
	if bind == types.GatewayBindCustom && customBindHost != "" {
		nextConfig.Gateway.CustomBindHost = customBindHost
	}

	resetOnExit := tailscaleResetOnExit
	nextConfig.Gateway.Tailscale = &types.GatewayTailscaleConfig{
		Mode:        tailscaleMode,
		ResetOnExit: &resetOnExit,
	}

	// --- 9. 安全命令默认拒绝列表 ---
	if !qs.HasExisting &&
		nextConfig.Gateway.Nodes == nil {
		nextConfig.Gateway.Nodes = &types.GatewayNodesConfig{
			DenyCommands: append([]string{}, DefaultDangerousNodeDenyCommands...),
		}
	} else if nextConfig.Gateway.Nodes != nil &&
		len(nextConfig.Gateway.Nodes.DenyCommands) == 0 &&
		len(nextConfig.Gateway.Nodes.AllowCommands) == 0 &&
		nextConfig.Gateway.Nodes.Browser == nil &&
		!qs.HasExisting {
		nextConfig.Gateway.Nodes.DenyCommands = append([]string{}, DefaultDangerousNodeDenyCommands...)
	}

	return &ConfigureGatewayResult{
		NextConfig: nextConfig,
		Settings: GatewayWizardSettings{
			Port:                 port,
			Bind:                 bind,
			CustomBindHost:       customBindHost,
			AuthMode:             authMode,
			GatewayToken:         gatewayToken,
			TailscaleMode:        tailscaleMode,
			TailscaleResetOnExit: tailscaleResetOnExit,
		},
	}, nil
}

// ---------- 内部辅助 ----------

// ensureGatewayConfig 确保 config 的 Gateway 字段不为 nil。
func ensureGatewayConfig(cfg *types.OpenAcosmiConfig) {
	if cfg.Gateway == nil {
		cfg.Gateway = &types.GatewayConfig{}
	}
}

// findTailscaleBinary 检测 Tailscale 二进制是否可用。
func findTailscaleBinary() bool {
	// 1. 检查 PATH
	if _, err := exec.LookPath("tailscale"); err == nil {
		return true
	}
	// 2. macOS 特定路径
	paths := []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/usr/local/bin/tailscale",
	}
	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			return true
		}
	}
	return false
}
