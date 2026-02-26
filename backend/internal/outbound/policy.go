package outbound

import (
	"errors"
	"fmt"
	"strings"
)

// ---------- 跨上下文装饰 ----------

// CrossContextDecoration 跨上下文消息装饰。
type CrossContextDecoration struct {
	Prefix string        `json:"prefix"`
	Suffix string        `json:"suffix,omitempty"`
	Embeds []interface{} `json:"embeds,omitempty"`
}

// ChannelMessageActionName 频道消息动作名。
type ChannelMessageActionName string

const (
	ActionSend           ChannelMessageActionName = "send"
	ActionPoll           ChannelMessageActionName = "poll"
	ActionReply          ChannelMessageActionName = "reply"
	ActionSendWithEffect ChannelMessageActionName = "sendWithEffect"
	ActionSendAttachment ChannelMessageActionName = "sendAttachment"
	ActionThreadCreate   ChannelMessageActionName = "thread-create"
	ActionThreadReply    ChannelMessageActionName = "thread-reply"
	ActionSticker        ChannelMessageActionName = "sticker"
)

// ---------- 动作分类集合 ----------

// contextGuardedActions 需要上下文守卫检查的动作。
var contextGuardedActions = map[ChannelMessageActionName]struct{}{
	ActionSend:           {},
	ActionPoll:           {},
	ActionReply:          {},
	ActionSendWithEffect: {},
	ActionSendAttachment: {},
	ActionThreadCreate:   {},
	ActionThreadReply:    {},
	ActionSticker:        {},
}

// contextMarkerActions 需要跨上下文标记的动作。
var contextMarkerActions = map[ChannelMessageActionName]struct{}{
	ActionSend:           {},
	ActionPoll:           {},
	ActionReply:          {},
	ActionSendWithEffect: {},
	ActionSendAttachment: {},
	ActionThreadReply:    {},
	ActionSticker:        {},
}

// ---------- 跨上下文策略配置 ----------

// CrossContextConfig 跨上下文消息策略配置。
type CrossContextConfig struct {
	AllowCrossContextSend bool                   `json:"allowCrossContextSend,omitempty"`
	CrossContext          *CrossContextSubConfig `json:"crossContext,omitempty"`
}

// CrossContextSubConfig 跨上下文子配置。
type CrossContextSubConfig struct {
	AllowWithinProvider  *bool               `json:"allowWithinProvider,omitempty"`
	AllowAcrossProviders *bool               `json:"allowAcrossProviders,omitempty"`
	Marker               *CrossContextMarker `json:"marker,omitempty"`
}

// CrossContextMarker 跨上下文标记配置。
type CrossContextMarker struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Prefix  string `json:"prefix,omitempty"`
	Suffix  string `json:"suffix,omitempty"`
}

// ToolContext 工具上下文信息。
type ToolContext struct {
	CurrentChannelID           string `json:"currentChannelId,omitempty"`
	CurrentChannelProvider     string `json:"currentChannelProvider,omitempty"`
	SkipCrossContextDecoration bool   `json:"skipCrossContextDecoration,omitempty"`
}

// TargetNormalizer 目标规范化函数。
type TargetNormalizer func(channel, raw string) string

// ---------- 上下文守卫 ----------

// resolveContextGuardTarget 解析上下文守卫目标。
func resolveContextGuardTarget(action ChannelMessageActionName, params map[string]interface{}) string {
	if _, ok := contextGuardedActions[action]; !ok {
		return ""
	}

	if action == ActionThreadReply || action == ActionThreadCreate {
		if cid, ok := params["channelId"].(string); ok {
			return cid
		}
		if to, ok := params["to"].(string); ok {
			return to
		}
		return ""
	}

	if to, ok := params["to"].(string); ok {
		return to
	}
	if cid, ok := params["channelId"].(string); ok {
		return cid
	}
	return ""
}

// normalizeTarget 规范化目标地址。
func normalizeTarget(channel, raw string, normalizer TargetNormalizer) string {
	if normalizer != nil {
		if n := normalizer(channel, raw); n != "" {
			return n
		}
	}
	return strings.TrimSpace(strings.ToLower(raw))
}

// isCrossContextTarget 判断是否为跨上下文目标。
func isCrossContextTarget(channel, target string, toolCtx *ToolContext, normalizer TargetNormalizer) bool {
	if toolCtx == nil {
		return false
	}
	currentTarget := strings.TrimSpace(toolCtx.CurrentChannelID)
	if currentTarget == "" {
		return false
	}
	nTarget := normalizeTarget(channel, target, normalizer)
	nCurrent := normalizeTarget(channel, currentTarget, normalizer)
	if nTarget == "" || nCurrent == "" {
		return false
	}
	return nTarget != nCurrent
}

// EnforceCrossContextPolicy 执行跨上下文消息策略检查。
// 如果策略不允许当前操作, 返回 error。
func EnforceCrossContextPolicy(params EnforcePolicyParams) error {
	if params.ToolContext == nil {
		return nil
	}
	currentTarget := strings.TrimSpace(params.ToolContext.CurrentChannelID)
	if currentTarget == "" {
		return nil
	}
	if _, ok := contextGuardedActions[params.Action]; !ok {
		return nil
	}

	// 全局允许
	if params.Config != nil && params.Config.AllowCrossContextSend {
		return nil
	}

	currentProvider := params.ToolContext.CurrentChannelProvider

	allowWithinProvider := true
	allowAcrossProviders := false
	if params.Config != nil && params.Config.CrossContext != nil {
		if params.Config.CrossContext.AllowWithinProvider != nil {
			allowWithinProvider = *params.Config.CrossContext.AllowWithinProvider
		}
		if params.Config.CrossContext.AllowAcrossProviders != nil {
			allowAcrossProviders = *params.Config.CrossContext.AllowAcrossProviders
		}
	}

	// 跨提供商检查
	if currentProvider != "" && currentProvider != params.Channel {
		if !allowAcrossProviders {
			return fmt.Errorf(
				"cross-context messaging denied: action=%s target provider %q while bound to %q",
				params.Action, params.Channel, currentProvider,
			)
		}
		return nil
	}

	if allowWithinProvider {
		return nil
	}

	target := resolveContextGuardTarget(params.Action, params.Args)
	if target == "" {
		return nil
	}

	if !isCrossContextTarget(params.Channel, target, params.ToolContext, params.Normalizer) {
		return nil
	}

	return fmt.Errorf(
		"cross-context messaging denied: action=%s target=%q while bound to %q (channel=%s)",
		params.Action, target, currentTarget, params.Channel,
	)
}

// EnforcePolicyParams 策略执行参数。
type EnforcePolicyParams struct {
	Channel     string
	Action      ChannelMessageActionName
	Args        map[string]interface{}
	ToolContext *ToolContext
	Config      *CrossContextConfig
	Normalizer  TargetNormalizer
}

// ---------- 跨上下文标记 ----------

// ShouldApplyCrossContextMarker 是否应用跨上下文标记。
func ShouldApplyCrossContextMarker(action ChannelMessageActionName) bool {
	_, ok := contextMarkerActions[action]
	return ok
}

// ApplyCrossContextDecoration 应用跨上下文装饰到消息。
func ApplyCrossContextDecoration(message string, decoration CrossContextDecoration, preferEmbeds bool) (result string, embeds []interface{}, usedEmbeds bool) {
	if preferEmbeds && len(decoration.Embeds) > 0 {
		return message, decoration.Embeds, true
	}
	return decoration.Prefix + message + decoration.Suffix, nil, false
}

// BuildCrossContextDecoration 构建跨上下文装饰。
func BuildCrossContextDecoration(params BuildDecorationParams) (*CrossContextDecoration, error) {
	if params.ToolContext == nil || params.ToolContext.CurrentChannelID == "" {
		return nil, nil
	}
	if params.ToolContext.SkipCrossContextDecoration {
		return nil, nil
	}
	if !isCrossContextTarget(params.Channel, params.Target, params.ToolContext, params.Normalizer) {
		return nil, nil
	}

	markerEnabled := true
	prefixTemplate := "[from {channel}] "
	suffixTemplate := ""

	if params.Config != nil && params.Config.CrossContext != nil && params.Config.CrossContext.Marker != nil {
		m := params.Config.CrossContext.Marker
		if m.Enabled != nil && !*m.Enabled {
			return nil, nil
		}
		if m.Prefix != "" {
			prefixTemplate = m.Prefix
		}
		if m.Suffix != "" {
			suffixTemplate = m.Suffix
		}
	}
	_ = markerEnabled

	// 解析来源标签
	originLabel := params.ToolContext.CurrentChannelID
	if params.DisplayResolver != nil {
		if display := params.DisplayResolver(params.Channel, params.ToolContext.CurrentChannelID, params.AccountID); display != "" {
			originLabel = display
		}
	}

	prefix := strings.ReplaceAll(prefixTemplate, "{channel}", originLabel)
	suffix := strings.ReplaceAll(suffixTemplate, "{channel}", originLabel)

	return &CrossContextDecoration{
		Prefix: prefix,
		Suffix: suffix,
	}, nil
}

// BuildDecorationParams 构建装饰的参数。
type BuildDecorationParams struct {
	Channel         string
	Target          string
	ToolContext     *ToolContext
	AccountID       string
	Config          *CrossContextConfig
	Normalizer      TargetNormalizer
	DisplayResolver func(channel, targetID, accountID string) string
}

// ---------- 错误类型 ----------

// IsCrossContextDenied 检查错误是否为跨上下文消息拒绝。
func IsCrossContextDenied(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errCrossContextDenied) ||
		strings.Contains(err.Error(), "cross-context messaging denied")
}

var errCrossContextDenied = errors.New("cross-context messaging denied")
