# Phase 10 最终审计报告 — Go 后端残留 TODO 全量扫描

> **审计时间**：2026-02-17
> **审计方法**：`grep -rn 'TODO|FIXME|HACK|STUB' --include='*.go' internal/ pkg/ cmd/`（排除测试文件）
> **任务清单**：[phase10-final-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-final-task.md)
> **Bootstrap**：[phase10-final-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-final-bootstrap.md)

---

## 一、审计概览

| 维度 | 数据 |
|------|------|
| 后端 Go 源文件 | 578（非测试） |
| 后端测试文件 | 127 |
| `go build ./...` | ✅ 通过 |
| `go test ./...` (57 包) | ✅ 全部 PASS |
| 前端 Vite build | ✅ 通过（408ms） |
| 残留 TODO 总数 | 36 处（29 处需处理） |
| 架构文档 | 27 模块 ✅ 全覆盖 |

---

## 二、Batch 分组

### Batch FA: CLI 入口串联（7 处 TODO）

> 影响：`acosmi` + `openacosmi` 主二进制无法执行完整 CLI 流程

| # | 文件 | 行号 | TODO 内容 | 说明 |
|---|------|------|-----------|------|
| FA-1 | [main.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/cmd/acosmi/main.go) | 40 | 启动网关服务器 | `acosmi` 主入口需调用 `gateway.RunGatewayBlocking` |
| FA-2 | [main.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/cmd/acosmi/main.go) | 41 | CLI 命令注册 | `acosmi` 需注册 Cobra 命令（或委托给 `openacosmi`） |
| FA-3 | [main.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/cmd/openacosmi/main.go) | 62 | 插件命令检测 | `PLUGIN_REQUIRED_COMMANDS` 判定 |
| FA-4 | [cmd_agent.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/cmd/openacosmi/cmd_agent.go) | 31 | Agent 运行逻辑 | `openacosmi agent` 子命令桩 |
| FA-5 | [cmd_doctor.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/cmd/openacosmi/cmd_doctor.go) | 15 | 诊断检查 | `openacosmi doctor` 桩 |
| FA-6 | [cmd_status.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/cmd/openacosmi/cmd_status.go) | 21 | 状态检查 | `openacosmi status` 桩 |
| FA-7 | [gateway_rpc.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/cli/gateway_rpc.go) | 25,38 | Gateway RPC 客户端 | CLI→Gateway WebSocket 调用通道 |

---

### Batch FB: Gateway HTTP 路由 + 幂等（5 处 TODO）

> 影响：OpenAI 兼容 API 不可用 + 重复请求无防护

| # | 文件 | 行号 | TODO 内容 | 说明 |
|---|------|------|-----------|------|
| FB-1 | [server_http.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/server_http.go) | 46 | OpenAI Chat Completions API | `/v1/chat/completions` 返回 501 |
| FB-2 | [server_http.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/server_http.go) | 55 | OpenAI Responses API | `/v1/responses` 返回 501 |
| FB-3 | [server_http.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/server_http.go) | 63 | Tools Invoke | `/tools/invoke/` 返回 501 |
| FB-4 | [server_methods_chat.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/server_methods_chat.go) | 204 | 幂等去重 | `chat.send` idempotencyKey 未实现 |
| FB-5 | [server_methods_send.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/server_methods_send.go) | 62 | 幂等去重 | `send` idempotencyKey 未实现 |

---

### Batch FC: 频道补全（9 处 TODO）

> 影响：各频道边缘功能缺失

| # | 文件 | 行号 | TODO 内容 | 说明 |
|---|------|------|-----------|------|
| FC-1 | [http_client.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/telegram/http_client.go) | 50 | SOCKS5 代理 | `golang.org/x/net/proxy` |
| FC-2 | [bot_delivery.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/telegram/bot_delivery.go) | 211 | 完整媒体处理 | download + media-understanding 集成 |
| FC-3 | [chunk.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/discord/chunk.go) | 247 | Markdown newline 分块 | 需从 autoreply 包引入 `chunkMarkdownTextWithMode` |
| FC-4 | [send_guild.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/discord/send_guild.go) | 81 | Audit Log Reason | `X-Audit-Log-Reason` header |
| FC-5 | [monitor_channel_config.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/slack/monitor_channel_config.go) | 72 | 频道 key 匹配 | `BuildChannelKeyCandidates` |
| FC-6 | [send.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/imessage/send.go) | 68 | web media | 依赖 `media/store` |
| FC-7 | [send.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/imessage/send.go) | 233 | media 常量 | 替换为 `media/constants` |
| FC-8 | [dock.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/dock.go) | 160 | 插件频道动态追加 | `PluginRegistry` |
| FC-9 | [message_actions.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/channels/message_actions.go) | 22,28,34 | 插件动作注册 | 动态扩展动作列表 |

---

### Batch FD: 辅助模块补全（6 处 TODO）

> 影响：媒体处理和配置边缘缺口

| # | 文件 | 行号 | TODO 内容 | 说明 |
|---|------|------|-----------|------|
| FD-1 | [image_ops.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/media/image_ops.go) | 321 | 图像旋转 | EXIF 旋转逻辑（sips / Go image） |
| FD-2 | [provider_image.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/media/understanding/provider_image.go) | 35 | URL 图像下载 | 从 URL 下载并编码为 base64 |
| FD-3 | [runner.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/media/understanding/runner.go) | 193 | Whisper 本地探测 | whisper-cpp / sherpa-onnx / gemini CLI |
| FD-4 | [server.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/media/server.go) | 13 | 完整 HTTP 服务器 | CORS + 认证集成 |
| FD-5 | [net.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/net.go) | 227 | Tailnet IP 查询 | tailscale status --json |
| FD-6 | [config_guard.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/cli/config_guard.go) | 56 | Config 快照 | `ReadConfigFileSnapshot()` |

---

### Batch FE: 性能基准 + 前端增强（2 项）

| # | 内容 | 说明 |
|---|------|------|
| FE-1 | Phase 10.3 性能基准对比 | Go vs Node.js: 内存/延迟/吞吐量 |
| FE-2 | 前端日期/数字本地化 | `Intl.DateTimeFormat` / `Intl.NumberFormat` |

---

### 标注性 TODO（无需处理，7 处）

| 文件 | 说明 |
|------|------|
| `plugin_auto_enable.go:368,436` | 过时注释，Phase 5 Extra 字段已完成 |
| `autoreply/reply/` (4 处) | Phase 9 D5 标注性注释（功能已填充） |
| `compaction.go:250` | LLM 提示词文本中的 "TODOs"（非代码 TODO） |

---

## 三、挂起到延迟待办

以下两项**不在本轮修复范围**，已追加到 `deferred-items.md`：

| 项目 | 说明 |
|------|------|
| Phase 11 Ollama 集成 | 本地 LLM 支持，对系统无根本影响 |
| 前端 View i18n 全量抽取 | 13 个 view 文件 ~275 key |
