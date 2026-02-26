---
summary: "Uninstall OpenAcosmi completely (CLI, service, state, workspace)"
read_when:
  - You want to remove OpenAcosmi from a machine
  - The gateway service is still running after uninstall
title: "Uninstall"
---

# Uninstall

Two paths:

- **Easy path** if `openacosmi` is still installed.
- **Manual service removal** if the CLI is gone but the service is still running.

## Easy path (CLI still installed)

Recommended: use the built-in uninstaller:

```bash
openacosmi uninstall
```

Non-interactive (automation / npx):

```bash
openacosmi uninstall --all --yes --non-interactive
npx -y openacosmi uninstall --all --yes --non-interactive
```

Manual steps (same result):

1. Stop the gateway service:

```bash
openacosmi gateway stop
```

2. Uninstall the gateway service (launchd/systemd/schtasks):

```bash
openacosmi gateway uninstall
```

3. Delete state + config:

```bash
rm -rf "${OPENACOSMI_STATE_DIR:-$HOME/.openacosmi}"
```

If you set `OPENACOSMI_CONFIG_PATH` to a custom location outside the state dir, delete that file too.

4. Delete your workspace (optional, removes agent files):

```bash
rm -rf ~/.openacosmi/workspace
```

5. Remove the CLI install (pick the one you used):

```bash
npm rm -g openacosmi
pnpm remove -g openacosmi
bun remove -g openacosmi
```

6. If you installed the macOS app:

```bash
rm -rf /Applications/OpenAcosmi.app
```

Notes:

- If you used profiles (`--profile` / `OPENACOSMI_PROFILE`), repeat step 3 for each state dir (defaults are `~/.openacosmi-<profile>`).
- In remote mode, the state dir lives on the **gateway host**, so run steps 1-4 there too.

## Manual service removal (CLI not installed)

Use this if the gateway service keeps running but `openacosmi` is missing.

### macOS (launchd)

Default label is `bot.molt.gateway` (or `bot.molt.<profile>`; legacy `com.openacosmi.*` may still exist):

```bash
launchctl bootout gui/$UID/bot.molt.gateway
rm -f ~/Library/LaunchAgents/bot.molt.gateway.plist
```

If you used a profile, replace the label and plist name with `bot.molt.<profile>`. Remove any legacy `com.openacosmi.*` plists if present.

### Linux (systemd user unit)

Default unit name is `openacosmi-gateway.service` (or `openacosmi-gateway-<profile>.service`):

```bash
systemctl --user disable --now openacosmi-gateway.service
rm -f ~/.config/systemd/user/openacosmi-gateway.service
systemctl --user daemon-reload
```

### Windows (Scheduled Task)

Default task name is `OpenAcosmi Gateway` (or `OpenAcosmi Gateway (<profile>)`).
The task script lives under your state dir.

```powershell
schtasks /Delete /F /TN "OpenAcosmi Gateway"
Remove-Item -Force "$env:USERPROFILE\.openacosmi\gateway.cmd"
```

If you used a profile, delete the matching task name and `~\.openacosmi-<profile>\gateway.cmd`.

## Normal install vs source checkout

### Normal install (install.sh / npm / pnpm / bun)

If you used `https://openacosmi.ai/install.sh` or `install.ps1`, the CLI was installed with `npm install -g openacosmi@latest`.
Remove it with `npm rm -g openacosmi` (or `pnpm remove -g` / `bun remove -g` if you installed that way).

### Source checkout (git clone)

If you run from a repo checkout (`git clone` + `openacosmi ...` / `bun run openacosmi ...`):

1. Uninstall the gateway service **before** deleting the repo (use the easy path above or manual service removal).
2. Delete the repo directory.
3. Remove state + workspace as shown above.
