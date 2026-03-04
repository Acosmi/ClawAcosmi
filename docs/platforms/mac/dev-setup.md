---
summary: "macOS 应用开发设置、构建和测试"
read_when:
  - 设置 macOS 开发环境
  - 构建和测试 macOS 应用
title: "开发设置"
---

> **架构提示 — Rust CLI + Go Gateway**
> macOS 应用依赖外部 Go Gateway 和 Rust CLI，
> 构建使用 Swift + Xcode，不再捆绑 Node.js 运行时。

# macOS 开发设置

## 前置条件

- macOS 14+
- Xcode 15+（含 Command Line Tools）
- Go 1.22+（构建 Gateway）
- Rust 工具链（构建 CLI）
- Apple Development 证书（用于稳定的 TCC 权限）

## 构建步骤

```bash
# 构建 Go Gateway
cd backend && make build && cd ..

# 构建 Rust CLI
cd cli-rust && cargo build --release && cd ..

# 安装 CLI
sudo cp cli-rust/target/release/openacosmi /usr/local/bin/

# 构建 macOS 应用
cd apps/macos
swift build

# 运行（开发模式）
swift run OpenAcosmi
```

## 签名和打包

```bash
SIGN_IDENTITY="Apple Development: <Your Name> (<TEAMID>)" scripts/restart-mac.sh
```

无签名快速构建（权限不持久）：

```bash
scripts/restart-mac.sh --no-sign
```

## 测试

```bash
cd apps/macos
swift test
```

## 常用脚本

- `scripts/restart-mac.sh` — 构建 + 签名 + 重启应用 + LaunchAgent
- `scripts/package-mac-app.sh` — 完整打包
- `scripts/clawlog.sh` — 查看应用日志

## 环境变量

- `OPENACOSMI_PROFILE` — 使用命名 profile（多实例）
- `OPENACOSMI_SKIP_CHANNELS=1` — 跳过渠道初始化（快速启动）
- `OPENACOSMI_SKIP_CANVAS_HOST=1` — 跳过 Canvas 主机
