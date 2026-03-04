# Phase 6 Bootstrap — CLI + 插件 + 钩子 + 守护进程

> 本文档为 Phase 6 新窗口提供完整启动上下文。
> 最后更新：2026-02-14

---

## 一、项目规则

- **技能规范**: `skills/acosmi-refactor/SKILL.md`
- **编码规范**: `skills/acosmi-refactor/references/coding-standards.md`
- **重构排序**: `skills/acosmi-refactor/references/refactor-order.md`
- **语言**: 中文交互/文档，英文代码标识符
- **验证**: 每批文件写完后 `cd backend && go build ./... && go vet ./...`

---

## 二、Go 代码库现状（340+ Go 文件）

### 已完成 Phase

| Phase | 状态 | Go 输出 |
| ---- | ---- | ---- |
| 0 基础搭建 | ✅ | `backend/go.mod`, 目录骨架 |
| 1 类型/配置 | ✅ | `internal/config/` (26 Go 文件), `pkg/types/` (12 文件) |
| 2 基础设施 | ✅ | `internal/infra/`, `pkg/log/`, `pkg/retry/`, `internal/outbound/` |
| 3 网关核心 | ✅ | `internal/gateway/` (24 文件), `internal/sessions/` |
| 4 Agent 引擎 | ✅ | `internal/agents/` (15 子包), `internal/routing/` |
| 5A 延迟项 | 4/12 ✅ | session key 路由、reasoning 降级、preview tool call |
| 5B 频道抽象 | ✅ | `internal/channels/` (23 文件), `pkg/contracts/` (3 文件) |
| 5C 工具桥接 | ✅ | `internal/channels/bridge/` (5 文件) |
| 5D 频道 SDK | ✅ | 6 频道共 135 Go 文件 |
| **6A Batch A** | ✅ | `internal/daemon/` (21 文件), `internal/plugins/` (13 文件), `internal/hooks/` (14 文件) |

### Phase 6 相关目录现状

| 目录 | 现状 |
| ---- | ---- |
| `internal/daemon/` | ✅ A1 完成（21 Go 文件 + 21 测试 PASS） |
| `internal/plugins/` | ✅ A2 完成（10 source + 3 test, 30 PASS） |
| `internal/hooks/` | ✅ A3 完成（10 source + 4 test, 35 PASS） |
| `internal/cron/` | 空目录 |
| `internal/cli/` | ✅ E1 完成（6 文件：version/argv/utils/banner/progress/gateway_rpc） |
| `cmd/openacosmi/` | ✅ E1-E3 完成（17 文件：main.go + 16 cmd_*.go，60+ 子命令 stub） |

### Phase 6 已就绪的关键前置

| 模块 | 文件 | 用途 |
| ---- | ---- | ---- |
| Session Key 路由 | `internal/routing/session_key.go` (340L, 18 函数) | `NormalizeAgentID`, `BuildAgentMainSessionKey`, `BuildAgentPeerSessionKey` 等 |
| Config 系统 | `internal/config/` (26 文件) | 完整配置加载/验证/默认值 |
| Gateway 核心 | `internal/gateway/` (24 文件) | HTTP/WS/SSE/认证/会话 |
| Channel 契约 | `pkg/contracts/` (3 文件) | 14 个适配器接口 + 23 字段 Plugin 契约 |

---

## 三、Phase 6 原版 TS 概况

| 模块 | TS 文件数 (非测试) | 核心职责 | 复杂度 |
| ---- | ---- | ---- | ---- |
| `cli/` | 138 | CLI 框架 + Commander 程序 + TUI | ⭐⭐⭐ |
| `commands/` | 174 | 核心/辅助命令（onboard, doctor, status 等） | ⭐⭐⭐⭐ |
| `plugins/` | 29 | 插件系统 (160KB) + plugin-sdk (13KB) | ⭐⭐⭐⭐⭐ |
| `hooks/` | 22 | 钩子系统 + Gmail 集成 | ⭐⭐⭐⭐ |
| `cron/` | 22 | 定时任务系统 | ⭐⭐⭐⭐ |
| `daemon/` | 19 | 守护进程 + macOS launchd | ⭐⭐⭐⭐ |
| `acp/` | 10 | ACP 协议 | ⭐⭐⭐ |
| **合计** | **414** | | |

> [!WARNING]
> `plugins/` (6.4) 和 `hooks/` (6.5) 存在**双向依赖**，必须在同一子阶段实现。

---

## 四、Phase 5A 未完成延迟项（8 项）

以下延迟项可在 Phase 6 中穿插完成：

| ID | 项目 | 预估代码量 | 依赖 |
| ---- | ---- | ---- | ---- |
| P2-D1 | Transform 管道 (`hooks_mapping.go`) | ~100 行 | Phase 6.5 钩子系统 |
| P2-D2 | HookMappingConfig 嵌套 match 结构 | ~20 行 | 无 |
| P2-D5 | Channel 动态插件注册 | ~15 行 | Phase 5B registry 已就绪 |
| P3-D1 | sessions.list Path 字段 | ~5 行 | `cfg.Session.Store` |
| P3-D2 | sessions.list Defaults 填充 | ~30 行 | `resolveConfiguredModelRef` |
| P3-D3 | sessions.delete 主 session 保护 | ~15 行 | `routing.BuildAgentMainSessionKey` ✅ 已就绪 |
| P4-DRIFT4 | API Key 环境变量映射 | ~50 行 | 无 |
| P4-NEW5 | 隐式供应商自动发现 | ~100 行 | 无 |

详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

---

## 五、频道 Gateway 集成桩汇总（32 项）

> Phase 5D 移植预留的 TODO 桩函数，均依赖 Phase 6 的 Gateway 消息分发管线。

| 频道 | 桩数 | 关键桩 |
| ---- | ---- | ---- |
| Signal | 3 | SIG-A 消息分发, SIG-B 配对, SIG-C 媒体+回执 |
| WhatsApp | 6 | WA-A WebSocket, WA-B 入站监控, WA-C 自动回复, WA-E Markdown 表格 |
| iMessage | 4 | IM-A 消息分发 (13 子项), IM-B 配对, IM-C 媒体, IM-D 分块 |
| Telegram | 5 | TG-1 消息分发, TG-2 handlers, TG-3 原生命令, TG-4 轮询, TG-5 Vision |
| Slack | 9 | SLK-A Socket Mode, SLK-B HTTP, SLK-C 分发, SLK-D~I 事件/缓存/回复 |
| Discord | 5 | DIS-A Gateway, DIS-B 消息管线, DIS-C 审批, DIS-D 命令, DIS-E 缓存 |

**共性依赖**（实现一次，6 频道共用）：

1. Gateway 消息分发管线 (`resolveAgentRoute → recordInboundSession → deliverReplies`)
2. `channels.PairingRegistry` — 配对请求管理
3. `routing.BuildAgentPeerSessionKey` — ✅ 已就绪
4. Markdown 表格转换 (`markdown/tables.go`) — Phase 7 但可提前实现

详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) Phase 6 各频道段落。

---

## 六、建议执行序

```
Phase 6.4-6.5 (plugins+hooks 同步，双向依赖)
    ↓
Phase 6.1 (CLI 框架，依赖 cli/program/)
    ↓
Phase 6.2-6.3 (核心/辅助命令，依赖 CLI 框架)
    ↓
Phase 6.6 (定时任务，依赖 hooks)
    ↓
Phase 6.7 (守护进程，依赖 daemon/ + build tags)
    ↓
Phase 6.8 (ACP 协议，相对独立)
```

> [!IMPORTANT]
> **Batch A-E 已完成**（daemon/plugins/hooks/cron/ACP/CLI 全部通过验证）。
> CLI 框架使用 Cobra，23 个新文件，60+ 子命令（stub）。业务逻辑待各 internal/ 模块就绪后填充。

---

## 七、已确认设计决策（2026-02-14 深度审计后）

> 详见 [phase6-deep-audit.md §6.3](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase6-deep-audit.md)

1. **插件加载**: Go 接口 + 编译时注册（`init()` 模式）。后续可叠加 HashiCorp go-plugin。
2. **Cron Agent**: Phase 6 实现 service 层；Agent 运行器（`run.ts` 22KB）等 Phase 4 完成后集成。
3. **CLI 命令**: 全量 80+ 命令。
4. **跨平台 daemon**: macOS + Linux 优先；Windows 视工作量纳入。

---

## 八、新窗口启动模板

在新窗口中粘贴以下内容（替换 `[批次编号]`）：

```
请阅读以下文件获取项目上下文：
1. `skills/acosmi-refactor/SKILL.md`
2. `docs/renwu/phase6-bootstrap.md`
3. `docs/renwu/phase6-deep-audit.md` — 深度审计报告（含设计决策）
4. `docs/renwu/phase6-task.md` — 任务清单（12 子任务版），完成后更新
5. `docs/renwu/deferred-items.md` — 检查 Phase 6 相关延迟项

当前任务：Phase 6 批次[批次编号]

请先分析 TS 源文件依赖关系，然后按依赖顺序逐文件移植。
每次只创建 1-2 个文件，写完编译验证。
完成后更新 docs/renwu/phase6-task.md 中对应子项为 [x]。
```
