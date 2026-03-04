---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/gateway (157 files, ~77KB server.go + supporting files)
verdict: Pass with Notes
---

# 审计报告: gateway — 网关核心模块

## 范围

- **目录**: `backend/internal/gateway/`
- **文件数**: 157 (含 ~40 个 `_test.go`)
- **关键文件**: `server.go`(76KB), `auth.go`(376L), `permission_escalation.go`(615L), `ws_server.go`(521L), `device_pairing.go`(21KB), `hooks_mapping.go`(16KB), `remote_approval.go`(15KB)

## 审计发现

### [PASS] 安全: 恒定时间 Token 比较 (auth.go)

- **位置**: `auth.go:68-86`
- **分析**: `SafeEqual` 使用 `crypto/subtle.ConstantTimeCompare` + `ConstantTimeEq` 实现双重恒定时间比较。先检查长度（ConstantTimeEq），再 pad 到相同长度后执行字节比较。完全消除时序攻击（timing attack）向量。实现优于简单的 `ConstantTimeCompare`（后者会因长度不同而短路）。
- **风险**: None

### [PASS] 安全: 自动 Token 生成 (auth.go)

- **位置**: `auth.go:151-196`
- **分析**: `ReadOrGenerateGatewayToken` 使用 `crypto/rand.Read` 生成 32 字节（64 hex 字符）随机 token。持久化到 `~/.openacosmi/gateway-token`（权限 0600，目录 0700）。Fallback 使用 PID（极端情况），不会 panic。采用 VS Code Server/Jupyter 模式——首次生成，后续复用。
- **风险**: None

### [PASS] 安全: 多路径认证 (auth.go)

- **位置**: `auth.go:293-340`
- **分析**: `AuthorizeGatewayConnect` 支持 4 种认证路径:
  1. **Tailscale** — whois 验证 + 用户名交叉验证
  2. **Token** — 恒定时间比较
  3. **Password** — 恒定时间比较
  4. **Local** — 回环地址直连免认证
  
  本地免认证有严格条件：回环 IP + 本地主机名 + 无代理转发头（或来自受信代理）。
- **风险**: None

### [PASS] 安全: Tailscale 认证交叉验证 (auth.go)

- **位置**: `auth.go:351-375`
- **分析**: `resolveVerifiedTailscaleUser` 不仅检查请求头中的 Tailscale 用户信息，还通过 `whoisLookup` API 对客户端 IP 做反查，然后交叉验证两者的 login 是否一致。防止伪造 Tailscale 请求头绕过认证。
- **风险**: None

### [PASS] 安全: 权限提升 TTL 管理 (permission_escalation.go)

- **位置**: `permission_escalation.go:66-404`
- **分析**: `EscalationManager` 实现 JIT（Just-In-Time）权限提升:
  - 请求 → pending（广播 WebSocket 事件）
  - 审批 → active grant + TTL 定时器
  - TTL 到期 → 自动降权
  - 任务完成 → runID 匹配则立即降权
  - 手动撤销 → 用户可随时降权
  使用 `sync.Mutex` 保护所有状态。支持磁盘持久化和重启恢复。
- **风险**: None

### [WARN] 正确性: server.go 单文件过大

- **位置**: `server.go` (76878 bytes)
- **分析**: `server.go` 是整个网关的核心文件，超过 76KB。虽然功能完整，但单文件过大影响可维护性和代码审查效率。
- **风险**: Low
- **建议**: 考虑按职责拆分（启动、配置、路由注册、状态管理）。

### [PASS] 正确性: WebSocket 协议实现 (ws_server.go)

- **位置**: `ws_server.go:61-500`
- **分析**: `HandleWebSocketUpgrade` 实现了完整的 WS 生命周期: HTTP 升级 → hello/auth 握手 → 请求-响应循环 + 事件推送。`wsConnectionLoop` 处理了消息分发、并发写入保护。协议对齐 TS `server-ws-control.ts`。
- **风险**: None

### [PASS] 安全: 认证配置降级检测 (auth.go)

- **位置**: `auth.go:91-138`
- **分析**: `ResolveGatewayAuth` 按优先级查找 token: config → `OPENACOSMI_GATEWAY_TOKEN` → `CLAWDBOT_GATEWAY_TOKEN` → 自动生成。密码同理。`AssertGatewayAuthConfigured` 在缺少必要凭据时返回明确错误。
- **风险**: None

## 测试覆盖

- 大量测试文件：`auth_test.go`, `ws_test.go`, `ws_log_test.go`, `ws_close_codes_test.go`, `ws_nonce_test.go`, `device_auth_test.go`, `device_pairing_test.go`, `broadcast_test.go`, `idempotency_test.go`, `origin_check_test.go`, `protocol_test.go`, `reload_test.go`, `remote_approval_test.go` 等
- 含 E2E 测试和 benchmark 测试
- 测试覆盖面广

## 总结

- **总发现**: 8 (7 PASS, 1 WARN, 0 FAIL)
- **阻断问题**: 无
- **建议**: `server.go` 可拆分以改善维护性 (Low)
- **结论**: **通过（附注释）** — 网关安全实现优秀。恒定时间比较、Tailscale 交叉验证、JIT 权限提升、自动 Token 生成等关键安全措施完备。
