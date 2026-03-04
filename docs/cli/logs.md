---
summary: "远程尾随 Gateway 文件日志（通过 RPC）"
read_when:
  - You want to tail gateway logs remotely
title: "logs"
status: active
arch: go-backend
source: cmd_logs.go
---

> [!WARNING]
> **架构状态：⚠️ Go Gateway 端实现** — 对应 `backend/cmd/openacosmi/cmd_logs.go`。
> Rust CLI 尚未迁移此子命令。

# `openacosmi logs`

Tail Gateway file logs over RPC (works in remote mode).

Related:

- Logging overview: [Logging](/logging)

## Examples

```bash
openacosmi logs
openacosmi logs --follow
openacosmi logs --json
openacosmi logs --limit 500
```
