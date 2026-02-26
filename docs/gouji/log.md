# 日志模块架构文档

> 最后更新：2026-02-26 | 代码级审计完成 | 4 源文件, 1 测试文件, ~680 行

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/pkg/log/` |
| Go 源文件数 | 4 |
| Go 测试文件数 | 1 |
| 总行数 | ~680 |
| 基础 | Go 标准库 `log/slog` |
| 对齐 TS | `src/logging/` 目录 |

统一日志库，基于 Go `log/slog` 封装，继承 TypeScript 版的 `createSubsystemLogger` 模式。

## 二、文件索引

| 文件 | 行 | 职责 |
|------|---|------|
| `log.go` | ~200 | 全局日志状态、级别控制、子系统 logger、同步双输出 (stderr + 文件) |
| `levels.go` | ~100 | 日志级别定义 (debug/info/warn/error)、级别规范化解析 |
| `file.go` | ~200 | `FileWriter`：日志文件轮转、按日期自动滚动、路径管理 |
| `redact.go` | ~180 | 敏感信息脱敏：API Key/Token/Password/PEM 密钥自动屏蔽 |

## 三、核心设计

### 全局状态

```go
var (
    globalLevel      types.LogLevel = types.LogInfo  // 全局最低日志级别
    globalFileWriter *FileWriter                      // 文件输出器
)
```

### 双输出架构

每条日志同时写入：

1. **stderr** — 控制台实时查看
2. **日志文件** — 通过 `FileWriter` 持久化

### 子系统 Logger

`CreateSubsystemLogger(subsystem)` 创建带子系统标签的 slog logger，支持按子系统过滤。

## 四、敏感信息脱敏 (redact.go)

### 脱敏规则

```go
const (
    redactMinLength = 18   // 短于此长度不脱敏
    redactKeepStart = 6    // 保留前 6 字符
    redactKeepEnd   = 4    // 保留后 4 字符
)
// sk-abc123...xyz → sk-abc1****xyz
```

### 匹配模式 `DefaultRedactPatterns`

覆盖多种敏感信息格式：

- ENV 赋值：`KEY=value`
- JSON 字段：`"apiKey": "..."`
- CLI 参数：`--token xxx`
- Auth Header：`Bearer xxx`
- PEM 密钥块
- 常见 token 前缀：`sk-`, `ghp_`, `op_` 等

### 脱敏模式

| 模式 | 行为 |
|------|------|
| `off` | 关闭脱敏 |
| `tools` | 仅对工具输出脱敏（默认）|
