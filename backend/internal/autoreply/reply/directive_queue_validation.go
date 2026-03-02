package reply

import (
	"fmt"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// TS 对照: auto-reply/reply/directive-handling.queue-validation.ts (79L)

// MaybeHandleQueueDirectiveParams 队列指令验证参数。
type MaybeHandleQueueDirectiveParams struct {
	Directives   InlineDirectives
	Cfg          *types.OpenAcosmiConfig
	Channel      string
	SessionEntry *SessionEntry
}

// MaybeHandleQueueDirective 处理队列指令验证。
// 若 directives 不含队列指令 → 返回 nil（不是队列指令，继续常规流程）。
// 若仅 /queue 无参数 → 展示当前队列设置 status。
// 若有非法参数 → 返回错误信息。
// 若参数合法 → 返回 nil（由持久化层处理）。
// TS 对照: directive-handling.queue-validation.ts maybeHandleQueueDirective (L8-78)
func MaybeHandleQueueDirective(params MaybeHandleQueueDirectiveParams) *autoreply.ReplyPayload {
	directives := params.Directives
	if !directives.HasQueueDirective {
		return nil
	}

	// status 路径：无 mode、无 reset、无 options、无 raw 参数 → 展示当前设置
	wantsStatus := directives.QueueMode == "" &&
		!directives.QueueReset &&
		!directives.HasQueueOptions &&
		directives.RawQueueMode == "" &&
		directives.RawDebounce == "" &&
		directives.RawCap == "" &&
		directives.RawDrop == ""

	if wantsStatus {
		settings := ResolveQueueSettings(ResolveQueueSettingsParams{
			Cfg:          params.Cfg,
			Channel:      params.Channel,
			SessionEntry: params.SessionEntry,
		})

		debounceLabel := "default"
		if settings.DebounceMs != nil {
			debounceLabel = fmt.Sprintf("%dms", *settings.DebounceMs)
		}
		capLabel := "default"
		if settings.Cap != nil {
			capLabel = fmt.Sprintf("%d", *settings.Cap)
		}
		dropLabel := "default"
		if settings.DropPolicy != "" {
			dropLabel = string(settings.DropPolicy)
		}

		text := fmt.Sprintf(
			"Current queue settings: mode=%s, debounce=%s, cap=%s, drop=%s.",
			settings.Mode, debounceLabel, capLabel, dropLabel,
		)
		return &autoreply.ReplyPayload{
			Text: WithOptions(
				text,
				"modes steer, followup, collect, steer+backlog, interrupt; debounce:<ms|s|m>, cap:<n>, drop:old|new|summarize",
			),
		}
	}

	// 验证路径：检查各 raw 参数的合法性
	queueModeInvalid := directives.QueueMode == "" &&
		!directives.QueueReset &&
		directives.RawQueueMode != ""
	queueDebounceInvalid := directives.RawDebounce != "" &&
		directives.DebounceMs == nil
	queueCapInvalid := directives.RawCap != "" &&
		directives.Cap == nil
	queueDropInvalid := directives.RawDrop != "" &&
		directives.DropPolicy == ""

	if queueModeInvalid || queueDebounceInvalid || queueCapInvalid || queueDropInvalid {
		var errors []string
		if queueModeInvalid {
			errors = append(errors, fmt.Sprintf(
				`Unrecognized queue mode "%s". Valid modes: steer, followup, collect, steer+backlog, interrupt.`,
				directives.RawQueueMode,
			))
		}
		if queueDebounceInvalid {
			errors = append(errors, fmt.Sprintf(
				`Invalid debounce "%s". Use ms/s/m (e.g. debounce:1500ms, debounce:2s).`,
				directives.RawDebounce,
			))
		}
		if queueCapInvalid {
			errors = append(errors, fmt.Sprintf(
				`Invalid cap "%s". Use a positive integer (e.g. cap:10).`,
				directives.RawCap,
			))
		}
		if queueDropInvalid {
			errors = append(errors, fmt.Sprintf(
				`Invalid drop policy "%s". Use drop:old, drop:new, or drop:summarize.`,
				directives.RawDrop,
			))
		}
		return &autoreply.ReplyPayload{
			Text: strings.Join(errors, " "),
		}
	}

	// 参数合法 → 交由持久化层处理
	return nil
}
