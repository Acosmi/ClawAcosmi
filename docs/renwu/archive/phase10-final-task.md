# Phase 10 最终补全 — 任务清单

> 最后更新：2026-02-17
> 审计来源：[phase10-final-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-final-audit.md)
> Bootstrap：[phase10-final-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-final-bootstrap.md)

---

## Batch FA：CLI 入口串联（P1 阻塞）

> 目标：使 `acosmi` 和 `openacosmi` 主二进制具备完整 CLI 功能。
> 预估文件数：6 改

- [x] FA-1: `cmd/acosmi/main.go` — 调用 `gateway.RunGatewayBlocking` 启动网关（含配置加载）
- [x] FA-2: `cmd/acosmi/main.go` — 纯网关入口（CLI 命令由 `openacosmi` 二进制承担）
- [x] FA-3: `cmd/openacosmi/main.go:62` — 实现 `PLUGIN_REQUIRED_COMMANDS` 判定逻辑（message/channels/directory 命令需插件注册表）
- [x] FA-4: `cmd/openacosmi/cmd_agent.go:31` — 串联 `internal/agents/` agent 运行逻辑（读取配置 → RunCliAgent）
- [x] FA-5: `cmd/openacosmi/cmd_doctor.go:15` — 串联诊断检查（config 校验 + port 检测 + 依赖检查）
- [x] FA-6: `cmd/openacosmi/cmd_status.go:21` — 串联 `internal/` 状态检查逻辑（Gateway RPC `status` 方法调用）
- [x] FA-7: `internal/cli/gateway_rpc.go` — 实现 CLI→Gateway WebSocket RPC 客户端（连接 + hello 握手 + method call + 响应解析）
- [x] FA-8: `internal/cli/config_guard.go:56` — 实现 `ReadConfigFileSnapshot()` 集成
- [x] FA-9: 编译验证 `go build ./...` + `go vet ./...`
- [x] FA-10: 运行测试 `go test -race ./cmd/... ./internal/cli/...`

---

## Batch FB：Gateway HTTP 路由 + 幂等（P1 功能）✅

> 目标：实现 OpenAI 兼容 API 端点 + 请求幂等去重。
> 预估文件数：3 改 + 1 新 → 实际：5 改 + 4 新
> 完成时间：2026-02-17

- [x] FB-1: 实现 `/v1/chat/completions` 处理器 — 转发到 Agent Pipeline，流式 SSE 响应
- [x] FB-2: 实现 `/v1/responses` 处理器 — OpenAI Responses API 兼容（代理到 chat completions）
- [x] FB-3: 实现 `/tools/invoke/` 处理器 — 工具调用端点（DI ToolInvoker 接口）
- [x] FB-4: 实现幂等去重模块 — `internal/gateway/idempotency.go`（sync.Map TTL 缓存 + reaper goroutine）
- [x] FB-5: `server_methods_chat.go:204` — `chat.send` 接入幂等检查
- [x] FB-6: `server_methods_send.go:62` — `send` 接入幂等检查
- [x] FB-7: 编译验证 + 单元测试（`go build ✅ | go vet ✅ | go test -race ✅`）

---

## Batch FC：频道补全（P2 增强）✅

> 目标：补全各频道的边缘功能缺口。
> 预估文件数：~8 改 → 实际：10 改
> 完成时间：2026-02-17

### FC-1: Telegram

- [x] FC-1a: `http_client.go:50` — SOCKS5 代理支持（`golang.org/x/net/proxy`，支持 `socks5://` + `socks5h://`）
- [x] FC-1b: `bot_delivery.go:211` — 完整媒体处理（`ResolveMedia` + `ResolveMediaFull` via `GetTelegramFile` + `DownloadTelegramFile`）

### FC-2: Discord

- [x] FC-2a: `chunk.go:247` — Markdown newline 分块模式（`autoreply.ChunkMarkdownTextWithMode`）
- [x] FC-2b: `send_guild.go:81` — `X-Audit-Log-Reason` header 支持（`discordRESTWithHeaders` + `DiscordTimeoutTarget.Reason`）

### FC-3: Slack

- [x] FC-3a: `monitor_channel_config.go:72` — 完善频道 key 匹配逻辑（`BuildChannelKeyCandidates` + `ResolveChannelEntryMatchWithFallback` 4 级 fallback：精确→不敏感→glob→通配符）

### FC-4: iMessage

- [x] FC-4a: `send.go:68` — `resolveAttachment` 集成 `media.SaveMediaSource`
- [x] FC-4b: `send.go:233` — `mediaKindFromMime` 委托 `media.MediaKindFromMime`

### FC-5: 插件频道

- [x] FC-5a: `dock.go:160` — `PluginChannelDockProvider` DI 回调，`GetChannelDock` + `ListChannelDocks` 查询插件频道
- [x] FC-5b: `message_actions.go:22,28,34` — `PluginActionProvider` / `PluginMessageButtonsProvider` / `PluginMessageCardsProvider` DI 回调

### FC-6: 验证

- [x] FC-6a: 编译验证 `go build ./...` ✅ + `go vet ./...` ✅
- [x] FC-6b: 运行频道测试 `go test -race ./internal/channels/...` ✅ (6 包全通过)

---

## Batch FD：辅助模块补全（P2-P3）✅

> 目标：补全媒体处理和网络辅助功能缺口。
> 预估文件数：~5 改 → 实际：6 改
> 完成时间：2026-02-17

- [x] FD-1: `image_ops.go:321` — 实现 EXIF 旋转逻辑（`sips -r 0` on macOS + Go `image` 像素变换 cross-platform，覆盖 8 种 orientation 值）
- [x] FD-2: `provider_image.go:35` — URL 图像下载并编码为 base64（复用 `security.SafeFetchURL` SSRF 防护 + `io.LimitReader` 大小限制）
- [x] FD-3: `runner.go:193` — 本地 whisper 探测（`exec.LookPath` 检测 whisper-cpp/whisper/sherpa-onnx/gemini + `--version` 版本提取）
- [x] FD-4: `server.go:13` — 媒体 HTTP 服务器完善（`MediaServerAuthConfig` Bearer token 认证 + CORS 预飞 OPTIONS 处理 + `Access-Control-*` 头）
- [x] FD-5: `net.go:227` — Tailnet IP 查询（`tailscale status --json` → 解析 `Self.TailscaleIPs[0]` IPv4 优先 + `CanBindToHost` 校验）
- [x] FD-6: `config_guard.go:56` — ✅ 已在 FA-8 中完成（`ReadConfigFileSnapshot()` 集成）
- [x] FD-7: 清理过时注释 — `plugin_auto_enable.go:368,436`（Phase 5 Extra 字段已完成，注释已更新）
- [x] FD-8: 编译验证 + 测试（`go build ✅ | go vet ✅ | go test -race ./... ✅` 全 57 包通过）

---

## Batch FE：性能基准 + 前端增强（P2-P3）✅

> 目标：完成 Phase 10.3 性能验证 + 前端日期本地化。
> 完成时间：2026-02-17

- [x] FE-1: 编写性能基准脚本（wrk/hey + 内存采样）— Go vs Node.js 对比
- [x] FE-2: 运行基准测试，生成 `phase10-bench-report.md`
- [x] FE-3: 前端日期/数字本地化 — `Intl.DateTimeFormat` / `Intl.NumberFormat` 集成到 `format.ts`
- [x] FE-4: Vite 构建验证

---

## 完成后文档更新

每个 Batch 完成后必须更新：

1. `deferred-items.md` — 对应项标记 ✅
2. 本文件 — 勾选完成项
3. `refactor-plan-full.md` — Phase 10 状态更新
