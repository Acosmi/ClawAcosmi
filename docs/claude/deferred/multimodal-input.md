---
document_type: Deferred
status: Archived
created: 2026-02-26
last_updated: 2026-03-01
source: docs/claude/audit/audit-2026-02-26-multimodal-input.md
full_review: docs/claude/audit/audit-2026-02-27-multimodal-full-review.md
fix_audit: docs/claude/audit/audit-2026-02-27-multimodal-deferred-5fix.md
---

# 多模态输入 — 延迟项

## ✅ 已修复

### GAP-2: send RPC 无 binary/base64 输入路径（已修复 2026-02-27）
- **审计报告**: `audit/audit-2026-02-27-multimedia-feishu-outbound.md`
- **修复内容**: `OutboundSendParams` 新增 `MediaData []byte` + `MediaMimeType string`；
  `handleSend` 解析 `mediaBase64` 参数并路由到 `ChannelMgr`；
  `FeishuPlugin` 新增 `sendMediaData` 方法直接上传二进制到飞书。
- **文件**: `channels/outbound.go`, `gateway/server_methods_send.go`, `channels/feishu/plugin.go`

### I-01 (was M-08): resource.go http.DefaultClient 无超时（已修复 2026-02-27）
- **审计报告**: `audit/audit-2026-02-27-multimodal-deferred-5fix.md`
- **修复内容**: 模块级 `httpClient = &http.Client{Timeout: 30s}`，替换全部 4 处 `http.DefaultClient`
- **文件**: `feishu/resource.go`

### I-02 (was M-11): getTenantAccessToken 两处无缓存（已修复 2026-02-27）
- **审计报告**: `audit/audit-2026-02-27-multimodal-deferred-5fix.md`
- **修复内容**: FeishuClient + feishuProvider 各添加 `sync.Mutex` 保护的 token 缓存（TTL 115min）
- **文件**: `feishu/client.go`, `feishu/resource.go`, `gateway/remote_approval_feishu.go`

### I-03 (was L-11): 附件顺序处理无并发（已修复 2026-02-27）
- **审计报告**: `audit/audit-2026-02-27-multimodal-deferred-5fix.md`
- **修复内容**: `sync.WaitGroup` 并发处理附件，索引数组保持顺序
- **文件**: `gateway/server_multimodal.go`

### O-01 (was L-12): mediaUrl 路径返回 stub（已修复 2026-02-27）
- **审计报告**: `audit/audit-2026-02-27-multimodal-deferred-5fix.md`
- **修复内容**: stub 前插入 ChannelMgr fallback，mediaUrl → 飞书上传发送
- **文件**: `gateway/server_methods_send.go`

### O-02 (was L-13): 自动回复只发文字不发媒体（已修复 2026-02-27）
- **审计报告**: `audit/audit-2026-02-27-multimodal-deferred-5fix.md`
- **修复内容**: 新增 `DispatchReply` 类型（Text + MediaData + MediaMimeType），
  `DispatchMultimodalFunc` 返回 `*DispatchReply`，飞书插件支持媒体回传，企微/钉钉适配新签名
- **文件**: `channels/channel_message.go`, `feishu/plugin.go`, `wecom/plugin.go`, `dingtalk/plugin.go`, `gateway/server.go`

## 待实现功能 (Phase E 额外发现)

### F-01: 多模态模型原生 vision 注入
- **来源**: Phase E 图片理解 Fallback 实现过程中发现
- **现状**: 图片 base64 仅传给前端显示，LLM 只收到文字描述（`[图片描述]: ...` 或 `[图片: image/png, ...]`）
- **问题**: 当主模型本身支持多模态（Claude、GPT-4o 等），应直接将 base64 图片注入 LLM 消息 `content` 数组走原生 vision，而非转为文字
- **Phase E 覆盖**: Phase E 的 `ImageDescriber` 解决了非多模态模型（DeepSeek 等）的 fallback 问题
- **待做**: 在 `attempt_runner.go` 或 agent 消息构建层，检测 `ModelSupportsVision()` → 将图片 base64 直接注入 `content[]` 的 `image` block
- **文件**: `runner/attempt_runner.go`, `runner/run_attempt.go`, `models/catalog.go`（`ModelSupportsVision` 已存在）
- **优先级**: MEDIUM — 多模态模型用户体验大幅提升

### F-02: send_media 工具未启用
- **来源**: Phase E 图片理解 Fallback 实现过程中发现
- **现状**: `backend/internal/agents/tools/send_media_tool.go` 已存在完整实现，但 `EnableSendMedia: false`
- **问题**: 智能体无法主动发送图片/文件给用户，只能发文字
- **待做**: 启用 `EnableSendMedia` + 确保 `feishu_actions.go` 等渠道 action 支持媒体发送
- **文件**: `tools/send_media_tool.go`, `tools/feishu_actions.go`（目前只有 `send_message` 纯文本）
- **优先级**: LOW — 功能完整但非核心路径

---

## MEDIUM (5 项，原 8 项已修复 3 项)

### M-07: sendMediaMessage 缺少 HTTP 状态码检查
- **文件**: `feishu/plugin.go`
- **影响**: 404/500 响应被当作有效媒体上传到飞书
- **修复**: 检查 `resp.StatusCode != 200`

### M-09: FeishuPlugin.Stop 未取消 WebSocket goroutine
- **文件**: `feishu/plugin.go:200-215`
- **影响**: Stop 后 WebSocket goroutine 泄漏
- **修复**: 调用 `wsCancel[accountID]()` 并清理 wsClients/wsCancel
- **注**: 预存在问题

### M-10: DownloadResource URL 路径注入
- **文件**: `feishu/resource.go:46-47`
- **影响**: messageID/fileKey 含特殊字符可改变 API 路径
- **修复**: 使用 `url.PathEscape()`

### M-12: stt.config.set 无输入验证
- **文件**: `gateway/server_methods_stt.go:75-113`
- **影响**: 可写入任意 provider 字符串、SSRF via BaseURL
- **修复**: Provider 白名单验证 + BaseURL HTTPS 检查

### M-14: MediaRecorder 缺少 onerror handler
- **文件**: `ui/controllers/voice-recorder.ts`
- **影响**: 录音期间 OS 错误可导致 stop() Promise 永远不 resolve
- **修复**: 添加 `recorder.onerror` → cleanup + reject

### M-03 (补充): Silent failure on voice start error
- **文件**: `ui/app.ts:507-519`
- **影响**: 用户点击麦克风无反应 (权限拒绝时无提示)
- **修复**: 在 catch 中设置 lastError 显示用户可见错误

## LOW (7 项，原 10+3 项已修复 3 项)

- L-01: truncateStr 按字节截断可能破坏 UTF-8 (server_methods_chat.go)
- L-02: detectImageMediaType 未知格式默认 image/png (server_multimodal.go)
- L-03: STT handler 超时未继承调用者 context (server_methods_stt.go)
- L-04: processAttachmentsForChat 每次调用重建 provider (server_methods_chat.go)
- L-05: 录音无最大时长限制 (voice-recorder.ts)
- L-06: 录音 blob 无大小检查 (app.ts)
- L-07: 计时器 500ms 间隔 vs 秒级显示不匹配 (voice-recorder.ts)
- L-08: 消息去重清理用 goroutine 而非 time.AfterFunc (plugin.go, 预存在)
- L-09: 上传文件名硬编码 audio.opus/file (plugin.go)
- L-10: detectMediaCategory magic bytes 检查有限 (plugin.go)
