# 内网穿透工具使用说明

本项目已安装 cloudflared 和 ngrok，用于本地开发时将 Gateway 暴露到公网。

> **飞书审批卡片回调不需要公网穿透**。
> 卡片回传交互（`card.action.trigger`）已改为 WebSocket 长连接模式，
> 通过 `OnP2CardActionTrigger` 注册到 SDK EventDispatcher，
> 与消息接收走同一条 WebSocket 出站连接。
>
> 以下穿透工具仅在需要 HTTP webhook（如切换到 HTTP 推送模式、
> 或其他外部服务需要回调本地）时使用。

---

## 已安装的工具

| 工具 | 版本 | 特点 |
|------|------|------|
| **cloudflared** | 2026.2.0 | 免注册、免费、无限连接数 |
| **ngrok** | 3.36.1 | 需注册获取 authtoken，免费版有连接数限制 |

---

## 方式一：cloudflared（推荐）

### 启动隧道

```bash
cloudflared tunnel --url http://localhost:18789
```

输出示例：
```
+--------------------------------------------------------------------------------------------+
|  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |
|  https://random-words-here.trycloudflare.com                                               |
+--------------------------------------------------------------------------------------------+
```

复制 `https://xxx.trycloudflare.com` 即为你的公网地址。

### 特点
- 无需注册、无需 token
- 每次启动生成随机域名
- 关闭终端即停止

---

## 方式二：ngrok

### 首次使用需注册

1. 访问 https://dashboard.ngrok.com/signup 注册
2. 获取 authtoken
3. 配置：
```bash
ngrok config add-authtoken <你的token>
```

### 启动隧道

```bash
ngrok http 18789
```

输出示例：
```
Forwarding  https://xxxx-xx-xx.ngrok-free.app -> http://localhost:18789
```

复制 `https://xxxx.ngrok-free.app` 即为你的公网地址。

### 特点
- 免费版每分钟 40 连接
- 固定域名需付费
- Web 面板可查看请求记录：http://127.0.0.1:4040

---

## 飞书开放平台配置（仅 HTTP 推送模式需要）

> 当前代码使用 WebSocket 长连接模式，以下配置**无需操作**。
> 仅在切换到 HTTP 推送模式时才需要。

拿到公网地址后，进入 [飞书开放平台](https://open.feishu.cn/) → 你的应用：

### 1. 事件请求网址

**路径**：开发配置 → 事件与回调 → 事件请求网址

```
https://<公网地址>/channels/feishu/webhook
```

### 2. 消息卡片请求网址

当前已改为 WebSocket 长连接模式下的 `card.action.trigger` 事件，不需要配置。

---

## 常见问题

**Q: 飞书审批卡片按钮点击需要公网穿透吗？**
- 不需要。卡片回调已通过 SDK `OnP2CardActionTrigger` 走 WebSocket 长连接，
  与消息接收共用同一条出站连接，只需本机能访问公网即可。

**Q: 每次重启隧道域名都变，能固定吗？**
- cloudflared: 可以创建命名隧道（需 Cloudflare 账号）：`cloudflared tunnel create my-tunnel`
- ngrok: 免费版不支持固定域名，付费版可以

**Q: Gateway 端口不是 18789？**
- 检查 `openacosmi.json` 中 `gateway.port` 配置

**Q: cloudflared 和 ngrok 哪个更适合生产环境？**
- 都不适合。生产环境应直接部署在有公网 IP 的服务器上，或使用反向代理（Nginx/Caddy）
