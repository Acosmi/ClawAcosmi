# 安全模块架构文档

> 最后更新：2026-02-26 | 代码级审计完成

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/internal/security/` |
| Go 源文件数 | 11 |
| Go 测试文件数 | 4 |
| 测试函数数 | 68 |
| 总行数 | 5,065 |

负责：配置安全审计、文件权限检查/修复、外部内容安全、SSRF 防护、技能文件扫描、频道元数据安全包装。

## 二、文件索引

### 审计与修复 (3 文件, 核心)

| 文件 | 行数 | 职责 | TS 来源 |
|------|------|------|---------|
| `audit.go` | 597 | 完整安全审计：网关配置/浏览器控制/日志/特权/钩子/密钥/模型/云同步检测 (16 项检查) | `audit.ts` |
| `audit_extra.go` | 971 | 扩展审计规则：文件权限/FS 审计/Windows ACL 审计 | `audit-extra.ts` |
| `fix.go` | 650 | 自动修复：safeChmod/safeAclReset + 配置修复 (groupPolicy/allowFrom) | `fix.ts` |

### SSRF 防护 (2 文件)

| 文件 | 行数 | 职责 | TS 来源 |
|------|------|------|---------|
| `ssrf.go` | 299 | SSRF 检测：私有 IP/阻止主机名/DNS pinning/安全 fetch | `ssrf.ts` |
| `guarded_fetch.go` | 120 | 高层安全 HTTP 请求封装 (DNS pinning + SSRF + 超时 + 重定向控制) | 新增 |

### 内容安全 (2 文件)

| 文件 | 行数 | 职责 | TS 来源 |
|------|------|------|---------|
| `external_content.go` | 345 | Unicode marker 净化 + prompt injection 检测 + 安全边界包装 | `external-content.ts` |
| `channel_metadata.go` | 87 | 不可信频道元数据安全包装 | `channel-metadata.ts` |

### 文件系统安全 (2 文件)

| 文件 | 行数 | 职责 | TS 来源 |
|------|------|------|---------|
| `audit_fs.go` | 208 | POSIX/Windows 文件权限审计 | `audit-fs.ts` |
| `windows_acl.go` | 323 | Windows icacls ACL 解析 + principal 分类 | `windows-acl.ts` |

### 技能扫描 (1 文件)

| 文件 | 行数 | 职责 | TS 来源 |
|------|------|------|---------|
| `skill_scanner.go` | 359 | JS/TS 文件安全规则扫描 (网络/FS/代码执行) | `skill-scanner.ts` |

### 工具 (1 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `jsonc.go` | 27 | JSON with comments 去注释工具 |

## 三、关键类型

| 类型 | 文件 | 说明 |
|------|------|------|
| `SecurityAuditReport` | `audit.go` | 完整审计报告 (findings + summary + deep) |
| `SecurityAuditFinding` | `audit.go` | 单条审计发现 (checkId/severity/title/detail/remediation) |
| `SecurityAuditOptions` | `audit.go` | 审计选项 (DI 注入: 配置快照) |
| `SecurityFixResult` | `fix.go` | 修复结果 (ok/changes/actions/errors) |
| `SsrfPolicy` | `ssrf.go` | SSRF 策略 (allowPrivate/allowedHostnames) |
| `GuardedFetchOptions` | `guarded_fetch.go` | 安全 HTTP 请求选项 |

## 四、测试覆盖

| 测试文件 | 测试数 | 覆盖范围 |
|----------|--------|----------|
| `audit_test.go` | 30 | 完整审计 16 项检查 |
| `external_content_test.go` | 17 | Unicode 净化 + prompt injection |
| `ssrf_test.go` | 14 | 私有 IP + 阻止主机名 + DNS pinning |
| `jsonc_test.go` | 7 | JSON 去注释 |
| **合计** | **68** | **全覆盖** |
