---
summary: "流式与分块行为（Block 回复、Draft 流式、限制）"
read_when:
  - 修改流式或分块行为
  - 排查重复的 Block 回复或 Draft 流式
title: "流式与分块"
status: active
arch: go-gateway
---

# 流式与分块

> [!NOTE]
> **架构状态**：流式与分块由 **Go Gateway** 实现（`backend/internal/outbound/`）。

OpenAcosmi 有两个独立的"流式"层：

- **Block streaming（通道消息）**：将完成的文本**块**作为普通通道消息发送。非 token 增量。
- **Token-ish streaming（仅 Telegram）**：通过 Draft 气泡更新部分文本。

目前**没有**真正的 token 级流式到外部通道消息。Telegram Draft 是唯一的部分流式表面。

## Block Streaming（通道消息）

Block streaming 在助手输出可用时以粗粒度块发送。

**控制项：**

- `agents.defaults.blockStreamingDefault`：`"on"` / `"off"`（默认 off）。
- 通道覆盖：`*.blockStreaming`（和按账户变体）强制 `"on"` / `"off"`。
- `agents.defaults.blockStreamingBreak`：`"text_end"` 或 `"message_end"`。
- `agents.defaults.blockStreamingChunk`：`{ minChars, maxChars, breakPreference? }`。
- `agents.defaults.blockStreamingCoalesce`：`{ minChars?, maxChars?, idleMs? }`。
- 通道硬上限：`*.textChunkLimit`。
- 通道分块模式：`*.chunkMode`（`length` 默认，`newline` 在空行处分割）。
- Discord 软上限：`channels.discord.maxLinesPerMessage`（默认 17）。

**边界语义：**

- `text_end`：块就绪时立即发送。
- `message_end`：等待消息完成后一次性发送（仍可能因超过 maxChars 而分为多块）。

## 分块算法

由 `EmbeddedBlockChunker` 实现：

- **低阈值**：缓冲区 >= `minChars` 前不发射（除非强制）。
- **高阈值**：优先在 `maxChars` 前分割；强制时在 `maxChars` 处硬切。
- **断行偏好**：段落 → 换行 → 句子 → 空格 → 硬切。
- **代码围栏**：永不在围栏内分割；强制时关闭 + 重新打开围栏以保持 Markdown 合法。

## 合并（减少单行刷屏）

启用 Block streaming 时，可**合并连续块**后再发送：

- 等待**空闲间隙**（`idleMs`）后刷新。
- `maxChars` 上限触发刷新。
- `minChars` 防止微小片段发送。
- 连接符由 `breakPreference` 决定。
- Signal/Slack/Discord 默认 `minChars` 提升到 1500。

## 类人节奏

启用 Block streaming 时，可在 block 回复间添加**随机停顿**：

- 配置：`agents.defaults.humanDelay`。
- 模式：`off`（默认）、`natural`（800–2500ms）、`custom`（`minMs` / `maxMs`）。
- 仅应用于**block 回复**，不影响最终回复或工具摘要。

## Telegram Draft Streaming

Telegram 是唯一支持 Draft streaming 的通道：

- `channels.telegram.streamMode`：`"partial"` | `"block"` | `"off"`。
- `partial`：Draft 更新最新流式文本。
- `block`：Draft 以块更新。
- Draft chunk 配置（仅 `block` 模式）：`channels.telegram.draftChunk`。
- 最终回复仍为普通消息。
- Draft streaming 启用时自动禁用 Block streaming 以避免双重流式。
