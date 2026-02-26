# Phase 10 前端集成修复 — 新窗口 Bootstrap

> 最后更新：2026-02-17
> 任务文档：[phase10-frontend-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-frontend-task.md)
> 审计报告：[phase10-frontend-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-frontend-audit.md)

---

## 新窗口启动模板

复制以下内容到新窗口即可快速恢复上下文：

```
@/refactor 执行 Phase 10 前端集成修复。

任务文档：docs/renwu/phase10-frontend-task.md
审计报告：docs/renwu/phase10-frontend-audit.md
延迟汇总：docs/renwu/deferred-items.md（Phase 10 前端审计章节）
全局路线图：docs/renwu/refactor-plan-full.md

当前进度：请读取 phase10-frontend-task.md 确认已完成和待办项。

本批次目标：[在此指定 Batch FE-A / FE-B / FE-C / FE-D]
```

---

## 一、背景

2026-02-17 对 `ui/src/` 进行了逐文件 API 审计，与 Go 后端 `backend/internal/gateway/` 逐一比对，发现以下问题：

| 类别 | 项数 | 严重性 |
|------|------|--------|
| Go 缺失 RPC 方法（未注册） | 14 个 | P0 阻塞 |
| Node.js 遗留编译依赖 | 7 处 | P0 阻塞 |
| Stub 方法待升级 | 18 个 | P1 功能 |
| i18n 改造 | ~34 文件 | P2 增强 |

---

## 二、批次概览

| 批次 | 内容 | 优先级 | 预估文件数 | 状态 |
|------|------|--------|-----------|------|
| **FE-A** | Go Gateway 缺失方法 Stub 注册 | P0 | 2 改 | ✅ 完成 |
| **FE-B** | 解耦 Node.js 遗留引用 | P0 | 6 改 + 3 新 | ✅ 完成 |
| **FE-C** | Stub 升级为真实实现 | P1 | ~6 新/改 | ⬜ |
| **FE-D** | i18n 基础设施搭建 | P2 | ~10 改 | ⬜ |

---

## 三、各批次关键指引

### Batch FE-A：Go Gateway 缺失方法 Stub 注册

**目标**：注册 14 个缺失 RPC 方法为 Stub，防止前端报 `unknown method` 错误。

**关键文件**：

- `backend/internal/gateway/server_methods_stubs.go` — 添加方法名到 `StubHandlers()`
- `backend/internal/gateway/server_methods.go` — 在权限表 `readMethods`/`adminMethodPrefixes`/`adminExactMethods` 中注册新方法

**需要添加的方法**：

```
web.login.start, web.login.wait,
update.run,
sessions.usage, sessions.usage.timeseries, sessions.usage.logs,
exec.approvals.get, exec.approvals.set, exec.approvals.node.get, exec.approvals.node.set,
agents.files.list, agents.files.get, agents.files.set,
sessions.preview
```

**验证**：

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/gateway/...
```

---

### Batch FE-B：解耦 Node.js 遗留引用

**目标**：将 7 个从 `ui/src/` 引用 TS 后端 `src/` 的 import 替换为内联实现。

**源引用清单**：

| UI 文件 | 引用源 | 需内联的函数/常量 |
|---------|--------|-----------------|
| `ui/src/ui/gateway.ts` L1 | `src/gateway/device-auth.ts` | `buildDeviceAuthPayload()` |
| `ui/src/ui/gateway.ts` L2-7 | `src/gateway/protocol/client-info.ts` | 常量 |
| `ui/src/ui/app-render.ts` L4 | `src/routing/session-key.ts` | `parseAgentSessionKey()` |
| `ui/src/ui/format.ts` L1-3 | `src/infra/format-time/*.ts`, `src/shared/text/*.ts` | 3 个工具函数 |
| `ui/src/ui/app-chat.ts` L4 | `src/sessions/session-key-utils.ts` | `parseAgentSessionKey()` |

**方法**：先读取每个 TS 源文件，将纯函数复制到 `ui/src/ui/` 下的对应文件或新建 `ui/src/ui/shared/` 目录。

**验证**：

```bash
cd ui && npm run build  # 或等价前端构建命令
```

---

### Batch FE-C：Stub → 真实实现

**目标**：实现 Agent Files、Sessions Usage、Exec Approvals 三组 RPC 方法。

**前置已就绪**：

- `internal/agents/` — Agent 管理基础 ✅
- `internal/gateway/server_methods_sessions.go` — Sessions 基础 ✅
- `internal/gateway/server_methods.go` — 方法注册表 ✅

**参考模式**：

- `server_methods_config.go` — config.get/set/apply/schema 实现模式
- `server_methods_sessions.go` — sessions.list/patch/delete 实现模式

---

### Batch FE-D：i18n 基础设施

**目标**：搭建国际化基础，暂不要求全量翻译。

**参考**：`phase10-frontend-audit.md` 第五章 i18n 调研结果。

---

## 四、编码规范提醒

1. 所有表述/文档使用中文，代码标识符使用英文
2. 接口在使用方定义（DI 模式）
3. 错误处理：`fmt.Errorf("xxx: %w", err)` 不可 panic
4. 详见 `skills/acosmi-refactor/references/coding-standards.md`

## 五、完成后文档更新

每个 Batch 完成后必须更新：

1. `deferred-items.md` — 对应项标记 ✅
2. `phase10-frontend-task.md` — 勾选完成项
3. `refactor-plan-full.md` — 如适用
