---
summary: "CLI reference for `openacosmi logs` (tail gateway logs via RPC)"
read_when:
  - You need to tail Gateway logs remotely (without SSH)
  - You want JSON log lines for tooling
title: "logs"
---

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
