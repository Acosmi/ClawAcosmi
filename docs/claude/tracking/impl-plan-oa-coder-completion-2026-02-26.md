---
document_type: Tracking
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-oa-coder-completion-p1p2.md
skill5_verified: true
---

# oa-coder 全量修复补全计划

> 审计延迟修复 + codesearch/websearch 新工具 + 可配置超时 + 前端配置向导

## Skill 5 验证摘要

| 主题 | 源 | 结论 |
|------|-----|------|
| 原子写入 | docs.rs/tempfile | `NamedTempFile::persist()` 跨平台原子替换 |
| 代码搜索 | GitHub REST API docs | `GET /search/code` 10req/min, 需 token |
| 网页搜索 | Tavily/Exa/Brave/SearXNG | OpenAPI 标准模式，用户自选 provider + endpoint |
| Levenshtein | VS Code filters.ts | VS Code 限 128 字符，我们用 10K + 快速拒绝 |
| MCP 超时 | MCP spec 2025-11-25 | TS SDK 默认 60s，per-tool 分级覆盖 |
| JSON-RPC | jsonrpc.org/specification | 8 条必须验证规则 |

## 依赖图

```
Phase 1 (审计修复, Rust)  ─────────────────────────┐
    [无依赖, 可与 P2 并行]                          │
                                                    ├──→ Phase 7 (审计+归档)
Phase 2 (可配置超时, Go) ──→ Phase 3 (codesearch)   │
                    │                               │
                    ├──→ Phase 4 (websearch) ────────┤
                    │                               │
                    └──→ Phase 5 (Schema+Hints) ────┤
                                   │                │
                                   └──→ Phase 6 (UI)┘
```

P1/P2 并行 → P3/P4 并行 → P5 → P6 → P7

---

## Phase 1: 审计 LOW/INFO 修复 (纯 Rust, ~240 LOC)

### 1A: 快速修复 (5 项)

- [x] **F-06**: Levenshtein DoS 防护 — `levenshtein.rs` 添加 `MAX_LEVENSHTEIN_INPUT = 10_000` + 快速拒绝
- [x] **F-14**: 序列化失败不再静默 — `server.rs:319` `.ok()` → `match` + `error!()` + `-32603`
- [x] **F-19**: 删除死代码 filetime 模块 — `util/filetime.rs` (22 行) + `util/mod.rs`
- [x] **F-21**: JSON-RPC `"2.0"` 版本校验 — `server.rs` 解析后校验 → `-32600`
- [x] **F-22**: 空 old_string + 已存在文件 — `tools/edit.rs` 添加 else 分支

### 1B: 中等修复 (3 项)

- [x] **F-07**: 二进制检测只读前 8KB — `tools/read.rs` `Read::take(8192)` 检测
- [x] **F-17**: grep 总数限制改为客户端计数 — `tools/grep.rs` 保留宽松限制 + 客户端截断
- [x] **F-18**: 正则提取到循环外 — `edit/replacers.rs` regex 编译从 per-line 移到循环前

### 1C: 较大重构 (2 项)

- [x] **F-11**: 原子写入 — 新增 `util/atomic.rs`, `tools/write.rs` + `tools/edit.rs` 调用
- [x] **F-23**: 移除未使用 tokio 依赖 — `Cargo.toml` (保留 reqwest)

### Phase 1 验证
- [x] `cargo test -p oa-coder` — 40/40 通过
- [x] `cargo clippy -p oa-coder` — 零 error (warnings 非阻塞)

---

## Phase 2: 可配置超时 (Go 后端, ~75 LOC)

- [x] 新增 `CoderDefaultsConfig` 类型 — `pkg/types/types_agent_defaults.go`
- [x] `tool_executor.go:684` 硬编码超时 → 从 params 读取 (默认 120s)
- [x] `attempt_runner.go` ToolExecParams 添加 CoderTimeoutSeconds + resolveCoderTimeoutSeconds()
- [~] `boot.go` 读 config 传 CLI args — **延后**: boot 在 config 加载前运行，需重构初始化顺序
- [~] `server.go` 从 loadedCfg 提取超时传入 — **不需要**: 超时通过 ToolExecParams 运行时注入

### Phase 2 验证
- [x] `go build ./...` 通过
- [x] `go test ./internal/agents/runner/...` 通过

---

## Phase 3: codesearch 工具 — 移至主系统 ⏭️

> **决策**: codesearch 应在主系统 Go backend 层面实现 (基于 GitHub REST API)，
> 在 `buildToolDefinitions()` 中注册暴露给 agent，而非在 oa-coder Rust 侧重复实现。
> 详见 `docs/claude/deferred/oa-coder.md`。

---

## Phase 4: websearch 工具 — 移至主系统 ⏭️

> **决策**: 主系统已有完整 web_search + web_fetch 实现 (`internal/agents/tools/web_fetch.go`,
> Brave/Perplexity 双 provider)，仅需在 `buildToolDefinitions()` 中注册暴露给 agent。
> 不在 oa-coder Rust 侧重复实现。详见 `docs/claude/deferred/oa-coder.md`。

---

## Phase 5: 后端 Config Schema + UI Hints — 延后 ⏸️

> 依赖 P3/P4 最终方案确定。CoderDefaultsConfig 已创建 (P2)，
> schema 自动包含。hints 等联网搜索方案确定后再补。

- [x] `CoderDefaultsConfig` 加入 `AgentDefaultsConfig` — 已在 P2 完成
- [ ] `schema_hints_data.go` 添加 UI hints — 延后至联网搜索方案确定
- [ ] 验证规则 — 延后

---

## Phase 6: 前端配置向导 — 延后 ⏸️

> 依赖 P5 schema + 联网搜索方案。

- [ ] Config Form 自动渲染 — 延后
- [ ] `wizard-coder.ts` — 延后
- [ ] `server_methods_wizard.go` — 延后
- [ ] `locales/{en,zh}.ts` — 延后

---

## Phase 7: Skill 4 审计 + 归档

- [x] P1+P2 审计报告 → `docs/claude/audit/audit-2026-02-26-oa-coder-completion-p1p2.md` (PASS, 7 findings, 0 CRITICAL/HIGH)
- [x] 更新 deferred doc: 审计项已标记完成，功能项已标记移至主系统
- [x] 归档本跟踪文档 ✅

---

## 关键文件清单

### Rust (oa-coder) — P1 已修改
- `src/edit/levenshtein.rs` — F-06 DoS 防护
- `src/edit/replacers.rs` — F-18 regex 优化
- `src/server.rs` — F-14 序列化 + F-21 版本校验
- `src/tools/{read,write,edit,grep}.rs` — F-07/F-11/F-17/F-22
- `src/util/{atomic,mod}.rs` — F-11 原子写入 (新增) + F-19 删除 filetime
- `Cargo.toml` — F-23 移除 tokio + 提升 tempfile

### Go (backend) — P2 已修改
- `pkg/types/types_agent_defaults.go` — 新增 CoderDefaultsConfig
- `internal/agents/runner/tool_executor.go` — 可配置超时 (默认 120s)
- `internal/agents/runner/attempt_runner.go` — resolveCoderTimeoutSeconds + 传入
