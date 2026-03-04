---
summary: "按频道分类的快速故障排查指南"
read_when:
  - 频道传输显示已连接但回复失败
  - 需要频道级别的检查
title: "频道故障排查（Channel Troubleshooting）"
---

# 频道故障排查

> [!IMPORTANT]
> **架构状态**：频道运行时由 **Go Gateway**（`backend/internal/channels/`）管理。
> 诊断命令通过 **Rust CLI**（`cli-rust/crates/oa-cmd-channels/`）执行。

当频道已连接但行为异常时使用本页面。

## 诊断命令阶梯

按顺序运行：

```bash
openacosmi status
openacosmi gateway status
openacosmi logs --follow
openacosmi doctor
openacosmi channels status --probe
```

正常基线：

- `Runtime: running`
- `RPC probe: ok`
- 频道探测显示 connected/ready

## WhatsApp

### WhatsApp 故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| 已连接但 DM 无回复 | `openacosmi pairing list whatsapp` | 批准发送者或切换 DM 策略/白名单。 |
| 群消息被忽略 | 检查配置中的 `requireMention` + 提及模式 | 在群中 @提及 bot 或放宽该群的提及策略。 |
| 随机断连/重登录循环 | `openacosmi channels status --probe` + 日志 | 重新登录并检查凭证目录是否健康。 |

完整排查：[/channels/whatsapp#troubleshooting-quick](/channels/whatsapp#troubleshooting-quick)

## Telegram

### Telegram 故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| `/start` 后无可用回复流 | `openacosmi pairing list telegram` | 批准配对或修改 DM 策略。 |
| Bot 在线但群保持沉默 | 检查提及要求和 bot 隐私模式 | 关闭隐私模式以获取群可见性，或 @提及 bot。 |
| 发送失败，网络错误 | 检查日志中的 Telegram API 调用失败 | 修复到 `api.telegram.org` 的 DNS/IPv6/代理路由。 |

完整排查：[/channels/telegram#troubleshooting](/channels/telegram#troubleshooting)

## Discord

### Discord 故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| Bot 在线但无 Guild 回复 | `openacosmi channels status --probe` | 允许 guild/channel 并验证 Message Content Intent。 |
| 群消息被忽略 | 检查日志中的提及门控丢弃 | @提及 bot 或设置 guild/channel `requireMention: false`。 |
| DM 回复缺失 | `openacosmi pairing list discord` | 批准 DM 配对或调整 DM 策略。 |

完整排查：[/channels/discord#troubleshooting](/channels/discord#troubleshooting)

## Slack

### Slack 故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| Socket Mode 已连接但无响应 | `openacosmi channels status --probe` | 验证 App Token + Bot Token 及所需权限范围。 |
| DM 被阻止 | `openacosmi pairing list slack` | 批准配对或放宽 DM 策略。 |
| 频道消息被忽略 | 检查 `groupPolicy` 和频道白名单 | 允许该频道或切换策略为 `open`。 |

完整排查：[/channels/slack#troubleshooting](/channels/slack#troubleshooting)

## iMessage 和 BlueBubbles

### iMessage/BlueBubbles 故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| 无入站事件 | 验证 webhook/服务器可达性和应用权限 | 修复 webhook URL 或 BlueBubbles 服务器状态。 |
| macOS 上可发送但无法接收 | 检查 macOS Messages 自动化隐私权限 | 重新授予 TCC 权限并重启频道进程。 |
| DM 发送者被阻止 | `openacosmi pairing list imessage` 或 `openacosmi pairing list bluebubbles` | 批准配对或更新白名单。 |

完整排查：

- [/channels/imessage#troubleshooting-macos-privacy-and-security-tcc](/channels/imessage#troubleshooting-macos-privacy-and-security-tcc)
- [/channels/bluebubbles#troubleshooting](/channels/bluebubbles#troubleshooting)

## Signal

### Signal 故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| 守护进程可达但 bot 沉默 | `openacosmi channels status --probe` | 验证 `signal-cli` 守护进程 URL/账户和接收模式。 |
| DM 被阻止 | `openacosmi pairing list signal` | 批准发送者或调整 DM 策略。 |
| 群回复不触发 | 检查群白名单和提及模式 | 添加发送者/群组或放宽门控。 |

完整排查：[/channels/signal#troubleshooting](/channels/signal#troubleshooting)

## Matrix

### Matrix 故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| 已登录但忽略房间消息 | `openacosmi channels status --probe` | 检查 `groupPolicy` 和房间白名单。 |
| DM 不处理 | `openacosmi pairing list matrix` | 批准发送者或调整 DM 策略。 |
| 加密房间失败 | 验证加密模块和加密设置 | 启用加密支持并重新加入/同步房间。 |

完整排查：[/channels/matrix#troubleshooting](/channels/matrix#troubleshooting)

## 飞书 / Feishu

### 飞书故障特征

| 症状 | 最快检查 | 修复 |
|------|---------|------|
| WebSocket 已连接但无响应 | `openacosmi channels status --probe` | 验证 App ID/Secret 和事件订阅配置。 |
| 多模态消息无法下载 | 检查飞书应用权限 | 确认 `im:message` + `im:resource` 权限已授予。 |
| 群消息被忽略 | 检查群策略和提及模式 | 在群中 @提及 bot 或调整 `groupPolicy`。 |

完整排查：[/channels/feishu#troubleshooting](/channels/feishu#troubleshooting)
