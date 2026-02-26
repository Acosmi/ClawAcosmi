---
document_type: Audit
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: self
skill5_verified: true
---

# Skill 4 审计报告: macOS 后端 (Phase 3)

## 审计范围

| 文件 | 职责 |
|------|------|
| `macos/mod.rs` | MacosRunner 编排: spawn, pre_exec, timeout |
| `macos/seatbelt.rs` | SBPL profile 生成 |
| `macos/ffi.rs` | sandbox_init_with_parameters FFI |
| `tests/macos_integration.rs` | 集成测试 |

## 判定: CONDITIONAL PASS

### 必须修复 (阻塞归档)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| F-04 | Medium | seatbelt.rs:106-117 | mount 路径用字符串内插而非 SBPL 参数, 与 workspace 路径处理方式不一致 |
| F-16 | Medium | seatbelt.rs:52 | workspace 路径未验证为绝对路径, "/" 等系统目录也被接受 |

### 建议修复 (非阻塞)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| F-02 | Medium | ffi.rs:67-70 | unsafe impl Sync 不必要, 仅需 Send |
| F-08 | Medium | seatbelt.rs:189 | process-exec 广泛允许执行任意二进制 |
| F-09 | Medium | seatbelt.rs:225-229 | /tmp 和 /private/tmp 可被沙箱进程完全读写 |
| F-11 | Medium | seatbelt.rs:160-175 | NetworkPolicy::Restricted 无法阻断 LAN 地址 (已有 deferred 文档) |
| F-13 | Low | seatbelt.rs:70-73 | TMPDIR canonicalize 失败静默回退 |
| F-15 | Low | seatbelt.rs:72 | to_string_lossy 可能导致非 UTF-8 路径不匹配 |

### 延迟处理

| ID | 风险 | 问题 |
|----|------|------|
| F-05 | Low | Timeout PID 理论上可被回收 |
| F-06 | Low | exit code 未包含信号信息 |
| F-10 | Low | pre_exec 闭包错误路径有 format!() 分配 |
| F-18 | Info | Mach services 白名单可能需要扩展 |
