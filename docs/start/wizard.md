---
summary: "CLI 引导向导：引导式设置 Gateway、工作区、通道和技能"
read_when:
  - 运行或配置引导向导
  - 设置新机器
title: "引导向导 (CLI)"
sidebarTitle: "引导: CLI"
status: active
arch: rust-cli+go-gateway
---

# 引导向导 (CLI)

> [!IMPORTANT]
> **架构状态**：引导向导由 **Rust CLI** (`oa-cmd-onboard` crate) 实现，配置 **Go Gateway** 服务。

引导向导是在 macOS、Linux 或 Windows（通过 WSL2；强烈推荐）上设置 OpenAcosmi 的**推荐**方式。
它在一个引导式流程中配置本地 Gateway 或远程 Gateway 连接，以及通道、技能和工作区默认值。

```bash
openacosmi onboard
```

<Info>
Fastest first chat: open the Control UI (no channel setup needed). Run
`openacosmi dashboard` and chat in the browser. Docs: [Dashboard](/web/dashboard).
</Info>

To reconfigure later:

```bash
openacosmi configure
openacosmi agents add <name>
```

<Note>
`--json` does not imply non-interactive mode. For scripts, use `--non-interactive`.
</Note>

<Tip>
Recommended: set up a Brave Search API key so the agent can use `web_search`
(`web_fetch` works without a key). Easiest path: `openacosmi configure --section web`
which stores `tools.web.search.apiKey`. Docs: [Web tools](/tools/web).
</Tip>

## QuickStart vs Advanced

The wizard starts with **QuickStart** (defaults) vs **Advanced** (full control).

<Tabs>
  <Tab title="QuickStart (defaults)">
    - Local gateway (loopback)
    - Workspace default (or existing workspace)
    - Gateway port **18789**
    - Gateway auth **Token** (auto‑generated, even on loopback)
    - Tailscale exposure **Off**
    - Telegram + WhatsApp DMs default to **allowlist** (you'll be prompted for your phone number)
  </Tab>
  <Tab title="Advanced (full control)">
    - Exposes every step (mode, workspace, gateway, channels, daemon, skills).
  </Tab>
</Tabs>

## What the wizard configures

**Local mode (default)** walks you through these steps:

1. **Model/Auth** — Anthropic API key (recommended), OAuth, OpenAI, or other providers. Pick a default model.
2. **Workspace** — Location for agent files (default `~/.openacosmi/workspace`). Seeds bootstrap files.
3. **Gateway** — Port, bind address, auth mode, Tailscale exposure.
4. **Channels** — WhatsApp, Telegram, Discord, Google Chat, Mattermost, Signal, BlueBubbles, or iMessage.
5. **守护进程** — 安装 LaunchAgent (macOS) 或 systemd 用户服务单元 (Linux/WSL2)，运行 Go Gateway (`acosmi`) 进程。
6. **Health check** — Starts the Gateway and verifies it's running.
7. **Skills** — Installs recommended skills and optional dependencies.

<Note>
Re-running the wizard does **not** wipe anything unless you explicitly choose **Reset** (or pass `--reset`).
If the config is invalid or contains legacy keys, the wizard asks you to run `openacosmi doctor` first.
</Note>

**Remote mode** only configures the local client to connect to a Gateway elsewhere.
It does **not** install or change anything on the remote host.

## Add another agent

Use `openacosmi agents add <name>` to create a separate agent with its own workspace,
sessions, and auth profiles. Running without `--workspace` launches the wizard.

What it sets:

- `agents.list[].name`
- `agents.list[].workspace`
- `agents.list[].agentDir`

Notes:

- Default workspaces follow `~/.openacosmi/workspace-<agentId>`.
- Add `bindings` to route inbound messages (the wizard can do this).
- Non-interactive flags: `--model`, `--agent-dir`, `--bind`, `--non-interactive`.

## Full reference

For detailed step-by-step breakdowns, non-interactive scripting, Signal setup,
RPC API, and a full list of config fields the wizard writes, see the
[Wizard Reference](/reference/wizard).

## Related docs

- CLI command reference: [`openacosmi onboard`](/cli/onboard)
- macOS app onboarding: [Onboarding](/start/onboarding)
- Agent first-run ritual: [Agent Bootstrapping](/start/bootstrapping)
