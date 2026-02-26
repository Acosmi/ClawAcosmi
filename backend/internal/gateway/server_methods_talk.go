package gateway

// server_methods_talk.go — talk.* 方法处理器
// 对应 TS: src/gateway/server-methods/misc (talk.mode 部分)
//
// 方法列表 (1): talk.mode
//
// TS: 切换对话模式（text/voice），广播变更给所有连接客户端。

import (
	"strings"
)

// TalkHandlers 返回 talk.* 方法映射。
func TalkHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"talk.mode": handleTalkMode,
	}
}

// ---------- talk.mode ----------

func handleTalkMode(ctx *MethodHandlerContext) {
	mode, _ := ctx.Params["mode"].(string)
	mode = strings.TrimSpace(mode)

	// TS: 验证 mode 值
	validModes := map[string]bool{
		"text":  true,
		"voice": true,
		"":      true, // 空值表示查询当前模式
	}
	if !validModes[mode] {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "talk.mode requires mode: 'text' | 'voice'"))
		return
	}

	if mode == "" {
		// 查询模式 — 返回当前模式
		ctx.Respond(true, map[string]interface{}{
			"mode": "text", // 默认文本模式
		}, nil)
		return
	}

	// 广播模式变更
	if ctx.Context.BroadcastFn != nil {
		ctx.Context.BroadcastFn("talk.mode.changed", map[string]interface{}{
			"mode": mode,
		}, nil)
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":   true,
		"mode": mode,
	}, nil)
}
