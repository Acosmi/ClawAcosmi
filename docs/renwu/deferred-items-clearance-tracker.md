# 延迟项清除 — 实施跟踪文档

> 创建时间: 2026-02-23 05:50
> 来源: `docs/renwu/deferred-items-clearance-plan.md`
> 联网验证: ✅ 已对照 VS Code / Gravitee / TailScale / Playwright / Mozilla any-llm-go 等 10 项可信源

---

## 验证摘要（联网可信源）

| 设计决策 | 行业对照 | 验证结论 |
|---------|---------|---------|
| 权限弹窗 3 层审批 | VS Code Extension Security (单次/会话/永久) | ✅ 完全对齐 |
| 渐进式向导配置 | Gravitee API Gateway Progressive Disclosure | ✅ 行业标准 |
| LLM 多 provider 统一接口 | Mozilla any-llm-go 规范化模式 | ✅ 最佳实践 |
| Windows PID + CreationTime | Microsoft GetProcessTimes API / MSDN | ✅ 唯一可靠方案 |
| mDNS 双模式 | grandcat/zeroconf (LAN) + TailScale MagicDNS | ✅ 工业实践 |
| 纯 Go 剪贴板 | aymanbagabas/go-nativeclipboard (无 Cgo) | ✅ 跨平台最优 |
| AI 浏览自动化 | Playwright MCP + Google DeepMind Mariner | ✅ 前沿方向 |

---

## 统计概览

| Sprint | 项数 | 状态 | 完成率 |
|--------|------|------|--------|
| S1 — P2 核心流程 | 4 项 | ✅ 已完成 | 4/4 |
| S2 — P2 体验 + P3 基础 | 8 项 | ✅ 已完成（审计通过） | 8/8 |
| S3 — P3 长尾补全 | 13 项 | ✅ 已完成 | 13/13 |
| S4 — Signal 频道 Plugin 层补全 | 4 项 | ✅ 已完成（代码验证确认） | 4/4 |
| S5 — shenji-016 审计后修复 + shenji-019 复核 | 4+1 项 | ✅ 已完成 | 5/5 |
| S6 — P3 延迟项清除（9→2） | 7 项 | ✅ 已完成 | 7/7 |
| S7 — P3 最终清零（2→0） | 2 项 | ✅ 已完成 | 2/2 |
| **合计** | **43 项** | **✅ 全部完成** | **43/43** |

---

## Sprint 1 — P2 核心流程修复（~6h）

- [x] **S1-1** PERM-POPUP-D1 — 权限弹窗 WebSocket 广播 (8 文件: `types.go`/`run_attempt.go`/`run.go`/`attempt_runner.go`/`model_fallback_executor.go`/`server.go`/`tool_executor.go`/`app-gateway.ts`)
- [x] **S1-2** GW-WIZARD-D2 — 简化向导完成后提供高级配置入口 (`wizard_onboarding.go`)
- [x] **S1-3** GW-WIZARD-D1 — Google provider OAuth 模式选择 + 引导 (`wizard_onboarding.go`)
- [x] **S1-4** GW-LLM-D1 — Gemini functionResponse Name 字段修复 + 测试 (`attempt_runner.go`/`gemini.go`/`client_test.go`/`gemini_test.go`)

---

## Sprint 2 — P2 体验优化 + P3 基础设施（~12h）

- [x] **S2-1** GW-TOKEN-D1 — Gateway 首次启动自动生成 token (已由 `auth.go` `ReadOrGenerateGatewayToken()` 实现)
- [x] **S2-2** GW-UI-D1 — 前端 chat 失败显示错误 toast (`error-toast.ts`/`.css` + `chat.ts` + i18n)
- [x] **S2-3** SANDBOX-D1 — recreate 过滤 browser 容器 (`cmd_sandbox.go`)
- [x] **S2-4** SANDBOX-D2 — explain 支持 session store 查询 (`cmd_sandbox.go` + `sessions.SessionStore`)
- [x] **S2-5** W5-D1 — Windows 进程检测优化 (`gateway_lock_windows.go` — OpenProcess API)
- [x] **S2-6** HIDDEN-4 — npm 黑盒行为等价: iso639 映射表 (`iso639.go`)
- [x] **S2-7** HIDDEN-8 — 平台特定隐藏依赖: clipboard / brew / wsl (`platform_*.go`)
- [x] **S2-8** W6-D1 — TUI 渲染主题色彩微调 (`theme.go` — 补充 6 个 chroma token)

---

## Sprint 3 — P3 长尾补全（~30h）

- [x] **S3-1** PHASE5-3 — TailScale + mDNS 集成 (`server_tailscale.go` 增强 + `bonjour_zeroconf.go` + i18n) ✅ 2026-02-23
- [x] **S3-2** PHASE5-4 — infra 剩余缺失文件 批次 A (`internal/infra/` 8 files) ✅ 2026-02-23
- [x] **S3-3** PHASE5-4 — infra 剩余缺失文件 批次 B (`internal/infra/` 7 files) ✅ 2026-02-23
- [x] **S3-4** PHASE5-4 — infra 剩余缺失文件 批次 C (`internal/infra/` 12 files) ✅ 2026-02-23
- [x] **S3-5** PHASE5-5 — Playwright AI 浏览自动化 窗口 1 (`pw_driver.go`+`pw_playwright.go`+`pw_playwright_browser.go`) ✅ 2026-02-23
- [x] **S3-6** PHASE5-5 — Playwright AI 浏览自动化 窗口 2 (`pw_ai_loop.go`+`pw_ai_vision.go`+i18n) ✅ 2026-02-23
- [x] **S3-7** PHASE5-5 — Playwright AI 浏览自动化 窗口 3 (`pw_driver.go` 统一接口 + tests) ✅ 2026-02-23
- [x] **S3-8** OR-IMAGE — OpenResponses 图像输入提取 (`openresponses_http.go`) ✅ 2026-02-23
- [x] **S3-9** OR-FILE — OpenResponses PDF/文件输入 (`openresponses_http.go`) ✅ 2026-02-23
- [x] **S3-10** OR-USAGE — OpenResponses usage 聚合 (`openresponses_http.go`) ✅ 2026-02-23
- [x] **S3-11** HEALTH-D4 — 图片工具缩放和转换 (`image_tool.go`) ✅ 2026-02-23
- [x] **S3-12** HEALTH-D6 — LINE 渠道 SDK 决策 (文档标记，无代码改动) ✅ 2026-02-23
- [x] **S3-13** GW-UI-D3 — Vite 代理 ECONNREFUSED 静默 (`vite.config.ts`) ✅ 2026-02-23

---

## Sprint 5 — shenji-016 审计后修复 + shenji-019 复核审计 ✅

> **来源**: 2026-02-25 shenji-016 复核审计 → deferred-items.md 4 项 P2/P3 修复
> **审计报告**: `docs/claude/goujia/shenji-019-4item-fix-fuhe-audit.md`
> **窗口内容**: 4 项代码修复 + 1 项审计后追加修复 + 复核审计报告 + 文档统计修正

### 代码修复（4 项）

- [x] **S5-1** GW-LLM-D1 — Anthropic `tool_result` content 字段始终包含（`anthropic.go:257`）+ 2 个测试
  - **修复**: 从 `if b.ResultText != "" { block["content"] = b.ResultText }` 改为始终 `block["content"] = b.ResultText`
  - **测试**: `TestToAnthropicMessages_ToolResultEmptyContent` + `TestToAnthropicMessages_ToolResultWithContent` PASS
  - **Gemini 验证**: `gemini.go:203-217` 格式正确无需修改

- [x] **S5-2** GW-WIZARD-D2 — 简化向导完成后跳转高级配置 Phase 8（`wizard_onboarding.go`）
  - **修复**: `RunOnboardingWizardAdvanced(deps, startFromPhase ...int)` 可变参数 + Phase 1-7 `skipTo <=` guard
  - **向后兼容**: 无参数调用时 `skipTo=1`，行为不变

- [x] **S5-3** DISCORD-GAP-1 — `SyncDiscordSlashCommands` 增量同步替代 `RegisterDiscordSlashCommands`（`monitor_native_command.go:250-343`）
  - **修复**: create/update/delete diff 逻辑 + `commandNeedsUpdate` 比较函数
  - **调用点**: `monitor_provider.go:217` 替换完成

- [x] **S5-4** PERM-POPUP-D1 — RPC 参数名修正 `approved` → `approve`（`app-render.ts:1141-1177`）
  - **修复**: 4 个回调（onAllowOnce/Session/Permanent/Deny）的 RPC 参数 key 修正
  - **端到端验证**: 前端 `approve` ↔ 后端 `ctx.Params["approve"]`（`server_methods_escalation.go:87`）完全对齐

### 审计后追加修复（1 项）

- [x] **S5-5** DISCORD-GAP-1 补强 — `commandNeedsUpdate` 递归嵌套 sub-option 比较（`monitor_native_command.go:313-359`）
  - **修复**: 拆分为 `commandNeedsUpdate` → `optionsNeedUpdate` → `optionNeedsUpdate` 三层递归；新增 `Description`/`Autocomplete` 字段比较
  - **测试**: 10 个新增测试全部 PASS（含嵌套 SubCommand/SubCommandGroup 场景）

### 复核审计

- [x] **shenji-019** 交叉代码层颗粒度复核审计
  - 4/4 项修复验证通过
  - 发现 deferred-items.md P3 合计 13→11 错误（TS-MIG-CH1/CH4/CH6 剩余缺失项均已解决）
  - 已修正文档统计

### 文档更新

- [x] `deferred-items.md`: 4 项标记 ✅ + P3 合计 13→11 + CH1/CH4/CH6 标记 ✅ + 归档行更新
- [x] `shenji-019-4item-fix-fuhe-audit.md`: 完整复核审计报告

---

## Sprint 4 — Signal 频道 Channel Plugin 层补全 ✅

> **来源**: 2026-02-24 Signal 模块审计
> **背景**: Signal 核心运行时（daemon/client/SSE/monitor/send/event_handler）已 100% 迁移至 Go，但 channel plugin 抽象层存在 3 处缺口。Go 端当前通过 `event_handler.go` 内联直调 send 模块绕过了 plugin 层，功能可用但缺少标准化接口。
> **触发时机**: 实现 Gateway 统一 `channels.send` RPC 或跨频道转发功能时需补全。
> **2026-02-25 复核确认**: 4 项全部已在后续开发中完成实现。Gateway 采用统一 `send` RPC + `OutboundPipeline` 接口架构（非 `channels.send`），Signal 已完整接入。

- [x] **S4-1** SIG-ACTIONS — `signal_actions.go` react 桩代码补全为真实调用 ✅ **已完成**
  - **实际**: `bridge/signal_actions.go` (110L) — sendMessage + react 两个 action 完整实现
  - sendMessage: 调用 `signal.SendMessageSignal()` 含 recipient/text/mediaUrl 参数，返回 messageId/timestamp
  - react: 含 `actionGate("reactions")` 权限检查、`targetAuthor` 群组支持、add/remove 分支调用 `SendReactionSignal()`/`RemoveReactionSignal()`

- [x] **S4-2** SIG-OUTBOUND — Signal outbound 适配器 ✅ **已完成**
  - **实际**: Gateway 采用统一 `send` RPC（`server_methods_send.go`）+ `OutboundPipeline` 接口架构
  - `outbound/session.go:123` 注册 `resolveSignalSession` — Signal 会话路由（group/direct/username）完整
  - `outbound/deliver.go` 统一投递管线 — `ChannelOutboundAdapter` 接口支持 per-channel 格式化 + 媒体大小限制
  - 架构说明：未使用 `channels.send` RPC 命名，而是通过 `send` RPC + OutboundPipeline 实现等价统一发送

- [x] **S4-3** SIG-NORMALIZE — Signal normalize 适配器 ✅ **已完成**
  - **实际**: `channels/normalize.go:247-311` 统一入口
  - `NormalizeSignalMessagingTarget()` — 处理 E.164 电话号码、UUID（含 `uuid:` 前缀）、群组 `group:xxx`、用户名 `username:`/`u:` 全部格式
  - `LooksLikeSignalTargetID()` — 正则校验所有 Signal 目标格式

- [x] **S4-4** SIG-TESTS — Signal Go 端单元测试 ✅ **已完成**
  - **实际**: **7 个** `_test.go` 文件（非原来的 0 个）
  - `send_test.go` (172L) — 目标解析、文本样式
  - `send_reactions_test.go` (151L) — 反应发送/移除
  - `format_test.go` — UTF-16 偏移计算
  - `identity_test.go` — allowlist 匹配
  - `accounts_test.go` — 账户管理
  - `daemon_test.go` — 守护进程管理
  - `reaction_level_test.go` — 反应级别检查

---

## Sprint 6 — P3 延迟项清除（9→2） ✅

> **来源**: 2026-02-25 P3 清除计划
> **目标**: 清除 deferred-items.md 剩余 9 项 P3，最终保留 2 项架构性阻塞
> **结果**: 7 项完成（代码+文档），2 项标注架构阻塞保留

### 代码实现（4 项）

- [x] **S6-1** HIDDEN-4 — go-readability 集成（`codeberg.org/readeck/go-readability/v2`）
  - `web_fetch.go` 新增 `htmlToReadableMarkdown()` + markdown 分支改用新函数
  - `web_fetch_test.go` 新增 3 个测试，全部通过

- [x] **S6-2** TS-MIG-MISC6 — poll 验证函数移植
  - `backend/internal/cron/poll_normalize.go`: `NormalizePollInput()` + `NormalizePollDurationHours()`
  - `backend/internal/cron/poll_normalize_test.go`: 5 个测试（dedup/truncate/trim/empty/clamp）

- [x] **S6-3** TS-MIG-CLI2 — CLI profile 验证补全
  - `backend/internal/cli/utils.go`: `IsValidProfileName()` 正则校验 + `ResolveProfile()` 返回 error
  - `backend/internal/cli/cli_test.go`: 5 个新增测试

- [x] **S6-4** TS-MIG-MISC2 — macOS 平台功能
  - `backend/internal/daemon/platform_darwin.go`: `CheckAccessibilityPermission()` osascript 探测
  - `backend/internal/daemon/notify_darwin.go`: `SendNotification()` osascript display notification

### 文档标记（3 项）

- [x] **S6-5** TS-MIG-MISC3 — 设备配对 TS 侧标记废弃（Go 已完整实现，TS 侧可在退役时移除）
- [x] **S6-6** TS-MIG-CLI3 — CLI wizard 架构重设计确认（CLI setup + Gateway HTTP wizard 双路径覆盖）
- [x] **S6-7** TS-MIG-CH3 — Telegram 渠道补全标记 + `send_media_group.go` 媒体组发送

### 架构阻塞更新（2 项 — 保留 P3）

- [x] TS-MIG-CH5 — WhatsApp 标注架构性阻塞（需 whatsmeow 或 Node bridge 独立方案设计）
- [x] TS-MIG-CLI1 — CLI 标注已决定 Rust CLI 替代（Go CLI 不再追加迁移）

---

## Sprint 7 — P3 最终清零（2→0） ✅

> **来源**: 2026-02-25 用户确认
> **目标**: 将剩余 2 项 P3 关闭，延迟项全部清零
> **结果**: CLI1 确认已完成 + CH5 用户决定不修复

### 审计验证后关闭（2 项）

- [x] **S7-1** TS-MIG-CLI1 — Rust CLI 替代已完成 ✅
  - 审计确认：`cli-rust/` 31 crate、57,474 行 Rust、1,305 测试全过、67+ 子命令
  - Go CLI 已 DEPRECATED + CODEOWNERS 保护，ADR-001 已采纳

- [x] **S7-2** TS-MIG-CH5 — WhatsApp 不修复 ❌
  - 审计确认：架构性阻塞（go.mod 无 WhatsApp 协议库、login_qr.go 仍为占位）
  - 用户决定不追加迁移，关闭此项

---

## 状态图例

| 符号 | 含义 |
|------|------|
| ⚪ | 未开始 |
| 🔵 | 进行中 |
| ✅ | 已完成（含审计通过） |
| ❌ | 阻塞 / 需返工 |

---

## 变更日志

| 日期 | 操作 | 详情 |
|------|------|------|
| 2026-02-23 | 创建 | 25 项跟踪表初始化，全部 ⚪ 未开始 |
| 2026-02-23 | Sprint 2 完成 | 8 项全部完成 (S2-1~S2-8)，go build/vet + tsc 通过 |
| 2026-02-23 | Sprint 3 批次 1 完成 | 6 项完成 (S3-8~S3-13)，OR 多模态 + 图片工具 + Vite fix，12 新测试全部通过 |
| 2026-02-23 | S3-1 完成 | TailScale CLI 增强(多策略发现/whois/sudo)+mDNS zeroconf 注册器+i18n(en/zh 176 keys)+12 新测试通过 |
| 2026-02-23 | S3-2/3/4 完成 | infra 缺失文件补全：27 新 Go 文件 + 3 测试文件(47+ tests)，覆盖率 57%→77%，go build/vet/test-race 全通过 |
| 2026-02-23 | S3-5/6/7 完成 | Playwright AI 浏览自动化：7 新 Go 文件(857L) + 2 测试文件(210L, 10 tests) + 2 i18n 语言包(48 keys)，双驱动架构(CDP+Playwright) + AI 视觉循环(Mariner 风格)，go build/vet/test-race 全通过 |
| 2026-02-24 | Sprint 4 新增 | Signal 模块审计后新增 4 项：actions 桩代码补全、outbound/normalize 适配器、单元测试。核心运行时已 100% 迁移，plugin 层缺口记入延迟项 |
| 2026-02-25 | Sprint 5 完成 | shenji-016 审计后 4 项修复（GW-LLM-D1 + GW-WIZARD-D2 + DISCORD-GAP-1 + PERM-POPUP-D1）+ shenji-019 复核审计 + commandNeedsUpdate 递归比较补强 + 文档 P3 统计 13→11 修正 |
| 2026-02-25 | Sprint 4 确认完成 | 代码验证确认 4 项已在后续开发中全部完成：SIG-ACTIONS 完整实现（非桩代码）、SIG-OUTBOUND 通过 OutboundPipeline 统一架构、SIG-NORMALIZE 统一入口函数、SIG-TESTS 7 个测试文件。**清除跟踪 34/34 全部完成** |
| 2026-02-25 | Sprint 6 完成 | P3 延迟项清除：4 项代码实现（go-readability + poll normalize + profile 验证 + macOS 平台）+ 3 项文档标记（MISC3 + CLI3 + CH3）+ 2 项架构阻塞更新（CH5 + CLI1）。**P3 待办 9→2，清除跟踪 41/41** |
| 2026-02-25 | Sprint 7 完成 | P3 最终清零：CLI1(Rust CLI) 经审计确认已完成 ✅ + CH5(WhatsApp) 用户决定不修复 ❌。**P3 待办 2→0，清除跟踪 43/43，延迟项全部清零 🎉** |
