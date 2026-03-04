---
summary: "Agent 控制的 Canvas 面板：通过 WKWebView + 自定义 URL 方案嵌入"
read_when:
  - 实现 macOS Canvas 面板
  - 添加 agent 视觉工作区控制
  - 调试 WKWebView Canvas 加载
title: "Canvas"
---

> **架构提示 — Rust CLI + Go Gateway**
> Canvas 命令通过 Go Gateway WebSocket 的 `node.invoke` 转发，
> Canvas 主机由 Go Gateway 提供服务（端口 18793）。

# Canvas（macOS 应用）

macOS 应用使用 `WKWebView` 嵌入 agent 控制的 **Canvas 面板**。这是
HTML/CSS/JS、A2UI 和小型交互 UI 界面的轻量级视觉工作区。

## Canvas 存储位置

Canvas 状态存储在 Application Support 下：

- `~/Library/Application Support/OpenAcosmi/canvas/<session>/...`

Canvas 面板通过**自定义 URL 方案**提供文件服务：

- `openacosmi-canvas://<session>/<path>`

示例：

- `openacosmi-canvas://main/` → `<canvasRoot>/main/index.html`
- `openacosmi-canvas://main/assets/app.css` → `<canvasRoot>/main/assets/app.css`

如果根目录不存在 `index.html`，应用显示**内置脚手架页面**。

## 面板行为

- 无边框、可调整大小的面板，锚定在菜单栏附近（或鼠标光标附近）。
- 按会话记住大小/位置。
- 本地 Canvas 文件更改时自动重载。
- 同一时间仅显示一个 Canvas 面板（按需切换会话）。

Canvas 可在设置 → **允许 Canvas** 中禁用。禁用时，Canvas 节点命令返回 `CANVAS_DISABLED`。

## Agent API 接口

Canvas 通过 **Go Gateway WebSocket** 暴露，agent 可以：

- 显示/隐藏面板
- 导航到路径或 URL
- 执行 JavaScript
- 捕获快照图像

CLI 示例：

```bash
openacosmi nodes canvas present --node <id>
openacosmi nodes canvas navigate --node <id> --url "/"
openacosmi nodes canvas eval --node <id> --js "document.title"
openacosmi nodes canvas snapshot --node <id>
```

说明：

- `canvas.navigate` 接受**本地 Canvas 路径**、`http(s)` URL 和 `file://` URL。
- 传递 `"/"` 时，Canvas 显示本地脚手架或 `index.html`。

## Canvas 中的 A2UI

A2UI 由 Go Gateway Canvas 主机托管，在 Canvas 面板内渲染。
当 Gateway 广播 Canvas 主机时，macOS 应用在首次打开时自动导航到 A2UI 主机页面。

默认 A2UI 主机 URL：

```
http://<gateway-host>:18793/__openacosmi__/a2ui/
```

### A2UI 命令（v0.8）

Canvas 当前接受 **A2UI v0.8** 服务端→客户端消息：

- `beginRendering`
- `surfaceUpdate`
- `dataModelUpdate`
- `deleteSurface`

`createSurface`（v0.9）不支持。

CLI 示例：

```bash
openacosmi nodes canvas a2ui push --node <id> --text "Hello from A2UI"
```

## 从 Canvas 触发 Agent 运行

Canvas 可通过深层链接触发新的 agent 运行：

- `openacosmi://agent?...`

示例（JavaScript）：

```js
window.location.href = "openacosmi://agent?message=Review%20this%20design";
```

应用会提示确认，除非提供有效密钥。

## 安全说明

- Canvas 方案阻止目录遍历；文件必须位于会话根目录下。
- 本地 Canvas 内容使用自定义方案（无需回环服务器）。
- 外部 `http(s)` URL 仅在显式导航时允许。
