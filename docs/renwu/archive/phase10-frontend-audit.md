# Phase 10 前端集成审计报告（逐文件审计版）

**审计日期**: 2026-02-17
**审计对象**: `ui/src/` (Frontend, 116 TS 文件) vs `backend/internal/gateway/` (Go Gateway)
**审计方法**: 逐文件阅读源码，提取所有 `.request()` RPC 调用，与 Go 后端逐一比对

---

## 一、概览

前端应用 (`ui/`) 通过 WebSocket JSON-RPC v3 协议连接 Go 网关。

- **连接协议**: WebSocket (JSON-RPC v3)
- **认证方式**: Device Identity / Token / Password (`gateway.ts`)
- **总计 RPC 方法**: 46+ 个不同方法
- **Go 已实现方法**: 18 个（真实处理器）
- **Go Stub 方法**: 42 个（返回 `{ok:true, stub:true}`）
- **完全缺失方法**: 9 个（未注册，调用将报 `unknown method`）

---

## 二、Node.js 遗留引用（跨系统依赖）⚠️

以下 UI 文件直接 import 了 TS 后端 `src/` 目录的模块：

| UI 文件 | 引用的 TS 后端模块 | 用途 | 风险 |
|---------|-------------------|------|------|
| `gateway.ts` L1 | `src/gateway/device-auth.ts` | `buildDeviceAuthPayload()` | ⚠️ 编译依赖 |
| `gateway.ts` L2-7 | `src/gateway/protocol/client-info.ts` | `GATEWAY_CLIENT_MODES/NAMES` | ⚠️ 编译依赖 |
| `app-render.ts` L4 | `src/routing/session-key.ts` | `parseAgentSessionKey()` | ⚠️ 编译依赖 |
| `format.ts` L1 | `src/infra/format-time/format-duration.ts` | `formatDurationHuman()` | ⚠️ 编译依赖 |
| `format.ts` L2 | `src/infra/format-time/format-relative.ts` | `formatRelativeTimestamp()` | ⚠️ 编译依赖 |
| `format.ts` L3 | `src/shared/text/reasoning-tags.ts` | `stripReasoningTagsFromText()` | ⚠️ 编译依赖 |
| `app-chat.ts` L4 | `src/sessions/session-key-utils.ts` | `parseAgentSessionKey()` | ⚠️ 编译依赖 |

**影响**：前端构建（Vite）仍依赖 TS 后端源码。若移除 `src/` 目录或单独部署前端，构建将失败。
**建议**：将上述 7 个函数/常量内联到 `ui/src/` 中，或抽取为独立的 `shared/` 包。

---

## 三、RPC 方法完整覆盖度矩阵（逐文件审计）

### 3.1 已实现方法（Go 真实处理器）✅

| 前端文件 | RPC 方法 | Go 处理器文件 | 状态 |
|----------|----------|-------------|------|
| `chat.ts` | `chat.history` | `server_methods_chat.go` | ✅ |
| `chat.ts` | `chat.send` | `server_methods_chat.go` | ✅ |
| `chat.ts` | `chat.abort` | `server_methods_chat.go` | ✅ |
| `config.ts` | `config.get` | `server_methods_config.go` | ✅ |
| `config.ts` | `config.set` | `server_methods_config.go` | ✅ |
| `config.ts` | `config.apply` | `server_methods_config.go` | ✅ |
| `config.ts` | `config.schema` | `server_methods_config.go` | ✅ |
| `agents.ts` | `agents.list` | `server_methods_agents.go` | ✅ |
| `agent-identity.ts` | `agent.identity.get` | `server_methods_agent.go` | ✅ |
| `channels.ts` | `channels.status` | `server_methods_channels.go` | ✅ |
| `channels.ts` | `channels.logout` | `server_methods_channels.go` | ✅ |
| `logs.ts` | `logs.tail` | `server_methods_logs.go` | ✅ |
| `debug.ts` | `status` | `server_methods_system.go` | ✅ |
| `debug.ts` | `health` | `server_methods_system.go` | ✅ |
| `debug.ts` | `last-heartbeat` | `server_methods_system.go` | ✅ |
| `debug.ts` | `models.list` | `server_methods_models.go` | ✅ |
| `sessions.ts` | `sessions.list` | `server_methods_sessions.go` | ✅ |
| `sessions.ts` | `sessions.patch` | `server_methods_sessions.go` | ✅ |
| `sessions.ts` | `sessions.delete` | `server_methods_sessions.go` | ✅ |
| `presence.ts` | `system-presence` | `server_methods_system.go` | ✅ |

### 3.2 Stub 方法（返回 `{ok:true, stub:true}`）⚠️

| 前端文件 | RPC 方法 | 使用场景 |
|----------|----------|---------|
| `cron.ts` | `cron.list` | 列出定时任务 |
| `cron.ts` | `cron.status` | 定时任务状态 |
| `cron.ts` | `cron.runs` | 任务执行历史 |
| `cron.ts` | `cron.add` | 添加定时任务 |
| `cron.ts` | `cron.update` | 更新定时任务 |
| `cron.ts` | `cron.remove` | 删除定时任务 |
| `cron.ts` | `cron.run` | 手动执行任务 |
| `nodes.ts` | `node.list` | 列出远程节点 |
| `devices.ts` | `device.pair.list` | 列出设备配对 |
| `devices.ts` | `device.pair.approve` | 批准配对 |
| `devices.ts` | `device.pair.reject` | 拒绝配对 |
| `devices.ts` | `device.token.rotate` | 轮换设备令牌 |
| `devices.ts` | `device.token.revoke` | 吊销设备令牌 |
| `app.ts` | `exec.approval.resolve` | 审批执行请求 |
| `skills.ts` | `skills.status` | 技能状态 |
| `skills.ts` | `skills.update` | 更新技能设置 |
| `skills.ts` | `skills.install` | 安装技能 |
| `usage.ts` | `usage.cost` | 使用成本统计 |

**影响**：前端 UI 页面可以打开，但功能不可用（列表为空、操作无效果）。

### 3.3 完全缺失方法（未注册，报 `unknown method`）❌

| 前端文件 | RPC 方法 | 严重性 | 说明 |
|----------|----------|--------|------|
| `channels.ts` L36 | `web.login.start` | 🔴 High | WhatsApp 二维码登录启动 |
| `channels.ts` L61 | `web.login.wait` | 🔴 High | WhatsApp 登录等待回调 |
| `config.ts` L165 | `update.run` | 🟡 Medium | 在线更新功能 |
| `usage.ts` L43 | `sessions.usage` | 🟡 Medium | 会话使用量统计 |
| `usage.ts` L75 | `sessions.usage.timeseries` | 🟡 Medium | 会话使用量时间序列 |
| `usage.ts` L97 | `sessions.usage.logs` | 🟡 Medium | 会话使用量日志 |
| `exec-approvals.ts` L56 | `exec.approvals.get` | 🟡 Medium | 获取审批配置 |
| `exec-approvals.ts` L70 | `exec.approvals.set` | 🟡 Medium | 保存审批配置 |
| `exec-approvals.ts` L62 | `exec.approvals.node.get` | 🟡 Medium | 获取节点审批配置 |
| `exec-approvals.ts` L76 | `exec.approvals.node.set` | 🟡 Medium | 保存节点审批配置 |
| `agent-files.ts` L42 | `agents.files.list` | 🟡 Medium | 列出 Agent 文件 |
| `agent-files.ts` L73 | `agents.files.get` | 🟡 Medium | 获取 Agent 文件内容 |
| `agent-files.ts` L111 | `agents.files.set` | 🟡 Medium | 保存 Agent 文件 |
| `sessions.ts` L49 | `sessions.preview` | 🟢 Low | 会话预览（在 readMethods 中但无处理器） |

**影响**：用户在前端调用这些方法会收到 `unknown method` 错误。

---

## 四、WebSocket 事件流审计

`app-gateway.ts` 中监听的事件类型：

| 事件名 | Go 后端广播 | 前端处理 |
|--------|-----------|---------|
| `agent` | ✅ `Broadcaster` | `handleAgentEvent()` — tool stream |
| `chat` | ✅ `ChatRunState` | `handleChatEvent()` — delta/final/aborted/error |
| `presence` | ✅ `PresenceStore` | 在线状态更新 |
| `cron` | ⚠️ Stub | 触发 `loadCron()` 刷新 |
| `device.pair.requested` | ⚠️ Stub | 触发 `loadDevices()` |
| `device.pair.resolved` | ⚠️ Stub | 触发 `loadDevices()` |
| `exec.approval.requested` | ⚠️ Stub | 添加审批队列条目 |
| `exec.approval.resolved` | ⚠️ Stub | 移除审批队列条目 |
| `connect.challenge` | ✅ 握手协议 | nonce 签名回应 |

---

## 五、国际化 (i18n) 前期调研

### 5.1 当前状态

- **无 i18n 框架**：项目中未使用任何国际化库
- **locale 传递**：`gateway.ts` L214 发送 `navigator.language` 给服务器
- **所有 UI 字符串均为硬编码英文**

### 5.2 硬编码字符串分布

| 类别 | 文件数 | 示例 |
|------|--------|------|
| 视图层 HTML 模板 | ~30 个 views/*.ts | 按钮文本、标签、标题等 |
| 控制器提示框 | 4 处 | `window.confirm("Reject...")` 等 |
| 错误/状态消息 | 20+ 处 | `"Config hash missing"` 等 |
| 格式化文本 | ~10 处 | `"No instances yet."` 等 |

### 5.3 影响范围统计

- `views/` 目录总计 **15,750 行**（含测试），是 i18n 主战场
- 最大文件：`views/usage.ts` (5406L)、`views/agents.ts` (1963L)、`views/nodes.ts` (1168L)
- 使用 `html` 模板字符串 (lit-html 风格)，字符串直接嵌入 HTML 模板

### 5.4 i18n 改造建议

1. **推荐框架**: `lit-localize`（与 lit-html 生态一致）或 `i18next`（通用方案）
2. **改造步骤**:
   - 第一步：抽取所有 `views/` 中的用户可见字符串到 JSON locale 文件
   - 第二步：替换 `window.confirm/prompt` 为自定义 UI 组件
   - 第三步：接入 `navigator.language` 自动检测 + 手动切换
3. **工作量估算**: ~30 个 view 文件 + 4 个 controller 文件需改造，约 2–3 人天

---

## 六、后续行动建议

### 优先级 P0（阻塞性）

1. **注册缺失 RPC 方法为 Stub**：`web.login.start/wait`、`update.run`、`sessions.usage*`、`agents.files.*`、`exec.approvals.*` — 防止前端调用报错
2. **解耦 Node.js 引用**：将 7 个 `src/` 引用内联到 `ui/src/` 或提取为 shared 包

### 优先级 P1（功能完善）

3. **实现 `agents.files.*`** — Agent 文件编辑是高频 UI 操作
4. **实现 `sessions.usage*`** — Usage 页面当前完全不可用
5. **实现 `exec.approvals.*`** — 审批配置管理

### 优先级 P2（增强功能）

6. 实现 Cron 完整管线（从 stub 升级）
7. 实现 Device pairing 完整管线
8. i18n 基础设施搭建

### 优先级 P3（长期规划）

9. Skills/TTS/Browser 完整实现
10. WhatsApp QR 登录完整实现（依赖 whatsmeow SDK）
