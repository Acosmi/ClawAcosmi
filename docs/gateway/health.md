---
summary: "频道连接健康检查步骤"
read_when:
  - 诊断频道连接健康状况
title: "健康检查（Health Checks）"
---

# 健康检查（CLI）

> [!IMPORTANT]
> **架构状态**：健康检查由 **Go Gateway**（`backend/internal/gateway/boot.go` 的 `GetHealthStatus`）提供，
> CLI 命令由 `cmd/openacosmi/cmd_status.go` 实现。

验证频道连接状态的快速指南。

## 快速检查

- `openacosmi status` — 本地摘要：Gateway 可达性/模式、频道状态、会话和最近活动。
- `openacosmi status --all` — 完整本地诊断（只读、安全可粘贴用于调试）。
- `openacosmi status --deep` — 同时探测运行中的 Gateway（支持按频道探测）。
- `openacosmi health --json` — 向运行中的 Gateway 请求完整健康快照（通过 WS RPC）。
- 发送 `/status` 消息到 WhatsApp/WebChat 获取状态回复（不触发 Agent）。
- 日志：查看 `/tmp/openacosmi/openacosmi-*.log` 并过滤 `web-heartbeat`、`web-reconnect` 等关键词。

## 深度诊断

- 凭据检查：`ls -l ~/.openacosmi/credentials/whatsapp/<accountId>/creds.json`（修改时间应较新）。
- 会话存储：`ls -l ~/.openacosmi/agents/<agentId>/sessions/sessions.json`（路径可在配置中覆盖）。
- 重新链接：`openacosmi channels logout && openacosmi channels login --verbose`（当日志中出现状态码 409–515 或 `loggedOut` 时）。

## 故障处理

- `logged out` 或状态码 409–515 → 使用 `openacosmi channels logout` 再 `openacosmi channels login` 重新链接。
- Gateway 不可达 → 启动：`openacosmi gateway start --port 19001`（端口被占用时用 `--force`）。
- 无入站消息 → 确认频道已连接、发送者在允许列表中（`channels.<channel>.allowFrom`）；群聊检查允许列表 + 提及规则。

## 专用 health 命令

`openacosmi health --json` 向运行中的 Gateway 请求健康快照（CLI 不直接连接频道）。
报告内容包括：频道认证状态、会话存储摘要和探测耗时。
Gateway 不可达或探测失败/超时时返回非零退出码。使用 `--timeout <ms>` 覆盖默认 10 秒超时。
