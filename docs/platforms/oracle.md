---
summary: "在 Oracle Cloud 上运行 OpenAcosmi（永久免费 ARM）"
read_when:
  - 在 Oracle Cloud 上设置 OpenAcosmi
  - 寻找低成本 VPS 托管 OpenAcosmi
  - 希望 24/7 运行 OpenAcosmi
title: "Oracle Cloud"
---

> **架构提示 — Rust CLI + Go Gateway**
> Go Gateway 和 Rust CLI 安装在 ARM VPS 上，
> 需要 Go 和 Rust 工具链的 ARM64（aarch64）构建。

# 在 Oracle Cloud (OCI) 上运行 OpenAcosmi

## 目标

在 Oracle Cloud 的**永久免费** ARM 层上运行持久化的 OpenAcosmi Go Gateway。

Oracle 免费层非常适合 OpenAcosmi（特别是已有 OCI 账户时），但有权衡：

- ARM 架构（大部分都能工作，某些二进制仅有 x86 版本）
- 容量和注册可能不稳定

## 费用对比（2026）

| 提供商 | 方案 | 配置 | 月费 | 备注 |
| ------ | ---- | ---- | ---- | ---- |
| Oracle Cloud | 永久免费 ARM | 最多 4 OCPU, 24GB RAM | $0 | ARM，容量有限 |
| Hetzner | CX22 | 2 vCPU, 4GB RAM | ~$4 | 最便宜的付费方案 |
| DigitalOcean | Basic | 1 vCPU, 1GB RAM | $6 | 简单 UI，文档好 |
| Vultr | Cloud Compute | 1 vCPU, 1GB RAM | $6 | 多地点 |
| Linode | Nanode | 1 vCPU, 1GB RAM | $5 | 现属 Akamai |

---

## 前置条件

- Oracle Cloud 账户（[注册](https://www.oracle.com/cloud/free/)）— 遇到问题参见 [社区注册指南](https://gist.github.com/rssnyder/51e3cfedd730e7dd5f4a816143b25dbd)
- Tailscale 账户（[tailscale.com](https://tailscale.com) 免费）
- 约 30 分钟

## 1）创建 OCI 实例

1. 登录 [Oracle Cloud Console](https://cloud.oracle.com/)
2. 导航到 **Compute → Instances → Create Instance**
3. 配置：
   - **名称：** `openacosmi`
   - **镜像：** Ubuntu 24.04 (aarch64)
   - **形状：** `VM.Standard.A1.Flex`（Ampere ARM）
   - **OCPU：** 2（最多 4）
   - **内存：** 12 GB（最多 24 GB）
   - **启动卷：** 50 GB（免费最多 200 GB）
   - **SSH 密钥：** 添加你的公钥
4. 点击 **Create**
5. 记录公网 IP 地址

**提示：** 如果实例创建失败提示"Out of capacity"，尝试不同的可用域或稍后重试。免费层容量有限。

## 2）连接并更新

```bash
# 通过公网 IP 连接
ssh ubuntu@YOUR_PUBLIC_IP

# 更新系统
sudo apt update && sudo apt upgrade -y
sudo apt install -y build-essential git curl
```

**说明：** `build-essential` 是 ARM 编译某些依赖所必需的。

## 3）配置用户和主机名

```bash
# 设置主机名
sudo hostnamectl set-hostname openacosmi

# 设置 ubuntu 用户密码
sudo passwd ubuntu

# 启用 lingering（注销后保持用户服务运行）
sudo loginctl enable-linger ubuntu
```

## 4）安装 Tailscale

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up --ssh --hostname=openacosmi
```

这启用 Tailscale SSH，你可以从 tailnet 上的任何设备通过 `ssh openacosmi` 连接 — 无需公网 IP。

验证：

```bash
tailscale status
```

**从现在起通过 Tailscale 连接：** `ssh ubuntu@openacosmi`（或使用 Tailscale IP）。

## 5）安装 OpenAcosmi

```bash
# 安装 Go（Gateway 编译需要）
wget https://go.dev/dl/go1.22.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.22.linux-arm64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 安装 Rust（CLI 编译需要）
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# 克隆并构建
git clone https://github.com/openacosmi/openacosmi.git
cd openacosmi

# 构建 Go Gateway（ARM64）
cd backend && make build && cd ..

# 构建 Rust CLI（ARM64）
cd cli-rust && cargo build --release && cd ..

# 安装 CLI 到 PATH
sudo cp cli-rust/target/release/openacosmi /usr/local/bin/
source ~/.bashrc
```

## 6）配置 Gateway（回环 + token 认证）并启用 Tailscale Serve

使用 token 认证作为默认值：

```bash
# 保持 Gateway 在 VM 上私有
openacosmi config set gateway.bind loopback

# 要求 Gateway + 控制 UI 认证
openacosmi config set gateway.auth.mode token
openacosmi doctor --generate-gateway-token

# 通过 Tailscale Serve 暴露（HTTPS + tailnet 访问）
openacosmi config set gateway.tailscale.mode serve
openacosmi config set gateway.trustedProxies '["127.0.0.1"]'

systemctl --user restart openacosmi-gateway
```

## 7）验证

```bash
# 检查版本
openacosmi --version

# 检查守护进程状态
systemctl --user status openacosmi-gateway

# 检查 Tailscale Serve
tailscale serve status

# 测试本地响应
curl http://localhost:18789
```

## 8）锁定 VCN 安全

一切正常后，锁定 VCN 以阻止除 Tailscale 以外的所有流量。OCI 虚拟云网络在网络边缘阻止流量 — 流量在到达实例之前就被拦截。

1. 在 OCI Console 中进入 **Networking → Virtual Cloud Networks**
2. 点击 VCN → **Security Lists** → Default Security List
3. **移除**除以下以外的所有入站规则：
   - `0.0.0.0/0 UDP 41641`（Tailscale）
4. 保留默认出站规则（允许所有出站）

这会阻止端口 22 上的 SSH、HTTP、HTTPS 和其他一切。从现在起只能通过 Tailscale 连接。

---

## 访问控制 UI

从 Tailscale 网络上的任何设备：

```
https://openacosmi.<tailnet-name>.ts.net/
```

将 `<tailnet-name>` 替换为你的 tailnet 名称（在 `tailscale status` 中可见）。

无需 SSH 隧道。Tailscale 提供：

- HTTPS 加密（自动证书）
- 通过 Tailscale 身份认证
- 从 tailnet 上的任何设备访问（笔记本、手机等）

---

## 安全性：VCN + Tailscale（推荐基准）

VCN 锁定（仅开放 UDP 41641）且 Gateway 绑定到回环后，获得了强大的纵深防御：公网流量在网络边缘被拦截，管理访问通过 tailnet 进行。

### 已保护的内容

| 传统步骤 | 是否需要？ | 原因 |
| -------- | --------- | ---- |
| UFW 防火墙 | 否 | VCN 在流量到达实例前拦截 |
| fail2ban | 否 | 端口 22 在 VCN 被阻止则无暴力破解 |
| sshd 加固 | 否 | Tailscale SSH 不使用 sshd |
| 禁用 root 登录 | 否 | Tailscale 使用 Tailscale 身份，非系统用户 |
| 仅 SSH 密钥认证 | 否 | Tailscale 通过 tailnet 认证 |

### 仍推荐的步骤

- **凭证权限：** `chmod 700 ~/.openacosmi`
- **安全审计：** `openacosmi security audit`
- **系统更新：** 定期 `sudo apt update && sudo apt upgrade`
- **监控 Tailscale：** 在 [Tailscale 管理控制台](https://login.tailscale.com/admin) 中审查设备

### 验证安全状态

```bash
# 确认无公网端口监听
sudo ss -tlnp | grep -v '127.0.0.1\|::1'

# 验证 Tailscale SSH 已激活
tailscale status | grep -q 'offers: ssh' && echo "Tailscale SSH active"

# 可选：完全禁用 sshd
sudo systemctl disable --now ssh
```

---

## 回退方案：SSH 隧道

如果 Tailscale Serve 不工作，使用 SSH 隧道：

```bash
# 从本地机器（通过 Tailscale）
ssh -L 18789:127.0.0.1:18789 ubuntu@openacosmi
```

然后打开 `http://localhost:18789`。

---

## 故障排除

### 实例创建失败（"Out of capacity"）

免费层 ARM 实例很热门。尝试：

- 不同的可用域
- 在非高峰时段重试（清晨）
- 选择形状时使用 "Always Free" 筛选器

### Tailscale 无法连接

```bash
# 检查状态
sudo tailscale status

# 重新认证
sudo tailscale up --ssh --hostname=openacosmi --reset
```

### Gateway 无法启动

```bash
openacosmi gateway status
openacosmi doctor --non-interactive
journalctl --user -u openacosmi-gateway -n 50
```

### 无法访问控制 UI

```bash
# 验证 Tailscale Serve 正在运行
tailscale serve status

# 检查 Gateway 正在监听
curl http://localhost:18789

# 需要时重启
systemctl --user restart openacosmi-gateway
```

### ARM 二进制问题

某些工具可能没有 ARM 构建。检查：

```bash
uname -m  # 应显示 aarch64
```

Go 和 Rust 原生支持 ARM64 交叉编译，大多数依赖可正常工作。

---

## 持久化

所有状态位于：

- `~/.openacosmi/` — 配置、凭证、会话数据
- `~/.openacosmi/workspace/` — 工作区（SOUL.md、记忆、产物）

定期备份：

```bash
tar -czvf openacosmi-backup.tar.gz ~/.openacosmi ~/.openacosmi/workspace
```

---

## 另请参见

- [Gateway 远程访问](/gateway/remote) — 其他远程访问模式
- [Tailscale 集成](/gateway/tailscale) — 完整 Tailscale 文档
- [Gateway 配置](/gateway/configuration) — 所有配置选项
- [DigitalOcean 指南](/platforms/digitalocean) — 付费 + 更简单注册
- [Hetzner 指南](/install/hetzner) — 基于 Docker 的替代方案
