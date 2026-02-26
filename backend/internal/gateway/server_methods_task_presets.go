package gateway

// server_methods_task_presets.go — P5 任务级预设权限 API 方法处理器
//
// 方法:
//   - security.taskPresets.list    — 列出所有预设
//   - security.taskPresets.add     — 新增预设
//   - security.taskPresets.update  — 更新预设
//   - security.taskPresets.remove  — 删除预设
//   - security.taskPresets.match   — 测试任务名匹配

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// TaskPresetHandlers 返回 security.taskPresets.* 方法处理器映射。
func TaskPresetHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"security.taskPresets.list":   handleTaskPresetsList,
		"security.taskPresets.add":    handleTaskPresetsAdd,
		"security.taskPresets.update": handleTaskPresetsUpdate,
		"security.taskPresets.remove": handleTaskPresetsRemove,
		"security.taskPresets.match":  handleTaskPresetsMatch,
	}
}

// ---------- security.taskPresets.list ----------

func handleTaskPresetsList(ctx *MethodHandlerContext) {
	mgr := ctx.Context.TaskPresetMgr
	if mgr == nil {
		ctx.Respond(true, map[string]interface{}{
			"presets": []interface{}{},
			"total":   0,
		}, nil)
		return
	}

	presets := mgr.List()
	ctx.Respond(true, map[string]interface{}{
		"presets": presets,
		"total":   len(presets),
	}, nil)
}

// ---------- security.taskPresets.add ----------

func handleTaskPresetsAdd(ctx *MethodHandlerContext) {
	mgr := ctx.Context.TaskPresetMgr
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "task preset manager not initialized"))
		return
	}

	name, _ := ctx.Params["name"].(string)
	name = strings.TrimSpace(name)
	pattern, _ := ctx.Params["pattern"].(string)
	pattern = strings.TrimSpace(pattern)
	level, _ := ctx.Params["level"].(string)
	level = strings.TrimSpace(level)
	description, _ := ctx.Params["description"].(string)
	description = strings.TrimSpace(description)
	autoApprove, _ := ctx.Params["autoApprove"].(bool)

	maxTTL := 60
	if ttlRaw, ok := ctx.Params["maxTtlMinutes"].(float64); ok && ttlRaw > 0 {
		maxTTL = int(ttlRaw)
	}

	preset := TaskPreset{
		ID:          generatePresetID(),
		Name:        name,
		Pattern:     pattern,
		Level:       level,
		AutoApprove: autoApprove,
		MaxTTL:      maxTTL,
		Description: description,
	}

	if err := mgr.Add(preset); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"id":     preset.ID,
		"status": "created",
	}, nil)
}

// ---------- security.taskPresets.update ----------

func handleTaskPresetsUpdate(ctx *MethodHandlerContext) {
	mgr := ctx.Context.TaskPresetMgr
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "task preset manager not initialized"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "preset id is required"))
		return
	}

	name, _ := ctx.Params["name"].(string)
	pattern, _ := ctx.Params["pattern"].(string)
	level, _ := ctx.Params["level"].(string)
	description, _ := ctx.Params["description"].(string)
	autoApprove, _ := ctx.Params["autoApprove"].(bool)

	maxTTL := 0
	if ttlRaw, ok := ctx.Params["maxTtlMinutes"].(float64); ok && ttlRaw > 0 {
		maxTTL = int(ttlRaw)
	}

	update := TaskPreset{
		Name:        strings.TrimSpace(name),
		Pattern:     strings.TrimSpace(pattern),
		Level:       strings.TrimSpace(level),
		AutoApprove: autoApprove,
		MaxTTL:      maxTTL,
		Description: strings.TrimSpace(description),
	}

	if err := mgr.Update(id, update); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"id":     id,
		"status": "updated",
	}, nil)
}

// ---------- security.taskPresets.remove ----------

func handleTaskPresetsRemove(ctx *MethodHandlerContext) {
	mgr := ctx.Context.TaskPresetMgr
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "task preset manager not initialized"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "preset id is required"))
		return
	}

	if err := mgr.Remove(id); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"id":     id,
		"status": "removed",
	}, nil)
}

// ---------- security.taskPresets.match ----------

func handleTaskPresetsMatch(ctx *MethodHandlerContext) {
	mgr := ctx.Context.TaskPresetMgr
	if mgr == nil {
		ctx.Respond(true, map[string]interface{}{
			"matched": false,
		}, nil)
		return
	}

	taskName, _ := ctx.Params["taskName"].(string)
	taskName = strings.TrimSpace(taskName)
	if taskName == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "taskName is required"))
		return
	}

	result := mgr.Match(taskName)
	ctx.Respond(true, result, nil)
}

// ---------- 辅助 ----------

func generatePresetID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "tp_" + hex.EncodeToString(buf)
}
