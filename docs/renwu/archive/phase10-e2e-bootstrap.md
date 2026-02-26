# Phase 10 E2E 验证 + CLI 修复 — 新窗口 Bootstrap

> **用途**：用于启动新窗口，修复残留的 CLI 审计入口问题，并指导后续的手动 E2E 验证。
> **前置**：Phase 10 代码审计已完成（2026-02-17）。
> **核心任务**：1 个代码修复 + 2 个手动验证流程。

---

## 1. 项目上下文

- **工作区**：`/Users/fushihua/Desktop/Claude-Acosmi`
- **Go 后端**：`backend/` (Go 1.25+)
- **前端**：`ui/` (或根目录 TS 源码)
- **文档中心**：`docs/renwu/`

## 2. 待修复代码任务 (P2) ✅ 已完成 (2026-02-17)

### 任务目标：`cmd_security.go` CLI 入口串联

**背景**：`internal/security/audit.go` 已有 400L+ 完整实现，但 `cmd/openacosmi/cmd_security.go:18` 仍打印 "not yet implemented"。

**操作步骤**：

1. **编辑文件**：`backend/cmd/openacosmi/cmd_security.go`
2. **修改内容**：
   - 引入 `github.com/anthropic/open-acosmi/backend/internal/security` 包
   - 在 `RunE` 中调用 `security.RunFullAudit(cmd.Context(), ...)`
   - 处理返回结果并打印到 `cmd.OutOrStdout()`

**验证命令**：

```bash
cd backend
go build -o openacosmi ./cmd/openacosmi
./openacosmi security audit
```

## 3. 手动 E2E 验证流程

### A. Agent 对话 UI 层 (需真实 API Key)

1. **配置**：在 `backend/config.yaml` 或环境变量中设置 `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY`。
2. **启动后端**：`cd backend && go run ./cmd/acosmi/`
3. **启动前端**：`npm run dev`
4. **测试动作**：
   - 打开浏览器访问前端
   - 发送 "Hello"
   - **预期**：看到 AI 回复流式输出，无错误弹窗。

### B. 频道连通性 (需真实账号)

各频道代码已就绪，请按需验证：

| 频道 | 配置文件/环境变量 | 验证动作 |
|------|-------------------|----------|
| **Telegram** | `TELEGRAM_BOT_TOKEN` | 给 Bot 发消息，确认收到自动回复 |
| **Discord** | `DISCORD_BOT_TOKEN` | 在频道 @Bot 发消息 |
| **Slack** | `SLACK_APP_TOKEN` + `SLACK_BOT_TOKEN` | (Socket Mode) @Bot 发消息 |
| **WhatsApp** | (无需 token) | 终端扫码登录，手机发消息给自身 |
| **Signal** | `SIGNAL_CLI_PATH` | 注册后发送消息 |
| **iMessage** | (macOS 原生) | 给自己发 iMessage (需授权磁盘访问) |

## 4. 参考文档

- [phase10-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-task.md) — 任务状态总览
- [phase10-final-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-final-audit.md) — 审计细节
- [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) — 延迟项详情
