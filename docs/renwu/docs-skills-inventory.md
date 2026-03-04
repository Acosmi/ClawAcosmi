# 项目文档与技能资产清单

> 生成日期：2026-02-23 | 状态：**仅分类，不删除**

---

## 统计总览

| 区域 | 文件数 | 说明 |
|------|--------|------|
| `docs/` 英文原文 | ~312 | 上游项目文档 |
| `docs/zh-CN/` 已翻译 | 311 | 中文翻译（覆盖率约 100%） |
| `docs/ja-JP/` 日文 | 4 | 仅少量 |
| `docs/renwu/` 自有任务 | 211 | OpenAcosmi 任务追踪 |
| `docs/gouji/` 自有构架 | 40 | OpenAcosmi 架构文档 |
| `docs/v2renwu/` 自有 | 8 | V2 版本任务 |
| `docs/refactor/` 自有 | 5 | 重构文档 |
| `docs/architecture/` 自有 | 1 | CLI 架构文档 |
| `skills/` 捆绑技能 | 53 目录 | 上游项目技能 |
| `extensions/` 插件技能 | 6 目录 | 上游 + 飞书技能 |

---

## A. 文档目录分类 (`docs/`)

### A1. 自有文档（✅ 保留，已中文）

| 目录 | 文件数 | 说明 |
|------|--------|------|
| `renwu/` | 211 | 任务追踪、审计报告、修复计划、延迟项 |
| `gouji/` | 40 | 模块架构文档 |
| `v2renwu/` | 8 | V2 版本计划 |
| `refactor/` | 5 | 重构文档 |
| `architecture/` | 1 | CLI 三层架构 |
| `前端审计未改.md` | 1 | 前端审计报告 |

---

### A2. 上游频道文档 (`docs/channels/` — 28 个)

| 文件 | 原标题 | 中文说明 | 建议 |
|------|--------|----------|------|
| `discord.md` | Discord | Discord 机器人集成 | ✅ 保留 |
| `telegram.md` | Telegram | Telegram 机器人集成 | ✅ 保留 |
| `slack.md` | Slack | Slack 工作区集成 | ✅ 保留 |
| `whatsapp.md` | WhatsApp | WhatsApp Business 集成 | ✅ 保留 |
| `feishu.md` | 飞书 | 飞书开放平台集成 | ✅ 保留 |
| `line.md` | LINE | LINE 消息平台集成 | ✅ 保留 |
| `signal.md` | Signal | Signal 隐私消息集成 | ✅ 保留 |
| `matrix.md` | Matrix | Matrix 去中心化通信 | ✅ 保留 |
| `msteams.md` | Microsoft Teams | Teams 企业通信集成 | ✅ 保留 |
| `googlechat.md` | Google Chat | Google Chat 集成 | ✅ 保留 |
| `mattermost.md` | Mattermost | 自托管团队协作 | 🔶 酌情 |
| `nextcloud-talk.md` | Nextcloud Talk | Nextcloud 通信集成 | 🔶 酌情 |
| `nostr.md` | Nostr | 去中心化社交协议 | 🔶 酌情 |
| `tlon.md` | Tlon | Urbit 生态通信 | 🔶 酌情 |
| `twitch.md` | Twitch | 直播互动集成 | 🔶 酌情 |
| `zalo.md` | Zalo | 越南通信平台 | 🔶 酌情 |
| `zalouser.md` | Zalo User | Zalo 用户模式 | 🔶 酌情 |
| `imessage.md` | iMessage | Apple iMessage 集成 | 🔶 酌情 |
| `bluebubbles.md` | BlueBubbles | iMessage 替代方案 | 🔶 酌情 |
| `grammy.md` | grammY | Telegram 框架文档 | 🔶 酌情 |
| `channel-routing.md` | Channel Routing | 频道路由机制 | ✅ 核心概念 |
| `broadcast-groups.md` | 广播组 | 消息广播机制 | ✅ 核心概念 |
| `group-messages.md` | Group Messages | 群消息处理 | ✅ 核心概念 |
| `groups.md` | Groups | 群组管理 | ✅ 核心概念 |
| `location.md` | 位置 | 位置消息处理 | ✅ 核心概念 |
| `pairing.md` | 配对 | 设备配对流程 | ✅ 核心概念 |
| `troubleshooting.md` | 故障排除 | 频道调试指南 | ✅ 核心概念 |
| `index.md` | 索引 | 频道总览首页 | ✅ 保留 |

---

### A3. 上游 CLI 文档 (`docs/cli/` — 41 个)

> 全部为 CLI 子命令参考文档。**zh-CN/ 已有完整翻译**。

| 文件 | 中文说明 | 建议 |
|------|----------|------|
| `agent.md` | Agent 命令 | ✅ |
| `agents.md` | Agent 管理 | ✅ |
| `browser.md` | 浏览器沙箱 | ✅ |
| `channels.md` | 频道管理 | ✅ |
| `config.md` | 配置管理 | ✅ |
| `configure.md` | 配置向导 | ✅ |
| `cron.md` | 定时任务 | ✅ |
| `dashboard.md` | 仪表盘 | ✅ |
| `devices.md` | 设备管理 | ✅ |
| `dns.md` | DNS 配置 | ✅ |
| `doctor.md` | 系统诊断 | ✅ |
| `gateway.md` | 网关命令 | ✅ |
| `health.md` | 健康检查 | ✅ |
| `hooks.md` | 钩子管理 | ✅ |
| `logs.md` | 日志查看 | ✅ |
| `memory.md` | 记忆管理 | ✅ |
| `message.md` | 消息发送 | ✅ |
| `models.md` | 模型管理 | ✅ |
| `node.md` / `nodes.md` | 节点管理 | ✅ |
| `onboard.md` | 初始化引导 | ✅ |
| `pairing.md` | 设备配对 | ✅ |
| `plugins.md` | 插件管理 | ✅ |
| `sandbox.md` | 沙箱管理 | ✅ |
| `security.md` | 安全管理 | ✅ |
| `sessions.md` | 会话管理 | ✅ |
| `setup.md` | 安装设置 | ✅ |
| `skills.md` | 技能管理 | ✅ |
| `status.md` | 状态查看 | ✅ |
| `其他...` | 42 个命令参考 | ✅ 全保留 |

---

### A4. 上游工具文档 (`docs/tools/` — 24 个)

| 文件 | 中文说明 | 建议 |
|------|----------|------|
| `skills.md` | 技能系统概览 | ✅ 核心 |
| `creating-skills.md` | 创建技能指南 | ✅ 核心 |
| `skills-config.md` | 技能配置参考 | ✅ 核心 |
| `exec.md` | 命令执行工具 | ✅ 核心 |
| `exec-approvals.md` | 执行审批机制 | ✅ 核心 |
| `browser.md` | 浏览器自动化工具 | ✅ |
| `browser-login.md` | 浏览器登录 | ✅ |
| `browser-linux-troubleshooting.md` | Linux 浏览器排障 | 🔶 |
| `slash-commands.md` | 斜杠命令 | ✅ 核心 |
| `subagents.md` | 子 Agent 系统 | ✅ 核心 |
| `multi-agent-sandbox-tools.md` | 多 Agent 沙箱工具 | ✅ |
| `reactions.md` | 表情回应 | ✅ |
| `thinking.md` | 思考级别配置 | ✅ |
| `web.md` | Web UI 工具 | ✅ |
| `lobster.md` | Lobster 扩展 | 🔶 上游特有 |
| `clawhub.md` | ClawHub 技能市场 | 🔶 上游特有 |
| `firecrawl.md` | Firecrawl 网页抓取 | 🔶 |
| `chrome-extension.md` | Chrome 扩展 | 🔶 |
| `llm-task.md` | LLM 任务工具 | 🔶 |
| `plugin.md` | 插件开发 | ✅ |
| `agent-send.md` | Agent 消息发送 | ✅ |
| `apply-patch.md` | 补丁应用工具 | 🔶 |
| `elevated.md` | 提权模式 | ✅ |
| `index.md` | 工具索引 | ✅ |

---

### A5. 上游提供商文档 (`docs/providers/` — 22 个)

| 文件 | 中文说明 | 建议 |
|------|----------|------|
| `anthropic.md` | Anthropic (Claude) | ✅ 核心 |
| `openai.md` | OpenAI (GPT) | ✅ 核心 |
| `ollama.md` | Ollama 本地模型 | ✅ 核心 |
| `qwen.md` | 通义千问 | ✅ 国内必需 |
| `moonshot.md` | 月之暗面 (Kimi) | ✅ 国内必需 |
| `glm.md` | 智谱 ChatGLM | ✅ 国内必需 |
| `minimax.md` | MiniMax | ✅ 国内必需 |
| `qianfan.md` | 百度千帆 | ✅ 国内必需 |
| `xiaomi.md` | 小米大模型 | ✅ 国内必需 |
| `zai.md` | Zai 模型 | ✅ 国内必需 |
| `bedrock.md` | AWS Bedrock | 🔶 酌情 |
| `github-copilot.md` | GitHub Copilot | 🔶 酌情 |
| `openrouter.md` | OpenRouter 聚合 | 🔶 酌情 |
| `venice.md` | Venice AI | 🔶 酌情 |
| `deepgram.md` | Deepgram 语音 | 🔶 酌情 |
| `synthetic.md` | 合成测试 | 🔶 酌情 |
| `openacosmi.md` | OpenAcosmi | 🔶 酌情 |
| `cloudflare-ai-gateway.md` | CF AI 网关 | 🔶 酌情 |
| `vercel-ai-gateway.md` | Vercel AI 网关 | 🔶 酌情 |
| `claude-max-api-proxy.md` | Claude Max 代理 | 🔶 酌情 |
| `models.md` | 模型配置总览 | ✅ 核心 |
| `index.md` | 提供商索引 | ✅ |

---

### A6. 其他上游文档目录

| 目录 | 文件数 | 中文说明 | 建议 |
|------|--------|----------|------|
| `concepts/` | 28 | 核心概念文档（Agent 循环、会话、模型、流式、系统提示词等） | ✅ 全保留 |
| `gateway/` | 29 | 网关运维文档（认证、配置、发现、健康、沙箱、远程访问等） | ✅ 全保留 |
| `install/` | 16 | 安装指南（npm、Docker、各平台安装） | ✅ 全保留 |
| `start/` | 13 | 快速开始教程 | ✅ 全保留 |
| `platforms/` | 27 | 平台文档（macOS/Linux/Windows/Android/Pi/Oracle） | ✅ 全保留 |
| `nodes/` | 9 | 多节点部署文档 | ✅ 全保留 |
| `security/` | 4 | 安全文档（加密、审计、策略） | ✅ 全保留 |
| `plugins/` | 4 | 插件开发文档 | ✅ 全保留 |
| `hooks/` | 1 | 钩子系统文档 | ✅ 全保留 |
| `automation/` | 8 | 自动化/Cron 文档 | ✅ 全保留 |
| `web/` | 5 | Web UI 文档 | ✅ 全保留 |
| `reference/` | 24 | 参考文档（配置模板、API 参考等） | ✅ 全保留 |
| `help/` | 9 | 帮助文档（FAQ、常见问题） | ✅ 全保留 |
| `experiments/` | 6 | 实验性功能文档 | 🔶 酌情 |
| `debug/` | 1 | 调试文档 | ✅ |
| `diagnostics/` | 1 | 诊断文档 | ✅ |

### A7. 根级独立文档

| 文件 | 中文说明 | 建议 |
|------|----------|------|
| `index.md` | 文档首页 | ✅ |
| `logging.md` | 日志系统文档 | ✅ |
| `network.md` | 网络配置文档 | ✅ |
| `tts.md` | 语音合成文档 | ✅ |
| `date-time.md` | 日期时间处理 | ✅ |
| `pi.md` | 树莓派集成架构 | ✅ |
| `pi-dev.md` | 树莓派开发工作流 | ✅ |
| `vps.md` | VPS 部署指南 | ✅ |
| `prose.md` | Prose 扩展文档 | 🔶 上游特有 |
| `brave-search.md` | Brave 搜索集成 | 🔶 |
| `perplexity.md` | Perplexity 集成 | 🔶 |
| `docs.json` | 文档站点配置 | ✅ 配置文件 |

### A8. 翻译覆盖率

| 目录 | 英文 | zh-CN | 覆盖率 |
|------|------|-------|--------|
| `automation/` | 8 | 8 | 100% |
| `channels/` | 28 | 28 | 100% |
| `cli/` | 41 | 41 | 100% |
| `concepts/` | 28 | 28 | 100% |
| `debug/` | 1 | 1 | 100% |
| `diagnostics/` | 1 | 1 | 100% |
| `experiments/` | 6 | 6 | 100% |
| `gateway/` | 29 | 29 | 100% |
| `help/` | 9 | ✅ | 100% |
| `hooks/` | 1 | ✅ | 100% |
| `install/` | 16 | ✅ | 100% |
| `nodes/` | 9 | ✅ | 100% |
| `platforms/` | 27 | ✅ | 100% |
| `plugins/` | 4 | ✅ | 100% |
| `providers/` | 22 | ✅ | 100% |
| `reference/` | 24 | ✅ | 100% |
| `security/` | 4 | ✅ | 100% |
| `start/` | 13 | ✅ | 100% |
| `tools/` | 24 | ✅ | 100% |
| `web/` | 5 | ✅ | 100% |
| **合计** | **~312** | **311** | **~100%** |

---

## B. 捆绑技能分类 (`skills/` — 53 个)

### B1. ✅ 通用/平台无关（建议保留）

| 技能 | 中文说明 |
|------|----------|
| `github` | GitHub CLI (`gh`) 集成 — Issues、PR、CI |
| `coding-agent` | 编程 Agent 集成（Codex/Claude Code/OpenAcosmi/Pi） |
| `skill-creator` | 技能创建器 — 设计、编写、打包技能 |
| `canvas` | 画布 — 在连接设备上展示 HTML 内容 |
| `healthcheck` | 主机安全加固 — 防火墙/SSH/更新审计 |
| `session-logs` | 会话日志搜索 — 用 jq 分析历史对话 |
| `tmux` | Tmux 远程控制 — 发送按键/抓取窗格输出 |
| `summarize` | 摘要生成 — URL/播客/本地文件转文字 |
| `weather` | 天气查询（无需 API Key） |
| `video-frames` | 视频帧提取 — ffmpeg 抽帧/剪辑 |
| `trello` | Trello 看板管理 |
| `notion` | Notion API 集成 |
| `obsidian` | Obsidian 笔记库操作 |

### B2. ✅ OpenAcosmi 专有

| 技能 | 中文说明 |
|------|----------|
| `acosmi-refactor` | Acosmi 重构核心技能 — TS→Go+Rust 重构指导 |
| `discord` | Discord 频道深度操控 — 消息/表情/投票/线程/频道管理 |
| `slack` | Slack 频道操控 — 回应/置顶/取消置顶 |
| `voice-call` | 语音通话 — 通过 voice-call 插件发起 |

### B3. 🔶 macOS 专属（酌情保留）

| 技能 | 中文说明 |
|------|----------|
| `apple-notes` | Apple 备忘录 — 通过 `memo` CLI 管理 |
| `apple-reminders` | Apple 提醒事项 — 通过 `remindctl` CLI 管理 |
| `bear-notes` | Bear 笔记 — 通过 `grizzly` CLI 管理 |
| `things-mac` | Things 3 任务管理 — URL Scheme + 数据库查询 |
| `imsg` | iMessage/SMS CLI — 消息/历史/监控 |
| `bluebubbles` | BlueBubbles — iMessage 替代集成 |
| `peekaboo` | Peekaboo — macOS UI 自动化截图 |
| `camsnap` | 摄像头抓帧 — RTSP/ONVIF 相机 |

### B4. 🔶 第三方 CLI 工具绑定（酌情删除）

| 技能 | 中文说明 | 依赖 |
|------|----------|------|
| `1password` | 1Password CLI 密码管理 | `op` |
| `openhue` | 飞利浦 Hue 灯光控制 | `openhue` |
| `sonoscli` | Sonos 音箱控制 | `sonoscli` |
| `spotify-player` | Spotify 终端播放 | `spogo` |
| `himalaya` | 终端邮件管理（IMAP/SMTP） | `himalaya` |
| `blucli` | BluOS 音响系统控制 | `blu` |
| `eightctl` | Eight Sleep 智能床垫控制 | `eightctl` |
| `ordercli` | 外卖订单查询（Foodora） | `ordercli` |
| `food-order` | 外卖重订（Foodora） | `ordercli` |
| `clawhub` | ClawHub 技能市场 CLI | `clawhub` |
| `gifgrep` | GIF 搜索/下载工具 | `gifgrep` |
| `songsee` | 音频频谱可视化 | `songsee` |
| `wacli` | WhatsApp CLI 发送/搜索 | `wacli` |
| `gog` | Google Workspace CLI | `gog` |
| `goplaces` | Google Places API 查询 | `goplaces` |
| `local-places` | 本地 Places API 代理 | 本地服务 |
| `sag` | ElevenLabs TTS（say 风格） | `sag` |
| `mcporter` | MCP Server 管理 CLI | `mcporter` |
| `oracle` | Oracle CLI 最佳实践 | `oracle` |
| `blogwatcher` | 博客/RSS 监控 | `blogwatcher` |
| `nano-pdf` | 自然语言 PDF 编辑 | `nano-pdf` |

### B5. 🔶 AI/模型工具绑定（酌情保留）

| 技能 | 中文说明 | 依赖 |
|------|----------|------|
| `gemini` | Gemini CLI 一键问答 | `gemini` |
| `openai-image-gen` | OpenAI 批量图片生成 | API Key |
| `openai-whisper` | 本地语音转文字（Whisper CLI） | `whisper` |
| `openai-whisper-api` | Whisper API 语音转文字 | API Key |
| `sherpa-onnx-tts` | 本地离线 TTS（sherpa-onnx） | `sherpa-onnx` |
| `nano-banana-pro` | Gemini 3 Pro 图片生成/编辑 | API Key |
| `model-usage` | 模型用量/成本统计 | `codexbar` |

---

## C. 插件技能 (`extensions/`)

### C1. ✅ 飞书技能（保留）

| 技能 | 中文说明 |
|------|----------|
| `feishu-doc` | 飞书文档操作 |
| `feishu-drive` | 飞书云盘操作 |
| `feishu-perm` | 飞书权限管理 |
| `feishu-wiki` | 飞书知识库操作 |

### C2. 🔶 上游插件技能

| 技能 | 中文说明 |
|------|----------|
| `extensions/lobster/SKILL.md` | Lobster 扩展技能 |
| `extensions/open-prose/skills/prose/` | Prose 写作助手技能 |

---

## D. 建议处理优先级

| 优先级 | 操作 | 范围 |
|--------|------|------|
| **暂不动** | 全部保持现状 | 所有文件 |
| 后续 P1 | 清理 B4（第三方 CLI 绑定 ~21 个） | `skills/` |
| 后续 P2 | 清理 B3 中用户明确不需要的 macOS 技能 | `skills/` |
| 后续 P3 | 审视 A2 中 🔶 标记的冷门频道文档 | `docs/channels/` |
| 后续 P4 | 检查 `docs.json` 站点配置是否需要去除已删技能引用 | `docs/docs.json` |
