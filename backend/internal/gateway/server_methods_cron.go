package gateway

// server_methods_cron.go — cron.* + wake 方法处理器
// 对应 TS: src/gateway/server-methods/cron.ts (228L)
//
// 方法列表 (8):
//   wake, cron.list, cron.status, cron.runs,
//   cron.add, cron.update, cron.remove, cron.run

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/cron"
)

// CronHandlers 返回 cron.* + wake 方法映射。
func CronHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"wake":        handleWake,
		"cron.list":   handleCronList,
		"cron.status": handleCronStatus,
		"cron.runs":   handleCronRuns,
		"cron.add":    handleCronAdd,
		"cron.update": handleCronUpdate,
		"cron.remove": handleCronRemove,
		"cron.run":    handleCronRun,
	}
}

// ---------- wake ----------

func handleWake(ctx *MethodHandlerContext) {
	svc := ctx.Context.CronService
	if svc == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron service not available"))
		return
	}

	mode, _ := ctx.Params["mode"].(string)
	if mode == "" {
		mode = "now"
	}
	text, _ := ctx.Params["text"].(string)

	result := svc.Wake(mode, text)
	ctx.Respond(true, result, nil)
}

// ---------- cron.list ----------

func handleCronList(ctx *MethodHandlerContext) {
	svc := ctx.Context.CronService
	if svc == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron service not available"))
		return
	}

	includeDisabled, _ := ctx.Params["includeDisabled"].(bool)

	jobs, err := svc.List(includeDisabled)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to list cron jobs: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"jobs": jobs,
	}, nil)
}

// ---------- cron.status ----------

func handleCronStatus(ctx *MethodHandlerContext) {
	svc := ctx.Context.CronService
	if svc == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron service not available"))
		return
	}

	result := svc.Status()
	ctx.Respond(true, result, nil)
}

// ---------- cron.runs ----------

func handleCronRuns(ctx *MethodHandlerContext) {
	storePath := ctx.Context.CronStorePath
	if storePath == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron store path not configured"))
		return
	}

	// 参数：limit, jobId
	limit := 200
	if l, ok := toInt(ctx.Params["limit"]); ok && l > 0 {
		limit = l
	}
	if limit > 5000 {
		limit = 5000
	}

	jobID, _ := ctx.Params["jobId"].(string)
	jobID = strings.TrimSpace(jobID)

	logPath := cron.ResolveCronRunLogPath(storePath, "")
	if jobID != "" {
		logPath = cron.ResolveCronRunLogPath(storePath, jobID)
	}

	entries := cron.ReadCronRunLogEntries(logPath, limit, jobID)
	if entries == nil {
		entries = []cron.CronRunLogEntry{}
	}

	ctx.Respond(true, map[string]interface{}{
		"entries": entries,
	}, nil)
}

// ---------- cron.add ----------

func handleCronAdd(ctx *MethodHandlerContext) {
	svc := ctx.Context.CronService
	if svc == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron service not available"))
		return
	}

	// 使用松散 map 解析规范化
	input, err := cron.NormalizeCronJobInput(ctx.Params)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid cron.add params: "+err.Error()))
		return
	}

	// 校验 at 调度时间戳
	tsResult := cron.ValidateScheduleTimestamp(input.Schedule, time.Now().UnixMilli())
	if !tsResult.OK {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, tsResult.Message))
		return
	}

	result, err := svc.Add(*input)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to add cron job: "+err.Error()))
		return
	}

	ctx.Respond(result.OK, result, nil)
}

// ---------- cron.update ----------

func handleCronUpdate(ctx *MethodHandlerContext) {
	svc := ctx.Context.CronService
	if svc == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron service not available"))
		return
	}

	// TS: jobId = p.id ?? p.jobId
	id, _ := ctx.Params["id"].(string)
	if id == "" {
		id, _ = ctx.Params["jobId"].(string)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid cron.update params: missing id"))
		return
	}

	// TS: patch 从 params.patch 子对象提取，not top-level
	patchRaw, ok := ctx.Params["patch"].(map[string]interface{})
	if !ok || len(patchRaw) == 0 {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "cron.update requires patch object"))
		return
	}

	// JSON round-trip: map → CronJobPatch struct
	patchBytes, err := json.Marshal(patchRaw)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid patch: "+err.Error()))
		return
	}
	var patch cron.CronJobPatch
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid patch: "+err.Error()))
		return
	}

	// 规范化 patch
	if err := cron.NormalizeCronJobPatch(&patch); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid patch: "+err.Error()))
		return
	}
	if patch.Schedule != nil {
		tsResult := cron.ValidateScheduleTimestamp(*patch.Schedule, time.Now().UnixMilli())
		if !tsResult.OK {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, tsResult.Message))
			return
		}
	}

	result, err := svc.Update(id, patch)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to update cron job: "+err.Error()))
		return
	}

	ctx.Respond(result.OK, result, nil)
}

// ---------- cron.remove ----------

func handleCronRemove(ctx *MethodHandlerContext) {
	svc := ctx.Context.CronService
	if svc == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron service not available"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "cron.remove requires id"))
		return
	}

	result, err := svc.Remove(id)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to remove cron job: "+err.Error()))
		return
	}

	ctx.Respond(result.OK, result, nil)
}

// ---------- cron.run ----------

func handleCronRun(ctx *MethodHandlerContext) {
	svc := ctx.Context.CronService
	if svc == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "cron service not available"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "cron.run requires id"))
		return
	}

	// TS: p.mode ?? "force"
	mode, _ := ctx.Params["mode"].(string)
	if mode == "" {
		mode = "force"
	}

	result, err := svc.Run(id, mode)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to run cron job: "+err.Error()))
		return
	}

	ctx.Respond(result.OK, result, nil)
}

// ---------- 辅助 ----------

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
