---
summary: "Doctor 命令：健康检查、配置迁移与修复步骤"
read_when:
  - 添加或修改 doctor 迁移
  - 引入破坏性配置变更
title: "Doctor"
---

# Doctor

> [!IMPORTANT]
> **架构状态**：`openacosmi doctor` 由 **Go**（`backend/cmd/openacosmi/cmd_doctor.go`）实现。

`openacosmi doctor` 是 OpenAcosmi 的修复 + 迁移工具。修复陈旧配置/状态，检查健康，提供可操作的修复步骤。

## 快速开始

```bash
openacosmi doctor           # 交互式
openacosmi doctor --yes     # 接受默认值
openacosmi doctor --repair  # 应用推荐修复
openacosmi doctor --deep    # 扫描系统服务
```

## 功能概览

1. **配置规范化**：遗留值形状迁移到当前 schema。
2. **遗留配置键迁移**：`routing.*` → `channels.*`、`agents.*`、`tools.*` 等。
3. **遗留磁盘布局迁移**：会话/Agent 目录迁移到 per-agent 路径。
4. **状态完整性检查**：验证 state 目录、权限、会话持久性。
5. **模型认证健康**：OAuth 过期检查、token 刷新。
6. **沙箱镜像修复**：检查 Docker 镜像完整性。
7. **Gateway 服务迁移**：检测遗留服务，提供清理指引。
8. **安全警告**：开放 DM 策略、缺失认证 token。
9. **systemd linger 检查**（Linux）。
10. **Skills 状态摘要**。
11. **Gateway 健康检查 + 重启提示**。
12. **频道状态警告**。
13. **Supervisor 配置审计 + 修复**。
14. **端口冲突诊断**。
15. **Workspace 提示**（备份 + 记忆系统）。

## 遗留配置迁移列表

- `routing.allowFrom` → `channels.whatsapp.allowFrom`
- `routing.groupChat.*` → `channels.*.groups` / `messages.groupChat.*`
- `routing.queue` → `messages.queue`
- `routing.bindings` → `bindings`
- `routing.agents` → `agents.list`
- `identity` → `agents.list[].identity`
- `agent.*` → `agents.defaults` + `tools.*`

## 状态完整性检查

- **State 目录缺失**：警告灾难性数据丢失。
- **权限**：验证可写性，提供 `chown` 提示。
- **会话目录**：确保存在以避免崩溃。
- **配置文件权限**：建议 `chmod 600`。

## 运维注意

- `openacosmi doctor --non-interactive`：仅安全迁移，跳过重启操作。
- Gateway 启动时也会自动运行 doctor 迁移。
- `openacosmi doctor --generate-gateway-token`：自动化中强制生成 token。

详见：[Agent Workspace](/concepts/agent-workspace)
