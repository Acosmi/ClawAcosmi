---
summary: "会话管理规则、键和持久化"
read_when:
  - 修改会话处理或存储
title: "会话管理"
status: active
arch: go-gateway
---

# 会话管理

> [!IMPORTANT]
> **架构状态**：会话管理由 **Go Gateway** 实现（`backend/internal/sessions/`）。
> Rust CLI 通过 `openacosmi sessions` 命令（`oa-cmd-sessions`）查看和管理会话。

OpenAcosmi 将**每个 Agent 一个主私聊会话**作为主要会话。私聊折叠到 `agent:<agentId>:<mainKey>`（默认 `main`），群组/频道各有独立键。`session.mainKey` 受尊重。

使用 `session.dmScope` 控制**私聊消息**的分组方式：

- `main`（默认）：所有私聊共享主会话以保持连续性。
- `per-peer`：按发送者 ID 跨通道隔离。
- `per-channel-peer`：按通道 + 发送者隔离（推荐用于多用户收件箱）。
- `per-account-channel-peer`：按账户 + 通道 + 发送者隔离（推荐用于多账户收件箱）。

使用 `session.identityLinks` 将 provider 前缀的 peer ID 映射到规范身份，使同一人在不同通道间共享私聊会话。

## 安全私聊模式（多用户场景强烈推荐）

> **安全警告：** 如果你的 Agent 可从**多人**接收私聊，强烈建议启用安全私聊模式。否则所有用户共享相同的对话上下文，可能在用户间泄露私密信息。

**问题示例（默认设置下）：**

- Alice 向你的 Agent 发消息讨论私密话题
- Bob 发消息问"我们之前在聊什么？"
- 因为两个私聊共享同一会话，模型可能用 Alice 的上下文回答 Bob

**解决方案：** 设置 `dmScope` 按用户隔离会话：

```json5
{
  session: {
    dmScope: "per-channel-peer",
  },
}
```

## Gateway 是真实来源

所有会话状态由 **Gateway 拥有**。UI 客户端（macOS app、WebChat 等）必须查询 Gateway 获取会话列表和 token 计数，而非读取本地文件。

- **远程模式**下，关心的会话存储在远程 Gateway 主机上，不在你的 Mac 上。
- UI 中显示的 token 计数来自 Gateway 的存储字段。客户端不解析 JSONL 转录来"修正"总数。

## 状态存储位置

- **Gateway 主机**上：
  - 存储文件：`~/.openacosmi/agents/<agentId>/sessions/sessions.json`（按 Agent）。
  - 转录：`~/.openacosmi/agents/<agentId>/sessions/<SessionId>.jsonl`。
- 存储是 `sessionKey -> { sessionId, updatedAt, ... }` 的映射。删除条目是安全的；会按需重建。
- 群组条目可包含 `displayName`、`channel`、`subject` 等用于在 UI 中标记会话。

## 传输层 → 会话键映射

- 私聊遵循 `session.dmScope`（默认 `main`）：
  - `main`：`agent:<agentId>:<mainKey>`（跨设备/通道的连续性）。
  - `per-peer`：`agent:<agentId>:dm:<peerId>`。
  - `per-channel-peer`：`agent:<agentId>:<channel>:dm:<peerId>`。
  - `per-account-channel-peer`：`agent:<agentId>:<channel>:<accountId>:dm:<peerId>`。
- 群组隔离：`agent:<agentId>:<channel>:group:<id>`。
- 其他来源：
  - 定时任务：`cron:<job.id>`
  - Webhook：`hook:<uuid>`
  - Node 运行：`node-<nodeId>`

## 生命周期

- 重置策略：会话重用直到过期，过期在下一条入站消息时评估。
- 每日重置：默认 **Gateway 主机本地时间凌晨 4:00**。
- 空闲重置（可选）：`idleMinutes` 添加滑动空闲窗口。
- 按类型覆盖（可选）：`resetByType` 可为 `direct`、`group`、`thread` 覆盖策略。
- 按通道覆盖（可选）：`resetByChannel` 为某通道覆盖策略。
- 重置触发器：`/new` 或 `/reset` 启动新会话 ID。

## 发送策略（可选）

按会话类型阻断投递：

```json5
{
  session: {
    sendPolicy: {
      rules: [
        { action: "deny", match: { channel: "discord", chatType: "group" } },
      ],
      default: "allow",
    },
  },
}
```

运行时覆盖（仅所有者）：

- `/send on` → 此会话允许
- `/send off` → 此会话拒绝
- `/send inherit` → 清除覆盖并使用配置规则

## 配置示例

```json5
{
  session: {
    dmScope: "main",
    identityLinks: {
      alice: ["telegram:123456789", "discord:987654321012345678"],
    },
    reset: {
      mode: "daily",
      atHour: 4,
      idleMinutes: 120,
    },
    resetByType: {
      thread: { mode: "daily", atHour: 4 },
      direct: { mode: "idle", idleMinutes: 240 },
      group: { mode: "idle", idleMinutes: 120 },
    },
  },
}
```

## 检查

- `openacosmi status` — 显示存储路径和近期会话。
- `openacosmi sessions --json` — 转储所有条目（用 `--active <minutes>` 过滤）。
- 发送 `/status` 查看 Agent 是否可达、上下文使用量及当前设置。
- 发送 `/context list` 或 `/context detail` 查看系统提示词内容。
- 发送 `/stop` 中止当前运行并清除排队的后续任务。
- 发送 `/compact` 总结旧上下文并释放窗口空间。

## 提示

- 保持主键专用于 1:1 流量；让群组保持各自的键。
- 自动化清理时，删除单个键而非整个存储以保留其他上下文。
