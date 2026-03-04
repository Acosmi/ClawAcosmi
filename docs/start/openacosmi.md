---
summary: "端到端个人助手设置指南，含安全注意事项"
read_when:
  - 引导新的助手实例
  - 审查安全/权限影响
title: "个人助手设置"
status: active
arch: rust-cli+go-gateway
---

# 使用 OpenAcosmi 构建个人助手

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - **Go Gateway**（`acosmi`）管理所有通道连接和 Agent 执行
> - **Rust CLI**（`openacosmi`）提供用户交互命令

OpenAcosmi 是一个支持 WhatsApp + Telegram + Discord + 飞书 + 钉钉 + 企业微信 + iMessage 的 AI Agent 网关。插件支持 Mattermost、Signal 等。本指南是"个人助手"设置：一个专用号码，行为类似始终在线的 Agent。

## ⚠️ 安全第一

你正在将 Agent 置于以下位置：

- 在你的机器上运行命令（取决于工具配置）
- 读写工作区中的文件
- 通过 WhatsApp/Telegram/Discord/飞书/钉钉等发送消息

保守起步：

- 始终设置 `channels.whatsapp.allowFrom`（不要在个人 Mac 上对所有人开放）。
- 为助手使用专用号码。
- 心跳默认每 30 分钟一次。在信任设置之前通过 `agents.defaults.heartbeat.every: "0m"` 禁用。

## 前置条件

- OpenAcosmi 已安装和引导 — 如果还没有，请参见 [快速开始](/start/getting-started)
- 一个用于助手的第二个电话号码（SIM/eSIM/预付费）

## 双手机设置（推荐）

你需要这样的架构：

```
你的手机（个人）                第二部手机（助手）
┌─────────────────┐           ┌─────────────────┐
│  你的 WhatsApp   │  ──────▶  │  助手 WA        │
│  +1-555-你       │  消息     │  +1-555-助手     │
└─────────────────┘           └────────┬────────┘
                                       │ 通过 QR 关联
                                       ▼
                              ┌─────────────────┐
                              │  你的 Mac        │
                              │  Go Gateway     │
                              │    AI Agent     │
                              └─────────────────┘
```

如果将个人 WhatsApp 连接到 OpenAcosmi，发给你的每条消息都会变成"Agent 输入"。这通常不是你想要的。

## 5 分钟快速开始

1. 配对 WhatsApp Web（显示 QR；用助手手机扫描）：

```bash
openacosmi channels login
```

1. 启动 Gateway（保持运行）：

```bash
openacosmi gateway --port 18789
```

1. 在 `~/.openacosmi/openacosmi.json` 放入最小配置：

```json5
{
  channels: { whatsapp: { allowFrom: ["+15555550123"] } },
}
```

现在从允许列表中的手机给助手号码发消息。

引导完成时，系统会自动打开 dashboard，打印干净的（未 token 化的）链接。如果提示认证，将 `gateway.auth.token` 中的 token 粘贴到 Control UI 设置中。稍后重新打开：`openacosmi dashboard`。

## 为 Agent 配置工作区 (AGENTS)

OpenAcosmi 从工作区目录读取操作指令和"记忆"。

默认使用 `~/.openacosmi/workspace` 作为 Agent 工作区，并在设置/首次 Agent 运行时自动创建它（附带起始文件 `AGENTS.md`、`SOUL.md`、`TOOLS.md`、`IDENTITY.md`、`USER.md`、`HEARTBEAT.md`）。`BOOTSTRAP.md` 仅在工作区全新时创建（删除后不会再出现）。`MEMORY.md` 可选（不自动创建）；存在时加载用于普通会话。子 Agent 会话仅注入 `AGENTS.md` 和 `TOOLS.md`。

建议：将此文件夹视为 OpenAcosmi 的"记忆"，并使其成为 git 仓库（最好是私有的），这样你的 `AGENTS.md` + 记忆文件会被备份。如果安装了 git，全新工作区会自动初始化。

```bash
openacosmi setup
```

完整工作区布局 + 备份指南：[Agent 工作区](/concepts/agent-workspace)
记忆工作流：[记忆](/concepts/memory)

可选：使用 `agents.defaults.workspace` 选择不同的工作区（支持 `~`）。

```json5
{
  agent: {
    workspace: "~/.openacosmi/workspace",
  },
}
```

如果你已经从仓库提供自己的工作区文件，可以完全禁用引导文件创建：

```json5
{
  agent: {
    skipBootstrap: true,
  },
}
```

## 助手化配置

OpenAcosmi 默认提供良好的助手设置，但通常你会想调整：

- `SOUL.md` 中的人格/指令
- thinking 默认值（如需要）
- 心跳（信任后启用）

示例：

```json5
{
  logging: { level: "info" },
  agent: {
    model: "anthropic/claude-opus-4-6",
    workspace: "~/.openacosmi/workspace",
    thinkingDefault: "high",
    timeoutSeconds: 1800,
    // 先设为 0；之后启用。
    heartbeat: { every: "0m" },
  },
  channels: {
    whatsapp: {
      allowFrom: ["+15555550123"],
      groups: {
        "*": { requireMention: true },
      },
    },
  },
  routing: {
    groupChat: {
      mentionPatterns: ["@openacosmi", "openacosmi"],
    },
  },
  session: {
    scope: "per-sender",
    resetTriggers: ["/new", "/reset"],
    reset: {
      mode: "daily",
      atHour: 4,
      idleMinutes: 10080,
    },
  },
}
```

## 会话和记忆

- 会话文件：`~/.openacosmi/agents/<agentId>/sessions/{{SessionId}}.jsonl`
- 会话元数据（token 用量、最后路由等）：`~/.openacosmi/agents/<agentId>/sessions/sessions.json`
- `/new` 或 `/reset` 为该聊天启动新会话（可通过 `resetTriggers` 配置）。单独发送时，Agent 会回复简短问候确认重置。
- `/compact [instructions]` 压缩会话上下文并报告剩余的上下文预算。

## 心跳（主动模式）

默认情况下，OpenAcosmi 每 30 分钟运行一次心跳，提示：
`Read HEARTBEAT.md if it exists (workspace context). Follow it strictly. Do not infer or repeat old tasks from prior chats. If nothing needs attention, reply HEARTBEAT_OK.`
设置 `agents.defaults.heartbeat.every: "0m"` 可禁用。

- 如果 `HEARTBEAT.md` 存在但实际为空（仅空行和 markdown 标题如 `# Heading`），OpenAcosmi 跳过心跳以节省 API 调用。
- 如果文件缺失，心跳仍会运行，由模型决定做什么。
- 如果 Agent 回复 `HEARTBEAT_OK`（可选带短填充；见 `agents.defaults.heartbeat.ackMaxChars`），OpenAcosmi 抑制该次心跳的出站投递。
- 心跳运行完整的 Agent turn — 更短的间隔消耗更多 token。

```json5
{
  agent: {
    heartbeat: { every: "30m" },
  },
}
```

## 媒体输入和输出

入站附件（图片/音频/文档）可通过模板暴露给命令：

- `{{MediaPath}}`（本地临时文件路径）
- `{{MediaUrl}}`（伪 URL）
- `{{Transcript}}`（如果启用了音频转录）

Agent 的出站附件：在独立行中包含 `MEDIA:<path-or-url>`（无空格）。示例：

```
这是截图。
MEDIA:https://example.com/screenshot.png
```

OpenAcosmi 提取这些内容并将其作为媒体附件随文本一起发送。

## 运维检查清单

```bash
openacosmi status          # 本地状态（凭证、会话、排队事件）
openacosmi status --all    # 完整诊断（只读，可粘贴）
openacosmi status --deep   # 添加 Gateway 健康探测（Telegram + Discord）
openacosmi health --json   # Gateway 健康快照（WebSocket）
```

日志存放在 `/tmp/openacosmi/`（默认：`openacosmi-YYYY-MM-DD.log`）。

## 下一步

- WebChat：[WebChat](/web/webchat)
- Gateway 运维：[Gateway 运维手册](/gateway)
- 定时任务 + 唤醒：[定时任务](/automation/cron-jobs)
- macOS 菜单栏伴侣：[macOS 应用](/platforms/macos)
- iOS 应用：[iOS 应用](/platforms/ios)
- Android 应用：[Android 应用](/platforms/android)
- Windows 状态：[Windows (WSL2)](/platforms/windows)
- Linux 状态：[Linux 应用](/platforms/linux)
- 安全：[安全](/gateway/security)
