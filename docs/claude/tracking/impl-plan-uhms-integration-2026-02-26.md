---
document_type: Tracking
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-uhms-p3-runner-gateway.md
skill5_verified: true
---

# UHMS 集成到 OpenAcosmi — 实施方案

## Context

OpenAcosmi 的 agent 工具循环 (`attempt_runner.go:131-350`) 中消息线性累积，无中间压缩，导致 token 爆炸（50 轮工具调用可达 300K+ tokens）。现有剪枝 (`context_pruning.go`) 仅基于 TTL 的软修剪/硬清除，无 LLM 摘要能力。

目标：将已复制的 UHMS 记忆系统（Go API 156 文件 + Rust FFI 15 crates）**内嵌**为 `backend/internal/memory/uhms/` 包，实现全局上下文压缩 + 长期记忆。

部署模型：**本地存储 + 本地向量 + 云端 LLM 计算**。数据不出本地，仅文本片段发送云端推理。

---

## 在线验证摘要 (Skill 5)

| 主题 | 来源 | 关键发现 |
|------|------|----------|
| Claude Compaction API | [docs](https://platform.claude.com/docs/en/build-with-claude/compaction) | Beta 2026-01, 服务端摘要, 150K 触发阈值, 自定义摘要指令 |
| Claude Prompt Caching | [docs](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) | 85% 延迟降低, 5min/1h TTL, 系统提示缓存 |
| OpenViking L0/L1/L2 | [github](https://github.com/volcengine/OpenViking) | ~95% token 降低(L0 vs L2), 文件系统范式 |
| MemGPT/Letta | [paper](https://arxiv.org/abs/2310.08560) | OS 式分页, 70%/100% 驱逐, 递归摘要 |
| Letta Context Repos | [blog](https://www.letta.com/blog/context-repositories) | 文件系统+git版本控制, sleep-time consolidation |
| LLMLingua-2 | [github](https://github.com/microsoft/LLMLingua) | BERT级token分类, 20x压缩, 3-6x快于v1 |
| Qdrant Segment | [docs](https://qdrant.tech/documentation/) | 无Rust嵌入模式; 通过FFI使用segment库(UHMS已实现) |
| Go↔Rust FFI | [rust2go](https://en.ihcblog.com/rust2go/) | CGo ~28ns/call; JSON-Lines ~5.8μs; purego无需CGo |

---

## 分阶段计划

### P0: 基础存储层 — SQLite + 内存缓存 + 核心类型

**范围**: 从 UHMS 提取存储层，适配为嵌入式包。不触碰运行时代码。

**新建文件**:
- `backend/internal/memory/uhms/types.go` — 统一记忆类型
  - 基于 `local-proxy/internal/storage/store.go:24-80` 的 SQLite 模型 (Memory, Entity, Relation, CoreMemory, TreeNode)
  - 补充 UHMS 完整字段: DecayFactor, EmbeddingRef, RetentionPolicy, EventTime (双时态)
  - 常量: MemoryType (episodic/semantic/procedural/permanent/imagination), Category (preference/habit/profile/skill 等 13 种)
  - L0Entry, L1Entry 结构体 (渐进加载)

- `backend/internal/memory/uhms/store.go` — SQLite 存储层
  - 适配自 `local-proxy/internal/storage/store.go` (已用 `gorm.io/driver/sqlite`)
  - 表: memories, entities, relations, core_memory, tree_nodes, decay_profiles
  - WAL 模式, 自动迁移
  - 路径: `~/.openacosmi/memory/uhms.db`

- `backend/internal/memory/uhms/cache.go` — 纯内存 LRU 缓存 (替代 Redis)
  - 接口: Get/Set/Delete, TTL 驱逐, 默认 1000 条

- `backend/internal/memory/uhms/interfaces.go` — 核心接口
  - `MemoryStore` (CRUD + 搜索)
  - `EmbeddingProvider` (适配已有 embedding 提供者)
  - `LLMProvider` (适配已有 agents/llmclient)
  - `VectorIndex` (抽象 segment store vs 纯Go余弦)

- `backend/pkg/types/types_memory.go` — 扩展 MemoryConfig (L68)
  - 新增 `UHMS *UHMSConfig` 字段
  - UHMSConfig: Enabled, DBPath, EmbeddingMode, CompressionThreshold(200000), DecayEnabled, TieredLoadingEnabled

**验证**: 单元测试 Store CRUD; 纯Go余弦搜索 1000 条 < 50ms

---

### P1: 嵌入向量 + 向量搜索

**范围**: 集成 embedding 路由 + Rust FFI 余弦搜索，实现语义检索。

**新建文件**:
- `backend/internal/memory/uhms/embedding.go` — Embedding 路由
  - 适配自 `local-proxy/internal/embedding/router.go`
  - 优先级: Ollama → Cloud (复用已有 agents/llmclient 的 embedding 能力)
  - 适配器桥接 OpenAcosmi 已有的 EmbeddingProvider 接口

- `backend/internal/memory/uhms/vector_index.go` — 向量搜索
  - 双后端: PureGoIndex (兜底) + FFIIndex (SIMD余弦)
  - 混合搜索: dense vector + BM25 (SQLite FTS5)
  - 多集合路由: episodic→mem_episodic, semantic→mem_semantic 等

- `backend/internal/memory/uhms/ffi_bridge_cgo.go` — CGo FFI (build tag: `cgo`)
  - 复制 `go-api/internal/ffi/vector.go` 的 CGo 模式
  - LDFLAGS: `-L${SRCDIR}/../../../cli-rust/libs/nexus-core/target/release -lnexus_unified -lm`
  - 函数: CosineSimilarity, BatchCosine, JiebaTokenize, BM25Score

- `backend/internal/memory/uhms/ffi_bridge_pure.go` — 纯Go兜底 (build tag: `!cgo`)
  - 匹配 `go-api/internal/ffi/pure_vector.go` 模式

- `scripts/build-nexus-ffi.sh` — Rust 构建脚本
  - `cargo build --release` in `cli-rust/libs/nexus-core/`
  - 生成 `libnexus_unified.a` 供 CGo 链接

**验证**: 集成测试 embed→store→search 全流程; FFI vs 纯Go性能对比

---

### P2: 记忆管理器核心 — 增/搜/衰减/提交

**范围**: 移植 UHMS MemoryManager 编排逻辑为嵌入式服务。

**新建文件**:
- `backend/internal/memory/uhms/manager.go` — 核心管理器
  - 适配自 `go-api/internal/services/memory_manager.go`
  - AddMemory: embed→dedup→classify→store→index
  - SearchMemories: embed→vector search→BM25 rerank→tiered load
  - CommitSession: transcript JSONL → LLM 摘要 → 6 类记忆提取 → 去重入库
  - BuildContextBlock: 为 P3 准备的上下文构建接口

- `backend/internal/memory/uhms/dedup.go` — 记忆去重管道
  - 三阶段: hash精确匹配 → 向量余弦相似 → LLM 语义判断
  - 决策: add/update/merge/noop

- `backend/internal/memory/uhms/decay.go` — FSRS-6 衰减
  - 适配自 `go-api/internal/services/memory_decay.go`
  - 指数衰减 + 访问频率提升 + 自适应半衰期
  - 后台 ticker: 6h 批量衰减

- `backend/internal/memory/uhms/tiered_loader.go` — L0/L1/L2 渐进加载
  - L0 (~100tk): 1-2 句摘要
  - L1 (~2Ktk): 中等详情
  - L2: 完整内容 (按需)
  - Token budget 控制

- `backend/internal/memory/uhms/graph.go` — 简化知识图谱
  - 实体提取 (LLM) → SQLite 存储
  - 关系图遍历增强搜索

- `backend/internal/memory/uhms/session_committer.go` — 会话→记忆提取
  - 适配自 `go-api/internal/services/memory_session_committer.go`
  - 输入: JSONL transcript (via `session/manager.go:LoadAllEntries()`)
  - 三步: 归档摘要 → 分类提取 → 去重入库

**关键接口**:
```go
type Manager interface {
    AddMemory(ctx, content, memType, category, userID) (*Memory, error)
    SearchMemories(ctx, query, userID, opts) ([]SearchResult, error)
    CommitSession(ctx, transcript, userID) (*CommitResult, error)
    BuildContextBlock(ctx, query, userID, tokenBudget) (string, error)
    RunDecayCycle(ctx, userID) error
    Close() error
}
```

**验证**: 单测 AddMemory 去重; CommitSession 端到端; 衰减周期验证

---

### P3: Runner 集成 — 上下文压缩中间件 (最高价值)

**范围**: 将 UHMS 接入 attempt_runner.go 工具循环，实现全局上下文压缩。

**注入点**: `attempt_runner.go:139` — `StreamChat()` 调用前

**新建文件**:
- `backend/internal/memory/uhms/context_middleware.go` — 压缩中间件
  - `CompressIfNeeded(messages, tokenBudget) → compressedMessages`
  - 压缩策略 (按激进度递进):
    1. 工具结果剪枝 (复用已有 context_pruning.go TTL 逻辑)
    2. 旧对话摘要: token > 200K 时, LLM 摘要旧消息块
    3. 记忆提取: 摘要的内容 → UHMS store 持久化
    4. 记忆注入: 搜索 UHMS 获取相关上下文 → 注入压缩后消息
  - 压缩后消息结构:
    ```
    [system_prompt]
    [memory_context_block]      ← UHMS 注入
    [conversation_summary]      ← LLM 摘要
    [recent_messages (last 3-5)] ← 原样保留
    ```

- `backend/internal/memory/uhms/claude_integration.go` — Claude API 特化
  - Prompt Caching: system prompt + skills 设置 `cache_control: ephemeral`
  - Compaction API: provider==Anthropic 时使用服务端压缩 (150K 触发)
  - 非 Anthropic provider: 本地 LLM 摘要压缩

**修改文件**:
- `backend/internal/agents/runner/attempt_runner.go`
  - 新增字段: `uhmsManager uhms.Manager` (可选, nil=禁用)
  - 工具循环 L131 前注入:
    ```go
    if r.uhmsManager != nil {
        messages = r.uhmsManager.CompressIfNeeded(ctx, messages, tokenBudget)
    }
    ```
  - 工具循环结束后: `r.uhmsManager.CommitSession(ctx, messages, userID)`

- `backend/internal/gateway/boot.go`
  - 初始化 UHMS Manager (alongside sandbox/argus bridge)
  - 新增 `uhmsManager` 字段到 `GatewayState`

- `backend/internal/gateway/server_methods_memory.go` — 新增 RPC
  - `memory.uhms.status` — UHMS 状态
  - `memory.uhms.search` — 手动搜索记忆
  - `memory.uhms.compress` — 手动触发压缩
  - `memory.uhms.commit` — 手动提交会话

- `backend/internal/gateway/ws_server.go`
  - 新增事件: `memory.compressed`, `memory.committed`

**数据流**:
```
工具循环 第N轮:
  messages = [... +assistant +tool_results]  ← 线性增长
      ↓
  CompressIfNeeded(messages, 200K)
      ↓
  EstimateTokens < 200K? → 直通 (无操作)
      ↓
  EstimateTokens >= 200K?
      ├─ 1. 剪枝旧工具结果 (TTL)
      ├─ 2. LLM 摘要旧对话块
      ├─ 3. 提取记忆 → UHMS 持久化
      ├─ 4. 搜索 UHMS → 注入相关上下文
      └─ 5. 返回压缩消息 (~80K tokens)
      ↓
  StreamChat(compressed_messages) → LLM
```

**Token 影响估算**:
| 场景 | 无 UHMS | 有 UHMS |
|------|---------|---------|
| 10 轮对话 | ~20K | ~20K (无需压缩) |
| 50 轮工具密集 | ~300K | ~80K |
| 100 轮 + 重复上下文 | ~800K | ~120K |

**验证**: CompressIfNeeded 输入 300K → 输出 < 100K; 50 轮工具循环 token 有界; 现有 137 测试零回归

---

### P4: CLI 集成 + Agent 记忆工具

**范围**: 通过 CLI 和 agent 工具暴露 UHMS 功能。

**修改文件**:
- `cli-rust/crates/oa-cmd-memory/src/lib.rs` — 扩展子命令
  - `memory uhms status/search/add/commit/compress/export`
  - 复用已有 `oa-gateway-rpc` 模式

**新建文件**:
- `backend/internal/agents/runner/tools_memory.go` — Agent 记忆工具
  - `memory_search`: Agent 搜索过去记忆
  - `memory_add`: Agent 主动保存重要信息
  - `memory_recall`: Agent 按 ID 召回记忆

**修改文件**:
- `backend/internal/agents/runner/attempt_runner.go`
  - `buildToolDefinitions()`: 如果 uhmsManager != nil, 注册记忆工具
  - `ExecuteToolCall`: 处理 memory_search/add/recall

**验证**: E2E 测试 `openacosmi memory uhms status`; Agent 使用 memory_search 工具

---

### P5: Rust FFI 构建系统 + CI

**范围**: 跨平台构建 + CI 自动化。

**新建文件**:
- `.github/workflows/uhms-ci.yml` — CI 流水线
  - macOS arm64/x86_64 + Linux x86_64
  - Rust: `cargo build --release` nexus-core
  - Go: CGo 构建 + 纯 Go 兜底构建
  - 测试: Rust 单测 + Go 集成测试

- `scripts/build-nexus-ffi.sh` — FFI 构建脚本

**验证**: 3 平台构建成功; 纯Go构建无需 Rust 工具链; FFI vs 纯Go 10K向量性能对比

---

## 依赖图

```
P0 (存储) ───────────────────────────┐
    │                                 │
    ↓                                 │
P1 (向量搜索) ──────────┐            │
    │                    │            │
    ↓                    ↓            ↓
P2 (记忆管理器) ───→ P3 (Runner集成) ← 最高价值
                         │
                    ┌────┴────┐
                    ↓         ↓
              P4 (CLI+工具) P5 (CI)
```

P0/P1 可并行开发。P3 是最高价值阶段。P4/P5 在 P3 后可并行。

---

## 风险评估

| 风险 | 级别 | 缓解 |
|------|------|------|
| LLM 压缩调用成本爆炸 | HIGH | Anthropic 用原生 Compaction API (无额外成本); 非 Anthropic 批量摘要 + 缓存 |
| CGo 构建复杂度 | MEDIUM | 所有 FFI 函数均有纯 Go 兜底; CGo 仅用于性能加速 |
| SQLite 并发写竞争 | MEDIUM | WAL 模式 + 单写 goroutine + 写操作低频 (会话提交/衰减) |
| GORM 依赖冲突 | MEDIUM | UHMS 用 gorm.io/driver/sqlite, 已有代码用 raw sql; 分包隔离 |
| Embedding 不可用 | LOW | 降级链: Ollama → Cloud → 纯 BM25 (FTS5); 无向量仅精度降低 |

---

## 关键源文件参考

| 文件 | 用途 |
|------|------|
| `backend/internal/agents/runner/attempt_runner.go:139` | P3 注入点: StreamChat 前压缩 |
| `backend/internal/memory/local-proxy/internal/storage/store.go` | P0 模板: SQLite 模型 |
| `backend/internal/memory/go-api/internal/ffi/vector.go` | P1 模板: CGo FFI 模式 |
| `backend/internal/memory/go-api/internal/services/memory_manager.go` | P2 源: MemoryManager 编排 |
| `backend/internal/memory/go-api/internal/services/memory_session_committer.go` | P2 源: 会话提交 |
| `backend/internal/memory/go-api/internal/services/memory_decay.go` | P2 源: FSRS-6 衰减 |
| `backend/internal/agents/extensions/context_pruning.go` | P3 复用: TTL 剪枝逻辑 |
| `backend/pkg/types/types_memory.go:68` | P0 扩展: MemoryConfig |
| `backend/internal/gateway/boot.go` | P3 初始化: UHMS Manager |
| `cli-rust/libs/nexus-core/` | P1/P5: Rust FFI 源码 |
