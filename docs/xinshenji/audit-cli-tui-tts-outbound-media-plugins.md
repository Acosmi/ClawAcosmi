---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/{cli, tui, tts, outbound, media, plugins}
verdict: Pass with Notes
---

# 审计报告: cli/tui/tts/outbound/media/plugins — CLI工具/语音/外发/媒体/插件

## 模块概览

| 模块 | 文件数 | 核心职责 |
|------|--------|---------|
| `cli` | 13 | CLI 工具函数（参数解析/banner/dotenv/config guard） |
| `tui` | 28 | 终端 TUI（Bubble Tea + lipgloss） |
| `tts` | 11 | 多 Provider 语音合成（OpenAI/ElevenLabs/Edge） |
| `outbound` | 9 | 外发消息路由（13个渠道会话解析器） |
| `media` | 20+15 | 媒体存储/图像处理/STT/文档转换/多模态理解 |
| `plugins` | 19 | 插件发现/安装/注册/更新/运行时管理 |

---

## cli — CLI 工具层

### [PASS] 正确性: dotenv 加载 (dotenv.go)

- **位置**: `cli/dotenv.go`
- **分析**: 支持 `.env` 文件加载，按行解析 `KEY=VALUE` 格式，处理引号包裹、注释忽略。有测试覆盖 (`dotenv_test.go`)。
- **风险**: None

### [PASS] 正确性: Config Guard (config_guard.go)

- **位置**: `cli/config_guard.go`
- **分析**: `EnsureConfigReady` 在 CLI 命令执行前检查配置文件是否就绪，引导用户执行 `setup` 初始化。`doctor`/`completion` 命令跳过此检查。
- **风险**: None

### [PASS] 正确性: Gateway RPC (gateway_rpc.go)

- **位置**: `cli/gateway_rpc.go`
- **分析**: CLI → Gateway 的 RPC 调用客户端，用于 CLI 命令远程调用正在运行的 Gateway 服务。
- **风险**: None

---

## tui — 终端用户界面

### [PASS] 架构: Bubble Tea Elm Architecture (tui.go)

- **位置**: `tui/tui.go:47-64`
- **分析**: 使用 `charmbracelet/bubbletea` (Elm Architecture: Model-Update-View) + `charmbracelet/lipgloss` 样式。支持全屏模式(`Run`)和内联模式(`RunInline`)。
- **风险**: None

### [PASS] 正确性: 完整 TUI 功能集

- **位置**: `tui/` (28 files)
- **分析**: 功能覆盖:
  - `gateway_ws.go` — WS 连接（有测试）
  - `stream_assembler.go` — 流式消息组装（有测试）
  - `view_*.go` — 聊天/消息/状态/工具/输入 5 个视图
  - `event_handlers.go` — 事件处理（有测试）
  - `formatters.go` — 输出格式化（有测试）
  - `wizard.go` — 配置向导
  - `local_shell.go` — 本地 shell 集成（有测试）
- **风险**: None

---

## tts — 语音合成

### [PASS] 正确性: 多 Provider 合成引擎 (synthesize.go)

- **位置**: `tts/synthesize.go:31-46`
- **分析**: `SynthesizeWithProvider` 根据 provider 类型路由: OpenAI → POST `/v1/audio/speech`，ElevenLabs → POST `/v1/text-to-speech/{voiceId}`，Edge → 调用 `edge-tts` CLI。每个 provider 独立实现完整的 HTTP 请求/响应处理。
- **风险**: None

### [PASS] 正确性: TTS 缓存与偏好 (cache.go, prefs.go)

- **位置**: `tts/cache.go`, `tts/prefs.go`
- **分析**: 基于文本哈希的音频缓存避免重复 API 调用。用户语音偏好配置支持按渠道自定义声音选择。
- **风险**: None

### [PASS] 正确性: 文本摘要预处理 (summarize.go)

- **位置**: `tts/summarize.go`
- **分析**: 长文本在合成前先用 LLM 摘要压缩，避免 TTS API 超长文本限制和过高费用。有测试覆盖(`summarize_test.go`)。
- **风险**: None

---

## outbound — 外发消息路由

### [PASS] 正确性: 13 渠道会话路由 (session.go)

- **位置**: `outbound/session.go:121-137, 139-538`
- **分析**: `init()` 注册 13 个渠道解析器: WhatsApp, Signal, Telegram, Discord, Slack, iMessage, Matrix, MSTeams, Mattermost, Nostr, NextcloudTalk, DingTalk, Feishu。每个解析器独立处理该渠道的 peer 类型识别(group/DM/thread)、session key 构建。正则预编译（L67-72: `uuidRE`/`hexRE` 等）。
- **风险**: None

### [PASS] 安全: 跨上下文消息策略 (policy.go)

- **位置**: `outbound/policy.go:143-203`
- **分析**: `EnforceCrossContextPolicy` 检查 within-provider 和 across-providers 两级策略。跨上下文消息可添加来源装饰标记。默认同渠道允许、跨渠道拒绝。
- **风险**: None

### [PASS] 正确性: 消息投递管道 (deliver.go, send.go)

- **位置**: `outbound/deliver.go`(10KB), `outbound/send.go`(8.5KB)
- **分析**: 投递管道: 消息接收 → 会话路由 → 跨上下文策略检查 → 渠道适配 → 发送。支持附件、贴纸、轮询、线程回复等消息类型。
- **风险**: None

---

## media — 媒体处理

### [PASS] 正确性: 媒体存储 + TTL 清理 (store.go)

- **位置**: `media/store.go:110-137`
- **分析**: `CleanOldMedia` 按 TTL 清理过期文件。UUID 嵌入文件名防止冲突。`sanitizeFilename` 过滤危险字符。最大 5MB 文件限制。
- **风险**: None

### [PASS] 正确性: 图像处理双模式 (image_ops.go)

- **位置**: `media/image_ops.go` (524L)
- **分析**: macOS 优先 sips（硬件加速），其他平台用 Go image 库。EXIF 方向处理支持 8 种方向值。HEIC→JPEG 转换。PNG 自适应压缩。
- **风险**: None

### [PASS] 正确性: STT 语音识别 (stt_openai.go, stt_local.go)

- **位置**: `media/stt_openai.go`(4.8KB), `media/stt_local.go`(2.8KB)
- **分析**: 双模式 STT: OpenAI Whisper API（远程）+ 本地 Whisper CLI。支持多音频格式。
- **风险**: None

### [PASS] 正确性: 文档转换 (docconv*.go)

- **位置**: `media/docconv.go`, `media/docconv_builtin.go`, `media/docconv_mcp.go`
- **分析**: 文档→文本转换支持两种后端: 内置(PDF/Office 基础解析)和 MCP(外部工具高质量转换)。
- **风险**: None

---

## plugins — 插件系统

### [PASS] 正确性: 完整插件生命周期 (registry.go, loader.go, install.go, update.go)

- **位置**: `plugins/` 19 files
- **分析**: 完整插件生命周期管理:
  - **发现** (`discovery.go` 13.6KB) — 扫描 npm/本地目录
  - **安装** (`install.go` 16.5KB) — npm install + 依赖解析
  - **加载** (`loader.go` 9KB) — manifest 解析 + 工厂创建
  - **注册** (`registry.go` 15KB) — 8 类注册(tools/hooks/channels/etc)
  - **更新** (`update.go` 14.7KB) — 版本比较 + 自动更新
  - **运行时** (`runtime.go`) — 插件进程管理
  - **命令** (`commands.go`) — CLI 交互
  - **配置** (`config_state.go`) — 插件配置持久化
- **风险**: None

### [PASS] 安全: 插件与核心网关隔离 (registry.go)

- **位置**: `plugins/registry.go:239-260`
- **分析**: `RegisterGatewayMethod` 不允许插件覆盖核心网关处理器（已注册的方法直接跳过）。
- **风险**: None

---

## 总结

- **总发现**: 16 (16 PASS, 0 WARN, 0 FAIL)
- **阻断问题**: 无
- **结论**: **通过** — 所有模块审查通过。TTS 多 provider 引擎、外发 13 渠道路由、插件完整生命周期均设计完善。
