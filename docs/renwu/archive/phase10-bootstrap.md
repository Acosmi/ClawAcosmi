# Phase 10 Bootstrap — 新窗口启动上下文

> 最后更新：2026-02-16

---

## 新窗口启动模板

将以下内容粘贴到新窗口即可恢复上下文：

```
@/refactor 我需要开始 Phase 10 集成与验证。

参考文件：
- 任务清单：docs/renwu/phase10-task.md
- Bootstrap：docs/renwu/phase10-bootstrap.md（本文件）
- 延迟项汇总：docs/renwu/deferred-items.md
- 全局路线图：docs/renwu/refactor-plan-full.md

Phase 10 目标：验证 Go 后端 ↔ 前端集成，确保 API 兼容性。
```

---

## 一、项目当前状态

### 后端代码统计 (2026-02-16)

| 维度 | 数据 |
|------|------|
| Go 源文件（非测试） | 609 个 |
| Go 源码行数 | 107,340 行 |
| Go 测试文件 | 110 个 |
| Go 测试行数 | 17,832 行 |
| internal/ 模块数 | 22 个 |
| pkg/ 共享包 | 7 个 |
| 编译状态 | ✅ `go build ./...` + `go vet ./...` PASS |
| 测试状态 | ✅ `go test -race ./...` 40 包全 PASS |

### 模块清单

**internal/**：acp, agents, autoreply, browser, channels, cli, config, cron, daemon, gateway, hooks, infra, linkparse, media, memory, ollama, outbound, plugins, routing, security, sessions, tts

**pkg/**：contracts, i18n, log, markdown, retry, types, utils

### Phase 0-9 完成总结

| Phase | 内容 | 状态 |
|-------|------|------|
| 0 | 项目基础搭建 | ✅ |
| 1 | 类型系统与配置 | ✅ |
| 2 | 基础设施层 | ✅ |
| 3 | 网关核心层 | ✅ |
| 4 | Agent 引擎 | ✅ |
| 5 | 通信频道适配器 | ✅ |
| 6 | CLI + 插件 + 钩子 + 守护进程 + ACP | ✅ |
| 7 | 辅助模块（auto-reply/memory/security/browser/media/tts/markdown） | ✅ |
| 8 | P7D 延迟项（reply/ 子包完整） | ✅ |
| Pre-9 | sessions + AgentExecutor + 类型修复 + HTTP Provider | ✅ |
| 9 | 延迟项清理（~60 项，4 批次） | ✅ 2026-02-16 审计通过 |

---

## 二、Phase 10 目标

**集成与验证** — 确保 Go 后端可以替代 Node.js 后端运行，与前端 UI 完全兼容。

### 10.1 前端 ↔ Go 网关集成测试

- 启动 Go 后端，原版 Vite 前端，验证 API 兼容性
- 验证 HTTP 端点响应格式与原版 TS 一致
- 验证 WebSocket 连接和消息格式

### 10.2 端到端功能测试

- 消息收发、Agent 对话、工具调用全链路
- 各频道（Telegram/Slack/Discord/WhatsApp/Signal/iMessage）基本连通
- 命令系统（/help, /status, /model 等）

### 10.3 性能基准对比

- 内存占用对比（Go vs Node.js）
- 请求延迟对比
- 并发吞吐量对比

---

## 三、Phase 10+ 遗留的延迟项

> 这些非阻塞项可在集成测试过程中穿插完成，或延至 Phase 11。

| 编号 | 内容 | 优先级 | 说明 |
|------|------|--------|------|
| DIS-F-4 | `loadWebMedia` 统一到 `pkg/media/` | 🟡 中 | 各频道有本地实现可工作，统一为代码复用 |
| SLK-P7-A | `chunkMarkdownIR` 完整 IR 管线接入 Slack/TG | 🟢 低 | 核心已实现，调用方有 TODO |
| SLK-P7-B | `files.uploadV2` 媒体上传 | 🟢 低 | 使用 legacy upload 不影响功能 |
| P7C-4 | Local Embeddings (Rust FFI) | 🟡 中 | stub 已确认，等 Phase 11 Rust |
| P7B-6 | Tailscale/Cloudflared 隧道 | 🟢 低 | 本地服务器完整 |
| P7A-3 | Security Audit 完整实现 | 🟡 中 | 骨架已有，需全模块集成后填充 |

---

## 四、关键文件路径

| 文件 | 用途 |
|------|------|
| `backend/cmd/gateway/main.go` | Go 网关入口 |
| `backend/internal/gateway/server.go` | 网关服务器核心 |
| `backend/internal/gateway/http.go` | HTTP 路由注册 |
| `backend/internal/gateway/ws.go` | WebSocket 连接管理 |
| `backend/internal/gateway/chat.go` | 聊天消息处理 |
| `ui/` | 前端 Vite + React 应用（保持不变） |
| `docker-compose.yml` | 容器编排 |

## 五、验证命令

```bash
# Go 后端编译验证
cd backend && go build ./... && go vet ./...

# Go 后端全量测试
cd backend && go test -race ./...

# 启动 Go 后端（开发模式）
cd backend && go run ./cmd/gateway/

# 启动前端
cd ui && npm run dev

# 原版 Node.js 后端（对比用）
pnpm start
```
