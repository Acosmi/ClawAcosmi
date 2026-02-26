package main

// setup_remote.go — Onboarding Remote Gateway 配置向导
// TS 对照: src/commands/onboard-remote.ts (156L)
//
// 提供 PromptRemoteGatewayConfig — Bonjour 发现 + 直连/SSH 选择 + URL/auth 配置。

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/infra"
	"github.com/anthropic/open-acosmi/internal/tui"
	"github.com/anthropic/open-acosmi/pkg/i18n"
	"github.com/anthropic/open-acosmi/pkg/types"
)

const defaultGatewayURL = "ws://127.0.0.1:18789"

// pickHost 从 beacon 中选择最佳主机地址。
// 对应 TS: pickHost (onboard-remote.ts L10-12)。
func pickHost(beacon *infra.GatewayBonjourBeacon) string {
	if beacon.TailnetDNS != "" {
		return beacon.TailnetDNS
	}
	if beacon.LanHost != "" {
		return beacon.LanHost
	}
	return beacon.Host
}

// buildBeaconLabel 构建 beacon 显示标签。
// 对应 TS: buildLabel (onboard-remote.ts L14-19)。
func buildBeaconLabel(beacon *infra.GatewayBonjourBeacon) string {
	host := pickHost(beacon)
	port := beacon.GatewayPort
	if port == 0 {
		port = beacon.Port
	}
	if port == 0 {
		port = 18789
	}
	title := beacon.DisplayName
	if title == "" {
		title = beacon.InstanceName
	}
	hint := "host unknown"
	if host != "" {
		hint = fmt.Sprintf("%s:%d", host, port)
	}
	return fmt.Sprintf("%s (%s)", title, hint)
}

// ensureWsUrl 确保 URL 非空，为空时返回默认值。
// 对应 TS: ensureWsUrl (onboard-remote.ts L22-28)。
func ensureWsUrl(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultGatewayURL
	}
	return trimmed
}

// PromptRemoteGatewayConfig 引导用户配置远程 gateway。
// 对应 TS: promptRemoteGatewayConfig (onboard-remote.ts L30-155)。
//
// 流程:
//  1. Bonjour 发现检测 (dns-sd / avahi-browse)
//  2. 可选 LAN 网关发现
//  3. 连接方式选择 (Direct WS / SSH tunnel)
//  4. URL 输入 + ws:// 验证
//  5. 认证选择 (token / no auth)
//  6. 返回更新后的 config
func PromptRemoteGatewayConfig(
	cfg *types.OpenAcosmiConfig,
	prompter tui.WizardPrompter,
) (*types.OpenAcosmiConfig, error) {
	var selectedBeacon *infra.GatewayBonjourBeacon
	suggestedURL := defaultGatewayURL
	if cfg != nil && cfg.Gateway != nil && cfg.Gateway.Remote != nil && cfg.Gateway.Remote.URL != "" {
		suggestedURL = cfg.Gateway.Remote.URL
	}

	// 1. Bonjour 工具检测
	hasBonjourTool := DetectBinary("dns-sd") || DetectBinary("avahi-browse")

	wantsDiscover := false
	if hasBonjourTool {
		var err error
		wantsDiscover, err = prompter.Confirm(i18n.Tp("onboard.remote.discover"), true)
		if err != nil {
			return cfg, fmt.Errorf("bonjour confirm: %w", err)
		}
	} else {
		prompter.Note(i18n.Tp("onboard.remote.discover_hint"), i18n.Tp("onboard.remote.title"))
	}

	// 2. LAN 发现
	if wantsDiscover {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var domains []string
		if cfg != nil && cfg.Discovery != nil && cfg.Discovery.WideArea != nil {
			if d := cfg.Discovery.WideArea.Domain; d != "" {
				domains = append(domains, d)
			}
		}

		beacons, err := infra.DiscoverGatewayBeacons(ctx, infra.DiscoverOpts{
			TimeoutMs: 2000,
			Domains:   domains,
		})

		if err != nil {
			prompter.Note(i18n.Tf("onboard.remote.discover_error", err), i18n.Tp("onboard.remote.title"))
		} else if len(beacons) == 0 {
			prompter.Note(i18n.Tp("onboard.remote.discover_none"), i18n.Tp("onboard.remote.title"))
		} else {
			prompter.Note(i18n.Tf("onboard.remote.discover_found", len(beacons)), i18n.Tp("onboard.remote.title"))

			// 网关选择
			selectOpts := make([]tui.PromptOption, 0, len(beacons)+1)
			for i := range beacons {
				selectOpts = append(selectOpts, tui.PromptOption{
					Value: fmt.Sprintf("%d", i),
					Label: buildBeaconLabel(&beacons[i]),
				})
			}
			selectOpts = append(selectOpts, tui.PromptOption{
				Value: "manual",
				Label: "Enter URL manually",
			})

			selection, err := prompter.Select(i18n.Tp("onboard.remote.select_gw"), selectOpts, "")
			if err != nil {
				return cfg, fmt.Errorf("gateway select: %w", err)
			}

			if selection != "manual" {
				var idx int
				if _, err := fmt.Sscanf(selection, "%d", &idx); err == nil && idx >= 0 && idx < len(beacons) {
					selectedBeacon = &beacons[idx]
				}
			}
		}
	}

	// 3. 连接方式选择
	if selectedBeacon != nil {
		host := pickHost(selectedBeacon)
		port := selectedBeacon.GatewayPort
		if port == 0 {
			port = 18789
		}
		if host != "" {
			modeOpts := []tui.PromptOption{
				{Value: "direct", Label: fmt.Sprintf("Direct gateway WS (%s:%d)", host, port)},
				{Value: "ssh", Label: "SSH tunnel (loopback)"},
			}
			mode, err := prompter.Select(i18n.Tp("onboard.remote.conn_method"), modeOpts, "")
			if err != nil {
				return cfg, fmt.Errorf("connection mode: %w", err)
			}
			if mode == "direct" {
				suggestedURL = fmt.Sprintf("ws://%s:%d", host, port)
			} else {
				suggestedURL = defaultGatewayURL
				sshPortHint := ""
				if selectedBeacon.SSHPort > 0 {
					sshPortHint = fmt.Sprintf(" -p %d", selectedBeacon.SSHPort)
				}
				prompter.Note(strings.Join([]string{
					"Start a tunnel before using the CLI:",
					fmt.Sprintf("ssh -N -L 18789:127.0.0.1:18789 <user>@%s%s", host, sshPortHint),
					"Docs: https://docs.openacosmi.ai/gateway/remote",
				}, "\n"), "SSH tunnel")
			}
		}
	}

	// 4. URL 输入
	urlInput, err := prompter.TextInput(
		i18n.Tp("onboard.remote.url_input"),
		"",
		suggestedURL,
		func(v string) string {
			trimmed := strings.TrimSpace(v)
			if strings.HasPrefix(trimmed, "ws://") || strings.HasPrefix(trimmed, "wss://") {
				return ""
			}
			return "URL must start with ws:// or wss://"
		},
	)
	if err != nil {
		return cfg, fmt.Errorf("url input: %w", err)
	}
	url := ensureWsUrl(urlInput)

	// 5. 认证选择
	authOpts := []tui.PromptOption{
		{Value: "token", Label: "Token (recommended)"},
		{Value: "off", Label: "No auth"},
	}
	authChoice, err := prompter.Select(i18n.Tp("onboard.remote.auth_method"), authOpts, "")
	if err != nil {
		return cfg, fmt.Errorf("auth select: %w", err)
	}

	existingToken := ""
	if cfg != nil && cfg.Gateway != nil && cfg.Gateway.Remote != nil {
		existingToken = cfg.Gateway.Remote.Token
	}

	token := ""
	if authChoice == "token" {
		tokenInput, err := prompter.TextInput(
			i18n.Tp("onboard.remote.token_input"),
			"",
			existingToken,
			func(v string) string {
				if strings.TrimSpace(v) == "" {
					return "Required"
				}
				return ""
			},
		)
		if err != nil {
			return cfg, fmt.Errorf("token input: %w", err)
		}
		token = strings.TrimSpace(tokenInput)
	}

	// 6. 构建更新后的配置
	next := shallowCopyConfig(cfg)
	if next.Gateway == nil {
		next.Gateway = &types.GatewayConfig{}
	}
	next.Gateway.Mode = "remote"
	if next.Gateway.Remote == nil {
		next.Gateway.Remote = &types.GatewayRemoteConfig{}
	}
	next.Gateway.Remote.URL = url
	next.Gateway.Remote.Token = token

	return next, nil
}
