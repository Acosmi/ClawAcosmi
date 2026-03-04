---
summary: "在 GCP Compute Engine VM（Docker）上 24/7 运行 OpenAcosmi Gateway，带持久化状态"
read_when:
  - 需要在 GCP 上 24/7 运行 OpenAcosmi
  - 需要生产级的始终在线 Gateway
  - 需要完全控制持久化、二进制和重启行为
title: "GCP"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# OpenAcosmi on GCP Compute Engine（Docker 生产 VPS 指南）

## Goal

Run a persistent OpenAcosmi Gateway on a GCP Compute Engine VM using Docker, with durable state, baked-in binaries, and safe restart behavior.

If you want "OpenAcosmi 24/7 for ~$5-12/mo", this is a reliable setup on Google Cloud.
Pricing varies by machine type and region; pick the smallest VM that fits your workload and scale up if you hit OOMs.

## What are we doing (simple terms)?

- Create a GCP project and enable billing
- Create a Compute Engine VM
- Install Docker (isolated app runtime)
- Start the OpenAcosmi Gateway in Docker
- Persist `~/.openacosmi` + `~/.openacosmi/workspace` on the host (survives restarts/rebuilds)
- Access the Control UI from your laptop via an SSH tunnel

The Gateway can be accessed via:

- SSH port forwarding from your laptop
- Direct port exposure if you manage firewalling and tokens yourself

This guide uses Debian on GCP Compute Engine.
Ubuntu also works; map packages accordingly.
For the generic Docker flow, see [Docker](/install/docker).

---

## Quick path (experienced operators)

1. Create GCP project + enable Compute Engine API
2. Create Compute Engine VM (e2-small, Debian 12, 20GB)
3. SSH into the VM
4. Install Docker
5. Clone OpenAcosmi repository
6. Create persistent host directories
7. Configure `.env` and `docker-compose.yml`
8. Bake required binaries, build, and launch

---

## What you need

- GCP account (free tier eligible for e2-micro)
- gcloud CLI installed (or use Cloud Console)
- SSH access from your laptop
- Basic comfort with SSH + copy/paste
- ~20-30 minutes
- Docker and Docker Compose
- Model auth credentials
- Optional provider credentials
  - WhatsApp QR
  - Telegram bot token
  - Gmail OAuth

---

## 1) Install gcloud CLI (or use Console)

**Option A: gcloud CLI** (recommended for automation)

Install from [https://cloud.google.com/sdk/docs/install](https://cloud.google.com/sdk/docs/install)

Initialize and authenticate:

```bash
gcloud init
gcloud auth login
```

**Option B: Cloud Console**

All steps can be done via the web UI at [https://console.cloud.google.com](https://console.cloud.google.com)

---

## 2) Create a GCP project

**CLI:**

```bash
gcloud projects create my-openacosmi-project --name="OpenAcosmi Gateway"
gcloud config set project my-openacosmi-project
```

Enable billing at [https://console.cloud.google.com/billing](https://console.cloud.google.com/billing) (required for Compute Engine).

Enable the Compute Engine API:

```bash
gcloud services enable compute.googleapis.com
```

**Console:**

1. Go to IAM & Admin > Create Project
2. Name it and create
3. Enable billing for the project
4. Navigate to APIs & Services > Enable APIs > search "Compute Engine API" > Enable

---

## 3) Create the VM

**Machine types:**

| Type     | Specs                    | Cost               | Notes              |
| -------- | ------------------------ | ------------------ | ------------------ |
| e2-small | 2 vCPU, 2GB RAM          | ~$12/mo            | Recommended        |
| e2-micro | 2 vCPU (shared), 1GB RAM | Free tier eligible | May OOM under load |

**CLI:**

```bash
gcloud compute instances create openacosmi-gateway \
  --zone=us-central1-a \
  --machine-type=e2-small \
  --boot-disk-size=20GB \
  --image-family=debian-12 \
  --image-project=debian-cloud
```

**Console:**

1. Go to Compute Engine > VM instances > Create instance
2. Name: `openacosmi-gateway`
3. Region: `us-central1`, Zone: `us-central1-a`
4. Machine type: `e2-small`
5. Boot disk: Debian 12, 20GB
6. Create

---

## 4) SSH into the VM

**CLI:**

```bash
gcloud compute ssh openacosmi-gateway --zone=us-central1-a
```

**Console:**

Click the "SSH" button next to your VM in the Compute Engine dashboard.

Note: SSH key propagation can take 1-2 minutes after VM creation. If connection is refused, wait and retry.

---

## 5) Install Docker (on the VM)

```bash
sudo apt-get update
sudo apt-get install -y git curl ca-certificates
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
```

Log out and back in for the group change to take effect:

```bash
exit
```

Then SSH back in:

```bash
gcloud compute ssh openacosmi-gateway --zone=us-central1-a
```

Verify:

```bash
docker --version
docker compose version
```

---

## 6) Clone the OpenAcosmi repository

```bash
git clone https://github.com/openacosmi/openacosmi.git
cd openacosmi
```

This guide assumes you will build a custom image to guarantee binary persistence.

---

## 7) Create persistent host directories

Docker containers are ephemeral.
All long-lived state must live on the host.

```bash
mkdir -p ~/.openacosmi
mkdir -p ~/.openacosmi/workspace
```

---

## 8) Configure environment variables

Create `.env` in the repository root.

```bash
OPENACOSMI_IMAGE=openacosmi:latest
OPENACOSMI_GATEWAY_TOKEN=change-me-now
OPENACOSMI_GATEWAY_BIND=lan
OPENACOSMI_GATEWAY_PORT=18789

OPENACOSMI_CONFIG_DIR=/home/$USER/.openacosmi
OPENACOSMI_WORKSPACE_DIR=/home/$USER/.openacosmi/workspace

GOG_KEYRING_PASSWORD=change-me-now
XDG_CONFIG_HOME=/home/node/.openacosmi
```

Generate strong secrets:

```bash
openssl rand -hex 32
```

**Do not commit this file.**

---

## 9) Docker Compose configuration

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
      # Recommended: keep the Gateway loopback-only on the VM; access via SSH tunnel.
      # To expose it publicly, remove the `127.0.0.1:` prefix and firewall accordingly.
      - "127.0.0.1:${OPENACOSMI_GATEWAY_PORT}:18789"

      # Optional: only if you run iOS/Android nodes against this VM and need Canvas host.
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

## 10) Bake required binaries into the image (critical)

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

## 11) Build and launch

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

## 12) Verify Gateway

```bash
docker compose logs -f openacosmi-gateway
```

Success:

```
[gateway] listening on ws://0.0.0.0:18789
```

---

## 13) Access from your laptop

Create an SSH tunnel to forward the Gateway port:

```bash
gcloud compute ssh openacosmi-gateway --zone=us-central1-a -- -L 18789:127.0.0.1:18789
```

Open in your browser:

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

---

## Updates

To update OpenAcosmi on the VM:

```bash
cd ~/openacosmi
git pull
docker compose build
docker compose up -d
```

---

## Troubleshooting

**SSH connection refused**

SSH key propagation can take 1-2 minutes after VM creation. Wait and retry.

**OS Login issues**

Check your OS Login profile:

```bash
gcloud compute os-login describe-profile
```

Ensure your account has the required IAM permissions (Compute OS Login or Compute OS Admin Login).

**Out of memory (OOM)**

If using e2-micro and hitting OOM, upgrade to e2-small or e2-medium:

```bash
# Stop the VM first
gcloud compute instances stop openacosmi-gateway --zone=us-central1-a

# Change machine type
gcloud compute instances set-machine-type openacosmi-gateway \
  --zone=us-central1-a \
  --machine-type=e2-small

# Start the VM
gcloud compute instances start openacosmi-gateway --zone=us-central1-a
```

---

## Service accounts (security best practice)

For personal use, your default user account works fine.

For automation or CI/CD pipelines, create a dedicated service account with minimal permissions:

1. Create a service account:

   ```bash
   gcloud iam service-accounts create openacosmi-deploy \
     --display-name="OpenAcosmi Deployment"
   ```

2. Grant Compute Instance Admin role (or narrower custom role):

   ```bash
   gcloud projects add-iam-policy-binding my-openacosmi-project \
     --member="serviceAccount:openacosmi-deploy@my-openacosmi-project.iam.gserviceaccount.com" \
     --role="roles/compute.instanceAdmin.v1"
   ```

Avoid using the Owner role for automation. Use the principle of least privilege.

See [https://cloud.google.com/iam/docs/understanding-roles](https://cloud.google.com/iam/docs/understanding-roles) for IAM role details.

---

## Next steps

- Set up messaging channels: [Channels](/channels)
- Pair local devices as nodes: [Nodes](/nodes)
- Configure the Gateway: [Gateway configuration](/gateway/configuration)
