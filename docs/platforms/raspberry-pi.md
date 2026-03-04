---
summary: "在树莓派上运行 OpenAcosmi（低成本自托管方案）"
read_when:
  - 在树莓派上设置 OpenAcosmi
  - 在 ARM 设备上运行 OpenAcosmi
  - 构建低成本的始终在线个人 AI
title: "树莓派"
---

> **架构提示 — Rust CLI + Go Gateway**
> Go Gateway 和 Rust CLI 在树莓派 ARM64 上编译运行，
> 守护进程管理由 Rust CLI 的 `oa-daemon` crate 实现（`systemd` 模块）。

# 在树莓派上运行 OpenAcosmi

## 目标

在树莓派上运行持久化、始终在线的 OpenAcosmi Go Gateway，一次性费用约 **$35-80**（无月费）。

适用于：

- 24/7 个人 AI 助手
- 家庭自动化中心
- 低功耗、始终可用的 Telegram/WhatsApp 机器人

## 硬件要求

| 型号 | 内存 | 可用？ | 备注 |
| ---- | ---- | ------ | ---- |
| **Pi 5** | 4GB/8GB | ✅ 最佳 | 最快，推荐 |
| **Pi 4** | 4GB | ✅ 良好 | 大多数用户的最佳选择 |
| **Pi 4** | 2GB | ✅ 可用 | 可工作，添加 swap |
| **Pi 4** | 1GB | ⚠️ 紧张 | 可用但需 swap + 精简配置 |
| **Pi 3B+** | 1GB | ⚠️ 慢 | 可工作但卡顿 |
| **Pi Zero 2 W** | 512MB | ❌ | 不推荐 |

**最低配置：** 1GB RAM, 1 核心, 500MB 磁盘
**推荐：** 2GB+ RAM, 64 位系统, 16GB+ SD 卡（或 USB SSD）

## 准备材料

- 树莓派 4 或 5（推荐 2GB+）
- MicroSD 卡（16GB+）或 USB SSD（性能更好）
- 电源（推荐官方 Pi 电源）
- 网络连接（以太网或 WiFi）
- 约 30 分钟

## 1）刷写操作系统

使用 **Raspberry Pi OS Lite (64-bit)** — 无头服务器不需要桌面。

1. 下载 [Raspberry Pi Imager](https://www.raspberrypi.com/software/)
2. 选择系统：**Raspberry Pi OS Lite (64-bit)**
3. 点击齿轮图标 (⚙️) 预配置：
   - 设置主机名：`gateway-host`
   - 启用 SSH
   - 设置用户名/密码
   - 配置 WiFi（如不使用以太网）
4. 刷写到 SD 卡/USB 驱动器
5. 插入并启动 Pi

## 2）通过 SSH 连接

```bash
ssh user@gateway-host
# 或使用 IP 地址
ssh user@192.168.x.x
```

## 3）系统设置

```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装必要软件包
sudo apt install -y git curl build-essential

# 设置时区（对 cron/提醒重要）
sudo timedatectl set-timezone Asia/Shanghai  # 按需修改
```

## 4）安装 Go 和 Rust 工具链（ARM64）

```bash
# 安装 Go（Gateway 编译需要）
wget https://go.dev/dl/go1.22.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.22.linux-arm64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version

# 安装 Rust（CLI 编译需要）
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env
rustc --version
```

## 5）添加 Swap（2GB 及以下重要）

Swap 防止内存不足崩溃：

```bash
# 创建 2GB swap 文件
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# 永久化
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab

# 优化低内存（降低 swappiness）
echo 'vm.swappiness=10' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

## 6）安装 OpenAcosmi

```bash
git clone https://github.com/openacosmi/openacosmi.git
cd openacosmi

# 构建 Go Gateway（ARM64）
cd backend && make build && cd ..

# 构建 Rust CLI（ARM64）
cd cli-rust && cargo build --release && cd ..

# 安装 CLI 到 PATH
sudo cp cli-rust/target/release/openacosmi /usr/local/bin/
```

## 7）运行引导向导

```bash
openacosmi onboard --install-daemon
```

按向导操作：

1. **Gateway 模式：** 本地
2. **认证：** 推荐 API 密钥（OAuth 在无头 Pi 上可能不稳定）
3. **渠道：** Telegram 最容易入手
4. **守护进程：** 是（systemd）

## 8）验证安装

```bash
# 检查状态
openacosmi status

# 检查服务
systemctl --user status openacosmi-gateway

# 查看日志
journalctl --user -u openacosmi-gateway -f
```

## 9）访问仪表盘

Pi 是无头的，使用 SSH 隧道：

```bash
# 从笔记本/台式机
ssh -L 18789:localhost:18789 user@gateway-host

# 然后在浏览器中打开
open http://localhost:18789
```

或使用 Tailscale 实现始终在线访问：

```bash
# 在 Pi 上
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up

# 更新配置
openacosmi config set gateway.bind tailnet
systemctl --user restart openacosmi-gateway
```

---

## 性能优化

### 使用 USB SSD（巨大提升）

SD 卡速度慢且会磨损。USB SSD 大幅提升性能：

```bash
# 检查是否从 USB 启动
lsblk
```

参见 [Pi USB 启动指南](https://www.raspberrypi.com/documentation/computers/raspberry-pi.html#usb-mass-storage-boot)。

### 减少内存使用

```bash
# 禁用 GPU 内存分配（无头）
echo 'gpu_mem=16' | sudo tee -a /boot/config.txt

# 如不需要则禁用蓝牙
sudo systemctl disable bluetooth
```

### 监控资源

```bash
# 检查内存
free -h

# 检查 CPU 温度
vcgencmd measure_temp

# 实时监控
htop
```

---

## ARM 特定说明

### 二进制兼容性

大多数 OpenAcosmi 功能在 ARM64 上工作，但某些外部二进制可能需要 ARM 构建：

| 工具 | ARM64 状态 | 备注 |
| ---- | ---------- | ---- |
| Go Gateway | ✅ | 原生 ARM64 编译 |
| Rust CLI | ✅ | 原生 ARM64 编译 |
| WhatsApp (Baileys) | ✅ | 纯 JS，无问题 |
| Telegram | ✅ | 纯 JS，无问题 |
| Chromium | ✅ | `sudo apt install chromium-browser` |

如果技能失败，检查其二进制是否有 ARM64 构建。Go/Rust 工具通常支持；某些不支持。

### 32 位 vs 64 位

**始终使用 64 位系统。** Go 和 Rust 工具链需要 64 位。验证：

```bash
uname -m
# 应显示：aarch64（64 位）而非 armv7l（32 位）
```

---

## 推荐模型设置

Pi 仅作 Gateway（模型在云端运行），使用基于 API 的模型：

```json
{
  "agents": {
    "defaults": {
      "model": {
        "primary": "anthropic/claude-sonnet-4-20250514",
        "fallbacks": ["openai/gpt-4o-mini"]
      }
    }
  }
}
```

**不要在 Pi 上运行本地 LLM** — 即使小模型也太慢了。让 Claude/GPT 承担重计算。

---

## 开机自启

引导向导会设置此功能，但验证确认：

```bash
# 检查服务是否已启用
systemctl --user is-enabled openacosmi-gateway

# 如未启用
systemctl --user enable openacosmi-gateway

# 启动
systemctl --user start openacosmi-gateway
```

---

## 故障排除

### 内存不足 (OOM)

```bash
# 检查内存
free -h

# 添加更多 swap（参见步骤 5）
# 或减少 Pi 上运行的服务
```

### 性能慢

- 使用 USB SSD 代替 SD 卡
- 禁用未使用的服务：`sudo systemctl disable cups bluetooth avahi-daemon`
- 检查 CPU 限流：`vcgencmd get_throttled`（应返回 `0x0`）

### 服务无法启动

```bash
# 检查日志
journalctl --user -u openacosmi-gateway --no-pager -n 100

# 常见修复：重新构建
cd ~/openacosmi/backend && make build
sudo cp ../cli-rust/target/release/openacosmi /usr/local/bin/
systemctl --user restart openacosmi-gateway
```

### WiFi 断连

无头 Pi 使用 WiFi 时：

```bash
# 禁用 WiFi 电源管理
sudo iwconfig wlan0 power off

# 永久化
echo 'wireless-power off' | sudo tee -a /etc/network/interfaces
```

---

## 费用对比

| 方案 | 一次性费用 | 月费 | 备注 |
| ---- | --------- | ---- | ---- |
| **Pi 4 (2GB)** | ~$45 | $0 | + 电费（~$5/年） |
| **Pi 4 (4GB)** | ~$55 | $0 | 推荐 |
| **Pi 5 (4GB)** | ~$60 | $0 | 最佳性能 |
| **Pi 5 (8GB)** | ~$80 | $0 | 过剩但面向未来 |
| DigitalOcean | $0 | $6/月 | $72/年 |
| Hetzner | $0 | €3.79/月 | ~$50/年 |

**回本周期：** Pi 在 6-12 个月内相比云 VPS 回本。

---

## 另请参见

- [Linux 指南](/platforms/linux) — 通用 Linux 设置
- [DigitalOcean 指南](/platforms/digitalocean) — 云替代方案
- [Hetzner 指南](/install/hetzner) — Docker 设置
- [Tailscale](/gateway/tailscale) — 远程访问
- [节点](/nodes) — 将笔记本/手机配对到 Pi Gateway
