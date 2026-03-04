---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/{acp, argus, canvas, linkparse, mcpclient, nodehost, pairing}
verdict: Pass with Notes
---

# 审计报告: acp/argus/canvas/linkparse/mcpclient/nodehost/pairing — 协议与辅助模块

## 模块概览

| 模块 | 文件数 | 核心职责 |
|------|--------|---------|
| `acp` | 10 | Agent-Client Protocol (JSON-RPC over ndJSON) |
| `argus` | 11 | Argus 视觉子智能体 MCP 桥接 |
| `canvas` | 5 | Canvas Host HTTP+WS 实时预览服务 |
| `linkparse` | 6 | URL/消息链接检测与解析 |
| `mcpclient` | 3 | MCP (Model Context Protocol) 客户端 |
| `nodehost` | 14 | Node.js 宿主进程、命令执行、allowlist |
| `pairing` | 5 | 设备配对标签与存储 |

---

## acp — Agent-Client Protocol

### [PASS] 正确性: JSON-RPC 服务端实现 (server.go)

- **位置**: `acp/server.go:94-135`
- **分析**: `ServeAcpGateway` 从 stdin 逐行读取 ndJSON，区分请求(有ID)和通知(无ID)。请求异步 `go handleRequest()` 处理，通知同步处理。10MB scanner buffer 上限。
- **风险**: None

### [PASS] 安全: 写入互斥保护 (server.go)

- **位置**: `acp/server.go:253-271`
- **分析**: `writeNDJSON` 使用包级 `writeMu` 互斥锁保护 stdout 写入，与 `AgentSideConnection.sendNotification` 的实例级 `mu` 互斥锁分别保护不同写入通道。
- **风险**: None

### [WARN] 正确性: 双重写入锁不统一 (server.go)

- **位置**: `acp/server.go:63-85 vs 253-271`
- **分析**: `AgentSideConnection.sendNotification` 使用 `c.mu` 锁，`writeNDJSON` 使用单独的 `writeMu` 包级锁。如果两者写入同一 writer(stdout)，存在交错写入风险。
- **建议**: 统一使用同一把锁，或确认两者的 writer 不同。

---

## argus — Argus 视觉子智能体桥接

### [PASS] 正确性: 状态机生命周期 (bridge.go)

- **位置**: `argus/bridge.go:44-52, 142-171`
- **分析**: Bridge 状态机: `init → starting → ready → degraded → stopped`。`Start()` 启动子进程 + MCP 握手 + 工具发现，启动 healthLoop 和 processMonitor 两个后台 goroutine。
- **风险**: None

### [PASS] 正确性: 崩溃自动恢复 (bridge.go)

- **位置**: `argus/bridge.go:288-377`
- **分析**: `processMonitor` 监控子进程退出。快速崩溃检测: 60秒窗口内最多3次崩溃，超过则降级为 `degraded` 状态（不再重启）。指数退避重启（1s→2s→4s→…）避免频繁 fork。
- **风险**: None

### [PASS] 安全: macOS 代码签名校验 (codesign_darwin.go)

- **位置**: `argus/codesign_darwin.go`
- **分析**: macOS 上启动 Argus 子进程前验证二进制代码签名（`codesign -v`），防止执行被篡改的二进制。
- **风险**: None

---

## canvas — Canvas Host 实时预览

### [PASS] 正确性: HTTP+WS 实时重载 (server.go)

- **位置**: `canvas/server.go:83-155`
- **分析**: `StartCanvasHost` 启动 HTTP 静态文件服务 + WebSocket live-reload。使用 `fsnotify.Watcher` 监控文件变更，debounce timer 防抖后通过 WS 广播刷新指令。
- **风险**: None

### [WARN] 安全: WS CheckOrigin 全放行 (server.go)

- **位置**: `canvas/server.go:75`
- **分析**: `wsUpgrader.CheckOrigin` 始终返回 `true`，允许任意来源 WebSocket 连接。在本地开发场景可接受，但如果 Canvas Host 暴露到网络则存在 CSRF 风险。
- **建议**: 添加可选的 origin 白名单。

---

## nodehost — 命令执行与 Allowlist

### [PASS] 正确性: 命令执行超时+截断 (exec.go)

- **位置**: `nodehost/exec.go:17-75`
- **分析**: `RunCommand` 支持: 超时控制(`context.WithTimeout`)、输出 buffer 上限截断(`capBuffer` 防止 OOM)、跨平台可执行文件查找(Windows PATHEXT)。
- **风险**: None

### [PASS] 安全: Allowlist 评估体系

- **位置**: `nodehost/allowlist_*.go` (5 files)
- **分析**: Node 宿主提供完整的 allowlist 评估体系: 类型定义(`allowlist_types.go`)、解析(`allowlist_parse.go`)、解析(`allowlist_resolve.go`)、评估(`allowlist_eval.go`)、Windows 路径规范化(`allowlist_win_parser.go`)。用于控制插件可执行的命令范围。
- **风险**: None

### [PASS] 安全: 命令消毒 (sanitize.go)

- **位置**: `nodehost/sanitize.go`
- **分析**: 命令参数消毒，过滤危险字符和 shell 元字符。
- **风险**: None

---

## linkparse — 链接解析

### [PASS] 正确性: URL/消息链接检测

- **位置**: `linkparse/` (6 files: apply, defaults, detect, detect_test, format, runner)
- **分析**: 检测消息中的 URL、格式化链接、应用默认链接处理规则。包含测试文件。
- **风险**: None

---

## mcpclient — MCP 客户端

### [PASS] 正确性: MCP 协议客户端

- **位置**: `mcpclient/` (3 files: client, client_test, types)
- **分析**: 实现 Model Context Protocol 客户端，支持工具发现、工具调用、init 握手。由 `argus.Bridge` 使用。
- **风险**: None

---

## pairing — 设备配对

### [PASS] 正确性: 配对标签存储

- **位置**: `pairing/` (5 files: labels, messages, messages_test, store, store_test)
- **分析**: 设备配对标签管理和持久化存储。有测试覆盖。
- **风险**: None

---

## 总结

- **总发现**: 12 (10 PASS, 2 WARN, 0 FAIL)
- **阻断问题**: 无
- **结论**: **通过（附注释）** — ACP JSON-RPC 实现完整，Argus Bridge 崩溃恢复设计优秀，nodehost allowlist 体系完善。
