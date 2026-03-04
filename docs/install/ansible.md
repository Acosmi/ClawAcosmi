---
summary: "使用 Ansible、Tailscale VPN 和防火墙隔离的自动化硬化 OpenAcosmi 安装"
read_when:
  - 需要带安全硬化的自动化服务器部署
  - 需要带 VPN 访问的防火墙隔离设置
  - 部署到远程 Debian/Ubuntu 服务器
title: "Ansible"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# Ansible 安装

The recommended way to deploy OpenAcosmi to production servers is via **[openacosmi-ansible](https://github.com/openacosmi/openacosmi-ansible)** — an automated installer with security-first architecture.

## Quick Start

One-command install:

```bash
curl -fsSL https://raw.githubusercontent.com/openacosmi/openacosmi-ansible/main/install.sh | bash
```

> **📦 Full guide: [github.com/openacosmi/openacosmi-ansible](https://github.com/openacosmi/openacosmi-ansible)**
>
> The openacosmi-ansible repo is the source of truth for Ansible deployment. This page is a quick overview.

## What You Get

- 🔒 **Firewall-first security**: UFW + Docker isolation (only SSH + Tailscale accessible)
- 🔐 **Tailscale VPN**: Secure remote access without exposing services publicly
- 🐳 **Docker**: Isolated sandbox containers, localhost-only bindings
- 🛡️ **Defense in depth**: 4-layer security architecture
- 🚀 **One-command setup**: Complete deployment in minutes
- 🔧 **Systemd integration**: Auto-start on boot with hardening

## Requirements

- **OS**: Debian 11+ or Ubuntu 20.04+
- **Access**: Root or sudo privileges
- **Network**: Internet connection for package installation
- **Ansible**: 2.14+ (installed automatically by quick-start script)

## What Gets Installed

The Ansible playbook installs and configures:

1. **Tailscale**（网状 VPN，用于安全远程访问）
2. **UFW 防火墙**（仅开放 SSH + Tailscale 端口）
3. **Docker CE + Compose V2**（用于 Agent 沙箱）
4. **Rust CLI + Go Gateway**（预编译二进制文件）
5. **OpenAcosmi**（基于主机运行，非容器化）
6. **Systemd 服务**（开机自启 + 安全硬化）

注意：Gateway 直接在主机上运行（非 Docker 内），但 Agent 沙箱使用 Docker 进行隔离。详见 [沙箱](/gateway/sandboxing)。

## Post-Install Setup

After installation completes, switch to the openacosmi user:

```bash
sudo -i -u openacosmi
```

The post-install script will guide you through:

1. **Onboarding wizard**: Configure OpenAcosmi settings
2. **Provider login**: Connect WhatsApp/Telegram/Discord/Signal
3. **Gateway testing**: Verify the installation
4. **Tailscale setup**: Connect to your VPN mesh

### Quick commands

```bash
# Check service status
sudo systemctl status openacosmi

# View live logs
sudo journalctl -u openacosmi -f

# Restart gateway
sudo systemctl restart openacosmi

# Provider login (run as openacosmi user)
sudo -i -u openacosmi
openacosmi channels login
```

## Security Architecture

### 4-Layer Defense

1. **Firewall (UFW)**: Only SSH (22) + Tailscale (41641/udp) exposed publicly
2. **VPN (Tailscale)**: Gateway accessible only via VPN mesh
3. **Docker Isolation**: DOCKER-USER iptables chain prevents external port exposure
4. **Systemd Hardening**: NoNewPrivileges, PrivateTmp, unprivileged user

### Verification

Test external attack surface:

```bash
nmap -p- YOUR_SERVER_IP
```

Should show **only port 22** (SSH) open. All other services (gateway, Docker) are locked down.

### Docker Availability

Docker is installed for **agent sandboxes** (isolated tool execution), not for running the gateway itself. The gateway binds to localhost only and is accessible via Tailscale VPN.

See [Multi-Agent Sandbox & Tools](/tools/multi-agent-sandbox-tools) for sandbox configuration.

## Manual Installation

If you prefer manual control over the automation:

```bash
# 1. Install prerequisites
sudo apt update && sudo apt install -y ansible git

# 2. Clone repository
git clone https://github.com/openacosmi/openacosmi-ansible.git
cd openacosmi-ansible

# 3. Install Ansible collections
ansible-galaxy collection install -r requirements.yml

# 4. Run playbook
./run-playbook.sh

# Or run directly (then manually execute /tmp/openacosmi-setup.sh after)
# ansible-playbook playbook.yml --ask-become-pass
```

## Updating OpenAcosmi

The Ansible installer sets up OpenAcosmi for manual updates. See [Updating](/install/updating) for the standard update flow.

To re-run the Ansible playbook (e.g., for configuration changes):

```bash
cd openacosmi-ansible
./run-playbook.sh
```

Note: This is idempotent and safe to run multiple times.

## Troubleshooting

### Firewall blocks my connection

If you're locked out:

- Ensure you can access via Tailscale VPN first
- SSH access (port 22) is always allowed
- The gateway is **only** accessible via Tailscale by design

### Service won't start

```bash
# Check logs
sudo journalctl -u openacosmi -n 100

# Verify permissions
sudo ls -la /opt/openacosmi

# Test manual start
sudo -i -u openacosmi
cd ~/openacosmi
openacosmi gateway start
```

### Docker sandbox issues

```bash
# Verify Docker is running
sudo systemctl status docker

# Check sandbox image
sudo docker images | grep openacosmi-sandbox

# Build sandbox image if missing
cd /opt/openacosmi/openacosmi
sudo -u openacosmi ./scripts/sandbox-setup.sh
```

### Provider login fails

Make sure you're running as the `openacosmi` user:

```bash
sudo -i -u openacosmi
openacosmi channels login
```

## Advanced Configuration

For detailed security architecture and troubleshooting:

- [Security Architecture](https://github.com/openacosmi/openacosmi-ansible/blob/main/docs/security.md)
- [Technical Details](https://github.com/openacosmi/openacosmi-ansible/blob/main/docs/architecture.md)
- [Troubleshooting Guide](https://github.com/openacosmi/openacosmi-ansible/blob/main/docs/troubleshooting.md)

## Related

- [openacosmi-ansible](https://github.com/openacosmi/openacosmi-ansible) — full deployment guide
- [Docker](/install/docker) — containerized gateway setup
- [Sandboxing](/gateway/sandboxing) — agent sandbox configuration
- [Multi-Agent Sandbox & Tools](/tools/multi-agent-sandbox-tools) — per-agent isolation
