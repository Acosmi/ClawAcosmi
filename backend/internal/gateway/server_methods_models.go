package gateway

// models.* 方法处理器 — 对应 src/gateway/server-methods/models.ts
//
// 提供模型目录查询功能。
// 依赖: ModelCatalog (models.list)

// ModelsHandlers 返回 models.* 方法处理器映射。
func ModelsHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"models.list": handleModelsList,
	}
}

// ---------- models.list ----------
// 对应 TS models.ts:L1-L30
// 返回模型目录全量条目。

func handleModelsList(ctx *MethodHandlerContext) {
	catalog := ctx.Context.ModelCatalog
	if catalog == nil {
		ctx.Respond(true, map[string]interface{}{
			"models": []interface{}{},
		}, nil)
		return
	}

	entries := catalog.All()
	ctx.Respond(true, map[string]interface{}{
		"models": entries,
	}, nil)
}
