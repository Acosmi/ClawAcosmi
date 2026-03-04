---
summary: "iOS 节点应用：连接 Gateway、配对、Canvas 和故障排除"
read_when:
  - 配对或重新连接 iOS 节点
  - 从源码运行 iOS 应用
  - 调试 Gateway 发现或 Canvas 命令
title: "iOS 应用"
---

> **架构提示 — Rust CLI + Go Gateway**
> iOS 应用作为节点连接到 Go Gateway WebSocket，
> 配对管理通过 Rust CLI 的 `openacosmi nodes` 命令完成。

# iOS 应用（节点）

可用性：内部预览。iOS 应用尚未公开分发。

## 功能

- 通过 WebSocket（局域网或 tailnet）连接到 Go Gateway。
- 暴露节点能力：Canvas、屏幕截图、摄像头捕获、位置、Talk 模式、语音唤醒。
- 接收 `node.invoke` 命令并报告节点状态事件。

## 要求

- 另一台设备上运行 Go Gateway（macOS、Linux 或 Windows WSL2）。
- 网络路径：
  - 同一局域网通过 Bonjour，**或**
  - 通过 Tailnet 的单播 DNS-SD（示例域名：`openacosmi.internal.`），**或**
  - 手动设置主机/端口（回退方案）。

## 快速开始（配对 + 连接）

1. 启动 Go Gateway：

```bash
openacosmi gateway --port 18789
```

1. 在 iOS 应用中，打开设置选择已发现的 Gateway（或启用手动主机并输入主机/端口）。

2. 在 Gateway 主机上批准配对请求：

```bash
openacosmi nodes pending
openacosmi nodes approve <requestId>
```

1. 验证连接：

```bash
openacosmi nodes status
openacosmi gateway call node.list --params "{}"
```

## 发现路径

### Bonjour（局域网）

Go Gateway 在 `local.` 上广播 `_openacosmi-gw._tcp`。iOS 应用自动列出这些 Gateway。

### Tailnet（跨网络）

如果 mDNS 被阻止，使用单播 DNS-SD 区域（选择域名；示例：`openacosmi.internal.`）和 Tailscale 分裂 DNS。
参见 [Bonjour](/gateway/bonjour) 获取 CoreDNS 示例。

### 手动主机/端口

在设置中，启用**手动主机**并输入 Gateway 主机 + 端口（默认 `18789`）。

## Canvas + A2UI

iOS 节点渲染 WKWebView Canvas。使用 `node.invoke` 驱动：

```bash
openacosmi nodes invoke --node "iOS Node" --command canvas.navigate --params '{"url":"http://<gateway-host>:18793/__openacosmi__/canvas/"}'
```

说明：

- Go Gateway 的 Canvas 主机在 `/__openacosmi__/canvas/` 和 `/__openacosmi__/a2ui/` 提供服务。
- iOS 节点在连接时如果 Canvas 主机 URL 已广播，会自动导航到 A2UI。
- 使用 `canvas.navigate` 和 `{"url":""}` 返回内置脚手架。

### Canvas eval / snapshot

```bash
openacosmi nodes invoke --node "iOS Node" --command canvas.eval --params '{"javaScript":"document.title"}'
```

```bash
openacosmi nodes invoke --node "iOS Node" --command canvas.snapshot --params '{"maxWidth":900,"format":"jpeg"}'
```

## 语音唤醒 + Talk 模式

- 语音唤醒和 Talk 模式可在设置中配置。
- iOS 可能会暂停后台音频；当应用不在前台时，语音功能为尽力而为。

## 常见错误

- `NODE_BACKGROUND_UNAVAILABLE`：将 iOS 应用切到前台（canvas/camera/screen 命令需要前台）。
- `A2UI_HOST_NOT_CONFIGURED`：Go Gateway 未广播 Canvas 主机 URL；检查 [Gateway 配置](/gateway/configuration) 中的 `canvasHost`。
- 配对提示未出现：运行 `openacosmi nodes pending` 并手动批准。
- 重新安装后重连失败：Keychain 配对 token 已清除；重新配对节点。

## 相关文档

- [配对](/gateway/pairing)
- [发现](/gateway/discovery)
- [Bonjour](/gateway/bonjour)
