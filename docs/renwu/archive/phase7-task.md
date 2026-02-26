# Phase 7 任务清单 — 辅助模块

> 上下文：[phase7-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase7-bootstrap.md)
> 审计参考：[phase4-9-deep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase4-9-deep-audit.md)
> 延迟项：[deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)
> 最后更新：2026-02-15 (Pre-Phase 9 WC/WD: P7B-1/2/3 延迟项全部完成)

---

## Batch A：叶子模块（无外部依赖） ✅

### A1: 7.7 链接理解 + Markdown（~1,729L → `internal/linkparse/` + `pkg/markdown/`）

- [x] `pkg/markdown/ir.go` — Markdown→IR 转换（Go 原生解析器）
- [x] `pkg/markdown/render.go` — IR→频道格式渲染（LIFO 标记器）
- [x] `pkg/markdown/code_spans.go` + `fences.go` + `frontmatter.go`（含 yaml.v3）
- [x] `internal/linkparse/detect.go` + `defaults.go` + `format.go`
- [x] 编译验证 + 9 tests PASS
- ⛔ **延迟**: `runner.go` + `apply.go` → Batch D（依赖 auto-reply/media-understanding）
- ⛔ **延迟**: SLK-P7-A: chunkMarkdownIR → Batch D（依赖 auto-reply/chunk）

### A2: 7.3 安全模块（~4,028L → `internal/security/` 扩展）

- [x] `security/audit_fs.go` — 文件系统权限审计
- [x] `security/skill_scanner.go` — 技能文件安全扫描（正则规则引擎）
- [x] `security/channel_metadata.go` + `windows_acl.go`
- [x] 编译验证 + 17 tests PASS
- ⛔ **延迟**: `audit.go` + `audit_extra.go` + `fix.go` → Phase 8+（依赖 config/agents/channels/hooks/plugins）

---

## Batch B：独立模块（轻依赖） ✅

### B1: 7.6 TTS（1,579L 单文件 → `internal/tts/` 8 文件 1,606L）

- [x] `tts/types.go` — 全部类型定义 + 常量 + 枚举（TtsAutoMode/Provider/OutputFormat 等）
- [x] `tts/config.go` — 配置解析（`ResolveTtsConfig` + nil-safe 访问器 + 模型覆盖策略）
- [x] `tts/prefs.go` — 用户偏好读写（原子文件操作 + 自动模式解析）
- [x] `tts/provider.go` — Provider 路由 + API key 解析（config/env 多源）+ 输出格式
- [x] `tts/directives.go` — `[[tts:...]]` 指令解析（provider/voice/model/seed 参数）
- [x] `tts/synthesize.go` — 三 provider 合成引擎（OpenAI/ElevenLabs/Edge — Pre-Phase 9 WD 完整 HTTP 实现）
- [x] `tts/cache.go` — 内存音频缓存 + 临时文件管理 + 延迟清理
- [x] `tts/tts.go` — 主入口（`SynthesizeTts` provider 回退 + `ApplyTts` 完整流程）
- [x] 编译验证：`go build ./internal/tts/...` + `go vet` ✅
- ~~⛔ **延迟**: Provider HTTP 调用实现 → Phase 8 集成~~ → ✅ Pre-Phase 9 WD 完成 (P7B-1)

### B2: 7.5 媒体工具（~2,000L → `internal/media/` 11 文件 1,858L）

- [x] `media/constants.go` — 大小限制 + MediaKind 枚举
- [x] `media/mime.go` — MIME 检测（Go stdlib 替代 file-type npm）
- [x] `media/audio.go` — 语音兼容音频格式检测
- [x] `media/audio_tags.go` — `[[audio_as_voice]]` 标签解析
- [x] `media/store.go` — UUID 存储 + 文件名消毒 + TTL 清理
- [x] `media/fetch.go` — 远程下载 + Content-Disposition + 大小限制
- [x] `media/parse.go` — MEDIA: token 解析器
- [x] `media/image_ops.go` — EXIF/resize/HEIC（sips macOS）— RUST_CANDIDATE P2
- [x] `media/input_files.go` — 图像/文本/PDF 内容提取（PDF 骨架）
- [x] `media/server.go` — HTTP 媒体服务器（骨架）
- [x] `media/host.go` — 媒体托管（骨架，tunnel 延迟）
- [x] 编译验证 ✅
- ~~⛔ **延迟**: SSRF 防护 → 集成 infra/net/ssrf~~ → ✅ Pre-Phase 9 WC 完成 (P7B-3)
- ⛔ **延迟**: PDF 提取 → 集成 pdfcpu

### B3: 7.5 媒体理解（~3,000L → `internal/media/understanding/` 15 文件 986L）

- [x] `understanding/types.go` — Kind/Attachment/Capability/Provider 接口/Output/Decision
- [x] `understanding/defaults.go` — 默认模型/超时/prompt 映射
- [x] `understanding/video.go` — Base64 大小估算
- [x] `understanding/scope.go` — 作用域决策（频道/聊天类型 gating）
- [x] `understanding/concurrency.go` — 泛型并发控制（Go 1.18+ generics）
- [x] `understanding/registry.go` — Provider 注册表 + 能力查找
- [x] `understanding/resolve.go` — 三级优先级级联解析（configured > per-capability > global > default）
- [x] `understanding/provider_*.go` (7 个) — 6 Provider（Pre-Phase 9 WD 完整 HTTP 实现）+ 通用图像处理
- [x] `understanding/runner.go` — 主运行器 + Provider 回退 + 批量执行
- [x] 编译验证 ✅
- ~~⛔ **延迟**: Provider HTTP 调用实现 → Phase 8 集成~~ → ✅ Pre-Phase 9 WD 完成 (P7B-2)

---

## Batch C：大模块（各自独立） ✅

### C1: 7.2 记忆系统（~7,001L → `internal/memory/` 15 文件）

- [x] `memory/types.go` — 核心类型 + MemorySearchManager 接口
- [x] `memory/internal.go` — chunkMarkdown, cosineSimilarity, listMemoryFiles, hashText, RunWithConcurrency
- [x] `memory/schema.go` — SQLite schema (meta/files/chunks/embedding_cache/FTS5)
- [x] `memory/config.go` — ResolveMemoryBackendConfig, QMD 配置
- [x] `memory/embeddings.go` — EmbeddingProvider 接口 + auto/fallback 选择
- [x] `memory/embeddings_{openai,gemini,voyage}.go` — 3 provider
- [x] `memory/hybrid.go` + `search.go` — 混合搜索 (BM25 + vector)
- [x] `memory/provider_key.go` — 缓存键 fingerprint
- [x] `memory/manager.go` — Manager (init/search/sync/status/close)
- [x] `memory/qmd_manager.go` — QMD 子进程管理
- [x] `memory/search_manager.go` — SearchManager (builtin/QMD 委派)
- [x] `memory/status.go` — 状态格式化
- [x] 编译验证：`go build` + `go vet` ✅
- ⛔ **延迟**: batch_{openai,gemini,voyage}.go → P2（可复用 embeddings 层）
- ⛔ **延迟**: sqlite.go + sqlite_vec.go → P2（sqlite-vec 扩展细节）
- ⛔ **延迟**: sync.go (fsnotify) → P2（文件监控）
- ⛔ **延迟**: local embeddings → Phase 10 Rust

### C2: 7.4 浏览器自动化（~10,478L → `internal/browser/` 10 文件）

- [x] `browser/constants.go` — 端口常量
- [x] `browser/config.go` — 配置解析 + EnsurePortAvailable
- [x] `browser/profiles.go` — CDP 端口分配 + 颜色分配
- [x] `browser/chrome_executables.go` — 三平台 Chrome/Chromium/Brave/Edge 发现
- [x] `browser/chrome.go` — Chrome 启动/停止 + DevTools URL 发现
- [x] `browser/cdp_helpers.go` — WebSocket CDP sender + HTTP fetch
- [x] `browser/cdp.go` — CDPClient (navigate/evaluate/screenshot/health)
- [x] `browser/extension_relay.go` — Chrome 扩展 WebSocket 中继
- [x] `browser/session.go` — Session (Chrome + CDP)
- [x] `browser/client.go` — Client 公共 API
- [x] 编译验证：`go build` + `go vet` ✅
- ⛔ **延迟**: server.go + server_context.go → P2（依赖 router/middleware）
- ⛔ **延迟**: client_actions.go → P2（高级页面交互）

---

## Batch D：自动回复引擎（最后执行）— D1-D3 ✅ D4-D6 ✅ 审计 A-

### D: 7.1 自动回复引擎（~22,028L TS → `internal/autoreply/` 36 文件 ~4,200L Go）

> 窗口 1-2 完成 D1-D6 基础骨架 + 回复子包 + 围栏感知分块；窗口 3 完成 P1 审计修复。复杂子系统延迟到 Phase 8+。

#### D1: 类型+基础 ✅

- [x] `tokens.go` (50L) — HeartbeatToken/SilentReplyToken 常量 + IsSilentReplyText
- [x] `thinking.go` (345L) — ThinkLevel/VerboseLevel/ElevatedLevel 枚举 + 规范化 + xhigh 检测
- [x] `model.go` (95L) — ExtractModelDirective 正则解析 + 别名
- [x] `types.go` (59L) — BlockReplyContext/ModelSelectedContext/ReplyPayload/GetReplyOptions
- [x] `heartbeat.go` (190L) — HEARTBEAT_TOKEN 处理 + StripHeartbeatToken + 标记清理
- [x] 测试：`tokens_test.go` + `thinking_test.go` + `model_test.go` + `heartbeat_test.go` = 35 tests ✅

#### D2: 命令系统 ✅

- [x] `commands_types.go` (123L) — CommandScope/Category/ArgType/ChatCommandDefinition 等类型
- [x] `commands_args.go` (121L) — NormalizeArgValue + FormatConfigArgs/DebugArgs/QueueArgs
- [x] `commands_registry.go` (232L) — 全局注册表 + Find/Resolve/Parse/Serialize/Detection
- [x] `commands_data.go` (212L) — 16 个内建命令注册（简化版）
- [x] `group_activation.go` (69L) — GroupActivationMode + /activation 解析
- [x] `send_policy.go` (64L) — SendPolicyOverride + /send 解析
- [x] 测试：`commands_test.go` = 13 tests ✅（累计 48）

#### D3: 入站处理 + 模板 ✅

- [x] `templating.go` (120L) — MsgContext + TemplateContext + {{var}} 替换（含 From/ThreadLabel/GroupChannel/GroupSubject）
- [x] `media_note.go` (56L) — FormatMediaAttachedLine + BuildInboundMediaNote
- [x] `inbound_debounce.go` (89L) — InboundDebouncer（sync.Mutex + time.AfterFunc）
- [x] `command_detection.go` (61L) — HasControlCommand + InlineCommandTokens 检测
- [x] `command_auth.go` (70L) — 命令授权（owner/allow/deny 列表）
- [x] 测试：`templating_test.go` = 13 tests ✅（累计 61）

#### D4: 回复核心 (reply/ 子包) 🔸 部分完成

已完成（逻辑完整，0 TODO）：

- [x] `reply/types.go` (42L) — ReplyDispatchKind/NormalizeReplySkipReason 等类型定义
- [x] `reply/normalize_reply.go` (84L) — 心跳剥离 + 静默检测 + 响应前缀注入
- [x] `reply/inbound_context.go` (235L) — 入站上下文最终化 + **NormalizeChatType + ResolveConversationLabel**（P1-2 审计修复）
- [x] `reply/reply_dispatcher.go` (227L) — goroutine 串行发送队列 + 人类延迟模拟

骨架（文件存在，编译通过，逻辑不完整）：

- [/] `reply/dispatch_from_config.go` (40L) — **骨架**，含 6 个 TODO，Phase 8 接入 agent-runner

未开始（Phase 8）：

- [ ] `reply/route-reply` — 回复路由
- [ ] `reply/get-reply-run` — 回复运行获取
- [ ] `reply/agent-runner-*` (7文件 ~2,000L) — Agent 运行器链路

#### D5: 回复辅助 🔸 部分完成

已完成：

- [x] `reply/abort.go` (53L) — AbortTriggers 对齐 TS（6 裸词 + 3 slash 命令）+ AbortableContext（P1-3 审计修复）
- [x] `reply/response_prefix.go` (21L) — ApplyResponsePrefix
- [x] `reply/body.go` (109L) — 回复体处理（StripThinkingTags + BuildResponseBody + BuildResponseBodyIR）

未开始（Phase 8）：

- [ ] `reply/commands-*` (14文件 ~3,500L) — 命令处理器实现
- [ ] `reply/directive-handling-*` (10文件 ~2,500L) — 指令处理链
- [ ] `reply/mentions` — 提及处理
- [ ] `reply/history` — 历史管理

#### D6: 投递+集成 🔸 部分完成

已完成：

- [x] `tool_meta.go` (60L) — FormatToolAggregate + ShortenToolPath

骨架（文件存在，编译通过，逻辑不完整）：

- [/] `dispatch.go` (28L) — **仅类型定义**，实际分发逻辑未实现
- [/] `envelope.go` (66L) — **简化版**，部分字段硬编码
- [x] `chunk.go` (380L) — **围栏感知分块引擎**，含段落分块、配置解析

未开始（Phase 8）：

- [ ] `reply/typing` — 打字指示器
- [ ] `reply/followup-runner` — 后续运行器
- [ ] `status.ts` 完整移植 (~430L) — 15+ 外部依赖
- [ ] `skill-commands.ts` (~300L) — 技能命令

---

## 验证 ✅

- [x] 完整编译: `go build ./...` ✅
- [x] 静态分析: `go vet ./internal/autoreply/...` ✅
- [x] 单元测试: `go test -race ./internal/autoreply/...` — 62 tests PASS ✅
- [x] pkg/markdown 测试: `go test -race ./pkg/markdown/...` — 9 tests PASS ✅
- [x] 架构文档: `docs/gouji/autoreply.md` ✅
- [x] 延迟项更新: `docs/renwu/deferred-items.md` ✅
- [x] 审计报告: `docs/renwu/phase7-d456-hidden-dep-audit.md` — 健康度 **A-**

### 窗口 2 新增 (2026-02-15)

- [x] `reply/normalize_reply_test.go` — 8 tests
- [x] `reply/inbound_context_test.go` — 15 tests
- [x] `reply/reply_dispatcher_test.go` — 9 tests
- [x] `chunk.go` 围栏感知升级 (87L → 380L) + `chunk_test.go` 30 tests
- [x] `reply/body.go` (109L) — 回复体处理
- [x] `ir.go` 新增 `ChunkMarkdownIR` (函数注入模式)

### 窗口 3 审计修复 (2026-02-15)

- [x] P1-3: `abort.go` 触发词对齐 TS（6 裸词 `stop/esc/abort/wait/exit/interrupt` + 3 slash 命令）
- [x] P1-2: `inbound_context.go` 新增 `NormalizeChatType` + `ResolveConversationLabel` + MsgContext 4 字段
- [x] P1-1 降级为 P2: `sanitizeUserFacingText` 属 agents 模块（Phase 8）
- [x] 审计报告: `docs/renwu/phase7-d456-hidden-dep-audit.md` — 健康度 A-
