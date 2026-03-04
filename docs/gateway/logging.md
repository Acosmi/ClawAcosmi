---
summary: "日志输出、文件日志、WS 日志模式与控制台格式"
read_when:
  - 修改日志输出或格式
  - 调试 CLI 或 Gateway 输出
title: "日志（Logging）"
---

# 日志

> [!IMPORTANT]
> **架构状态**：日志由 **Go Gateway** 使用 `log/slog` 实现（`backend/internal/gateway/ws_log.go`），
> 文件日志通过 `pkg/log/` 管理。

用户概览见 [/logging](/logging)。

OpenAcosmi 有两个日志"输出面"：

- **控制台输出**（终端 / 调试 UI 中可见）。
- **文件日志**（JSON 行格式），由 Gateway 日志器写入。

## 文件日志器

- 默认滚动日志文件位于 `/tmp/openacosmi/`（按日）：`openacosmi-YYYY-MM-DD.log`
  - 日期使用 Gateway 宿主的本地时区。
- 日志文件路径和级别可通过 `~/.openacosmi/openacosmi.json` 配置：
  - `logging.file`
  - `logging.level`

文件格式为每行一个 JSON 对象。

控制 UI 的日志标签通过 Gateway RPC（`logs.tail`）拉取此文件。CLI 同样：

```bash
openacosmi logs --follow
```

**Verbose 与日志级别**

- **文件日志**由 `logging.level` 独立控制。
- `--verbose` 仅影响**控制台输出**和 WS 日志模式。
- 要在文件日志中捕获 verbose 级别的细节，设置 `logging.level` 为 `debug` 或 `trace`。

## 控制台捕获

Go Gateway 通过 `slog` 将日志同时输出到 stdout/stderr 和文件日志。

可独立调整控制台输出：

- `logging.consoleLevel`（默认 `info`）
- `logging.consoleStyle`（`pretty` | `compact` | `json`）

## 工具摘要脱敏

详细的工具摘要可在到达控制台流之前掩码敏感 token。仅影响**工具输出**，不改变文件日志。

- `logging.redactSensitive`：`off` | `tools`（默认：`tools`）
- `logging.redactPatterns`：正则字符串数组（覆盖默认值）

## Gateway WebSocket 日志

Gateway 以两种模式打印 WS 协议日志（`ws_log.go`）：

- **正常模式（无 `--verbose`）**：仅打印"有趣"的 RPC 结果：
  - 错误（`ok=false`）
  - 慢调用（默认阈值：`>= 50ms`）
  - 解析错误
- **Verbose 模式（`--verbose`）**：打印所有 WS 请求/响应流量。

### WS 日志模式

- `--ws-log auto`（默认）
- `--ws-log compact`：紧凑模式（配对请求/响应）
- `--ws-log full`：完整帧输出

```bash
# 优化模式（仅错误/慢调用）
openacosmi gateway start

# 所有 WS 流量（紧凑）
openacosmi gateway start --verbose --ws-log compact
```

## 控制台格式

Go Gateway 使用 `slog` 子系统前缀，保持输出分组和可扫描：

- **子系统前缀**（`[gateway]`、`[channels]`、`[tailscale]`）
- **子系统颜色**（按子系统固定）+ 级别着色
- **TTY 感知**：TTY 时自动彩色，尊重 `NO_COLOR`
- **日志级别**：控制台级别独立于文件级别
