---
document_type: Audit
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: self
skill5_verified: true
---

# Skill 4 审计报告: 共享核心代码 (Phase 1)

## 审计范围

| 文件 | 职责 |
|------|------|
| `lib.rs` | Crate 入口, SandboxRunner trait |
| `config.rs` | SandboxConfig, SecurityLevel, NetworkPolicy |
| `error.rs` | SandboxError 类型 |
| `output.rs` | SandboxOutput JSON IPC |
| `platform.rs` | 平台检测 + backend 选择 |
| `docker/mod.rs` | Docker fallback stub |
| `Cargo.toml` | 依赖与 lint 配置 |

## 判定: CONDITIONAL PASS

### 必须修复 (阻塞归档)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| F-01 | High | config.rs | SandboxConfig 无输入验证 (空命令/路径穿越/null 字节/LD_PRELOAD) |
| F-02 | High | config.rs:154-157 | network_policy 可覆盖 security_level 默认值, 降低安全级别 |
| F-22 | Medium | linux/mod.rs:121-170 | pre_exec 闭包用 format!() 分配内存, 违反 async-signal-safety |

### 建议修复 (非阻塞)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| F-05 | Medium | platform.rs:189-205 | seccomp 检测最终回退为 true, 应改为 false |
| F-12 | Medium | output.rs/error.rs | Timeout 返回 Err 而非 exit_code=3, 需统一约定 |
| F-21 | Medium | lib.rs:35 | #![allow(unsafe_code)] 全 crate 范围过宽 |
| F-07 | Medium | config.rs | Serde 反序列化接受语义无效状态 (空路径/timeout=0/根挂载) |
| F-18 | Low | Cargo.toml:67 | expect_used = "warn" 应改为 "deny" |

### 延迟处理 (Phase 5+)

| ID | 风险 | 问题 |
|----|------|------|
| F-03 | Medium | User namespace 检测可能误报 (Docker-in-Docker) |
| F-04 | Medium | Landlock ABI 版本固定为 1, 需使用 landlock_create_ruleset 检测 |
| F-08 | Low | 缺少 DockerExec 错误变体 |
| F-09 | Medium | docker_available() 仅检查 PATH, 未验证 daemon 连通性 |
