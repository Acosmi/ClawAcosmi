# docs/channels 目录文档审计报告

> 审计日期：2026-03-01 | 共 29 个文件

---

## 一、总览与基础设施文档（8 个）

### 1. `index.md` — 频道总览索引

**用途：** 频道功能的主入口页面，列出 OpenAcosmi 支持的全部聊天频道（WhatsApp、Telegram、Discord、Slack、飞书、Google Chat、Mattermost、Signal、BlueBubbles、iMessage、MS Teams、LINE、Nextcloud Talk、Matrix、Nostr、Tlon、Twitch、Zalo、Zalo Personal、WebChat），并标注哪些是插件形式安装。提供快速导航链接。

**关键信息：**

- 所有频道可同时运行，OpenAcosmi 会按聊天来源路由
- 最快上手频道是 **Telegram**（只需 bot token）
- WhatsApp 需要 QR 配对，状态存储较多

---

### 2. `channel-routing.md` — 频道路由规则

**用途：** 描述 OpenAcosmi 的消息路由机制。回复消息会**确定性地路由回消息来源频道**，模型不会选择频道。

**关键概念：**

- **Channel**：频道标识（whatsapp/telegram/discord 等）
- **AccountId**：每频道的账户实例
- **AgentId**：隔离工作区 + 会话存储
- **SessionKey**：会话上下文存储与并发控制的桶键
- 路由优先级：精确 peer 匹配 → Guild 匹配 → Team 匹配 → Account 匹配 → Channel 匹配 → 默认 agent
- 广播组（Broadcast Groups）可在同一消息上运行多个 agent

---

### 3. `broadcast-groups.md` — 广播组（多 Agent 并行）

**用途：** 允许多个 agent 同时处理并响应同一条消息。实验性功能，当前仅支持 **WhatsApp**。

**核心特性：**

- 每个 agent 维护完全独立的会话、历史、工作区、工具权限
- 支持并行（parallel，默认）和顺序（sequential）两种处理策略
- 适用场景：专业 agent 团队、多语言支持、质量保证流程、任务自动化
- 广播组在频道白名单和群组激活规则**之后**评估
- 优先级：`broadcast` > `bindings`

---

### 4. `group-messages.md` — WhatsApp 群消息处理

**用途：** 专门描述 WhatsApp 群消息的行为细节，包括激活模式、群策略、会话隔离、上下文注入、发送者标识等。

**关键配置：**

- 激活模式：`mention`（默认，需要 @提及）或 `always`（始终响应）
- 群策略：`open | disabled | allowlist`（默认 allowlist）
- 每群独立会话键：`agent:<agentId>:whatsapp:group:<jid>`
- 上下文注入：未处理的群消息（默认 50 条）作为上下文前缀
- 发送者标识：`[from: 发送者名称 (+E164)]`

---

### 5. `groups.md` — 跨平台群聊行为

**用途：** 统一描述所有平台的群聊行为（WhatsApp/Telegram/Discord/Slack/Signal/iMessage/MS Teams），是群聊配置的核心参考文档。

**核心内容：**

- 群策略评估顺序：`groupPolicy` → 群白名单 → 提及门控
- 三种群策略：`open`（绕过白名单）、`disabled`（完全阻止）、`allowlist`（仅匹配白名单）
- 提及门控：默认需要 @提及，回复 bot 消息算隐式提及
- 每群可配置独立工具限制（`tools`/`toolsBySender`）
- 个人 DM + 公共群组的单 agent 模式：DM 在主机运行，群在 Docker 沙箱

---

### 6. `pairing.md` — 配对（权限审批）

**用途：** 描述 OpenAcosmi 的"配对"机制，即**所有者显式审批**步骤，用于两种场景：

1. **DM 配对**：控制谁可以与 bot 对话
2. **设备配对**：控制哪些设备/节点可以加入网关网络

**关键信息：**

- 配对码：8 字符大写字母，1 小时过期
- 每频道最多 3 个待处理配对请求
- 设备配对可通过 Telegram `/pair` 命令完成
- 配对状态存储在 `~/.openacosmi/credentials/` 目录

---

### 7. `location.md` — 位置信息解析

**用途：** 描述聊天频道中分享位置的标准化处理。将位置信息转为人类可读文本和结构化字段。

**支持平台：** Telegram（位置标记/场所/实时位置）、WhatsApp（位置消息/实时位置）、Matrix（`m.location`）

**上下文字段：** `LocationLat`、`LocationLon`、`LocationAccuracy`、`LocationName`、`LocationAddress`、`LocationSource`、`LocationIsLive`

---

### 8. `troubleshooting.md` — 频道故障排查

**用途：** 快速频道级故障排查指南，为每个频道提供常见问题特征（symptom）和修复方案。

**覆盖频道：** WhatsApp、Telegram、Discord、Slack、iMessage/BlueBubbles、Signal、Matrix

**通用诊断命令：**

```bash
openacosmi status
openacosmi gateway status
openacosmi logs --follow
openacosmi doctor
openacosmi channels status --probe
```

---

## 二、主要平台频道文档（6 个）

### 9. `whatsapp.md` — WhatsApp 集成

**用途：** WhatsApp Web 频道的完整文档，通过 Baileys 库实现。Gateway 拥有 WhatsApp 会话。

**核心功能：**

- 支持多账户（multi-account）
- 登录方式：QR 码链接设备（`openacosmi channels login`）
- DM 策略：`pairing`（默认）/ `allowlist` / `open` / `disabled`
- 群消息：策略控制 + 提及门控 + 历史注入（默认 50 条）
- 确认反应（ackReaction）：收到消息后自动发送 emoji 反应
- 自聊模式（selfChatMode）：个人号使用场景
- 已读回执控制、消息分块（默认 4000 字符）、媒体限制
- 心跳检测与重连策略

**重要提示：** 不推荐使用 Twilio（24 小时回复窗口限制 + 频繁封号）；不推荐 Bun 运行时

---

### 10. `telegram.md` — Telegram Bot API 集成

**用途：** 通过 grammY 框架实现的 Telegram Bot API 频道，功能最全面的平台之一。

**核心功能：**

- 长轮询（默认）或 Webhook 模式
- 草稿流式传输（Draft Streaming）：DM 中实时显示生成中的回复
- Telegram HTML 格式化：Markdown 转为 Telegram 安全 HTML
- 贴纸系统：接收/缓存/搜索/发送贴纸，含 AI 视觉描述缓存
- 内联按钮（Inline Buttons）：回调式按钮 UI
- 论坛主题（Forum Topics）：每个 topic 独立会话
- 反应通知系统：`off` / `own` / `all`
- 自定义命令注册到 Telegram 菜单
- 多账户支持、代理支持、重试策略

**配置项极为丰富**，是所有频道中配置选项最多的。

---

### 11. `discord.md` — Discord Bot API 集成

**用途：** Discord Bot API + Gateway 集成，支持 DM、服务器频道和线程。

**核心功能：**

- 基于 Guild（服务器）和 Channel（频道）的细粒度权限控制
- 需要 Message Content Intent 和 Server Members Intent
- DM 配对（默认）、群策略白名单
- PluralKit 支持（代理消息解析）
- 丰富的工具操作：反应/贴纸/投票/权限/线程/固定/搜索/角色/频道管理/审核等
- 回复标签线程控制
- 原生斜杠命令支持
- 执行审批按钮 UI（DM 中）

**操作权限粒度最细**，支持 19 种可独立开关的工具操作组。

---

### 12. `slack.md` — Slack 集成

**用途：** 通过 Bolt SDK 的 Slack 工作区应用集成，支持 Socket 模式和 HTTP 模式。

**核心功能：**

- Socket Mode（默认）或 HTTP Events API
- 三种 Token：App Token (`xapp-`)、Bot Token (`xoxb-`)、User Token (`xoxp-`，可选只读)
- 回复线程控制：`off` / `first` / `all`，支持按聊天类型（DM/群/频道）分别设置
- 斜杠命令支持
- DM 配对、频道白名单
- 工具操作：反应/消息/固定/成员信息/自定义 emoji 列表
- 提供完整 Slack App Manifest 模板

---

### 13. `feishu.md` — 飞书/Lark 集成

**用途：** 飞书（Lark）机器人集成，通过插件安装，使用 WebSocket 长连接接收事件。

**核心功能：**

- 无需公网 URL（WebSocket 长连接）
- 安装方式：插件（`openacosmi plugins install @openacosmi/feishu`）
- 设置向导或 CLI 添加
- 国际版 Lark 支持（`domain: "lark"`）
- DM 配对（默认）、群策略（默认 open）
- 流式回复（Streaming Card）：通过交互卡片实时更新
- 多 Agent 路由（通过 bindings）
- 支持接收：文本/富文本/图片/文件/音频/视频/贴纸
- 多账户支持

**已有中文版详细设置步骤和截图引用。**

---

### 14. `signal.md` — Signal 集成

**用途：** 通过 `signal-cli` 外部 CLI 的 Signal 集成，注重隐私。

**核心功能：**

- Gateway 通过 HTTP JSON-RPC + SSE 与 `signal-cli` 通信
- 推荐使用独立 Signal 号码
- DM 配对（默认）、群策略
- 外部守护进程模式（`httpUrl`）：适合管理慢启动的 JVM
- 打字指示器和已读回执支持
- 反应支持（通过消息工具）
- UUID 格式发送者支持

---

## 三、次要平台频道文档（6 个）

### 15. `bluebubbles.md` — BlueBubbles（macOS iMessage REST）

**用途：** 通过 BlueBubbles macOS 服务端的 REST API 实现 iMessage 集成。**推荐用于新的 iMessage 集成**（替代旧版 imsg 方案）。

**核心功能：**

- Gateway 通过 REST API 与 BlueBubbles 通信（`/api/v1/ping`、`/message/text` 等）
- 入站消息通过 Webhook 接收，出站回复/打字指示器/已读回执/Tapback 反应通过 REST 调用
- 高级操作：编辑/撤回/回复线程/消息特效/群组管理（更名/设图标/添加删除成员）
- DM 配对（默认）、群策略白名单、提及门控
- 短 ID 与完整 ID 系统：短 ID 节省 Token 但内存存储可能过期
- VM/无头环境保活：提供 AppleScript + LaunchAgent 定期唤醒 Messages.app

**重要限制：** macOS Tahoe (26) 上编辑功能目前不可用，群头像更新也不稳定

---

### 16. `imessage.md` — iMessage（旧版 imsg CLI）

**用途：** 旧版 iMessage 集成，通过 `imsg` CLI 的 JSON-RPC over stdio 实现。**新安装推荐使用 BlueBubbles。**

**核心功能：**

- Gateway 生成 `imsg rpc` 进程进行通信
- 需要 macOS + Full Disk Access + Automation 权限
- 支持远程 SSH 方案（`cliPath` 指向 SSH 包装脚本）
- Tailscale 跨网络方案详细文档
- "类群组线程"（`is_group=false` 但多参与者）的特殊处理
- 多账户支持（bot 专用 macOS 用户）

**标记为 legacy，未来可能移除。**

---

### 17. `msteams.md` — Microsoft Teams 集成

**用途：** Microsoft Teams Bot 集成（插件形式），这是所有频道中设置最复杂的。共 770 行。

**核心功能：**

- 需要 Azure Bot 注册 + App ID + Client Secret + Tenant ID
- 需要 Teams App Manifest（JSON + 图标打包）+ RSC 权限
- DM 配对（默认）+ Guild/频道级别白名单
- 两种频道样式：Posts（经典卡片式）和 Threads（Slack 风格），需手动配置 `replyStyle`
- 发送文件到群聊需要 SharePoint（`sharePointSiteId`）+ Graph API 权限
- Adaptive Cards 支持（投票、任意卡片发送）
- 图片/附件需要 Microsoft Graph Application 权限 + 管理员同意

**破坏性变更（2026.1.15）：** MS Teams 已从核心包移出，需作为插件安装

---

### 18. `googlechat.md` — Google Chat 集成

**用途：** Google Chat App 集成，通过 Chat API Webhook（仅 HTTP）实现。

**核心功能：**

- 需要 Google Cloud 项目 + Service Account + JSON 密钥
- Chat App 需在 Google Cloud Console 创建并启用交互功能
- 仅 Webhook（需 HTTPS 公网端点）
- 详细的公网暴露方案：Tailscale Funnel / Caddy 反向代理 / Cloudflare Tunnel
- DM 配对（默认）、群组提及门控
- 反应支持、打字指示器（消息或反应）

---

### 19. `mattermost.md` — Mattermost 集成

**用途：** 自托管团队消息平台 Mattermost 的 Bot 集成（插件形式）。

**核心功能：**

- 使用 Bot Token + WebSocket 事件连接
- 三种聊天模式：`oncall`（默认，需 @提及）、`onmessage`（全部响应）、`onchar`（前缀触发）
- DM 配对（默认）、群策略白名单
- 多账户支持（不同 Mattermost 实例）

**文档最简洁**（139 行），配置直接。

---

### 20. `matrix.md` — Matrix 集成

**用途：** Matrix 去中心化消息协议的集成（插件形式），通过 `@vector-im/matrix-bot-sdk` 实现。

**核心功能：**

- OpenAcosmi 以 Matrix 用户身份登录（需要一个 Matrix 账户）
- 端到端加密（E2EE）支持：通过 Rust crypto SDK
- DM、房间（Rooms）、线程（Threads）、媒体、反应、投票、位置全部支持
- 自动加入邀请房间（可配置白名单）
- 加密状态存储在 SQLite，按 account + access token 隔离
- 登录方式：access token 或 userId + password
- Beeper 客户端兼容（需启用 E2EE）

---

## 四、小众/插件频道文档（8 个）

### 21. `grammy.md` — grammY 技术说明

**用途：** 内部技术说明文档，记录 Telegram 从 fetch 实现迁移到 grammY 框架的原因和成果。**非用户配置指南**。

**核心内容：** grammY 成为 Telegram 唯一客户端路径；内置限流器（throttler）；支持代理、Webhook、长轮询。与 `telegram.md` 配套参考。

---

### 22. `line.md` — LINE Messaging API 集成

**用途：** LINE Messaging API 插件集成，面向日本市场的主流即时通讯平台。

**核心功能：**

- 需要 Channel Access Token + Channel Secret
- Webhook 接收事件（需 HTTPS）
- 文本分块上限 5000 字符
- Markdown 自动转 Flex 卡片
- 快速回复（Quick Replies）、位置、Flex 消息、模板消息
- 不支持反应和线程

---

### 23. `nostr.md` — Nostr 去中心化消息集成

**用途：** Nostr 去中心化协议的 DM 频道（NIP-04 加密），面向 Web3/去中心化用户。

**核心功能：**

- 需要 Nostr 私钥（`nsec` 或 hex 格式）
- 默认中继：`relay.damus.io` 和 `nos.lol`
- NIP-01 profile metadata 发布
- 仅 DM（无群聊）、无媒体附件
- NIP-17/NIP-44 计划中

---

### 24. `tlon.md` — Tlon/Urbit 去中心化消息集成

**用途：** Tlon（基于 Urbit 的去中心化通讯）插件集成。

**核心功能：**

- 连接到 Urbit ship 进行 DM 和群聊
- 自动发现群频道（可禁用）
- 群回复需要 @ 提及
- 线程回复支持
- 媒体仅文本 + URL 降级（无原生上传）
- 无反应、投票支持

---

### 25. `twitch.md` — Twitch 聊天集成

**用途：** Twitch 直播聊天频道插件集成，通过 IRC 连接。

**核心功能：**

- 需要 OAuth Access Token（`chat:read` + `chat:write` 权限）
- 默认需要 @提及
- 基于角色（moderator/owner/vip/subscriber/all）或用户 ID 的访问控制
- 可选 Token 自动刷新（需创建 Twitch 应用）
- 消息上限 500 字符，自动按词边界分块
- 多账户支持（多频道）

---

### 26. `zalo.md` — Zalo Bot API 集成

**用途：** Zalo Bot API 集成（实验性），面向越南市场。

**核心功能：**

- 仅 DM（群聊按 Zalo 官方文档"即将推出"）
- 长轮询（默认）或 Webhook 模式
- 出站文本分块 2000 字符
- 图片消息收发支持
- DM 配对（默认）
- 流式传输默认阻止（2000 字符限制致使用价值低）

---

### 27. `zalouser.md` — Zalo Personal（非官方个人号自动化）

**用途：** 通过 `zca-cli` 工具自动化 Zalo 个人账户（非官方，有封号风险）。

**核心功能：**

- QR 码扫码登录
- 通过 `zca listen` 接收消息、`zca msg` 发送回复
- DM + 群聊支持
- 目录查询 CLI（查找好友/群组 ID）
- 多账户通过 `zca profiles` 映射

**⚠️ 警告：** 非官方集成，可能导致账户封禁

---

### 28. `feishu-multimodal-permissions.md` — 飞书多模态权限指南

**用途：** 飞书多模态功能（图片/语音/文件下载）的权限配置专项指南。**已是中文文档**。

**核心内容：**

- 所需权限：`im:message` + `im:message:readonly` + `im:resource`
- API 接口：`GET /open-apis/im/v1/messages/{message_id}/resources/{file_key}`
- 认证：`tenant_access_token`
- 各消息类型的 `file_key` 获取路径
- 常见错误码和解决方案
- 与钉钉/企微的多模态能力对比

---

### 29. `nextcloud-talk.md` — Nextcloud Talk 集成

**用途：** Nextcloud Talk（自托管协作平台的聊天功能）Webhook 机器人集成。

**核心功能：**

- 通过 Nextcloud OCC 命令注册 Bot + 共享密钥
- DM + 房间支持、反应支持
- 媒体仅 URL 形式（Bot API 不支持上传）
- Webhook 无法区分 DM 和房间（需配置 `apiUser` + `apiPassword` 启用区分）
- Bot 无法主动发起 DM

---

## 五、总结

### 文档分类统计

| 类别 | 数量 | 文件 |
|------|------|------|
| 总览与路由 | 3 | `index.md`, `channel-routing.md`, `broadcast-groups.md` |
| 群聊行为 | 3 | `group-messages.md`, `groups.md`, `pairing.md` |
| 辅助功能 | 2 | `location.md`, `troubleshooting.md` |
| 内置频道 | 5 | `whatsapp.md`, `telegram.md`, `discord.md`, `slack.md`, `signal.md` |
| 插件频道 | 14 | 其余所有平台 |
| 技术说明 | 1 | `grammy.md` |
| 已有中文 | 1 | `feishu-multimodal-permissions.md` |

### 关键发现

1. **`feishu-multimodal-permissions.md` 已经是中文**，无需翻译
2. **`grammy.md` 是内部技术备忘**，非用户面向文档，翻译优先级低
3. **`msteams.md` 最长**（770 行），设置复杂度最高
4. **所有频道共享统一模式**：DM 策略 / 群策略 / 白名单 / 配对 / 提及门控
5. **插件频道**（14 个）需通过 `openacosmi plugins install` 单独安装
6. **文档一致性高**，每个平台文档都遵循相同结构：快速设置 → 配置 → 访问控制 → 故障排查
