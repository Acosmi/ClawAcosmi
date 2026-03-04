# Phase 10 最终补全 — 新窗口 Bootstrap

> 最后更新：2026-02-17
> 任务文档：[phase10-final-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-final-task.md)
> 审计报告：[phase10-final-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-final-audit.md)

---

## 新窗口启动模板

复制以下内容到新窗口即可快速恢复上下文：

```
@/refactor 执行 Phase 10 最终补全。

任务文档：docs/renwu/phase10-final-task.md
审计报告：docs/renwu/phase10-final-audit.md
延迟汇总：docs/renwu/deferred-items.md
全局路线图：docs/renwu/refactor-plan-full.md
编码规范：skills/acosmi-refactor/references/coding-standards.md

当前进度：请读取 phase10-final-task.md 确认已完成和待办项。

本批次目标：[在此指定 Batch FA / FB / FC / FD / FE]
```

---

## 一、背景

2026-02-17 全局审计发现后端 Go 代码残留 36 处 TODO/FIXME/STUB，其中 29 处需要处理。按组件分为 5 个 Batch，覆盖 CLI 入口串联、Gateway HTTP 路由、频道补全、辅助模块和性能基准。

以下两项已挂起到 `deferred-items.md` 延迟待办，不在本轮范围：

- Phase 11 Ollama 集成
- 前端 View i18n 全量抽取（13 文件 ~275 key）

---

## 二、批次概览

| 批次 | 内容 | 优先级 | 预估文件数 | 预估工时 |
|------|------|--------|-----------|---------|
| **FA** | CLI 入口串联 | P1 | ~6 改 | 0.5-1 天 |
| **FB** | Gateway HTTP 路由 + 幂等去重 | P1 | ~4 改/新 | 1-2 天 |
| **FC** | 频道补全（TG/DIS/SLK/IM/插件） | P2 | ~8 改 | 1 天 |
| **FD** | 辅助模块（媒体/网络/配置） | P2-P3 | ~5 改 | 0.5 天 |
| **FE** | 性能基准 + 前端增强 | P2-P3 | 脚本 + 报告 | 1-2 天 |

**建议执行顺序**：FA → FB → FC → FD → FE

---

## 三、各批次关键指引

### Batch FA：CLI 入口串联

**目标**：使 `acosmi` 和 `openacosmi` 主二进制具备完整 CLI 功能。

**关键文件**：

- `cmd/acosmi/main.go` (46L) — 当前仅有信号处理和日志初始化，需串联 Gateway 启动
- `cmd/openacosmi/main.go` (118L) — Cobra 框架已就绪，但 3 个子命令是桩
- `cmd/openacosmi/cmd_agent.go` — 需串联 `internal/agents/exec/cli_runner.go`
- `cmd/openacosmi/cmd_doctor.go` — 需串联诊断检查逻辑
- `cmd/openacosmi/cmd_status.go` — 需通过 Gateway RPC 获取状态
- `internal/cli/gateway_rpc.go` — **核心**：CLI→Gateway WebSocket RPC 客户端

**前置已就绪**：

- `internal/gateway/` — 完整 Gateway 服务器 + WS + 方法注册 ✅
- `internal/agents/exec/cli_runner.go` — CLI Agent 运行逻辑 ✅
- `internal/cli/` — 全部 6 个工具文件 ✅

**参考模式**：

- TS `src/cli/program/` — Commander 命令注册
- TS `src/gateway/client.ts` — WebSocket RPC 客户端实现

**验证**：

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./cmd/... ./internal/cli/...
```

---

### Batch FB：Gateway HTTP 路由 + 幂等

**目标**：实现 OpenAI 兼容 API（SSE 流式响应）+ 幂等去重。

**关键文件**：

- `internal/gateway/server_http.go` (302L) — 3 个 501 占位路由
- `internal/gateway/server_methods_chat.go` — `chat.send` 幂等
- `internal/gateway/server_methods_send.go` — `send` 幂等

**TS 参考**：

- `src/gateway/server-http.ts` — OpenAI API 路由
- `src/gateway/openai-api.ts` — Chat Completions 处理
- `src/gateway/openai-responses-api.ts` — Responses API

**实现方案**：

1. **幂等去重**：新建 `idempotency.go`，使用 `sync.Map` + TTL 缓存（5 分钟过期），key 去重 + 旧结果返回
2. **OpenAI API**：解析 chat completions 请求 → 映射到 agent session → 调用 `DispatchInboundMessage` → SSE 流式推送

**验证**：

```bash
cd backend && go build ./... && go vet ./...
cd backend && go test -race ./internal/gateway/...
```

---

### Batch FC：频道补全

**目标**：补全各频道的边缘功能缺口。

**前置已就绪**：

- `internal/channels/` — 全部 6 频道 SDK 完整 ✅
- `pkg/markdown/` — Markdown 处理 ✅
- `internal/media/` — 媒体处理管线 ✅
- `internal/plugins/` — 插件注册表 ✅

**逐频道指引**：

| 频道 | 需改文件 | 核心变更 |
|------|---------|---------|
| Telegram | `http_client.go`, `bot_delivery.go` | SOCKS5 Transport + 媒体 download→understand |
| Discord | `chunk.go`, `send_guild.go` | autoreply 分块引入 + audit 头 |
| Slack | `monitor_channel_config.go` | 频道 key 候选构建 |
| iMessage | `send.go` (2 处) | media/store 集成 + constants |
| 插件 | `dock.go`, `message_actions.go` | PluginRegistry 动态查询 |

---

### Batch FD：辅助模块补全

**目标**：补全媒体处理和网络辅助功能。

**快速实现指引**：

| 项 | 方案 |
|------|------|
| FD-1 图像旋转 | macOS: `sips -r <deg> <file>`; 其他: `golang.org/x/image/draw` |
| FD-2 URL 图像下载 | 复用 `security.SafeFetchURL()` + `base64.StdEncoding.EncodeToString()` |
| FD-3 Whisper 探测 | `exec.LookPath("whisper-cpp")` / `exec.LookPath("sherpa-onnx")` |
| FD-4 媒体服务器 | 复用 `gateway.ExtractBearerToken()` + `corsMiddleware()` |
| FD-5 Tailnet IP | `exec.Command("tailscale", "status", "--json")` → JSON parse |
| FD-6 Config 快照 | 调用 `config.LoadConfig()` + 缓存 |
| FD-7 过时注释 | 删除或更新 `plugin_auto_enable.go` 注释 |

---

### Batch FE：性能基准 + 前端 ✅

**已完成** (2026-02-17):

- FE-1/FE-2: Go 性能基准报告刷新（`TestGenerateBenchReport` PASS）
- FE-3: 前端日期/数字本地化 — `format.ts` 新增 `formatDateTime`/`formatTimeShort`/`formatNumber`
- FE-4: Vite 构建验证通过

---

## 四、编码规范提醒

1. 所有表述/文档使用中文，代码标识符使用英文
2. 接口在使用方定义（DI 模式）
3. 错误处理：`fmt.Errorf("xxx: %w", err)` 不可 panic
4. 详见 `skills/acosmi-refactor/references/coding-standards.md`

## 五、完成后文档更新

每个 Batch 完成后必须更新：

1. `deferred-items.md` — 对应项标记 ✅
2. `phase10-final-task.md` — 勾选完成项
3. `refactor-plan-full.md` — Phase 10 状态更新
