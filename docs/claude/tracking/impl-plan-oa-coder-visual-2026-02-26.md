---
document_type: Tracking
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-oa-coder-visual.md
skill5_verified: true
---

# oa-coder 可视化集成 + 独立开源计划

## 概述

将 oa-coder 从后台静默 MCP 子进程改造为:
1. 聊天流内富卡片可视化 (diff 预览、命令执行)
2. 写文件/执行命令需用户卡片确认
3. 提取为独立开源 Rust CLI 项目

**核心原则**: 增量式改动，不破坏现有功能。MCP Bridge 子进程模式保留。

## Online Verification Log

### Claude Code Tool Cards Pattern
- **Query**: claude code tool cards streaming confirmation flow
- **Source**: Claude Code CLI 参考 + exec-approval 模式
- **Key finding**: tool cards + streaming + 确认流程可复用
- **Verified date**: 2026-02-26

### MCP Sub-agent Card Visualization
- **Query**: claude-devtools subagent inline card
- **Source**: claude-devtools 参考
- **Key finding**: subagent 可展开内联卡片 + 工具 trace
- **Verified date**: 2026-02-26

## 依赖图

```
Phase 1 (Go 确认流) ──→ Phase 3 (前端确认卡片)
Phase 2 (前端富卡片)     ↗  (Phase 2 可与 P1 并行)
Phase 4 (独立提取)          (完全独立，可与 P1-3 并行)
Phase 5 (Coder 技能文件)    (独立)
Phase 6 (审计)              (依赖 P1-5)
```

---

## Phase 1: Go 后端确认流 (~260 LOC)

### 目标
在 Go 层拦截 coder 的写/执行操作，广播确认事件，等待用户审批。

### 关键文件
| 文件 | 操作 | 改动量 |
|------|------|--------|
| `runner/coder_confirmation.go` | 新增 | ~180 LOC |
| `runner/tool_executor.go` | 修改 | ~45 LOC |
| `runner/attempt_runner.go` | 修改 | ~15 LOC |
| `gateway/server_methods.go` | 修改 | ~25 LOC |
| `gateway/broadcast.go` | 修改 | ~4 LOC |
| `gateway/ws_server.go` | 修改 | ~2 LOC |
| `gateway/boot.go` | 修改 | ~10 LOC |
| `gateway/server.go` | 修改 | ~5 LOC |

### 子任务
- [x] 1.1 新增 CoderConfirmationManager (coder_confirmation.go)
- [x] 1.2 修改 executeCoderTool 拦截 (tool_executor.go)
- [x] 1.3 ToolExecParams 新增 CoderConfirmation 字段
- [x] 1.4 attempt_runner.go 连线
- [x] 1.5 WebSocket RPC 注册 (server_methods.go)
- [x] 1.6 广播事件注册 (broadcast.go + ws_server.go)
- [x] 1.7 boot.go / server.go 初始化

### 设计细节

**CoderConfirmationManager**:
```
type CoderConfirmationRequest struct {
    ID, ToolName, Args, Preview, CreatedAtMs, ExpiresAtMs
}
type CoderConfirmPreview struct {
    FilePath, OldString, NewString, Content, Command, LineCount
}
type CoderConfirmationManager struct {
    mu sync.Mutex, pending map[string]chan string, bc, timeout
}
```

- `RequestConfirmation(req)` → 广播 `coder.confirm.requested` → channel 等待
- `ResolveConfirmation(id, decision)` → WebSocket RPC 回调
- `extractCoderPreview(toolName, args)` → 解析 JSON args
- `CoderConfirmation=nil` 时完全跳过 → 零破坏

---

## Phase 2: 前端富工具卡片 (~265 LOC)

### 目标
为 coder_* 工具在聊天流中渲染增强卡片 (diff 预览、命令预览)。

### 关键文件
| 文件 | 操作 | 改动量 |
|------|------|--------|
| `ui/src/ui/tool-display.json` | 修改 | ~20 LOC |
| `ui/src/ui/chat/coder-tool-cards.ts` | 新增 | ~150 LOC |
| `ui/src/ui/chat/tool-cards.ts` | 修改 | ~15 LOC |
| `ui/src/styles/chat/coder-cards.css` | 新增 | ~80 LOC |

### 子任务
- [x] 2.1 tool-display.json 注册 coder 工具
- [x] 2.2 新建 coder-tool-cards.ts 渲染器
- [x] 2.3 集成到 tool-cards.ts
- [x] 2.4 CSS 样式

---

## Phase 3: 前端确认卡片 (~200 LOC)

### 目标
处理 `coder.confirm.requested` WebSocket 事件，渲染内联确认卡片。

### 关键文件
| 文件 | 操作 | 改动量 |
|------|------|--------|
| `ui/src/ui/controllers/coder-confirmation.ts` | 新增 | ~80 LOC |
| `ui/src/ui/app-gateway.ts` | 修改 | ~20 LOC |
| `ui/src/ui/chat/coder-tool-cards.ts` | 修改 | ~60 LOC |
| `ui/src/ui/app.ts` | 修改 | ~20 LOC |
| `ui/src/styles/chat/coder-cards.css` | 修改 | ~20 LOC |

### 子任务
- [x] 3.1 新建 coder-confirmation.ts 控制器
- [x] 3.2 WebSocket 事件处理 (app-gateway.ts)
- [x] 3.3 确认卡片渲染 (coder-tool-cards.ts)
- [x] 3.4 用户决策 RPC (app.ts)

---

## Phase 4: 独立开源提取 (~105 LOC)

### 目标
将 oa-coder 提取为可独立 `cargo install` 的开源项目。

### 关键文件
| 文件 | 操作 | 改动量 |
|------|------|--------|
| `oa-coder/Cargo.toml` | 修改 | ~10 LOC |
| `oa-coder/src/tools/bash.rs` | 修改 | ~15 LOC |
| `oa-cmd-coder/Cargo.toml` | 修改 | ~2 LOC |
| `~/Desktop/oa-coder/` | 新建 | 复制 + 独立化 |

### 子任务
- [x] 4.1 移除 oa-types 依赖
- [x] 4.2 oa-sandbox 改为可选 feature
- [x] 4.3 bash.rs 条件编译
- [x] 4.4 复制到桌面 + 独立化
- [x] 4.5 monorepo 兼容验证

---

## Phase 5: Coder 技能文件

### 子任务
- [x] 5.1 创建 docs/skills/tools/coder/SKILL.md

---

## Phase 6: Skill 4 审计 + 归档

### 子任务
- [x] 6.1 全量审计 P1-P5
- [x] 6.2 审计报告
- [x] 6.3 归档

---

## 风险评估

| 风险 | 级别 | 缓解措施 |
|------|------|----------|
| 破坏现有 coder 功能 | LOW | `CoderConfirmation=nil` 完全跳过 |
| 确认超时阻塞工具循环 | MEDIUM | 60s 超时 + 自动拒绝 + ctx 取消 |
| 前端卡片渲染冲突 | LOW | `isCoderTool()` 前缀隔离 |
| 独立提取破坏 monorepo | LOW | feature flag，默认启用 |
| Channel 竞态 | LOW | 复用 exec-approval 成熟模式 |

## 总量

| Phase | 新文件 | 修改文件 | LOC |
|-------|--------|----------|-----|
| P1 Go 确认流 | 1 | 7 | ~260 |
| P2 富卡片 | 2 | 2 | ~265 |
| P3 确认卡片 | 1 | 4 | ~200 |
| P4 独立提取 | 1(桌面) | 3 | ~105 |
| P5 技能文件 | 1 | 0 | ~50 |
| **总计** | **6** | **16** | **~880** |
