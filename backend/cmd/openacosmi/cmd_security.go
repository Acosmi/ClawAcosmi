package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openacosmi/claw-acismi/internal/config"
	"github.com/openacosmi/claw-acismi/internal/security"
)

// 对应 TS src/cli/security-cli.ts + src/cli/update-cli.ts

func newSecurityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Security and update tools",
	}

	cmd.AddCommand(
		newAuditCmd(),
		newUpdateCmd(),
	)

	return cmd
}

func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Run security audit",
		Long:  "Scan configuration, filesystem permissions, and runtime settings for security issues.",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")
			deepFlag, _ := cmd.Flags().GetBool("deep")

			// --- 1. 构建 SecurityAuditOptions ---
			stateDir := config.ResolveStateDir()
			configPath := config.ResolveConfigPath()

			opts := security.SecurityAuditOptions{
				StateDir:          stateDir,
				ConfigPath:        configPath,
				IncludeFilesystem: deepFlag,
			}

			// 尝试加载配置快照，失败时仍可执行基础审计
			cfgLoader := config.NewConfigLoader()
			snapshot, err := cfgLoader.ReadConfigFileSnapshot()
			if err == nil && snapshot.Valid {
				cfg := snapshot.Config

				// Gateway 配置快照
				if gw := cfg.Gateway; gw != nil {
					gwSnap := &security.GatewayConfigSnapshot{
						Bind:           string(gw.Bind),
						TrustedProxies: gw.TrustedProxies,
					}
					if gw.Auth != nil {
						gwSnap.AuthMode = string(gw.Auth.Mode)
						gwSnap.AuthToken = gw.Auth.Token
						gwSnap.AuthPassword = gw.Auth.Password
					}
					if gw.Tailscale != nil {
						gwSnap.TailscaleMode = string(gw.Tailscale.Mode)
					}
					if gw.ControlUI != nil {
						gwSnap.ControlUIEnabled = gw.ControlUI.Enabled != nil && *gw.ControlUI.Enabled
						if gw.ControlUI.AllowInsecureAuth != nil {
							gwSnap.AllowInsecureAuth = *gw.ControlUI.AllowInsecureAuth
						}
						if gw.ControlUI.DangerouslyDisableDeviceAuth != nil {
							gwSnap.DangerouslyDisableDeviceAuth = *gw.ControlUI.DangerouslyDisableDeviceAuth
						}
					}
					opts.GatewayConfig = gwSnap
				}

				// Logging 配置快照
				if lc := cfg.Logging; lc != nil {
					opts.LoggingConfig = &security.LoggingConfigSnapshot{
						RedactSensitive: lc.RedactSensitive,
					}
				}

				// Hooks 配置快照
				if hc := cfg.Hooks; hc != nil {
					hcSnap := &security.HooksConfigSnapshot{
						Token: hc.Token,
						Path:  hc.Path,
					}
					if hc.Enabled != nil {
						hcSnap.Enabled = *hc.Enabled
					}
					// 用于检测 token 复用
					if gw := cfg.Gateway; gw != nil && gw.Auth != nil {
						hcSnap.GatewayToken = gw.Auth.Token
					}
					opts.HooksConfig = hcSnap
				}

				// Browser 配置快照
				if bc := cfg.Browser; bc != nil {
					bcSnap := &security.BrowserConfigSnapshot{
						Enabled: bc.Enabled != nil && *bc.Enabled,
					}
					for name, profile := range bc.Profiles {
						if profile == nil {
							continue
						}
						cdpURL := profile.CdpURL
						isLoopback := isLoopbackURL(cdpURL)
						bcSnap.Profiles = append(bcSnap.Profiles, security.BrowserProfileSnapshot{
							Name:       name,
							CDPUrl:     cdpURL,
							IsLoopback: isLoopback,
						})
					}
					opts.BrowserConfig = bcSnap
				}
			}

			// --- 2. 执行审计 ---
			report, err := security.RunSecurityAudit(opts)
			if err != nil {
				return fmt.Errorf("安全审计失败: %w", err)
			}

			// --- 3. 输出结果 ---
			if jsonFlag {
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("JSON 序列化失败: %w", err)
				}
				cmd.Println(string(data))
				return nil
			}

			// 人类可读输出
			cmd.Println("🔒 安全审计报告")
			cmd.Println()

			s := report.Summary
			cmd.Printf("  摘要: %d critical · %d warn · %d info\n", s.Critical, s.Warn, s.Info)
			cmd.Println()

			if len(report.Findings) == 0 {
				cmd.Println("  ✅ 未发现安全问题")
				return nil
			}

			for _, f := range report.Findings {
				icon := "🔵"
				switch f.Severity {
				case security.SeverityCritical:
					icon = "🔴"
				case security.SeverityWarn:
					icon = "🟡"
				}
				cmd.Printf("  %s [%s] %s\n", icon, f.CheckID, f.Title)
				cmd.Printf("     %s\n", f.Detail)
				if f.Remediation != "" {
					cmd.Printf("     💡 %s\n", f.Remediation)
				}
				cmd.Println()
			}

			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("deep", false, "Include filesystem permission checks")
	return cmd
}

// isLoopbackURL 检测 CDP URL 是否指向环回地址。
func isLoopbackURL(rawURL string) bool {
	// 简化检测：检查 host 部分是否为 localhost 或 127.x.x.x
	if strings.Contains(rawURL, "localhost") || strings.Contains(rawURL, "127.0.0.1") {
		return true
	}
	// 尝试更精确的检测
	host := rawURL
	// 去掉 scheme
	if idx := strings.Index(host, "://"); idx >= 0 {
		host = host[idx+3:]
	}
	// 去掉 path
	if idx := strings.IndexByte(host, '/'); idx >= 0 {
		host = host[:idx]
	}
	// 检查 IP
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("🔄 Update check not yet implemented")
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "Force update")
	return cmd
}
