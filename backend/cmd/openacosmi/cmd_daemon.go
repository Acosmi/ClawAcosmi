package main

import "github.com/spf13/cobra"

// 对应 TS src/cli/daemon-cli/ — Daemon 服务管理

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Gateway service (legacy alias)",
		Long:  "Daemon management — start/stop the background Gateway service. Alias for `gateway` commands.",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start the daemon",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("🔧 Daemon start not yet implemented")
				return nil
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the daemon",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("🛑 Daemon stop not yet implemented")
				return nil
			},
		},
		&cobra.Command{
			Use:   "install",
			Short: "Install daemon as system service",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("📦 Daemon install not yet implemented")
				return nil
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Uninstall daemon system service",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("🗑️ Daemon uninstall not yet implemented")
				return nil
			},
		},
	)

	return cmd
}
