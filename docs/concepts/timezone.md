---
summary: "时区处理：Agent、消息信封和提示词中的时间戳"
read_when:
  - 了解时间戳如何为模型标准化
  - 配置系统提示词中的用户时区
title: "时区"
status: active
arch: go-gateway
---

# 时区

> [!NOTE]
> **架构状态**：时区处理由 **Go Gateway** 实现（`backend/internal/agents/runner/`）。

OpenAcosmi 标准化时间戳使模型看到**单一参考时间**。

## 消息信封（默认主机本地时间）

入站消息被包装在信封中：

```
[Provider ... 2026-01-05 16:26 PST] 消息文本
```

信封中的时间戳默认为**主机本地时间**，精确到分钟。

覆盖配置：

```json5
{
  agent: {
    envelopeTimezone: "local",    // "utc" | "local" | "user" | IANA 时区
    envelopeTimestamp: "on",      // "on" | "off"
    envelopeElapsed: "on",        // "on" | "off"
  },
}
```

- `envelopeTimezone: "utc"` 使用 UTC。
- `envelopeTimezone: "user"` 使用 `agents.defaults.userTimezone`（回退到主机时区）。
- 使用显式 IANA 时区（如 `"Asia/Shanghai"`）设定固定偏移。
- `envelopeTimestamp: "off"` 移除信封头中的绝对时间戳。
- `envelopeElapsed: "off"` 移除经过时间后缀（`+2m` 样式）。

### 示例

**本地（默认）：**

```
[Signal Alice +1555 2026-01-18 00:19 PST] 你好
```

**固定时区：**

```
[Signal Alice +1555 2026-01-18 08:19 CST] 你好
```

**经过时间：**

```
[Signal Alice +1555 +2m 2026-01-18T05:19Z] 跟进
```

## 工具载荷（原始 Provider 数据 + 标准化字段）

工具调用返回**原始 Provider 时间戳**。同时附加标准化字段：

- `timestampMs`（UTC 纪元毫秒）
- `timestampUtc`（ISO 8601 UTC 字符串）

原始 Provider 字段保留。

## 系统提示词中的用户时区

设置 `agents.defaults.userTimezone` 告知模型用户的本地时区。未设置时，OpenAcosmi 在运行时解析**主机时区**（不写入配置）。

```json5
{
  agent: { userTimezone: "Asia/Shanghai" },
}
```

系统提示词包含：

- `当前日期与时间` 段落（含本地时间和时区）
- `时间格式: 12小时制` 或 `24小时制`

通过 `agents.defaults.timeFormat`（`auto` | `12` | `24`）控制提示词格式。

参见 [日期与时间](/date-time)。
