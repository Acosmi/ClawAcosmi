// view_status.go — TUI 状态栏组件
//
// 对齐 TS: tui-status-summary.ts(89L) + tui-waiting.ts(52L)
// 连接状态 + token 使用 + 活动状态 + 等待动画。
//
// W3 产出文件 #4。
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// ---------- 等待短语（对标 TS tui-waiting.ts）----------

// defaultWaitingPhrases 等待提示词列表。
var defaultWaitingPhrases = []string{
	"flibbertigibbeting",
	"kerfuffling",
	"dillydallying",
	"twiddling thumbs",
	"noodling",
	"bamboozling",
	"moseying",
	"hobnobbing",
	"pondering",
	"conjuring",
}

// PickWaitingPhrase 按 tick 轮换选择等待短语。
// TS 参考: tui-waiting.ts pickWaitingPhrase
func PickWaitingPhrase(tick int) string {
	if len(defaultWaitingPhrases) == 0 {
		return "waiting"
	}
	idx := (tick / 10) % len(defaultWaitingPhrases)
	return defaultWaitingPhrases[idx]
}

// ShimmerText 生成闪烁文本效果。
// TS 参考: tui-waiting.ts shimmerText
func ShimmerText(text string, tick int) string {
	runes := []rune(text)
	width := 6
	pos := tick % (len(runes) + width)
	start := pos - width
	if start < 0 {
		start = 0
	}
	end := pos
	if end >= len(runes) {
		end = len(runes) - 1
	}

	hi := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA"))
	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	var out strings.Builder
	for i, ch := range runes {
		if i >= start && i <= end {
			out.WriteString(hi.Render(string(ch)))
		} else {
			out.WriteString(dim.Render(string(ch)))
		}
	}
	return out.String()
}

// BuildWaitingStatusMessage 构建等待状态消息。
// TS 参考: tui-waiting.ts buildWaitingStatusMessage
func BuildWaitingStatusMessage(tick int, elapsed string, connStatus string) string {
	phrase := PickWaitingPhrase(tick)
	cute := ShimmerText(phrase+"…", tick)
	return fmt.Sprintf("%s • %s | %s", cute, elapsed, connStatus)
}

// ---------- 状态栏 ----------

// StatusBar TUI 状态栏组件。
type StatusBar struct {
	// 连接状态
	ConnectionStatus string // "connected", "disconnected", "reconnecting"
	IsConnected      bool

	// 活动状态
	ActivityStatus string // "idle", "streaming", "running", "error", "aborted"

	// Session 信息
	Model         string
	ModelProvider string
	TotalTokens   *int
	ContextTokens *int

	// 等待动画
	Tick      int
	StartedAt time.Time
}

// NewStatusBar 创建状态栏。
func NewStatusBar() *StatusBar {
	return &StatusBar{
		ConnectionStatus: "connecting",
		ActivityStatus:   "idle",
		StartedAt:        time.Now(),
	}
}

// connectionIcon 返回连接状态图标。
func (sb *StatusBar) connectionIcon() string {
	if sb.IsConnected {
		return "🟢"
	}
	if sb.ConnectionStatus == "reconnecting" {
		return "🟡"
	}
	return "🔴"
}

// View 渲染状态栏。
// 水平布局：左=连接状态 中=model 右=tokens
func (sb *StatusBar) View(width int) string {
	// 左侧: 连接状态
	icon := sb.connectionIcon()
	leftParts := []string{icon}

	activity := sb.ActivityStatus
	if activity == "" || activity == "idle" {
		leftParts = append(leftParts, sb.ConnectionStatus)
	} else if activity == "streaming" || activity == "running" {
		// 等待动画
		elapsed := formatElapsed(time.Since(sb.StartedAt))
		waitMsg := BuildWaitingStatusMessage(sb.Tick, elapsed, sb.ConnectionStatus)
		leftParts = append(leftParts, waitMsg)
	} else {
		leftParts = append(leftParts, fmt.Sprintf("%s | %s", sb.ConnectionStatus, activity))
	}
	left := strings.Join(leftParts, " ")

	// 中间: model
	modelLabel := "unknown"
	if sb.Model != "" {
		if sb.ModelProvider != "" {
			modelLabel = sb.ModelProvider + "/" + sb.Model
		} else {
			modelLabel = sb.Model
		}
	}
	center := MutedStyle.Render(modelLabel)

	// 右侧: tokens
	right := ""
	if sb.TotalTokens != nil || sb.ContextTokens != nil {
		right = FormatTokens(sb.TotalTokens, sb.ContextTokens)
	}

	// 布局
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	centerWidth := width - leftWidth - rightWidth - 4
	if centerWidth < 0 {
		centerWidth = 0
	}

	gap := ""
	if centerWidth > 0 {
		centerRendered := lipgloss.PlaceHorizontal(centerWidth, lipgloss.Center, center)
		gap = centerRendered
	}

	line := left + gap + right
	return MutedStyle.Width(width).Render(line)
}

// ---------- 格式化辅助 ----------

// formatElapsed 格式化经过时间。
func formatElapsed(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	mins := secs / 60
	remaining := secs % 60
	return fmt.Sprintf("%dm%ds", mins, remaining)
}

// FormatStatusSummary 格式化 gateway 状态摘要。
// TS 参考: tui-status-summary.ts formatStatusSummary（完整 6 段）
func FormatStatusSummary(summary map[string]interface{}) []string {
	var lines []string
	lines = append(lines, "Gateway status")

	// 1. Link channel
	if linkChannel, ok := summary["linkChannel"].(map[string]interface{}); ok {
		linkLabel := "Link channel"
		if l, ok := linkChannel["label"].(string); ok && l != "" {
			linkLabel = l
		}
		linked, _ := linkChannel["linked"].(bool)
		linkStatus := "not linked"
		authAge := ""
		if linked {
			linkStatus = "linked"
			if ageMs, ok := linkChannel["authAgeMs"].(float64); ok {
				authAge = fmt.Sprintf(" (last refreshed %s)", formatTimeAgo(int64(ageMs)))
			}
		}
		lines = append(lines, fmt.Sprintf("%s: %s%s", linkLabel, linkStatus, authAge))
	} else {
		lines = append(lines, "Link channel: unknown")
	}

	// 2. Provider summary
	if providers, ok := summary["providerSummary"].([]interface{}); ok && len(providers) > 0 {
		lines = append(lines, "")
		lines = append(lines, "System:")
		for _, p := range providers {
			if s, ok := p.(string); ok {
				lines = append(lines, "  "+s)
			}
		}
	}

	// 3. Heartbeat agents
	if heartbeat, ok := summary["heartbeat"].(map[string]interface{}); ok {
		if agents, ok := heartbeat["agents"].([]interface{}); ok && len(agents) > 0 {
			var parts []string
			for _, a := range agents {
				agentMap, ok := a.(map[string]interface{})
				if !ok {
					continue
				}
				agentID := "unknown"
				if id, ok := agentMap["agentId"].(string); ok && id != "" {
					agentID = id
				}
				enabled, _ := agentMap["enabled"].(bool)
				if !enabled {
					parts = append(parts, fmt.Sprintf("disabled (%s)", agentID))
					continue
				}
				every := "unknown"
				if e, ok := agentMap["every"].(string); ok && e != "" {
					every = e
				}
				parts = append(parts, fmt.Sprintf("%s (%s)", every, agentID))
			}
			if len(parts) > 0 {
				lines = append(lines, "")
				lines = append(lines, fmt.Sprintf("Heartbeat: %s", strings.Join(parts, ", ")))
			}
		}
	}

	// 4-6. Sessions block
	if sessions, ok := summary["sessions"].(map[string]interface{}); ok {
		// Session store paths
		if paths, ok := sessions["paths"].([]interface{}); ok {
			if len(paths) == 1 {
				if p, ok := paths[0].(string); ok {
					lines = append(lines, fmt.Sprintf("Session store: %s", p))
				}
			} else if len(paths) > 1 {
				lines = append(lines, fmt.Sprintf("Session stores: %d", len(paths)))
			}
		}

		// Default model
		if defaults, ok := sessions["defaults"].(map[string]interface{}); ok {
			model := "unknown"
			if m, ok := defaults["model"].(string); ok && m != "" {
				model = m
			}
			ctxStr := ""
			if ctx, ok := defaults["contextTokens"].(float64); ok {
				ctxStr = fmt.Sprintf(" (%s ctx)", autoreply.FormatTokenCount(int(ctx)))
			}
			lines = append(lines, fmt.Sprintf("Default model: %s%s", model, ctxStr))
		}

		// Active sessions count
		if count, ok := sessions["count"].(float64); ok {
			lines = append(lines, fmt.Sprintf("Active sessions: %d", int(count)))
		}

		// Recent sessions
		if recent, ok := sessions["recent"].([]interface{}); ok && len(recent) > 0 {
			lines = append(lines, "Recent sessions:")
			for _, entry := range recent {
				entryMap, ok := entry.(map[string]interface{})
				if !ok {
					continue
				}
				key, _ := entryMap["key"].(string)
				kind, _ := entryMap["kind"].(string)

				// Age
				ageLabel := "no activity"
				if age, ok := entryMap["age"].(float64); ok {
					ageLabel = formatTimeAgo(int64(age))
				}

				// Model
				model := "unknown"
				if m, ok := entryMap["model"].(string); ok && m != "" {
					model = m
				}

				// Usage
				var total, context, remaining, percent *int
				if v, ok := entryMap["totalTokens"].(float64); ok {
					iv := int(v)
					total = &iv
				}
				if v, ok := entryMap["contextTokens"].(float64); ok {
					iv := int(v)
					context = &iv
				}
				if v, ok := entryMap["remainingTokens"].(float64); ok {
					iv := int(v)
					remaining = &iv
				}
				if v, ok := entryMap["percentUsed"].(float64); ok {
					iv := int(v)
					percent = &iv
				}
				usage := FormatContextUsageLine(total, context, remaining, percent)

				// Flags
				flagStr := ""
				if flags, ok := entryMap["flags"].([]interface{}); ok && len(flags) > 0 {
					var flagParts []string
					for _, f := range flags {
						if s, ok := f.(string); ok {
							flagParts = append(flagParts, s)
						}
					}
					if len(flagParts) > 0 {
						flagStr = fmt.Sprintf(" | flags: %s", strings.Join(flagParts, ", "))
					}
				}

				kindStr := ""
				if kind != "" {
					kindStr = fmt.Sprintf(" [%s]", kind)
				}
				lines = append(lines, fmt.Sprintf("- %s%s | %s | model %s | %s%s",
					key, kindStr, ageLabel, model, usage, flagStr))
			}
		}
	}

	// 7. Queued system events
	if queued, ok := summary["queuedSystemEvents"].([]interface{}); ok && len(queued) > 0 {
		var preview []string
		limit := 3
		if len(queued) < limit {
			limit = len(queued)
		}
		for _, q := range queued[:limit] {
			if s, ok := q.(string); ok {
				preview = append(preview, s)
			}
		}
		lines = append(lines, fmt.Sprintf("Queued system events (%d): %s",
			len(queued), strings.Join(preview, " | ")))
	}

	return lines
}

// formatTimeAgo 格式化相对时间（毫秒 → 可读字符串）。
// TS 参考: infra/format-time/format-relative.ts formatTimeAgo
func formatTimeAgo(ms int64) string {
	secs := ms / 1000
	if secs < 5 {
		return "just now"
	}
	if secs < 60 {
		return fmt.Sprintf("%ds ago", secs)
	}
	mins := secs / 60
	if mins < 60 {
		return fmt.Sprintf("%dm ago", mins)
	}
	hours := mins / 60
	if hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%dd ago", days)
}
