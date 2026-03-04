---
summary: "Gateway 调度器的 Cron 定时任务与唤醒"
read_when:
  - 调度后台任务或唤醒
  - 配置需与心跳配合或独立运行的自动化
  - 在 Heartbeat 和 Cron 之间选择
title: "Cron 定时任务"
---

# Cron 定时任务（Gateway 调度器）

> [!IMPORTANT]
> **架构状态**：Cron 调度器由 **Go Gateway** 实现。
> 核心代码：`backend/internal/cron/`（调度引擎）、`backend/internal/gateway/server_methods_cron.go`（8 个 RPC 方法）。
> CLI 命令：Go CLI `backend/cmd/openacosmi/cmd_cron.go`（Cobra 命令，通过 RPC 调用 Gateway）。
>
> **Cron vs Heartbeat？** 参见 [Cron vs Heartbeat](/automation/cron-vs-heartbeat) 了解选择指南。

Cron 是 Gateway 的内置调度器。它持久化任务、在正确的时间唤醒 Agent，并可选择将输出投递到聊天渠道。

如果你需要 _"每天早上运行"_ 或 _"20 分钟后提醒 Agent"_，Cron 就是对应的机制。

故障排查：[自动化故障排查](/automation/troubleshooting)

## 概要

- Cron 运行在 **Gateway 内部**（不在模型内部）。
- 任务持久化在 `~/.openacosmi/cron/` 下，重启不会丢失调度。
- 两种执行方式：
  - **主会话**：入队一个 system event，在下次心跳时运行。
  - **隔离**：在 `cron:<jobId>` 中运行独立 agent turn，默认投递摘要。
- 唤醒是一等公民：任务可请求"立即唤醒"或"下次心跳"。

## 快速开始

创建一次性提醒，验证并立即运行：

```bash
openacosmi cron add \
  --name "提醒" \
  --at "2026-02-01T16:00:00Z" \
  --session main \
  --system-event "提醒：检查 cron 文档草稿" \
  --wake now \
  --delete-after-run

openacosmi cron list
openacosmi cron run <job-id>
openacosmi cron logs --id <job-id>
```

调度一个定期隔离任务并投递：

```bash
openacosmi cron add \
  --name "晨间简报" \
  --cron "0 7 * * *" \
  --tz "America/Los_Angeles" \
  --session isolated \
  --message "总结隔夜更新。" \
  --announce \
  --channel slack \
  --to "channel:C1234567890"
```

## 核心概念

### 任务（Jobs）

一个 cron 任务是一条存储记录，包含：

- **调度**（何时运行）
- **载荷**（运行什么）
- 可选的**投递模式**（announce 或 none）
- 可选的 **Agent 绑定**（`agentId`）：在指定 agent 下运行；缺失或未知时回退到默认 agent。

任务通过稳定的 `jobId` 标识（CLI / Gateway API 使用）。
一次性任务成功后默认自动删除；设置 `deleteAfterRun: false` 可保留。

### 调度类型

支持三种调度方式：

- `at`：一次性时间戳，通过 `schedule.at`（ISO 8601）。
- `every`：固定间隔（毫秒）。
- `cron`：5 字段 cron 表达式，可选 IANA 时区。

如省略时区，使用 Gateway 主机的本地时区。

### 主会话 vs 隔离执行

#### 主会话任务（system event）

主会话任务入队一个 system event 并可选唤醒心跳。必须使用 `payload.kind = "systemEvent"`。

- `wakeMode: "now"`（默认）：事件触发立即心跳。
- `wakeMode: "next-heartbeat"`：事件等待下次定期心跳。

适合需要正常心跳提示 + 主会话上下文的场景。详见 [Heartbeat](/gateway/heartbeat)。

#### 隔离任务（独立 cron 会话）

隔离任务在 `cron:<jobId>` 会话中运行独立 agent turn。

关键行为：

- 提示前缀 `[cron:<jobId> <任务名>]` 用于溯源。
- 每次运行启动**全新会话 ID**（无前次对话延续）。
- 默认行为：省略 `delivery` 时，隔离任务投递摘要（`delivery.mode = "announce"`）。
- `delivery.mode`（仅隔离任务）：
  - `announce`：向目标渠道投递摘要，并向主会话发布简短摘要。
  - `none`：仅内部处理（不投递、不发主会话摘要）。

适合频繁、嘈杂或"后台杂务"类任务。

### Payload 类型

两种 payload：

- `systemEvent`：仅主会话，通过心跳提示路由。
- `agentTurn`：仅隔离会话，运行独立 agent turn。

`agentTurn` 常用字段：

- `message`：必填文本提示。
- `model` / `thinking`：可选覆盖。
- `timeoutSeconds`：可选超时覆盖。

### 投递配置（Delivery）

隔离任务可通过 `delivery` 配置投递输出：

- `delivery.mode`：`none` | `announce`。
- `delivery.channel`：`whatsapp` / `telegram` / `discord` / `slack` / `signal` / `imessage` / `last`。
- `delivery.to`：渠道特定接收方。
- `delivery.bestEffort`：投递失败时不中断任务。

Announce 投递直接通过出站渠道适配器投递，不启动主 Agent。
`HEARTBEAT_OK`（无实质内容）的响应不会被投递。

### 模型和思考级别覆盖

隔离任务可覆盖模型和思考级别：

- `model`：Provider/model 字符串（如 `anthropic/claude-sonnet-4-20250514`）或别名（如 `opus`）
- `thinking`：思考级别（`off`、`minimal`、`low`、`medium`、`high`、`xhigh`）

优先级：任务 payload 覆盖 > Hook 特定默认值 > Agent 配置默认值。

## 工具调用 JSON Schema

通过 Gateway `cron.*` 工具直接调用时使用以下格式。CLI 接受人类可读时长（如 `20m`），但工具调用应使用 ISO 8601 字符串（`schedule.at`）和毫秒（`schedule.everyMs`）。

### cron.add 参数

一次性主会话任务（system event）：

```json
{
  "name": "提醒",
  "schedule": { "kind": "at", "at": "2026-02-01T16:00:00Z" },
  "sessionTarget": "main",
  "wakeMode": "now",
  "payload": { "kind": "systemEvent", "text": "提醒文本" },
  "deleteAfterRun": true
}
```

定期隔离任务（含投递）：

```json
{
  "name": "晨间简报",
  "schedule": { "kind": "cron", "expr": "0 7 * * *", "tz": "America/Los_Angeles" },
  "sessionTarget": "isolated",
  "wakeMode": "next-heartbeat",
  "payload": {
    "kind": "agentTurn",
    "message": "总结隔夜更新。"
  },
  "delivery": {
    "mode": "announce",
    "channel": "slack",
    "to": "channel:C1234567890",
    "bestEffort": true
  }
}
```

说明：

- `schedule.kind`：`at`（`at`）、`every`（`everyMs`）或 `cron`（`expr`，可选 `tz`）。
- `schedule.at` 接受 ISO 8601（时区可选；省略时按 UTC）。
- `sessionTarget` 必须为 `"main"` 或 `"isolated"`，且必须与 `payload.kind` 匹配。
- 可选字段：`agentId`、`description`、`enabled`、`deleteAfterRun`（`at` 类型默认 true）、`delivery`。

### cron.update 参数

```json
{
  "jobId": "job-123",
  "patch": {
    "enabled": false,
    "schedule": { "kind": "every", "everyMs": 3600000 }
  }
}
```

### cron.run 和 cron.remove 参数

```json
{ "jobId": "job-123", "mode": "force" }
```

```json
{ "jobId": "job-123" }
```

## 存储与历史

- 任务存储：`~/.openacosmi/cron/jobs.json`（Gateway 管理的 JSON）。
- 运行历史：`~/.openacosmi/cron/runs/<jobId>.jsonl`（JSONL，自动裁剪）。
- 覆盖存储路径：配置 `cron.store`。

## 配置

```json5
{
  cron: {
    enabled: true, // 默认 true
    store: "~/.openacosmi/cron/jobs.json",
    maxConcurrentRuns: 1, // 默认 1
  },
}
```

禁用 cron：

- `cron.enabled: false`（配置）
- `OPENACOSMI_SKIP_CRON=1`（环境变量）

## CLI 快速参考

一次性提醒（UTC ISO，成功后自动删除）：

```bash
openacosmi cron add \
  --name "发送提醒" \
  --at "2026-01-12T18:00:00Z" \
  --session main \
  --system-event "提醒：提交费用报告。" \
  --wake now \
  --delete-after-run
```

一次性提醒（主会话，立即唤醒）：

```bash
openacosmi cron add \
  --name "日历检查" \
  --at "20m" \
  --session main \
  --system-event "下次心跳：检查日历。" \
  --wake now
```

定期隔离任务（announce 到 WhatsApp）：

```bash
openacosmi cron add \
  --name "晨间状态" \
  --cron "0 7 * * *" \
  --tz "America/Los_Angeles" \
  --session isolated \
  --message "总结收件箱 + 今日日历。" \
  --announce \
  --channel whatsapp \
  --to "+15551234567"
```

隔离任务（含模型和思考级别覆盖）：

```bash
openacosmi cron add \
  --name "深度分析" \
  --cron "0 6 * * 1" \
  --tz "America/Los_Angeles" \
  --session isolated \
  --message "每周项目进度深度分析。" \
  --model "opus" \
  --thinking high \
  --announce \
  --channel whatsapp \
  --to "+15551234567"
```

Agent 选择（多 Agent 场景）：

```bash
# 将任务绑定到 agent "ops"
openacosmi cron add --name "Ops 巡检" --cron "0 6 * * *" --session isolated --message "检查 ops 队列" --agent ops

# 切换或清除现有任务的 agent
openacosmi cron edit <jobId> --agent ops
openacosmi cron edit <jobId> --clear-agent
```

手动运行（默认 force，用 `--due` 仅在到期时运行）：

```bash
openacosmi cron run <jobId>
openacosmi cron run <jobId> --due
```

编辑现有任务：

```bash
openacosmi cron edit <jobId> \
  --message "更新后的提示" \
  --model "opus" \
  --thinking low
```

运行历史：

```bash
openacosmi cron logs --id <jobId> --tail 50
```

无需创建任务的立即 system event：

```bash
openacosmi system event --mode now --text "下次心跳：检查电量。"
```

## Gateway API 接口

- `cron.list`、`cron.status`、`cron.add`、`cron.update`、`cron.remove`
- `cron.run`（force 或 due）、`cron.runs`

对于无需任务的立即 system event，使用 [`openacosmi system event`](/cli/system)。

## 故障排查

### "什么都没运行"

- 检查 cron 是否启用：`cron.enabled` 和 `OPENACOSMI_SKIP_CRON`。
- 确认 Gateway 持续运行中（cron 在 Gateway 进程内部运行）。
- 对于 `cron` 调度：确认时区（`--tz`）与主机时区的关系。

### 定期任务在失败后持续延迟

- OpenAcosmi 在连续错误后对定期任务应用指数退避重试：30s、1m、5m、15m，然后 60m。
- 下次成功运行后退避自动重置。
- 一次性（`at`）任务在终止运行（`ok`、`error` 或 `skipped`）后禁用，不重试。

### Telegram 投递到错误位置

- 对于论坛主题，使用 `-100…:topic:<id>` 格式以明确无歧义。
- 如果在日志或存储的"last route"目标中看到 `telegram:...` 前缀，这是正常的；cron 投递接受这些前缀并正确解析 topic ID。
