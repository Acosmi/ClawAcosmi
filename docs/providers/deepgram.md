---
summary: "Deepgram 语音转写（用于入站语音消息）"
read_when:
  - 使用 Deepgram 语音转文字处理音频附件
  - 需要 Deepgram 配置示例
title: "Deepgram"
status: active
arch: rust-cli+go-gateway
---

# Deepgram（音频转写）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Deepgram 集成由 **Go Gateway** 处理，通过 `tools.media.audio` 配置
> - API Key 环境变量 `DEEPGRAM_API_KEY` 由 Go Gateway 解析（`backend/internal/agents/models/providers.go`）

Deepgram 是一个语音转文字 API。在 OpenAcosmi 中，它通过 `tools.media.audio` 用于**入站音频/语音消息转写**。

启用后，OpenAcosmi 将音频文件上传到 Deepgram，并将转录内容注入回复管道（`{{Transcript}}` + `[Audio]` 块）。这**不是流式传输**——使用的是预录音转写端点。

网站：[https://deepgram.com](https://deepgram.com)
文档：[https://developers.deepgram.com](https://developers.deepgram.com)

## 快速开始

1. 设置 API Key：

```
DEEPGRAM_API_KEY=dg_...
```

1. 启用供应商：

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

## 选项

- `model`：Deepgram 模型 ID（默认：`nova-3`）
- `language`：语言提示（可选）
- `tools.media.audio.providerOptions.deepgram.detect_language`：启用语言检测（可选）
- `tools.media.audio.providerOptions.deepgram.punctuate`：启用标点（可选）
- `tools.media.audio.providerOptions.deepgram.smart_format`：启用智能格式化（可选）

带语言的示例：

```json5
{
  tools: {
    media: {
      audio: {
        enabled: true,
        models: [{ provider: "deepgram", model: "nova-3", language: "zh" }],
      },
    },
  },
}
```

带 Deepgram 选项的示例：

```json5
{
  tools: {
    media: {
      audio: {
        enabled: true,
        providerOptions: {
          deepgram: {
            detect_language: true,
            punctuate: true,
            smart_format: true,
          },
        },
        models: [{ provider: "deepgram", model: "nova-3" }],
      },
    },
  },
}
```

## 注意事项

- 认证遵循标准供应商认证顺序；`DEEPGRAM_API_KEY` 是最简单的方式。
- 使用代理时，可通过 `tools.media.audio.baseUrl` 和 `tools.media.audio.headers` 覆盖端点或 headers。
- 输出遵循与其他供应商相同的音频规则（大小限制、超时、转录注入）。
