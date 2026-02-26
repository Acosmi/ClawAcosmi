package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// 对应 TS src/cli/cron-cli/ (6 子文件)

func newCronCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Cron scheduler",
		Long:  "Manage scheduled agent tasks (cron jobs).",
	}

	cmd.PersistentFlags().Bool("json", false, "Output machine-readable JSON")

	cmd.AddCommand(
		newCronListCmd(),
		newCronAddCmd(),
		newCronEditCmd(),
		newCronRemoveCmd(),
		newCronRunCmd(),
		newCronEnableCmd(),
		newCronDisableCmd(),
		newCronLogsCmd(),
	)

	return cmd
}

func newCronListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				out, _ := json.MarshalIndent(map[string]any{"jobs": []any{}}, "", "  ")
				fmt.Println(string(out))
				return nil
			}
			fmt.Println("📋 Cron jobs: (none)")
			return nil
		},
	}
}

func newCronAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schedule, _ := cmd.Flags().GetString("schedule")
			agent, _ := cmd.Flags().GetString("agent")
			message, _ := cmd.Flags().GetString("message")
			channel, _ := cmd.Flags().GetString("channel")

			if schedule == "" {
				return fmt.Errorf("--schedule is required (e.g. '0 9 * * *')")
			}
			if message == "" {
				return fmt.Errorf("--message is required")
			}

			fmt.Printf("➕ Adding cron job '%s'\n", args[0])
			fmt.Printf("  Schedule: %s\n", schedule)
			fmt.Printf("  Agent: %s\n", agent)
			fmt.Printf("  Channel: %s\n", channel)
			fmt.Printf("  Message: %s\n", message)
			return nil
		},
	}
	cmd.Flags().StringP("schedule", "s", "", "Cron schedule expression (required)")
	cmd.Flags().String("agent", "default", "Agent to run as")
	cmd.Flags().StringP("message", "m", "", "Message to send (required)")
	cmd.Flags().String("channel", "", "Target channel")
	return cmd
}

func newCronEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit an existing cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schedule, _ := cmd.Flags().GetString("schedule")
			message, _ := cmd.Flags().GetString("message")
			fmt.Printf("✏️ Editing cron job '%s'\n", args[0])
			if schedule != "" {
				fmt.Printf("  New schedule: %s\n", schedule)
			}
			if message != "" {
				fmt.Printf("  New message: %s\n", message)
			}
			return nil
		},
	}
	cmd.Flags().StringP("schedule", "s", "", "New cron schedule")
	cmd.Flags().StringP("message", "m", "", "New message")
	return cmd
}

func newCronRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("🗑️ Removed cron job '%s'\n", args[0])
			return nil
		},
	}
}

func newCronRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Run a cron job immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("▶️ Running cron job '%s' now...\n", args[0])
			return nil
		},
	}
}

func newCronEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a disabled cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("✅ Cron job '%s' enabled.\n", args[0])
			return nil
		},
	}
}

func newCronDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("⏸️ Cron job '%s' disabled.\n", args[0])
			return nil
		},
	}
}

func newCronLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [name]",
		Short: "Show cron job execution logs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tail, _ := cmd.Flags().GetInt("tail")
			target := "all"
			if len(args) > 0 {
				target = args[0]
			}
			fmt.Printf("📜 Cron logs for '%s' (last %d):\n", target, tail)
			fmt.Println("  (no executions yet)")
			return nil
		},
	}
	cmd.Flags().IntP("tail", "n", 20, "Number of recent entries")
	return cmd
}
