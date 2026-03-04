---
summary: "渠道/会话综合诊断（服务状态、更新、用量）"
read_when:
  - You want a quick health/status overview of channels and sessions
title: "status"
status: active
arch: rust-cli
crate: oa-cmd-status
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-status`（97+ 测试）。
> 入口二进制：`openacosmi`（Rust），通过 RPC 探测 Go Gateway 获取状态。

# `openacosmi status`

Diagnostics for channels + sessions.

```bash
openacosmi status
openacosmi status --all
openacosmi status --deep
openacosmi status --usage
```

Notes:

- `--deep` runs live probes (WhatsApp Web + Telegram + Discord + Google Chat + Slack + Signal).
- Output includes per-agent session stores when multiple agents are configured.
- Overview includes Gateway + node host service install/runtime status when available.
- Overview includes update channel + git SHA (for source checkouts).
- Update info surfaces in the Overview; if an update is available, status prints a hint to run `openacosmi update` (see [Updating](/install/updating)).
