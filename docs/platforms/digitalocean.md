---
summary: "在 DigitalOcean 上运行 OpenAcosmi（简单付费 VPS 方案）"
read_when:
  - 在 DigitalOcean 上设置 OpenAcosmi
  - 寻找低成本 VPS 托管 OpenAcosmi
title: "DigitalOcean"
---

> **架构提示 — Rust CLI + Go Gateway**
> Go Gateway 和 Rust CLI 安装在 VPS 上，
> 守护进程管理由 Rust CLI 的 `oa-daemon` crate 实现（`systemd` 模块）。

# 在 DigitalOcean 上运行 OpenAcosmi

## 目标

在 DigitalOcean 上运行持久化的 OpenAcosmi Go Gateway，月费 **$6**（保留定价 $4/月）。

如果想要 $0/月方案且不介意 ARM + 特定 provider 设置，参见 [Oracle Cloud 指南](/platforms/oracle)。

## 费用对比（2026）

| 提供商 | 方案 | 配置 | 月费 | 备注 |
| ------ | ---- | ---- | ---- | ---- |
| Oracle Cloud | 永久免费 ARM | 最多 4 OCPU, 24GB RAM | $0 | ARM，容量有限/注册复杂 |
| Hetzner | CX22 | 2 vCPU, 4GB RAM | €3.79 (~$4) | 最便宜的付费方案 |
| DigitalOcean | Basic | 1 vCPU, 1GB RAM | $6 | 简单 UI，文档好 |
| Vultr | Cloud Compute | 1 vCPU, 1GB RAM | $6 | 多地点 |
| Linode | Nanode | 1 vCPU, 1GB RAM | $5 | 现属 Akamai |

**选择提供商：**

- DigitalOcean：最简单的 UX + 可预测的设置（本指南）
- Hetzner：性价比好（参见 [Hetzner 指南](/install/hetzner)）
- Oracle Cloud：可 $0/月，但更复杂且仅限 ARM（参见 [Oracle 指南](/platforms/oracle)）

---

## 前置条件

- DigitalOcean 账户（[注册可获 $200 免费额度](https://m.do.co/c/signup)）
- SSH 密钥对（或使用密码认证）
- 约 20 分钟

## 1）创建 Droplet

1. 登录 [DigitalOcean](https://cloud.digitalocean.com/)
2. 点击 **Create → Droplets**
3. 选择：
   - **区域：** 离你最近的（或你的用户最近的）
   - **镜像：** Ubuntu 24.04 LTS
   - **大小：** Basic → Regular → **$6/月**（1 vCPU, 1GB RAM, 25GB SSD）
   - **认证：** SSH 密钥（推荐）或密码
4. 点击 **Create Droplet**
5. 记录 IP 地址

## 2）通过 SSH 连接

```bash
ssh root@YOUR_DROPLET_IP
```

## 3）安装 OpenAcosmi

```bash
# 更新系统
apt update && apt upgrade -y

# 安装构建工具
apt install -y build-essential git curl

# 安装 Go（Gateway 编译需要）
wget https://go.dev/dl/go1.22.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 安装 Rust（CLI 编译需要）
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# 克隆并构建
git clone https://github.com/openacosmi/openacosmi.git
cd openacosmi

# 构建 Go Gateway
cd backend && make build && cd ..

# 构建 Rust CLI
cd cli-rust && cargo build --release && cd ..

# 安装 CLI 到 PATH
sudo cp cli-rust/target/release/openacosmi /usr/local/bin/

# 验证
openacosmi --version
```

## 4）运行引导向导

```bash
openacosmi onboard --install-daemon
```

向导将引导你完成：

- 模型认证（API 密钥或 OAuth）
- 渠道设置（Telegram、WhatsApp、Discord 等）
- Gateway token（自动生成）
- 守护进程安装（systemd）

## 5）验证 Gateway

```bash
# 检查状态
openacosmi status

# 检查服务
systemctl --user status openacosmi-gateway.service

# 查看日志
journalctl --user -u openacosmi-gateway.service -f
```

## 6）访问仪表盘

Go Gateway 默认绑定到回环地址。访问控制 UI：

**方式 A：SSH 隧道（推荐）**

```bash
# 从本地机器
ssh -L 18789:localhost:18789 root@YOUR_DROPLET_IP

# 然后打开：http://localhost:18789
```

**方式 B：Tailscale Serve（HTTPS，仅回环）**

```bash
# 在 Droplet 上
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up

# 配置 Gateway 使用 Tailscale Serve
openacosmi config set gateway.tailscale.mode serve
openacosmi gateway restart
```

打开：`https://<magicdns>/`

说明：

- Serve 保持 Gateway 仅回环并通过 Tailscale 身份头认证。
- 如需 token/密码认证，设置 `gateway.auth.allowTailscale: false` 或使用 `gateway.auth.mode: "password"`。

**方式 C：Tailnet 绑定（无 Serve）**

```bash
openacosmi config set gateway.bind tailnet
openacosmi gateway restart
```

打开：`http://<tailscale-ip>:18789`（需要 token）。

## 7）连接渠道

### Telegram

```bash
openacosmi pairing list telegram
openacosmi pairing approve telegram <CODE>
```

### WhatsApp

```bash
openacosmi channels login whatsapp
# 扫描二维码
```

参见 [渠道](/channels) 了解其他 provider。

---

## 1GB 内存优化

$6 Droplet 仅有 1GB 内存。保持运行顺畅：

### 添加 swap（推荐）

```bash
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

### 使用更轻量的模型

如果遇到 OOM，考虑：

- 使用基于 API 的模型（Claude、GPT）而非本地模型
- 设置 `agents.defaults.model.primary` 为更小的模型

### 监控内存

```bash
free -h
htop
```

---

## 持久化

所有状态位于：

- `~/.openacosmi/` — 配置、凭证、会话数据
- `~/.openacosmi/workspace/` — 工作区（SOUL.md、记忆等）

重启后保留。定期备份：

```bash
tar -czvf openacosmi-backup.tar.gz ~/.openacosmi ~/.openacosmi/workspace
```

---

## 故障排除

### Gateway 无法启动

```bash
openacosmi gateway status
openacosmi doctor --non-interactive
journalctl -u openacosmi --no-pager -n 50
```

### 端口已被占用

```bash
lsof -i :18789
kill <PID>
```

### 内存不足

```bash
# 检查内存
free -h

# 添加更多 swap
# 或升级到 $12/月 Droplet（2GB RAM）
```

---

## 另请参见

- [Hetzner 指南](/install/hetzner) — 更便宜、更强大
- [Docker 安装](/install/docker) — 容器化设置
- [Tailscale](/gateway/tailscale) — 安全远程访问
- [配置](/gateway/configuration) — 完整配置参考
