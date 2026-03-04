---
summary: "模型提供商 OAuth 过期监控与告警"
read_when:
  - 设置认证过期监控或告警
  - 自动化 Claude Code / Codex OAuth 刷新检查
title: "认证监控"
---

# 认证监控

> [!IMPORTANT]
> **架构状态**：此功能通过 **Go CLI**（`backend/cmd/openacosmi/`）提供 `openacosmi models status` 命令。
> 可选运维脚本位于 `scripts/` 目录。

OpenAcosmi 通过 `openacosmi models status` 暴露 OAuth 过期健康状态。可用于自动化和告警；脚本为可选附加工具，适用于手机运维流程。

## 推荐方式：CLI 检查（跨平台）

```bash
openacosmi models status --check
```

退出码：

- `0`：正常
- `1`：凭证已过期或缺失
- `2`：即将过期（24 小时内）

此方式可直接在 cron / systemd 中使用，无需额外脚本。

## 可选脚本（运维 / 手机工作流）

以下脚本位于 `scripts/` 目录，为**可选项**。它们假设通过 SSH 访问 Gateway 主机，并针对 systemd + Termux 环境优化。

- `scripts/claude-auth-status.sh`：使用 `openacosmi models status --json` 作为数据源（CLI 不可用时回退到直接读取文件），因此需确保 `openacosmi` 在 `PATH` 中。
- `scripts/auth-monitor.sh`：cron / systemd 定时器目标；发送告警（ntfy 或手机通知）。
- `scripts/systemd/openacosmi-auth-monitor.{service,timer}`：systemd 用户定时器。
- `scripts/claude-auth-status.sh`：Claude Code + OpenAcosmi 认证检查器（full / json / simple 模式）。
- `scripts/mobile-reauth.sh`：通过 SSH 引导重新授权流程。
- `scripts/termux-quick-auth.sh`：一键 widget 查看状态 + 打开授权 URL。
- `scripts/termux-auth-widget.sh`：完整引导式 widget 流程。
- `scripts/termux-sync-widget.sh`：同步 Claude Code 凭证到 OpenAcosmi。

如果不需要手机自动化或 systemd 定时器，可跳过这些脚本。
