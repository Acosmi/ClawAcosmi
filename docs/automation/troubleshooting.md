---
summary: "Cron 和 Heartbeat 调度与投递的故障排查"
read_when:
  - Cron 任务未运行
  - Cron 运行了但未投递消息
  - Heartbeat 似乎静默或被跳过
title: "自动化故障排查"
---

# 自动化故障排查

> [!IMPORTANT]
> **架构状态**：Cron 调度和 Heartbeat 均由 **Go Gateway**（`backend/internal/cron/`）实现。
> CLI 诊断命令由 Go CLI（`backend/cmd/openacosmi/`）提供。

本页面用于排查调度器和投递问题（`cron` + `heartbeat`）。

## 诊断命令梯队

```bash
openacosmi status
openacosmi gateway status
openacosmi logs --follow
openacosmi doctor
openacosmi channels status --probe
```

然后运行自动化专项检查：

```bash
openacosmi cron status
openacosmi cron list
openacosmi system heartbeat last
```

## Cron 未触发

```bash
openacosmi cron status
openacosmi cron list
openacosmi cron runs --id <jobId> --limit 20
openacosmi logs --follow
```

正常输出特征：

- `cron status` 报告已启用且有未来的 `nextWakeAtMs`。
- 任务已启用且有有效的调度/时区。
- `cron runs` 显示 `ok` 或明确的跳过原因。

常见特征：

- `cron: scheduler disabled; jobs will not run automatically` → cron 在配置/环境变量中被禁用。
- `cron: timer tick failed` → 调度器 tick 崩溃；检查周围的堆栈/日志上下文。
- `reason: not-due` → 手动运行时未使用 `--force` 且任务尚未到期。

## Cron 触发但无投递

```bash
openacosmi cron runs --id <jobId> --limit 20
openacosmi cron list
openacosmi channels status --probe
openacosmi logs --follow
```

正常输出特征：

- 运行状态为 `ok`。
- 隔离任务已设置 delivery mode 和 target。
- Channel probe 报告目标渠道已连接。

常见特征：

- 运行成功但 delivery mode 为 `none` → 不会发送外部消息。
- delivery target 缺失或无效（`channel`/`to`） → 运行可能内部成功但跳过外部投递。
- 渠道认证错误（`unauthorized`、`missing_scope`、`Forbidden`） → 投递被渠道凭证/权限阻止。

## Heartbeat 被抑制或跳过

```bash
openacosmi system heartbeat last
openacosmi logs --follow
openacosmi config get agents.defaults.heartbeat
openacosmi channels status --probe
```

正常输出特征：

- Heartbeat 已启用且间隔非零。
- 上次心跳结果为 `ran`（或跳过原因已知）。

常见特征：

- `heartbeat skipped` 且 `reason=quiet-hours` → 不在 `activeHours` 时段内。
- `requests-in-flight` → 主通道忙碌；心跳被延迟。
- `empty-heartbeat-file` → `HEARTBEAT.md` 存在但无可执行内容。
- `alerts-disabled` → 可见性设置抑制了外发心跳消息。

## 时区与 activeHours 陷阱

```bash
openacosmi config get agents.defaults.heartbeat.activeHours
openacosmi config get agents.defaults.heartbeat.activeHours.timezone
openacosmi config get agents.defaults.userTimezone || echo "agents.defaults.userTimezone not set"
openacosmi cron list
openacosmi logs --follow
```

快速规则：

- `Config path not found: agents.defaults.userTimezone` 表示该键未设置；heartbeat 回退到主机时区（或 `activeHours.timezone`，如果已设置）。
- Cron 未指定 `--tz` 时使用 Gateway 主机时区。
- Heartbeat `activeHours` 使用配置的时区解析方式（`user`、`local` 或显式 IANA 时区）。
- ISO 时间戳不含时区的，cron `at` 调度按 UTC 处理。

常见特征：

- 主机时区变更后，任务在错误的 wall-clock 时间运行。
- 因为 `activeHours.timezone` 配置错误，Heartbeat 在白天时段总是被跳过。

相关文档：

- [Cron 定时任务](/automation/cron-jobs)
- [Heartbeat 心跳](/gateway/heartbeat)
- [Cron vs Heartbeat](/automation/cron-vs-heartbeat)
- [时区](/concepts/timezone)
