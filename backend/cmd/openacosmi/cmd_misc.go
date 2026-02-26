package main

import "github.com/spf13/cobra"

// 对应 TS acp、logs、docs、memory、sandbox、system、completion、directory、
// message、configure、health、sessions 等杂项命令

func newMiscCmd() *cobra.Command {
	misc := &cobra.Command{
		Use:    "misc",
		Short:  "Miscellaneous tools",
		Hidden: true, // 内部组织用，用户不直接看到
	}
	// 杂项命令直接注册到 root，不需要包装
	return misc
}

func init() {
	// 直接注册到 root 命令的非分组命令
	rootCmd.AddCommand(
		newHealthCmd(),
		newSessionsCmd(),
		newMessageCmd(),
		newConfigureCmd(),
		newACPCmd(),
		newLogsCmd(),
		newDocsCmd(),
		newMemoryCmd(),
		newSystemCmd(),
		newDirectoryCmd(),
		newCompletionCmd(),
		newDashboardCmd(),
		newResetCmd(),
	)
}

func newHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "System health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, _ := cmd.Flags().GetInt("timeout")
			_ = timeout
			cmd.Println("❤️ Health check not yet implemented")
			return nil
		},
	}
	cmd.Flags().Int("timeout", 10000, "Timeout in milliseconds")
	return cmd
}

func newSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage active sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("📋 Sessions not yet implemented")
			return nil
		},
	}
	cmd.Flags().String("store", "", "Session store")
	cmd.Flags().String("active", "", "Active session filter")
	return cmd
}

func newMessageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Send and manage messages",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "send",
			Short: "Send a message",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("📨 Message send not yet implemented")
				return nil
			},
		},
	)
	return cmd
}

func newConfigureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Configuration wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("⚙️ Configure not yet implemented")
			return nil
		},
	}
	return cmd
}

func newACPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acp",
		Short: "Agent Control Protocol tools",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "ACP status",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("📊 ACP status not yet implemented")
				return nil
			},
		},
		&cobra.Command{
			Use:   "invoke",
			Short: "Invoke an ACP method",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("🔧 ACP invoke not yet implemented")
				return nil
			},
		},
	)
	return cmd
}

// newLogsCmd 定义在 cmd_logs.go
// newMemoryCmd 定义在 cmd_memory.go

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Open documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("📖 Docs: https://docs.openacosmi.ai")
			return nil
		},
	}
}

// newSandboxCmd 已移至 cmd_sandbox.go（完整实现）

func newSystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System events, heartbeat, and presence",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "events",
			Short: "Show system events",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("📊 System events not yet implemented")
				return nil
			},
		},
		&cobra.Command{
			Use:   "heartbeat",
			Short: "Send heartbeat",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.Println("💓 Heartbeat not yet implemented")
				return nil
			},
		},
	)
	return cmd
}

func newDirectoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "directory",
		Short: "Directory commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("📂 Directory not yet implemented")
			return nil
		},
	}
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for OpenAcosmi CLI.

Example:
  openacosmi completion bash > /etc/bash_completion.d/openacosmi
  openacosmi completion zsh > "${fpath[1]}/_openacosmi"
  openacosmi completion fish > ~/.config/fish/completions/openacosmi.fish`,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return rootCmd.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				cmd.Printf("Unsupported shell: %s\n", args[0])
				return nil
			}
		},
	}
}

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Open dashboard in browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("📊 Dashboard not yet implemented")
			return nil
		},
	}
}

func newResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset OpenAcosmi state",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("⚠️ Reset not yet implemented")
			return nil
		},
	}
	cmd.Flags().Bool("confirm", false, "Confirm reset (required)")
	return cmd
}
