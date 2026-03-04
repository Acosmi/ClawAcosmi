# 网关 E2E 修复任务清单

**创建日期**: 2026-02-17
**审计报告**: `docs/renwu/gateway-e2e-audit.md`
**前置修复**: `docs/renwu/gateway-fix-task.md`（Batch A-D ✅）
**Bootstrap**: `docs/renwu/gateway-e2e-fix-bootstrap.md`

---

## Batch E: P0 字段名修复

> **Issue E2E-2**: 前端发送 `message` 字段，后端读取 `text` 字段

- [x] **E-1**: 修改 `server_methods_chat.go:174` — 增加 `message` 字段兼容读取 ✅
- [x] **E-2**: 验证 `go build ./... && go vet ./...` ✅
- [x] **E-3**: 验证 `go test -race ./internal/gateway/...` ✅
- [ ] **E-4**: 重启后端，浏览器发送消息验证后端日志 `text=` 不再为空

---

## Batch F: P1 Token 自动传递

> **Issue E2E-1**: localhost 直连 + 无 token → `4008: token_missing`
> **方案选择**: 方案 B — 后端 localhost 免认证

### 分析详情

`app-settings.ts:92-106` 已支持 `?token=` URL 参数解析并保存到 localStorage。问题在于用户首次访问 `http://localhost:5173/` 时 URL 中没有 token 参数。

3 个可选方案比较：

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. URL token 透传 | 前端已支持 | 需要用户手动输入带 token 的 URL |
| **B. localhost 免认证** ⭐ | 开发体验最好，OpenAcosmi 原版行为 | 安全性略降（仅 localhost） |
| C. 后端重定向 | 自动化 | 需要修改 server.go 和前端参数传递 |

**推荐方案 B**：在 `AuthorizeGatewayConnect` 中增加 localhost 直连检测 + 自动放行。前端通过 Vite 代理访问时，remote addr 为 `127.0.0.1`，符合本地直连条件。

### 修复步骤

- [x] **F-1**: 修改 `auth.go:AuthorizeGatewayConnect` — localhost 免认证 ✅
- [x] **F-2**: 确认 `ws_server.go:147` 已正确传递 `Req: r` ✅
- [x] **F-3**: 补充 `auth_test.go` — `TestAuthorizeGatewayConnect_LocalDirect` ✅
- [x] **F-4**: 验证 `go build && go vet && go test -race` — 全部通过 ✅
- [ ] **F-5**: 重启后端，浏览器访问验证无 `token_missing` 错误

---

## Batch G: 依赖验证（E2E-4 + E2E-3）

> **Issue E2E-4**: WS 频繁重连 (~15s 间隔)
> **Issue E2E-3**: 用户消息不在聊天框显示
> 这两个问题很可能是 E2E-1 的副作用。修复 token 传递后需重新验证。

- [ ] **G-1**: Batch E+F 完成后，重启后端 + 前端
- [ ] **G-2**: 观察后端日志 — 确认不再有频繁 `ws: new connection` 日志（E2E-4 验证）
- [ ] **G-3**: 发送聊天消息 — 确认用户消息立即显示在聊天框（E2E-3 验证）
- [ ] **G-4**: 发送聊天消息 — 确认 AI 回复正确显示（E2E-2 验证）
- [ ] **G-5**: 观察 15 分钟 — 确认 WS 连接稳定（无频繁断开/重连）

### E2E-4 未自愈的后备方案

如果 WS 重连循环在修复 token 后仍然存在：

- [ ] **G-6**: 检查 Vite HMR WS 与应用 WS 是否冲突
- [ ] **G-7**: 检查 `gateway.ts:376-384` 的 `queueConnect` 750ms 延迟是否导致超时
- [ ] **G-8**: 在 `ws_server.go` 增加心跳 ping/pong 机制

### E2E-3 未自愈的后备方案

如果用户消息不显示问题在修复后仍然存在：

- [ ] **G-9**: 检查 `chat.ts:92-99` 的 `chatMessages` 追加逻辑
- [ ] **G-10**: 检查 `app-gateway.ts:132-147` 的 `onHello` 回调是否覆盖消息
- [ ] **G-11**: 检查聊天视图组件的渲染逻辑

---

## 文档更新

- [ ] **H-1**: 更新本文件标注已修复项
- [ ] **H-2**: 更新 `gateway-e2e-audit.md` 添加修复记录
- [ ] **H-3**: 更新 `phase10-task.md` 更新 E2E 验证状态
- [ ] **H-4**: 按需更新 `deferred-items.md`

---

## 验证命令速查

```bash
# 后端编译验证
cd backend && go build ./... && go vet ./...

# 后端测试
cd backend && go test -race ./internal/gateway/...

# 前端编译验证
cd ui && npx tsc --noEmit

# E2E 连通性测试（手动）
# 1. 启动后端: DEEPSEEK_API_KEY=... go run ./cmd/acosmi/
# 2. 启动前端: npm run dev
# 3. 浏览器访问 http://localhost:5173 检查 WS 连接成功
# 4. 发送消息，确认 AI 正确回复
```
