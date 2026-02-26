---
summary: "CLI reference for `openacosmi config` (get/set/unset config values)"
read_when:
  - You want to read or edit config non-interactively
title: "config"
---

# `openacosmi config`

Config helpers: get/set/unset values by path. Run without a subcommand to open
the configure wizard (same as `openacosmi configure`).

## Examples

```bash
openacosmi config get browser.executablePath
openacosmi config set browser.executablePath "/usr/bin/google-chrome"
openacosmi config set agents.defaults.heartbeat.every "2h"
openacosmi config set agents.list[0].tools.exec.node "node-id-or-name"
openacosmi config unset tools.web.search.apiKey
```

## Paths

Paths use dot or bracket notation:

```bash
openacosmi config get agents.defaults.workspace
openacosmi config get agents.list[0].id
```

Use the agent list index to target a specific agent:

```bash
openacosmi config get agents.list
openacosmi config set agents.list[1].tools.exec.node "node-id-or-name"
```

## Values

Values are parsed as JSON5 when possible; otherwise they are treated as strings.
Use `--json` to require JSON5 parsing.

```bash
openacosmi config set agents.defaults.heartbeat.every "0m"
openacosmi config set gateway.port 19001 --json
openacosmi config set channels.whatsapp.groups '["*"]' --json
```

Restart the gateway after edits.
