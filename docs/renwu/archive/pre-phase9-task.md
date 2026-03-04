# Pre-Phase 9 任务清单 — 延迟项清理（WA-WD）

> 上下文：[pre-phase9-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/pre-phase9-bootstrap.md)
> 延迟项：[deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)
> 最后更新：2026-02-15（全部 4 窗口完成 ✅）

---

## Window A：sessions + agent-limits ✅

### WA-1: P1-F14a sessions/ 模块 ✅

- [x] `internal/sessions/paths.go` — 目录/路径解析
- [x] `internal/sessions/reset.go` — daily/idle 重置策略 + 新鲜度评估
- [x] `internal/sessions/group.go` — 群组键解析 + 显示名构建
- [x] `internal/sessions/main_session.go` — 主会话键 + 别名规范化 + 会话键推导
- [x] `internal/sessions/metadata.go` — Origin 合并 + 元数据补丁
- [x] `internal/sessions/transcript.go` — 转录文件管理 + 助手消息追加
- [x] 5 个测试文件 PASS

### WA-2: P1-F14b agent-limits 确认 ✅

- [x] 已在 Phase 5 中实现（`internal/agents/limits.go` + `limits_test.go`），无需重复工作

### 验证 ✅

- [x] `go build ./...` 通过
- [x] `go vet ./...` 通过
- [x] `go test -race ./internal/sessions/...` 通过

---

## Window B：AgentExecutor + ModelFallback ✅

### WB-1: P4-GA-RUN1 ExtraSystemPrompt ✅

- [x] `runner/run_attempt.go` — `AttemptParams` 新增 `ExtraSystemPrompt string`
- [x] `runner/run.go` L127 — 构建 `AttemptParams` 时传递 `ExtraSystemPrompt`
- [x] `autoreply/reply/agent_runner_execution.go` — `AgentTurnParams` 新增 `ExtraSystemPrompt`
- [x] `autoreply/reply/agent_runner.go` — 从 `FollowupRun.Run.ExtraSystemPrompt` 传入

### WB-2: C2-P1a runWithModelFallback ✅

- [x] `agents/models/fallback.go` — `AuthProfileChecker` 接口 + auth profile cooldown 跳过逻辑
- [x] `autoreply/reply/model_fallback_executor.go` **[NEW]** (204L) — `ModelFallbackExecutor` 实现
- [x] `autoreply/reply/model_fallback_executor_test.go` **[NEW]** — 7 测试 PASS

### WB-3: P8W2-D1 AgentExecutor 完整实现 ✅

- [x] `ModelFallbackExecutor` 替代 `StubAgentExecutor`
- [x] 管线: `RunTurn` → `models.RunWithModelFallback` → `runner.RunEmbeddedPiAgent`

### 验证 ✅

- [x] `go build ./...` 通过
- [x] `go test -race` 7 新测试 PASS

---

## Window C：类型修复 + 安全 ✅

### WC-1: C2-P0 MessagingToolSentTargets 类型修复 ✅

- [x] `runner/types.go` — 新增 `MessagingToolSend` struct + 重命名 `MessagingToolSentTexts` → `MessagingToolSentTargets`
- [x] 级联更新 6 文件: `run_attempt.go`, `agent_runner_execution.go`, `model_fallback_executor.go`, `agent_runner_payloads.go`, `agent_runner.go`, `model_fallback_executor_test.go`

### WC-2: P7B-3 SSRF 防护集成 ✅

- [x] `internal/security/ssrf.go` **[NEW]** (230L) — `IsPrivateIP()` + `IsBlockedHostname()` + `SafeFetchURL()`
- [x] `internal/security/ssrf_test.go` **[NEW]** — 12 测试 PASS
- [x] `internal/media/fetch.go` — 替换裸 `http.Get` 为 `SafeFetchURL`
- [x] `internal/media/input_files.go` — 同上

### 验证 ✅

- [x] `go build ./...` 通过
- [x] `go test -race` 12 新安全测试 PASS

---

## Window D：HTTP Provider 实现 ✅

### WD-1: P7B-1 TTS Provider HTTP 调用 ✅

- [x] `internal/tts/synthesize.go` — 从骨架到完整 HTTP 实现
  - OpenAI: POST `/v1/audio/speech` (JSON body)
  - ElevenLabs: POST `/v1/text-to-speech/{voiceId}` (voice_settings 支持)
  - Edge: `edge-tts` CLI fallback + 超时控制

### WD-2: P7B-2 媒体理解 Provider HTTP 调用 ✅

- [x] 6 个 `provider_*.go` 文件从骨架到完整 HTTP 实现
  - `provider_openai.go` — Whisper (multipart) + GPT-4V (chat completions)
  - `provider_google.go` — Gemini (generateContent, 3 种能力共用)
  - `provider_anthropic.go` — Claude Vision (/v1/messages, base64)
  - `provider_deepgram.go` — Deepgram Nova-2 (/v1/listen, raw audio)
  - `provider_groq.go` — Groq Whisper (OpenAI-compat)
  - `provider_minimax.go` — MiniMax (chatcompletion_v2)

### 验证 ✅

- [x] `go build ./...` 通过
- [x] `go vet ./...` 通过

---

## 延迟项状态更新

以下 `deferred-items.md` 中的项在 Pre-Phase 9 中已标记 ✅：

| 延迟项 | 窗口 |
|--------|------|
| P1-F14a sessions/ | WA |
| P1-F14b agent-limits | WA |
| P4-GA-RUN1 ExtraSystemPrompt | WB |
| C2-P1a runWithModelFallback | WB |
| P8W2-D1 AgentExecutor | WB |
| C2-P0 MessagingToolSentTargets | WC |
| P7B-3 SSRF 防护 | WC |
| P7B-1 TTS Provider HTTP | WD |
| P7B-2 媒体理解 Provider HTTP | WD |
