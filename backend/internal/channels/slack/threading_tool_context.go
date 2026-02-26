package slack

import (
	"fmt"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// Slack 线程工具上下文 — 继承自 src/slack/threading-tool-context.ts (29L)

// ChannelThreadingContext 频道线程上下文（与 TS ChannelThreadingContext 对应）
type ChannelThreadingContext struct {
	To              string
	ChatType        string
	ThreadLabel     string
	MessageThreadId string
	ReplyToId       string
}

// ChannelThreadingToolContext 工具调用时的线程上下文
type ChannelThreadingToolContext struct {
	CurrentChannelID string
	CurrentThreadTs  string
	ReplyToMode      ReplyToMode
	HasRepliedRef    *bool
}

// BuildSlackThreadingToolContext 构建 Slack 工具调用的线程上下文。
func BuildSlackThreadingToolContext(cfg *types.OpenAcosmiConfig, accountID string, ctx ChannelThreadingContext, hasRepliedRef *bool) ChannelThreadingToolContext {
	account := ResolveSlackAccount(cfg, accountID)
	configuredReplyToMode := ResolveSlackReplyToMode(account, ctx.ChatType)

	effectiveReplyToMode := configuredReplyToMode
	if ctx.ThreadLabel != "" {
		effectiveReplyToMode = ReplyToModeAll
	}

	threadID := ctx.MessageThreadId
	if threadID == "" {
		threadID = ctx.ReplyToId
	}

	var currentChannelID string
	if strings.HasPrefix(ctx.To, "channel:") {
		currentChannelID = strings.TrimPrefix(ctx.To, "channel:")
	}

	var currentThreadTs string
	if threadID != "" {
		currentThreadTs = fmt.Sprintf("%v", threadID)
	}

	return ChannelThreadingToolContext{
		CurrentChannelID: currentChannelID,
		CurrentThreadTs:  currentThreadTs,
		ReplyToMode:      effectiveReplyToMode,
		HasRepliedRef:    hasRepliedRef,
	}
}
