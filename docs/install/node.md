---
title: "Node.js（已弃用）"
summary: "Node.js 运行时说明 — 当前架构已不再需要"
read_when:
  - 查看旧版 Node.js 相关信息
---

> [!WARNING]
> 当前 OpenAcosmi 已迁移至 **Rust CLI + Go Gateway** 架构，**不再需要 Node.js 运行时**。
> 本页面保留作为历史参考。CLI 为 Rust 预编译二进制文件，Gateway 为 Go 预编译二进制文件。

# Node.js（已弃用）

OpenAcosmi 的旧版架构需要 Node.js 22+。当前版本已完全迁移至 Rust + Go 原生二进制文件，无需安装 Node.js。

## 当前架构

- **Rust CLI**：预编译二进制文件，直接下载运行，无需 Node.js
- **Go Gateway**：预编译二进制文件，直接下载运行，无需 Node.js
- **前端 UI**：构建产物为静态文件，由 Go Gateway 提供服务

## 从源码构建

如需从源码构建，需要以下工具链：

- **Rust**：安装 [rustup](https://rustup.rs/)
- **Go 1.22+**：安装 [Go](https://go.dev/dl/)
- **Node.js 20+**（仅用于构建前端 UI）

```bash
git clone https://github.com/openacosmi/openacosmi.git
cd openacosmi
make build        # 构建 Rust CLI + Go Gateway
make ui-build     # 构建前端 UI（需要 Node.js）
```

## 故障排除

### `openacosmi: command not found`

确保 OpenAcosmi 二进制文件所在目录已添加到 PATH 中：

```bash
echo "$PATH"
which openacosmi
```

如使用安装脚本安装，默认安装到 `~/.local/bin/` 或 `/usr/local/bin/`。

若未找到，请手动添加到 shell 配置文件（`~/.zshrc` 或 `~/.bashrc`）：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

然后打开新终端或运行 `source ~/.zshrc`。
