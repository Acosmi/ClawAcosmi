---
summary: "卸载 Gateway 服务和本地数据（保留 CLI 本身）"
read_when:
  - You want to remove the Gateway service and/or local data
  - You want a dry-run of what would be removed
title: "uninstall"
status: active
arch: rust-cli
crate: oa-cmd-supporting
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-supporting`（`uninstall` 模块）。

# `openacosmi uninstall`

Uninstall the gateway service + local data (CLI remains).

```bash
openacosmi uninstall
openacosmi uninstall --all --yes
openacosmi uninstall --dry-run
```
