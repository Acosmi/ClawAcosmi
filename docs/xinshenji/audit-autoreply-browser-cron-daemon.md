---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/{autoreply, browser, cron, daemon} (~3500+ LOC core)
verdict: Pass with Notes
---

# 审计报告: autoreply/browser/cron/daemon — 自动回复与系统服务层

## 范围

- `backend/internal/autoreply/` — 50 files + reply/ subdir (自动回复/命令/分发)
- `backend/internal/browser/` — 31 files (CDP浏览器控制/Playwright)
- `backend/internal/cron/` — 22 files (定时任务/隔离Agent)
- `backend/internal/daemon/` — 27 files (守护进程/launchd/systemd)

## 审计发现

### [PASS] 安全: 命令授权体系 (autoreply/command_auth.go)

- **位置**: `command_auth.go:30-76`
- **分析**: `ResolveCommandAuthorization` 实现三层授权:
  1. Bot 所有者始终授权
  2. 拒绝列表优先（大小写不敏感匹配）
  3. 允许列表约束 → 私聊默认放行，群组默认拒绝
- **风险**: None

### [PASS] 正确性: DI 驱动的分发架构 (autoreply/dispatch.go)

- **位置**: `dispatch.go:1-173`
- **分析**: 3 种分发模式（普通/带分发器/带缓冲分发器）均通过 DI 函数签名注入 reply 包实现，避免循环导入。`WaitForIdle` 确保异步分发完成后再返回。
- **风险**: None

### [PASS] 正确性: 命令注册表与文本别名 (autoreply/commands_registry.go)

- **位置**: `commands_registry.go:1-656`
- **分析**: 全局命令注册表支持: 命令注册/查找/过滤、文本别名缓存（`sync.Once` + mutex）、命令参数解析/序列化、配置驱动的命令启用控制。`config/debug/bash` 命令默认禁用需显式开启。
- **风险**: None

### [PASS] 安全: Browser DOM 遍历参数限制 (browser/cdp.go)

- **位置**: `cdp.go:267-279, 416-433`
- **分析**: `SnapshotDom` 限制: limit ∈ (0, 5000], maxTextChars ∈ (0, 5000]。`QuerySelector` 限制: limit ∈ (0, 200], maxTextChars ∈ (0, 5000], maxHtmlChars ∈ (0, 20000]。有效防止恶意页面导致的内存爆炸。
- **风险**: None

### [PASS] 正确性: CDP WebSocket URL 规范化 (browser/cdp.go)

- **位置**: `cdp.go:44-98`
- **分析**: `NormalizeCdpWsURL` 处理 4 种场景: 回环→远程重映射、协议升级(ws→wss)、凭据继承、查询参数合并。覆盖与远程浏览器实例通信的所有 URL 适配场景。
- **风险**: None

### [PASS] 正确性: Cron 隔离 Agent 运行器 (cron/isolated_agent.go)

- **位置**: `isolated_agent.go:149-507`
- **分析**: `RunCronIsolatedAgentTurn` 编排完整的 cron agent 生命周期: 会话创建→模型解析→prompt 构建→runner 执行→结果投递→日志记录。所有外部依赖通过 `IsolatedAgentDeps` DI 注入，高度可测试。
- **风险**: None

### [PASS] 正确性: Daemon 服务审计 (daemon/audit.go)

- **位置**: `daemon/audit.go:13-319`
- **分析**: `AuditGatewayServiceConfig` 检查 5 个维度: gateway 子命令存在性、PATH 最小化、运行时版本(Bun/NVM node)、systemd unit 配置(After/Wants/RestartSec)、launchd plist 配置。跨平台支持 (macOS/Linux/Windows)。
- **风险**: None

### [WARN] 正确性: 全局命令注册表非线程安全 (autoreply/commands_registry.go)

- **位置**: `commands_registry.go:14-20`
- **分析**: `globalCommands` 是一个普通 map，`RegisterCommand` 无锁保护。虽然命令注册通常在 init() 期间完成（单线程），但如果在运行时动态注册则存在数据竞争风险。
- **风险**: Low
- **建议**: 添加 `sync.Mutex` 或使用 `sync.Map`。

### [WARN] 正确性: Browser HealthCheck 使用 DefaultClient (browser/cdp.go)

- **位置**: `cdp.go:200`
- **分析**: `HealthCheck` 使用 `http.DefaultClient`，无超时设置。如果 CDP 端点无响应，可能阻塞调用者。
- **风险**: Low
- **建议**: 使用带超时的自定义 Client 或依赖 ctx 超时。

## 总结

- **总发现**: 9 (7 PASS, 2 WARN, 0 FAIL)
- **阻断问题**: 无
- **结论**: **通过（附注释）** — 命令授权体系、DI 分发架构、CDP 参数限制均设计良好。
