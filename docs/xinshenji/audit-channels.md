---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/channels (43 files + 12 subdirs, ~3000+ LOC core)
verdict: Pass with Notes
---

# 审计报告: channels — 多渠道模块

## 范围

- **目录**: `backend/internal/channels/`
- **文件数**: 43 顶层文件 + 12 子目录 (discord/48, telegram/43, slack/41, signal/21, whatsapp/19, line/16, bridge/14, imessage/13, feishu/8, dingtalk/5, wecom/5, ratelimit/2)
- **核心文件**: `channels.go`(227L), `dock.go`(358L), `normalize.go`(388L), `registry.go`(145L), `outbound.go`, `actions.go`, `status_issues.go`

## 审计发现

### [PASS] 正确性: 频道管理器并发安全 (channels.go)

- **位置**: `channels.go:57-227`
- **分析**: `Manager` 使用 `sync.Mutex` 保护所有状态操作。`RegisterPlugin`, `StartChannel`, `StopChannel`, `GetSnapshot`, `SendMessage` 均持锁操作。`SendMessage` 在查询完 plugin 后释放锁再调用发送，避免长时间持锁。
- **风险**: None

### [PASS] 正确性: 频道 Dock 行为配置 (dock.go)

- **位置**: `dock.go:88-158`
- **分析**: 7 个核心频道 dock 配置完整：Telegram (4000字/block streaming), Discord (2000字/streaming), Slack (4000字/streaming/threads), Signal (4000字/streaming), WhatsApp (4000字/polls/reactions), iMessage (4000字/reactions), GoogleChat (4000字/block streaming/threads)。每个频道的能力、限制、行为配置与实际平台匹配。
- **风险**: None

### [WARN] 性能: normalize.go 多处动态正则编译

- **位置**: `normalize.go:64`, `normalize.go:295-300`, `normalize.go:377`
- **分析**:
  - `NormalizeTelegramMessagingTarget` 内 `regexp.MustCompile` t.me 链接正则 (L64)
  - `LooksLikeSignalTargetID` 内 3 处 `regexp.MustCompile` (L295-300)
  - `LooksLikeIMessageTargetID` 内 `regexp.MustCompile` (L377)
  
  这些函数在消息处理路径上可能被频繁调用。
- **风险**: Medium
- **建议**: 所有正则提升为 package-level `var` 预编译。注意 `tgTargetRe`, `tgNumericRe` 等已正确预编译（L74-75），应将剩余改为一致风格。

### [PASS] 正确性: Discord 目标规范化 (normalize.go)

- **位置**: `normalize.go:99-166`
- **分析**: `NormalizeDiscordMessagingTarget` 正确处理了 5 种输入格式：mention (`<@123>`), `user:ID`, `channel:ID`, `discord:ID`, 裸数字 ID。`discord:` 前缀映射为 `user:` 与 TS 对齐。裸数字默认 `channel:` 也与 TS `defaultKind: "channel"` 一致。
- **风险**: None

### [PASS] 正确性: 频道注册表与别名 (registry.go)

- **位置**: `registry.go:1-144`
- **分析**: 12 个核心频道 + 10 个别名映射（tg→telegram, wa→whatsapp, lark→feishu 等）。`ResolveChannelIDByAlias` 先查精确匹配再查别名。`chatChannelOrder` 定义了 UI 展示排序。文中包含中文频道标签（飞书、钉钉、企业微信）。
- **风险**: None

### [PASS] 正确性: 插件扩展机制 (dock.go + channels.go)

- **位置**: `dock.go:160-177`, `channels.go:48-52`
- **分析**: `PluginChannelDockProvider` 通过 DI 注入插件频道 dock。`Plugin` 接口最小化（ID/Start/Stop），可选能力通过类型断言（`MessageSender`）实现。`ListChannelDocks` 合并核心频道和插件频道并排序。
- **风险**: None

### [WARN] 正确性: DefaultAccountID 重复定义

- **位置**: `channels.go:29` vs `routing/session_key.go:21`
- **分析**: `DefaultAccountID = "default"` 在 `channels` 和 `routing` 两个包中各定义了一份。目前值一致，但分散定义增加不一致风险。
- **风险**: Low
- **建议**: 统一到公共包（如 `infra` 或 `session`）。

## 总结

- **总发现**: 7 (5 PASS, 2 WARN, 0 FAIL)
- **阻断问题**: 无
- **建议**:
  1. `normalize.go` 中动态正则编译应提升为包级预编译 (Medium)
  2. `DefaultAccountID` 应统一定义 (Low)
- **结论**: **通过（附注释）** — 多渠道架构设计良好，插件扩展机制灵活。频道规范化覆盖 6 个主要平台。主要问题是部分正则未预编译。
