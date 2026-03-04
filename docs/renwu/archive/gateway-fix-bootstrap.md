# 网关修复 — 新窗口 Bootstrap

**创建日期**: 2026-02-17
**前置文档**: `docs/renwu/gateway-connectivity-audit.md`（含 8 个 Issue 的完整审计报告）

---

## 新窗口启动模板

> 复制以下内容到新窗口作为首条消息：

```
我需要修复网关连通性问题。请按以下步骤操作：

1. 读取 `docs/renwu/gateway-fix-bootstrap.md`（本文件）
2. 读取 `docs/renwu/gateway-fix-task.md`（任务清单）
3. 读取 `docs/renwu/gateway-connectivity-audit.md`（审计报告，重点看"全局审计发现"部分的 Issue 3-8）
4. 按照 task.md 中的 Batch 顺序逐批修复
5. 遵循 /refactor 工作流的验证和文档更新步骤
```

---

## 项目概况

- **后端**: Go，入口 `backend/cmd/acosmi/main.go`，核心在 `backend/internal/gateway/`
- **前端**: TypeScript/Lit，入口 `ui/src/ui/app.ts`，WS 客户端在 `ui/src/ui/gateway.ts`
- **网关端口**: 18789（可配置）
- **前端端口**: 5173（Vite dev server）
- **运行方式**: 后端 `go run ./cmd/acosmi/`；前端 `npm run dev`

---

## 修复优先级与分批策略

### Batch A: P0 协议修复（阻断性，必须最先修复）

**Issue 3** — WS connect 帧类型不匹配

- `ui/src/ui/gateway.ts` 的 `sendConnect()` 用 `request("connect", params)` 发送 `{type:"req"}`
- 后端 `ws_server.go:118` 期望首帧 `{type:"connect"}`
- **修复**: 新增 `sendRawConnect(params)` 方法，直接 `ws.send(JSON.stringify({type:"connect", ...params}))`

**Issue 7** — hello-ok 响应不走 pending map（与 Issue 3 联动）

- 后端返回 `{type:"hello-ok"}` 不是 `{type:"res"}`
- **修复**: 在 `handleMessage()` 中增加 `type === "hello-ok"` 分支，直接调用 `onHello` 回调

### Batch B: P0 路由修复

**Issue 4** — HTTP 路由嵌套 + nil 回调

- `server.go:221-222` 把 `CreateGatewayHTTPHandler` 挂在 `/hooks/` 下，导致路由双重嵌套
- `GetHooksConfig` 和 `GetAuth` 回调为 nil → panic
- **修复**: 将 hooks/openai/tools 路由直接注册到 `server.go` 顶层 mux，不再嵌套

### Batch C: P1 功能修复

**Issue 5** — Stub 覆盖 `sessions.preview`

- `server_methods_stubs.go` 中移除 `sessions.preview`

**Issue 8** — 前端 WS URL 默认指向 Vite 端口

- `storage.ts` 或 `vite.config.ts` 添加代理/默认 URL

### Batch D: P2 体验修复

**Issue 6** — 根路径 `/` 返回 404

- `server.go` 注册 `/` 路径

---

## 关键文件速查

| 文件 | 修改内容 |
|------|----------|
| `ui/src/ui/gateway.ts` | Batch A: 重写 sendConnect + handleMessage |
| `backend/internal/gateway/server.go` | Batch B/D: 路由重构 + `/` handler |
| `backend/internal/gateway/server_http.go` | Batch B: 对应调整或拆分 |
| `backend/internal/gateway/server_methods_stubs.go` | Batch C: 移除重复方法 |
| `ui/src/ui/storage.ts` 或 `ui/vite.config.ts` | Batch C: WS URL 修复 |

---

## 验证方法

每个 Batch 完成后：

```bash
# 后端编译验证
cd backend && go build ./... && go vet ./...

# 后端测试
cd backend && go test -race ./internal/gateway/...

# 前端构建验证
cd ui && npx tsc --noEmit

# E2E 连通性测试（手动）
# 1. 启动后端: DEEPSEEK_API_KEY=... go run ./cmd/acosmi/
# 2. 启动前端: npm run dev
# 3. 浏览器访问 http://localhost:5173 检查是否显示 "Connected"
```
