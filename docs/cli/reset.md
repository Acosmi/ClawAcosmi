---
summary: "CLI reference for `openacosmi reset` (reset local state/config)"
read_when:
  - You want to wipe local state while keeping the CLI installed
  - You want a dry-run of what would be removed
title: "reset"
---

# `openacosmi reset`

Reset local config/state (keeps the CLI installed).

```bash
openacosmi reset
openacosmi reset --dry-run
openacosmi reset --scope config+creds+sessions --yes --non-interactive
```
