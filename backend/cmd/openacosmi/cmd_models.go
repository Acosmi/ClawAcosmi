package main

import "github.com/spf13/cobra"

// 对应 TS src/cli/models-cli.ts + src/commands/models/

func newModelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Model configuration",
		Long:  "List, set, and manage AI model configurations.",
	}

	cmd.AddCommand(
		newModelsListCmd(),
		newModelsSetCmd(),
		newModelsGetCmd(),
	)

	return cmd
}

func newModelsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available models",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("📋 Models list not yet implemented")
			return nil
		},
	}
}

func newModelsSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the active model",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("⚙️ Models set not yet implemented")
			return nil
		},
	}
	cmd.Flags().String("provider", "", "Provider name")
	cmd.Flags().String("model", "", "Model name")
	return cmd
}

func newModelsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show current model",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("🔍 Models get not yet implemented")
			return nil
		},
	}
}
