---
summary: "入站音频/语音笔记的下载、转写和注入回复流程"
read_when:
  - 修改音频转写或媒体处理逻辑
title: "音频与语音笔记"
---

> **架构提示 — Rust CLI + Go Gateway**
> 音频/语音处理逻辑由 Go Gateway 实现（`backend/internal/media/`），
> 包括 STT 转写（`stt.go`、`stt_openai.go`、`stt_local.go`）和音频标签解析（`audio_tags.go`）。

# 音频 / 语音笔记

## 功能说明

- **媒体理解（音频）**：启用音频理解后（或自动检测），OpenAcosmi Go Gateway：
  1. 定位第一个音频附件（本地路径或 URL）并按需下载。
  2. 发送到各模型条目前执行 `maxBytes` 限制检查。
  3. 按顺序运行第一个符合条件的模型条目（provider 或 CLI）。
  4. 如果失败或跳过（大小/超时），尝试下一个条目。
  5. 成功时，将 `Body` 替换为 `[Audio]` 块并设置 `{{Transcript}}`。
- **命令解析**：转写成功后，`CommandBody`/`RawBody` 设置为转写文本，使斜杠命令仍能正常工作。
- **详细日志**：在 `--verbose` 模式下，记录转写运行和替换 body 的日志。

## 自动检测（默认）

如果**未配置模型**且 `tools.media.audio.enabled` 未设置为 `false`，
Go Gateway 按以下顺序自动检测并在第一个可用选项处停止：

1. **本地 CLI 工具**（如已安装）
   - `sherpa-onnx-offline`（需要 `SHERPA_ONNX_MODEL_DIR` 包含 encoder/decoder/joiner/tokens）
   - `whisper-cli`（来自 `whisper-cpp`；使用 `WHISPER_CPP_MODEL` 或内置 tiny 模型）
   - `whisper`（Python CLI；自动下载模型）
2. **Gemini CLI**（`gemini`）使用 `read_many_files`
3. **Provider API 密钥**（OpenAI → Groq → Deepgram → Google）

Go 实现参见 `backend/internal/media/stt.go`（`NewSTTFromConfig` 函数）。

禁用自动检测：设置 `tools.media.audio.enabled: false`。
自定义配置：设置 `tools.media.audio.models`。

说明：CLI 二进制检测在 macOS/Linux/Windows 上为尽力而为；确保 CLI 在 `PATH` 上（支持 `~` 展开），或设置带完整路径的显式 CLI 模型。

## 配置示例

### Provider + CLI 回退（OpenAI + Whisper CLI）

```json5
{
  tools: {
    media: {
      audio: {
        enabled: true,
        maxBytes: 20971520,
        models: [
          { provider: "openai", model: "gpt-4o-mini-transcribe" },
          {
            type: "cli",
            command: "whisper",
            args: ["--model", "base", "{{MediaPath}}"],
            timeoutSeconds: 45,
          },
        ],
      },
    },
  },
}
```

### 仅 Provider 并启用范围控制

```json5
{
  tools: {
    media: {
      audio: {
        enabled: true,
        scope: {
          default: "allow",
          rules: [{ action: "deny", match: { chatType: "group" } }],
        },
        models: [{ provider: "openai", model: "gpt-4o-mini-transcribe" }],
      },
    },
  },
}
```

### 仅 Provider（Deepgram）

```json5
{
  tools: {
    media: {
      audio: {
        enabled: true,
        models: [{ provider: "deepgram", model: "nova-3" }],
      },
    },
  },
}
```

## 说明与限制

- Provider 认证遵循标准模型认证顺序（auth profiles、环境变量、`models.providers.*.apiKey`）。
- Deepgram 在使用 `provider: "deepgram"` 时读取 `DEEPGRAM_API_KEY`。
- Deepgram 设置详情：[Deepgram（音频转写）](/providers/deepgram)。
- 音频 provider 可通过 `tools.media.audio` 覆盖 `baseUrl`、`headers` 和 `providerOptions`。
- 默认大小上限为 20MB（`tools.media.audio.maxBytes`）。超大音频在该模型中被跳过，尝试下一个条目。
- 音频默认 `maxChars` **未设置**（完整转写）。设置 `tools.media.audio.maxChars` 或每条目 `maxChars` 以截断输出。
- OpenAI 自动默认模型为 `gpt-4o-mini-transcribe`；设置 `model: "gpt-4o-transcribe"` 获得更高精度。
- 使用 `tools.media.audio.attachments` 处理多个语音笔记（`mode: "all"` + `maxAttachments`）。
- 转写文本可通过模板变量 `{{Transcript}}` 使用。
- CLI stdout 上限 5MB；保持 CLI 输出简洁。

## Go 实现细节

- STT 接口定义：`backend/internal/media/stt.go`（`STTProvider` interface）
- OpenAI STT：`backend/internal/media/stt_openai.go`（`OpenAISTT` 结构体，POST `/v1/audio/transcriptions`）
- 本地 Whisper STT：`backend/internal/media/stt_local.go`（`LocalWhisperSTT`）
- 音频标签解析：`backend/internal/media/audio_tags.go`（`ParseAudioTag` 函数）
- 音频工具类型：`backend/internal/media/audio.go`

## 注意事项

- Scope 规则使用先匹配优先。`chatType` 标准化为 `direct`、`group` 或 `room`。
- 确保 CLI 退出码为 0 并输出纯文本；JSON 输出需通过 `jq -r .text` 转换。
- 保持合理的超时时间（`timeoutSeconds`，默认 60s）以避免阻塞回复队列。
