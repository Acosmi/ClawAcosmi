# 网关修复任务清单

**创建日期**: 2026-02-17
**审计报告**: `gateway-connectivity-audit.md`
**Bootstrap**: `gateway-fix-bootstrap.md`
**完成日期**: 2026-02-17（Batch A-D 全部完成）

---

## Batch A: P0 协议修复 ✅

- [x] **A-1**: 修复 `gateway.ts` 的 `sendConnect()` — 直接发送 `{type:"connect"}` 帧
- [x] **A-2**: 在 `gateway.ts` 的 `handleMessage()` 中增加 `hello-ok` 帧处理
- [x] **A-3**: 移除 `sendConnect()` 中对 `request()` 方法的调用
- [x] **A-4**: 验证前端 tsc 编译通过

## Batch B: P0 路由修复 ✅

- [x] **B-1**: 重构 `server.go` 路由注册 — hooks/openai/tools 路由直接注册到顶层 mux
- [x] **B-2**: 填充 `GatewayHTTPHandlerConfig` 的 `GetHooksConfig` 和 `GetAuth` 回调
- [x] **B-3**: 验证 `go build` + `go vet` + `go test` 通过

## Batch C: P1 功能修复 ✅

- [x] **C-1**: 从 `server_methods_stubs.go` 移除 `sessions.preview`
- [x] **C-2**: 修复前端 WS URL 默认值（`vite.config.ts` 添加 `/ws` 代理 + `storage.ts` 注释）
- [x] **C-3**: 验证编译通过

## Batch D: P2 体验修复 ✅

- [x] **D-1**: `server.go` 注册 `/` 路径处理器（有 ControlUIDir → 重定向 `/ui/`，否则 JSON）
- [x] **D-2**: 验证编译通过

## E2E 验证

- [ ] **E-1**: 启动后端 + 前端，浏览器确认 WS 连接成功
- [ ] **E-2**: 确认 `/health` 返回 200
- [ ] **E-3**: 确认 `/` 不再返回 404

## 文档更新

- [x] **F-1**: 更新本文件标注已修复项
- [ ] **F-2**: 按需更新 `deferred-items.md`
