# Phase 10 前端集成修复 — 任务清单

> 最后更新：2026-02-17
> 审计来源：[phase10-frontend-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-frontend-audit.md)
> Bootstrap：[phase10-frontend-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-frontend-bootstrap.md)

---

## Batch FE-A：Go Gateway 缺失方法注册（P0 阻塞）✅

> 目标：将 14 个前端调用但 Go 后端未注册的 RPC 方法注册为 Stub，防止 `unknown method` 错误。
> **完成时间**：2026-02-17

- [x] FE-A1: 在 `server_methods_stubs.go` 添加 `web.login.start`、`web.login.wait`
- [x] FE-A2: 在 `server_methods_stubs.go` 添加 `update.run`
- [x] FE-A3: 在 `server_methods_stubs.go` 添加 `sessions.usage`、`sessions.usage.timeseries`、`sessions.usage.logs`
- [x] FE-A4: 在 `server_methods_stubs.go` 添加 `exec.approvals.get`、`exec.approvals.set`、`exec.approvals.node.get`、`exec.approvals.node.set`
- [x] FE-A5: 在 `server_methods_stubs.go` 添加 `agents.files.list`、`agents.files.get`、`agents.files.set`
- [x] FE-A6: 在 `server_methods_stubs.go` 添加 `sessions.preview`
- [x] FE-A7: 更新 `server_methods.go` 权限表（`readMethods`/`writeMethods`/`adminExactMethods`）覆盖新方法
- [x] FE-A8: 编译验证 `go build ./...` + `go vet ./...`
- [x] FE-A9: 运行测试 `go test -race ./internal/gateway/...`

---

## Batch FE-B：解耦 Node.js 遗留引用（P0 阻塞）✅

> 目标：消除前端对 TS 后端 `src/` 目录的编译依赖，使前端可独立构建。
> **完成时间**：2026-02-17

- [x] FE-B1: 内联 `buildDeviceAuthPayload()` 到 `ui/src/ui/gateway.ts`
- [x] FE-B2: 内联 `GATEWAY_CLIENT_MODES`/`GATEWAY_CLIENT_NAMES` 常量到 `ui/src/ui/gateway.ts`
- [x] FE-B3: 创建 `ui/src/ui/session-key.ts` 内联 `parseAgentSessionKey()`
- [x] FE-B4: 内联 `formatDurationHuman()` 到 `ui/src/ui/format.ts`
- [x] FE-B5: 内联 `formatRelativeTimestamp()` 到 `ui/src/ui/format.ts`
- [x] FE-B6: 内联 `stripReasoningTagsFromText()` 到 `ui/src/ui/format.ts`
- [x] FE-B7: 内联 `formatDurationCompact()` 到 `ui/src/ui/format.ts`（审计外发现）
- [x] FE-B8: 创建 `ui/src/ui/tool-policy.ts` 内联工具策略函数（审计外发现）
- [x] FE-B9: 删除所有 `from "../../../src/"` import 语句
- [x] FE-B10: 前端构建验证 — Vite build 通过（421ms, 123 modules）
- [x] FE-B11: grep 验证 — 零跨系统 import 残留

---

## Batch FE-C：Stub 方法升级为真实实现（P1 功能完善）

> 目标：将高优先级 Stub 方法升级为真实处理器。

### FE-C1: agents.files.* 实现

- [x] FE-C1a: 实现 `agents.files.list` 处理器（读取 agent 文件列表）
- [x] FE-C1b: 实现 `agents.files.get` 处理器（读取文件内容）
- [x] FE-C1c: 实现 `agents.files.set` 处理器（保存文件内容）
- [x] FE-C1d: 单元测试

### FE-C2: sessions.usage.* 实现

- [x] FE-C2a: 实现 `sessions.usage` 处理器（schema-correct 空响应）
- [x] FE-C2b: 实现 `sessions.usage.timeseries` 处理器（schema-correct 空响应）
- [x] FE-C2c: 实现 `sessions.usage.logs` 处理器（schema-correct 空响应）
- [x] FE-C2d: 单元测试

### FE-C3: exec.approvals.* 实现

- [x] FE-C3a: 实现 `exec.approvals.get` / `exec.approvals.set`（含 OCC）
- [x] FE-C3b: 实现 `exec.approvals.node.get` / `exec.approvals.node.set`（node stub）
- [x] FE-C3c: 单元测试

---

## Batch FE-D：i18n 基础设施搭建（P2 增强）✅

> 目标：搭建 i18n 基础设施，支持中/英双语切换。
> **完成时间**：2026-02-17

- [x] FE-D1: 创建零依赖 `i18n.ts` 核心模块（`t()` + `{{var}}` 插值 + locale 检测）
- [x] FE-D2: 创建 `locales/zh.ts` + `locales/en.ts` 翻译文件（~100 keys）
- [x] FE-D3: 替换 `window.confirm/prompt` 为自定义 `<dialog>` 组件（4 处）
- [x] FE-D4: MVP 字符串抽取（`navigation.ts` + `app-render.ts` 品牌/状态/侧边栏）
- [x] FE-D5: 语言切换器 UI（topbar，持久化到 `UiSettings`）
- [x] FE-D6: Vite 构建验证通过

> 剩余 view 文件的 i18n 改造记录在 [frontend-deferred-items.md](./frontend-deferred-items.md)。

---

## 完成后文档更新

每个 Batch 完成后必须更新：

1. `frontend-deferred-items.md` — 前端延迟项
2. 本文件 — 勾选完成项
3. `refactor-plan-full.md` — 如适用
