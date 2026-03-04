---
summary: "模型认证：OAuth、API Key 与 setup-token"
read_when:
  - 调试模型认证或 OAuth 过期
  - 记录认证或凭据存储方式
title: "认证（Authentication）"
---

# 认证

> [!IMPORTANT]
> **架构状态**：模型认证由 **Go Gateway**（`backend/internal/gateway/wizard_auth.go`）管理，
> CLI 命令通过 `cmd/openacosmi/setup_auth_*.go` 实现。凭据存储在 `~/.openacosmi/agents/<agentId>/agent/auth-profiles.json`。

OpenAcosmi 支持 OAuth 和 API Key 来认证模型提供商。对于 Anthropic 账户，推荐使用 **API Key**。
对于 Claude 订阅访问，使用 `claude setup-token` 创建的长期 token。

另见 [OAuth 流程](/concepts/oauth) 了解完整的 OAuth 流程和存储布局。

## 推荐的 Anthropic 设置（API Key）

如果直接使用 Anthropic，使用 API Key：

1. 在 Anthropic Console 创建 API Key。
2. 在 **Gateway 宿主机**（运行 `openacosmi gateway start` 的机器）上配置：

```bash
export ANTHROPIC_API_KEY="..."
openacosmi models status
```

1. 如果 Gateway 在 systemd/launchd 下运行，建议将 Key 放入 `~/.openacosmi/.env`：

```bash
cat >> ~/.openacosmi/.env <<'EOF'
ANTHROPIC_API_KEY=...
EOF
```

然后重启 Gateway 进程并重新检查：

```bash
openacosmi models status
openacosmi doctor
```

如果不想手动管理环境变量，引导向导可存储 API Key：`openacosmi onboard`。

另见 [帮助](/help) 了解环境变量继承（`env.shellEnv`、`~/.openacosmi/.env`、systemd/launchd）。

## Anthropic: setup-token（订阅认证）

对于 Anthropic，推荐使用 **API Key**。如果使用 Claude 订阅，也支持 setup-token 流程。
在 **Gateway 宿主机**上运行：

```bash
claude setup-token
```

然后粘贴到 OpenAcosmi：

```bash
openacosmi models auth setup-token --provider anthropic
```

如果 token 在其他机器上创建，手动粘贴：

```bash
openacosmi models auth paste-token --provider anthropic
```

如果看到 Anthropic 错误如：

```
This credential is only authorized for use with Claude Code and cannot be used for other API requests.
```

…请使用 Anthropic API Key 替代。

手动输入 token（任意提供商；写入 `auth-profiles.json` 并更新配置）：

```bash
openacosmi models auth paste-token --provider anthropic
openacosmi models auth paste-token --provider openrouter
```

自动化检查（过期/缺失返回 `1`，即将过期返回 `2`）：

```bash
openacosmi models status --check
```

可选的运维脚本（systemd/Termux）文档：[认证监控](/automation/auth-monitoring)

> `claude setup-token` 需要交互式 TTY。

## 检查模型认证状态

```bash
openacosmi models status
openacosmi doctor
```

## 控制使用哪个凭据

### 按会话（chat 命令）

使用 `/model <alias-or-id>@<profileId>` 为当前会话指定特定的提供商凭据（示例 profileId：`anthropic:default`、`anthropic:work`）。

使用 `/model`（或 `/model list`）获取紧凑选择器；使用 `/model status` 获取完整视图。

### 按 agent（CLI 覆盖）

为 agent 设置显式的认证 profile 顺序（存储在该 agent 的 `auth-profiles.json` 中）：

```bash
openacosmi models auth order get --provider anthropic
openacosmi models auth order set --provider anthropic anthropic:default
openacosmi models auth order clear --provider anthropic
```

使用 `--agent <id>` 指定特定 agent；省略则使用默认 agent。

## 故障排除

### "No credentials found"

如果 Anthropic token profile 缺失，在 **Gateway 宿主机**上运行 `claude setup-token`，然后重新检查：

```bash
openacosmi models status
```

### Token 过期/即将过期

运行 `openacosmi models status` 确认哪个 profile 正在过期。如果 profile 缺失，重新运行 `claude setup-token`。

## 要求

- Claude Max 或 Pro 订阅（用于 `claude setup-token`）
- Claude Code CLI 已安装（`claude` 命令可用）
