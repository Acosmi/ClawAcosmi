---
summary: "管理 Gateway 插件（安装/启用/禁用/更新）"
read_when:
  - You want to install, enable/disable, or update plugins
  - You want to list or inspect plugins
title: "plugins"
status: active
arch: go-backend
source: cmd_plugins.go
---

> [!WARNING]
> **架构状态：⚠️ Go Gateway 端实现** — 对应 `backend/cmd/openacosmi/cmd_plugins.go`。
> Rust CLI 尚未迁移此子命令。

# `openacosmi plugins`

Manage Gateway plugins/extensions (loaded in-process).

Related:

- Plugin system: [Plugins](/tools/plugin)
- Plugin manifest + schema: [Plugin manifest](/plugins/manifest)
- Security hardening: [Security](/gateway/security)

## Commands

```bash
openacosmi plugins list
openacosmi plugins info <id>
openacosmi plugins enable <id>
openacosmi plugins disable <id>
openacosmi plugins doctor
openacosmi plugins update <id>
openacosmi plugins update --all
```

Bundled plugins ship with OpenAcosmi but start disabled. Use `plugins enable` to
activate them.

All plugins must ship a `openacosmi.plugin.json` file with an inline JSON Schema
(`configSchema`, even if empty). Missing/invalid manifests or schemas prevent
the plugin from loading and fail config validation.

### Install

```bash
openacosmi plugins install <path-or-spec>
```

Security note: treat plugin installs like running code. Prefer pinned versions.

Supported archives: `.zip`, `.tgz`, `.tar.gz`, `.tar`.

Use `--link` to avoid copying a local directory (adds to `plugins.load.paths`):

```bash
openacosmi plugins install -l ./my-plugin
```

### Update

```bash
openacosmi plugins update <id>
openacosmi plugins update --all
openacosmi plugins update <id> --dry-run
```

Updates only apply to plugins installed from npm (tracked in `plugins.installs`).
