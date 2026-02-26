---
document_type: Audit
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: self
skill5_verified: true
---

# Skill 4 审计报告: Windows 后端 (Phase 4)

## 审计范围

| 文件 | 职责 |
|------|------|
| `windows/mod.rs` | WindowsRunner 编排: CreateProcess, Job, timeout |
| `windows/job.rs` | Job Object RAII 管理 |
| `windows/token.rs` | Restricted Token 创建 |
| `windows/acl.rs` | NTFS ACL 临时授权 + RAII 恢复 |
| `windows/appcontainer.rs` | 可选 AppContainer 隔离 |
| `tests/windows_integration.rs` | 集成测试 |

## 判定: FAIL — 需修复 Critical/High 问题

### 必须修复 (阻塞归档)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| S-1 | Critical | mod.rs:222-228 | process_handle 未用 RAII 保护, Job assign 失败时泄漏句柄 + 孤儿进程 |
| S-2 | High | mod.rs:246-271 | process_handle 在主线程和 timeout 线程间共享, 存在 use-after-close 竞态 |
| S-3 | High | mod.rs:288-297 | timeout 路径 CloseHandle 后主路径可能再次 close (double-close) |
| S-6 | High | token.rs:86-116 | set_low_integrity_level 失败时 restricted_token 句柄泄漏 |
| S-7 | Medium | mod.rs:395-414 | 空 env_vars 产生空环境块而非继承父环境 |

### 建议修复 (非阻塞但重要)

| ID | 风险 | 位置 | 问题 |
|----|------|------|------|
| S-4 | High | acl.rs:66-103 | AclGuard Drop 失败导致提升的 DACL 永久保留 |
| S-5 | High | acl.rs:206-215 | mem::forget + 手动 free 模式脆弱, 易引发 double-free |
| S-8/9 | Medium | token.rs:155, mod.rs:457 | Vec<u8> 转 TOKEN_GROUPS/TOKEN_USER 可能违反对齐要求 |
| S-10 | Medium | job.rs:109 | memory_bytes u64 转 usize, 32位平台截断 |
| S-12 | Medium | mod.rs:330 | exit_code u32 转 i32, 大值回绕为负数 |
| S-13-15 | Medium | token/acl/appcontainer | transmute 应替换为显式 HLOCAL 构造 |

### 延迟处理

| ID | 风险 | 问题 |
|----|------|------|
| S-11 | Medium | CPU rate 限制上限 1000 millicores (1 核) |
| S-16 | Low | quote_arg 不处理 cmd.exe 的 % 展开 |
| S-17 | Low | workspace 路径用 to_string_lossy 而非 encode_wide |
| S-22 | Low | ResumeThread 返回值未检查 |
| S-23 | Info | 集成测试覆盖不足 (缺少 ACL 恢复/资源限制验证) |
