package main

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/openacosmi/claw-acismi/internal/agents/exec"
	"github.com/openacosmi/claw-acismi/internal/cli"
	"github.com/openacosmi/claw-acismi/internal/config"
)

// 对应 TS src/commands/agent.ts + agent-via-gateway.ts

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "AI Agent management",
		Long:  "Run, send, and manage AI agents.",
	}

	cmd.AddCommand(
		newAgentRunCmd(),
		newAgentSendCmd(),
		newAgentListCmd(),
		newAgentAddCmd(),
		newAgentDeleteCmd(),
		newAgentIdentityCmd(),
	)

	return cmd
}

func newAgentRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an agent interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			message, _ := cmd.Flags().GetString("message")
			channel, _ := cmd.Flags().GetString("channel")
			provider, _ := cmd.Flags().GetString("provider")
			model, _ := cmd.Flags().GetString("model")

			if message == "" {
				return fmt.Errorf("--message 参数必填")
			}

			// 加载配置
			cfgLoader := config.NewConfigLoader()
			cfg, err := cfgLoader.LoadConfig()
			if err != nil {
				return fmt.Errorf("配置加载失败: %w", err)
			}

			// 确定 provider
			if provider == "" {
				provider = "openai" // 默认 provider
			}

			// 构建 CLI Runner 参数
			params := exec.CliRunnerParams{
				SessionID:    uuid.NewString(),
				Config:       cfg,
				Prompt:       message,
				Provider:     provider,
				Model:        model,
				TimeoutMs:    120000,
				RunID:        uuid.NewString(),
				WorkspaceDir: ".",
			}

			result, err := exec.RunCliAgent(params)
			if err != nil {
				return fmt.Errorf("Agent 运行失败: %w", err)
			}

			// 提取输出文本
			var outputText string
			if result != nil && len(result.Payloads) > 0 {
				outputText = result.Payloads[0].Text
			}

			// 输出结果
			if outputText != "" {
				cmd.Println(outputText)
			}

			// 如果指定了 channel，通过 Gateway RPC 发送
			if channel != "" && outputText != "" {
				_, rpcErr := cli.CallGatewayFromCLI("send", cli.GatewayRPCOpts{}, map[string]interface{}{
					"channel": channel,
					"text":    outputText,
				})
				if rpcErr != nil {
					cmd.PrintErrf("⚠️ 频道发送失败: %v\n", rpcErr)
				}
			}

			return nil
		},
	}
	cmd.Flags().String("to", "", "Target recipient (phone/channel)")
	cmd.Flags().String("message", "", "Initial message")
	cmd.Flags().Bool("deliver", false, "Deliver response via channel")
	cmd.Flags().String("channel", "", "Channel to use (whatsapp|telegram|...)")
	cmd.Flags().String("provider", "", "CLI backend provider (openai|anthropic|...)")
	cmd.Flags().String("model", "", "Model to use")
	return cmd
}

func newAgentSendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message through the agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			to, _ := cmd.Flags().GetString("to")
			message, _ := cmd.Flags().GetString("message")
			channel, _ := cmd.Flags().GetString("channel")

			if message == "" {
				return fmt.Errorf("--message 参数必填")
			}

			// 通过 Gateway RPC 委托
			params := map[string]interface{}{
				"text": message,
			}
			if to != "" {
				params["to"] = to
			}
			if channel != "" {
				params["channel"] = channel
			}

			result, err := cli.CallGatewayFromCLI("send", cli.GatewayRPCOpts{}, params)
			if err != nil {
				return fmt.Errorf("发送失败: %w", err)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag && result != nil {
				data, _ := json.MarshalIndent(result, "", "  ")
				cmd.Println(string(data))
			} else {
				cmd.Println("📨 消息已发送")
			}
			return nil
		},
	}
	cmd.Flags().String("to", "", "Target recipient")
	cmd.Flags().String("message", "", "Message content")
	cmd.Flags().String("channel", "", "Channel to use")
	return cmd
}

func newAgentListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")

			result, err := cli.CallGatewayFromCLI("agents.list", cli.GatewayRPCOpts{
				JSON: jsonFlag,
			}, nil)
			if err != nil {
				// Gateway 未运行时从配置直接读取
				cfgLoader := config.NewConfigLoader()
				cfg, loadErr := cfgLoader.LoadConfig()
				if loadErr != nil || cfg == nil || cfg.Agents == nil || len(cfg.Agents.List) == 0 {
					cmd.Println("📋 无已配置 Agent（Gateway 未运行）")
					return nil
				}
				for _, agent := range cfg.Agents.List {
					id := agent.ID
					if id == "" {
						id = "(未命名)"
					}
					cmd.Printf("  • %s\n", id)
				}
				return nil
			}

			if jsonFlag && result != nil {
				data, _ := json.MarshalIndent(result, "", "  ")
				cmd.Println(string(data))
			} else {
				cmd.Printf("📋 Agents: %v\n", result)
			}
			return nil
		},
	}
	cmd.Flags().Bool("bindings", false, "Show channel bindings")
	return cmd
}

func newAgentAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("➕ Agent add not yet implemented")
			return nil
		},
	}
}

func newAgentDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("🗑️ Agent delete not yet implemented")
			return nil
		},
	}
	cmd.Flags().String("name", "", "Agent name")
	return cmd
}

func newAgentIdentityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "identity",
		Short: "Manage agent identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("🪪 Agent identity not yet implemented")
			return nil
		},
	}
}
