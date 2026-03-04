---
summary: "通过 RPC 获取 Gateway 运行健康状态"
read_when:
  - You want to quickly check the running Gateway’s health
title: "health"
status: active
arch: rust-cli
crate: oa-cmd-health
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-health`（48+ 测试）。
> 入口二进制：`openacosmi`（Rust），通过 WebSocket RPC 探测 Go Gateway。

# `openacosmi health`

Fetch health from the running Gateway.

```bash
openacosmi health
openacosmi health --json
openacosmi health --verbose
```

Notes:

- `--verbose` runs live probes and prints per-account timings when multiple accounts are configured.
- Output includes per-agent session stores when multiple agents are configured.
