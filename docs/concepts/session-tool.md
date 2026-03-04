---
summary: "Agent 会话工具：列出会话、获取历史和跨会话发送消息"
read_when:
  - 添加或修改会话工具
title: "会话工具"
status: active
arch: go-gateway
---

# 会话工具

> [!NOTE]
> **架构状态**：会话工具由 **Go Gateway** 实现（`backend/internal/sessions/`）。
> Rust CLI 通过 `openacosmi sessions` 命令（`oa-cmd-sessions`）查看会话列表。

目标：小巧、不易误用的工具集，使 Agent 能列出会话、获取历史并向其他会话发送消息。

## 工具名称

- `sessions_list`
- `sessions_history`
- `sessions_send`
- `sessions_spawn`

## 键模型

- 主私聊桶始终为字面键 `"main"`（解析为当前 Agent 的主键）。
- 群组使用 `agent:<agentId>:<channel>:group:<id>` 或 `agent:<agentId>:<channel>:channel:<id>`（传完整键）。
- 定时任务使用 `cron:<job.id>`。
- Hook 使用 `hook:<uuid>`（除非显式设置）。
- Node 会话使用 `node-<nodeId>`（除非显式设置）。

`global` 和 `unknown` 是保留值，永不列出。

## sessions_list

列出会话数组。

参数：

- `kinds?: string[]` 过滤：`"main" | "group" | "cron" | "hook" | "node" | "other"` 中的任意值
- `limit?: number` 最大行数
- `activeMinutes?: number` 仅 N 分钟内更新的会话
- `messageLimit?: number` 0 = 无消息（默认 0）；>0 = 包含最后 N 条消息

行格式（JSON）：

- `key`：会话键
- `kind`：`main | group | cron | hook | node | other`
- `channel`：`whatsapp | telegram | discord | signal | imessage | webchat | feishu | dingtalk | wecom | internal | unknown`
- `displayName`、`updatedAt`、`sessionId`
- `model`、`contextTokens`、`totalTokens`

## sessions_history

获取单个会话的转录。

参数：

- `sessionKey`（必需；接受会话键或 `sessions_list` 中的 `sessionId`）
- `limit?: number` 最大消息数
- `includeTools?: boolean`（默认 false）

## sessions_send

向另一个会话发送消息。

参数：

- `sessionKey`（必需）
- `message`（必需）
- `timeoutSeconds?: number`（默认 >0；0 = 即发即忘）

行为：

- `timeoutSeconds = 0`：入队并返回 `{ runId, status: "accepted" }`。
- `timeoutSeconds > 0`：等待最多 N 秒完成，然后返回 `{ runId, status: "ok", reply }`。
- 等待超时时：`{ runId, status: "timeout", error }`。运行继续；稍后调用 `sessions_history`。
- 运行失败时：`{ runId, status: "error", error }`。
- 主运行完成后，OpenAcosmi 运行**回复循环**（Agent 间 ping-pong）。
  - 回复 `REPLY_SKIP` 停止 ping-pong。
  - 最大轮数：`session.agentToAgent.maxPingPongTurns`（0–5，默认 5）。

## sessions_spawn

在隔离会话中生成子 Agent 运行。

参数：

- `task`（必需）
- `label?`（可选；用于日志/UI）
- `agentId?`（可选；在另一个 Agent 下生成，需白名单允许）
- `model?`（可选；覆盖子 Agent 模型）
- `runTimeoutSeconds?`（默认 0；设置时在 N 秒后中止子 Agent 运行）
- `cleanup?`（`delete|keep`，默认 `keep`）

白名单：

- `agents.list[].subagents.allowAgents`：允许通过 `agentId` 使用的 Agent ID 列表（`["*"]` 允许所有）。

行为：

- 启动新的 `agent:<agentId>:subagent:<uuid>` 会话，`deliver: false`。
- 子 Agent 默认拥有完整工具集**减去会话工具**。
- 子 Agent 不允许调用 `sessions_spawn`（无子 Agent → 子 Agent 生成）。
- 始终非阻塞：立即返回 `{ status: "accepted", runId, childSessionKey }`。
- 完成后运行公告步骤并将结果发送到请求者的聊天通道。

## 安全 / 发送策略

基于策略的按通道/聊天类型阻断（非按会话 ID）：

```json5
{
  session: {
    sendPolicy: {
      rules: [
        { match: { channel: "discord", chatType: "group" }, action: "deny" }
      ],
      default: "allow"
    }
  }
}
```

运行时覆盖（按会话条目）：

- `sendPolicy: "allow" | "deny"`（未设置 = 继承配置）
- 可通过 `sessions.patch` 或 `/send on|off|inherit` 设置。

## 沙箱会话可见性

沙箱会话可使用会话工具，但默认只能看到通过 `sessions_spawn` 生成的会话。

```json5
{
  agents: {
    defaults: {
      sandbox: {
        sessionToolsVisibility: "spawned", // 或 "all"
      },
    },
  },
}
```
