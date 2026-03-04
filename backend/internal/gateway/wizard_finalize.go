package gateway

// wizard_finalize.go — Onboarding 完成阶段
// TS 对照: src/wizard/onboarding.finalize.ts (525L)
//
// 负责 daemon 服务安装、gateway 探测、control-ui 准备、
// TUI/Web/Later hatch 选择、shell completion、web search 提示。

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/daemon"
	"github.com/Acosmi/ClawAcosmi/pkg/i18n"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 类型定义 ----------

// FinalizeOnboardingOptions finalize 阶段入参。
type FinalizeOnboardingOptions struct {
	Flow       string // "quickstart" | "guided"
	BaseConfig *types.OpenAcosmiConfig
	NextConfig *types.OpenAcosmiConfig
	Settings   GatewayWizardSettings
	Prompter   WizardPrompter

	SkipHealth bool
	SkipUI     bool
}

// FinalizeResult finalize 阶段返回值。
type FinalizeResult struct {
	LaunchedTUI bool
}

// GatewayService daemon 服务抽象接口。
type GatewayService interface {
	IsLoaded() (bool, error)
	Install(port int, token string) error
	Restart() error
	Uninstall() error
}

// ---------- 主函数 ----------

// FinalizeOnboardingWizard 执行 onboarding 完成阶段。
func FinalizeOnboardingWizard(opts FinalizeOnboardingOptions) (*FinalizeResult, error) {
	prompter := opts.Prompter
	settings := opts.Settings
	nextConfig := opts.NextConfig
	baseConfig := opts.BaseConfig
	if nextConfig == nil {
		nextConfig = &types.OpenAcosmiConfig{}
	}
	if baseConfig == nil {
		baseConfig = &types.OpenAcosmiConfig{}
	}

	// --- 1. Daemon 服务安装 ---
	if err := handleDaemonInstall(opts.Flow, settings, prompter); err != nil {
		slog.Warn("daemon install", "error", err)
	}

	// --- 2. Gateway 可达性等待 + Health check ---
	if !opts.SkipHealth {
		probeLinks := ResolveControlUiLinks(ResolveControlUiLinksParams{
			Port:           settings.Port,
			Bind:           string(resolveBindOrDefault(nextConfig)),
			CustomBindHost: nextConfig.Gateway.CustomBindHost,
		})
		probe := WaitForGatewayReachable(WaitParams{
			URL:        probeLinks.WsURL,
			Token:      settings.GatewayToken,
			DeadlineMs: 15000,
		})
		if !probe.OK {
			slog.Warn("gateway not reachable after wait", "detail", probe.Detail)
		}
	}

	// --- 3. Control UI ---
	controlUiEnabled := true
	if nextConfig.Gateway != nil && nextConfig.Gateway.ControlUI != nil &&
		nextConfig.Gateway.ControlUI.Enabled != nil {
		controlUiEnabled = *nextConfig.Gateway.ControlUI.Enabled
	} else if baseConfig.Gateway != nil && baseConfig.Gateway.ControlUI != nil &&
		baseConfig.Gateway.ControlUI.Enabled != nil {
		controlUiEnabled = *baseConfig.Gateway.ControlUI.Enabled
	}
	if !opts.SkipUI && controlUiEnabled {
		// control-ui 资产检查（Go 端简化为日志提示）
		slog.Info("control-ui assets check: ensure built or download")
	}

	// --- 4. Optional apps 提示 ---
	_ = prompter.Note(
		"Add nodes for extra features:\n"+
			"- macOS app (system + notifications)\n"+
			"- iOS app (camera/canvas)\n"+
			"- Android app (camera/canvas)",
		"Optional apps", // 技术标签，不翻译
	)

	// --- 5. Control UI 链接展示 ---
	controlUiBasePath := resolveControlUiBasePath(nextConfig, baseConfig)
	links := ResolveControlUiLinks(ResolveControlUiLinksParams{
		Port:           settings.Port,
		Bind:           string(settings.Bind),
		CustomBindHost: settings.CustomBindHost,
		BasePath:       controlUiBasePath,
	})
	authedURL := buildAuthedURL(links.HttpURL, settings)

	gatewayProbe := ProbeGatewayReachable(ProbeParams{
		URL:   links.WsURL,
		Token: resolveProbeToken(settings, nextConfig),
	})
	gatewayStatusLine := "Gateway: reachable"
	if !gatewayProbe.OK {
		gatewayStatusLine = "Gateway: not detected"
		if gatewayProbe.Detail != "" {
			gatewayStatusLine += " (" + gatewayProbe.Detail + ")"
		}
	}

	noteLines := []string{
		"Web UI: " + links.HttpURL,
	}
	if settings.AuthMode == "token" && settings.GatewayToken != "" {
		noteLines = append(noteLines, "Web UI (with token): "+authedURL)
	}
	noteLines = append(noteLines,
		"Gateway WS: "+links.WsURL,
		gatewayStatusLine,
		"Docs: docs/skills/web/control-ui/SKILL.md",
	)
	_ = prompter.Note(strings.Join(noteLines, "\n"), i18n.Tp("onboard.controlui.title"))

	// --- 6. TUI / Web / Later hatch ---
	launchedTUI := false
	if !opts.SkipUI && gatewayProbe.OK {
		launchedTUI = handleHatchChoice(prompter, links, authedURL, settings, controlUiBasePath)
	}

	// --- 7. Shell completion ---
	handleShellCompletion(prompter)

	// --- 8. 引导提示 ---
	showFinalNotes(prompter, settings, nextConfig)

	return &FinalizeResult{LaunchedTUI: launchedTUI}, nil
}

// ---------- 子流程 ----------

// handleDaemonInstall 处理 daemon 服务安装/重启。
// 对应 TS: onboarding.finalize.ts L92-201。
func handleDaemonInstall(flow string, settings GatewayWizardSettings, prompter WizardPrompter) error {
	// quickstart 流程跳过 daemon 安装提示
	if flow == "quickstart" {
		_ = prompter.Note(
			i18n.Tp("onboard.daemon.quickstart"),
			i18n.Tp("onboard.daemon.title"),
		)
		return nil
	}

	installDaemon, err := prompter.Confirm(i18n.Tp("onboard.daemon.confirm"), true)
	if err != nil {
		return fmt.Errorf("daemon confirm: %w", err)
	}
	if !installDaemon {
		return nil
	}

	// 检测 systemd 可用性 (Linux)
	if !isSystemdAvailable() {
		_ = prompter.Note(
			i18n.Tp("onboard.daemon.systemd_unavail"),
			i18n.Tp("onboard.daemon.title"),
		)
		return nil
	}

	service := daemon.ResolveGatewayService()
	envMap := resolveEnvMap()

	// 检查是否已安装
	loaded, _ := service.IsLoaded(envMap)
	if loaded {
		action, selectErr := prompter.Select(i18n.Tp("onboard.daemon.already_installed"), []WizardStepOption{
			{Value: "restart", Label: i18n.Tp("onboard.daemon.opt_restart")},
			{Value: "reinstall", Label: i18n.Tp("onboard.daemon.opt_reinstall")},
			{Value: "skip", Label: i18n.Tp("onboard.daemon.opt_skip")},
		}, "restart")
		if selectErr != nil {
			return fmt.Errorf("daemon action select: %w", selectErr)
		}

		switch fmt.Sprint(action) {
		case "restart":
			slog.Info("restarting gateway service")
			if restartErr := service.Restart(envMap); restartErr != nil {
				return fmt.Errorf("daemon restart: %w", restartErr)
			}
			_ = prompter.Note(i18n.Tp("onboard.daemon.restarted"), i18n.Tp("onboard.daemon.title"))
			return nil

		case "reinstall":
			slog.Info("uninstalling gateway service for reinstall")
			if uninstallErr := service.Uninstall(envMap); uninstallErr != nil {
				slog.Warn("daemon uninstall", "error", uninstallErr)
			}
			// 继续到安装流程

		default: // "skip"
			return nil
		}
	}

	// 构建安装参数并安装
	progArgs, progErr := daemon.ResolveGatewayProgramArguments(settings.Port)
	if progErr != nil {
		_ = prompter.Note(
			i18n.Tf("onboard.daemon.install_failed", progErr),
			i18n.Tp("onboard.daemon.title"),
		)
		return nil
	}

	serviceEnv := daemon.BuildServiceEnvironment(envMap, settings.Port, settings.GatewayToken)

	slog.Info("installing gateway service",
		"programArguments", progArgs.ProgramArguments,
		"workingDirectory", progArgs.WorkingDirectory,
	)

	installErr := service.Install(daemon.GatewayServiceInstallArgs{
		Env:              envMap,
		ProgramArguments: progArgs.ProgramArguments,
		WorkingDirectory: progArgs.WorkingDirectory,
		Environment:      serviceEnv,
		Description:      "OpenAcosmi Gateway",
	})
	if installErr != nil {
		_ = prompter.Note(
			i18n.Tf("onboard.daemon.install_failed", installErr),
			i18n.Tp("onboard.daemon.title"),
		)
		hint := i18n.Tp("onboard.daemon.hint_unix")
		if runtime.GOOS == "windows" {
			hint = i18n.Tp("onboard.daemon.hint_windows")
		}
		_ = prompter.Note(hint, i18n.Tp("onboard.daemon.title"))
		return nil // 非致命：不阻断 onboarding
	}

	_ = prompter.Note(i18n.Tp("onboard.daemon.installed"), i18n.Tp("onboard.daemon.title"))
	return nil
}

// handleShellCompletion 提示用户安装 shell 补全。
// 对应 TS: onboarding.finalize.ts L400-442。
func handleShellCompletion(prompter WizardPrompter) {
	// 检测当前 shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		return
	}

	var shellName string
	var profileHint string
	switch {
	case strings.HasSuffix(shell, "/zsh"):
		shellName = "zsh"
		profileHint = "~/.zshrc"
	case strings.HasSuffix(shell, "/bash"):
		shellName = "bash"
		profileHint = "~/.bashrc"
	case strings.HasSuffix(shell, "/fish"):
		shellName = "fish"
		profileHint = "~/.config/fish/config.fish"
	default:
		return // 不支持的 shell
	}

	installCompletion, err := prompter.Confirm(
		i18n.Tf("onboard.completion.prompt", shellName),
		true,
	)
	if err != nil || !installCompletion {
		return
	}

	_ = prompter.Note(
		i18n.Tf("onboard.completion.hint", shellName, profileHint, profileHint),
		i18n.Tp("onboard.completion.title"),
	)
}

// isSystemdAvailable 检测 Linux systemd user service 是否可用（非 Linux 始终返回 true）。
func isSystemdAvailable() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	// 简单检测：XDG_RUNTIME_DIR 存在即可使用 systemd --user
	return os.Getenv("XDG_RUNTIME_DIR") != ""
}

// resolveEnvMap 将 os.Environ() 转为 map[string]string。
func resolveEnvMap() map[string]string {
	result := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// handleHatchChoice 处理 TUI/Web/Later 选择。
func handleHatchChoice(
	prompter WizardPrompter,
	links ControlUiLinks,
	authedURL string,
	settings GatewayWizardSettings,
	controlUiBasePath string,
) bool {
	// Token 提示
	_ = prompter.Note(
		"Gateway token: shared auth for the Gateway + Control UI.\n"+
			"Stored in: ~/.openacosmi/openacosmi.json (gateway.auth.token) or OPENACOSMI_GATEWAY_TOKEN.\n"+
			"View token: openacosmi config get gateway.auth.token\n"+
			"Generate token: openacosmi doctor --generate-gateway-token",
		"Token",
	)

	hatchOptions := []WizardStepOption{
		{Value: "tui", Label: i18n.Tp("onboard.hatch.opt_tui")},
		{Value: "web", Label: i18n.Tp("onboard.hatch.opt_web")},
		{Value: "later", Label: i18n.Tp("onboard.hatch.opt_later")},
	}
	choice, err := prompter.Select(i18n.Tp("onboard.hatch.prompt"), hatchOptions, "tui")
	if err != nil {
		return false
	}

	switch fmt.Sprint(choice) {
	case "tui":
		_ = prompter.Note(
			"To start TUI: openacosmi tui\n"+
				"Connect URL: "+links.WsURL,
			"TUI",
		)
		return true

	case "web":
		browserSupport := DetectBrowserOpenSupport()
		opened := false
		if browserSupport.OK {
			opened = OpenURL(authedURL)
		}
		if opened {
			_ = prompter.Note(
				"Dashboard link (with token): "+authedURL+"\n"+
					"Opened in your browser. Keep that tab to control OpenAcosmi.",
				"Dashboard ready",
			)
		} else {
			hint := FormatControlUiSshHint(
				settings.Port, controlUiBasePath,
				resolveTokenForHint(settings),
			)
			_ = prompter.Note(
				"Dashboard link (with token): "+authedURL+"\n"+
					"Copy/paste this URL in a browser on this machine.\n"+hint,
				"Dashboard ready",
			)
		}

	default: // "later"
		_ = prompter.Note(
			"When you're ready: openacosmi dashboard --no-open",
			"Later",
		)
	}
	return false
}

// showFinalNotes 展示最终引导提示。
func showFinalNotes(prompter WizardPrompter, settings GatewayWizardSettings, cfg *types.OpenAcosmiConfig) {
	// Workspace backup
	_ = prompter.Note(
		"Back up your agent workspace.\n"+
			"Docs: docs/skills/concepts/agent-workspace/SKILL.md",
		"Workspace backup",
	)
	// Security
	_ = prompter.Note(
		"Running agents on your computer is risky — harden your setup. See: docs/skills/general/security/SKILL.md",
		"Security",
	)
	// Web search
	webSearchEnv := strings.TrimSpace(os.Getenv("BRAVE_API_KEY"))
	hasWebSearchKey := webSearchEnv != ""
	if cfg.Tools != nil {
		if ws := cfg.Tools.Web; ws != nil {
			if s := ws.Search; s != nil && strings.TrimSpace(s.APIKey) != "" {
				hasWebSearchKey = true
			}
		}
	}
	if hasWebSearchKey {
		_ = prompter.Note(
			"Web search is enabled, so your agent can look things up online when needed.\n"+
				"Docs: docs/skills/tools/web/SKILL.md",
			"Web search (optional)",
		)
	} else {
		_ = prompter.Note(
			"If you want your agent to search the web, you'll need a Brave Search API key.\n"+
				"Set it up: openacosmi configure --section web\n"+
				"Docs: docs/skills/tools/web/SKILL.md",
			"Web search (optional)",
		)
	}
	// Outro
	_ = prompter.Note(
		"What now: https://github.com/Acosmi/Claw-Acismi",
		"What now",
	)
	_ = prompter.Outro(i18n.Tp("onboard.finalize.outro"))
}

// ---------- 辅助函数 ----------

func resolveBindOrDefault(cfg *types.OpenAcosmiConfig) types.GatewayBindMode {
	if cfg.Gateway != nil && cfg.Gateway.Bind != "" {
		return cfg.Gateway.Bind
	}
	return types.GatewayBindLoopback
}

func resolveControlUiBasePath(next, base *types.OpenAcosmiConfig) string {
	if next.Gateway != nil && next.Gateway.ControlUI != nil && next.Gateway.ControlUI.BasePath != "" {
		return next.Gateway.ControlUI.BasePath
	}
	if base.Gateway != nil && base.Gateway.ControlUI != nil && base.Gateway.ControlUI.BasePath != "" {
		return base.Gateway.ControlUI.BasePath
	}
	return ""
}

func buildAuthedURL(httpURL string, settings GatewayWizardSettings) string {
	if settings.AuthMode == "token" && settings.GatewayToken != "" {
		return httpURL + "#token=" + url.QueryEscape(settings.GatewayToken)
	}
	return httpURL
}

func resolveProbeToken(settings GatewayWizardSettings, cfg *types.OpenAcosmiConfig) string {
	if settings.AuthMode == "token" {
		return settings.GatewayToken
	}
	return ""
}

func resolveTokenForHint(settings GatewayWizardSettings) string {
	if settings.AuthMode == "token" {
		return settings.GatewayToken
	}
	return ""
}
