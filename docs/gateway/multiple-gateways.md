---
summary: "在同一宿主机上运行多个 Gateway 实例（隔离、端口与 profile）"
read_when:
  - 在同一机器上运行多个 Gateway
  - 需要每个 Gateway 的独立配置/状态/端口
title: "多 Gateway 实例"
---

# 多 Gateway 实例（同一宿主机）

> [!IMPORTANT]
> **架构状态**：多实例支持由 **Go Gateway** 的实例锁（`backend/internal/infra/gateway_lock.go`）和
> CLI profile 机制实现。

大多数场景应使用一个 Gateway，单个 Gateway 可处理多个消息连接和 Agent。需要更强隔离或冗余（如救援 Bot）时，使用独立 profile/端口运行多个 Gateway。

## 隔离清单（必须）

- `OPENACOSMI_CONFIG_PATH` — 每实例独立配置文件
- `OPENACOSMI_STATE_DIR` — 每实例独立会话、凭据、缓存
- `agents.defaults.workspace` — 每实例独立工作区根目录
- `gateway.port`（或 `--port`）— 每实例唯一
- 衍生端口（浏览器/Canvas）不能重叠

共享这些资源会导致配置竞争和端口冲突。

## 推荐：profile（`--profile`）

Profile 自动隔离 `OPENACOSMI_STATE_DIR` + `OPENACOSMI_CONFIG_PATH` 并为服务名添加后缀。

```bash
# 主实例
openacosmi --profile main setup
openacosmi --profile main gateway start --port 19001

# 救援实例
openacosmi --profile rescue setup
openacosmi --profile rescue gateway start --port 19021
```

按 profile 安装服务：

```bash
openacosmi --profile main gateway install
openacosmi --profile rescue gateway install
```

## 救援 Bot 指南

在同一宿主机上运行第二个 Gateway，使用独立的：

- profile/配置
- state 目录
- workspace
- 基础端口（加衍生端口）

端口间距：基础端口间至少留 20 个端口以避免衍生端口冲突。

### 安装（救援 Bot）

```bash
# 主 Bot
openacosmi onboard
openacosmi gateway install

# 救援 Bot（隔离 profile + 端口）
openacosmi --profile rescue onboard
openacosmi --profile rescue gateway install
```

## 端口映射（衍生）

基础端口 = `gateway.port`（或 `OPENACOSMI_GATEWAY_PORT` / `--port`）。

- 浏览器控制服务端口 = base + 2（仅 loopback）
- `canvasHost.port = base + 4`
- 浏览器 profile CDP 端口从 `browser.controlPort + 9 .. + 108` 自动分配

## 浏览器/CDP 注意事项

- **不要**在多个实例上将 `browser.cdpUrl` 设为相同值。
- 每个实例需要独立的浏览器控制端口和 CDP 范围。
- 远程 Chrome：使用 `browser.profiles.<name>.cdpUrl`（按 profile、按实例）。

## 手动环境变量示例

```bash
OPENACOSMI_CONFIG_PATH=~/.openacosmi/main.json \
OPENACOSMI_STATE_DIR=~/.openacosmi-main \
openacosmi gateway start --port 19001

OPENACOSMI_CONFIG_PATH=~/.openacosmi/rescue.json \
OPENACOSMI_STATE_DIR=~/.openacosmi-rescue \
openacosmi gateway start --port 19021
```

## 快速检查

```bash
openacosmi --profile main status
openacosmi --profile rescue status
```
