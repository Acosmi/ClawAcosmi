package signal

import (
	"context"
	"fmt"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/utils"
)

// Signal 反应发送/移除 — 继承自 src/signal/send-reactions.ts (216L)

// SignalReactionOpts 反应操作选项
type SignalReactionOpts struct {
	BaseURL   string
	Account   string
	AccountID string
}

// normalizeSignalUUID 规范化 UUID 格式
func normalizeSignalUUID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "uuid:") {
		return strings.TrimSpace(lower[len("uuid:"):])
	}
	return lower
}

// normalizeSignalRecipient 规范化 Signal 收件人
func normalizeSignalRecipient(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "signal:") {
		trimmed = strings.TrimSpace(trimmed[len("signal:"):])
	}
	// 若是电话号码格式，规范化为 E.164
	if len(trimmed) > 0 && (trimmed[0] == '+' || (trimmed[0] >= '0' && trimmed[0] <= '9')) {
		return utils.NormalizeE164(trimmed)
	}
	return trimmed
}

// resolveReactionTargetAuthor 解析反应目标作者
// 对于群组消息，需要解析 targetAuthor; 对于 DM 使用 recipient
func resolveReactionTargetAuthor(targetAuthor, targetAuthorUUID, recipient string) string {
	if uuid := normalizeSignalUUID(targetAuthorUUID); uuid != "" {
		return uuid
	}
	if author := normalizeSignalRecipient(targetAuthor); author != "" {
		return author
	}
	return normalizeSignalRecipient(recipient)
}

// SendReactionSignal 发送 Signal 反应
func SendReactionSignal(ctx context.Context, to string, emoji string, timestamp int64, opts SignalReactionOpts) error {
	target := ParseTarget(to)
	if target == nil {
		return fmt.Errorf("invalid signal target: %q", to)
	}

	params := buildTargetParams(target)
	params["emoji"] = emoji
	params["targetTimestamp"] = timestamp
	// targetAuthor 在 send 时默认使用 account 自身
	if opts.Account != "" {
		params["targetAuthor"] = opts.Account
	}

	_, err := SignalRpcRequest(ctx, opts.BaseURL, "sendReaction", params, opts.Account)
	if err != nil {
		return fmt.Errorf("send reaction: %w", err)
	}
	return nil
}

// SendReactionToTargetSignal 发送反应（完整参数版本，指定 targetAuthor）
func SendReactionToTargetSignal(ctx context.Context, to string, emoji string, timestamp int64, targetAuthor string, opts SignalReactionOpts) error {
	target := ParseTarget(to)
	if target == nil {
		return fmt.Errorf("invalid signal target: %q", to)
	}

	params := buildTargetParams(target)
	params["emoji"] = emoji
	params["targetTimestamp"] = timestamp
	if targetAuthor != "" {
		params["targetAuthor"] = resolveReactionTargetAuthor(targetAuthor, "", "")
	} else if opts.Account != "" {
		params["targetAuthor"] = opts.Account
	}

	_, err := SignalRpcRequest(ctx, opts.BaseURL, "sendReaction", params, opts.Account)
	if err != nil {
		return fmt.Errorf("send reaction: %w", err)
	}
	return nil
}

// RemoveReactionSignal 移除 Signal 反应
func RemoveReactionSignal(ctx context.Context, to string, emoji string, timestamp int64, opts SignalReactionOpts) error {
	target := ParseTarget(to)
	if target == nil {
		return fmt.Errorf("invalid signal target: %q", to)
	}

	params := buildTargetParams(target)
	params["emoji"] = emoji
	params["targetTimestamp"] = timestamp
	params["remove"] = true
	if opts.Account != "" {
		params["targetAuthor"] = opts.Account
	}

	_, err := SignalRpcRequest(ctx, opts.BaseURL, "sendReaction", params, opts.Account)
	if err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}
	return nil
}

// ResolveSignalReactionTargets 解析反应消息的目标列表
// 从 reaction 中提取 targetAuthor 信息
type SignalReactionTarget struct {
	Kind    string // "phone"|"uuid"
	ID      string
	Display string
}

// ResolveReactionTargets 从 SignalReactionMessage 中解析目标列表
func ResolveReactionTargets(targetAuthor, targetAuthorUUID string) []SignalReactionTarget {
	var targets []SignalReactionTarget
	if uuid := strings.TrimSpace(targetAuthorUUID); uuid != "" {
		targets = append(targets, SignalReactionTarget{
			Kind:    "uuid",
			ID:      uuid,
			Display: "uuid:" + uuid,
		})
	}
	if author := strings.TrimSpace(targetAuthor); author != "" {
		e164 := utils.NormalizeE164(author)
		if e164 != "" {
			targets = append(targets, SignalReactionTarget{
				Kind:    "phone",
				ID:      e164,
				Display: e164,
			})
		}
	}
	return targets
}

// IsSignalReactionMessage 判断是否为有效的反应消息
func IsSignalReactionMessage(emoji string, hasTargetTimestamp bool) bool {
	return strings.TrimSpace(emoji) != "" && hasTargetTimestamp
}

// ShouldEmitSignalReactionNotification 判断是否应该发出反应通知
func ShouldEmitSignalReactionNotification(
	mode string,
	account string,
	targets []SignalReactionTarget,
	sender *SignalSender,
	allowlist []string,
) bool {
	if mode == "" || mode == "off" {
		return false
	}
	if mode == "all" {
		return true
	}
	if mode == "own" {
		for _, t := range targets {
			if t.Kind == "phone" && account != "" {
				if t.ID == utils.NormalizeE164(account) {
					return true
				}
			}
		}
		return false
	}
	if mode == "allowlist" {
		if sender == nil {
			return false
		}
		return IsSignalSenderAllowed(sender, allowlist)
	}
	return false
}

// BuildSignalReactionSystemEventText 构建反应系统事件文本
func BuildSignalReactionSystemEventText(emojiLabel, actorLabel, messageID string, targetLabel, groupLabel string) string {
	parts := []string{
		fmt.Sprintf("%s reacted %s", actorLabel, emojiLabel),
		fmt.Sprintf("to message %s", messageID),
	}
	if targetLabel != "" {
		parts = append(parts, fmt.Sprintf("by %s", targetLabel))
	}
	if groupLabel != "" {
		parts = append(parts, fmt.Sprintf("in %s", groupLabel))
	}
	return strings.Join(parts, " ")
}
