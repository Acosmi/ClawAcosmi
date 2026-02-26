---
document_type: Audit
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: self
skill5_verified: true
---

# Skill 4 审计报告: Linux 后端 (Phase 2)

## 审计范围

| 文件 | 职责 |
|------|------|
| `linux/mod.rs` | LinuxRunner 编排: spawn, pre_exec, timeout |
| `linux/landlock.rs` | Landlock LSM 文件系统隔离 |
| `linux/seccomp.rs` | Seccomp-BPF 系统调用过滤 |
| `linux/namespace.rs` | User NS + Mount NS (可选) |
| `linux/cgroup.rs` | Cgroups v2 资源限制 (可选) |
| `tests/linux_integration.rs` | 集成测试 |

## 判定: CONDITIONAL PASS

### 必须修复 (阻塞归档)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| S-01 | High | seccomp.rs:109 | Default-allow seccomp 遗漏新 mount API (open_tree, move_mount, fsopen, fspick, fsconfig, fsmount) 和 clone3 |
| S-02 | Medium | seccomp.rs:136-172 | Restricted 模式未阻断 AF_NETLINK, AF_PACKET, AF_VSOCK |
| S-04 | Medium | landlock.rs:34-45 | Landlock 开放 /proc, /sys, /dev 全部读权限, 暴露敏感信息 |

### 建议修复 (非阻塞)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| S-03 | Medium | seccomp.rs:114,127 | 未识别系统调用名静默跳过, 可能掩盖拼写错误 |
| S-05 | Medium | landlock.rs:96-100 | TMPDIR 环境变量未验证, 可被设为任意路径 |
| S-06 | Medium | landlock.rs:103-110 | mount host_path 未 canonicalize, 符号链接/路径穿越风险 |
| C-03 | Low | cgroup.rs:95 | Cgroup 名称用父 PID, 并发运行可冲突 |
| C-07 | Medium | namespace.rs:89-183 | Mount NS 未屏蔽 /proc/sysrq-trigger, /proc/kcore 等敏感路径 |

### 延迟处理 (低风险)

| ID | 风险 | 问题 |
|----|------|------|
| S-07 | Low | pre_exec 闭包用 format!() 在 fork 后分配内存 (async-signal-safety) |
| S-08 | Low | 危险系统调用用 EPERM 而非 KILL |
| R-01 | Low | CgroupGuard::drop() 静默忽略失败 |
| R-02 | Low | Timeout 线程 PID 理论上可被回收 |
| C-02 | Low | 信号终止进程的 exit code 信息丢失 |
