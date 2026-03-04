---
summary: "macOS 应用日志系统和调试日志"
read_when:
  - 调试 macOS 应用问题
  - 查看日志输出
title: "日志"
---

# macOS 应用日志

## 日志子系统

macOS 应用使用 Apple 的 `os.log` 框架：

- 子系统：`bot.molt`
- 分类示例：`WebChatSwiftUI`、`VoiceWake`、`Gateway`、`Node`

## 查看日志

### 快捷脚本

```bash
./scripts/clawlog.sh
```

### 手动查看

```bash
log stream --predicate 'subsystem == "bot.molt"' --level debug
```

### 按分类过滤

```bash
log stream --predicate 'subsystem == "bot.molt" AND category == "VoiceWake"'
```

## Gateway 日志

Go Gateway 的 launchd 日志位于：

```
/tmp/openacosmi/openacosmi-gateway.log
```

也可使用 Rust CLI：

```bash
openacosmi logs --follow
```

## 调试设置

macOS 应用的调试设置面板提供：

- 日志级别调整
- Gateway 日志路径显示
- 连接诊断信息
