# security 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W8 (中型模块)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 8 | 10 | 125.0% |
| 总行数 | 4028 | 3960 | 98.3% |

*(注：Go 端额外拆分了 `guarded_fetch.go` 和 `ssrf.go` 作为专门的网络安全防御模块，结构更清晰)*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 |
|------|---------|---------|
| ✅ FULL | `channel-metadata.ts` | `channel_metadata.go` |
| ✅ FULL | `audit-fs.ts` | `audit_fs.go` |
| ✅ FULL | `windows-acl.ts` | `windows_acl.go` |
| ✅ FULL | `external-content.ts` | `external_content.go`, `guarded_fetch.go`, `ssrf.go` |
| ✅ FULL | `skill-scanner.ts` | `skill_scanner.go` |
| ✅ FULL | `fix.ts` | `fix.go` |
| ✅ FULL | `audit.ts` | `audit.go` |
| ✅ FULL | `audit-extra.ts` | `audit_extra.go` |

> 评价：Go 端的文件划分颗粒度更细，特别是网络安全部分拆分成了 SSRF 防御和受限 Fetch 两部分，这是很好的安全架构改进。核心的安全扫描和修复逻辑 `audit.go/audit_extra.go` 与 TS 的 `audit.ts/audit-extra.ts` 行数高度匹配，逻辑平移非常顺利。

## 隐藏依赖审计

| # | 类别 | 检查结果 | Go端实现方案 |
|---|------|----------|-------------|
| 1 | npm 包黑盒行为 | ⚠️ 使用了 Windows ACL 相关的外部库或原生调用 | Go 使用 `syscall` 和 `golang.org/x/sys/windows` 提供更安全和原生的 Windows ACL 处理 |
| 2 | 全局状态/单例 | ✅ 无 | 无全局副作用变量，均通过 Context 或实例传递 |
| 3 | 事件总线/回调链 | ✅ 无 | 无事件链，功能独立 |
| 4 | 环境变量依赖 | ⚠️ SSRF 配置或被动安全配置可能依赖环境变量 | Go 端统一在 `config` 包和 `gateway_lock` 初始化时注入 |
| 5 | 文件系统约定 | ⚠️ `audit-fs.ts` 和 `windows-acl.ts` 强依赖系统级文件权限操作 | Go 端利用 `os` 和平台特性代码（如 `//go:build windows` 等）实现，完美取代 Node.js 的 fs 模块 API |
| 6 | 协议/消息格式 | ⚠️ `external-content.ts` 涉及 HTTP/HTTPS 协议和内网 IP 过滤 | Go 拆分出专门的 `ssrf.go` 实现安全的 `http.Transport` 和 `DialContext`。拦截私有 IP 和高危端口 |
| 7 | 错误处理约定 | ⚠️ TS 会抛出指定的安全拦截错误 | Go 通过结构化的错误类型区分 "SSRF拦截", "权限受限" 和 "非法路径"，与 TS 表现一致 |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| SEC-1 | 架构重构 | `external-content.ts` | `guarded_fetch.go`, `ssrf.go` | Go 采用标准库 `http.Transport` 的 DialContext 拦截 TCP 连接阶段的 SSRF，比 TS 端处理更为底层和安全。 | P3 | 优秀的安全加固，无需修复 |
| SEC-2 | 底层实现 | `windows-acl.ts` | `windows_acl.go` | Go 直接使用系统级 API 管理权限，比依赖 Node 环境更可靠。 | P3 | 提升了可靠性，无需修复 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- **模块审计评级: A** (Go 端增强了网络安全层和平台级 FS 控制，是一个比 TS 原版更安全的实现)
