---
summary: "Talk 模式：使用 ElevenLabs TTS 的持续语音对话"
read_when:
  - 在 macOS/iOS/Android 上实现 Talk 模式
  - 修改语音/TTS/中断行为
title: "Talk 模式"
---

> **架构提示 — Rust CLI + Go Gateway**
> Talk 模式的 TTS 处理由 Go Gateway 实现（`backend/internal/tts/`），
> 语音对话通过 Gateway WebSocket 的 `chat.send` 进行。

# Talk 模式

Talk 模式是持续语音对话循环：

1. 监听语音
2. 将转写文本发送给模型（主会话，`chat.send`）
3. 等待回复
4. 通过 ElevenLabs 播放回复（流式播放）

## 行为（macOS）

- Talk 模式启用时显示**常驻浮窗**。
- **监听 → 思考 → 朗读** 阶段转换。
- **短暂停顿**（静默窗口）时，当前转写文本被发送。
- 回复被**写入 WebChat**（与打字相同）。
- **语音中断**（默认开启）：如果用户在助手朗读时开始说话，停止播放并记录中断时间戳用于下一次提示。

## 回复中的语音指令

助手可以在回复前缀一个 **JSON 行**来控制语音：

```json
{ "voice": "<voice-id>", "once": true }
```

规则：

- 仅限第一个非空行。
- 未知键被忽略。
- `once: true` 仅对当前回复生效。
- 不带 `once` 时，该语音成为 Talk 模式的新默认值。
- JSON 行在 TTS 播放前被移除。

支持的键：

- `voice` / `voice_id` / `voiceId`
- `model` / `model_id` / `modelId`
- `speed`、`rate`（WPM）、`stability`、`similarity`、`style`、`speakerBoost`
- `seed`、`normalize`、`lang`、`output_format`、`latency_tier`
- `once`

## 配置（`~/.openacosmi/openacosmi.json`）

```json5
{
  talk: {
    voiceId: "elevenlabs_voice_id",
    modelId: "eleven_v3",
    outputFormat: "mp3_44100_128",
    apiKey: "elevenlabs_api_key",
    interruptOnSpeech: true,
  },
}
```

默认值：

- `interruptOnSpeech`：true
- `voiceId`：回退到 `ELEVENLABS_VOICE_ID` / `SAG_VOICE_ID`（或在有 API key 时使用第一个 ElevenLabs 语音）
- `modelId`：未设置时默认为 `eleven_v3`
- `apiKey`：回退到 `ELEVENLABS_API_KEY`（或 Gateway shell profile，如可用）
- `outputFormat`：macOS/iOS 默认 `pcm_44100`，Android 默认 `pcm_24000`（设置 `mp3_*` 强制 MP3 流式传输）

## macOS UI

- 菜单栏切换：**Talk**
- 配置选项卡：**Talk 模式**组（voice ID + 中断开关）
- 浮窗：
  - **监听**：云朵随麦克风音量脉动
  - **思考**：下沉动画
  - **朗读**：辐射光环
  - 点击云朵：停止朗读
  - 点击 X：退出 Talk 模式

## 说明

- 需要语音识别 + 麦克风权限。
- 使用 `chat.send` 对会话键 `main` 操作（通过 Go Gateway 处理）。
- TTS 使用 ElevenLabs 流式 API，在 macOS/iOS/Android 上进行增量播放以降低延迟。
- `eleven_v3` 的 `stability` 验证为 `0.0`、`0.5` 或 `1.0`；其他模型接受 `0..1`。
- `latency_tier` 设置时验证为 `0..4`。
- Android 支持 `pcm_16000`、`pcm_22050`、`pcm_24000` 和 `pcm_44100` 输出格式，用于低延迟 AudioTrack 流式播放。

Go TTS 实现：`backend/internal/tts/`（TTS provider 管理和流式输出）。
