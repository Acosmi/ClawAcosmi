# docs/cli 目录文档审计报告（中文）

> 审计日期：2026-03-01
> 共 41 个文件（含 `index.md` 总索引），逐个审计用途并翻译为中文。

---

## 1. `acp.md` — ACP 桥接（IDE 集成）

**用途：** 运行 ACP（Agent Client Protocol）桥接服务，连接 IDE 与 OpenAcosmi Gateway。通过 stdio 与 IDE 通信，通过 WebSocket 转发提示词到 Gateway。

**核心功能：**

- 将 ACP 会话映射到 Gateway session key
- 支持 Zed 编辑器配置
- 内置 ACP client 用于调试
- 支持 `--session` / `--session-label` 指定会话

**适用场景：** IDE 集成开发、ACP 会话路由调试

---

## 2. `agent.md` — 单次 Agent 执行

**用途：** 通过 Gateway 运行一次 agent 回合（turn），可选择投递回复。

**核心功能：**

- `--to` 指定发送目标
- `--agent` 指定目标 agent
- `--deliver` 投递回复到渠道
- `--thinking` 设定思考模式（medium 等）

**适用场景：** 脚本化调用 agent、自动化任务触发

---

## 3. `agents.md` — Agent 管理（多 Agent）

**用途：** 管理隔离的 agent（工作区 + 认证 + 路由）。

**核心功能：**

- `list` / `add` / `delete` agent
- `set-identity` 设置 agent 身份（名称、主题、emoji、头像）
- 支持 `IDENTITY.md` 身份文件
- 每个 agent 有独立工作区

**适用场景：** 多 agent 架构管理、agent 身份定制

---

## 4. `approvals.md` — 执行审批管理

**用途：** 管理本地主机、Gateway 主机或 node 主机上的 exec 审批白名单。

**核心功能：**

- `get` / `set` 审批规则
- `allowlist add/remove` 白名单管理
- 支持 `--gateway` / `--node` 指定目标主机
- 审批文件存储在 `~/.openacosmi/exec-approvals.json`

**适用场景：** 安全管控、命令执行权限管理

---

## 5. `browser.md` — 浏览器控制

**用途：** 管理 OpenAcosmi 的浏览器控制服务器，执行浏览器操作（标签页、截图、导航、点击、输入等）。

**核心功能：**

- Profile 管理（openacosmi 专用实例 / chrome 扩展中继）
- 标签页操作（tabs / open / focus / close）
- 快照 / 截图 / 导航 / 点击 / 输入
- Chrome 扩展中继安装
- 远程浏览器控制（通过 node host 代理）

**适用场景：** 浏览器自动化、远程浏览器操控

---

## 6. `channels.md` — 渠道管理

**用途：** 管理聊天渠道账号及其运行状态（WhatsApp / Telegram / Discord / Slack / Signal / iMessage 等）。

**核心功能：**

- `list` / `status` / `capabilities` 查看状态
- `add` / `remove` 添加/移除渠道
- `login` / `logout` 交互式登录
- `resolve` 名称到 ID 的解析
- 能力探测（Discord intents、Slack scopes 等）

**适用场景：** 多渠道接入管理、渠道状态排查

---

## 7. `config.md` — 配置读写

**用途：** 以 get / set / unset 方式非交互式读取或编辑配置值。

**核心功能：**

- 点号或方括号路径表示法：`agents.list[0].id`
- 值自动按 JSON5 解析，`--json` 强制 JSON5
- 修改后需重启 Gateway

**适用场景：** 脚本化配置操作、CI/CD 流水线中自动配置

---

## 8. `configure.md` — 交互式配置向导

**用途：** 交互式设置凭据、设备和 agent 默认值。

**核心功能：**

- 包含模型多选（`agents.defaults.models` 白名单）
- 渠道服务配置（Slack/Discord/Matrix/Teams）自动解析名称到 ID
- 可按 `--section` 指定配置区域

**适用场景：** 首次安装后的配置向导、凭据设置

---

## 9. `cron.md` — 定时任务

**用途：** 管理 Gateway 调度器的定时任务（cron jobs）。

**核心功能：**

- `add` / `edit` / `list` / `delete` 定时任务
- 支持 `--announce` 投递（替代旧的 `--deliver`）
- 一次性任务（`--at`）默认执行后自动删除
- 循环任务失败后采用指数退避重试（30s → 1m → 5m → 15m → 60m）

**适用场景：** 自动化调度、定期唤醒 agent

---

## 10. `dashboard.md` — 打开控制面板

**用途：** 使用当前认证打开 Control UI（Web 控制台）。

**核心功能：**

- `--no-open` 仅打印 URL 而不打开浏览器

**适用场景：** 快速访问 Web 管理界面

---

## 11. `devices.md` — 设备配对管理

**用途：** 管理设备配对请求和设备级令牌。

**核心功能：**

- `list` 列出待配对请求和已配对设备
- `approve` / `reject` 审批配对请求
- `rotate` 轮换设备令牌
- `revoke` 吊销设备令牌
- 需要 `operator.pairing` 权限范围

**适用场景：** 多设备安全管理、令牌生命周期管理

---

## 12. `directory.md` — 目录查询

**用途：** 对支持目录查询的渠道进行联系人/群组/"自己"的 ID 查找。

**核心功能：**

- `self` / `peers list` / `groups list` 子命令
- 结果可配合 `message send --target` 使用
- 各渠道 ID 格式说明（WhatsApp E.164、Telegram @username 等）

**适用场景：** 查找渠道联系人/群组 ID 以用于消息发送

---

## 13. `dns.md` — DNS 发现辅助

**用途：** 广域发现的 DNS 辅助工具（Tailscale + CoreDNS），当前聚焦 macOS + Homebrew CoreDNS。

**核心功能：**

- `dns setup` / `dns setup --apply`

**适用场景：** 通过 DNS-SD 实现跨网络的 Gateway 自动发现

---

## 14. `docs.md` — 在线文档搜索

**用途：** 从终端搜索 OpenAcosmi 在线文档索引。

**核心功能：**

- `openacosmi docs <关键词>` 即时搜索

**适用场景：** 快速查阅文档、开发时即时查询

---

## 15. `doctor.md` — 健康检查与修复

**用途：** 对 Gateway 和渠道进行健康检查，并提供快速修复建议。

**核心功能：**

- `--repair` / `--fix` 自动修复（备份配置后清理未知配置键）
- `--deep` 深度扫描
- 交互式修复需要 TTY 环境
- 检测 macOS `launchctl` 环境变量覆盖问题

**适用场景：** 连接/认证问题排查、更新后合理性检查

---

## 16. `gateway.md` — Gateway 服务

**用途：** OpenAcosmi 核心 WebSocket 服务器的运行、查询和发现。

**核心功能：**

- `gateway run` 运行 Gateway 进程
- `gateway health / status / probe` 查询运行状态
- `gateway call <method>` 低级 RPC 调用
- `gateway install / start / stop / restart / uninstall` 服务管理
- `gateway discover` Bonjour 局域网发现
- 支持 `--bind` 模式（loopback/lan/tailnet/auto/custom）
- Tailscale serve / funnel 集成
- SSH 远程探测

**适用场景：** Gateway 部署运维、服务调试、网络发现

---

## 17. `health.md` — Gateway 健康检查

**用途：** 通过 RPC 快速获取 Gateway 运行健康状态。

**核心功能：**

- `--verbose` 运行实时探测，含多账号计时
- 输出含多 agent 会话存储信息

**适用场景：** 快速确认 Gateway 是否正常运行

---

## 18. `hooks.md` — Agent 钩子管理

**用途：** 管理 agent 钩子（事件驱动自动化，如 `/new`、`/reset`、Gateway 启动）。

**核心功能：**

- `list` / `info` / `check` / `enable` / `disable` 钩子
- `install` / `update` 安装/更新钩子包
- 内置钩子：
  - `session-memory` — `/new` 时保存会话上下文到记忆
  - `command-logger` — 记录所有命令事件到审计文件
  - `soul-evil` — 在清洗窗口或随机时替换 SOUL 内容
  - `boot-md` — Gateway 启动时执行 BOOT.md

**适用场景：** 自动化工作流、事件驱动行为定制

---

## 19. `logs.md` — 日志追踪

**用途：** 通过 RPC 远程追踪 Gateway 文件日志（无需 SSH）。

**核心功能：**

- `--follow` 实时跟踪
- `--json` 机器可读输出
- `--limit` 限制行数

**适用场景：** 远程日志查看、故障排查

---

## 20. `memory.md` — 语义记忆管理

**用途：** 管理语义记忆索引和搜索（由 `memory-core` 插件提供）。

**核心功能：**

- `status` 查看记忆状态（`--deep` 探测 vector + embedding 可用性）
- `index` 执行索引（`--verbose` 显示详情）
- `search <查询>` 语义搜索
- `--agent <id>` 限定特定 agent

**适用场景：** 语义记忆系统运维、知识检索调试

---

## 21. `message.md` — 消息发送与渠道操作

**用途：** 跨渠道的统一消息发送命令（Discord / Slack / Telegram / WhatsApp / Signal / iMessage / Teams 等）。

**核心功能：**

- **核心操作：** `send` / `poll` / `react` / `reactions` / `read` / `edit` / `delete` / `pin` / `unpin` / `search`
- **线程操作：** `thread create` / `thread list` / `thread reply`
- **表情/贴纸：** `emoji list` / `emoji upload` / `sticker send` / `sticker upload`
- **角色/成员/语音：** `role info/add/remove` / `channel info/list` / `member info` / `voice status`
- **事件：** `event list` / `event create`
- **审核（Discord）：** `timeout` / `kick` / `ban`
- **广播：** `broadcast` 多目标批量发送

**适用场景：** 跨平台消息操作的统一入口，功能最全面的 CLI 命令

---

## 22. `models.md` — 模型管理

**用途：** 模型发现、扫描和配置（默认模型、fallback、认证 profile）。

**核心功能：**

- `status` 查看已解析的默认/备选模型及认证概况（`--probe` 实时探测）
- `list` / `set` / `scan` 模型列表与切换
- `aliases list` / `fallbacks list` 别名和备选列表
- `auth add/login/setup-token/paste-token` 认证管理

**适用场景：** LLM 模型切换、Provider 认证配置

---

## 23. `node.md` — 无头 Node Host

**用途：** 运行无头 node host，通过 WebSocket 连接 Gateway，在远程机器上暴露 `system.run` / `system.which`。

**核心功能：**

- `node run` 前台运行
- `node install / status / stop / restart / uninstall` 服务管理
- 自动 browser proxy（零配置）
- 配对流程（首次连接需 Gateway 审批）
- 执行受 exec approvals 管控

**适用场景：** 远程命令执行、CI 节点、构建服务器接入

---

## 24. `nodes.md` — 节点管理

**用途：** 管理已配对节点（设备），调用节点能力。

**核心功能：**

- `list` / `pending` / `approve` / `status` 节点管理
- `invoke` / `run` 远程调用节点命令
- `--connected` / `--last-connected` 筛选在线节点
- 支持 exec-style 默认值和审批流程

**适用场景：** 多节点集群管理、远程命令执行

---

## 25. `onboard.md` — 初始引导向导

**用途：** 交互式初始引导向导（本地或远程 Gateway 设置）。

**核心功能：**

- `--flow quickstart` 最小化提示，自动生成 token
- `--flow manual` 完整手动配置
- `--mode remote` 远程 Gateway 连接
- `--non-interactive` 用于脚本自动化

**适用场景：** 首次安装部署、远程 Gateway 初始化

---

## 26. `pairing.md` — DM 配对审批

**用途：** 审批或检查 DM 配对请求（适用于支持配对的渠道）。

**核心功能：**

- `list <渠道>` 列出配对请求
- `approve <渠道> <code> --notify` 审批并通知

**适用场景：** WhatsApp 等渠道的私信配对安全管控

---

## 27. `plugins.md` — 插件管理

**用途：** 管理 Gateway 插件/扩展（进程内加载）。

**核心功能：**

- `list` / `info` / `enable` / `disable` / `doctor`
- `install` / `update` 安装和更新插件
- 支持 npm 包、本地目录、归档文件安装
- `--link` 链接本地目录（不复制）
- 所有插件需包含 `openacosmi.plugin.json` 清单文件

**适用场景：** 功能扩展、第三方插件管理

---

## 28. `reset.md` — 重置本地状态

**用途：** 重置本地配置/状态（保留 CLI 本身）。

**核心功能：**

- `--dry-run` 预览将被删除的内容
- `--scope config+creds+sessions` 指定重置范围

**适用场景：** 清理环境、恢复出厂设置

---

## 29. `sandbox.md` — 沙箱容器管理

**用途：** 管理 Docker 沙箱容器，用于隔离 agent 执行环境。

**核心功能：**

- `explain` 查看当前沙箱策略（模式/范围/工作区/工具策略）
- `list` 列出所有沙箱容器及状态
- `recreate` 重建容器（更新镜像/配置后使用）
- 配置项：`sandbox.mode`（off/non-main/all）、`sandbox.scope`（session/agent/shared）
- 自动清理：空闲 24h 或 7 天后自动回收

**适用场景：** 安全隔离执行环境、Docker 镜像更新后容器刷新

---

## 30. `security.md` — 安全审计

**用途：** 对配置/状态进行安全审计并提供修复建议。

**核心功能：**

- `audit` / `audit --deep` / `audit --fix`
- 检测多 DM 发送者共享主会话的风险
- 检测小模型无沙箱时使用 web/browser 工具的风险

**适用场景：** 安全合规检查、配置加固

---

## 31. `sessions.md` — 会话列表

**用途：** 列出已存储的对话会话。

**核心功能：**

- `--active 120` 仅显示近 120 分钟活跃会话
- `--json` 机器可读输出

**适用场景：** 查看会话活跃度、会话管理

---

## 32. `setup.md` — 初始化配置

**用途：** 初始化 `~/.openacosmi/openacosmi.json` 和 agent 工作区。

**核心功能：**

- `--workspace <路径>` 指定工作区路径
- `--wizard` 运行完整向导

**适用场景：** 首次运行的精简初始化（不走完整引导流程）

---

## 33. `skills.md` — 技能管理

**用途：** 检查技能（内置 + 工作区 + 托管覆盖），查看可用性和缺失依赖。

**核心功能：**

- `list` / `list --eligible` 列出技能
- `info <name>` 查看技能详情
- `check` 检查技能资格状态

**适用场景：** 技能系统调试、依赖缺失排查

---

## 34. `status.md` — 综合诊断

**用途：** 渠道 + 会话的综合诊断信息。

**核心功能：**

- `--all` 全量输出
- `--deep` 实时探测各渠道（WhatsApp/Telegram/Discord/Slack/Signal 等）
- `--usage` 显示用量快照
- 输出包含 Gateway + node host 服务状态、更新渠道、git SHA

**适用场景：** 一站式系统状态概览、粘贴到调试报告

---

## 35. `system.md` — 系统事件与心跳

**用途：** Gateway 系统级辅助工具：入队系统事件、控制心跳、查看 presence。

**核心功能：**

- `system event --text <文本> --mode now` 立即触发系统事件
- `system heartbeat enable/disable/last` 心跳控制
- `system presence` 查看 presence 条目
- 系统事件是临时的，不持久化

**适用场景：** 系统事件注入、心跳调试

---

## 36. `tui.md` — 终端 UI

**用途：** 打开连接到 Gateway 的终端用户界面。

**核心功能：**

- 支持 `--url` / `--token` / `--session` 远程连接
- `--deliver` 投递模式

**适用场景：** 无 Web UI 时的终端交互、远程 Gateway 会话

---

## 37. `uninstall.md` — 卸载

**用途：** 卸载 Gateway 服务 + 本地数据（CLI 本身保留）。

**核心功能：**

- `--all --yes` 全量删除
- `--dry-run` 预览

**适用场景：** 完整清理 Gateway 安装

---

## 38. `update.md` — 安全更新

**用途：** 安全更新 OpenAcosmi 并切换 stable / beta / dev 频道。

**核心功能：**

- `update` / `update status` / `update wizard`
- `--channel <stable|beta|dev>` 切换更新频道
- `--no-restart` 更新后不重启 Gateway
- Git checkout 流程：clean worktree → 切换分支/标签 → preflight lint → 构建 → doctor 检查
- Dev 频道：自动回溯最多 10 个 commit 寻找稳定构建
- `--update` 简写

**适用场景：** 版本管理、频道切换、CI/CD 更新流程

---

## 39. `voicecall.md` — 语音通话

**用途：** 语音通话插件的 CLI 命令（需安装并启用 voice-call 插件）。

**核心功能：**

- `call` / `continue` / `end` / `status` 通话操作
- `expose` / `unexpose` Webhook 暴露（Tailscale）

**适用场景：** 电话通知、语音交互

---

## 40. `webhooks.md` — Webhook 辅助

**用途：** Webhook 辅助工具和集成（当前主要支持 Gmail Pub/Sub）。

**核心功能：**

- `webhooks gmail setup --account <邮箱>`
- `webhooks gmail run`

**适用场景：** Gmail 事件接入、Webhook 集成

---

## 41. `index.md` — CLI 总索引

**用途：** CLI 命令的总索引页面（27KB），汇总所有子命令的概要说明和用法参考。作为 docs/cli 目录的入口文档。

**适用场景：** 全局命令速查、文档导航

---

# 总结

| 分类 | 命令 |
|------|------|
| **核心服务** | `gateway`, `health`, `status`, `doctor`, `logs` |
| **配置管理** | `config`, `configure`, `setup`, `onboard`, `reset`, `uninstall` |
| **Agent 管理** | `agent`, `agents`, `sessions`, `memory` |
| **渠道通信** | `channels`, `message`, `directory`, `pairing` |
| **安全管控** | `approvals`, `security`, `sandbox`, `devices` |
| **自动化** | `cron`, `hooks`, `webhooks`, `system` |
| **扩展系统** | `plugins`, `skills`, `models`, `browser` |
| **网络发现** | `dns`, `nodes`, `node` |
| **IDE 集成** | `acp` |
| **界面入口** | `dashboard`, `tui` |
| **维护工具** | `update`, `docs`, `voicecall` |
