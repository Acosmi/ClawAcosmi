> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Argus MCP 桥接接入全量任务完成汇总

- **日期**: 2026-02-25
- **编号**: shenji-argus-mcp-bridge
- **前置**: Argus go-sensory MCP Server 已就绪（18 工具，JSON-RPC 2.0 stdio）
- **任务跟踪**: `docs/claude/tracking/impl-argus-mcp-bridge-2026-02-25.md`
- **审计报告**: `docs/claude/audit/audit-2026-02-25-argus-mcp-bridge.md`
- **架构文档**: `docs/claude/goujia/arch-argus-mcp-bridge.md`

---

## 一、项目概要

### 1.1 决策背景

Argus 是独立开发的视觉理解/执行子智能体（Go+Rust 混合架构），内置完整的 MCP Server（JSON-RPC 2.0 stdio 传输，16-18 个工具）。需要将它接入主系统 OpenAcosmi 网关的心跳、网关、技能三个子系统。

**核心约束**: 零修改 Argus 代码。

### 1.2 采纳架构

```
OpenAcosmi Gateway (Go)                        Argus (Go+Rust)
┌─────────────────────────┐                    ┌──────────────────────┐
│  boot.go                │                    │  MCP Server (stdio)  │
│    └── argusBridge ─────│── stdin/stdout ────│──→ 16 工具           │
│  server_methods_argus   │    JSON-RPC 2.0    │     perception (6)   │
│    ├── argus.status     │    行分隔 10MB     │     action (7)       │
│    ├── argus.<tool>     │                    │     shell (1)        │
│    └── argus.approval   │                    │     macos (2)        │
│  mcpclient              │                    └──────────────────────┘
│    └── Client (管道管理)│
└─────────────────────────┘
```

### 1.3 方案选型

| 方案 | 描述 | 结论 |
|------|------|------|
| A | 在网关中直接调用 Argus Go API | **否决** — 引入循环依赖，且 Argus 包含 CGo/Rust FFI 不可直接 import |
| B | 通过 MCP 协议 stdio 管道通信 | **采纳** — 零修改 Argus、进程隔离、协议标准化 |
| C | 通过 HTTP/WebSocket 通信 | **否决** — 额外端口管理、不如 stdio 简洁 |

---

## 二、变更清单

### 2.1 新增文件（8 个源文件 + 5 个测试文件）

| 文件 | 行数 | 职责 |
|------|------|------|
| `backend/internal/mcpclient/types.go` | 107 | MCP JSON-RPC 2.0 协议类型（版本 `2024-11-05`） |
| `backend/internal/mcpclient/client.go` | 257 | MCP stdio 客户端：行分隔 JSON-RPC，10MB buffer，pending map |
| `backend/internal/argus/bridge.go` | 423 | Argus 进程生命周期：状态机、健康 ping、指数退避重启 |
| `backend/internal/argus/skills.go` | 145 | MCP 工具 → skillStatusEntry 转换（4 分类 × 3 风险等级） |
| `backend/internal/argus/codesign_darwin.go` | ~130 | macOS 代码签名：.app bundle 发现 + 裸二进制自动签名 |
| `backend/internal/argus/codesign_other.go` | 15 | 非 macOS no-op 存根 |
| `backend/internal/gateway/server_methods_argus.go` | ~105 | argus.* RPC 方法：静态 + 动态注册 |
| **测试文件** | | |
| `backend/internal/mcpclient/client_test.go` | ~200 | 9 用例：Initialize/ListTools/CallTool/Ping/Close/Concurrent |
| `backend/internal/argus/bridge_test.go` | ~180 | 14 用例：IsAvailable/状态机/Skills/Emoji/slogWriter |
| `backend/internal/argus/codesign_test.go` | ~80 | 4 用例：FindBundle/EnsureSigned/InsideAppBundle |
| `backend/internal/argus/integration_test.go` | ~110 | 2 集成用例：全生命周期 + 技能覆盖 |

### 2.2 修改文件（5 个）

| 文件 | 修改点 | 影响行数 |
|------|--------|----------|
| `boot.go` | +`argusBridge` 字段、`ArgusBridge()`/`StopArgus()` 访问器、`resolveArgusBinaryPath()` 四级优先级、`EnsureCodeSigned()` 兜底 | ~50 |
| `server_methods.go` | +`ArgusBridge` 到 `GatewayMethodContext`、`argus.*` 前缀权限规则 | ~15 |
| `server.go` | +`ArgusHandlers()`/动态方法注册、`StopArgus()` 在 Close 中 | ~6 |
| `ws_server.go` | +`ArgusBridge` 接入 methodCtx | ~1 |
| `server_methods_skills.go` | +Argus 技能条目追加到 `skills.status` 响应 | ~20 |

### 2.3 文档文件（4 个）

| 文件 | 类型 |
|------|------|
| `docs/claude/tracking/impl-argus-mcp-bridge-2026-02-25.md` | 任务追踪（已归档） |
| `docs/claude/audit/audit-2026-02-25-argus-mcp-bridge.md` | 代码审计报告（已归档） |
| `docs/claude/goujia/shenji-argus-mcp-bridge-full-summary.md` | 本文：完成汇总 |
| `docs/claude/goujia/arch-argus-mcp-bridge.md` | 架构设计文档 |

---

## 三、技术实现要点

### 3.1 MCP 客户端（mcpclient 包）

**设计决策**：
- 行分隔 JSON-RPC 而非 Content-Length 头 — 匹配 Argus server.go 实现
- `sync.Map` 做 pending 请求管理 — 支持并发调用
- `atomic.Int64` 单调递增 ID — 零冲突请求关联
- 10MB scanner buffer — 匹配 Argus 服务端（base64 截屏可达数 MB）

**公共 API**:
```go
NewClient(stdin, stdout) → Client
Client.Initialize(ctx) → MCPInitializeResult
Client.ListTools(ctx) → []MCPToolDef
Client.CallTool(ctx, name, args, timeout) → MCPToolsCallResult
Client.Ping(ctx) → RTT
Client.Close()
```

### 3.2 Bridge 状态机

```
                ┌──────────┐
                │   init   │
                └────┬─────┘
                     │ Start()
                ┌────▼─────┐
                │ starting │
                └────┬─────┘
          成功 /     │      \ 失败
    ┌────▼─────┐           ┌────▼─────┐
    │  ready   │←──恢复────│ degraded │
    └────┬─────┘           └────┬─────┘
         │ Stop()               │ 重启失败 × 5
    ┌────▼─────┐           ┌────▼─────┐
    │ stopped  │←──────────│ stopped  │
    └──────────┘           └──────────┘
```

**健康循环**: 每 30s MCP ping → 3 次连续失败 → degraded → 恢复后回到 ready

**进程重启**: 子进程退出 → 指数退避 1s→2s→4s→8s→16s→32s→60s（cap）→ 最多 5 次

### 3.3 动态方法注册

```go
// 从 tools/list 自动生成 RPC 方法
tools := bridge.Tools()  // ["capture_screen", "click", ...]
for _, tool := range tools {
    registry.Register("argus."+tool.Name, handler)
}
// 前端调用: ws.send({method: "argus.capture_screen", params: {quality: "vlm"}})
```

新增 Argus 工具时 **网关代码零修改** — tools/list 响应变化即自动扩展。

### 3.4 权限模型

| 方法 | 所需 Scope | 理由 |
|------|------------|------|
| `argus.status` | `operator.read` 或 `operator.write` | 只读状态查询 |
| `argus.<tool>` | `operator.write` | 工具执行有副作用 |
| `argus.approval.resolve` | `operator.write` | 审批决策（Phase 2） |

### 3.5 macOS 授权持久化

**问题**: `go build` 每次产生新哈希 → macOS TCC 按哈希追踪裸二进制授权 → 授权丢失

**解决方案（双保险）**:

| 优先级 | 方案 | 机制 |
|--------|------|------|
| A（首选） | `.app` bundle 内二进制 | TCC 按 `CFBundleIdentifier` 追踪，哈希变化无影响 |
| B（兜底） | 裸二进制自动签名 | 用 `Argus Dev` 证书 + `com.argus.sensory.mcp` identifier 签名 |

**路径解析优先级**:
```
$ARGUS_BINARY_PATH → .app bundle 搜索 → ~/.openacosmi/bin/ → PATH
```

**.app bundle 搜索路径**:
1. `{monorepo}/Argus/build/Argus.app/Contents/MacOS/argus-sensory`
2. `{monorepo}/Argus/go-sensory/Argus Sensory.app/Contents/MacOS/sensory-server`
3. `/Applications/Argus.app/Contents/MacOS/argus-sensory`
4. `~/Applications/Argus.app/Contents/MacOS/argus-sensory`
5. `~/.openacosmi/Argus.app/Contents/MacOS/argus-sensory`

---

## 四、测试矩阵

### 4.1 单元测试

| 包 | 测试数 | 覆盖范围 | 结果 |
|---|---|---|---|
| mcpclient | 9 | Initialize, ListTools, CallTool×3, Ping, ContextCancel, Close, Concurrent | 全部 PASS |
| argus (bridge) | 14 | IsAvailable×3, NewBridge, DefaultConfig, StartInvalid, StopIdempotent, CallToolNA, Skills×4, Emoji, slogWriter | 全部 PASS |
| argus (codesign) | 4 | FindBundle, EnsureNonExist, EnsureInsideBundle, IsInsideAppBundle | 全部 PASS |

### 4.2 集成测试

| 测试 | 验证点 | 结果 |
|---|---|---|
| BridgeLifecycle | MCP 握手 → 工具发现(16) → capture_screen → mouse_position → Stop | PASS |
| SkillsCoverage | 16 工具 → 16 条目，source/category/risk/eligible 全覆盖 | PASS |

### 4.3 手动验证

| 验证项 | 结果 |
|---|---|
| `go build ./...` | 零错误 |
| MCP 协议握手（直接 stdin 测试） | 成功：protocol `2024-11-05` |
| macOS codesign 方案 B | 成功：`a.out` → `com.argus.sensory.mcp`，`Argus Dev` 签名 |

---

## 五、审计结论

**审计报告**: `docs/claude/audit/audit-2026-02-25-argus-mcp-bridge.md`

**裁决**: **PASS** — 4 个低风险/信息级发现，均不阻塞归档。

| ID | 严重性 | 位置 | 描述 | 处置 |
|---|---|---|---|---|
| F-1 | 低 | client.go:64 | JSON 解析失败静默跳过 | 可接受 |
| F-2 | 低 | bridge.go:288/375 | cmd.Wait() 可能重复调用 | Go 安全 |
| F-3 | 信息 | bridge.go:182 | 握手超时硬编码 5s | stdio 够用 |
| F-4 | 低 | server_methods_argus.go:84 | `_timeout` 参数名不直观 | 文档说明 |

**合规检查**:
- Skill 3（零 panic）: ✅ 无 unwrap/panic/todo
- Skill 3（资源安全）: ✅ RAII + defer 全覆盖
- Skill 3（并发安全）: ✅ RWMutex + atomic + sync.Map
- Skill 4（审计完成）: ✅ 9 个文件逐项审计
- Skill 5（在线验证）: ✅ MCP 协议 + 工具清单已验证

---

## 六、Phase 2 待办

已录入 `docs/claude/deferred/`:

| 编号 | 待办事项 | 优先级 |
|------|----------|--------|
| D-1 | `argus.approval.resolve` 审批决策中继实现 | P1 |
| D-2 | Argus ApprovalGateway 与网关 EscalationManager 对接 | P1 |
| D-3 | 前端 Argus 工具面板 UI | P2 |
| D-4 | Argus 工具执行日志/审计追踪 | P2 |
| D-5 | 多显示器支持（capture_screen display 参数） | P3 |

---

## 七、运维指南

### 7.1 启动前置

```bash
# 方式 1: 使用 .app bundle（推荐 — 授权持久化）
cd Argus && make app
# 网关自动发现 Argus/build/Argus.app/Contents/MacOS/argus-sensory

# 方式 2: 使用裸二进制
cd Argus/go-sensory && go build -o /tmp/argus-sensory ./cmd/server
export ARGUS_BINARY_PATH=/tmp/argus-sensory
# 网关启动时自动签名（需要 Keychain 中有 "Argus Dev" 证书）

# 方式 3: 创建签名证书（首次）
cd Argus && ./scripts/package/create-dev-cert.sh
```

### 7.2 验证命令

```bash
# 检查签名证书
security find-identity -v -p codesigning | grep "Argus Dev"

# 检查二进制签名状态
codesign -dv /path/to/argus-sensory

# 运行单元测试
cd backend && go test ./internal/mcpclient/ ./internal/argus/ -v

# 运行集成测试
cd backend && ARGUS_BINARY_PATH=/path/to/argus-sensory go test ./internal/argus/ -run TestIntegration -v
```

### 7.3 日志关键字

| 日志 | 含义 |
|------|------|
| `argus: ready, tools=N` | Bridge 启动成功，发现 N 个工具 |
| `argus: degraded` | 健康检查连续 3 次失败 |
| `argus: restarting, attempt=N` | 子进程退出后自动重启 |
| `argus: signing bare binary` | 方案 B 自动签名触发 |
| `argus: using .app bundle binary` | 方案 A 找到 .app bundle |
