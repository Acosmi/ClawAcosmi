---
summary: "配对概览：审批 DM 对话权限和设备加入权限"
read_when:
  - 设置 DM 访问控制
  - 配对新的 iOS/Android 节点
  - 审查 OpenAcosmi 安全态势
title: "配对（Pairing）"
---

# 配对

> [!IMPORTANT]
> **架构状态**：配对逻辑由 **Go Gateway**（`backend/internal/channels/pairing.go`、`backend/internal/gateway/channel_pairing_bridge.go`）实现。
> CLI 配对命令通过 **Rust CLI** 提供。

"配对"是 OpenAcosmi 的显式**所有者审批**步骤，用于两种场景：

1. **DM 配对**（控制谁可以与 bot 对话）
2. **设备配对**（控制哪些设备/节点可以加入 Gateway 网络）

安全上下文：[安全](/gateway/security)

## 1) DM 配对（入站聊天访问控制）

当频道配置为 DM 策略 `pairing` 时，未知发送者会收到一个短码，其消息**不会被处理**，直到你批准。

默认 DM 策略记录在：[安全](/gateway/security)

配对码：

- 8 个字符，大写字母，无歧义字符（`0O1I`）。
- **1 小时后过期**。Bot 仅在创建新请求时发送配对消息（每个发送者大约每小时一次）。
- 待处理的 DM 配对请求默认每频道上限为 **3 个**；超出的请求会被忽略，直到有请求过期或被批准。

### 批准发送者

```bash
openacosmi pairing list telegram
openacosmi pairing approve telegram <CODE>
```

支持频道：`telegram`、`whatsapp`、`signal`、`imessage`、`discord`、`slack`、`feishu`。

### 状态存储位置

存储在 `~/.openacosmi/credentials/` 下：

- 待处理请求：`<channel>-pairing.json`
- 已批准白名单：`<channel>-allowFrom.json`

请将这些文件视为敏感数据（它们控制着你的助手的访问权限）。

## 2) 设备配对（iOS/Android/macOS/无头节点）

节点以 `role: node` **设备**身份连接到 Gateway。Gateway 创建设备配对请求，需要被批准。

### 通过 Telegram 配对（推荐用于 iOS）

如果你使用 `device-pair` 插件，可以完全通过 Telegram 完成首次设备配对：

1. 在 Telegram 中，向你的 bot 发送：`/pair`
2. Bot 回复两条消息：一条指导消息和一条单独的 **设置码** 消息（方便在 Telegram 中复制粘贴）。
3. 在手机上，打开 OpenAcosmi iOS 应用 → 设置 → Gateway。
4. 粘贴设置码并连接。
5. 回到 Telegram：`/pair approve`

设置码是一个 base64 编码的 JSON 载荷，包含：

- `url`：Gateway WebSocket URL（`ws://...` 或 `wss://...`）
- `token`：短时效配对令牌

在有效期内，请像对待密码一样处理设置码。

### 批准设备

```bash
openacosmi devices list
openacosmi devices approve <requestId>
openacosmi devices reject <requestId>
```

### 设备配对状态存储

存储在 `~/.openacosmi/devices/` 下：

- `pending.json`（短时效；待处理请求会过期）
- `paired.json`（已配对设备 + 令牌）

### 备注

- 旧版 `node.pair.*` API（CLI：`openacosmi nodes pending/approve`）是独立的 Gateway 拥有的配对存储。WS 节点仍需设备配对。

## 相关文档

- 安全模型 + 提示注入：[安全](/gateway/security)
- 安全更新（运行 doctor）：[更新](/install/updating)
- 频道配置：
  - Telegram：[Telegram](/channels/telegram)
  - WhatsApp：[WhatsApp](/channels/whatsapp)
  - Signal：[Signal](/channels/signal)
  - BlueBubbles (iMessage)：[BlueBubbles](/channels/bluebubbles)
  - iMessage (旧版)：[iMessage](/channels/imessage)
  - Discord：[Discord](/channels/discord)
  - Slack：[Slack](/channels/slack)
  - 飞书：[Feishu](/channels/feishu)
