---
summary: "语音通话插件：通过 Twilio/Telnyx/Plivo 的出站+入站通话（安装 + 配置 + CLI + 工具）"
read_when:
  - 需要从 OpenAcosmi 发起出站语音通话
  - 配置或开发语音通话插件
title: "语音通话插件"
---

> **架构提示 — Rust CLI + Go Gateway**
> 语音通话插件在 Go Gateway 进程内运行，
> CLI 命令由 Rust 二进制处理（`cli-rust/crates/oa-cmd-voicecall/`），
> TTS 处理参见 `backend/internal/tts/`。

# 语音通话（插件）

OpenAcosmi 的语音通话功能通过插件实现。支持出站通知和多轮对话（含入站策略）。

当前 provider：

- `twilio`（Programmable Voice + Media Streams）
- `telnyx`（Call Control v2）
- `plivo`（Voice API + XML transfer + GetInput speech）
- `mock`（开发/无网络）

快速思维模型：

- 安装插件
- 重启 Go Gateway
- 在 `plugins.entries.voice-call.config` 中配置
- 使用 `openacosmi voicecall ...` 或 `voice_call` 工具

## 运行位置（本地与远程）

语音通话插件在 **Go Gateway 进程内**运行。

如果使用远程 Gateway，在**运行 Gateway 的机器上**安装/配置插件，然后重启 Gateway 以加载。

## 安装

### 方式 A：从 npm 安装（推荐）

```bash
openacosmi plugins install @openacosmi/voice-call
```

安装后重启 Go Gateway。

### 方式 B：从本地文件夹安装（开发用，不复制文件）

```bash
openacosmi plugins install ./extensions/voice-call
cd ./extensions/voice-call && pnpm install
```

安装后重启 Go Gateway。

## 配置

在 `plugins.entries.voice-call.config` 下设置配置：

```json5
{
  plugins: {
    entries: {
      "voice-call": {
        enabled: true,
        config: {
          provider: "twilio", // 或 "telnyx" | "plivo" | "mock"
          fromNumber: "+15550001234",
          toNumber: "+15550005678",

          twilio: {
            accountSid: "ACxxxxxxxx",
            authToken: "...",
          },

          plivo: {
            authId: "MAxxxxxxxxxxxxxxxxxxxx",
            authToken: "...",
          },

          // Webhook 服务器
          serve: {
            port: 3334,
            path: "/voice/webhook",
          },

          // Webhook 安全（推荐用于隧道/代理）
          webhookSecurity: {
            allowedHosts: ["voice.example.com"],
            trustedProxyIPs: ["100.64.0.1"],
          },

          // 公网暴露（选择其一）
          // publicUrl: "https://example.ngrok.app/voice/webhook",
          // tunnel: { provider: "ngrok" },
          // tailscale: { mode: "funnel", path: "/voice/webhook" }

          outbound: {
            defaultMode: "notify", // notify | conversation
          },

          streaming: {
            enabled: true,
            streamPath: "/voice/stream",
          },
        },
      },
    },
  },
}
```

说明：

- Twilio/Telnyx 需要**公网可达**的 webhook URL。
- Plivo 需要**公网可达**的 webhook URL。
- `mock` 是本地开发 provider（无网络调用）。
- `skipSignatureVerification` 仅用于本地测试。
- 使用 ngrok 免费层时，将 `publicUrl` 设置为确切的 ngrok URL；签名验证始终强制执行。
- `tunnel.allowNgrokFreeTierLoopbackBypass: true` 仅在 `tunnel.provider="ngrok"` 且 `serve.bind` 为回环地址（ngrok 本地代理）时允许无效签名的 Twilio webhook。仅限本地开发使用。
- ngrok 免费层 URL 可能变化或添加中间页面；如果 `publicUrl` 偏移，Twilio 签名将失败。生产环境建议使用稳定域名或 Tailscale funnel。

## Webhook 安全

当代理或隧道位于 Gateway 前端时，插件重建公网 URL 用于签名验证。以下选项控制信任哪些转发头。

`webhookSecurity.allowedHosts` 允许列出转发头中的主机。

`webhookSecurity.trustForwardingHeaders` 无需允许列表即信任转发头。

`webhookSecurity.trustedProxyIPs` 仅在请求远程 IP 匹配列表时信任转发头。

稳定公网主机示例：

```json5
{
  plugins: {
    entries: {
      "voice-call": {
        config: {
          publicUrl: "https://voice.example.com/voice/webhook",
          webhookSecurity: {
            allowedHosts: ["voice.example.com"],
          },
        },
      },
    },
  },
}
```

## 通话 TTS

语音通话使用核心 `messages.tts` 配置（OpenAI 或 ElevenLabs）进行通话中的流式语音。你可以在插件配置中使用**相同结构**覆盖——它会与 `messages.tts` 深度合并。

```json5
{
  tts: {
    provider: "elevenlabs",
    elevenlabs: {
      voiceId: "pMsXgVXv3BLzUgSXRplE",
      modelId: "eleven_multilingual_v2",
    },
  },
}
```

Go TTS 实现：`backend/internal/tts/`（TTS provider 管理和流式输出）。

说明：

- **语音通话忽略 Edge TTS**（电话音频需要 PCM；Edge 输出不可靠）。
- 启用 Twilio 媒体流时使用核心 TTS；否则通话回退到 provider 原生语音。

### 更多示例

仅使用核心 TTS（不覆盖）：

```json5
{
  messages: {
    tts: {
      provider: "openai",
      openai: { voice: "alloy" },
    },
  },
}
```

仅对通话覆盖为 ElevenLabs（其他地方保持核心默认）：

```json5
{
  plugins: {
    entries: {
      "voice-call": {
        config: {
          tts: {
            provider: "elevenlabs",
            elevenlabs: {
              apiKey: "elevenlabs_key",
              voiceId: "pMsXgVXv3BLzUgSXRplE",
              modelId: "eleven_multilingual_v2",
            },
          },
        },
      },
    },
  },
}
```

仅对通话覆盖 OpenAI 模型（深度合并示例）：

```json5
{
  plugins: {
    entries: {
      "voice-call": {
        config: {
          tts: {
            openai: {
              model: "gpt-4o-mini-tts",
              voice: "marin",
            },
          },
        },
      },
    },
  },
}
```

## 入站通话

入站策略默认为 `disabled`。要启用入站通话，设置：

```json5
{
  inboundPolicy: "allowlist",
  allowFrom: ["+15550001234"],
  inboundGreeting: "你好！有什么可以帮助您的？",
}
```

自动响应使用 agent 系统。可通过以下参数调优：

- `responseModel`
- `responseSystemPrompt`
- `responseTimeoutMs`

## CLI

```bash
openacosmi voicecall call --to "+15555550123" --message "Hello from OpenAcosmi"
openacosmi voicecall continue --call-id <id> --message "还有什么问题吗？"
openacosmi voicecall speak --call-id <id> --message "请稍等"
openacosmi voicecall end --call-id <id>
openacosmi voicecall status --call-id <id>
openacosmi voicecall tail
openacosmi voicecall expose --mode funnel
```

Rust CLI 实现：`cli-rust/crates/oa-cmd-voicecall/`。

## Agent 工具

工具名称：`voice_call`

操作：

- `initiate_call`（message、to?、mode?）
- `continue_call`（callId、message）
- `speak_to_user`（callId、message）
- `end_call`（callId）
- `get_status`（callId）

本仓库附带匹配的技能文档：`skills/voice-call/SKILL.md`。

## Gateway RPC

- `voicecall.initiate`（`to?`、`message`、`mode?`）
- `voicecall.continue`（`callId`、`message`）
- `voicecall.speak`（`callId`、`message`）
- `voicecall.end`（`callId`）
- `voicecall.status`（`callId`）
