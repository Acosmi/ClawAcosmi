# 延迟项清除 — 任务清单

> 最后更新：2026-02-18（深度审计完毕 ✅ — 24 项归档，7 差异项留延迟待办）
> 来源：`deferred-items.md` 剩余 26 项
> 策略：24 项纳入清除，2 项（Ollama/i18n）推迟 Phase 12
> 每窗口 ≤ 5 文件，按依赖序执行

---

## 范围确认

| 决策 | 结论 |
|------|------|
| Phase 12 推迟 | 仅 P11-1 Ollama + P11-2 i18n（2 项） |
| Channel Dock 策略 | 建运行时注册表骨架（一劳永逸） |
| 清除目标 | **24 项**（P1×2 + P2×14 + P3×5 + Setup Wizard） |

---

## Window 1: P1 立即可做（2 项）

> 预估：~5 文件改动，无外部依赖

- [x] **H1-1**: `internal/sessions/` 类型与函数去重 (P11-B5 / P1)
  - 删除 `sessions.go` (199L) + `sessions_test.go` (133L) — 全部重复代码
  - 参考: [phase11-c4-sessions-dedup-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-c4-sessions-dedup-bootstrap.md)
- [x] **H1-2**: `ResolveDefaultModelForAgent` stub 填充 (P11-D-P2-6 / P2)
  - 实现 agent 级别模型覆盖: `resolveAgentModelPrimary` + `shallowOverrideModelPrimary`
  - 优先级链: `agent.model.primary → global defaults.model.primary → DefaultModel`
- [x] 验证: `go build && go vet && go test -race` — sessions/models/scope/gateway 全部通过

---

## Window 2: AutoReply 补强（2 项）

> 预估：~3 文件改动

- [x] **H2-1**: `command-lane` 清理缺失 (P11-C-EXTRA-2 / P2)
  - 新建 `queue_command_lane.go` 实现 command-lane 队列系统
  - `queue_cleanup.go` 集成 `ClearCommandLane(resolveEmbeddedSessionLane(cleaned))`
- [x] **H2-2**: `directive.ts` tokenizer 架构对齐 (P11-C-EXTRA-4 / P2)
  - 重写 `queue_directive.go` 从固定正则→tokenizer
  - 支持复合时长 (`1h30m`)、`consumed` 偏移量精确切割、无法识别 token 停止解析
- [x] 验证: `go build && go vet && go test -race ./internal/autoreply/...` — 88 项测试全通过

---

## Window 3: Agent Runner 补强（6 项）✅

> 预估：~5 文件改动，单项均为小改动

- [x] **H3-1**: G3 session marker 持久化 (P11-D-P2-1 / P2)
  - SessionManager 增加 `AppendCustomEntry` + `LoadAllEntries` + `readAllEntriesFromFile`
- [x] **H3-2**: G5 `compactionFailureEmitter` (P11-D-P2-2 / P2)
  - Go `sync.Mutex` + callback slice + defer/recover 替代 Node.js EventEmitter
- [x] **H3-3**: S6 `rawStream` 调试日志 (P11-D-P2-3 / P2)
  - `OPENACOSMI_RAW_STREAM` 环境变量控制 JSONL 调试输出
- [x] **H3-4**: S10 `promoteThinkingTagsToBlocks` (P11-D-P2-4 / P2)
  - `<thinking>/<think>/<thought>/<antthinking>` XML→结构化 content blocks
- [x] **H3-5**: `normalizeToolParameters` (P11-D-P2-5 / P2)
  - anyOf/oneOf union 扁平化 + 强制 type:"object" + Gemini 清洗
- [x] **H3-6**: `loadWebMedia` 图片优化 (P11-D-P3-1 / P3)
  - JPEG 压缩 + 多分辨率×质量网格（HEIC 标记 RUST_CANDIDATE: P2）
- [x] 验证: `go build && go vet && go test -race` — runner/session/llmclient 全通过

---

## Window 4: Channel Dock 骨架 + 联动（4 项）✅

> 预估：~5 文件改动，含 1 个新建前置依赖
> ⚠️ 注意：`channels/*` 子包导入 `autoreply`，因此 `autoreply` 不能直接导入 `channels`。
> 采用 DI 注入模式（`NativeCommandSurfaceProvider`/`PluginDebounceProvider`/`BlockStreamingCoalesceDefaultsProvider`），由 gateway 启动注入。

- [x] **前置**: 建立 channel dock 运行时注册表骨架
  - `dock.go` 增加 `DockQueueDefaults` 结构体 + `GetPluginDebounce` + `GetBlockStreamingCoalesceDefaults` + `ListNativeCommandChannels`
  - 新建 `dock_test.go` (10 测试)
- [x] **H4-1**: `IsNativeCommandSurface` 动态化 (P11-C-EXTRA-1 / P1)
  - 硬编码 map → `NativeCommandSurfaceProvider` DI 注入（gateway 启动注入 `channels.ListNativeCommandChannels`）
- [x] **H4-2**: `channel plugin debounce` 解析 (P11-C-EXTRA-3 / P2)
  - `queue_settings.go` 增加 `PluginDebounceProvider` + `resolvePluginDebounce` 调用
- [x] **H4-3**: `block-streaming` channel dock 联动 (P11-C-EXTRA-5 / P2)
  - `block_streaming.go` 增加 `BlockStreamingCoalesceDefaultsProvider` + `ResolveBlockStreamingChunkingWithDock`
- [x] 验证: `go build && go vet && go test -race ./internal/channels/... ./internal/autoreply/...` — 全绿

---

## Window 5: Channel Dock 消费者 + Session 合并管线（4 项）✅

> 预估：~5 文件改动，依赖 Window 4

- [x] **H5-1**: `reply-elevated` 提权管线 (P11-C-P2 延迟)
  - `reply_elevated.go` (309L)：双层权限检查 + DI 注入打破导入环
  - `reply_elevated_test.go` (11 tests)
- [x] **H5-2**: LINE 指令 + 线程回复 (P11-C-P2 延迟)
  - `channels/line/bot_message_context.go`（骨架）+ test (3 tests)
- [x] **H5-3**: `updateLastRoute` 3 层合并管线 (G-AUDIT-4 / P3)
  - `delivery_context.go`：normalize/merge/removeThread/computeDeliveryFields
  - 重写 `UpdateLastRoute` 为 `UpdateLastRouteParams` + 3 层合并
  - `delivery_context_test.go` (15 tests)
- [x] **H5-4**: `recordSessionMeta` 推导链 (G-AUDIT-5 / P3)
  - `session_metadata.go`：mergeOrigin/deriveSessionOrigin/deriveSessionMetaPatch
  - `session_metadata_test.go` (12 tests)
- [x] 验证: `go build && go vet && go test -race` — autoreply/gateway/channels 全通过

---

## Window 6: WS 设备配对管理（1 大项）✅

> 预估：~2 文件新建，TS 移植 ~500L

- [x] **H6-1**: 设备配对管理 (P11-E-P2-3 / P2)
  - TS `device-pairing.ts` (~559L) 完整移植:
    - `requestDevicePairing` / `approveDevicePairing` / `getPairedDevice`
    - 设备 token 轮换 / 持久化存储
    - `verifyDeviceToken` / `ensureDeviceToken` / `rotateDeviceToken` / `revokeDeviceToken`
  - 新建 `internal/gateway/device_pairing.go` (~500L)
  - 新建 `internal/gateway/device_pairing_test.go` (15 tests)
- [x] 验证: `go build && go vet && go test -race ./internal/gateway/...` — 全绿

---

## Window 7: Config 校验补全（3 项） ✅

> 预估：~3 文件改动

- [x] **H7-1**: `schema.go` UI Hints 补全 (P11-F-P2-1 / P2)
  - 补全 290 labels + 226 help + 8 placeholders + sensitive 自动标记
- [x] **H7-2**: Zod Schema 深层校验 (P11-F-P2-2 / P2)
  - 扩展 `validator.go` 覆盖 9 个枚举/范围约束
- [x] **H7-3**: `validation.ts` 语义验证 (P11-F-P2-3 / P2)
  - avatar 路径、heartbeat target 格式、agent 目录去重校验
- [x] 验证: `go build && go vet && go test -race ./internal/config/... ./pkg/types/...`

---

## Window 8: Maintenance 定时器 + WS 日志（4 项）

> 预估：~4 文件改动，需 `chatRunState` 基础设施

- [x] **H8-1**: maintenance — health 刷新定时器 (G-AUDIT-1 / P3)
  - 60s 周期 `refreshGatewayHealthSnapshot({ probe: true })`
  - 依赖: `HealthSummary` 类型
- [x] **H8-2**: maintenance — chatAbort 超时清理 (G-AUDIT-2 / P3)
  - 60s 周期检查 `chatAbortControllers` 过期条目
  - 依赖: `ChatAbortControllerEntry` + `abortChatRunById()`
- [x] **H8-3**: maintenance — abortedRuns TTL 清理 (G-AUDIT-3 / P3)
  - 清理 1h 过期的 `abortedRuns` + `chatRunBuffers` + `chatDeltaSentAt`
  - 依赖: `chatRunState` 基础设施
- [x] **H8-4**: WS 日志 summarize + redact (G-AUDIT-6 / P3)
  - `summarizeAgentEventForWsLog()` + `redactSensitiveText()`
- [x] 验证: `go build && go vet && go test -race ./internal/gateway/...`

---

## Window 9: 初始化向导 Setup Wizard（1 大项跨前后端）

> 预估：多步骤实现，可能需要 2~3 窗口

- [x] **H9-1**: Setup Wizard 后端 (P11-3 / P1) ✅
  - 新建 `wizard_session.go` (310L) — goroutine+channel 替代 TS Deferred/Promise
  - 新建 `wizard_onboarding.go` (280L) — Provider → API Key → Model 三步引导
  - 新建 `server_methods_wizard.go` (170L) — 4 RPC handlers
  - 新建 `wizard_session_test.go` (320L) — 13 测试 (`-race`)
  - 修改 `server_methods_stubs.go` — 移除 wizard.* stubs
  - 修改 `server.go` — 注册真实 wizard handlers
- [x] **H9-2**: Setup Wizard 前端 ✅
  - 新建 `views/wizard.ts` (300L) — 苹果风全屏叠层 UI（4 种步骤类型）
  - 新建 `styles/wizard.css` (260L) — dark/light 双主题、glassmorphism、动画
  - 修改 `app-view-state.ts` — wizardOpen/wizardState 类型
  - 修改 `app.ts` — 状态声明 + handler
  - 修改 `app-render.ts` — renderWizard 集成
  - 修改 `styles.css` — CSS import
- [x] 验证 (H9-1): `go build && go vet && go test -race` — 13 tests PASS
- [x] 验证 (H9-2): `tsc --noEmit` — 零新增错误 ✅

---

## Phase 12 推迟（2 项）

| 项 | 理由 |
|----|------|
| P11-1 Ollama 本地 LLM 集成 | 独立功能，不影响核心 |
| P11-2 前端 i18n 全量抽取 (275 key) | 纯前端，工作量大 |

---

## 执行序与依赖关系

```
W1 (P1 即刻) ─→ W2 (AutoReply) ─→ W3 (Agent Runner)
                                         │
W4 (Dock 骨架) ─→ W5 (Dock 消费者) ──────┘
                                         │
W6 (设备配对) ─→ W7 (Config) ────────────┘
                                         │
W8 (Maintenance) ─→ W9 (Wizard) ────────┘
```

W1~W3 可并行于 W4~W5。W6~W9 各自独立。

## 统计

| 窗口 | 项数 | 预估文件 | 预估行数 |
|------|------|----------|----------|
| W1 | 2 | ~5 | ~300L |
| W2 | 2 | ~3 | ~300L |
| W3 | 6 | ~5 | ~400L |
| W4 | 4 | ~5 | ~500L |
| W5 | 4 | ~5 | ~500L |
| W6 | 1 | ~2 | ~500L |
| W7 | 3 | ~3 | ~400L |
| W8 | 4 | ~4 | ~400L |
| W9 | 2 | ~5+ | ~600L+ |
| **合计** | **28 子项** | **~37 文件** | **~3,900L** |
