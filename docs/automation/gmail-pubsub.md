---
summary: "Gmail Pub/Sub 推送接入 OpenAcosmi webhook（通过 gogcli）"
read_when:
  - 将 Gmail 收件箱触发器接入 OpenAcosmi
  - 设置 Pub/Sub 推送唤醒 Agent
title: "Gmail PubSub"
---

# Gmail Pub/Sub → OpenAcosmi

> [!IMPORTANT]
> **架构状态**：Webhook 端点由 **Go Gateway**（`backend/internal/gateway/hooks.go`）原生实现。
> Gmail watch 守护进程由外部工具 `gog`（gogcli）管理。

目标链路：Gmail Watch → Pub/Sub Push → `gog gmail watch serve` → OpenAcosmi `/hooks/gmail`

## 前置条件

- `gcloud` 已安装并登录（[安装指南](https://docs.cloud.google.com/sdk/docs/install-sdk)）。
- `gog`（gogcli）已安装并对 Gmail 账户授权（[gogcli.sh](https://gogcli.sh/)）。
- OpenAcosmi hooks 已启用（见 [Webhooks](/automation/webhook)）。
- `tailscale` 已登录（[tailscale.com](https://tailscale.com/)）。支持的方案使用 Tailscale Funnel 作为公网 HTTPS 端点。其他隧道服务可自行配置但不提供官方支持。目前仅正式支持 Tailscale。

Hook 配置示例（启用 Gmail 预设映射）：

```json5
{
  hooks: {
    enabled: true,
    token: "OPENACOSMI_HOOK_TOKEN",
    path: "/hooks",
    presets: ["gmail"],
  },
}
```

如需将 Gmail 摘要投递到聊天界面，可用自定义映射覆盖预设，设置 `deliver` + 可选 `channel`/`to`：

```json5
{
  hooks: {
    enabled: true,
    token: "OPENACOSMI_HOOK_TOKEN",
    presets: ["gmail"],
    mappings: [
      {
        match: { path: "gmail" },
        action: "agent",
        wakeMode: "now",
        name: "Gmail",
        sessionKey: "hook:gmail:{{messages[0].id}}",
        messageTemplate: "新邮件来自 {{messages[0].from}}\n主题: {{messages[0].subject}}\n{{messages[0].snippet}}\n{{messages[0].body}}",
        model: "openai/gpt-5.2-mini",
        deliver: true,
        channel: "last",
        // to: "+15551234567"
      },
    ],
  },
}
```

如需固定渠道，设置 `channel` + `to`。否则 `channel: "last"` 使用上次投递路由（回退到 WhatsApp）。

要为 Gmail 运行强制使用更便宜的模型，在映射中设置 `model`（`provider/model` 或别名）。如果启用了 `agents.defaults.models`，需将其包含在内。

如需专门为 Gmail hook 设置默认模型和思考级别，在配置中添加 `hooks.gmail.model` / `hooks.gmail.thinking`：

```json5
{
  hooks: {
    gmail: {
      model: "openrouter/meta-llama/llama-3.3-70b-instruct:free",
      thinking: "off",
    },
  },
}
```

说明：

- 映射中的 `model`/`thinking` 仍会覆盖这些默认值。
- 回退顺序：`hooks.gmail.model` → `agents.defaults.model.fallbacks` → 主模型（auth/rate-limit/timeout 回退）。
- 如果设置了 `agents.defaults.models`，Gmail 模型必须在允许列表中。
- Gmail hook 内容默认使用外部内容安全边界包装。要禁用（危险），设置 `hooks.gmail.allowUnsafeExternalContent: true`。

## 向导（推荐）

使用 OpenAcosmi 辅助工具一键配置（macOS 上通过 Homebrew 安装依赖）：

```bash
openacosmi webhooks gmail setup \
  --account openacosmi@gmail.com
```

默认行为：

- 使用 Tailscale Funnel 作为公网推送端点。
- 为 `openacosmi webhooks gmail run` 写入 `hooks.gmail` 配置。
- 启用 Gmail hook 预设（`hooks.presets: ["gmail"]`）。

路径说明：当 `tailscale.mode` 启用时，OpenAcosmi 自动将 `hooks.gmail.serve.path` 设为 `/`，公网路径保持 `hooks.gmail.tailscale.path`（默认 `/gmail-pubsub`），因为 Tailscale 在代理时会剥离 set-path 前缀。如需后端接收带前缀的路径，设置 `hooks.gmail.tailscale.target`（或 `--tailscale-target`）为完整 URL（如 `http://127.0.0.1:8788/gmail-pubsub`）并匹配 `hooks.gmail.serve.path`。

需要自定义端点？使用 `--push-endpoint <url>` 或 `--tailscale off`。

平台说明：macOS 上向导通过 Homebrew 安装 `gcloud`、`gogcli` 和 `tailscale`；Linux 上需手动预装。

Gateway 自动启动（推荐）：

- 当 `hooks.enabled=true` 且 `hooks.gmail.account` 已设置时，Gateway 启动时自动运行 `gog gmail watch serve` 并自动续期 watch。
- 设置 `OPENACOSMI_SKIP_GMAIL_WATCHER=1` 可退出（适用于自行运行守护进程的场景）。
- 不要同时手动运行守护进程，否则会遇到 `listen tcp 127.0.0.1:8788: bind: address already in use`。

手动守护进程（启动 `gog gmail watch serve` + 自动续期）：

```bash
openacosmi webhooks gmail run
```

## 一次性设置

1. 选择**拥有 `gog` 所用 OAuth 客户端**的 GCP 项目。

```bash
gcloud auth login
gcloud config set project <project-id>
```

注意：Gmail watch 要求 Pub/Sub topic 与 OAuth 客户端在同一项目中。

1. 启用 API：

```bash
gcloud services enable gmail.googleapis.com pubsub.googleapis.com
```

1. 创建 topic：

```bash
gcloud pubsub topics create gog-gmail-watch
```

1. 允许 Gmail push 发布：

```bash
gcloud pubsub topics add-iam-policy-binding gog-gmail-watch \
  --member=serviceAccount:gmail-api-push@system.gserviceaccount.com \
  --role=roles/pubsub.publisher
```

## 启动 Watch

```bash
gog gmail watch start \
  --account openacosmi@gmail.com \
  --label INBOX \
  --topic projects/<project-id>/topics/gog-gmail-watch
```

保存输出中的 `history_id`（用于调试）。

## 运行推送处理器

本地示例（共享 token 认证）：

```bash
gog gmail watch serve \
  --account openacosmi@gmail.com \
  --bind 127.0.0.1 \
  --port 8788 \
  --path /gmail-pubsub \
  --token <shared> \
  --hook-url http://127.0.0.1:18789/hooks/gmail \
  --hook-token OPENACOSMI_HOOK_TOKEN \
  --include-body \
  --max-bytes 20000
```

说明：

- `--token` 保护推送端点（`x-gog-token` 或 `?token=`）。
- `--hook-url` 指向 OpenAcosmi `/hooks/gmail`（映射；隔离运行 + 摘要到主会话）。
- `--include-body` 和 `--max-bytes` 控制发送到 OpenAcosmi 的邮件正文片段。

推荐：`openacosmi webhooks gmail run` 封装了相同流程并自动续期 watch。

## 暴露处理器（高级，不提供支持）

如需非 Tailscale 隧道，请手动配置并在推送订阅中使用公网 URL（不提供支持，无保障）：

```bash
cloudflared tunnel --url http://127.0.0.1:8788 --no-autoupdate
```

使用生成的 URL 作为推送端点：

```bash
gcloud pubsub subscriptions create gog-gmail-watch-push \
  --topic gog-gmail-watch \
  --push-endpoint "https://<public-url>/gmail-pubsub?token=<shared>"
```

生产环境：使用稳定的 HTTPS 端点并配置 Pub/Sub OIDC JWT，然后运行：

```bash
gog gmail watch serve --verify-oidc --oidc-email <svc@...>
```

## 测试

向被监视的收件箱发送消息：

```bash
gog gmail send \
  --account openacosmi@gmail.com \
  --to openacosmi@gmail.com \
  --subject "watch 测试" \
  --body "ping"
```

检查 watch 状态和历史：

```bash
gog gmail watch status --account openacosmi@gmail.com
gog gmail history --account openacosmi@gmail.com --since <historyId>
```

## 故障排查

- `Invalid topicName`：项目不匹配（topic 不在 OAuth 客户端所在项目中）。
- `User not authorized`：topic 缺少 `roles/pubsub.publisher` 角色。
- 空消息：Gmail push 仅提供 `historyId`；通过 `gog gmail history` 获取详情。

## 清理

```bash
gog gmail watch stop --account openacosmi@gmail.com
gcloud pubsub subscriptions delete gog-gmail-watch-push
gcloud pubsub topics delete gog-gmail-watch
```
