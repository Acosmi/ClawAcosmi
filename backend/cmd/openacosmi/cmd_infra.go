package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/openacosmi/claw-acismi/internal/config"
	"github.com/openacosmi/claw-acismi/internal/infra"
)

// 对应 TS pairing、dns、ports、webhooks、system-presence

func newInfraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "infra",
		Short: "Infrastructure tools",
		Long:  "Pairing, DNS, ports, webhooks, and system presence utilities.",
	}

	cmd.AddCommand(
		newPairingCmd(),
		newDNSCmd(),
		newWebhooksCmd(),
		newPresenceCmd(),
		newPortsCmd(),
	)

	return cmd
}

// ---------- pairing ----------

func newPairingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pairing",
		Short: "Device pairing",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start pairing mode",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("🔗 Pairing start not yet implemented")
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Pairing status",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("📊 Pairing status not yet implemented")
				return nil
			},
		},
	)
	return cmd
}

// ---------- dns ----------

func newDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "DNS helpers",
		Long:  "DNS lookup and gateway discovery utilities.",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "lookup <host>",
			Short: "DNS lookup a hostname",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				host := args[0]
				start := time.Now()
				addrs, err := net.LookupHost(host)
				elapsed := time.Since(start)
				if err != nil {
					return fmt.Errorf("DNS 查询失败 %s: %w", host, err)
				}
				cmd.Printf("🌐 %s → %s  (%dms)\n", host, strings.Join(addrs, ", "), elapsed.Milliseconds())
				return nil
			},
		},
		&cobra.Command{
			Use:   "discover",
			Short: "Discover local gateway instances via mDNS",
			RunE: func(cmd *cobra.Command, args []string) error {
				timeout := 3 * time.Second
				cmd.Printf("🔍 扫描 mDNS 中（等待 %s）…\n", timeout)
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				beacons, err := infra.DiscoverGatewayBeacons(ctx, infra.DiscoverOpts{
					TimeoutMs: int(timeout.Milliseconds()),
				})
				if err != nil {
					return fmt.Errorf("mDNS 发现失败: %w", err)
				}
				if len(beacons) == 0 {
					cmd.Println("  （未发现本地 gateway 实例）")
					return nil
				}
				for _, b := range beacons {
					cmd.Printf("  • %s  %s:%d", b.InstanceName, b.Host, b.Port)
					if b.TailnetDNS != "" {
						cmd.Printf("  tailnet=%s", b.TailnetDNS)
					}
					cmd.Println()
				}
				return nil
			},
		},
	)

	return cmd
}

// ---------- webhooks ----------

func newWebhooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhooks",
		Short: "Webhook management",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List configured webhooks",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("📋 Webhooks list not yet implemented")
				return nil
			},
		},
		&cobra.Command{
			Use:   "test",
			Short: "Send a test payload to a webhook URL",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("🧪 Webhooks test not yet implemented")
				return nil
			},
		},
	)
	return cmd
}

// ---------- presence ----------

func newPresenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "presence",
		Short: "Check local gateway presence",
		Long:  "Show whether a gateway instance is currently running on this machine.",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")

			stateDir := config.ResolveStateDir()
			gatewayPort := config.ResolveGatewayPort(nil)

			presence := infra.CheckSystemPresence(stateDir, gatewayPort)

			if jsonFlag {
				data, _ := json.MarshalIndent(presence, "", "  ")
				cmd.Println(string(data))
				return nil
			}

			if presence.GatewayRunning {
				cmd.Printf("✅ Gateway 正在运行  端口: %d", presence.GatewayPort)
				if presence.LockFilePID > 0 {
					cmd.Printf("  PID: %d", presence.LockFilePID)
				}
				cmd.Println()
			} else {
				cmd.Printf("⚠️  Gateway 未运行（端口 %d 无响应）\n", presence.GatewayPort)
				if presence.LockFileStale {
					cmd.Println("   锁文件存在但进程已死，可能是上次未正常退出")
				}
			}

			if presence.StateDirReady {
				cmd.Printf("✅ 状态目录可写  %s\n", presence.StateDir)
			} else {
				cmd.Printf("❌ 状态目录不可用  %s\n", presence.StateDir)
			}

			// Canvas URL
			canvasURL := infra.CanvasURLFromGatewayPort(gatewayPort, false)
			cmd.Printf("   Canvas: %s\n", canvasURL)

			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}

// ---------- ports ----------

func newPortsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ports",
		Short: "Show derived port assignments",
		Long:  "Display all port numbers derived from the gateway port.",
		RunE: func(cmd *cobra.Command, args []string) error {
			gatewayPort := config.ResolveGatewayPort(nil)
			bridgePort := config.DeriveDefaultBridgePort(gatewayPort)
			browserCtrlPort := config.DeriveDefaultBrowserControlPort(gatewayPort)
			canvasPort := config.DeriveDefaultCanvasHostPort(gatewayPort)
			cdpRange := config.DeriveDefaultBrowserCDPPortRange(browserCtrlPort)

			cmd.Printf("  Gateway:        %d\n", gatewayPort)
			cmd.Printf("  Bridge:         %d\n", bridgePort)
			cmd.Printf("  BrowserControl: %d\n", browserCtrlPort)
			cmd.Printf("  Canvas:         %d\n", canvasPort)
			cmd.Printf("  CDP range:      %d–%d\n", cdpRange.Start, cdpRange.End)
			return nil
		},
	}
}
