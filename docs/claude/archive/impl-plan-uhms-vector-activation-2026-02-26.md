---
document_type: Archive
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-uhms-vector-activation.md
skill5_verified: true
---

# UHMS 记忆系统向量检索激活 — 实施计划

## Context

当前 UHMS 记忆系统存在三个关键架构缺陷：

1. **VectorIndex 从未注入**: `uhms.VectorIndex` 接口已定义 (`interfaces.go:48`)，`DefaultManager` 有 `vectorIndex` 字段，`SetVectorBackend()` 方法存在 — 但**从未被调用**。结果：所有记忆搜索仅走 FTS5 关键词匹配，语义搜索完全失效。

2. **Qdrant FFI 桥与 UHMS 断裂**: Qdrant segment 引擎 FFI 桥完整实现在 `go-api/internal/ffi/` (独立 Go module `github.com/uhms/go-api`)，但 UHMS (`backend/internal/memory/uhms/`) 在主模块 (`github.com/anthropic/open-acosmi`) 中，由于 Go `internal` 包可见性规则无法跨模块导入。

3. **Embedding 维度不统一**: Go API 默认 512 维，Rust 默认 1536 维，OpenAI text-embedding-3-small 输出 1536 维。无验证机制。

## Online Verification Log

### Qdrant HNSW vs SQLite-vec 性能对比
- **Query**: "Qdrant HNSW vs brute force vector search performance benchmark 2025"
- **Source**: https://qdrant.tech/benchmarks/ + https://alexgarcia.xyz/blog/2024/sqlite-vec-stable-release/
- **Key finding**: HNSW 检索复杂度 O(log n)，暴力搜索 O(n)。sqlite-vec 无 ANN 索引（暴力扫描），Qdrant segment HNSW 在 >10K 条目时快数量级。
- **Verified date**: 2026-02-26

### RAG 压缩时自动记忆注入
- **Query**: "LLM agent memory retrieval during context compression automatic memory injection 2025"
- **Source**: https://arxiv.org/html/2601.01885v1 (AgeMem) + https://arxiv.org/pdf/2510.00615 (ACON)
- **Key finding**: ACON 提出压缩时 context-aware retrieval 优于事后检索。当前 UHMS 已有压缩时注入 (`BuildContextBlock`)，但依赖 vectorIndex 才能发挥最大效果。
- **Verified date**: 2026-02-26

### Embedding 维度不匹配最佳实践
- **Query**: "embedding dimension mismatch detection validation vector database best practice"
- **Source**: https://medium.com/@epappas/dealing-with-vector-dimension-mismatch
- **Key finding**: 始终在插入前验证 embedding 维度 == collection 维度。最佳实践：collection 创建时记录 dimension，insert/search 时校验 `len(vec) == collection.dim`。
- **Verified date**: 2026-02-26

### Qdrant Segment 进程内模式
- **Query**: "Qdrant segment library in-process vector store"
- **Source**: https://qdrant.tech/qdrant-vector-database/ + openviking-vector 源码
- **Key finding**: Qdrant `segment` crate 支持进程内模式（无需 gRPC 服务），openviking-vector 已封装为 `SegmentVectorStore`。HNSW 索引、余弦距离、磁盘持久化均已就绪。
- **Verified date**: 2026-02-26

---

## Phase 1: VectorIndex Adapter — Qdrant Segment FFI (~280 LOC Go) ✅

- [x] 1.1 `uhms/vectoradapter/segment_cgo.go` — CGO FFI 绑定 (~195 LOC)
- [x] 1.2 `uhms/vectoradapter/segment_pure.go` — 纯 Go fallback (~165 LOC)
- [x] 1.3 `uhms/vectoradapter/adapter.go` — uhms.VectorIndex 实现 (~117 LOC)

## Phase 2: EmbeddingProvider Adapter (~80 LOC Go) ✅

- [x] 2.1 `uhms/vectoradapter/embedding.go` — HTTP embedding 客户端 (~225 LOC)

## Phase 3: Gateway 接线 (~50 LOC Go) ✅

- [x] 3.1 `gateway/server.go` — `initUHMSVectorBackend()` 函数 + 注入调用

## Phase 4: 维度验证与安全 (~40 LOC Go) ✅

- [x] 4.1 `adapter.go` Upsert/Search 维度校验 (已含在 P1)
- [x] 4.2 `indexVector()` 错误传播到 slog.Warn (已有)

## Phase 5: lastSummary 持久化 (~40 LOC Go) ✅

- [x] 5.1 `uhms/manager.go` — `persistLastSummary`/`loadLastSummary` + VFS 直接 I/O
- [x] 5.2 不需要修改 vfs.go — 使用 `os.WriteFile`/`os.ReadFile` 直接操作 VFS root

---

## 验证

- [x] `CGO_ENABLED=0 go build ./...` 全量编译通过
- [x] VectorMode=off 时行为不变 (initUHMSVectorBackend 不调用)
- [x] 纯 Go fallback (CGO_ENABLED=0) 使用内存暴力搜索

## 新增文件清单

| 文件 | LOC | 说明 |
|------|-----|------|
| `uhms/vectoradapter/segment_cgo.go` | ~195 | CGO FFI 绑定 (Qdrant segment) |
| `uhms/vectoradapter/segment_pure.go` | ~165 | 纯 Go fallback (暴力余弦) |
| `uhms/vectoradapter/adapter.go` | ~117 | uhms.VectorIndex 实现 + 维度校验 |
| `uhms/vectoradapter/embedding.go` | ~225 | HTTP embedding 多 provider 支持 |

## 修改文件清单

| 文件 | 变更 |
|------|------|
| `uhms/manager.go` | +import os/filepath, +loadLastSummary 调用, +persistLastSummary 调用, +persist/load 方法 |
| `gateway/server.go` | +import vectoradapter, +initUHMSVectorBackend 函数, +注入调用 |
