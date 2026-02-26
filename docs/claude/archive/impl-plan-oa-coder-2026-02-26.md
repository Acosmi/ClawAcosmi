---
document_type: Tracking
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-oa-coder-full.md
skill5_verified: true
---

# oa-coder: Rust 编程子智能体实施计划

> MCP stdio 协议 + 9 层模糊编辑 + oa-sandbox 沙箱集成，可独立开源

## 架构概览

```
Gateway (Go)                      oa-coder 子进程 (Rust)
┌─────────────┐                  ┌──────────────────────────┐
│ CoderBridge │──MCP stdio──────▶│ oa-coder MCP Server      │
│ (argus.Bridge│  JSON-RPC 2.0  │  ├─ edit (9层模糊匹配)    │
│  复用)       │◀────────────────│  ├─ read (文件读取)       │
└─────────────┘                  │  ├─ grep (rg --json)      │
                                 │  ├─ glob (globset)        │
                                 │  ├─ bash (oa-sandbox)     │
                                 │  └─ write (文件写入)      │
                                 └──────────────────────────┘
```

## Online Verification Log

### 1. rmcp (Rust MCP SDK)
- **Query**: "rmcp crate Rust MCP SDK crates.io"
- **Source**: https://crates.io/crates/rmcp, https://github.com/modelcontextprotocol/rust-sdk
- **Key finding**: v0.16.0, 官方 SDK, 支持 stdio transport + proc-macro 工具生成 (`#[tool]`)。Pre-1.0 需 pin 精确版本。
- **Verified date**: 2026-02-26

### 2. similar crate (Diff 算法)
- **Query**: "similar crate rust diff algorithm"
- **Source**: https://crates.io/crates/similar, https://github.com/mitsuhiko/similar
- **Key finding**: v2.7.0 稳定版, 零依赖, 支持 Myers/Patience/LCS 三种算法, 行级 diff `TextDiff::from_lines()`.
- **Verified date**: 2026-02-26

### 3. globset crate (Glob 匹配)
- **Query**: "globset crate rust glob matching"
- **Source**: https://crates.io/crates/globset
- **Key finding**: v0.4.18, BurntSushi (ripgrep 作者), 支持 `**` 递归 + glob set 多模式匹配, 经 ripgrep 实战检验.
- **Verified date**: 2026-02-26

### 4. MCP Protocol 规范
- **Query**: "MCP specification JSON-RPC stdio transport"
- **Source**: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
- **Key finding**: 规范版本 2025-06-18, stdio 传输用换行分隔 JSON-RPC 2.0, 必需方法: initialize / tools/list / tools/call. 与现有 worker JSON-Lines 协议结构一致.
- **Verified date**: 2026-02-26

### 5. grep 方案选择
- **Query**: "grep-regex crate vs rg subprocess"
- **Source**: https://github.com/BurntSushi/ripgrep/discussions/2067
- **Key finding**: BurntSushi 推荐 subprocess (`rg --json`), VS Code 也这么做. grep-regex 文档缺失且需手动组装文件遍历. 决定用 subprocess 方案.
- **Verified date**: 2026-02-26

## 技术决策汇总

| 组件 | 选型 | 风险 |
|------|------|------|
| MCP Server | 手写 JSON-RPC 2.0 stdio (避免 rmcp pre-1.0 风险) | 低 |
| Diff | `similar` 2.7 (Patience 算法) | 低 |
| Glob | `globset` 0.4 | 低 |
| Grep | `rg --json` subprocess | 低 |
| MCP 传输 | stdio, 换行分隔 JSON-RPC 2.0 | 低 |
| 沙箱 | `oa-sandbox` 直接依赖 | 低 |

## 实施阶段

### Phase 1: Crate 脚手架 + MCP Server ✅
- [x] Skill 5 联网验证
- [x] 创建 `oa-coder` crate 目录 + Cargo.toml
- [x] 创建 `oa-cmd-coder` crate 目录 + Cargo.toml
- [x] 注册到 workspace Cargo.toml (members + dependencies)
- [x] 实现 MCP Server 骨架 (手写 JSON-RPC 2.0 stdio, 不用 rmcp 避免 pre-1.0 风险)
- [x] 注册工具存根 (edit/read/write/grep/glob/bash) — 含完整实现
- [x] 创建 `oa-cmd-coder/src/lib.rs` clap 子命令
- [x] 注册到 `oa-cli/src/commands.rs` (CoderCommand/CoderAction/dispatch_coder)
- [x] `cargo check` 通过 (oa-coder + oa-cmd-coder + oa-cli)
- [x] 11 个单元测试通过 (levenshtein + replacers + diff)
- [x] clippy 零 error

### Phase 2: 9 层模糊匹配引擎 ✅
- [x] 移植 SimpleReplacer (精确匹配)
- [x] 移植 LineTrimmedReplacer (行 trim 比较)
- [x] 移植 BlockAnchorReplacer (首尾锚定 + Levenshtein)
- [x] 移植 WhitespaceNormalizedReplacer (空白归一化)
- [x] 移植 IndentationFlexibleReplacer (缩进容忍)
- [x] 移植 EscapeNormalizedReplacer (转义归一化)
- [x] 移植 TrimmedBoundaryReplacer (边界修剪)
- [x] 移植 ContextAwareReplacer (上下文感知)
- [x] 移植 MultiOccurrenceReplacer (多处出现)
- [x] 实现 `replace()` 编排函数
- [x] 实现 Levenshtein 距离算法
- [x] 实现 diff 输出 (similar crate)
- [x] 集成到 edit tool

### Phase 3: read/write/grep/glob/bash 工具 ✅
- [x] 实现 read tool (行号, offset/limit, 二进制检测)
- [x] 实现 write tool (新建/覆盖, 目录自动创建)
- [x] 实现 grep tool (rg --json subprocess, 结果解析)
- [x] 实现 glob tool (globset, 文件发现)
- [x] 实现 bash tool (oa-sandbox 沙箱执行)

### Phase 4: 测试 ✅
- [x] edit 9层模糊匹配单元测试 (含 OpenAcosmi 测试用例)
- [x] MCP 协议测试 (initialize/tools-list/tools-call)
- [x] read/write/grep/glob 工具测试
- [x] bash 执行测试
- [x] `cargo test` 全部通过 (12/12)
- [x] `cargo clippy` 零 error

### Phase 5: Gateway CoderBridge 接入 ✅
- [x] `coderBridgeAdapter` in `server.go` (复用 argus.Bridge MCP stdio 管理)
- [x] `boot.go` CoderBridge 字段 + 启动/停止逻辑
- [x] `attempt_runner.go` CoderBridgeForAgent 接口 + `coder_` 前缀工具注册
- [x] `tool_executor.go` `executeCoderTool()` 分发函数
- [x] `server.go` AttemptRunner 注入 + StopCoder 关闭
- [x] `go build ./...` 通过

### Phase 6: codesearch + websearch (延迟)
- 延迟到后续迭代，见 `docs/claude/deferred/oa-coder.md`

### Phase 7: 审计 + 归档 ✅
- [x] Skill 4 代码审计 (37 findings: 2C/4H/10M/13L/8I)
- [x] 修复所有 CRITICAL/HIGH (6 fixes)
- [x] 修复关键 MEDIUM (symlink loops, bash timeout)
- [x] 审计报告 → `docs/claude/audit/audit-2026-02-26-oa-coder-full.md`
- [x] 状态改为 Archived
