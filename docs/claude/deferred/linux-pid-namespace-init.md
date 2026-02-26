---
document_type: Deferred
status: Draft
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: true
---

# Linux PID Namespace + Init 进程

## 问题

Phase 2 Linux 后端实现了 User NS + Mount NS，但 **PID Namespace 未实现**。PID NS 需要 double-fork 模式和一个 PID 1 init 进程来正确收割孤儿进程。

## 影响

- 沙箱内进程可以看到宿主的 PID 空间（通过 `/proc`）
- 如果沙箱内进程 fork 子进程后退出，孤儿进程由宿主 init 收割而非沙箱 init
- 无法完全隐藏宿主进程信息

## 根因

PID NS 要求：
1. `unshare(CLONE_NEWPID)` — 新 PID 命名空间
2. `fork()` — 第一个子进程成为新 NS 的 PID 1
3. PID 1 进程必须处理 SIGCHLD 信号并 `waitpid(-1)` 收割所有孤儿
4. 实际命令在 PID 1 进程内执行或由 PID 1 再次 fork

这需要自定义 init 进程（mini-init），复杂度较高。

## 解决方案

```rust
// Simplified double-fork approach:
// Parent: unshare(CLONE_NEWPID) → fork()
//   └─ Child (PID 1 in new NS):
//        ├─ install SIGCHLD handler (reap zombies)
//        ├─ fork() → exec(command)
//        └─ waitpid(command_pid) → exit(status)
```

参考实现: runc `nsexec.c`, containerd `init.go`

## 优先级

低 — Landlock + Seccomp 已提供足够的隔离。PID NS 是增强层。

## 相关代码

- `cli-rust/crates/oa-sandbox/src/linux/namespace.rs` — 注释标记 PID NS 推迟
- `cli-rust/crates/oa-sandbox/src/linux/mod.rs` — `apply_user_namespace()` + `apply_mount_namespace()` 已实现
