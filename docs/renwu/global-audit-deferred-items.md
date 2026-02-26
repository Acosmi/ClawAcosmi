# Deferred Items 全局审计报告

> 审计日期：2026-02-20 | 审计窗口：W-Deferred (文档真实性审计)

## 概览

本次审计专门针对 `docs/renwu/deferred-items.md` 中提及的遗漏/推迟项，通过与 Go 后端代码的实际状态进行逐文件对照，验证了文档表述的真实性。

| 维度 | 审查项数 | 结论状态 |
|------|---------|----------|
| 总检查项 | 34 | - |
| **存在偏差 (已实现)** | **9** | **需从推迟列表清理** |
| 表述准确 (确为存根) | 25 | 继续保留 |

---

## 逐文件真实性复核

### 🔴 发现的文档滞后（代码已实现，文档未更新）

1. **P11-1: Ollama 本地 LLM 集成** `[代码已实现]`
   - **涉及文件**: `backend/internal/agents/llmclient/ollama.go`, `client.go`
   - **实际状态**: 代码中包含了完整的 `ollamaStreamChat` 实现（底层调用 HTTP API `/api/chat`，解析了 NDJSON 的流式响应与所有用量 Token），并且已经在 `client.go` 路由器中完全接入了 `"ollama"` Provider。功能已完备，非推迟状态。

2. **Phase 13 exec_tool.go GAP-1 到 GAP-8** `[底层全部已修补]`
   这些关于安全执行网关与架构不符的 Bug 实际上均已经在代码中解决：
   - **GAP-1**: `executeNode` / `executeGateway` 均非同步死锁，已经能通过 goroutine 并立即通过 `Status: "approval-pending"` 响应异步挂起。
   - **GAP-2**: Node 的 `buildInvokeParams` 明确封装了 `rawCommand`, `agentId`, `approved`, `approvalDecision`, `runId`, `idempotencyKey` 这些字段 (L928)。
   - **GAP-3**: `supportsSystemRun` 明确在 Node 发送前进行了权限能力扫描 (L831)。
   - **GAP-4**: Node 的 payload 有进行准确的 `.payload` 层级解构并校验了 `error`/`success` (L1084)。
   - **GAP-5 / GAP-6**: `getWarningText(&warnings)` 前缀已经加到了所有返回中，且 `EmitExecSystemEvent` 在整个工作流中大量铺设。
   - **GAP-7 / GAP-8**: Deny 检查均已完备；`onAbortSignal` (经由 `context.Done`) 明确封装了 `if !handle.Session.Backgrounded { handle.Kill() }`。

### 🟢 验证正确的真实推迟项

通过逐个文件探查，以下推迟项切实处于缺失或存根(Stub)状态，文档描述完全真实：

- **测试与测试架缺失项**
  - **BW1-D1**: Sandbox / Cost 目录中未找到任何对应的 `*_test.go`。
  - **TUI-D1**: `resolveGatewayConnection` `(tui/gateway_ws.go)` 确实在硬编码调用 `config.NewConfigLoader()`。
- **业务逻辑存根与外部依赖缺失**
  - **BW1-D3**: Auth 取值确实仍用简化的 `readAuthProfile` `(cost/provider_auth.go)`。
  - **W5-D2 / W5-D3**: `cmd/` 下确实完全没有 `memory-cli` 和 `logs-cli` 等效的具体实现。
  - **W5-D1**: Windows PID 检测继续沿用了低效的 `exec.Command("tasklist")` 外挂调用，未用 OpenProcess 等原生 API。
  - **HEALTH-D1**: Skills安装依然报错 `ErrCodeNotImplemented`。
  - **HEALTH-D4 / D6**: 图片工具缩放缺失 `imaging`，LINE 接入也只有基本上下文结构而没有发转收逻辑。
- **高阶功能实现留白**
  - **PHASE5-3**: `bonjour.go` 确实只做了接口，注明需要外挂 `grandcat/zeroconf`。
  - **PHASE5-2**: OpenAI API 代理未实现深度的 `file` / `tools` SSE 回推等高级内容。
  - **PHASE5-5**: 后端 Go `pw_tools_cdp.go` 采用了原生 CDP 实现替代 `playwright-go`，因此依然缺失了直接操纵 Playwright 客户端或高级视觉引导的能力（虽然 TS 端已有部分实现）。

---

## 隐藏依赖审计复核

(由于核心的 8 项隐藏依赖主要为配置读取、三方 SDK 和底层常量，经交叉引用确认大部分均作为 P2/P3 继续遗留，符合长期推迟特性)

## 差异清单与总结

- **虚假 推迟项**: 9 项
- **真实 推迟项**: 25 项
- **模块审计评级**: A (文档大部分保持极高质量的同步，仅 exec_tool 经历过重构没有同步变更)

### 后续建议

1. 在 `deferred-items.md` 中将 **P11-1** 以及 **GAP-1 到 GAP-8** 修改为已解决 (可以移入 `archive/deferred-items-completed.md`)。
2. 保持对已探明的 25 项真实推迟项的优先级跟踪。
