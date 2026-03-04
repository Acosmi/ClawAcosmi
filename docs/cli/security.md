---
summary: "安全审计并修复配置/状态问题"
read_when:
  - You want to audit and fix common security issues
title: "security"
status: active
arch: go-backend
source: cmd_security.go
---

> [!WARNING]
> **架构状态：⚠️ Go Gateway 端实现** — 对应 `backend/cmd/openacosmi/cmd_security.go`。

# `openacosmi security`

Security tools (audit + optional fixes).

Related:

- Security guide: [Security](/gateway/security)

## Audit

```bash
openacosmi security audit
openacosmi security audit --deep
openacosmi security audit --fix
```

The audit warns when multiple DM senders share the main session and recommends **secure DM mode**: `session.dmScope="per-channel-peer"` (or `per-account-channel-peer` for multi-account channels) for shared inboxes.
It also warns when small models (`<=300B`) are used without sandboxing and with web/browser tools enabled.
