# 延迟待办修复任务清单 (deferred-fix)

> 来源：2026-02-16 `deferred-items.md` 审计
> 上下文：[deferred-fix-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-fix-bootstrap.md)
> 最后更新：2026-02-16（Batch DF-D 4/4 完成）

---

## Batch DF-A：Session 管理模块（P1，阻塞生产） ✅

- [x] 创建 `internal/agents/session/` 目录
- [x] 实现 `manager.go` — session 文件读写 + 文件锁（sync.Mutex）
- [x] 实现 `system_prompt.go` — 动态提示词组装 + context pruning
- [x] 实现 `history.go` — 历史消息验证 + Token 截断
- [x] 编写 `manager_test.go` / `system_prompt_test.go` / `history_test.go` (37 tests)
- [x] `go build ./...` + `go vet ./...` + `go test -race ./internal/agents/session/...` PASS
- [x] 更新 `deferred-items.md` P10-W2 状态为 ✅

## Batch DF-B：调用端接入（高优先级） ✅

### DF-B1: chunkMarkdownIR 接入

- [x] 修改 `internal/channels/slack/format.go` — IR + RenderMarkdownWithMarkers + ChunkMarkdownIR
- [x] 修改 `internal/channels/telegram/format.go` — IR + Telegram HTML markers + ChunkMarkdownIR
- [x] 编写 `format_test.go` (Slack 7 tests + Telegram 11 tests)

### DF-B2: loadWebMedia 统一

- [x] 新建 `pkg/media/web_media.go` — 共享 WebMedia + LoadWebMedia（支持本地+HTTP+maxBytes）
- [x] 修改 `internal/channels/whatsapp/media.go` — type alias + 委托给 pkg/media
- [x] 修改 `internal/channels/discord/send_media.go` — 委托给 pkg/media，移除重复代码
- [x] 修改 `internal/channels/slack/send.go` — 移除 loadWebMedia TODO
- [x] Telegram `send.go` / `bot_delivery.go` — loadWebMedia TODO 保留（媒体发送需更多上下文）
- [x] 编写 `pkg/media/web_media_test.go` (8 tests)
- [x] `go build ./...` + `go vet ./...` + `go test -race` 全部 PASS

## Batch DF-C：辅助补全（中优先级） ✅

### DF-C1: Slack files.uploadV2

- [x] 修改 `internal/channels/slack/client.go` — 升级到 `files.uploadV2` 3 步 API
- [x] 测试 Slack 文件上传正常 — `client_upload_test.go` 6 tests PASS
- [x] 更新 `deferred-items.md` SLK-P7-B 状态为 ✅

### DF-C2: Runner 集成测试

- [x] 新建 `internal/agents/runner/integration_test.go` — 5 tests PASS
- [x] 测试 llmclient + tool_executor 集成场景
- [x] E2E 真实 LLM 调用测试 — 保留为可选，未实施
- [x] 更新 `deferred-items.md` P10-W3 状态为 ✅

## Batch DF-D：低优先级保留（不阻塞发布） ✅ 4/4

- [x] P7A-3: Security Audit 完整实现 — `audit.go` + `audit_extra.go` + `audit_test.go`
- [x] P7C-4: Local Embeddings — Ollama `/api/embed` PureGo 实现（`embeddings_local.go` + 测试）
- [x] P7B-5: 图像双三次缩放 — 升级 Go CatmullRom（Rust FFI 保留为可选优化）
- [x] P7B-6: 媒体隧道 Tailscale/Cloudflared — `host.go` CLI 集成 + 降级链

## 文档更新（每批次完成后）

- [x] 更新 `deferred-items.md` 对应条目状态
- [x] 更新 `refactor-plan-full.md` 相关章节
