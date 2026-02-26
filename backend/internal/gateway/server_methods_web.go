package gateway

// server_methods_web.go — web.login.* 方法处理器
// 对应 TS: src/gateway/server-methods/ (web.login 部分)
//
// 方法列表 (2): web.login.start, web.login.wait
//
// TS: WhatsApp QR 扫码登录流程。
// Go: 委托 ChannelPluginsProvider 接口。

import (
	"strings"
)

// WebHandlers 返回 web.login.* 方法映射。
func WebHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"web.login.start": handleWebLoginStart,
		"web.login.wait":  handleWebLoginWait,
	}
}

// ---------- web.login.start ----------

func handleWebLoginStart(ctx *MethodHandlerContext) {
	if ctx.Context.ChannelPlugins == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "channel plugins not available"))
		return
	}

	accountID, _ := ctx.Params["accountId"].(string)
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "web.login.start requires accountId"))
		return
	}

	provider, err := ctx.Context.ChannelPlugins.FindWebLoginProvider(accountID)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "web login provider not found: "+err.Error()))
		return
	}

	result, err := provider.LoginWithQrStart(accountID)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "web.login.start failed: "+err.Error()))
		return
	}

	ctx.Respond(true, result, nil)
}

// ---------- web.login.wait ----------

func handleWebLoginWait(ctx *MethodHandlerContext) {
	if ctx.Context.ChannelPlugins == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "channel plugins not available"))
		return
	}

	accountID, _ := ctx.Params["accountId"].(string)
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "web.login.wait requires accountId"))
		return
	}

	provider, err := ctx.Context.ChannelPlugins.FindWebLoginProvider(accountID)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "web login provider not found: "+err.Error()))
		return
	}

	result, err := provider.LoginWithQrWait(accountID)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "web.login.wait failed: "+err.Error()))
		return
	}

	ctx.Respond(true, result, nil)
}
