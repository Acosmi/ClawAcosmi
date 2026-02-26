# 审计修复执行跟踪

> 基于 [修复计划](./post-audit-remediation-plan.md) + [深度对比](./post-audit-deep-comparison.md)
> 创建日期：2026-02-22

## W-FIX-0：Infra 隐藏依赖审计补全（前置）✅

- [x] INFRA-AUDIT：补完 `global-audit-infra.md` 隐藏依赖审计表（7 类全部完成）
- [x] INFRA-AUDIT：补完 `global-audit-infra.md` 差异清单（P0=0, P1=0新增, P2=3已跟踪, P3=25+）
- [x] 验证：确认审计评级 → **A（优秀）**

## W-FIX-1：P0 Sandbox CLI（~2h）✅

- [x] CMD-1：实现 `sandbox explore/explain` 子命令
- [x] 验证：`go build` + `go vet` + 手动测试

## W-FIX-2：P1 Media 缺陷（~4.5h）✅

- [x] MEDIA-2：`parse.go` Markdown Fenced Code 边界检测
- [x] MEDIA-1：PDF 页面渲染转图片
- [x] 验证：`go test ./internal/media/...`（14 tests PASS）

## W-FIX-3：P1 Security + TTS + OAuth（~6.5h）✅

- [x] W1-SEC1：JSONC 配置解析修复 — `security/jsonc.go`（ParseJSONC via hujson）+ 5 处调用替换 + 7 个测试
- [x] W1-TTS1：长文本 TTS LLM 智能摘要 — `tts/summarize.go`（SummarizerFunc 注入 + maybeSummarizeText 三路 fallback）+ 10 个测试
- [x] CMD-2：OAuth CLI Web Flow — `cmd/openacosmi/setup_oauth_web.go`（PKCE + state + 本地回调 + 5 provider 注册）
- [x] 验证：`go build` ✅ + `go vet` ✅ + security tests (7) ✅ + tts tests (10) ✅
- [x] 复核审计：✅ 通过 (14/14, 0 虚标, 隐形依赖 7/7 ✅)

## W-FIX-4：P1 Gateway WS + Env（~2.5h）✅

- [x] GW-3：WS Close Code `4008`→1008 对齐 + `ws_close_codes.go`（5 codes + 16 reasons）+ 3 测试
- [x] CMD-5：代码已对齐，新增 `dotenv_test.go`（4 测试）
- [x] 验证：`go build` ✅ + `go vet` ✅ + gateway tests ✅ + cli tests ✅
- [x] 复核审计：✅ 通过

## W-FIX-5：P2 TTS + Log（~3h）✅

- [x] TTS-1：`OPENAI_TTS_BASE_URL` 自定义端点（`types.go` + `config.go` + `synthesize.go`）
- [x] LOG-1：RPC-first follow 模式 + 本地 fallback（`cmd_logs.go`）
- [x] 验证：`go build` ✅ + `go vet` ✅ + tts tests ✅
- [x] 复核审计：✅ 通过

## W-FIX-6：P2 Media + Browser（~3h）✅

- [x] MEDIA-3：`SaveMediaSourceWithHeaders` + `downloadAndSave` headers 透传 + 3 测试
- [x] BRW-2：pw-ai 确认移交——ARIA 在 `pw_role_snapshot.go`，AI 视觉由 Agent Runner 承接
- [x] 验证：`go build` ✅ + `go vet` ✅ + media tests (3) ✅ + browser tests ✅
- [x] 复核审计：✅ 通过

## W-FIX-7：P2 Gemini + OpenRS（~17h，拆 2-3 窗口）✅

- [x] PHASE5-1：Gemini SSE 分块解析器 — `gemini.go`（310L）+ 7 tests PASS
- [x] PHASE5-2：`/v1/responses` 完整实现 — `openresponses_http.go`（430L）+ 17 tests PASS
- [x] 验证：`go build` ✅ + `go vet` ✅ + `go test -race` ✅（34 tests total）
- [x] 复核审计：⚠️ 有条件通过 — 3 项延迟（OR-IMAGE/OR-FILE/OR-USAGE）记入 deferred-items

## W-FIX-8：P2 TUI 攻坚（~18h 预估 → 实际 ~3h）✅

- [x] TUI-1：核心组件功能补全（FormatStatusSummary 4 段补全 + OverlaySettings 处理器）
- [x] TUI-2：`ensureExplicitGatewayAuth` 校验逻辑（URL 覆盖时强制要求 auth）
- [x] 验证：`go build` ✅ + `go vet` ✅ + `go test -race` ✅（56 tests，12 新增）
- [x] 复核审计：✅ 通过 (8/8, 0 虚标, 隐形依赖 7/7 ✅)

## 文档清理 ✅

- [x] 清理 `global-audit-gateway.md` 重复总结段（L94-99 删除）
- [x] 补完 `global-audit-autoreply.md` 逐文件对照（119 TS → 98 Go，8 组 ~100 文件映射）
- [x] 消除 TUI-1/TUI-2 ID 冲突（审计报告改为 TUI-A1/TUI-A2 + 消歧注释）

---

## W-FIX-AR：Autoreply 复核修复（~8-12h，拆 2-3 窗口）✅

> 来源：2026-02-22 /fuhe 复核审计 → [复核报告](file:///Users/fushihua/.gemini/antigravity/brain/81c97b8c-5db4-4328-9ceb-b6ad06dfd811/fuhe-audit-autoreply.md)
> 评级修正：S → B+ → **A**（P0/P1/P2/P3 全部清零）

### 窗口 1：P1 核心缺失（~5h）

- [x] **AR-3** (P1)：Bash 聊天命令系统 ✅ 确认完整（2026-02-22）
  - [x] ~~新建 `autoreply/reply/bash_command.go`~~ → 实际已在 `commands_handler_bash.go`(370L) 完整实现
  - [x] 移植 `parseBashRequest` 命令解析（help/run/poll/stop）→ `ParseBashRequest()` L45-84
  - [x] 移植 `handleBashChatCommand` 主流程（权限校验 → 前台/后台执行 → 结果格式化）→ `HandleBashCommand()` + DI 委托
  - [x] 移植 `activeJob` 状态管理（单例互斥、watcher attach、生命周期清理）→ `ActiveBashJob` + mutex L192-239
  - [x] 连接 `BashExecutor` DI 接口 → 真实实现 → 已通过 DI 接口代理
  - [x] TS 对照：`src/auto-reply/reply/bash-command.ts` (426L) → Go 覆盖 12/12 功能

- [x] **AR-4** (P1)：模型指令处理 & 选择器 ✅ 完成（2026-02-22）
  - [x] 新建 `reply/directive_handling_model.go`（310L）
  - [x] 移植 `maybeHandleModelDirectiveInfo`（status/summary/list 三模式）
  - [x] 移植 `resolveModelSelectionFromDirective`（显式/模糊/numeric 拒绝）
  - [x] 移植 `buildModelPickerCatalog`（config 解析 + allowlist 合并）
  - [x] 新建 `reply/directive_model_picker.go`（130L）
  - [x] 移植 `buildModelPickerItems`（去重 + provider 优先级排序）
  - [x] 移植 `resolveProviderEndpointLabel`（baseUrl/api 展示）
  - [x] TS 对照：`reply/directive-handling.model.ts` (403L) + `model-picker.ts` (98L)
  - [x] 验证：`go build` ✅ + `go vet` ✅ + `go test -race` ✅

### 窗口 2：P2 补全（~3h）✅

- [x] **AR-5** (P2)：队列指令验证 ✅ 完成（2026-02-22）
  - [x] 新建 `reply/directive_queue_validation.go`（105L）
  - [x] 移植 `maybeHandleQueueDirective`（status 展示 + mode/debounce/cap/drop 合法性校验）
  - [x] TS 对照：`reply/directive-handling.queue-validation.ts` (79L) → 100% 分支覆盖
  - [x] 验证：10 tests PASS

- [x] **AR-6** (P2)：dispatch_from_config 补全 ✅ 完成（2026-02-22）
  - [x] 补全 `reply/dispatch_from_config.go` 至 ≥80% 覆盖（149L→400L）
  - [x] 添加 `resolveSessionTtsAuto`（TTS auto 模式解析）
  - [x] 添加 `recordProcessed` 诊断记录
  - [x] 添加 `markProcessing/markIdle` 状态跟踪
  - [x] 添加 `sendPayloadAsync`（含 mirror 路由逻辑）
  - [x] 添加 `onBlockReply` 回调机制 + block text 累积 TTS 合成
  - [x] 添加 fast abort 检测 + hook runner fire-and-forget
  - [x] TS 对照：`reply/dispatch-from-config.ts` (459L) → ≥80% 覆盖
  - [x] 验证：`go build` ✅ + `go vet` ✅ + `go test -race` ✅（209 tests）

### 窗口 3：P3 收尾 + 验证（~2h）✅

- [x] **AR-7** (P3)：response_prefix 缺失函数 ✅ 完成（2026-02-22）
  - [x] 添加 `ExtractShortModelName`（provider 前缀剥离 + 日期/latest 后缀去除）
  - [x] 添加 `HasTemplateVariables`（模板变量检测正则）
  - [x] 更新 `ResponsePrefixContext` 添加 `ModelFull`/`ThinkingLevel`/`IdentityName`
  - [x] 升级 `applyResponsePrefixTemplate` 支持 TS 单花括号 `{var}` 语法
  - [x] TS 对照：`reply/response-prefix-template.ts` (102L) → 3/3 函数完全覆盖

- [x] **AR-8** (P3)：TODO 桩填充 ✅ 完成（2026-02-22）
  - [x] `reply/get_reply_inline_actions.go` L67 — Phase 9 D5 注释移除（函数已完整）
  - [x] `reply/get_reply_directives_apply.go` L56 — Phase 9 D5 注释移除（函数已完整）
  - [x] `reply/get_reply_directives.go` L65 — TODO(集成) 移除 + `Channel` 字段补全
  - [x] `reply/followup_runner.go` L92 — HEALTH-D5 已完成（文档注释，非 TODO）

- [x] 全量验证
  - [x] `go build ./internal/autoreply/...` ✅
  - [x] `go vet ./internal/autoreply/...` ✅
  - [x] `go test -race ./internal/autoreply/...` ✅（2 packages PASS）
  - [x] /fuhe 复核：✅ 通过 (8/8, 0 虚标, 隐形依赖 7/7 ✅)，评级 B+ → **A**
