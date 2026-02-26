> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Signal TS→Go 文件映射表

| TS 文件 | 行数 | Go 文件 | 状态 |
|---------|------|---------|------|
| src/signal/accounts.ts | 92 | internal/channels/signal/accounts.go | 完成 |
| src/signal/identity.ts | 136 | internal/channels/signal/identity.go | 完成 |
| src/signal/client.ts | 195 | internal/channels/signal/client.go | 完成 |
| src/signal/daemon.ts | 103 | internal/channels/signal/daemon.go | 完成 |
| src/signal/probe.ts | 58 | internal/channels/signal/probe.go | 完成 |
| src/signal/format.ts | 239 | internal/channels/signal/format.go | 完成 |
| src/signal/send.ts | 282 | internal/channels/signal/send.go | 完成 |
| src/signal/send-reactions.ts | 216 | internal/channels/signal/send_reactions.go | 完成 |
| src/signal/reaction-level.ts | 72 | internal/channels/signal/reaction_level.go | 完成 |
| src/signal/sse-reconnect.ts | 81 | internal/channels/signal/sse_reconnect.go | 完成 |
| src/signal/monitor.ts | 401 | internal/channels/signal/monitor.go | 完成 |
| src/signal/monitor/event-handler.ts | 582 | internal/channels/signal/event_handler.go | 完成 |
| src/signal/monitor/event-handler.types.ts | 118 | internal/channels/signal/monitor_types.go | 完成 |
| (跨模块 DI) | — | internal/channels/signal/monitor_deps.go | 完成 |
| extensions/signal/src/channel.ts | 316 | internal/channels/dock.go (core channel) | 等价 |
| src/channels/plugins/actions/signal.ts | 147 | internal/channels/bridge/signal_actions.go | 完成 |
| (onboarding 子集) | — | internal/channels/onboarding_signal.go | 完成 |
