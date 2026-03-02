package main

// cmd_memory.go — Agent 记忆管理 CLI
// 对应 TS: src/commands/memory-cli.ts
//
// 子命令:
//   openacosmi memory status        — 查看记忆存储状态
//   openacosmi memory reindex       — 离线手动重新索引
//   openacosmi memory search <query> — 执行向量搜索测试

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openacosmi/claw-acismi/internal/cli"
)

func newMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Agent memory management",
		Long:  "Manage agent memory — status, reindex, and search operations.",
	}
	cmd.AddCommand(
		newMemoryStatusCmd(),
		newMemoryReindexCmd(),
		newMemorySearchCmd(),
		newMemoryClearCmd(),
	)
	return cmd
}

// ---------- memory status ----------

func newMemoryStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show memory store status",
		Long:  "Display the current state of the agent memory store, including vector count, index size, and last sync time.",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")
			timeout, _ := cmd.Flags().GetInt("timeout")

			opts := cli.GatewayRPCOpts{
				JSON:      jsonFlag,
				TimeoutMs: timeout,
			}

			result, err := cli.CallGatewayFromCLI("memory.status", opts, nil)
			if err != nil {
				if jsonFlag {
					data, _ := json.MarshalIndent(map[string]interface{}{
						"status": "error",
						"error":  err.Error(),
					}, "", "  ")
					cmd.Println(string(data))
				} else {
					cmd.Printf("🧠 Memory status: 无法连接 Gateway (%v)\n", err)
				}
				return nil
			}

			if jsonFlag {
				data, _ := json.MarshalIndent(result, "", "  ")
				cmd.Println(string(data))
			} else {
				cmd.Println("🧠 Memory 状态")
				cmd.Println()
				if m, ok := result.(map[string]interface{}); ok {
					if mode, ok := m["mode"].(string); ok {
						cmd.Printf("  📦 存储模式: %s\n", mode)
					}
					if count, ok := m["vectorCount"].(float64); ok {
						cmd.Printf("  📊 向量数量: %d\n", int(count))
					}
					if lastSync, ok := m["lastSync"].(string); ok && lastSync != "" {
						cmd.Printf("  🕐 最后同步: %s\n", lastSync)
					}
					if indexSize, ok := m["indexSizeBytes"].(float64); ok {
						cmd.Printf("  💾 索引大小: %s\n", formatBytes(int64(indexSize)))
					}
				} else {
					cmd.Printf("  ✅ 状态: %v\n", result)
				}
			}
			return nil
		},
	}
	cmd.Flags().Int("timeout", 10000, "Timeout in milliseconds")
	return cmd
}

// ---------- memory reindex ----------

func newMemoryReindexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Trigger offline re-indexing of memory store",
		Long:  "Force a full re-index of the agent memory store. This rebuilds the vector index from the raw memory entries.",
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, _ := cmd.Flags().GetInt("timeout")
			force, _ := cmd.Flags().GetBool("force")

			opts := cli.GatewayRPCOpts{
				TimeoutMs: timeout,
			}

			params := map[string]interface{}{
				"force": force,
			}

			cmd.Println("🔄 正在触发内存重新索引...")
			result, err := cli.CallGatewayFromCLI("memory.reindex", opts, params)
			if err != nil {
				cmd.Printf("❌ 重新索引失败: %v\n", err)
				return nil
			}

			if m, ok := result.(map[string]interface{}); ok {
				if indexed, ok := m["indexed"].(float64); ok {
					cmd.Printf("✅ 重新索引完成: %d 条记录已处理\n", int(indexed))
				} else {
					cmd.Println("✅ 重新索引完成")
				}
			} else {
				cmd.Println("✅ 重新索引完成")
			}
			return nil
		},
	}
	cmd.Flags().Int("timeout", 60000, "Timeout in milliseconds (reindex can be slow)")
	cmd.Flags().Bool("force", false, "Force full reindex even if index is up-to-date")
	return cmd
}

// ---------- memory search ----------

func newMemorySearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search agent memory",
		Long:  "Execute a vector similarity search against the agent memory store. Useful for testing and debugging memory retrieval.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			jsonFlag, _ := cmd.Flags().GetBool("json")
			timeout, _ := cmd.Flags().GetInt("timeout")
			limit, _ := cmd.Flags().GetInt("limit")
			agentID, _ := cmd.Flags().GetString("agent")

			opts := cli.GatewayRPCOpts{
				JSON:      jsonFlag,
				TimeoutMs: timeout,
			}

			params := map[string]interface{}{
				"query": query,
				"limit": limit,
			}
			if agentID != "" {
				params["agentId"] = agentID
			}

			result, err := cli.CallGatewayFromCLI("memory.search", opts, params)
			if err != nil {
				if jsonFlag {
					data, _ := json.MarshalIndent(map[string]interface{}{
						"error": err.Error(),
					}, "", "  ")
					cmd.Println(string(data))
				} else {
					cmd.Printf("❌ 搜索失败: %v\n", err)
				}
				return nil
			}

			if jsonFlag {
				data, _ := json.MarshalIndent(result, "", "  ")
				cmd.Println(string(data))
			} else {
				cmd.Printf("🔍 搜索结果: \"%s\"\n\n", query)
				if m, ok := result.(map[string]interface{}); ok {
					if results, ok := m["results"].([]interface{}); ok {
						if len(results) == 0 {
							cmd.Println("  (无结果)")
						}
						for i, r := range results {
							if entry, ok := r.(map[string]interface{}); ok {
								score, _ := entry["score"].(float64)
								text, _ := entry["text"].(string)
								// 截断过长文本
								if len(text) > 200 {
									text = text[:200] + "..."
								}
								cmd.Printf("  %d. [%.3f] %s\n", i+1, score, text)
							}
						}
					}
				} else {
					cmd.Printf("  %v\n", result)
				}
			}
			return nil
		},
	}
	cmd.Flags().Int("limit", 10, "Maximum number of results")
	cmd.Flags().Int("timeout", 10000, "Timeout in milliseconds")
	cmd.Flags().String("agent", "", "Agent ID to search within")
	return cmd
}

// ---------- memory clear ----------

func newMemoryClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear agent memory",
		Long:  "Clear all agent memory entries. This action is irreversible.",
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("confirm")
			if !confirm {
				cmd.Println("⚠️  此操作将清除所有记忆数据且不可恢复")
				cmd.Println("   使用 --confirm 确认执行")
				return nil
			}

			timeout, _ := cmd.Flags().GetInt("timeout")
			opts := cli.GatewayRPCOpts{
				TimeoutMs: timeout,
			}

			result, err := cli.CallGatewayFromCLI("memory.clear", opts, nil)
			if err != nil {
				cmd.Printf("❌ 清除失败: %v\n", err)
				return nil
			}

			if m, ok := result.(map[string]interface{}); ok {
				if cleared, ok := m["cleared"].(float64); ok {
					cmd.Printf("🧹 已清除 %d 条记忆记录\n", int(cleared))
					return nil
				}
			}
			cmd.Println("🧹 记忆已清除")
			return nil
		},
	}
	cmd.Flags().Bool("confirm", false, "Confirm clear (required)")
	cmd.Flags().Int("timeout", 10000, "Timeout in milliseconds")
	return cmd
}

// ---------- 辅助函数 ----------

func formatBytes(bytes int64) string {
	const (
		kB = 1024
		mB = 1024 * kB
		gB = 1024 * mB
	)
	switch {
	case bytes >= gB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gB))
	case bytes >= mB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mB))
	case bytes >= kB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
