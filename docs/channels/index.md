---
summary: "OpenAcosmi 支持的消息平台一览"
read_when:
  - 你想为 OpenAcosmi 选择聊天频道
  - 你需要快速了解支持的消息平台
title: "聊天频道（Chat Channels）"
---

# 聊天频道

> [!IMPORTANT]
> **架构状态**：所有频道由 **Go Gateway**（`backend/internal/channels/`）以 Plugin 接口实现，
> CLI 命令通过 **Rust CLI**（`cli-rust/crates/oa-cmd-channels/`）提供。
> 原始 Node.js 实现已废弃。

OpenAcosmi 可通过你日常使用的任何聊天应用与你对话。每个频道通过 Go Gateway 连接。
所有频道支持文本消息；媒体和反应（reaction）支持因频道而异。

## 内置频道（Go 原生）

以下频道在 Go Gateway 中原生实现，无需额外安装：

- [WhatsApp](/channels/whatsapp) — 最流行；通过 QR 码配对连接。
- [Telegram](/channels/telegram) — Telegram Bot API；支持群组和频道。
- [Discord](/channels/discord) — Discord Bot API + Gateway；支持服务器、频道和 DM。
- [Slack](/channels/slack) — Slack 工作区应用；支持 Socket Mode 和 HTTP 模式。
- [飞书 / Feishu](/channels/feishu) — 飞书/Lark 机器人；WebSocket 长连接。
- [Google Chat](/channels/googlechat) — Google Chat API 应用；通过 HTTP Webhook。
- [Signal](/channels/signal) — 通过 signal-cli 集成；注重隐私。
- [iMessage](/channels/bluebubbles) — 通过 BlueBubbles macOS 服务端 REST API（推荐）。
- [iMessage (旧版)](/channels/imessage) — 旧版 macOS imsg CLI 集成（已废弃，新部署请用 BlueBubbles）。
- [Microsoft Teams](/channels/msteams) — Azure Bot 集成；企业级支持。
- [钉钉 / DingTalk](/channels/dingtalk) — 钉钉机器人集成。
- [企业微信 / WeCom](/channels/wecom) — 企业微信应用集成。
- [LINE](/channels/line) — LINE Messaging API 机器人。
- [WebChat](/web/webchat) — Gateway 内置 WebChat UI，通过 WebSocket 通信。

## 扩展频道（Go 插件）

以下频道通过 Go 插件机制加载：

- [Mattermost](/channels/mattermost) — 自托管团队消息平台。
- [Matrix](/channels/matrix) — Matrix 去中心化消息协议。
- [Nextcloud Talk](/channels/nextcloud-talk) — Nextcloud 自托管聊天。
- [Nostr](/channels/nostr) — 去中心化 DM（NIP-04 加密）。
- [Tlon](/channels/tlon) — Urbit 去中心化通讯。
- [Twitch](/channels/twitch) — Twitch 直播聊天（IRC）。
- [Zalo](/channels/zalo) — Zalo Bot API（越南市场）。
- [Zalo Personal](/channels/zalouser) — Zalo 个人号自动化（非官方）。
- [微信公众号 / WeChat MP](/channels/wechat_mp) — 微信公众号发布集成。
- [小红书 / Xiaohongshu](/channels/xiaohongshu) — 小红书 RPA 自动化集成。
- [网站发布 / Website](/channels/website) — 自有网站内容发布。

## Rust CLI 频道管理命令

```bash
openacosmi channels list          # 列出已配置的频道
openacosmi channels add           # 添加新频道
openacosmi channels remove        # 移除频道
openacosmi channels status        # 查看频道运行状态
openacosmi channels login         # 登录频道
openacosmi channels logout        # 登出频道
openacosmi channels capabilities  # 查看频道能力
openacosmi channels logs          # 查看频道日志
openacosmi channels resolve       # 解析频道配置
```

## 注意事项

- 频道可同时运行；配置多个频道后 OpenAcosmi 会按聊天来源自动路由。
- 最快上手频道通常是 **Telegram**（仅需 bot token）。WhatsApp 需要 QR 配对且在磁盘上存储更多状态。
- 群聊行为因频道而异；详见[群组](/channels/groups)。
- DM 配对和白名单机制确保安全性；详见[安全](/gateway/security)。
- 频道路由规则详见 [channel-routing](/channels/channel-routing)。
- 故障排查：[频道故障排查](/channels/troubleshooting)。
- 模型提供商文档参见 [Model Providers](/providers/models)。
