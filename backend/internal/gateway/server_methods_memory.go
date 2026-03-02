package gateway

// server_methods_memory.go — memory.* 直接操作 RPC 方法
//
// 补全前端对记忆系统的完整管理能力:
//   memory.list           — 分页列表
//   memory.get            — 详情 + VFS 内容
//   memory.delete         — 删除 (含所有权校验)
//   memory.compress       — 上下文压缩
//   memory.commit         — 会话提交 → 记忆提取
//   memory.decay.run      — 手动触发衰减
//   memory.import.skills  — 批量导入技能文档到 UHMS (L0/L1/L2)

import (
	"context"

	"github.com/openacosmi/claw-acismi/internal/agents/scope"
	"github.com/openacosmi/claw-acismi/internal/agents/skills"
	"github.com/openacosmi/claw-acismi/internal/memory/uhms"
)

// MemoryHandlers 返回 memory.* 直接操作方法映射。
func MemoryHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"memory.list":          handleMemoryList,
		"memory.get":           handleMemoryGet,
		"memory.delete":        handleMemoryDelete,
		"memory.compress":      handleMemoryCompress,
		"memory.commit":        handleMemoryCommit,
		"memory.decay.run":     handleMemoryDecayRun,
		"memory.import.skills": handleMemoryImportSkills,
		"memory.stats":         handleMemoryStats,
	}
}

// ---------- memory.list ----------

func handleMemoryList(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	userID, _ := ctx.Params["userId"].(string)
	if userID == "" {
		userID = "default"
	}

	// 分页参数
	limit := 50
	if raw, ok := ctx.Params["limit"].(float64); ok && raw > 0 {
		limit = int(raw)
	}
	if limit > 200 {
		limit = 200
	}

	offset := 0
	if raw, ok := ctx.Params["offset"].(float64); ok && raw >= 0 {
		offset = int(raw)
	}

	// 过滤参数
	opts := uhms.ListOptions{
		Limit:  limit,
		Offset: offset,
	}
	if raw, ok := ctx.Params["type"].(string); ok {
		opts.MemoryType = uhms.MemoryType(raw)
	}
	if raw, ok := ctx.Params["category"].(string); ok {
		opts.Category = uhms.MemoryCategory(raw)
	}
	if raw, ok := ctx.Params["minImportance"].(float64); ok {
		opts.MinImportance = raw
	}

	memories, total, err := mgr.ListMemories(context.Background(), userID, opts)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "list failed: "+err.Error()))
		return
	}

	items := make([]map[string]interface{}, len(memories))
	for i, m := range memories {
		items[i] = memoryToMap(&m)
	}

	ctx.Respond(true, map[string]interface{}{
		"memories": items,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	}, nil)
}

// ---------- memory.get ----------

func handleMemoryGet(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "id is required"))
		return
	}

	mem, err := mgr.GetMemory(context.Background(), id)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeNotFound, "memory not found: "+err.Error()))
		return
	}

	result := memoryToMap(mem)

	// 按 level 加载 VFS 内容
	level := 0
	if raw, ok := ctx.Params["level"].(float64); ok {
		level = int(raw)
	}
	if level < 0 || level > 2 {
		level = 0
	}

	content, vfsErr := mgr.ReadVFSContent(
		mem.UserID, string(mem.MemoryType), string(mem.Category), mem.ID, level,
	)
	if vfsErr == nil && content != "" {
		result["vfsContent"] = content
		result["vfsLevel"] = level
	}

	ctx.Respond(true, result, nil)
}

// ---------- memory.delete ----------

func handleMemoryDelete(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	id, _ := ctx.Params["id"].(string)
	if id == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "id is required"))
		return
	}

	userID, _ := ctx.Params["userId"].(string)
	if userID == "" {
		userID = "default"
	}

	if err := mgr.DeleteMemory(context.Background(), userID, id); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "delete failed: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"deleted": true,
		"id":      id,
	}, nil)
}

// ---------- memory.compress ----------

func handleMemoryCompress(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	rawMessages, ok := ctx.Params["messages"].([]interface{})
	if !ok || len(rawMessages) == 0 {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "messages array is required"))
		return
	}

	// 解析消息
	messages := make([]uhms.Message, 0, len(rawMessages))
	for _, raw := range rawMessages {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "" || content == "" {
			continue
		}
		messages = append(messages, uhms.Message{Role: role, Content: content})
	}

	if len(messages) == 0 {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "no valid messages provided"))
		return
	}

	tokenBudget := 0
	if raw, ok := ctx.Params["tokenBudget"].(float64); ok && raw > 0 {
		tokenBudget = int(raw)
	}

	originalCount := len(messages)
	compressed, err := mgr.CompressIfNeeded(context.Background(), messages, tokenBudget)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "compress failed: "+err.Error()))
		return
	}

	// 构建响应消息
	respMessages := make([]map[string]interface{}, len(compressed))
	for i, msg := range compressed {
		respMessages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	ctx.Respond(true, map[string]interface{}{
		"messages":        respMessages,
		"originalCount":   originalCount,
		"compressedCount": len(compressed),
	}, nil)
}

// ---------- memory.commit ----------

func handleMemoryCommit(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	userID, _ := ctx.Params["userId"].(string)
	if userID == "" {
		userID = "default"
	}

	sessionKey, _ := ctx.Params["sessionKey"].(string)
	if sessionKey == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "sessionKey is required"))
		return
	}

	rawTranscript, ok := ctx.Params["transcript"].([]interface{})
	if !ok || len(rawTranscript) == 0 {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "transcript array is required"))
		return
	}

	// 解析 transcript
	transcript := make([]uhms.Message, 0, len(rawTranscript))
	for _, raw := range rawTranscript {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "" || content == "" {
			continue
		}
		transcript = append(transcript, uhms.Message{Role: role, Content: content})
	}

	if len(transcript) == 0 {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "no valid transcript messages"))
		return
	}

	result, err := mgr.CommitSession(context.Background(), userID, sessionKey, transcript)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "commit failed: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"sessionKey":      result.SessionKey,
		"memoriesCreated": result.MemoriesCreated,
		"memoryIds":       result.MemoryIDs,
		"archivePath":     result.ArchivePath,
		"tokensSaved":     result.TokensSaved,
	}, nil)
}

// ---------- memory.decay.run ----------

func handleMemoryDecayRun(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	userID, _ := ctx.Params["userId"].(string)
	if userID == "" {
		userID = "default"
	}

	if err := mgr.RunDecayCycle(context.Background(), userID); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "decay run failed: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"success": true,
		"userId":  userID,
	}, nil)
}

// ---------- memory.import.skills ----------

func handleMemoryImportSkills(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	userID, _ := ctx.Params["userId"].(string)
	if userID == "" {
		userID = "default"
	}

	// 加载配置获取技能列表
	loader := ctx.Context.ConfigLoader
	if loader == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config not available"))
		return
	}
	cfg, err := loader.LoadConfig()
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to load config: "+err.Error()))
		return
	}

	agentID := scope.ResolveDefaultAgentId(cfg)
	workspaceDir := scope.ResolveAgentWorkspaceDir(cfg, agentID)
	bundledDir := skills.ResolveBundledSkillsDir("")

	entries := skills.LoadSkillEntries(workspaceDir, "", bundledDir, cfg)
	if len(entries) == 0 {
		ctx.Respond(true, map[string]interface{}{
			"imported": 0, "skipped": 0, "updated": 0, "failed": 0, "total": 0,
			"skills": []interface{}{},
		}, nil)
		return
	}

	var imported, skipped, updated, failed int
	skillResults := make([]map[string]interface{}, 0, len(entries))

	for _, entry := range entries {
		s := entry.Skill
		if s.Content == "" {
			continue
		}

		result, importErr := mgr.ImportSkill(context.Background(), userID, s.Name, s.Content)
		if importErr != nil {
			failed++
			skillResults = append(skillResults, map[string]interface{}{
				"name":   s.Name,
				"status": "failed",
				"error":  importErr.Error(),
			})
			continue
		}

		switch result.Status {
		case "imported":
			imported++
		case "skipped":
			skipped++
		case "updated":
			updated++
		}

		entry := map[string]interface{}{
			"name":   s.Name,
			"id":     result.Memory.ID,
			"status": result.Status,
		}
		skillResults = append(skillResults, entry)
	}

	ctx.Respond(true, map[string]interface{}{
		"imported": imported,
		"skipped":  skipped,
		"updated":  updated,
		"failed":   failed,
		"total":    len(entries),
		"skills":   skillResults,
	}, nil)
}

// ---------- memory.stats ----------

func handleMemoryStats(ctx *MethodHandlerContext) {
	mgr := ctx.Context.UHMSManager
	if mgr == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "UHMS not enabled"))
		return
	}

	userID, _ := ctx.Params["userId"].(string)
	if userID == "" {
		userID = "default"
	}

	stats, err := mgr.AggregateStats(userID)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "stats failed: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"byType":        stats.ByType,
		"byCategory":    stats.ByCategory,
		"byRetention":   stats.ByRetention,
		"decayHealth":   stats.DecayHealth,
		"totalAccess":   stats.TotalAccess,
		"avgImportance": stats.AvgImportance,
		"oldestAt":      stats.OldestAt,
		"newestAt":      stats.NewestAt,
	}, nil)
}

// ---------- 共享 helper ----------

// memoryToMap 将 Memory 结构体转为 JSON-friendly map，时间戳用 Unix 秒。
func memoryToMap(m *uhms.Memory) map[string]interface{} {
	result := map[string]interface{}{
		"id":              m.ID,
		"userId":          m.UserID,
		"content":         m.Content,
		"type":            string(m.MemoryType),
		"category":        string(m.Category),
		"importanceScore": m.ImportanceScore,
		"decayFactor":     m.DecayFactor,
		"retentionPolicy": string(m.RetentionPolicy),
		"accessCount":     m.AccessCount,
		"ingestedAt":      m.IngestedAt.Unix(),
		"createdAt":       m.CreatedAt.Unix(),
		"updatedAt":       m.UpdatedAt.Unix(),
	}

	if m.LastAccessedAt != nil {
		result["lastAccessedAt"] = m.LastAccessedAt.Unix()
	}
	if m.ArchivedAt != nil {
		result["archivedAt"] = m.ArchivedAt.Unix()
	}
	if m.EventTime != nil {
		result["eventTime"] = m.EventTime.Unix()
	}
	if m.VFSPath != "" {
		result["vfsPath"] = m.VFSPath
	}

	return result
}
