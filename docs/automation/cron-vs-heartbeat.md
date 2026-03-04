---
summary: "在 Heartbeat 和 Cron 定时任务之间选择的指南"
read_when:
  - 决定如何调度周期性任务
  - 设置后台监控或通知
  - 优化定期检查的 token 消耗
title: "Cron vs Heartbeat"
---

# Cron vs Heartbeat：何时使用哪个

> [!IMPORTANT]
> **架构状态**：Cron 调度器和 Heartbeat 均由 **Go Gateway** 实现。
> Cron：`backend/internal/cron/`、RPC 方法：`backend/internal/gateway/server_methods_cron.go`。
> CLI 命令：Go CLI `backend/cmd/openacosmi/cmd_cron.go`。

Heartbeat 和 Cron 都可以按计划运行任务。本指南帮助你为特定场景选择正确的机制。

## 快速决策表

| 使用场景 | 推荐方式 | 原因 |
| --- | --- | --- |
| 每 30 分钟检查收件箱 | Heartbeat | 可与其他检查批量执行，上下文感知 |
| 每天 9 点准时发报告 | Cron（隔离） | 需要精确定时 |
| 监控日历的即将到来的事件 | Heartbeat | 适合周期性感知 |
| 运行每周深度分析 | Cron（隔离） | 独立任务，可使用不同模型 |
| 20 分钟后提醒我 | Cron（主会话，`--at`） | 一次性精确定时 |
| 后台项目健康检查 | Heartbeat | 搭载现有心跳周期 |

## Heartbeat：周期性感知

Heartbeat 在**主会话**中以固定间隔（默认 30 分钟）运行。设计用于让 Agent 检查各项事务并报告重要信息。

### 适合使用 Heartbeat 的场景

- **多项周期性检查**：一次心跳可批量检查收件箱、日历、天气、通知和项目状态，而非创建 5 个独立 cron 任务。
- **上下文感知决策**：Agent 拥有完整主会话上下文，可智能判断什么紧急、什么可以等待。
- **对话连续性**：心跳运行共享同一会话，Agent 记住最近的对话并可自然跟进。
- **低开销监控**：一次心跳替代多个小型轮询任务。

### Heartbeat 优势

- **批量多项检查**：一次 agent turn 可同时检查收件箱、日历和通知。
- **减少 API 调用**：一次心跳比 5 个独立 cron 任务更经济。
- **上下文感知**：Agent 知道你在做什么，可相应排列优先级。
- **智能抑制**：如果没有需要关注的事项，Agent 回复 `HEARTBEAT_OK` 且不投递消息。
- **自然定时**：基于队列负载略有漂移，对大多数监控来说足够。

### Heartbeat 示例：HEARTBEAT.md 检查清单

```md
# 心跳检查清单

- 检查邮件中的紧急消息
- 查看未来 2 小时内的日历事件
- 如果后台任务已完成，总结结果
- 如果空闲超过 8 小时，发送简短签到
```

Agent 在每次心跳时读取此文件并在一次 turn 中处理所有项目。

### 配置 Heartbeat

```json5
{
  agents: {
    defaults: {
      heartbeat: {
        every: "30m", // 间隔
        target: "last", // 告警投递目标
        activeHours: { start: "08:00", end: "22:00" }, // 可选
      },
    },
  },
}
```

详见 [Heartbeat](/gateway/heartbeat)。

## Cron：精确调度

Cron 任务在**精确时间**运行，可在隔离会话中运行而不影响主上下文。

### 适合使用 Cron 的场景

- **需要精确定时**："每周一 9:00 AM 发送"（而非"大约 9 点左右"）。
- **独立任务**：不需要对话上下文的任务。
- **不同模型/思考级别**：需要更强模型的重度分析。
- **一次性提醒**：用 `--at` 实现"20 分钟后提醒我"。
- **频繁/嘈杂的任务**：会干扰主会话历史的任务。
- **外部触发**：应独立于 Agent 其他活动运行的任务。

### Cron 优势

- **精确定时**：支持 5 字段 cron 表达式和时区。
- **会话隔离**：在 `cron:<jobId>` 中运行，不污染主历史。
- **模型覆盖**：可按任务使用更便宜或更强的模型。
- **投递控制**：隔离任务默认 `announce`（投递摘要）；可选 `none`。
- **即时投递**：announce 模式直接发送，无需等待心跳。
- **无需 Agent 上下文**：即使主会话空闲或已压缩也能运行。
- **一次性支持**：`--at` 用于精确未来时间戳。

### Cron 示例：每日晨报

```bash
openacosmi cron add \
  --name "晨间简报" \
  --cron "0 7 * * *" \
  --tz "America/New_York" \
  --session isolated \
  --message "生成今日简报：天气、日历、重要邮件、新闻摘要。" \
  --model opus \
  --announce \
  --channel whatsapp \
  --to "+15551234567"
```

此任务在纽约时间每天早上 7:00 准时运行，使用 Opus 模型以保证质量，并将摘要直接推送到 WhatsApp。

### Cron 示例：一次性提醒

```bash
openacosmi cron add \
  --name "会议提醒" \
  --at "20m" \
  --session main \
  --system-event "提醒：站会将在 10 分钟后开始。" \
  --wake now \
  --delete-after-run
```

详见 [Cron 定时任务](/automation/cron-jobs)。

## 决策流程图

```
任务是否需要在精确时间运行？
  是 → 使用 Cron
  否 → 继续...

任务是否需要与主会话隔离？
  是 → 使用 Cron（隔离）
  否 → 继续...

此任务能否与其他周期性检查合并？
  是 → 使用 Heartbeat（添加到 HEARTBEAT.md）
  否 → 使用 Cron

这是一次性提醒吗？
  是 → 使用 Cron + --at
  否 → 继续...

是否需要不同的模型或思考级别？
  是 → 使用 Cron（隔离）+ --model/--thinking
  否 → 使用 Heartbeat
```

## 组合使用

最高效的方案是**同时使用两者**：

1. **Heartbeat** 处理日常监控（收件箱、日历、通知），每 30 分钟批量执行一次。
2. **Cron** 处理精确调度（每日报告、每周总结）和一次性提醒。

### 示例：高效的自动化配置

**HEARTBEAT.md**（每 30 分钟检查）：

```md
# 心跳检查清单

- 扫描收件箱中的紧急邮件
- 检查未来 2 小时内的日历事件
- 查看待处理任务
- 如果安静超过 8 小时，轻量签到
```

**Cron 任务**（精确定时）：

```bash
# 每天早上 7 点晨报
openacosmi cron add --name "晨间简报" --cron "0 7 * * *" --session isolated --message "..." --announce

# 每周一早上 9 点项目总结
openacosmi cron add --name "每周总结" --cron "0 9 * * 1" --session isolated --message "..." --model opus

# 一次性提醒
openacosmi cron add --name "回电" --at "2h" --session main --system-event "回电给客户" --wake now
```

## Lobster：带审批的确定性工作流

Lobster 是用于**多步骤工具管道**的工作流运行时，需要确定性执行和显式审批。当任务超过单次 agent turn 且你需要带人工检查点的可恢复工作流时使用。

### Lobster 适用场景

- **多步骤自动化**：需要固定的工具调用管道，而非一次性提示。
- **审批门控**：副作用应在你批准前暂停，然后恢复。
- **可恢复运行**：继续暂停的工作流而无需重新执行前面的步骤。

### 与 Heartbeat 和 Cron 的配合

- **Heartbeat / Cron** 决定 _何时_ 运行。
- **Lobster** 定义运行开始后 _执行哪些步骤_。

对于定时工作流，使用 cron 或 heartbeat 触发一次调用 Lobster 的 agent turn。对于临时工作流，直接调用 Lobster。

### 运行说明

- Lobster 作为**本地子进程**（`lobster` CLI）以 tool 模式运行，返回 **JSON 封装**。
- 如果工具返回 `needs_approval`，使用 `resumeToken` 和 `approve` 标志恢复。
- 该工具为**可选插件**；通过 `tools.alsoAllow: ["lobster"]` 额外启用（推荐）。
- 如传入 `lobsterPath`，必须是**绝对路径**。

详见 [Lobster](/tools/lobster)。

## 主会话 vs 隔离会话

Heartbeat 和 Cron 都可与主会话交互，但方式不同：

|  | Heartbeat | Cron（主会话） | Cron（隔离） |
| --- | --- | --- | --- |
| 会话 | 主会话 | 主会话（通过 system event） | `cron:<jobId>` |
| 历史 | 共享 | 共享 | 每次运行全新 |
| 上下文 | 完整 | 完整 | 无（干净启动） |
| 模型 | 主会话模型 | 主会话模型 | 可覆盖 |
| 输出 | 非 `HEARTBEAT_OK` 时投递 | 心跳提示 + 事件 | 默认 announce 摘要 |

### 何时使用主会话 Cron

使用 `--session main` + `--system-event` 当你想要：

- 提醒/事件出现在主会话上下文中
- Agent 在下次心跳时以完整上下文处理
- 不产生独立隔离运行

```bash
openacosmi cron add \
  --name "检查项目" \
  --every "4h" \
  --session main \
  --system-event "该做项目健康检查了" \
  --wake now
```

### 何时使用隔离 Cron

使用 `--session isolated` 当你想要：

- 没有先前上下文的干净环境
- 不同的模型或思考设置
- 直接向渠道投递 announce 摘要
- 不干扰主会话的历史记录

```bash
openacosmi cron add \
  --name "深度分析" \
  --cron "0 6 * * 0" \
  --session isolated \
  --message "每周代码仓库分析..." \
  --model opus \
  --thinking high \
  --announce
```

## 成本考量

| 机制 | 成本特征 |
| --- | --- |
| Heartbeat | 每 N 分钟一次 turn；随 HEARTBEAT.md 大小增长 |
| Cron（主会话） | 将事件添加到下次心跳（无隔离 turn） |
| Cron（隔离） | 每个任务一次完整 agent turn；可使用更便宜的模型 |

**建议**：

- 保持 `HEARTBEAT.md` 简洁以最小化 token 开销。
- 将类似检查合并到心跳中，而非创建多个 cron 任务。
- 如果只需内部处理，对心跳使用 `target: "none"`。
- 对常规任务使用隔离 cron + 更便宜的模型。

## 相关文档

- [Heartbeat 心跳](/gateway/heartbeat) — 完整心跳配置
- [Cron 定时任务](/automation/cron-jobs) — 完整 cron CLI 和 API 参考
- [System](/cli/system) — system event + 心跳控制
