---
summary: "完全卸载 OpenAcosmi（CLI、服务、状态、工作区）"
read_when:
  - 需要从机器上移除 OpenAcosmi
  - 卸载后 Gateway 服务仍在运行
title: "卸载"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# 卸载

两种方式：

- **简便方式** — 如果 `openacosmi` 仍然已安装。
- **手动移除服务** — 如果 CLI 已删除但服务仍在运行。

## 简便方式（CLI 仍可用）

推荐：使用内置卸载器：

```bash
openacosmi uninstall
```

非交互式（自动化）：

```bash
openacosmi uninstall --all --yes --non-interactive
```

手动步骤（效果相同）：

1. 停止 Gateway 服务：

```bash
openacosmi gateway stop
```

1. 卸载 Gateway 服务（launchd/systemd/schtasks）：

```bash
openacosmi gateway uninstall
```

1. 删除状态和配置：

```bash
rm -rf "${OPENACOSMI_STATE_DIR:-$HOME/.openacosmi}"
```

如果将 `OPENACOSMI_CONFIG_PATH` 设置为状态目录外的自定义位置，也请删除该文件。

1. 删除工作区（可选，会移除 Agent 文件）：

```bash
rm -rf ~/.openacosmi/workspace
```

1. 移除 CLI 二进制文件：

```bash
rm -f /usr/local/bin/openacosmi
# 或者如果安装到其他位置
rm -f ~/.local/bin/openacosmi
```

1. 如果安装了 macOS 应用：

```bash
rm -rf /Applications/OpenAcosmi.app
```

注意：

- 如果使用了 Profile（`--profile` / `OPENACOSMI_PROFILE`），请对每个状态目录重复步骤 3（默认为 `~/.openacosmi-<profile>`）。
- 在远程模式下，状态目录位于 **Gateway 主机**上，因此也需在该主机上运行步骤 1-4。

## 手动移除服务（CLI 未安装）

当 Gateway 服务仍在运行但 `openacosmi` 命令不存在时使用此方式。

### macOS（launchd）

默认标签为 `bot.molt.gateway`（或 `bot.molt.<profile>`；旧版 `com.openacosmi.*` 也可能存在）：

```bash
launchctl bootout gui/$UID/bot.molt.gateway
rm -f ~/Library/LaunchAgents/bot.molt.gateway.plist
```

如使用了 Profile，请替换标签和 plist 名称为 `bot.molt.<profile>`。如存在旧版 `com.openacosmi.*` plist 也一并删除。

### Linux（systemd 用户单元）

默认单元名为 `openacosmi-gateway.service`（或 `openacosmi-gateway-<profile>.service`）：

```bash
systemctl --user disable --now openacosmi-gateway.service
rm -f ~/.config/systemd/user/openacosmi-gateway.service
systemctl --user daemon-reload
```

### Windows（计划任务）

默认任务名为 `OpenAcosmi Gateway`（或 `OpenAcosmi Gateway (<profile>)`）。
任务脚本位于状态目录下。

```powershell
schtasks /Delete /F /TN "OpenAcosmi Gateway"
Remove-Item -Force "$env:USERPROFILE\.openacosmi\gateway.cmd"
```

如使用了 Profile，请删除对应的任务名和 `~\.openacosmi-<profile>\gateway.cmd`。

## 安装方式对应的卸载

### 安装脚本 / 预编译二进制

如通过 `https://openacosmi.ai/install.sh` 或 `install.ps1` 安装，CLI 二进制文件位于 PATH 中：

```bash
which openacosmi    # 查找位置
rm -f $(which openacosmi)
```

### 源码检出（git clone）

如从仓库检出运行（`git clone` + `make build`）：

1. 卸载 Gateway 服务后再删除仓库（使用上述简便方式或手动移除服务）。
2. 删除仓库目录。
3. 按上述步骤移除状态和工作区。
