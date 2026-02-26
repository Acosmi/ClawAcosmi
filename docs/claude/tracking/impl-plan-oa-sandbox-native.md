---
document_type: Tracking
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: |
  - docs/claude/audit/audit-2026-02-25-shared-core.md (Phase 1 Core - CONDITIONAL PASS)
  - docs/claude/audit/audit-2026-02-25-linux-backend.md (Phase 2 Linux - CONDITIONAL PASS)
  - docs/claude/audit/audit-2026-02-25-macos-backend.md (Phase 3 macOS - CONDITIONAL PASS)
  - docs/claude/audit/audit-2026-02-25-windows-backend.md (Phase 4 Windows - FAIL → findings fixed)
  - docs/claude/audit/audit-2026-02-25-phase5-docker-cli.md (Phase 5 Docker+CLI - PASS)
  - docs/claude/audit/audit-2026-02-25-phase6-security-hardening.md (Phase 6 Hardening - PASS)
skill5_verified: true
---

# oa-sandbox 原生沙箱实施计划

> 替代 Docker CLI，使用 OS 原生隔离机制，实现免依赖部署与 10ms 级冷启动。

## 在线验证摘要（Skill 5 已完成）

详细验证记录见：
- `docs/claude/tracking/verification-linux-sandbox-apis-2026-02-25.md`
- `docs/claude/tracking/verification-macos-sandbox-apis-2026-02-25.md`
- `docs/claude/tracking/windows-sandbox-api-verification.md`
- `docs/claude/tracking/research-sandbox-isolation-survey-2026-02-25.md`

### 关键修正（与原设计文档的偏差）

| # | 原设计假设 | 验证结果 | 修正方案 |
|---|---|---|---|
| 1 | 使用 `sandbox-rs` crate | **sandbox-rs 需要 root 权限**，不适用于桌面用户 | 自建组合：`clone3` + `landlock` + `libseccomp` + `nix`/`libc` |
| 2 | Linux 无特权 Namespace 可用 | **Ubuntu 24.04 默认通过 AppArmor 禁止无特权 user namespace** | Landlock + seccomp 作为一级方案；Namespace 作为增强层（可用时启用） |
| 3 | macOS 导入 `system.sb` | **正确的最小基线是 `bsd.sb`**，`system.sb` 权限过宽 | 改用 `(import "bsd.sb")` |
| 4 | macOS 调用 `sandbox-exec` CLI | Chromium 直接调用 `sandbox_init_with_parameters` FFI | 改用 FFI 直调，避免额外进程开销 |
| 5 | Windows 以 AppContainer 为主 | **AppContainer 对传统 .exe 兼容性极差**（Electron/Chromium 均验证） | Restricted Token + Job Object 为默认层；AppContainer 为可选增强 |
| 6 | Cgroups v2 可直接使用 | **无特权用户需要 systemd delegation** | Cgroups 层设为可选，通过 `systemd-run --user` 获取委托 |
| 7 | PID 1 收割自动生效 | **PID 1 默认忽略 SIGTERM/SIGINT**，必须显式注册信号处理器 | 实现完整 init 进程：SIGCHLD 收割 + 信号转发 |
| 8 | Landlock 可控网络 | **Landlock 网络过滤仅限 TCP**（ABI 4+），无 UDP/ICMP | 完整网络隔离需 Network Namespace；受限模式可用 Landlock TCP 过滤 |
| 9 | Workspace `unsafe_code = "forbid"` | 当前 workspace lint 全局禁止 unsafe | oa-sandbox crate 需 `#![allow(unsafe_code)]` 覆盖，配合严格 `// SAFETY:` 审计 |

### 行业最佳实践借鉴

| 来源 | 借鉴要点 |
|---|---|
| **Chromium** | Broker/Target 模型；Windows 四层防御（Restricted Token + Job Object + Alternate Desktop + Integrity Level）；macOS `sandbox_init_with_parameters` FFI 直调 + 参数化 profile |
| **Firecracker Jailer** | 特权设置 → 不可逆降权 → exec 进入沙箱（setup-then-drop 模式） |
| **Bubblewrap** | 空根 + 白名单 bind mount；`--unshare-*-try` 优雅降级；轻量 PID 1 收割 |
| **Claude Code sandbox-runtime** | 代理式网络控制（HTTP/SOCKS5 proxy + 域名白名单）；预编译 seccomp BPF；阻断 Unix socket 防代理绕过 |
| **gVisor** | 68 系统调用缩减目标（seccomp profile 基线参考） |
| **Deno** | 应用层权限检查**无法存活 fork/exec**——验证了 OS 级强制执行的必要性 |

---

## 架构总览

```
┌──────────────────────────────────────────────────────────────┐
│                    Go 调度层 (Agent Core)                      │
│  attempt_runner.go → tool_executor.go                        │
│       │                          ▲                           │
│       │ exec.CommandContext       │ stdout JSON               │
│       ▼                          │                           │
│  ┌────────────────────────────────────────────────────────┐  │
│  │              oa-cmd-sandbox (CLI 入口)                  │  │
│  │  openacosmi sandbox run --security L1 --format json    │  │
│  └────────────────────┬───────────────────────────────────┘  │
└───────────────────────┼──────────────────────────────────────┘
                        │
┌───────────────────────┼──────────────────────────────────────┐
│              oa-sandbox (核心运行时库)                         │
│                       │                                      │
│  ┌────────────────────▼───────────────────────┐              │
│  │          SandboxRunner trait                │              │
│  │  fn run(&self, config) -> Result<Output>    │              │
│  └────┬─────────┬──────────┬──────────┬───────┘              │
│       │         │          │          │                       │
│  ┌────▼───┐ ┌──▼────┐ ┌──▼─────┐ ┌──▼──────┐               │
│  │ Linux  │ │ macOS │ │Windows │ │ Docker  │               │
│  │Runner  │ │Runner │ │Runner  │ │Fallback │               │
│  └────────┘ └───────┘ └────────┘ └─────────┘               │
│                                                              │
│  核心模块：                                                   │
│  ├── config.rs      配置 & 安全级别定义                        │
│  ├── output.rs      JSON IPC 输出契约                         │
│  ├── platform.rs    平台检测 & 能力探测                        │
│  ├── init.rs        PID 1 init 进程 (Linux)                  │
│  ├── network.rs     网络策略执行                               │
│  └── resource.rs    RAII 资源守卫                              │
└──────────────────────────────────────────────────────────────┘
```

---

## 工程代码布局

```
cli-rust/crates/
├── oa-sandbox/                    ← [NEW] 沙箱运行时核心库
│   ├── Cargo.toml
│   └── src/
│       ├── lib.rs                 # 公开 API：SandboxRunner trait, run()
│       ├── config.rs              # SecurityLevel, MountSpec, ResourceLimits, NetworkPolicy
│       ├── output.rs              # SandboxOutput (JSON IPC 契约)
│       ├── error.rs               # SandboxError (thiserror 类型化错误)
│       ├── platform.rs            # 平台能力探测 (namespace? landlock? seatbelt?)
│       │
│       ├── linux/
│       │   ├── mod.rs             # LinuxRunner 入口
│       │   ├── namespace.rs       # User/PID/Mount/Net namespace 设置
│       │   ├── landlock.rs        # Landlock 文件系统 & 网络规则
│       │   ├── seccomp.rs         # Seccomp-BPF 过滤器
│       │   ├── cgroup.rs          # Cgroups v2 资源限制 (可选, 需 systemd delegation)
│       │   └── init.rs            # PID 1 init 进程 (信号转发 + 僵尸收割)
│       │
│       ├── macos/
│       │   ├── mod.rs             # MacosRunner 入口
│       │   ├── seatbelt.rs        # SBPL profile 动态生成
│       │   └── ffi.rs             # sandbox_init_with_parameters FFI 绑定
│       │
│       ├── windows/
│       │   ├── mod.rs             # WindowsRunner 入口
│       │   ├── token.rs           # Restricted Token 创建
│       │   ├── job.rs             # Job Object 资源限制 & 进程树收割
│       │   ├── appcontainer.rs    # AppContainer SID 管理 (可选增强)
│       │   └── acl.rs             # NTFS ACL 动态修改 & 撤销
│       │
│       └── docker/
│           └── mod.rs             # DockerFallback: 降级到 docker CLI
│
├── oa-cmd-sandbox/                ← [MODIFY] CLI 入口层
│   └── src/
│       ├── lib.rs                 # 扩展：注册 run 子命令
│       ├── run.rs                 ← [NEW] `openacosmi sandbox run` 解析 & 调度
│       ├── list.rs                # 现有
│       ├── recreate.rs            # 现有
│       ├── explain.rs             # 现有
│       ├── display.rs             # 现有
│       └── formatters.rs          # 现有
```

---

## Phase 1：核心骨架 + 平台探测 + 配置层

### 目标
建立 `oa-sandbox` crate 的类型基础、trait 抽象、JSON IPC 契约和平台能力探测机制。

### 任务清单

- [x] **1.1** 创建 `oa-sandbox` crate 并注册到 workspace ✅ 2026-02-26
  - 新建 `cli-rust/crates/oa-sandbox/Cargo.toml`
  - 在 `cli-rust/Cargo.toml` workspace members 中添加
  - 在 workspace.dependencies 中添加 `oa-sandbox` + 平台依赖 (landlock, libseccomp, nix, libc, windows)
  - 配置 lint 覆盖：手动复制 workspace lints + `unsafe_code = "allow"` (仅此 crate)

- [x] **1.2** 定义核心类型 (`config.rs`) ✅ 2026-02-26
  - `SecurityLevel` (L0Deny/L1Sandbox/L2Full) + `default_network_policy()` 映射
  - `NetworkPolicy` (None/Restricted/Host)
  - `MountMode` (ReadOnly/ReadWrite), `MountSpec`
  - `ResourceLimits` (memory_bytes/cpu_millicores/max_pids/timeout_secs)
  - `OutputFormat` (Json/Text), `BackendPreference` (Auto/Native/Docker)
  - `SandboxConfig` + `effective_network_policy()` 方法

- [x] **1.3** 定义 JSON IPC 输出契约 (`output.rs`) ✅ 2026-02-26
  - `SandboxOutput` (stdout/stderr/exit_code/error/duration_ms/sandbox_backend)
  - `exit_codes` 模块 (CONFIG_ERROR=2, TIMEOUT=3, RESOURCE_EXCEEDED=4)

- [x] **1.4** 定义错误类型 (`error.rs`) ✅ 2026-02-26
  - `SandboxError` (thiserror) 覆盖：InvalidConfig, CommandNotFound, PathError,
    PlatformNotSupported, NoBackendAvailable, Namespace, Landlock, Seccomp,
    Seatbelt, Win32, ResourceExceeded, Timeout, Degraded, Io

- [x] **1.5** 定义 `SandboxRunner` trait (`lib.rs`) ✅ 2026-02-26
  - `name() -> &'static str`, `available() -> bool`, `run(&SandboxConfig) -> Result<SandboxOutput, SandboxError>`
  - `select_runner()` 顶层入口函数

- [x] **1.6** 实现平台能力探测 (`platform.rs`) ✅ 2026-02-26
  - Linux: user namespace (sysctl + AppArmor 检测), Landlock ABI (LSM 列表), seccomp, cgroup v2 delegation
  - macOS: OS 版本 (sw_vers), Seatbelt 可用性 (sandbox-exec 检测)
  - Windows: Job Object (always), AppContainer (assumed on modern Windows)
  - 每平台 Runner stub: LinuxRunner, MacosRunner, WindowsRunner, DockerFallbackRunner

- [x] **1.7** 实现自动降级选择链 ✅ 2026-02-26
  - `select_native_runner()` → `select_docker_runner()` → `select_auto_runner()`
  - BackendPreference (Auto/Native/Docker) 三路选择
  - `SandboxBackend` 枚举 + `name()` + `is_native()` 方法

---

## Phase 2：Linux 后端实现

### 目标
实现 Linux 双保险沙箱：Landlock+Seccomp（一级方案，无特权）+ Namespace（增强层，需 user ns 可用）。

### 任务清单

- [x] **2.1** 实现 Landlock 文件系统隔离 (`linux/landlock.rs`) ✅ 2026-02-25
  - 使用 `landlock` crate（官方 Rust 绑定），ABI V4 best-effort 兼容
  - Workspace、system paths、temp dirs、additional mounts 规则映射
  - `apply_landlock_rules()` — 不可逆自限制

- [x] **2.2** 实现 Seccomp-BPF 过滤 (`linux/seccomp.rs`) ✅ 2026-02-25
  - 使用 `libseccomp` crate (v0.4+)，default-allow + 危险 syscall 黑名单
  - L0: 阻断所有网络 syscall（socket, connect, bind 等）
  - L1: 阻断 AF_UNIX + socketpair，允许 AF_INET/AF_INET6
  - 35+ 危险 syscall 永久阻断（ptrace, mount, bpf, unshare, setns 等）

- [x] **2.3** 实现 Namespace 增强层 (`linux/namespace.rs`) ✅ 2026-02-25
  - User NS: `unshare(CLONE_NEWUSER)` + UID/GID 映射 (host → 1000:1000)
  - Mount NS: `unshare(CLONE_NEWNS)` + `MS_PRIVATE` + bind mount
  - 仅在 `has_user_namespace` 时启用（Ubuntu 24.04 AppArmor 检测）

- [ ] **2.4** PID 1 init 进程 (`linux/init.rs`) — **推迟**
  - 需要 PID NS (double-fork + init 进程)，复杂度高
  - 当前 User NS + Mount NS 已提供足够隔离
  - 将在后续增强阶段实现

- [x] **2.5** 实现 Cgroups v2 资源限制 (`linux/cgroup.rs`) ✅ 2026-02-25
  - `CgroupGuard` RAII 守卫（Drop 清理 cgroup 目录）
  - `find_user_cgroup()` 检测 systemd user slice 委托
  - 写入 `memory.max`, `cpu.max`, `pids.max`
  - 无 systemd 时静默跳过

- [x] **2.6** 组装 LinuxRunner (`linux/mod.rs`) ✅ 2026-02-25
  - 完整执行流程: `pre_exec` 闭包中依次应用 User NS → Mount NS → Cgroup → Landlock → Seccomp
  - 50ms 轮询超时线程 + SIGKILL
  - Landlock+Seccomp 为必需层；Namespace+Cgroup 为可选增强

- [x] **2.7** 网络策略实现 ✅ 2026-02-25
  - 集成在 seccomp.rs 中: L0 阻断所有网络 syscall, L1 阻断 Unix socket
  - Landlock 处理文件系统, Seccomp 处理网络策略
  - 完整 LAN 阻断需 Net NS (推迟到增强阶段)

- [x] **2.8** 集成测试 (`tests/linux_integration.rs`) ✅ 2026-02-25
  - 10 个测试: echo、退出码、command not found、workspace 读/写、
    文件隔离、超时、额外 mount、环境变量、L0 网络阻断
  - **注意**: 需要 Linux CI 验证编译和运行（macOS 上 cfg-gated 跳过）

---

## Phase 3：macOS 后端实现

### 目标
实现 macOS Seatbelt 动态 profile 生成与 FFI 直调。

### 任务清单

- [x] **3.1** 实现 Seatbelt FFI 绑定 (`macos/ffi.rs`) ✅ 2026-02-26
  - `unsafe extern "C"` 声明 `sandbox_init_with_parameters` / `sandbox_free_error`
  - `SandboxArgs` 结构体：预构建 CString 避免 fork 后堆分配
  - `apply()` 方法：在 `pre_exec` 闭包中无分配调用 FFI
  - `// SAFETY:` 注释覆盖所有 unsafe 块

- [x] **3.2** 实现 SBPL Profile 生成器 (`macos/seatbelt.rs`) ✅ 2026-02-26
  - 基础模板：`(version 1) (deny default) (import "bsd.sb")`
  - 路径 canonicalize 处理 macOS `/var` → `/private/var` 符号链接
  - Workspace/TMPDIR/mount 路径参数化注入
  - 网络策略映射：None/Restricted/Host
  - **已知限制**：SBPL 不支持 CIDR 网络规则，LAN 地址无法通过 SBPL 阻断
  - 完整覆盖：系统路径、临时目录、设备文件、Mach 服务、sysctl

- [x] **3.3** 组装 MacosRunner (`macos/mod.rs`) ✅ 2026-02-26
  - `Command::pre_exec` 闭包应用沙箱（fork 后 exec 前）
  - 50ms 轮询超时线程 + `SIGKILL`
  - stdout/stderr pipe 收集 + `SandboxOutput` 构建

- [x] **3.4** 集成测试 (`tests/macos_integration.rs`) ✅ 2026-02-26
  - 9 个集成测试全部通过：基本执行、退出码、命令未找到、workspace 读/写、
    文件系统隔离、超时、额外 mount、环境变量
  - 6 个 SBPL 生成单元测试 + 3 个 FFI 单元测试全部通过
  - 零 clippy 警告

---

## Phase 4：Windows 后端实现

### 目标
实现 Windows Restricted Token + Job Object 双层防御，可选 AppContainer 增强。

### 任务清单

- [x] **4.1** 实现 Job Object 管理 (`windows/job.rs`) ✅ 2026-02-25
  - 使用 `windows` crate (`Win32_System_JobObjects` feature)
  - `CreateJobObjectW` → 配置：
    - `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`（核心：Drop 即收割）
    - `JOB_OBJECT_LIMIT_JOB_MEMORY`
    - `JOBOBJECT_CPU_RATE_CONTROL_INFORMATION` (hard cap)
    - `JOB_OBJECT_LIMIT_ACTIVE_PROCESS`
  - RAII `JobGuard`: `Drop` 关闭 handle → 自动终止所有进程
  - `AssignProcessToJobObject` 绑定子进程

- [x] **4.2** 实现 Restricted Token (`windows/token.rs`) ✅ 2026-02-25
  - 使用 `windows` crate (`Win32_Security` feature)
  - `OpenProcessToken` → `CreateRestrictedToken`：
    - 剥离所有特权 (`DISABLE_MAX_PRIVILEGE`)
    - 所有 group SID 设为 deny-only（保留 logon SID）
    - 设置 Low Integrity Level (S-1-16-4096)
  - `CreateProcessAsUserW` 使用受限 token 启动子进程

- [x] **4.3** 实现 NTFS ACL 管理 (`windows/acl.rs`) ✅ 2026-02-25
  - `GetNamedSecurityInfoW` → `SetEntriesInAclW` → `SetNamedSecurityInfoW`
  - `grant_workspace_access(path, sid, mode)` → 临时 ACE
  - RAII `AclGuard`: `Drop` 自动恢复原始 DACL，防提权漏洞

- [x] **4.4** 实现 AppContainer 可选增强 (`windows/appcontainer.rs`) ✅ 2026-02-25
  - `CreateAppContainerProfile` / `DeleteAppContainerProfile`
  - RAII `AppContainerGuard`: Drop 删除 profile
  - 处理 ERROR_ALREADY_EXISTS（崩溃残留清理）
  - 仅在用户显式 opt-in 时启用

- [x] **4.5** 组装 WindowsRunner (`windows/mod.rs`) ✅ 2026-02-25
  - 默认层：Job Object + Restricted Token + ACL
  - 可选层：AppContainer（模块已实现，未集成到 runner 主流程）
  - 执行流：创建 Job → 创建 Restricted Token → 修改 ACL
    → CreateProcessAsUserW(SUSPENDED) → AssignToJob → ResumeThread → Wait → Drop 撤销 ACL
  - 辅助函数：`build_command_line`、`quote_arg`、`build_environment_block`
  - 注：stdout/stderr 管道捕获待后续实现（TODO）

- [x] **4.6** 集成测试 ✅ 2026-02-25
  - 7 个测试用例（`#[cfg(target_os = "windows")]` 门控）
  - echo 基本执行、非零退出码、命令未找到
  - timeout 超时终止、workspace 写入、环境变量传递、进程数限制

---

## Phase 5：Docker Fallback + CLI 集成

### 目标
实现降级方案 + `openacosmi sandbox run` 子命令 + Go 层对接。

### 任务清单

- [x] **5.1** 实现 Docker Fallback (`docker/mod.rs`) ✅ 2026-02-25
  - `DockerFallbackRunner::run()` 完整实现
  - SandboxConfig → `docker run` 参数映射（security、network、mounts、limits、env）
  - `--rm` + `--cap-drop=ALL` + `--security-opt no-new-privileges` 安全基线
  - `try_wait` + `child.kill()` 超时机制（解决 Docker 进程杀不掉问题）
  - stdout/stderr 后台线程读取避免死锁
  - 6 个单元测试（L0/L1 参数、内存限制、环境变量、网络、自定义镜像）

- [x] **5.2** 实现 CLI 子命令 (`oa-cmd-sandbox/src/run.rs`) ✅ 2026-02-25
  - clap 参数定义：`--security`、`--workspace`、`--net`、`--timeout`、`--format`、`--backend`、`--mount`、`--env`、`--memory`、`--cpu`、`--pids`
  - `SandboxRunOptions` → `SandboxConfig` 转换
  - `sandbox_run_command()` → `select_runner()` → `runner.run()` → JSON/Text 输出
  - 错误映射：Timeout→exit 3, ResourceExceeded→exit 4, Config→exit 2
  - 9 个单元测试（mount 解析、env 解析、config 构建）
  - 在 `oa-cli/src/commands.rs` 中注册 `SandboxAction::Run` + `SandboxRunArgs`

- [x] **5.3** 实现自动降级逻辑 ✅ 2026-02-25 (Phase 1 已完成)
  - `platform::select_auto_runner()` 已在 Phase 1 实现
  - `--backend auto`（默认）：native 失败时自动尝试 Docker
  - `--backend native`：仅原生，失败即报错
  - `--backend docker`：仅 Docker

- [x] **5.4** Go 调度层集成接口 ✅ 2026-02-25
  - `SandboxOutput` JSON 格式兼容 Go 端 `DockerExecutionResult` struct
  - 额外字段 `sandbox_backend` 标识使用的后端
  - Exit code 约定：0=成功, 1=命令失败, 2=配置错误, 3=超时, 4=资源超限
  - Go 端通过 `exec.Command("openacosmi", "sandbox", "run", ...)` 调用

- [x] **5.5** 端到端集成测试 ✅ 2026-02-25
  - 8 个 Docker 集成测试（`tests/docker_integration.rs`）
  - echo、退出码、command not found、超时、环境变量、workspace mount
  - 网络隔离（`--network=none` 阻断连接）、`select_runner` Docker 选择
  - Go → Rust 全链路验证待 Phase 6 CI 管线

---

## Phase 6：安全加固 + CI + 文档

### 6.1 审计修复（已完成 2026-02-25）

- [x] **F-01** `SandboxConfig::validate()` — 输入验证 (config.rs)
  - 空命令、null byte、绝对路径、路径遍历、挂载验证
  - 12 项单元测试
  - 集成到 `platform::select_runner()` 入口
- [x] **F-02** 网络策略弱化检测 — validate() 拒绝 L0+Host 等弱化组合
- [x] **F-04** macOS SBPL 挂载注入防护 — 挂载路径使用 `(param "MOUNT_N")` 参数化
- [x] **S-01** Linux seccomp 缺失系统调用 — 添加 open_tree/move_mount/fsopen/fspick/fsconfig/fsmount/clone3
- [x] **S-02** Linux AF_NETLINK/AF_PACKET/AF_VSOCK 阻断 — Restricted 模式新增 3 条 socket 规则
- [x] **S-04** Linux Landlock 范围缩小 — /proc→自身进程+7文件, /sys→CPU拓扑, /dev→10设备文件
- [x] **F-22** Linux pre_exec 静态错误字符串 — 移除 format!() 减少 post-fork 堆分配
- [x] **S-1** Windows process_handle RAII — 用 HandleGuard 包装，消除手动 CloseHandle
- [x] **S-2** Windows use-after-close 竞态 — timeout 线程使用句柄副本，主线程 join 后 guard 释放
- [x] **S-3** Windows double-close — 移除所有手动 CloseHandle，guard 统一关闭
- [x] **S-6** Windows token handle 泄漏 — 创建后立即包装 RestrictedToken，set_low_integrity_level 失败时 Drop 关闭
- [x] **S-7** Windows 空环境块 — 空 env_vars 传 None 给 CreateProcessAsUserW 以继承父进程环境

### 6.1b Phase 5 审计（已完成 2026-02-25）

- [x] Docker Fallback 审计 — PASS（0 Critical/High，4 Low/Info）
  - 审计报告: `docs/claude/audit/audit-2026-02-25-phase5-docker-cli.md`

### 6.2 CI 管线（已完成 2026-02-25）

- [x] `.github/workflows/oa-sandbox-ci.yml` 创建
  - **Lint**: Ubuntu 24.04 — clippy + rustfmt (需 libseccomp-dev)
  - **Test Linux**: Ubuntu 24.04 — unit + integration (Landlock/Seccomp) + Docker
  - **Test macOS**: macos-latest — unit + integration (Seatbelt)
  - **Test Windows**: windows-latest — unit + integration (Job Object/Token) + Docker
  - **Cross-check**: 3 targets (linux-gnu, aarch64-apple-darwin, x86_64-pc-windows-msvc)
  - 路径过滤触发 (oa-sandbox/\*\*, oa-cmd-sandbox/\*\*)
  - 并发控制 (cancel-in-progress)
  - cargo cache 加速
- [x] 代码格式统一 (`cargo fmt` 应用到 oa-sandbox + oa-cmd-sandbox)

### 6.3 Dry Run 机制（已完成 2026-02-25）

- [x] `--dry-run` CLI flag 添加到 `SandboxRunArgs` (commands.rs)
- [x] `dry_run: bool` 字段添加到 `SandboxRunOptions` (run.rs)
- [x] `emit_dry_run_preview()` 实现 JSON + Text 两种输出模式
  - JSON: 完整执行计划（backend, security, command, workspace, network, mounts, env, limits）
  - Text: 人类可读的格式化预览（eprintln 输出到 stderr）
- [x] L2 Full + Text 格式自动触发 dry run（human-in-the-loop 安全门控）
- [x] 2 个新测试: `l2_text_triggers_auto_dry_run`, `l1_text_no_auto_dry_run`
- [x] 全部 50 个 oa-cmd-sandbox 测试通过

### 6.4 性能基准（已完成 2026-02-25）

- [x] `criterion` 基准测试框架集成 (workspace dep + oa-sandbox dev-dep)
- [x] `benches/cold_start.rs` — 3 组基准测试:
  - **baseline_no_sandbox**: 裸 `Command::new` 基线（~780µs true, ~860µs echo）
  - **native_cold_start**: 原生沙箱冷启动（macOS Seatbelt ~65ms）
  - **docker_cold_start**: Docker 容器冷启动（~215ms true, ~231ms echo）
- [x] 平台自适应命令路径（Alpine `/bin/true` vs macOS `/usr/bin/true` vs Windows `cmd.exe`）
- [x] Docker 用 reduced sample (10) + extended measurement (20s) 适配慢速测试

#### 基准结果（macOS Apple Silicon, 2026-02-25）

| Benchmark | 延迟 | vs 基线倍数 |
|---|---|---|
| baseline true (无沙箱) | ~781µs | 1x |
| native macOS Seatbelt true | ~65.5ms | 84x |
| native macOS Seatbelt echo | ~65.8ms | 76x |
| Docker true | ~215ms | 275x |
| Docker echo | ~231ms | 267x |

**分析**: macOS Seatbelt ~65ms 冷启动主要来自 SBPL profile 编译开销（内核侧），
非 Rust 代码瓶颈。Linux Landlock+Seccomp 预计 <10ms（无 profile 编译步骤）。
Docker ~215ms 是容器创建+namespace setup+exec 的固有开销。
原生后端比 Docker 快 **3.3x**，验证了原生沙箱的性能优势。

---

## 依赖矩阵

### oa-sandbox Cargo.toml 依赖规划

```toml
[dependencies]
# 错误处理
thiserror = { workspace = true }
anyhow = { workspace = true }

# 序列化 (JSON IPC)
serde = { workspace = true }
serde_json = { workspace = true }

# 日志
tracing = { workspace = true }

# 内部
oa-types = { workspace = true }

[target.'cfg(target_os = "linux")'.dependencies]
# Landlock (unprivileged FS + network isolation)
landlock = "0.4"
# Seccomp-BPF
libseccomp = "0.4"
# POSIX API
nix = { version = "0.29", features = ["sched", "signal", "mount", "user", "process"] }

[target.'cfg(target_os = "macos")'.dependencies]
# macOS 仅需 libc 做 FFI
libc = "0.2"

[target.'cfg(target_os = "windows")'.dependencies]
windows = { version = "0.62", features = [
    "Win32_Security",
    "Win32_Security_Authorization",
    "Win32_Security_Isolation",
    "Win32_System_JobObjects",
    "Win32_System_Threading",
    "Win32_Foundation",
] }
```

### 新增 workspace 级依赖

```toml
# 以下需添加到 cli-rust/Cargo.toml [workspace.dependencies]
landlock = "0.4"
libseccomp = "0.4"
nix = { version = "0.29", features = ["sched", "signal", "mount", "user", "process"] }
libc = "0.2"
windows = { version = "0.62" }
```

---

## 实施优先级排序

```
Phase 1 (骨架)     ██████████  ✅ 完成 (2026-02-26) — 审计: CONDITIONAL PASS
Phase 2 (Linux)    █████████░  ✅ 完成 (2026-02-25, PID NS init 推迟) — 审计: CONDITIONAL PASS
Phase 3 (macOS)    ██████████  ✅ 完成 (2026-02-26) — 审计: CONDITIONAL PASS
Phase 4 (Windows)  █████████░  ✅ 完成 (2026-02-25, stdout/stderr 管道捕获待后续) — 审计: FAIL
Phase 5 (CLI集成)  ██████████  ✅ 完成 (2026-02-25) — Docker Fallback + CLI run + 8 集成测试
Phase 6 (加固)     ██████████  ✅ 完成 — 6.1 审计修复 ✅, 6.2 CI 管线 ✅, 6.3 Dry Run ✅, 6.4 性能基准 ✅
```

**建议立即开始**：Phase 1 → Phase 2（当前开发环境为 macOS，但 Linux 是主战场且验证资料最充分）。
