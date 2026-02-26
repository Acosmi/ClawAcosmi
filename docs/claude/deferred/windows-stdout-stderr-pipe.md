---
document_type: Deferred
status: Draft
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: false
---

# Windows stdout/stderr 管道捕获

## 问题

Windows 后端使用 `CreateProcessAsUserW` 创建子进程（而非 Rust `std::process::Command`），因此无法自动获得 stdout/stderr 的管道捕获。当前 `SandboxOutput.stdout` 和 `SandboxOutput.stderr` 始终返回空字符串。

## 影响

- Windows 后端 `SandboxOutput` 中 stdout/stderr 为空
- Go 调度层无法获取子进程输出
- 集成测试无法验证输出内容（仅验证 exit code）

## 根因

`CreateProcessAsUserW` 不走 Rust 的 `Command` 抽象，需手动：
1. 调用 `CreatePipe` 创建匿名管道
2. 设置 `STARTUPINFOW.hStdOutput` / `hStdError` 为管道写端
3. 设置 `STARTUPINFOW.dwFlags = STARTF_USESTDHANDLES`
4. 在父进程中读取管道读端
5. 确保管道读取和 `WaitForSingleObject` 不死锁（需线程化读取或异步 I/O）

## 解决方案

在 `windows/mod.rs` 的 `WindowsRunner::run()` 中：

```rust
// 1. CreatePipe for stdout
let mut stdout_read = HANDLE::default();
let mut stdout_write = HANDLE::default();
CreatePipe(&mut stdout_read, &mut stdout_write, Some(&sa), 0)?;
SetHandleInformation(stdout_read, HANDLE_FLAG_INHERIT, HANDLE_FLAGS(0))?;

// 2. Same for stderr

// 3. Pass to STARTUPINFOW
si.hStdOutput = stdout_write;
si.hStdError = stderr_write;
si.dwFlags = STARTF_USESTDHANDLES;

// 4. After CreateProcess, close write ends, then read from read ends
// 5. Use a thread to read pipes while waiting for process
```

## 优先级

中等 — Phase 5 CLI 集成前需完成（Go 层依赖 stdout/stderr）。

## 相关代码

- `cli-rust/crates/oa-sandbox/src/windows/mod.rs` `WindowsRunner::run()`
- 注释标记: `// TODO: Implement pipe-based stdout/stderr capture in a follow-up.`
