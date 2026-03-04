# W1 全局审计报告 (Gateway, Security, Config)

> 审计日期：2026-02-20 | 审计窗口：W1

## 1. Config 模块审计报告

### 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 85 | 34 | 40% (Go架构更紧凑/类型收敛) |
| 总行数 | ~8000 | ~3500 | ~43% |

### 逐文件对照

| 状态 | TS文件/模块 | Go实现对应 | 评估说明 |
|------|-------------|------------|----------|
| ✅ FULL | `paths.ts`, `normalize-paths.ts`, `config-paths.ts` | `paths.go`, `normpaths.go`, `configpath.go` | 路径解析完全对齐 |
| ✅ FULL | `legacy*.ts` | `legacy.go`, `legacy_migrations*.go` | 遗留配置迁移逻辑完整 |
| ✅ FULL | `schema.ts`, `zod-schema*.ts`, `types*.ts` | `schema.go`, `validator.go`, `schema_hints_data.go` | 结构体映射与校验完整 |
| ✅ FULL | `io.ts`, `config.ts` | `loader.go` | 配置文件读取与写入对等 |
| ✅ FULL | `env-substitution.ts` | `envsubst.go` | 环境变量注入完全实现 |
| ✅ FULL | `sessions/*.ts` | `session_*.go` | 会话管理逻辑全面迁移 |
| ✅ FULL | `redact-snapshot.ts`, `plugin-auto-enable.ts` | `redact.go`, `plugin_auto_enable.go` | 副作用与安全处理对等 |
| 🔄 REFACTORED | 各种细分 `types.*.ts` | 聚合在了 `schema.go` 等核心文件 | Go 采用了统一结构体定义，抛弃了 TS 零散冗余类型 |

### 隐藏依赖审计

1. **npm 包黑盒行为**: 无特殊依赖，依赖标准库。
2. **全局状态/单例**: 配置模块本身作为单例结构体在 Go 中传递，无静态陷阱。
3. **事件总线/回调链**: 无。
4. **环境变量依赖**: `OPENACOSMI_HOME`, `OPENACOSMI_CONFIG_PATH` 均在 `paths.go` 正确解析。
5. **文件系统约定**: `fs.mkdir/readFile/writeFile` 与 `loader.go` 中的 `os` 包行为对等，提供相同的并发锁。
6. **协议/消息格式**: JSON 序列化标签在 Go `schema.go` 中完全对齐 TS。
7. **错误处理约定**: `MissingEnvVarError`, `DuplicateAgentDirError` 均使用 Go 的自定义错误类型对齐。

### 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| - | - | - | - | Config 模块实现高度严谨，未发现显著 P0/P1/P2 功能缺陷 | - | - |

---

## 2. Gateway 模块审计报告

### 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 61 | 60 | ~98% |
| 总行数 | ~35000 | ~14000 | ~40% (TS包含大量类型声名，Go更为聚合) |

### 逐文件对照

| 状态 | TS文件/模块 | Go实现对应 | 评估说明 |
|------|-------------|------------|----------|
| ✅ FULL | `server-methods/*.ts` | `server_methods_*.go` | JSON-RPC/WS 方法注册1:1实现 |
| ✅ FULL | `ws-connection.ts` | `ws_server.go` | WebSocket 连接管理、心跳、协议协商对齐 |
| ✅ FULL | `auth.ts` | `auth.go` | Preshared-Key/Device Auth 逻辑完整 |
| ✅ FULL | `session-utils.fs.ts` | `session_utils_fs.go` | JSON会话存取逻辑完全对等 |
| ✅ FULL | `server-http.ts` | `server_http.go` | HTTP 路由注册对等 |
| ✅ FULL | `hooks.ts`, `hooks-mapping.ts` | `hooks*.go` | Hook 生命周期扩展对齐 |
| 🔄 REFACTORED | 众多单文件工具类 | 整合至各相关 `server*.go` | Go 架构在方法上做了平滑的拆分与聚合 |

### 隐藏依赖审计

1. **npm 包黑盒行为**: 无特殊依赖。
2. **全局状态/单例**: WebSocket 广播中心在 Go 使用 `Broadcaster` 结构体，替代全局可变状态。
3. **事件总线/回调链**: 事件分发如 `presence.changed` 使用原生 Go Channel 与回调。
4. **环境变量依赖**: 通过配置模块注入，无直读环境变量。
5. **文件系统约定**: `fs.stat`/`readdir` 用于 `/logs` 及会话读取，Go 中全部使用 `os` 对应方法提供。
6. **协议/消息格式**: WebSocket 帧的 `type`, `event`, `error` 格式严格对齐协议文档。
7. **错误处理约定**: `formatError(err)` 及各类 `throw new Error()` 对齐为 `NewErrorShape`。

### 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| - | - | - | - | Gateway 模块核心流程已完全接管，WebSocket 对话帧结构正常。未发现缺失。 | - | - |

---

## 3. Security 模块审计报告

### 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 8 | 10 | >100% (Go 有额外的拆分) |
| 总行数 | ~3000 | ~3000 | ~100% |

### 逐文件对照

| 状态 | TS文件/模块 | Go实现对应 | 评估说明 |
|------|-------------|------------|----------|
| ✅ FULL | `audit.ts`, `audit-extra.ts`, `audit-fs.ts` | `audit.go`, `audit_extra.go`, `audit_fs.go` | 全局安全审计项与深度扫描规则一致 |
| ✅ FULL | `fix.ts` | `fix.go` | 针对审计报告的自动修复逻辑全面对齐 |
| ✅ FULL | `skill-scanner.ts` | `skill_scanner.go` | 针对插件源码的恶意代码模式扫描对齐 |
| ✅ FULL | `windows-acl.ts` | `windows_acl.go` | 仅在Windows下的 icacls 权限管理对齐 |
| ✅ FULL | `external-content.ts` | `external_content.go`, `ssrf.go` | 外部 URL SSRF 过滤与验证对齐 |

### 隐藏依赖审计

1. **npm 包黑盒行为**: 无特殊依赖，依赖标准库。
2. **全局状态/单例**: 无静态陷阱。
3. **事件总线/回调链**: 无。
4. **环境变量依赖**: 仅读取特定的凭证路径配置。
5. **文件系统约定**: 大量使用 `fs.lstat`, `fs.chmod`, `fs.readdir` 进行权限探查，Go 中对应的 `os.Lstat` 和 `os.Chmod` 完美对齐。
6. **协议/消息格式**: 无特殊。
7. **错误处理约定**: TS中抛出的 `Error` 被用作收集清单，Go 中以切片收集 `error` (Warnings)。

### 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| - | - | - | - | Security 模块完全一致，各项安全扫描规则均已迁移至 Go。 | - | - |

---

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- W1 模块汇总评级: **A** (各模块与 TS 高度一致，功能对齐完成，符合提测与预发布标准。)
