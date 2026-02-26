# Telegram P5 审计完成报告（TS 未映射文件确认）

## 审计范围

7 项 TS 未映射文件确认。

## 审计日期

2026-02-24

## 确认结果

| # | TS 文件 | Go 去向 | 状态 |
|---|---------|---------|------|
| 1 | index.ts (5L) | 无需映射 | 纯 re-export 桶文件，Go package 内直接可见 |
| 2 | allowed-updates.ts (11L) | network.go AllowedUpdates() | 已合并，14 个更新类型完整硬编码 |
| 3 | fetch.ts (44L) | http_client.go NewTelegramHTTPClient() | 已合并，autoSelectFamily 通过 ResolveAutoSelectFamilyDecision 处理 |
| 4 | network-config.ts (38L) + network-errors.ts (151L) | network.go | 已合并，env vars + config 决策 + error BFS 分类 |
| 5 | proxy.ts (14L) | http_client.go L48-72 | 已合并，undici ProxyAgent → http.Transport.Proxy + golang.org/x/net/proxy (SOCKS5) |
| 6 | webhook-set.ts (42L) | webhook.go L17-42 | 已合并，SetTelegramWebhook + DeleteTelegramWebhook |
| 7 | extensions/telegram/ | 不存在 | TS 源中无此目录，误列项已确认 |

## 结论

7 项全部确认，无需修复。所有 TS 未映射文件均已正确合并至对应 Go 文件或无需映射。
