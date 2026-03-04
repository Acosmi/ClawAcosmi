// runner.go — 链接理解运行器。
//
// TS 对照: link-understanding/runner.ts (151L)
//
// 从消息中提取链接，通过 CLI 工具运行链接理解，返回理解结果。
package linkparse

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/internal/media/understanding"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// LinkUnderstandingResult 链接理解结果。
// TS 对照: runner.ts LinkUnderstandingResult
type LinkUnderstandingResult struct {
	URLs    []string
	Outputs []string
}

// CLIOutputMaxBuffer CLI 输出最大缓冲区大小。
const CLIOutputMaxBuffer = 1024 * 1024 // 1MB

// ResolveScopeDecision 解析链接理解作用域策略。
// TS 对照: runner.ts resolveScopeDecision()
func ResolveScopeDecision(config *types.LinkToolsConfig, ctx *autoreply.MsgContext) string {
	if config == nil || config.Scope == nil {
		return "allow"
	}

	channel := ctx.Surface
	if channel == "" {
		channel = ctx.Provider
	}
	chatType := normalizeChatType(ctx.ChatType)

	result := understanding.ResolveMediaUnderstandingScope(understanding.ScopeParams{
		Channel:  channel,
		ChatType: chatType,
	})
	if !result.Allowed {
		return "deny"
	}
	return "allow"
}

// normalizeChatType 规范化聊天类型。
func normalizeChatType(chatType string) string {
	switch strings.ToLower(chatType) {
	case "dm", "direct", "private":
		return "dm"
	case "group", "channel":
		return chatType
	default:
		return "dm"
	}
}

// ResolveTimeoutMsFromConfig 从配置解析超时时间（毫秒）。
// TS 对照: runner.ts resolveTimeoutMsFromConfig()
func ResolveTimeoutMsFromConfig(config *types.LinkToolsConfig, entry types.LinkModelConfig) int {
	if entry.TimeoutSeconds != nil {
		return *entry.TimeoutSeconds * 1000
	}
	if config != nil && config.TimeoutSeconds != nil {
		return *config.TimeoutSeconds * 1000
	}
	return DefaultLinkTimeoutSeconds * 1000
}

// RunCliEntry 运行单个 CLI 链接理解条目。
// TS 对照: runner.ts runCliEntry()
func RunCliEntry(entry types.LinkModelConfig, ctx *autoreply.MsgContext, url string, config *types.LinkToolsConfig, verbose bool) (string, error) {
	entryType := entry.Type
	if entryType == "" {
		entryType = "cli"
	}
	if entryType != "cli" {
		return "", nil
	}

	command := strings.TrimSpace(entry.Command)
	if command == "" {
		return "", nil
	}

	timeoutMs := ResolveTimeoutMsFromConfig(config, entry)

	// 构建参数，替换模板变量 {{LinkUrl}}
	args := make([]string, 0, len(entry.Args))
	for _, arg := range entry.Args {
		replaced := strings.ReplaceAll(arg, "{{LinkUrl}}", url)
		args = append(args, replaced)
	}

	if verbose {
		log.Printf("[linkparse] CLI: %s %s", command, strings.Join(args, " "))
	}

	// 执行命令
	execCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(execCtx, command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("CLI 执行失败: %w (stderr: %s)", err, stderr.String())
	}

	trimmed := strings.TrimSpace(stdout.String())
	if trimmed == "" {
		return "", nil
	}
	// 截断过长输出
	if len(trimmed) > CLIOutputMaxBuffer {
		trimmed = trimmed[:CLIOutputMaxBuffer]
	}
	return trimmed, nil
}

// RunLinkEntries 按顺序尝试多个链接理解条目，返回第一个成功结果。
// TS 对照: runner.ts runLinkEntries()
func RunLinkEntries(entries []types.LinkModelConfig, ctx *autoreply.MsgContext, url string, config *types.LinkToolsConfig, verbose bool) string {
	var lastErr error
	for _, entry := range entries {
		output, err := RunCliEntry(entry, ctx, url, config, verbose)
		if err != nil {
			lastErr = err
			if verbose {
				log.Printf("[linkparse] 链接理解失败 %s: %v", url, err)
			}
			continue
		}
		if output != "" {
			return output
		}
	}
	if lastErr != nil && verbose {
		log.Printf("[linkparse] 链接理解所有条目已耗尽: %s", url)
	}
	return ""
}

// RunLinkUnderstandingParams 链接理解运行参数。
type RunLinkUnderstandingParams struct {
	ToolsConfig *types.ToolsConfig
	Ctx         *autoreply.MsgContext
	Message     string
	Verbose     bool
}

// RunLinkUnderstanding 执行链接理解流程。
// TS 对照: runner.ts runLinkUnderstanding()
func RunLinkUnderstanding(params RunLinkUnderstandingParams) LinkUnderstandingResult {
	empty := LinkUnderstandingResult{URLs: nil, Outputs: nil}

	config := params.ToolsConfig
	if config == nil || config.Links == nil {
		return empty
	}
	linkConfig := config.Links
	if linkConfig.Enabled != nil && !*linkConfig.Enabled {
		return empty
	}

	// 检查作用域
	scopeDecision := ResolveScopeDecision(linkConfig, params.Ctx)
	if scopeDecision == "deny" {
		if params.Verbose {
			log.Printf("[linkparse] 链接理解被作用域策略禁止")
		}
		return empty
	}

	// 提取消息文本
	message := params.Message
	if message == "" {
		message = params.Ctx.CommandBody
	}
	if message == "" {
		message = params.Ctx.RawBody
	}
	if message == "" {
		message = params.Ctx.Body
	}

	// 提取链接
	links := ExtractLinksFromMessage(message, linkConfig.MaxLinks)
	if len(links) == 0 {
		return empty
	}

	entries := linkConfig.Models
	if len(entries) == 0 {
		return LinkUnderstandingResult{URLs: links, Outputs: nil}
	}

	// 对每个链接运行理解
	var outputs []string
	for _, url := range links {
		output := RunLinkEntries(entries, params.Ctx, url, linkConfig, params.Verbose)
		if output != "" {
			outputs = append(outputs, output)
		}
	}

	return LinkUnderstandingResult{URLs: links, Outputs: outputs}
}
