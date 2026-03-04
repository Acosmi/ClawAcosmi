---
summary: "初始化配置文件和 agent 工作区"
read_when:
  - You’re doing first-run setup without the full onboarding wizard
  - You want to set the default workspace path
title: "setup"
status: active
arch: rust-cli
crate: oa-cmd-supporting
---

> [!NOTE]
> **架构状态：✅ Rust CLI 已实现** — 对应 crate `oa-cmd-supporting`（`setup` 模块）。

# `openacosmi setup`

Initialize `~/.openacosmi/openacosmi.json` and the agent workspace.

Related:

- Getting started: [Getting started](/start/getting-started)
- Wizard: [Onboarding](/start/onboarding)

## Examples

```bash
openacosmi setup
openacosmi setup --workspace ~/.openacosmi/workspace
```

To run the wizard via setup:

```bash
openacosmi setup --wizard
```
