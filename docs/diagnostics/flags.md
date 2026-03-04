---
summary: "诊断标志：用于定向调试日志的开关"
read_when:
  - 需要定向调试日志但不想提高全局日志级别
  - 需要捕获子系统特定日志用于排查
title: "诊断标志"
status: active
arch: go-gateway
---

# 诊断标志

> [!NOTE]
> **架构状态**：诊断标志由 **Go Gateway** 日志子系统实现（`backend/internal/logging/`）。

诊断标志允许你启用定向调试日志，而无需开启全局详细日志。标志是选择性启用的，除非子系统检查它们，否则不会产生任何效果。

## 工作原理

- 标志是字符串（不区分大小写）。
- 可通过配置文件或环境变量启用标志。
- 支持通配符：
  - `telegram.*` 匹配 `telegram.http`
  - `*` 启用所有标志

## 通过配置启用

```json
{
  "diagnostics": {
    "flags": ["telegram.http"]
  }
}
```

多个标志：

```json
{
  "diagnostics": {
    "flags": ["telegram.http", "gateway.*"]
  }
}
```

更改标志后需重启 Gateway。

## 环境变量覆盖（一次性）

```bash
OPENACOSMI_DIAGNOSTICS=telegram.http,telegram.payload
```

禁用所有标志：

```bash
OPENACOSMI_DIAGNOSTICS=0
```

## 日志输出位置

标志将日志输出到标准诊断日志文件。默认路径：

```
/tmp/openacosmi/openacosmi-YYYY-MM-DD.log
```

如果设置了 `logging.file`，则使用该路径。日志格式为 JSONL（每行一个 JSON 对象）。敏感信息脱敏仍按 `logging.redactSensitive` 配置生效。

## 提取日志

选取最新日志文件：

```bash
ls -t /tmp/openacosmi/openacosmi-*.log | head -n 1
```

过滤 Telegram HTTP 诊断信息：

```bash
rg "telegram http error" /tmp/openacosmi/openacosmi-*.log
```

或在复现问题时实时跟踪：

```bash
tail -f /tmp/openacosmi/openacosmi-$(date +%F).log | rg "telegram http error"
```

对于远程 Gateway，也可使用 Rust CLI 命令 `openacosmi logs --follow`（参见 [CLI logs](/cli/logs)）。

## 注意事项

- 如果 `logging.level` 设置为高于 `warn`，这些日志可能被抑制。默认 `info` 级别即可。
- 标志可安全保持启用状态；它们仅影响特定子系统的日志量。
- 使用 [日志配置](/logging) 更改日志目标、级别和脱敏设置。
