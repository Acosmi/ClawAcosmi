---
summary: "管理语义记忆索引和搜索"
read_when:
  - You want to check/rebuild the semantic memory index
  - You want to search memory embeddings
title: "memory"
status: active
arch: go-backend
source: cmd_memory.go
---

> [!WARNING]
> **架构状态：⚠️ Go Gateway 端实现** — 对应 `backend/cmd/openacosmi/cmd_memory.go`。
> Rust CLI 尚未迁移此子命令。

# `openacosmi memory`

Manage semantic memory indexing and search.
Provided by the active memory plugin (default: `memory-core`; set `plugins.slots.memory = "none"` to disable).

Related:

- Memory concept: [Memory](/concepts/memory)
- Plugins: [Plugins](/tools/plugin)

## Examples

```bash
openacosmi memory status
openacosmi memory status --deep
openacosmi memory status --deep --index
openacosmi memory status --deep --index --verbose
openacosmi memory index
openacosmi memory index --verbose
openacosmi memory search "release checklist"
openacosmi memory status --agent main
openacosmi memory index --agent main --verbose
```

## Options

Common:

- `--agent <id>`: scope to a single agent (default: all configured agents).
- `--verbose`: emit detailed logs during probes and indexing.

Notes:

- `memory status --deep` probes vector + embedding availability.
- `memory status --deep --index` runs a reindex if the store is dirty.
- `memory index --verbose` prints per-phase details (provider, model, sources, batch activity).
- `memory status` includes any extra paths configured via `memorySearch.extraPaths`.
