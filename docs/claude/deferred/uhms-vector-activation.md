---
document_type: Deferred
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-uhms-vector-activation.md
skill5_verified: true
---

# UHMS 向量检索激活 — 延迟项 ✅ 全部修复

## 来源

审计报告 `audit-2026-02-26-uhms-vector-activation.md` LOW/INFO 发现。

## 已完成

### 1. EmbeddingModel / EmbeddingBaseURL 配置字段 (F-07 LOW) ✅

- `types_memory.go`: 添加 `EmbeddingModel` / `EmbeddingBaseURL` 到 `MemoryUHMSConfig`
- `config.go`: 同步到 `UHMSConfig`
- `server.go`: `configToUHMSConfig` 传递新字段
- `initUHMSVectorBackend`: model/baseURL 传递到 `NewHTTPEmbeddingProvider`

### 2. Embedding HTTP 重试 (F-06 LOW) ✅

- `embedding.go`: `Embed()` 拆分为 `doEmbed()` + retry wrapper
- 1 次重试 + 1s 退避，仅对可恢复错误 (5xx, timeout, connection reset)
- `isRetryable()` 基于错误字符串关键词匹配

### 3. adapter.go memoryCollections 动态化 (F-10 INFO) ✅

- `memoryCollections` 从 `var []string` 改为 `func() []string`
- 动态从 `uhms.AllMemoryTypes` 生成，新增 MemoryType 自动同步

### 4. segment_pure.go Upsert 维度校验 (F-04 LOW) ✅

- 纯 Go fallback `Upsert` 添加 `len(denseVec) != col.dim` 校验
- 与 adapter 层双重防护
