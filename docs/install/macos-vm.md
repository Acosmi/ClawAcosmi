---
summary: "在 macOS VM（本地或托管）中运行 OpenAcosmi，用于隔离或 iMessage 集成"
read_when:
  - 需要将 OpenAcosmi 与主 macOS 环境隔离
  - 需要在沙箱中集成 iMessage（BlueBubbles）
  - 需要可重置的可克隆 macOS 环境
  - 需要比较本地 vs 托管 macOS VM 选项
title: "macOS VMs"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# OpenAcosmi on macOS VMs（沙箱）

## Recommended default (most users)

- **Small Linux VPS** for an always-on Gateway and low cost. See [VPS hosting](/vps).
- **Dedicated hardware** (Mac mini or Linux box) if you want full control and a **residential IP** for browser automation. Many sites block data center IPs, so local browsing often works better.
- **Hybrid:** keep the Gateway on a cheap VPS, and connect your Mac as a **node** when you need browser/UI automation. See [Nodes](/nodes) and [Gateway remote](/gateway/remote).

Use a macOS VM when you specifically need macOS-only capabilities (iMessage/BlueBubbles) or want strict isolation from your daily Mac.

## macOS VM options

### Local VM on your Apple Silicon Mac (Lume)

Run OpenAcosmi in a sandboxed macOS VM on your existing Apple Silicon Mac using [Lume](https://cua.ai/docs/lume).

This gives you:

- Full macOS environment in isolation (your host stays clean)
- iMessage support via BlueBubbles (impossible on Linux/Windows)
- Instant reset by cloning VMs
- No extra hardware or cloud costs

### Hosted Mac providers (cloud)

If you want macOS in the cloud, hosted Mac providers work too:

- [MacStadium](https://www.macstadium.com/) (hosted Macs)
- Other hosted Mac vendors also work; follow their VM + SSH docs

Once you have SSH access to a macOS VM, continue at step 6 below.

---

## Quick path (Lume, experienced users)

1. Install Lume
2. `lume create openacosmi --os macos --ipsw latest`
3. Complete Setup Assistant, enable Remote Login (SSH)
4. `lume run openacosmi --no-display`
5. SSH in, install OpenAcosmi, configure channels
6. Done

---

## What you need (Lume)

- Apple Silicon Mac (M1/M2/M3/M4)
- macOS Sequoia or later on the host
- ~60 GB free disk space per VM
- ~20 minutes

---

## 1) Install Lume

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/trycua/cua/main/libs/lume/scripts/install.sh)"
```

If `~/.local/bin` isn't in your PATH:

```bash
echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.zshrc && source ~/.zshrc
```

Verify:

```bash
lume --version
```

Docs: [Lume Installation](https://cua.ai/docs/lume/guide/getting-started/installation)

---

## 2) Create the macOS VM

```bash
lume create openacosmi --os macos --ipsw latest
```

This downloads macOS and creates the VM. A VNC window opens automatically.

Note: The download can take a while depending on your connection.

---

## 3) Complete Setup Assistant

In the VNC window:

1. Select language and region
2. Skip Apple ID (or sign in if you want iMessage later)
3. Create a user account (remember the username and password)
4. Skip all optional features

After setup completes, enable SSH:

1. Open System Settings → General → Sharing
2. Enable "Remote Login"

---

## 4) Get the VM's IP address

```bash
lume get openacosmi
```

Look for the IP address (usually `192.168.64.x`).

---

## 5) SSH into the VM

```bash
ssh youruser@192.168.64.X
```

Replace `youruser` with the account you created, and the IP with your VM's IP.

---

## 6) Install OpenAcosmi

Inside the VM:

```bash
curl -fsSL https://openacosmi.ai/install.sh | bash
```

Follow the onboarding prompts to set up your model provider (Anthropic, OpenAI, etc.).

---

## 7) Configure channels

Edit the config file:

```bash
nano ~/.openacosmi/openacosmi.json
```

Add your channels:

```json
{
  "channels": {
    "whatsapp": {
      "dmPolicy": "allowlist",
      "allowFrom": ["+15551234567"]
    },
    "telegram": {
      "botToken": "YOUR_BOT_TOKEN"
    }
  }
}
```

Then login to WhatsApp (scan QR):

```bash
openacosmi channels login
```

---

## 8) Run the VM headlessly

Stop the VM and restart without display:

```bash
lume stop openacosmi
lume run openacosmi --no-display
```

The VM runs in the background. OpenAcosmi's daemon keeps the gateway running.

To check status:

```bash
ssh youruser@192.168.64.X "openacosmi status"
```

---

## Bonus: iMessage integration

This is the killer feature of running on macOS. Use [BlueBubbles](https://bluebubbles.app) to add iMessage to OpenAcosmi.

Inside the VM:

1. Download BlueBubbles from bluebubbles.app
2. Sign in with your Apple ID
3. Enable the Web API and set a password
4. Point BlueBubbles webhooks at your gateway (example: `https://your-gateway-host:3000/bluebubbles-webhook?password=<password>`)

Add to your OpenAcosmi config:

```json
{
  "channels": {
    "bluebubbles": {
      "serverUrl": "http://localhost:1234",
      "password": "your-api-password",
      "webhookPath": "/bluebubbles-webhook"
    }
  }
}
```

Restart the gateway. Now your agent can send and receive iMessages.

Full setup details: [BlueBubbles channel](/channels/bluebubbles)

---

## Save a golden image

Before customizing further, snapshot your clean state:

```bash
lume stop openacosmi
lume clone openacosmi openacosmi-golden
```

Reset anytime:

```bash
lume stop openacosmi && lume delete openacosmi
lume clone openacosmi-golden openacosmi
lume run openacosmi --no-display
```

---

## Running 24/7

Keep the VM running by:

- Keeping your Mac plugged in
- Disabling sleep in System Settings → Energy Saver
- Using `caffeinate` if needed

For true always-on, consider a dedicated Mac mini or a small VPS. See [VPS hosting](/vps).

---

## Troubleshooting

| Problem                  | Solution                                                                           |
| ------------------------ | ---------------------------------------------------------------------------------- |
| Can't SSH into VM        | Check "Remote Login" is enabled in VM's System Settings                            |
| VM IP not showing        | Wait for VM to fully boot, run `lume get openacosmi` again                           |
| Lume command not found   | Add `~/.local/bin` to your PATH                                                    |
| WhatsApp QR not scanning | Ensure you're logged into the VM (not host) when running `openacosmi channels login` |

---

## Related docs

- [VPS hosting](/vps)
- [Nodes](/nodes)
- [Gateway remote](/gateway/remote)
- [BlueBubbles channel](/channels/bluebubbles)
- [Lume Quickstart](https://cua.ai/docs/lume/guide/getting-started/quickstart)
- [Lume CLI Reference](https://cua.ai/docs/lume/reference/cli-reference)
- [Unattended VM Setup](https://cua.ai/docs/lume/guide/fundamentals/unattended-setup) (advanced)
- [Docker Sandboxing](/install/docker) (alternative isolation approach)
