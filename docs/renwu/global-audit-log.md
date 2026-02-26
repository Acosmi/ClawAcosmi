# 全局审计报告 — Log 模块

## 概览

| 维度 | TS (`src/log*`, `src/gateway/*log*`) | Go (`backend/pkg/log`, `backend/internal/gateway`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 10+ | 6+ | 架构对齐，局部实现有差异 |
| 总行数 | ~1500 | ~562 | 85% 逻辑覆盖 |

### 架构演进

本模块在 TS 中包含以下功能点，且在 Go 中做了相应转化：

1. **基础 Log 框架**: TS 使用了 `tslog` 并魔改了 subsystem 机制。Go 自行使用 `log/slog` 构建了 `pkg/log` 组件，支持全局级别控制、双写（Console + JSON File）和相同的子系统继承模式。
2. **WebSocket 日志采集**: TS 提供 `auto/compact/full` 三种级别的交互日志追踪，并在 `ws-log.ts` 中针对 Agent 的 stream 推流做了极简摘要。Go 的 `ws_log.go` 实现完整复刻了这一套机制。
3. **敏感信息脱敏**: 正则脱敏逻辑（API Keys, PEMs）已在 `ws_log.go` 的 `RedactSensitiveText` 中 1:1 还原。
4. **RPC 尾部流**: `logs.tail` 接口（分页、按字节或游标截取日志结尾段）均精确对齐。

## 差异清单

### P2 功能降级：`openacosmi logs --follow` 变为仅限本机

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| LOG-1 | **CLI 流式追踪原理降级** | `logs-cli.ts` 通过轮询 RPC (`logs.tail`) 来追踪新日志。如果 Gateway 是远端服务，CLI 同样可以跨域获取实时追踪。 | `cmd_logs.go` 在处理 `--follow` 时，**完全放弃了 RPC，退化为本地 `os.Open` 文件读取并轮询 I/O**。这导致 Go 版 CLI 的 `-f` 命令无法用于监听远程 Gateway。 | **需修复 (P2)**。应该在 `--follow` 模式下默认走 RPC Polling（如果配置了 Gateway URL），或至少在远端模式下提供 RPC Polling 的支持。 |

### P3 细微差异

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| LOG-2 | **Log CLI 的本地 Fallback** | 不支持。如果 Gateway 未启动（无法接受 RPC），`openacosmi logs` 直接抛出网络无法连接错误。 | `tailLogs` 若 RPC 拿不到，自动回退到本地直接读 `gateway.log`。 | 加分项，提升了本地排查体验。无需修复，属于正向重构。 |
| LOG-3 | **非 JSON 日志行降级解析** | `parse-log-line.ts` 提供鲁棒的 JSON 提取和着色。 | `cmd_logs.go` 提供 `extractLogLevel` 来暴力查抄 `[ERROR]` 或者 `LEVEL=INFO` 字符串以强行过滤级别。 | 加分项，可处理底层框架不严格崩溃时的日志级别判断。无需修复。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | 未引入特殊的非预期环境变量（沿用 Config 载入体系） | 通过。 |
| **2. 并发安全** | WS 暂态追踪 (Inflight Maps) 和全局 Log Level 设置。 | Go 使用了 `sync.Mutex` (`globalWsLog.mu`) 控制 `wsInflightOptimized` 和 `wsInflightCompact`。`log/slog` 自身支持并发写。并发安全。 |
| **3. 第三方包黑盒** | 没有复杂的第三方包差异 (TS `tslog` 对标 Go 标准库 `slog`)。 | 无黑盒隐患。对标准库的应用非常规范。 |

## 下一步建议

Log 模块核心功能运转良好，文件留存格式兼容。仅留下一个 `openacosmi logs -f` 无法针对远程 Gateway 生效的缺陷 (P2)，可记录为 deferred item。全局向导可继续往下推下一步。
