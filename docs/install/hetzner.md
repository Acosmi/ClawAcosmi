---
summary: "在廉价 Hetzner VPS（Docker）上 24/7 运行 OpenAcosmi Gateway，带持久状态和内置二进制"
read_when:
  - 需要在云 VPS 上 24/7 运行 OpenAcosmi
  - 需要生产级的始终在线 Gateway
  - 需要完全控制持久化、二进制和重启行为
  - 在 Hetzner 或类似提供商上使用 Docker 运行 OpenAcosmi
title: "Hetzner"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# OpenAcosmi on Hetzner（Docker 生产 VPS 指南）

## Goal

Run a persistent OpenAcosmi Gateway on a Hetzner VPS using Docker, with durable state, baked-in binaries, and safe restart behavior.

If you want “OpenAcosmi 24/7 for ~$5”, this is the simplest reliable setup.
Hetzner pricing changes; pick the smallest Debian/Ubuntu VPS and scale up if you hit OOMs.

## What are we doing (simple terms)?

- Rent a small Linux server (Hetzner VPS)
- Install Docker (isolated app runtime)
- Start the OpenAcosmi Gateway in Docker
- Persist `~/.openacosmi` + `~/.openacosmi/workspace` on the host (survives restarts/rebuilds)
- Access the Control UI from your laptop via an SSH tunnel

The Gateway can be accessed via:

- SSH port forwarding from your laptop
- Direct port exposure if you manage firewalling and tokens yourself

This guide assumes Ubuntu or Debian on Hetzner.  
If you are on another Linux VPS, map packages accordingly.
For the generic Docker flow, see [Docker](/install/docker).

---

## Quick path (experienced operators)

1. Provision Hetzner VPS
2. Install Docker
3. Clone OpenAcosmi repository
4. Create persistent host directories
5. Configure `.env` and `docker-compose.yml`
6. Bake required binaries into the image
7. `docker compose up -d`
8. Verify persistence and Gateway access

---

## What you need

- Hetzner VPS with root access
- SSH access from your laptop
- Basic comfort with SSH + copy/paste
- ~20 minutes
- Docker and Docker Compose
- Model auth credentials
- Optional provider credentials
  - WhatsApp QR
  - Telegram bot token
  - Gmail OAuth

---

## 1) Provision the VPS

Create an Ubuntu or Debian VPS in Hetzner.

Connect as root:

```bash
ssh root@YOUR_VPS_IP
```

This guide assumes the VPS is stateful.
Do not treat it as disposable infrastructure.

---

## 2) Install Docker (on the VPS)

```bash
apt-get update
apt-get install -y git curl ca-certificates
curl -fsSL https://get.docker.com | sh
```

Verify:

```bash
docker --version
docker compose version
```

---

## 3) Clone the OpenAcosmi repository

```bash
git clone https://github.com/openacosmi/openacosmi.git
cd openacosmi
```

This guide assumes you will build a custom image to guarantee binary persistence.

---

## 4) Create persistent host directories

Docker containers are ephemeral.
All long-lived state must live on the host.

```bash
mkdir -p /root/.openacosmi
mkdir -p /root/.openacosmi/workspace

# Set ownership to the container user (uid 1000):
chown -R 1000:1000 /root/.openacosmi
chown -R 1000:1000 /root/.openacosmi/workspace
```

---

## 5) Configure environment variables

Create `.env` in the repository root.

```bash
OPENACOSMI_IMAGE=openacosmi:latest
OPENACOSMI_GATEWAY_TOKEN=change-me-now
OPENACOSMI_GATEWAY_BIND=lan
OPENACOSMI_GATEWAY_PORT=18789

OPENACOSMI_CONFIG_DIR=/root/.openacosmi
OPENACOSMI_WORKSPACE_DIR=/root/.openacosmi/workspace

GOG_KEYRING_PASSWORD=change-me-now
XDG_CONFIG_HOME=/home/node/.openacosmi
```

Generate strong secrets:

```bash
openssl rand -hex 32
```

**Do not commit this file.**

---

## 6) Docker Compose configuration

Create or update `docker-compose.yml`.

```yaml
services:
  openacosmi-gateway:
    image: ${OPENACOSMI_IMAGE}
    build: .
    restart: unless-stopped
    env_file:
      - .env
    environment:
      - HOME=/home/node
      - NODE_ENV=production
      - TERM=xterm-256color
      - OPENACOSMI_GATEWAY_BIND=${OPENACOSMI_GATEWAY_BIND}
      - OPENACOSMI_GATEWAY_PORT=${OPENACOSMI_GATEWAY_PORT}
      - OPENACOSMI_GATEWAY_TOKEN=${OPENACOSMI_GATEWAY_TOKEN}
      - GOG_KEYRING_PASSWORD=${GOG_KEYRING_PASSWORD}
      - XDG_CONFIG_HOME=${XDG_CONFIG_HOME}
      - PATH=/home/linuxbrew/.linuxbrew/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    volumes:
      - ${OPENACOSMI_CONFIG_DIR}:/home/node/.openacosmi
      - ${OPENACOSMI_WORKSPACE_DIR}:/home/node/.openacosmi/workspace
    ports:
      # Recommended: keep the Gateway loopback-only on the VPS; access via SSH tunnel.
      # To expose it publicly, remove the `127.0.0.1:` prefix and firewall accordingly.
      - "127.0.0.1:${OPENACOSMI_GATEWAY_PORT}:18789"

      # Optional: only if you run iOS/Android nodes against this VPS and need Canvas host.
      # If you expose this publicly, read /gateway/security and firewall accordingly.
      # - "18793:18793"
    command:
      [
        "node",
        "dist/index.js",
        "gateway",
        "--bind",
        "${OPENACOSMI_GATEWAY_BIND}",
        "--port",
        "${OPENACOSMI_GATEWAY_PORT}",
      ]
```

---

## 7) Bake required binaries into the image (critical)

Installing binaries inside a running container is a trap.
Anything installed at runtime will be lost on restart.

All external binaries required by skills must be installed at image build time.

The examples below show three common binaries only:

- `gog` for Gmail access
- `goplaces` for Google Places
- `wacli` for WhatsApp

These are examples, not a complete list.
You may install as many binaries as needed using the same pattern.

If you add new skills later that depend on additional binaries, you must:

1. Update the Dockerfile
2. Rebuild the image
3. Restart the containers

**Example Dockerfile**

```dockerfile
FROM golang:1.22-bookworm AS builder

RUN apt-get update && apt-get install -y socat && rm -rf /var/lib/apt/lists/*

# Example binary 1: Gmail CLI
RUN curl -L https://github.com/steipete/gog/releases/latest/download/gog_Linux_x86_64.tar.gz \
  | tar -xz -C /usr/local/bin && chmod +x /usr/local/bin/gog

# Example binary 2: Google Places CLI
RUN curl -L https://github.com/steipete/goplaces/releases/latest/download/goplaces_Linux_x86_64.tar.gz \
  | tar -xz -C /usr/local/bin && chmod +x /usr/local/bin/goplaces

# Example binary 3: WhatsApp CLI
RUN curl -L https://github.com/steipete/wacli/releases/latest/download/wacli_Linux_x86_64.tar.gz \
  | tar -xz -C /usr/local/bin && chmod +x /usr/local/bin/wacli

# Build Go Gateway
WORKDIR /app
COPY backend/ ./backend/
RUN cd backend && go build -o /openacosmi-gateway ./cmd/gateway

FROM debian:bookworm-slim
COPY --from=builder /openacosmi-gateway /usr/local/bin/openacosmi-gateway
COPY --from=builder /usr/local/bin/gog /usr/local/bin/gog
COPY --from=builder /usr/local/bin/goplaces /usr/local/bin/goplaces
COPY --from=builder /usr/local/bin/wacli /usr/local/bin/wacli

# Copy prebuilt Rust CLI binary
COPY target/release/openacosmi /usr/local/bin/openacosmi

# Copy UI static files
COPY ui/dist /app/ui/dist

CMD ["openacosmi-gateway"]
```

---

## 8) Build and launch

```bash
docker compose build
docker compose up -d openacosmi-gateway
```

Verify binaries:

```bash
docker compose exec openacosmi-gateway which gog
docker compose exec openacosmi-gateway which goplaces
docker compose exec openacosmi-gateway which wacli
```

Expected output:

```
/usr/local/bin/gog
/usr/local/bin/goplaces
/usr/local/bin/wacli
```

---

## 9) Verify Gateway

```bash
docker compose logs -f openacosmi-gateway
```

Success:

```
[gateway] listening on ws://0.0.0.0:18789
```

From your laptop:

```bash
ssh -N -L 18789:127.0.0.1:18789 root@YOUR_VPS_IP
```

Open:

`http://127.0.0.1:18789/`

Paste your gateway token.

---

## What persists where (source of truth)

OpenAcosmi runs in Docker, but Docker is not the source of truth.
All long-lived state must survive restarts, rebuilds, and reboots.

| Component           | Location                          | Persistence mechanism  | Notes                            |
| ------------------- | --------------------------------- | ---------------------- | -------------------------------- |
| Gateway config      | `/home/node/.openacosmi/`           | Host volume mount      | Includes `openacosmi.json`, tokens |
| Model auth profiles | `/home/node/.openacosmi/`           | Host volume mount      | OAuth tokens, API keys           |
| Skill configs       | `/home/node/.openacosmi/skills/`    | Host volume mount      | Skill-level state                |
| Agent workspace     | `/home/node/.openacosmi/workspace/` | Host volume mount      | Code and agent artifacts         |
| WhatsApp session    | `/home/node/.openacosmi/`           | Host volume mount      | Preserves QR login               |
| Gmail keyring       | `/home/node/.openacosmi/`           | Host volume + password | Requires `GOG_KEYRING_PASSWORD`  |
| External binaries   | `/usr/local/bin/`                 | Docker image           | Must be baked at build time      |
| Node runtime        | Container filesystem              | Docker image           | Rebuilt every image build        |
| OS packages         | Container filesystem              | Docker image           | Do not install at runtime        |
| Docker container    | Ephemeral                         | Restartable            | Safe to destroy                  |
