---
summary: "心跳轮询消息与通知规则"
read_when:
  - 调整心跳频率或消息行为
  - 选择心跳还是定时任务
title: "心跳（Heartbeat）"
---

# 心跳（Gateway）

> [!IMPORTANT]
> **架构状态**：心跳由 **Go Gateway**（`backend/internal/gateway/heartbeat.go`）管理。

> **心跳 vs 定时任务？** 见 [Cron vs Heartbeat](/automation/cron-vs-heartbeat)。

心跳在主会话中运行**定期 Agent 回合**，让模型自主浮现需要关注的事项。

故障排除：[/automation/troubleshooting](/automation/troubleshooting)

## 快速开始

1. 保持心跳启用（默认 `30m`）或设置自定义频率。
2. 在 agent workspace 创建 `HEARTBEAT.md` 清单（可选但推荐）。
3. 选择心跳消息去向（`target: "last"` 为默认）。

配置示例：

```json5
{
  agents: {
    defaults: {
      heartbeat: {
        every: "30m",
        target: "last",
        // activeHours: { start: "08:00", end: "24:00" },
      },
    },
  },
}
```

## 默认值

- 间隔：`30m`。设置 `0m` 禁用。
- 默认提示词指示 Agent 读取 `HEARTBEAT.md`，无事则回复 `HEARTBEAT_OK`。
- 活跃时段（`heartbeat.activeHours`）按配置时区检查。

## 响应契约

- 无需关注时回复 **`HEARTBEAT_OK`**。
- `HEARTBEAT_OK` 出现在回复开头或末尾时被识别并剥离（剩余内容 ≤ `ackMaxChars` 默认 300 字符时丢弃）。
- 告警时**不要**包含 `HEARTBEAT_OK`。

## 配置

```json5
{
  agents: {
    defaults: {
      heartbeat: {
        every: "30m",
        model: "anthropic/claude-opus-4-6",
        includeReasoning: false,
        target: "last", // last | none | <channel id>
        to: "+15551234567", // 可选频道特定覆盖
        accountId: "ops-bot", // 可选多账户频道 ID
        prompt: "Read HEARTBEAT.md ...",
        ackMaxChars: 300,
      },
    },
  },
}
```

### 作用域和优先级

- `agents.defaults.heartbeat` 设置全局行为。
- `agents.list[].heartbeat` 在其上合并；任何 agent 有 `heartbeat` 块时，**仅那些 agent** 运行心跳。
- `channels.defaults.heartbeat` 设置频道可见性默认值。

### 按 Agent 心跳

```json5
{
  agents: {
    list: [
      { id: "main", default: true },
      {
        id: "ops",
        heartbeat: {
          every: "1h",
          target: "whatsapp",
          to: "+15551234567",
        },
      },
    ],
  },
}
```

### 活跃时段

```json5
{
  agents: {
    defaults: {
      heartbeat: {
        every: "30m",
        activeHours: {
          start: "09:00",
          end: "22:00",
          timezone: "Asia/Shanghai",
        },
      },
    },
  },
}
```

### 字段说明

- `every`：心跳间隔（持续时间字符串）。
- `model`：可选模型覆盖。
- `includeReasoning`：启用时额外发送推理消息。
- `session`：可选会话键。
- `target`：`last`（默认）| 显式频道 | `none`。
- `to`：可选接收者覆盖。
- `activeHours`：限制心跳运行的时间窗口。

## 投递行为

- 心跳默认在 agent 主会话中运行。
- 主队列忙时跳过心跳，稍后重试。
- 心跳回复不保持会话活跃。

## 可见性控制

```yaml
channels:
  defaults:
    heartbeat:
      showOk: false     # 隐藏 HEARTBEAT_OK
      showAlerts: true   # 显示告警
      useIndicator: true # 发射指示器事件
```

优先级：按账户 → 按频道 → 频道默认值 → 内置默认值。

## HEARTBEAT.md（可选）

workspace 中的 `HEARTBEAT.md` 文件作为心跳清单。保持精简。

```md
# 心跳清单

- 快速扫描：有紧急事项吗？
- 白天时做轻量级签到。
```

## 手动唤醒

```bash
openacosmi system event --text "检查紧急跟进" --mode now
```

## 成本意识

心跳运行完整 Agent 回合。更短间隔消耗更多 token。保持 `HEARTBEAT.md` 精简。
