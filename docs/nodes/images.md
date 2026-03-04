---
summary: "图像和媒体处理规则：发送、Gateway 和 agent 回复"
read_when:
  - 修改媒体管道或附件处理
title: "图像与媒体支持"
---

> **架构提示 — Rust CLI + Go Gateway**
> 媒体处理管道由 Go Gateway 实现（`backend/internal/media/`），
> 包括图像操作（`image_ops.go`）、输入文件处理（`input_files.go`）和存储（`store.go`）。
> CLI 发送命令由 Rust 二进制处理（`cli-rust/crates/oa-cmd-supporting/`）。

# 图像与媒体支持

本文档描述了消息发送、Gateway 和 agent 回复中的媒体处理规则。

## 目标

- 通过 `openacosmi message send --media` 发送带可选标题的媒体。
- 允许来自 Web 收件箱的自动回复包含媒体和文本。
- 保持各类型的限制合理且可预测。

## CLI 命令

- `openacosmi message send --media <path-or-url> [--message <caption>]`
  - `--media` 可选；标题可为空以发送纯媒体。
  - `--dry-run` 输出解析后的载荷；`--json` 输出 `{ channel, to, messageId, mediaUrl, caption }`。

## WhatsApp Web 渠道行为

- 输入：本地文件路径**或** HTTP(S) URL。
- 流程：加载到缓冲区，检测媒体类型，构建正确的载荷：
  - **图像：** 缩放并重新压缩为 JPEG（最大边 2048px），目标 `agents.defaults.mediaMaxMb`（默认 5 MB），上限 6 MB。
  - **音频/语音/视频：** 直接传输，上限 16 MB；音频作为语音消息发送（`ptt: true`）。
  - **文档：** 其他类型，上限 100 MB，保留文件名（如可用）。
- WhatsApp GIF 风格播放：对 MP4 设置 `gifPlayback: true`（CLI：`--gif-playback`），移动客户端会循环内联播放。
- MIME 检测优先级：magic bytes → headers → 文件扩展名。
- 标题来自 `--message` 或 `reply.text`；允许空标题。
- 日志：非 verbose 模式显示 `↩️`/`✅`；verbose 模式包含大小和源路径/URL。

Go 实现：`backend/internal/media/image_ops.go`（图像缩放/压缩）。

## 自动回复管道

- `getReplyFromConfig` 返回 `{ text?, mediaUrl?, mediaUrls? }`。
- 存在媒体时，发送器使用与 `openacosmi message send` 相同的管道解析本地路径或 URL。
- 多媒体条目按顺序依次发送。

## 入站媒体到命令

- 入站消息包含媒体时，Go Gateway 下载到临时文件并暴露模板变量：
  - `{{MediaUrl}}`：入站媒体的伪 URL。
  - `{{MediaPath}}`：运行命令前写入的本地临时路径。
- 启用每会话 Docker 沙箱时，入站媒体被复制到沙箱工作区，`MediaPath`/`MediaUrl` 被重写为相对路径（如 `media/inbound/<filename>`）。
- 媒体理解（通过 `tools.media.*` 或共享 `tools.media.models` 配置）在模板化之前运行，可将 `[Image]`、`[Audio]` 和 `[Video]` 块插入 `Body`。
  - 音频设置 `{{Transcript}}` 并使用转写文本进行命令解析，斜杠命令仍可工作。
  - 视频和图像描述保留标题文本用于命令解析。
- 默认仅处理第一个匹配的图像/音频/视频附件；设置 `tools.media.<cap>.attachments` 处理多个附件。

Go 实现：`backend/internal/media/input_files.go`（入站媒体处理）。

## 限制与错误

**出站发送上限（WhatsApp Web 发送）**

- 图像：重新压缩后约 6 MB 上限。
- 音频/语音/视频：16 MB 上限；文档：100 MB 上限。
- 超大或不可读媒体：日志中显示明确错误，回复被跳过。

**媒体理解上限（转写/描述）**

- 图像默认：10 MB（`tools.media.image.maxBytes`）。
- 音频默认：20 MB（`tools.media.audio.maxBytes`）。
- 视频默认：50 MB（`tools.media.video.maxBytes`）。
- 超大媒体跳过理解，但回复仍以原始 body 发送。

## 测试说明

- 覆盖图像/音频/文档的发送和回复流程。
- 验证图像重新压缩（大小限制）和音频语音消息标志。
- 确保多媒体回复以顺序发送展开。
