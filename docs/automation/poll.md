---
summary: "通过 Gateway + CLI 发送投票消息"
read_when:
  - 添加或修改投票功能
  - 调试 CLI 或 Gateway 的投票发送
title: "投票"
---

# 投票（Poll）

> [!IMPORTANT]
> **架构状态**：投票发送逻辑由 **Go Gateway**（`backend/pkg/polls/polls.go`）实现，
> CLI 通过 `openacosmi message poll` 调用 Gateway RPC。

## 支持渠道

- WhatsApp（Web Channel）
- Discord
- MS Teams（Adaptive Cards）

## CLI 用法

```bash
# WhatsApp
openacosmi message poll --target +15555550123 \
  --poll-question "今天午餐？" --poll-option "好" --poll-option "不好" --poll-option "随便"
openacosmi message poll --target 123456789@g.us \
  --poll-question "会议时间？" --poll-option "10am" --poll-option "2pm" --poll-option "4pm" --poll-multi

# Discord
openacosmi message poll --channel discord --target channel:123456789 \
  --poll-question "零食？" --poll-option "披萨" --poll-option "寿司"
openacosmi message poll --channel discord --target channel:123456789 \
  --poll-question "方案？" --poll-option "A" --poll-option "B" --poll-duration-hours 48

# MS Teams
openacosmi message poll --channel msteams --target conversation:19:abc@thread.tacv2 \
  --poll-question "午餐？" --poll-option "披萨" --poll-option "寿司"
```

参数说明：

- `--channel`：`whatsapp`（默认）、`discord` 或 `msteams`
- `--poll-multi`：允许多选
- `--poll-duration-hours`：仅 Discord 有效（省略时默认 24 小时）

## Gateway RPC

方法：`poll`

参数：

- `to`（string，必填）
- `question`（string，必填）
- `options`（string[]，必填）
- `maxSelections`（number，可选）
- `durationHours`（number，可选）
- `channel`（string，可选，默认 `whatsapp`）
- `idempotencyKey`（string，必填）

## 渠道差异

- **WhatsApp**：2-12 个选项，`maxSelections` 必须在选项数量范围内，忽略 `durationHours`。
- **Discord**：2-10 个选项，`durationHours` 限制为 1-768 小时（默认 24）。`maxSelections > 1` 启用多选；Discord 不支持精确选择数量限制。
- **MS Teams**：Adaptive Card 投票（OpenAcosmi 自行管理）。无原生投票 API；忽略 `durationHours`。

## Agent 工具（Message）

使用 `message` 工具的 `poll` 动作（`to`、`pollQuestion`、`pollOption`，可选 `pollMulti`、`pollDurationHours`、`channel`）。

注意：Discord 没有"恰好选 N 个"的模式；`pollMulti` 映射为多选。
Teams 投票以 Adaptive Card 形式渲染，需要 Gateway 持续在线以将投票数据记录到 `~/.openacosmi/msteams-polls.json`。
