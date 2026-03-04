---
summary: "使用 Nix 声明式安装 OpenAcosmi"
read_when:
  - 需要可复现、可回滚的安装
  - 已经在使用 Nix/NixOS/Home Manager
  - 需要所有内容都固定并声明式管理
title: "Nix"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# Nix 安装

The recommended way to run OpenAcosmi with Nix is via **[nix-openacosmi](https://github.com/openacosmi/nix-openacosmi)** — a batteries-included Home Manager module.

## Quick Start

Paste this to your AI agent (Claude, Cursor, etc.):

```text
I want to set up nix-openacosmi on my Mac.
Repository: github:openacosmi/nix-openacosmi

What I need you to do:
1. Check if Determinate Nix is installed (if not, install it)
2. Create a local flake at ~/code/openacosmi-local using templates/agent-first/flake.nix
3. Help me create a Telegram bot (@BotFather) and get my chat ID (@userinfobot)
4. Set up secrets (bot token, Anthropic key) - plain files at ~/.secrets/ is fine
5. Fill in the template placeholders and run home-manager switch
6. Verify: launchd running, bot responds to messages

Reference the nix-openacosmi README for module options.
```

> **📦 Full guide: [github.com/openacosmi/nix-openacosmi](https://github.com/openacosmi/nix-openacosmi)**
>
> The nix-openacosmi repo is the source of truth for Nix installation. This page is just a quick overview.

## What you get

- Gateway + macOS app + tools (whisper, spotify, cameras) — all pinned
- Launchd service that survives reboots
- Plugin system with declarative config
- Instant rollback: `home-manager switch --rollback`

---

## Nix Mode Runtime Behavior

When `OPENACOSMI_NIX_MODE=1` is set (automatic with nix-openacosmi):

OpenAcosmi supports a **Nix mode** that makes configuration deterministic and disables auto-install flows.
Enable it by exporting:

```bash
OPENACOSMI_NIX_MODE=1
```

On macOS, the GUI app does not automatically inherit shell env vars. You can
also enable Nix mode via defaults:

```bash
defaults write bot.molt.mac openacosmi.nixMode -bool true
```

### Config + state paths

OpenAcosmi reads JSON5 config from `OPENACOSMI_CONFIG_PATH` and stores mutable data in `OPENACOSMI_STATE_DIR`.
When needed, you can also set `OPENACOSMI_HOME` to control the base home directory used for internal path resolution.

- `OPENACOSMI_HOME` (default precedence: `HOME` / `USERPROFILE` / `os.homedir()`)
- `OPENACOSMI_STATE_DIR` (default: `~/.openacosmi`)
- `OPENACOSMI_CONFIG_PATH` (default: `$OPENACOSMI_STATE_DIR/openacosmi.json`)

When running under Nix, set these explicitly to Nix-managed locations so runtime state and config
stay out of the immutable store.

### Runtime behavior in Nix mode

- Auto-install and self-mutation flows are disabled
- Missing dependencies surface Nix-specific remediation messages
- UI surfaces a read-only Nix mode banner when present

## Packaging note (macOS)

The macOS packaging flow expects a stable Info.plist template at:

```
apps/macos/Sources/OpenAcosmi/Resources/Info.plist
```

[`scripts/package-mac-app.sh`](https://github.com/openacosmi/openacosmi/blob/main/scripts/package-mac-app.sh) copies this template into the app bundle and patches dynamic fields
(bundle ID, version/build, Git SHA, Sparkle keys). This keeps the plist deterministic for SwiftPM
packaging and Nix builds (which do not rely on a full Xcode toolchain).

## Related

- [nix-openacosmi](https://github.com/openacosmi/nix-openacosmi) — full setup guide
- [Wizard](/start/wizard) — non-Nix CLI setup
- [Docker](/install/docker) — containerized setup
