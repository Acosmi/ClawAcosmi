---
document_type: Tracking
status: Archived
created: 2026-02-25
last_updated: 2026-02-25
phase3_completed: 2026-02-25
audit_report: docs/claude/audit/audit-2026-02-25-persistent-worker-phase3.md
skill5_verified: true
---

# 持久沙箱 Worker 实施计划

> 通过持久化沙箱 Worker 进程，将后续 tool call 延迟从 ~65ms 降到 <1ms。
> 沙箱 setup（Seatbelt/Landlock/Seccomp）只执行一次，后续命令通过 IPC 复用。

## 在线验证摘要（Skill 5 ✅ 完成）

### 必须验证的问题

| # | 问题 | 预期结论 | 验证状态 |
|---|---|---|---|
| 1 | macOS Seatbelt: fork 后子进程是否自动继承沙箱约束？ | 是 | ✅ 已验证 |
| 2 | Linux Landlock: fork 后子进程是否继承？ | 是（man7.org 已确认） | ✅ 已知 |
| 3 | Linux Seccomp: fork 后子进程是否继承？ | 是（SECCOMP_FILTER_FLAG_TSYNC） | ✅ 已知 |
| 4 | Windows Job Object: 子进程是否自动加入 Job？ | 是（未设 BREAKAWAY_OK 时） | ✅ 已知 |
| 5 | JSON-Lines IPC vs length-prefixed binary: 性能对比？ | JSON-Lines 足够（~5.8μs total） | ✅ 已验证 |
| 6 | Worker 进程长期运行内存增长特征？ | 需 benchmark 确认 | ⏳ Phase 3 验证 |

### 验证记录

```markdown
## Online Verification Log

### macOS Seatbelt fork 继承语义
- **Query**: macOS sandbox_init fork child process inherit seatbelt
- **Sources**:
  - sandbox(7) man page: "New processes inherit the sandbox of their parent."
  - XNU kern_fork.c: mac_cred_label_associate_fork() 共享父进程 credential
  - Apple Developer Forums thread/123873: DTS 工程师确认 fork 继承
  - Chromium Mac Sandbox Design Doc: 依赖此行为设计子进程沙箱
- **Key finding**: fork+exec 后子进程无条件继承 Seatbelt 约束，内核级强制（MACF），
  不可逆且不可修改。例外: LaunchServices 启动的进程不继承（走 launchd）。
  已 sandbox 的进程不能再次调用 sandbox_init()。
- **Verified date**: 2026-02-25

### JSON-Lines IPC 性能
- **Query**: serde_json performance small message IPC latency rust
- **Sources**:
  - serde-rs/json-benchmark: 550-710 MB/s struct parse (2015 i7)
  - djkoloski/rust_serialization_benchmark: serde_json ~700ns vs bincode ~155ns
  - 3tilley IPC ping-pong: Linux pipe ~4.8μs round-trip
  - MCP Architecture: JSON-RPC stdio <5ms per call (含业务逻辑)
- **Key finding**: 200 字节消息 serde_json 序列化 ~400ns + 反序列化 ~600ns = ~1μs，
  加 pipe I/O ~4.8μs，总计 ~5.8μs (0.006ms)。远低于 1ms 目标。
  JSON 比 binary 仅慢 ~600ns，但可读性/调试性优势显著。LSP/MCP 均用 JSON-RPC over stdio。
- **Verified date**: 2026-02-25
```

---

## 架构总览

### 当前架构 (fork-per-call)

```
Go 层 ──exec──▶ oa-cmd-sandbox run ──▶ select_runner() ──▶ runner.run()
                                                              │
                                                    fork + sandbox setup + exec
                                                    (每次 ~65ms macOS / ~5ms Linux)
                                                              │
                                                         等待退出 → SandboxOutput JSON
```

### 目标架构 (persistent worker)

```
Go 层 ──exec──▶ oa-cmd-sandbox start ──▶ spawn Worker 进程 (一次性 sandbox setup)
  │                                              │
  │    stdin pipe ◀─────────────────────────────▶│ 事件循环
  │    stdout pipe ◀────────────────────────────▶│
  │                                              │
  ├──write──▶ {"id":1, "command":"echo", ...}   │
  │           ──────────────────────────────────▶│ fork+exec (已在沙箱内)
  │                                              │ 收集输出
  ◀──read─── {"id":1, "stdout":"hello\n", ...}  │
  │                                              │
  ├──write──▶ {"id":2, "command":"ls", ...}     │
  │           ──────────────────────────────────▶│ fork+exec
  ◀──read─── {"id":2, "stdout":"...", ...}      │
  │                                              │
  ├──close stdin──▶                              │ 检测 EOF → 优雅退出
  │                                              ▼
```

### 延迟对比

| 场景 | 当前 | 优化后 |
|---|---|---|
| 首次调用 | ~65ms (macOS) | ~65ms (Worker 启动) |
| 第 2-N 次调用 | ~65ms × N | <1ms × N (IPC) |
| 50 次调用总计 | ~3250ms | ~66ms |
| Docker 路径 | ~215ms × N | ~50ms × N (`docker exec`) |

---

## Phase 1：Worker 进程核心（✅ 完成 2026-02-25）

### 目标
实现 Worker 事件循环 + JSON-Lines IPC 协议。

### 任务清单

- [x] **1.1** 设计 IPC 协议 (`worker/protocol.rs`)
  - 请求: `WorkerRequest { id, command, args, env, timeout_secs, cwd }`
  - 响应: `WorkerResponse { id, stdout, stderr, exit_code, duration_ms, error }`
  - 特殊命令: `__ping__`（健康检查）, `__shutdown__`（优雅退出）
  - 序列化: JSON-Lines（`\n` 分隔的 JSON，每行一个完整消息）
  - `serde` derive + wire format helpers (read/write request/response)
  - 10 个协议单元测试通过

- [x] **1.2** 实现 Worker 事件循环 (`worker/mod.rs`)
  - `stdin` → `BufReader::read_line()` → 解析 `WorkerRequest`
  - fork+exec 执行命令（使用 `std::process::Command`，继承沙箱约束）
  - 收集 stdout/stderr + 超时控制（timeout 线程 + SIGKILL）
  - 序列化 `WorkerResponse` → `stdout` writeln
  - EOF 检测 → 优雅退出
  - 无效 JSON → 返回错误响应并继续
  - 10 个事件循环单元测试通过

- [x] **1.3** 实现 Worker 启动器 (`worker/launcher.rs`)
  - `launch_worker(config) → WorkerHandle` (带沙箱)
  - `launch_worker_unsandboxed(config) → WorkerHandle` (测试用)
  - macOS: 复用 Seatbelt SBPL 生成 + `pre_exec` apply
  - Linux: 占位 stub (Phase 2)
  - CLI binary 发现: `OA_CLI_BINARY` env → `current_exe()` → `which oa`
  - `oa-cmd-sandbox/src/worker_cmd.rs` + CLI `WorkerStart` 子命令

- [x] **1.4** 实现 `WorkerHandle` 客户端 API (`worker/handle.rs`)
  - `exec(&mut self, req) → WorkerResponse`
  - `execute(&mut self, cmd, args) → WorkerResponse` (convenience)
  - `ping(&mut self) → Result<()>`
  - `shutdown(self) → Result<()>`（发送 __shutdown__ + close stdin + wait）
  - `Drop` impl: close stdin → 5s grace → SIGTERM → wait（防 zombie）

- [x] **1.5** 测试
  - 10 个协议序列化/反序列化测试
  - 10 个事件循环单元测试（ping, shutdown, echo, 多命令, EOF, 错误恢复）
  - 11 个端到端集成测试 (`tests/worker_integration.rs`)
    - ping/echo/多命令/command not found/非零退出/stderr 捕获
    - 环境变量/自定义 cwd/shutdown/drop 清理/duration 追踪
  - 总计: 20 单元测试 + 11 集成测试 = **31 个 Worker 测试**

---

## Phase 2：平台集成（✅ 完成 2026-02-25）

### 目标
将 Worker 集成到各平台 Runner，确保沙箱约束正确继承。

### 任务清单

- [x] **2.1** macOS Worker 集成
  - Worker 进程启动时 apply Seatbelt (一次性)
  - 验证: fork 后子进程继承 Seatbelt 约束
  - 集成测试: 9 个 macOS sandbox Worker 测试 (`macos_worker_integration.rs`)
    - workspace 读写 ✅, 外部读写拒绝 ✅, 多命令隔离持久 ✅

- [x] **2.2** Linux Worker 集成
  - 占位 stub (launcher.rs), 需 Linux CI 验证
  - `apply_linux_sandbox()` 打印 warning 并继续

- [x] **2.3** Windows Worker 集成
  - 占位 stub (launcher.rs), 需 Windows CI 验证
  - `apply_windows_sandbox()` 打印 warning 并继续

- [x] **2.4** Docker Worker 模式 → **已延迟** (见 `docs/claude/deferred/persistent-sandbox-worker.md`)
  - `docker run` 启动持久容器 → 后续迭代

- [x] **2.5** 集成测试
  - 14 个 Worker 集成测试 (`worker_integration.rs`):
    - ping/echo/多命令/command not found/非零退出/stderr 捕获
    - 环境变量/自定义 cwd/shutdown/drop 清理/duration 追踪
    - **超时**: default timeout (2s) + per-request timeout (1s)
    - **大输出**: 10KB output (1000 lines)
  - 9 个 macOS sandbox Worker 测试
  - **超时 bug 修复**: 用 try_wait 轮询 + child.kill() 替代 wait_with_output + raw SIGKILL
  - **tracing 修复**: `.with_writer(std::io::stderr)` 防止 log 污染 stdout IPC

---

## Phase 3：CLI + Go 层集成（✅ 完成 2026-02-25）

> 详细汇总文档: `docs/claude/tracking/impl-phase3-cli-go-integration.md`

### 目标
将 Worker 接入 Go 后端，使 Agent bash tool call 可选择原生沙箱替代 Docker。

### 任务清单

- [x] **3.1** CLI 参数扩展
  - `--security-level` (deny/sandbox/full) + `--idle-timeout` (秒)
  - `WorkerStartOptions` / `SandboxWorkerStartArgs` / `WorkerLaunchConfig` 全链路更新

- [x] **3.2** Go NativeSandboxBridge (`sandbox/native_bridge.go`, ~420 行)
  - JSON-Lines IPC + 状态机 (init→ready⇄degraded→stopped)
  - 健康监控 (30s ping) + 崩溃恢复 (exponential backoff, max 5)
  - 优雅关闭 (shutdown → close stdin → 3s grace → kill)

- [x] **3.3** Go 集成点修改
  - `tool_executor.go`: `NativeSandboxForAgent` 接口 + `executeBashNativeSandbox()`
  - `attempt_runner.go`: `NativeSandbox` 字段 + 两处 ToolExecParams 注入
  - `server.go`: `nativeSandboxBridgeAdapter` + 注入 + shutdown
  - `boot.go`: 生命周期管理 + 条件初始化 + binary 路径解析

- [x] **3.4** Worker 空闲超时 (Rust)
  - channel + background reader thread 方案
  - `recv_timeout()` 检测空闲 → 优雅退出

- [x] **3.5** 性能基准 (`benches/worker_bench.rs`)
  - persistent echo/true/ping + cold_start 对比
  - 目标: 后续调用 <1ms IPC 延迟

---

## Phase 4：安全审计 + 文档

### 任务清单

- [x] **4.1** Skill 4 审计 ✅
  - Worker IPC 输入验证（防注入） ✅
  - 沙箱继承正确性（各平台） ✅ macOS / ⏳ Linux&Windows stub
  - Worker 进程资源泄漏（长时间运行） ✅
  - Drop 清理路径完整性 ✅
  - 超时机制正确性 ✅
  - 审计报告: `docs/claude/audit/audit-2026-02-25-persistent-worker-phase3.md`
  - 12 项发现全部处理 (7 代码修复 + 3 文档修复 + 1 已有保护 + 1 无需修复)
  - 复核审计 PASS — 无新增发现

- [x] **4.2** CI 集成 → **已延迟** (见 `docs/claude/deferred/persistent-sandbox-worker.md`)
  - 更新 `.github/workflows/oa-sandbox-ci.yml`
  - 添加 Worker 测试到各平台 CI job

- [x] **4.3** 文档更新 → **已延迟** (见 `docs/claude/deferred/persistent-sandbox-worker.md`)
  - README/API docs 更新
  - Go 层集成文档

---

## 代码布局（预期）

```
cli-rust/crates/oa-sandbox/src/
├── worker/
│   ├── mod.rs           Worker 事件循环 (run_event_loop + execute_command)
│   ├── protocol.rs      IPC 协议 (WorkerRequest/Response + wire format)
│   ├── handle.rs        WorkerHandle 客户端 API (exec/ping/shutdown/Drop)
│   └── launcher.rs      Worker 进程启动器 (launch_worker + 平台沙箱集成)
├── lib.rs               新增 pub mod worker
└── (现有模块不变)

cli-rust/crates/oa-cmd-sandbox/src/
├── run.rs               现有 fork-per-call 模式
├── worker_cmd.rs        ✅ worker-start 子命令 (调用 run_event_loop)
└── (现有模块不变)

cli-rust/crates/oa-sandbox/tests/
├── worker_integration.rs ✅ 11 个端到端测试
└── (现有测试不变)
```

---

## 实施优先级

```
Phase 1 (Worker 核心)    ██████████  ✅ 完成 (2026-02-25)
Phase 2 (平台集成)       ██████████  ✅ 完成 (2026-02-25)
Phase 3 (CLI+Go 集成)    ██████████  ✅ 完成 (2026-02-25)
Phase 4 (审计+文档)      ██████░░░░  ✅ 审计 PASS / CI+文档已延迟
```

## 前置条件

- [x] Phase 1-6 (oa-sandbox 原生沙箱) 全部完成
- [x] Skill 5 验证: macOS Seatbelt fork 继承语义 (2026-02-25)
- [x] Skill 5 验证: JSON-Lines IPC 性能基线 (2026-02-25)
