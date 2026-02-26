---
document_type: Audit
status: Final
created: 2026-02-26
last_updated: 2026-02-26
---

# 审计报告: UHMS 记忆系统向量检索激活

## Scope

审计范围覆盖 UHMS 向量检索激活全部代码变更:

| 文件 | 操作 | LOC |
|------|------|-----|
| `uhms/vectoradapter/segment_cgo.go` | 新建 | ~220 |
| `uhms/vectoradapter/segment_pure.go` | 新建 | ~165 |
| `uhms/vectoradapter/adapter.go` | 新建 | ~122 |
| `uhms/vectoradapter/embedding.go` | 新建 | ~255 |
| `uhms/manager.go` | 编辑 | +35 |
| `gateway/server.go` | 编辑 | +48 |

---

## Findings

### F-01 [HIGH] segment_cgo.go — 空 dataDir 导致 CGO panic ✅ FIXED

**Location**: `segment_cgo.go:67-76` (`NewSegmentStore`)

**Issue**: `[]byte("")` 创建长度为 0 的 slice，`&dirBytes[0]` 产生 index out of bounds panic。虽然 gateway 调用路径保证 dataDir 非空 (`filepath.Join(resolvedVFSPath, "segment-vectors")`)，但底层 CGO 函数缺少独立防护。

**Risk**: HIGH — 若被其他调用方以空字符串调用，进程直接 crash。

**Fix**: 添加 `if dataDir == "" { return nil, error }` 前置校验。

**Verification**: 已修复，编译通过。

---

### F-02 [MEDIUM] embedding.go — dimension 字段并发写入竞态 ✅ FIXED

**Location**: `embedding.go:120-122, 172-174` (`embedOpenAI`, `embedOllama`)

**Issue**: `if e.dimension == 0 { e.dimension = len(vec) }` 在并发调用时存在 data race。多个 goroutine 首次调用 `Embed()` 时可能同时读写 `e.dimension`。

**Mitigating factor**: `inferDimension()` 对所有已知 provider 返回非零值，实际生产中 `e.dimension` 在构造时就非零。仅当使用完全未知的 provider+model 组合时才触发。

**Risk**: MEDIUM — 虽然写入值幂等（所有 goroutine 写同一值），但仍违反 Go race detector。

**Fix**: 使用 `sync.Once` 保护自动检测路径。

**Verification**: 已修复，编译通过。

---

### F-03 [MEDIUM] embedding.go — 空 text 无校验 ✅ FIXED

**Location**: `embedding.go:54-61` (`Embed`)

**Issue**: 空字符串直接发送 HTTP 请求，浪费网络往返。大多数 embedding API 对空输入返回错误，但应在客户端提前拦截。

**Risk**: MEDIUM — 不会崩溃，但浪费资源 + 产生不必要的错误日志。

**Fix**: 添加 `if strings.TrimSpace(text) == "" { return error }` 前置校验。

**Verification**: 已修复，编译通过。

---

### F-04 [LOW] segment_pure.go — Upsert 无维度校验

**Location**: `segment_pure.go:64-88` (`Upsert`)

**Issue**: 纯 Go fallback 的 `Upsert` 不检查 `len(denseVec) == col.dim`，接受任意维度向量。

**Mitigating factor**: `adapter.go` 的 `SegmentVectorIndex.Upsert` 在调用 store 前已验证 `len(vector) != s.dimension`，所以通过正常路径不会到达不匹配情况。

**Risk**: LOW — 防御层在 adapter 而非 store。

**Recommendation**: 接受。adapter 层校验足够，store 层不需要重复。

---

### F-05 [LOW] segment_cgo.go — Use-after-Close 返回 FFI 错误

**Location**: `segment_cgo.go:80-87` (`Close`) + 所有方法

**Issue**: `Close()` 后 `s.handle` 为 nil，后续方法调用传 nil handle 给 Rust FFI。

**Mitigating factor**: Rust FFI 层检查 null pointer (`FfiErrorCode::NullPointer`)，不会 crash。Go 侧 mutex 保证不会与 Close 并发。`DefaultManager.Close()` 设置 `closed = true` 后不再调用 vectorIndex 方法。

**Risk**: LOW — 正常关闭路径安全。

**Recommendation**: 接受。多层防护已足够。

---

### F-06 [LOW] embedding.go — 无重试逻辑

**Location**: `embedding.go:70-125, 128-177` (`embedOpenAI`, `embedOllama`)

**Issue**: HTTP 调用失败（网络超时、速率限制）时无重试。

**Mitigating factor**: 调用方已有容错：
- `indexVector()` (异步 safeGo): 失败仅 slog.Warn，不影响记忆存储
- `searchByVector()`: 失败 fallback 到 FTS5
- `BuildContextBlock()`: 通过 `SearchMemories` 间接调用，向量搜索失败不阻塞

**Risk**: LOW — 优雅降级已覆盖。

**Recommendation**: 作为后续优化记录到 deferred。可添加 1 次重试 + 指数退避。

---

### F-07 [LOW] initUHMSVectorBackend — 未传递 EmbeddingModel/EmbeddingBaseURL

**Location**: `server.go:425` (`NewHTTPEmbeddingProvider(embProvider, "", "", apiKey)`)

**Issue**: model 和 baseURL 参数传空字符串，依赖 provider 默认值。`MemoryUHMSConfig` 缺少 `EmbeddingModel` 和 `EmbeddingBaseURL` 配置字段，用户无法自定义 embedding 模型。

**Mitigating factor**: 默认值覆盖主流场景（OpenAI text-embedding-3-small, Ollama nomic-embed-text）。

**Risk**: LOW — 功能缺失，非 bug。

**Recommendation**: 后续在 `MemoryUHMSConfig` 添加 `EmbeddingModel`/`EmbeddingBaseURL` 字段。记录到 deferred。

---

### F-08 [INFO] manager.go — lastSummary 注释已过时 ✅ FIXED

**Location**: `manager.go:55-56`

**Issue**: 注释说 "Not persisted — resets on gateway restart"，但实际已持久化。

**Fix**: 更新注释为 "Persisted to VFS root as .last_summary; restored on gateway restart."

**Verification**: 已修复。

---

### F-09 [INFO] segment_cgo.go — LDFLAGS 路径依赖项目布局

**Location**: `segment_cgo.go:9-11`

**Issue**: `${SRCDIR}/../../../../../cli-rust/libs/openviking-rs/target/release` 硬编码 5 层相对路径。项目重组会导致编译失败。

**Risk**: INFO — monorepo 布局稳定，可接受。

---

### F-10 [INFO] adapter.go — memoryCollections 硬编码而非引用 AllMemoryTypes

**Location**: `adapter.go:22-28`

**Issue**: collection 名称硬编码 `["mem_episodic", ...]` 而非从 `uhms.AllMemoryTypes` 动态生成。如果 `types.go` 添加新 MemoryType，这里不会自动同步。

**Mitigating factor**: 新增 MemoryType 需要全项目搜索 `AllMemoryTypes` 并同步更新。5 种类型已稳定数年。

**Risk**: INFO — 低频变更风险。

**Recommendation**: 可选改进：`for _, mt := range uhms.AllMemoryTypes { collections = append(collections, "mem_"+string(mt)) }`。不阻塞归档。

---

## Verdict: **PASS**

| 级别 | 数量 | 已修复 | 待修复 |
|------|------|--------|--------|
| HIGH | 1 | 1 ✅ | 0 |
| MEDIUM | 2 | 2 ✅ | 0 |
| LOW | 4 | 0 | 0 (接受) |
| INFO | 3 | 1 ✅ | 0 (接受) |

**全部 HIGH/MEDIUM 发现已修复。** LOW/INFO 发现有充分的缓解措施，不阻塞归档。

### 安全检查

- [x] 无路径穿越风险（dataDir 从 config 构建，不接受用户输入）
- [x] 无信息泄露（API key 仅在 Authorization header，不在日志/错误中）
- [x] 输入验证完整（dimension、empty text、empty dataDir）
- [x] 资源释放正确（Close → store.Close → FFI free, 在 DefaultManager.Close 调用链中）
- [x] 并发安全（mutex 保护 FFI 调用，dimOnce 保护 dimension 写入）

### 正确性检查

- [x] CGO_ENABLED=0 完整编译通过
- [x] VectorMode=off 时零行为变更
- [x] 5 个 memory type collection 与 `AllMemoryTypes` 对齐
- [x] VectorHit.Score 类型转换正确（float32 → float64）
- [x] lastSummary 持久化/恢复路径正确

### 资源安全检查

- [x] SegmentStore handle 在 Close 中释放，DefaultManager.Close 触发
- [x] HTTP client 有 30s 超时
- [x] Response body 有 10 MiB 读取限制
- [x] safeGo 包装防止 goroutine panic 崩溃进程
