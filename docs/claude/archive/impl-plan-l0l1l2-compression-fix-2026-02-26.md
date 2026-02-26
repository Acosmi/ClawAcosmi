# L0/L1/L2 分布式 3 层压缩补全 — 实施计划

---
document_type: Archive
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-l0l1l2-compression-fix.md
skill5_verified: true
---

## Context

UHMS 记忆系统的 L0/L1/L2 分层架构（参考 OpenViking VikingFS `.abstract.md`/`.overview.md`/content 模式）在**记忆存储**和**记忆检索**路径已完整实现，但以下 4 个关键环节断裂：

1. **会话归档缺 L2**: `commitSession()` 只生成 L0/L1 摘要，原始对话全文**未持久化**。跨会话恢复时丢失原始上下文。
2. **实时压缩与 L0/L1/L2 脱节**: `CompressIfNeeded()` 生成的 summary 存入 `.last_summary` 单一文件，**不写入 VFS L0/L1/L2 结构**，无法被 `BuildContextBlock()` 渐进加载检索到。
3. **子智能体零记忆**: oa-coder/argus 无状态 MCP server，不接入 UHMS。主智能体的上下文信息无法传播给子智能体。
4. **Token 估算粗糙**: Go 侧用 `len([]rune)*2/3`（±40% 误差），导致压缩触发时机不准。Rust 生态有精确 BPE tokenizer 可通过 FFI 复用。

**影响范围**: 压缩效率低（误触发/延迟触发）、跨会话恢复质量差、子智能体每次从零开始、会话全文不可追溯。

## Online Verification Log

### ACON: Agent Context Optimization (NeurIPS 2025)
- **Query**: "LLM context compression tiered summarization L0 L1 L2 progressive loading best practice 2025"
- **Source**: https://arxiv.org/html/2510.00615v1
- **Key finding**: ACON 压缩环境观测+交互历史，降低 26-54% peak tokens 同时保持任务性能。动态分层压缩优于均匀压缩。70% token budget 时触发压缩为最佳实践。
- **Verified date**: 2026-02-26

### Letta/MemGPT: Recall + Archival 双层存储
- **Query**: "session transcript archival full text L2 storage conversation recovery long-term memory LLM agent 2025"
- **Source**: https://www.letta.com/blog/agent-memory + https://arxiv.org/pdf/2504.19413
- **Key finding**: MemGPT 设计"recall storage"(全文检索) + "archival storage"(语义向量)双层。全文保留是跨会话恢复的基础，L0/L1 摘要加速检索但不能替代 L2 原文。
- **Verified date**: 2026-02-26

### Multi-Agent Memory: Hybrid Topology
- **Query**: "LLM agent sub-agent memory sharing context propagation multi-agent memory architecture 2025"
- **Source**: https://arxiv.org/html/2505.18279v1 + https://arxiv.org/abs/2508.08997
- **Key finding**: Hybrid topology 中 orchestrator 维护 global context (L0 summary)，worker agents 维护 local context。选择性传播相关上下文，避免淹没子智能体 context window。36.9% 多智能体失败归因于 inter-agent misalignment。
- **Verified date**: 2026-02-26

### tiktoken-rs / bpe: Rust 精确 Token 计数
- **Query**: "Rust tiktoken BPE tokenizer library fast token counting 2025"
- **Source**: https://crates.io/crates/tiktoken-rs + https://github.com/github/rust-gems/blob/main/crates/bpe/README.md
- **Key finding**: `tiktoken-rs` 支持 cl100k_base (GPT-4/Claude 兼容)，`bpe` crate (GitHub) 比 HuggingFace 快 10x。O(n) 复杂度。可通过 staticlib + C ABI 导出给 Go CGO。
- **Verified date**: 2026-02-26

### OpenViking VikingFS L0/L1 原生设计
- **Query**: 源码分析 `openviking-vfs/src/viking_fs.rs`
- **Source**: 本地 `cli-rust/libs/openviking-rs/openviking-vfs/src/viking_fs.rs:178-440`
- **Key finding**: VikingFS 原生支持 `write_context(uri, content, abstract_text, overview, filename)` — L0 写入 `.abstract.md`，L1 写入 `.overview.md`，L2 写入 `{content_filename}`。`batch_read(uris, "l0"/"l1")` 支持批量渐进读取。Go 侧 UHMS VFS 的 `l0.txt/l1.txt/l2.txt` 模式与此对齐。
- **Verified date**: 2026-02-26

---

## Phase 1: 会话归档 L2 全文持久化 (~50 LOC Go) ✅

**目标**: `commitSession()` 同时保存 L2 原始对话全文到 VFS。

### 1.1 VFS WriteArchive 扩展

**文件**: `backend/internal/memory/uhms/vfs.go`

在 `WriteArchive()` 增加 L2 参数:

```go
// 当前签名:
func (v *LocalVFS) WriteArchive(userID, sessionKey, l0Summary, l1Overview string) (string, error)

// 修改为:
func (v *LocalVFS) WriteArchive(userID, sessionKey, l0Summary, l1Overview, l2Transcript string) (string, error)
// 新增: 写入 l2.txt (对话全文)
```

### 1.2 session_committer 调用更新

**文件**: `backend/internal/memory/uhms/session_committer.go`

```go
// 当前 (L27-36): 只传 l0, l1
archivePath, err := m.vfs.WriteArchive(userID, sessionKey, l0Summary, l1Overview)

// 修改为: 同时传 L2 全文
archivePath, err := m.vfs.WriteArchive(userID, sessionKey, l0Summary, l1Overview, conversationText)
```

### 1.3 L2 大小限制

L2 全文可能很大 (50K+ tokens)。添加截断保护:
- 最大 200KB 原文 (~50K tokens)
- 超过则截断尾部 + 附加 `\n[Transcript truncated at 200KB]`

### Phase 1 文件清单

| 文件 | 变更 |
|------|------|
| `uhms/vfs.go` | WriteArchive +l2Transcript 参数, 写 l2.txt |
| `uhms/session_committer.go` | 传 conversationText 给 WriteArchive |

---

## Phase 2: 压缩结果写入 VFS L0/L1/L2 (~100 LOC Go) ✅

**目标**: `CompressIfNeeded()` 产生的 summary 同步写入 VFS，使压缩产物可被 `BuildContextBlock()` 在下次（跨会话）检索。

### 2.1 压缩时写入会话中间存档

**文件**: `backend/internal/memory/uhms/manager.go`

在 `CompressIfNeeded()` 生成 summary 后 (约 L450):

```go
// 现有: 持久化 lastSummary 到 .last_summary
m.persistLastSummary(summary)

// 新增: 同时写入 VFS 结构化存档 (异步, non-blocking)
safeGo("compress_archive_"+sessionKey, func() {
    // L0: summary 截取前 200 字
    l0 := truncate(summary, 200)
    // L1: summary 全文 (结构化 7 段)
    l1 := summary
    // L2: 被压缩的旧消息原文
    l2 := buildTranscriptText(toCompress)
    archiveKey := fmt.Sprintf("compress_%s_%d", sessionKey, time.Now().UnixMilli())
    m.vfs.WriteArchive(userID, archiveKey, l0, l1, l2)
})
```

### 2.2 压缩存档作为 semantic 记忆入库

压缩产生的 summary 作为 `semantic/summary` 类型记忆入库:

```go
// 在 extractAndStoreMemories 之外，直接存一条 summary 记忆
safeGo("compress_memory", func() {
    m.AddMemory(ctx, userID, summary, MemTypeSemantic, CatSummary)
})
```

需在 `types.go` 添加 `CatSummary MemoryCategory = "summary"`（如不存在）。

### Phase 2 文件清单

| 文件 | 变更 |
|------|------|
| `uhms/manager.go` | CompressIfNeeded() +VFS 写入 +记忆入库 |
| `uhms/types.go` | +CatSummary (如不存在) |

---

## Phase 3: Rust 精确 Token 计数 FFI (~250 LOC Rust+Go) ✅

**目标**: 用 tiktoken-rs (cl100k_base) 替换 Go 侧粗糙估算，通过 FFI 提供精确 token 计数。

**实施说明**: 由于 Go `internal` 循环导入限制 (uhms→vectoradapter→uhms)，tokenizer 文件放在 `uhms/` 包内 (`tokenizer.go` + `tokenizer_cgo.go` + `tokenizer_pure.go`)，而非 `vectoradapter/`。

### 3.1 Rust FFI 导出

**文件**: `cli-rust/libs/openviking-rs/openviking-ffi/src/tokenizer_api.rs` (新建)

```rust
//! FFI exports for BPE token counting.
//! Uses tiktoken-rs cl100k_base encoding (compatible with GPT-4/Claude).

use tiktoken_rs::cl100k_base;
use std::sync::LazyLock;

static TOKENIZER: LazyLock<tiktoken_rs::CoreBPE> = LazyLock::new(|| {
    cl100k_base().expect("failed to load cl100k_base")
});

/// Count tokens in a UTF-8 text string.
/// Returns token count, or -1 on error.
#[no_mangle]
pub unsafe extern "C" fn ovk_token_count(
    text: *const u8,
    text_len: usize,
) -> i32 {
    if text.is_null() || text_len == 0 { return 0; }
    let slice = unsafe { std::slice::from_raw_parts(text, text_len) };
    match std::str::from_utf8(slice) {
        Ok(s) => TOKENIZER.encode_ordinary(s).len() as i32,
        Err(_) => -1,
    }
}

/// Truncate text to fit within max_tokens, respecting UTF-8 boundaries.
/// Returns actual byte length of truncated text.
#[no_mangle]
pub unsafe extern "C" fn ovk_token_truncate(
    text: *const u8,
    text_len: usize,
    max_tokens: usize,
    out_byte_len: *mut usize,
) -> i32 { /* ... */ }
```

### 3.2 Cargo.toml 依赖

**文件**: `cli-rust/libs/openviking-rs/openviking-ffi/Cargo.toml`

```toml
[dependencies]
tiktoken-rs = "0.6"   # cl100k_base BPE tokenizer
```

### 3.3 Go CGO 绑定

**文件**: `backend/internal/memory/uhms/vectoradapter/tokenizer_cgo.go` (新建)

```go
//go:build cgo

package vectoradapter

/*
#cgo LDFLAGS: -L... -lopenviking_ffi
extern int ovk_token_count(const unsigned char* text, unsigned long text_len);
*/
import "C"

// CountTokens returns exact BPE token count via Rust FFI.
func CountTokens(text string) int {
    if len(text) == 0 { return 0 }
    b := []byte(text)
    n := C.ovk_token_count((*C.uchar)(&b[0]), C.ulong(len(b)))
    if n < 0 { return len([]rune(text)) * 2 / 3 } // fallback
    return int(n)
}
```

**文件**: `backend/internal/memory/uhms/vectoradapter/tokenizer_pure.go` (新建)

```go
//go:build !cgo

package vectoradapter

// CountTokens fallback: ~4 chars/token (English), ~1.5 chars/token (Chinese).
func CountTokens(text string) int {
    return len([]rune(text)) * 2 / 3
}
```

### 3.4 UHMS Manager 接入

**文件**: `backend/internal/memory/uhms/manager.go`

替换 `estimateTokens()`:

```go
func (m *DefaultManager) estimateTokens(text string) int {
    if m.llm != nil {
        return m.llm.EstimateTokens(text)
    }
    return vectoradapter.CountTokens(text) // Rust FFI (精确) 或 纯 Go (fallback)
}
```

### Phase 3 文件清单

| 文件 | 操作 | LOC |
|------|------|-----|
| `openviking-ffi/src/tokenizer_api.rs` | 新建, Rust FFI | ~80 |
| `openviking-ffi/src/lib.rs` | +pub mod tokenizer_api | ~1 |
| `openviking-ffi/Cargo.toml` | +tiktoken-rs 依赖 | ~1 |
| `uhms/vectoradapter/tokenizer_cgo.go` | 新建, CGO 绑定 | ~40 |
| `uhms/vectoradapter/tokenizer_pure.go` | 新建, 纯 Go fallback | ~15 |
| `uhms/manager.go` | estimateTokens() 替换 | ~5 |

---

## Phase 4: 子智能体上下文传播 (~150 LOC Go) ✅

**目标**: 主智能体将 L0 级别的会话上下文摘要注入子智能体调用，减少 inter-agent misalignment。

**实施说明**: `BuildContextBrief()` 直接从 `lastSummary` 提取，不需要传入 recentMessages。简化了接口签名为 `BuildContextBrief(ctx, userID)` 和 `BuildContextBrief(ctx)` (bridge)。

### 4.1 Context Brief 生成

**文件**: `backend/internal/memory/uhms/manager.go`

新增方法:

```go
// BuildContextBrief 生成 L0 级别的上下文简报 (~200 tokens)，
// 用于注入子智能体 (coder/argus) 的 system prompt。
// 包含: 当前任务目标 + 最近修改的文件 + 关键决策。
func (m *DefaultManager) BuildContextBrief(ctx context.Context, userID string, recentMessages []Message) string {
    // 1. 如果有 lastSummary, 从中提取 Session Intent + Files Modified
    // 2. 如果没有, 从 recentMessages 中截取关键信息
    // 3. 限制到 ~200 tokens
}
```

### 4.2 UHMSBridge 接口扩展

**文件**: `backend/internal/agents/runner/attempt_runner.go`

```go
type UHMSBridgeForAgent interface {
    CompressChatMessages(...) (...)
    CommitChatSession(...) error
    // 新增:
    BuildContextBrief(ctx context.Context, recentMessages []llmclient.ChatMessage) string
}
```

### 4.3 Coder Tool 调用注入

**文件**: `backend/internal/gateway/runner/tool_executor.go` 或 coder bridge

在 coder tool 调用前注入 context brief:

```go
// 现有: 直接调用 coder tool
result := coderBridge.CallTool(toolName, args)

// 修改: 注入 context brief 到 args (如果是 bash/edit/write 类工具)
if r.UHMSBridge != nil {
    brief := r.UHMSBridge.BuildContextBrief(ctx, messages)
    if brief != "" {
        args["_context_brief"] = brief  // coder server 可选读取
    }
}
```

### 4.4 网关 Bridge Adapter 实现

**文件**: `backend/internal/gateway/server.go`

```go
func (a *uhmsBridgeAdapter) BuildContextBrief(ctx context.Context, messages []llmclient.ChatMessage) string {
    umsgs := chatMessagesToUHMS(messages)
    return a.m.BuildContextBrief(ctx, a.defaultUserID(), umsgs)
}
```

### Phase 4 文件清单

| 文件 | 变更 |
|------|------|
| `uhms/manager.go` | +BuildContextBrief() 方法 |
| `agents/runner/attempt_runner.go` | +UHMSBridgeForAgent.BuildContextBrief |
| `gateway/server.go` | +uhmsBridgeAdapter.BuildContextBrief |
| `gateway/runner/tool_executor.go` | +coder 调用前注入 |

---

## Phase 5: 默认配置激活 + 架构文档更新 (~40 LOC) ✅

**目标**: 启用 observation masking 和优化触发阈值，更新架构文档。

### 5.1 默认配置调优

**文件**: `backend/internal/memory/uhms/config.go`

```go
// 当前默认:
// CompressionTriggerPercent: 0  (legacy, 到 200K 才触发)
// ObservationMaskTurns: 0       (关闭)

// 修改为:
// CompressionTriggerPercent: 75  (75% budget 时触发, 参考 ACON 最佳实践)
// ObservationMaskTurns: 3        (保留最近 3 轮 user turn 完整输出)
```

### 5.2 向后兼容

零值仍保持 legacy 行为（已有 `ResolvedTriggerPercent()` / `ResolvedKeepRecent()` guard）。仅修改 `DefaultUHMSConfig()` 初始值，用户已设值不受影响。

### 5.3 架构文档更新

**文件**: `docs/claude/goujia/arch-uhms-memory-system.md`

更新:
- 会话归档新增 L2 段落
- 压缩-VFS 桥段落
- 子智能体上下文传播段落
- Rust token 计数器段落
- 默认配置变更记录

### Phase 5 文件清单

| 文件 | 变更 |
|------|------|
| `uhms/config.go` | DefaultUHMSConfig 调优 |
| `docs/claude/goujia/arch-uhms-memory-system.md` | 架构文档更新 |

---

## 依赖关系

```
P1 (L2 Archive)     → 独立
P2 (Compress→VFS)   → 需 P1 (WriteArchive 新签名)
P3 (Rust Tokenizer) → 独立
P4 (Sub-Agent Brief) → 独立
P5 (Config + Docs)  → 需 P1-P4 完成
```

**执行顺序**: P1 → P2 (串行) | P3 | P4 (并行) → P5

---

## 总量

| Phase | 描述 | 新建文件 | 修改文件 | LOC |
|-------|------|---------|---------|-----|
| P1 | 会话 L2 全文归档 | 0 | 2 | ~50 |
| P2 | 压缩→VFS 桥 | 0 | 2 | ~100 |
| P3 | Rust Token 计数 FFI | 3 (Rust+Go) | 3 | ~250 |
| P4 | 子智能体上下文传播 | 0 | 4 | ~150 |
| P5 | 默认配置 + 架构文档 | 0 | 2 | ~40 |
| **总计** | | **3 新建** | **~10 修改** | **~590 LOC** |

---

## 验证

1. `CGO_ENABLED=0 go build ./...` 全量编译通过 (P3 纯 Go fallback)
2. `commitSession()` 后 VFS 目录包含 `l0.txt` + `l1.txt` + `l2.txt`
3. `CompressIfNeeded()` 后 VFS archives 目录包含 `compress_*` 中间存档
4. Rust `ovk_token_count("Hello world")` 返回 2 (精确值)
5. Go `vectoradapter.CountTokens("你好世界")` 返回精确 token 数
6. Coder tool 调用时 args 包含 `_context_brief` 字段
7. `BuildContextBrief()` 返回 ~200 tokens L0 摘要
8. `ObservationMaskTurns=3` 时压缩减少 60%+ tokens
9. `CompressionTriggerPercent=75` 时在 150K tokens 触发压缩 (budget=200K)
10. Gateway 重启: 已归档的 L2 全文可通过 `ReadL2` 恢复
