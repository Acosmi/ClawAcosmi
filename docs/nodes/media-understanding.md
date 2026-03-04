---
summary: "入站图像/音频/视频理解（可选）：provider + CLI 回退机制"
read_when:
  - 设计或重构媒体理解功能
  - 调优入站音频/视频/图像预处理
title: "媒体理解"
---

> **架构提示 — Rust CLI + Go Gateway**
> 媒体理解管道由 Go Gateway 实现（`backend/internal/media/`），
> 包括 STT 转写（`stt.go`）、图像操作（`image_ops.go`）和输入文件处理（`input_files.go`）。

# 媒体理解（入站）

OpenAcosmi Go Gateway 可以在回复管道运行前**预处理入站媒体**（图像/音频/视频）。当本地工具或 provider 密钥可用时自动检测，可禁用或自定义。如果理解功能关闭，模型仍会照常接收原始文件/URL。

## 目标

- 可选功能：将入站媒体预消化为短文本，加速路由和改善命令解析。
- 始终保留原始媒体传递给模型。
- 支持 **provider API** 和 **CLI 回退**。
- 允许多个模型按顺序回退（错误/大小/超时）。

## 高层行为

1. 收集入站附件（`MediaPaths`、`MediaUrls`、`MediaTypes`）。
2. 对每个启用的能力（图像/音频/视频），按策略选择附件（默认：**第一个**）。
3. 选择第一个符合条件的模型条目（大小 + 能力 + 认证）。
4. 如果模型失败或媒体过大，**回退到下一个条目**。
5. 成功时：
   - `Body` 变为 `[Image]`、`[Audio]` 或 `[Video]` 块。
   - 音频设置 `{{Transcript}}`；命令解析在有标题文本时使用标题，否则使用转写文本。
   - 标题保留为块内的 `User text:`。

如果理解失败或被禁用，**回复流程继续**使用原始 body + 附件。

## 配置概览

`tools.media` 支持**共享模型列表**和按能力单独覆盖：

- `tools.media.models`：共享模型列表（使用 `capabilities` 控制适用范围）。
- `tools.media.image` / `tools.media.audio` / `tools.media.video`：
  - 默认值（`prompt`、`maxChars`、`maxBytes`、`timeoutSeconds`、`language`）
  - provider 覆盖（`baseUrl`、`headers`、`providerOptions`）
  - Deepgram 音频选项：`tools.media.audio.providerOptions.deepgram`
  - 可选的**按能力 `models` 列表**（优先于共享模型）
  - `attachments` 策略（`mode`、`maxAttachments`、`prefer`）
  - `scope`（按渠道/chatType/会话键的可选控制）
- `tools.media.concurrency`：最大并发能力运行数（默认 **2**）。

```json5
{
  tools: {
    media: {
      models: [
        /* 共享列表 */
      ],
      image: {
        /* 可选覆盖 */
      },
      audio: {
        /* 可选覆盖 */
      },
      video: {
        /* 可选覆盖 */
      },
    },
  },
}
```

### 模型条目

每个 `models[]` 条目可以是 **provider** 或 **CLI** 类型：

```json5
{
  type: "provider", // 省略时默认
  provider: "openai",
  model: "gpt-5.2",
  prompt: "Describe the image in <= 500 chars.",
  maxChars: 500,
  maxBytes: 10485760,
  timeoutSeconds: 60,
  capabilities: ["image"], // 可选，用于多模态条目
  profile: "vision-profile",
  preferredProfile: "vision-fallback",
}
```

```json5
{
  type: "cli",
  command: "gemini",
  args: [
    "-m",
    "gemini-3-flash",
    "--allowed-tools",
    "read_file",
    "Read the media at {{MediaPath}} and describe it in <= {{MaxChars}} characters.",
  ],
  maxChars: 500,
  maxBytes: 52428800,
  timeoutSeconds: 120,
  capabilities: ["video", "image"],
}
```

CLI 模板还可使用：

- `{{MediaDir}}`（包含媒体文件的目录）
- `{{OutputDir}}`（为本次运行创建的临时目录）
- `{{OutputBase}}`（临时文件基础路径，无扩展名）

## 默认值和限制

推荐默认值：

- `maxChars`：图像/视频为 **500**（简短、命令友好）
- `maxChars`：音频为**未设置**（完整转写，除非设置限制）
- `maxBytes`：
  - 图像：**10MB**
  - 音频：**20MB**
  - 视频：**50MB**

规则：

- 如果媒体超过 `maxBytes`，该模型被跳过，**尝试下一个模型**。
- 如果模型返回超过 `maxChars`，输出被截断。
- `prompt` 默认为简单的 "Describe the {media}." 加 `maxChars` 指导（仅图像/视频）。
- 如果 `<capability>.enabled: true` 但未配置模型，Go Gateway 在 provider 支持该能力时尝试**活动回复模型**。

### 自动检测媒体理解（默认）

如果 `tools.media.<capability>.enabled` **未**设置为 `false` 且未配置模型，
Go Gateway 按以下顺序自动检测并在**第一个可用选项处停止**：

1. **本地 CLI**（仅音频；如已安装）
   - `sherpa-onnx-offline`（需要 `SHERPA_ONNX_MODEL_DIR` 包含 encoder/decoder/joiner/tokens）
   - `whisper-cli`（`whisper-cpp`；使用 `WHISPER_CPP_MODEL` 或内置 tiny 模型）
   - `whisper`（Python CLI；自动下载模型）
2. **Gemini CLI**（`gemini`）使用 `read_many_files`
3. **Provider 密钥**
   - 音频：OpenAI → Groq → Deepgram → Google
   - 图像：OpenAI → Anthropic → Google → MiniMax
   - 视频：Google

禁用自动检测：

```json5
{
  tools: {
    media: {
      audio: {
        enabled: false,
      },
    },
  },
}
```

说明：CLI 二进制检测在 macOS/Linux/Windows 上为尽力而为；确保 CLI 在 `PATH` 上（支持 `~` 展开），或设置带完整路径的显式 CLI 模型。

## 能力（可选）

设置 `capabilities` 时，该条目仅对指定的媒体类型运行。对共享列表，Go Gateway 可推断默认值：

- `openai`、`anthropic`、`minimax`：**image**
- `google`（Gemini API）：**image + audio + video**
- `groq`：**audio**
- `deepgram`：**audio**

对 CLI 条目，请**显式设置 `capabilities`** 以避免意外匹配。
省略 `capabilities` 时，该条目适用于其所在列表。

## Provider 支持矩阵

| 能力 | Provider 集成 | 说明 |
| ---- | ------------ | ---- |
| 图像 | OpenAI / Anthropic / Google / 其他（通过 `pi-ai`） | 注册表中任何支持图像的模型均可使用。 |
| 音频 | OpenAI、Groq、Deepgram、Google | Provider 转写（Whisper/Deepgram/Gemini）。 |
| 视频 | Google（Gemini API） | Provider 视频理解。 |

## 推荐 Provider

**图像**

- 优先使用支持图像的活动模型。
- 推荐默认：`openai/gpt-5.2`、`anthropic/claude-opus-4-6`、`google/gemini-3-pro-preview`。

**音频**

- `openai/gpt-4o-mini-transcribe`、`groq/whisper-large-v3-turbo` 或 `deepgram/nova-3`。
- CLI 回退：`whisper-cli`（whisper-cpp）或 `whisper`。
- Deepgram 设置：[Deepgram（音频转写）](/providers/deepgram)。

**视频**

- `google/gemini-3-flash-preview`（快速）、`google/gemini-3-pro-preview`（更丰富）。
- CLI 回退：`gemini` CLI（支持对视频/音频 `read_file`）。

## 附件策略

按能力的 `attachments` 控制处理哪些附件：

- `mode`：`first`（默认）或 `all`
- `maxAttachments`：处理数量上限（默认 **1**）
- `prefer`：`first`、`last`、`path`、`url`

当 `mode: "all"` 时，输出标记为 `[Image 1/2]`、`[Audio 2/2]` 等。

## 配置示例

### 1）共享模型列表 + 覆盖

```json5
{
  tools: {
    media: {
      models: [
        { provider: "openai", model: "gpt-5.2", capabilities: ["image"] },
        {
          provider: "google",
          model: "gemini-3-flash-preview",
          capabilities: ["image", "audio", "video"],
        },
        {
          type: "cli",
          command: "gemini",
          args: [
            "-m",
            "gemini-3-flash",
            "--allowed-tools",
            "read_file",
            "Read the media at {{MediaPath}} and describe it in <= {{MaxChars}} characters.",
          ],
          capabilities: ["image", "video"],
        },
      ],
      audio: {
        attachments: { mode: "all", maxAttachments: 2 },
      },
      video: {
        maxChars: 500,
      },
    },
  },
}
```

### 2）仅音频 + 视频（图像关闭）

```json5
{
  tools: {
    media: {
      audio: {
        enabled: true,
        models: [
          { provider: "openai", model: "gpt-4o-mini-transcribe" },
          {
            type: "cli",
            command: "whisper",
            args: ["--model", "base", "{{MediaPath}}"],
          },
        ],
      },
      video: {
        enabled: true,
        maxChars: 500,
        models: [
          { provider: "google", model: "gemini-3-flash-preview" },
          {
            type: "cli",
            command: "gemini",
            args: [
              "-m",
              "gemini-3-flash",
              "--allowed-tools",
              "read_file",
              "Read the media at {{MediaPath}} and describe it in <= {{MaxChars}} characters.",
            ],
          },
        ],
      },
    },
  },
}
```

### 3）可选图像理解

```json5
{
  tools: {
    media: {
      image: {
        enabled: true,
        maxBytes: 10485760,
        maxChars: 500,
        models: [
          { provider: "openai", model: "gpt-5.2" },
          { provider: "anthropic", model: "claude-opus-4-6" },
          {
            type: "cli",
            command: "gemini",
            args: [
              "-m",
              "gemini-3-flash",
              "--allowed-tools",
              "read_file",
              "Read the media at {{MediaPath}} and describe it in <= {{MaxChars}} characters.",
            ],
          },
        ],
      },
    },
  },
}
```

### 4）多模态单条目（显式 capabilities）

```json5
{
  tools: {
    media: {
      image: {
        models: [
          {
            provider: "google",
            model: "gemini-3-pro-preview",
            capabilities: ["image", "video", "audio"],
          },
        ],
      },
      audio: {
        models: [
          {
            provider: "google",
            model: "gemini-3-pro-preview",
            capabilities: ["image", "video", "audio"],
          },
        ],
      },
      video: {
        models: [
          {
            provider: "google",
            model: "gemini-3-pro-preview",
            capabilities: ["image", "video", "audio"],
          },
        ],
      },
    },
  },
}
```

## 状态输出

媒体理解运行时，`/status` 包含简短摘要行：

```
📎 Media: image ok (openai/gpt-5.2) · audio skipped (maxBytes)
```

显示每种能力的结果及选定的 provider/模型。

## 说明

- 理解是**尽力而为**的。错误不会阻塞回复。
- 即使理解功能禁用，附件仍传递给模型。
- 使用 `scope` 限制理解运行的范围（如仅私聊）。

## 相关文档

- [配置](/gateway/configuration)
- [图像与媒体支持](/nodes/images)
