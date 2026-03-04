---
summary: "CLI reference for `openacosmi dns` (wide-area discovery helpers)"
read_when:
  - You want wide-area discovery (DNS-SD) via Tailscale + CoreDNS
  - You’re setting up split DNS for a custom discovery domain (example: openacosmi.internal)
title: "dns"
status: active
arch: rust-cli
---

> [!NOTE]
> **架构状态：✅ 已适配** — Rust CLI crate (oa-cmd-dns) + Go Gateway stub 均已注册。
> 命令功能为 stub 实现，待后续补充完整业务逻辑。

# `openacosmi dns`

DNS helpers for wide-area discovery (Tailscale + CoreDNS). Currently focused on macOS + Homebrew CoreDNS.

Related:

- Gateway discovery: [Discovery](/gateway/discovery)
- Wide-area discovery config: [Configuration](/gateway/configuration)

## Setup

```bash
openacosmi dns setup
openacosmi dns setup --apply
```
