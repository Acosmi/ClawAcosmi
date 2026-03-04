---
summary: "链接到所有 OpenAcosmi 文档的中心页面"
read_when:
  - 想要文档的完整导航图
title: "文档中心"
status: active
arch: rust-cli+go-gateway
---

# 文档中心

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。

<Note>
如果你是 OpenAcosmi 新手，请从 [快速开始](/start/getting-started) 开始。
</Note>

使用这些中心页面发现每个文档页面，包括深入解析和参考文档。

## 从这里开始

- [首页](/)
- [快速开始](/start/getting-started)
- [引导](/start/onboarding)
- [向导](/start/wizard)
- [高级设置](/start/setup)
- [Dashboard（本地 Gateway）](http://127.0.0.1:18789/)
- [帮助](/help)
- [文档目录](/start/docs-directory)
- [配置](/gateway/configuration)
- [配置示例](/gateway/configuration-examples)
- [OpenAcosmi 助手](/start/openacosmi)
- [展示](/start/showcase)
- [历史](/start/lore)

## 安装 + 更新

- [Docker](/install/docker)
- [Nix](/install/nix)
- [更新 / 回滚](/install/updating)

## 核心概念

- [架构](/concepts/architecture)
- [功能特性](/concepts/features)
- [网络中心](/network)
- [Agent 运行时](/concepts/agent)
- [Agent 工作区](/concepts/agent-workspace)
- [记忆](/concepts/memory)
- [Agent 循环](/concepts/agent-loop)
- [流式传输 + 分块](/concepts/streaming)
- [多 Agent 路由](/concepts/multi-agent)
- [上下文压缩](/concepts/compaction)
- [会话](/concepts/session)
- [会话（别名）](/concepts/sessions)
- [会话修剪](/concepts/session-pruning)
- [会话工具](/concepts/session-tool)
- [队列](/concepts/queue)
- [斜杠命令](/tools/slash-commands)
- [RPC 适配器](/reference/rpc)
- [时区处理](/concepts/timezone)
- [在线状态](/concepts/presence)
- [发现 + 传输](/gateway/discovery)
- [Bonjour](/gateway/bonjour)
- [通道路由](/channels/channel-routing)
- [群组](/channels/groups)
- [群组消息](/channels/group-messages)
- [模型故障转移](/concepts/model-failover)
- [OAuth](/concepts/oauth)

## Provider + 入口

- [通道中心](/channels)
- [模型 Provider 中心](/providers/models)
- [WhatsApp](/channels/whatsapp)
- [Telegram](/channels/telegram)
- [Slack](/channels/slack)
- [Discord](/channels/discord)
- [Mattermost](/channels/mattermost)（插件）
- [Signal](/channels/signal)
- [BlueBubbles (iMessage)](/channels/bluebubbles)
- [iMessage（旧版）](/channels/imessage)
- [飞书](/channels/feishu)
- [钉钉](/channels/dingtalk)
- [企业微信](/channels/wecom)
- [LINE](/channels/line)
- [微信公众号](/channels/wechat-mp)
- [小红书](/channels/xiaohongshu)
- [位置解析](/channels/location)
- [WebChat](/web/webchat)
- [Webhooks](/automation/webhook)
- [Gmail Pub/Sub](/automation/gmail-pubsub)

## Gateway + 运维

- [Gateway 运维手册](/gateway)
- [网络模型](/gateway/network-model)
- [Gateway 配对](/gateway/pairing)
- [Gateway 锁](/gateway/gateway-lock)
- [后台进程](/gateway/background-process)
- [健康检查](/gateway/health)
- [心跳](/gateway/heartbeat)
- [Doctor](/gateway/doctor)
- [日志](/gateway/logging)
- [沙箱](/gateway/sandboxing)
- [Dashboard](/web/dashboard)
- [Control UI](/web/control-ui)
- [远程访问](/gateway/remote)
- [远程 Gateway README](/gateway/remote-gateway-readme)
- [Tailscale](/gateway/tailscale)
- [安全](/gateway/security)
- [故障排除](/gateway/troubleshooting)

## 工具 + 自动化

- [工具面板](/tools)
- [CLI 参考](/cli)
- [Exec 工具](/tools/exec)
- [提升模式](/tools/elevated)
- [定时任务](/automation/cron-jobs)
- [定时 vs 心跳](/automation/cron-vs-heartbeat)
- [Thinking + verbose](/tools/thinking)
- [模型](/concepts/models)
- [子 Agent](/tools/subagents)
- [Agent send CLI](/tools/agent-send)
- [终端 UI](/web/tui)
- [浏览器控制](/tools/browser)
- [浏览器（Linux 排错）](/tools/browser-linux-troubleshooting)
- [投票](/automation/poll)

## Node、媒体、语音

- [Node 概述](/nodes)
- [摄像头](/nodes/camera)
- [图片](/nodes/images)
- [音频](/nodes/audio)
- [位置命令](/nodes/location-command)
- [语音唤醒](/nodes/voicewake)
- [对话模式](/nodes/talk)

## 平台

- [平台概述](/platforms)
- [macOS](/platforms/macos)
- [iOS](/platforms/ios)
- [Android](/platforms/android)
- [Windows (WSL2)](/platforms/windows)
- [Linux](/platforms/linux)
- [Web 界面](/web)

## macOS 伴侣应用（高级）

- [macOS 开发设置](/platforms/mac/dev-setup)
- [macOS 菜单栏](/platforms/mac/menu-bar)
- [macOS 语音唤醒](/platforms/mac/voicewake)
- [macOS 语音覆盖层](/platforms/mac/voice-overlay)
- [macOS WebChat](/platforms/mac/webchat)
- [macOS Canvas](/platforms/mac/canvas)
- [macOS 子进程](/platforms/mac/child-process)
- [macOS 健康检查](/platforms/mac/health)
- [macOS 图标](/platforms/mac/icon)
- [macOS 日志](/platforms/mac/logging)
- [macOS 权限](/platforms/mac/permissions)
- [macOS 远程](/platforms/mac/remote)
- [macOS 签名](/platforms/mac/signing)
- [macOS 发布](/platforms/mac/release)
- [macOS Gateway (launchd)](/platforms/mac/bundled-gateway)
- [macOS XPC](/platforms/mac/xpc)
- [macOS 技能](/platforms/mac/skills)
- [macOS Peekaboo](/platforms/mac/peekaboo)

## 工作区 + 模板

- [技能](/tools/skills)
- [ClawHub](/tools/clawhub)
- [技能配置](/tools/skills-config)
- [默认 AGENTS](/reference/AGENTS.default)
- [模板: AGENTS](/reference/templates/AGENTS)
- [模板: BOOTSTRAP](/reference/templates/BOOTSTRAP)
- [模板: HEARTBEAT](/reference/templates/HEARTBEAT)
- [模板: IDENTITY](/reference/templates/IDENTITY)
- [模板: SOUL](/reference/templates/SOUL)
- [模板: TOOLS](/reference/templates/TOOLS)
- [模板: USER](/reference/templates/USER)

## 实验性（探索中）

- [引导配置协议](/experiments/onboarding-config-protocol)
- [定时强化笔记](/experiments/plans/cron-add-hardening)
- [群组策略强化笔记](/experiments/plans/group-policy-hardening)
- [研究：记忆](/experiments/research/memory)
- [模型配置探索](/experiments/proposals/model-config)

## 项目

- [致谢](/reference/credits)

## 测试 + 发布

- [测试](/reference/test)
- [发布检查表](/reference/RELEASING)
- [设备型号](/reference/device-models)
