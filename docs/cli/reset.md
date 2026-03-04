---
summary: "重置本地配置/状态（保留 CLI 本身）"
read_when:
  - You want to wipe local state while keeping the CLI installed
  - You want a dry-run of what would be removed
title: "reset"
status: active
arch: rust-cli
crate: oa-cmd-supporting
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-supporting`（`reset` 模块）。

# `openacosmi reset`

Reset local config/state (keeps the CLI installed).

```bash
openacosmi reset
openacosmi reset --dry-run
openacosmi reset --scope config+creds+sessions --yes --non-interactive
```
