---
summary: "检查可用技能及其资格状态"
read_when:
  - You want to see which skills are available and eligible
  - You're debugging missing skill requirements
title: "skills"
status: active
arch: go-backend
source: cmd_skills.go
---

> [!WARNING]
> **架构状态：⚠️ Go Gateway 端实现** — 对应 `backend/cmd/openacosmi/cmd_skills.go`。

# `openacosmi skills`

Inspect skills (bundled + workspace + managed overrides) and see what’s eligible vs missing requirements.

Related:

- Skills system: [Skills](/tools/skills)
- Skills config: [Skills config](/tools/skills-config)
- ClawHub installs: [ClawHub](/tools/clawhub)

## Commands

```bash
openacosmi skills list
openacosmi skills list --eligible
openacosmi skills info <name>
openacosmi skills check
```
