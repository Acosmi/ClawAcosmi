package gateway

// server_methods_media.go — 媒体子系统 RPC 方法
// 提供 media.trending.fetch / media.trending.sources / media.drafts.list / media.drafts.get / media.drafts.delete 方法
// 遵循 server_methods_image.go 模式

import (
	"context"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/media"
)

// MediaHandlers 返回媒体子系统 RPC 方法处理器。
func MediaHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"media.trending.fetch":   handleMediaTrendingFetch,
		"media.trending.sources": handleMediaTrendingSources,
		"media.drafts.list":      handleMediaDraftsList,
		"media.drafts.get":       handleMediaDraftsGet,
		"media.drafts.delete":    handleMediaDraftsDelete,
		"media.publish.list":     handleMediaPublishList,
		"media.publish.get":      handleMediaPublishGet,
	}
}

// ---------- media.trending.fetch ----------

func handleMediaTrendingFetch(ctx *MethodHandlerContext) {
	sub := ctx.Context.MediaSubsystem
	if sub == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "media subsystem not available"))
		return
	}

	source, _ := ctx.Params["source"].(string)
	category, _ := ctx.Params["category"].(string)
	limit := 20
	if l, ok := ctx.Params["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	fetchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if source != "" {
		topics, err := sub.Aggregator.FetchBySource(fetchCtx, source, category, limit)
		if err != nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "fetch trending: "+err.Error()))
			return
		}
		ctx.Respond(true, map[string]interface{}{
			"source": source,
			"topics": topics,
			"count":  len(topics),
		}, nil)
		return
	}

	topics, results := sub.Aggregator.FetchAll(fetchCtx, category, limit)
	var errors []map[string]string
	for _, r := range results {
		if r.Err != nil {
			errors = append(errors, map[string]string{
				"source": r.Source,
				"error":  r.Err.Error(),
			})
		}
	}

	ctx.Respond(true, map[string]interface{}{
		"topics": topics,
		"count":  len(topics),
		"errors": errors,
	}, nil)
}

// ---------- media.trending.sources ----------

func handleMediaTrendingSources(ctx *MethodHandlerContext) {
	sub := ctx.Context.MediaSubsystem
	if sub == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "media subsystem not available"))
		return
	}

	names := sub.Aggregator.SourceNames()
	ctx.Respond(true, map[string]interface{}{
		"sources": names,
	}, nil)
}

// ---------- media.drafts.list ----------

func handleMediaDraftsList(ctx *MethodHandlerContext) {
	sub := ctx.Context.MediaSubsystem
	if sub == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "media subsystem not available"))
		return
	}

	platform, _ := ctx.Params["platform"].(string)
	drafts, err := sub.DraftStore.List(platform)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "list drafts: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"drafts": drafts,
		"count":  len(drafts),
	}, nil)
}

// ---------- media.drafts.get ----------

func handleMediaDraftsGet(ctx *MethodHandlerContext) {
	sub := ctx.Context.MediaSubsystem
	if sub == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "media subsystem not available"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing draft id"))
		return
	}

	draft, err := sub.DraftStore.Get(id)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "get draft: "+err.Error()))
		return
	}

	ctx.Respond(true, draft, nil)
}

// ---------- media.drafts.delete ----------

func handleMediaDraftsDelete(ctx *MethodHandlerContext) {
	sub := ctx.Context.MediaSubsystem
	if sub == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "media subsystem not available"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing draft id"))
		return
	}

	if err := sub.DraftStore.Delete(id); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "delete draft: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"deleted": true,
		"id":      id,
	}, nil)
}

// ---------- media.publish.list ----------

func handleMediaPublishList(ctx *MethodHandlerContext) {
	sub := ctx.Context.MediaSubsystem
	if sub == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "media subsystem not available"))
		return
	}
	if sub.PublishHistory == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "publish history not available"))
		return
	}

	var opts *media.PublishListOptions
	limit, hasLimit := ctx.Params["limit"].(float64)
	offset, hasOffset := ctx.Params["offset"].(float64)
	if hasLimit || hasOffset {
		opts = &media.PublishListOptions{}
		if hasLimit && limit > 0 {
			opts.Limit = int(limit)
		}
		if hasOffset && offset > 0 {
			opts.Offset = int(offset)
		}
	}

	records, err := sub.PublishHistory.List(opts)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "list publish history: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"records": records,
		"count":   len(records),
	}, nil)
}

// ---------- media.publish.get ----------

func handleMediaPublishGet(ctx *MethodHandlerContext) {
	sub := ctx.Context.MediaSubsystem
	if sub == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "media subsystem not available"))
		return
	}
	if sub.PublishHistory == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "publish history not available"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing publish record id"))
		return
	}

	record, err := sub.PublishHistory.Get(id)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "get publish record: "+err.Error()))
		return
	}

	ctx.Respond(true, record, nil)
}
