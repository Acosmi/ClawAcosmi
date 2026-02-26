---
document_type: Tracking
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: Pending
skill5_verified: true
---

# 三项 Bug 修复计划

## Context

三个前端/后端 Bug 需要修复：
1. **定时任务页面崩溃** — `channelMeta` 类型不匹配导致 `TypeError`
2. **飞书审批卡片无法回调** — 按钮使用 `url` 字段导致跳转而非服务端回调
3. **飞书消息会话割裂** — 飞书消息创建独立会话，网页端无感知

---

## Fix 1: Cron 页面崩溃 ✅

**根因**: 后端 `server_methods_channels.go` 两处返回 `channelMeta: map[string]interface{}{}` (空对象)，前端期望 `ChannelUiMetaEntry[]` (数组)。`cron.ts:51` 调用 `.find()` 在对象上崩溃。

**方案**: 双重修复 — 后端返回正确类型 + 前端防御性检查

### 后端 (1 file, 2 lines)

`backend/internal/gateway/server_methods_channels.go`:
- [x] **Line 164**: `map[string]interface{}{}` → `[]interface{}{}`
- [x] **Line 260**: `map[string]interface{}{}` → `[]interface{}{}`

### 前端 (2 files, 3 locations)

- [x] `ui/src/ui/views/cron.ts` line 51: `Array.isArray()` 防御
- [x] `ui/src/ui/views/agents.ts` line 970: `Array.isArray()` 防御
- [x] `ui/src/ui/views/agents.ts` line 985: `Array.isArray()` 防御

---

## Fix 2: 飞书审批卡片回调 — WebSocket 长连接 `card.action.trigger` ✅

**根因**: `remote_approval_feishu.go` 按钮使用 `"url"` 字段，飞书客户端在浏览器中打开链接而非回调服务端。

**方案** (重构): 使用飞书 SDK `OnP2CardActionTrigger` 通过 WebSocket 长连接接收卡片回传交互。
按钮改为 `value` 字段，点击后飞书通过长连接 POST `card.action.trigger` 事件到 SDK EventDispatcher，
**无需公网穿透、无需配置卡片回调 URL**。

### 修改文件

- [x] `remote_approval_feishu.go`: 移除 `approveURL`/`denyURL` 拼接，按钮从 `"url"` 改为 `"value"` (携带 action/id/ttl)
- [x] `feishu/webhook.go`: 新增 `CardActionHandler` 类型 + `RegisterCardActionHandler()` 函数
- [x] `feishu/plugin.go`: 新增 `CardActionFunc` 字段，`Start()` 时注册到 EventDispatcher
- [x] `server.go`: 注入 `buildFeishuCardActionHandler()` 到 feishu plugin (解析 value → `ResolveEscalation`)
- [x] ~~`server_channel_webhooks.go`~~: HTTP 回调方案已移除，改用 WebSocket 长连接
- [x] ~~`server.go` HTTP 路由~~: `/channels/feishu/card-callback` 不再需要

### 关键设计

- SDK `dispatcher.OnP2CardActionTrigger()` 注册 `card.action.trigger` 事件处理
- 回调数据: `event.Event.Action.Value` → `{"action":"approve","id":"xxx","ttl":30}`
- 返回: `callback.CardActionTriggerResponse{Toast: {Type: "success", Content: "..."}}`
- 与消息接收共用同一条 WebSocket 出站连接

---

## Fix 3: 飞书消息通知 ✅

**根因**: 后端为飞书消息创建独立 session，前端过滤掉非当前 sessionKey 的消息，没有跨会话通知机制。

**方案**: 新增 `channel.message.incoming` 广播 + 前端通知 Toast

### 后端

- [x] `server.go` feishuDispatch: 广播 `channel.message.incoming` 事件

### 前端

- [x] `channel-notification-toast.ts`: 新文件，通知 Toast 组件 (~95 行)
- [x] `app-gateway.ts`: 处理 `channel.message.incoming` 事件
- [x] `app.ts`: 新增 `channelNotification` state
- [x] `app-view-state.ts`: 新增 `channelNotification` 类型声明
- [x] `app-render.ts`: chat tab 内渲染通知 toast，点击"查看"跳转会话

---

## 最终变更总结

| 文件 | 类型 | Fix |
|------|------|-----|
| `server_methods_channels.go` | 改 2 行 | #1 |
| `cron.ts` | 改 1 行 | #1 |
| `agents.ts` | 改 2 行 | #1 |
| `remote_approval_feishu.go` | 改 ~10 行 | #2 |
| `feishu/webhook.go` | 增 ~15 行 | #2 |
| `feishu/plugin.go` | 增 ~8 行 | #2 |
| `server.go` | 增 ~80 行 | #2+#3 |
| `channel-notification-toast.ts` | **新** ~95 行 | #3 |
| `app-gateway.ts` | 增 ~20 行 | #3 |
| `app.ts` | 增 1 行 | #3 |
| `app-view-state.ts` | 增 1 行 | #3 |
| `app-render.ts` | 增 ~25 行 | #3 |

## 验证

1. 定时任务页面无崩溃，F12 无 TypeError
2. 飞书审批卡片点击"批准" → 飞书内显示 toast "审批通过" → 无浏览器跳转 → 智能体恢复执行
3. 飞书发消息 → 网页版顶部弹出通知 Toast → 点击跳转到飞书会话

## 补充文档

- `docs/tunnel-setup.md`: 内网穿透工具使用说明 (cloudflared + ngrok)
