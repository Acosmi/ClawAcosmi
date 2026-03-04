---
summary: "广播组：让多个 Agent 同时处理同一条 WhatsApp 消息"
read_when:
  - 配置广播组
  - 调试 WhatsApp 中的多 Agent 回复
status: experimental
title: "广播组（Broadcast Groups）"
---

# 广播组

> [!IMPORTANT]
> **架构状态**：广播组由 **Go Gateway**（`backend/internal/channels/`）实现。
> 配置在 Go Gateway 的 `openacosmi.json` 中定义。

**状态：** 实验性
**版本：** 添加于 2026.1.9

## 概述

广播组允许多个 agent 同时处理和响应同一条消息。你可以创建专业的 agent 团队，在单个 WhatsApp 群组或 DM 中协同工作——全部使用一个手机号码。

当前范围：**仅 WhatsApp**（Web 频道）。

广播组在频道白名单和群组激活规则之后评估。在 WhatsApp 群组中，这意味着广播在 OpenAcosmi 通常会回复时发生（例如：提及时，取决于你的群组设置）。

## 使用场景

### 1. 专业 Agent 团队

部署多个具有原子化、聚焦职责的 agent：

```
群组："开发团队"
Agents:
  - CodeReviewer (审查代码片段)
  - DocumentationBot (生成文档)
  - SecurityAuditor (检查漏洞)
  - TestGenerator (建议测试用例)
```

每个 agent 处理同一条消息并提供其专业视角。

### 2. 多语言支持

```
群组："国际支持"
Agents:
  - Agent_EN (英语回复)
  - Agent_DE (德语回复)
  - Agent_ZH (中文回复)
```

### 3. 质量保证流程

```
群组："客户支持"
Agents:
  - SupportAgent (提供回答)
  - QAAgent (审查质量，仅在发现问题时回复)
```

### 4. 任务自动化

```
群组："项目管理"
Agents:
  - TaskTracker (更新任务数据库)
  - TimeLogger (记录花费时间)
  - ReportGenerator (创建摘要)
```

## 配置

### 基本设置

添加顶级 `broadcast` 节点（与 `bindings` 并列）。键为 WhatsApp peer id：

- 群聊：群组 JID（如 `120363403215116621@g.us`）
- DM：E.164 电话号码（如 `+15555550123`）

```json
{
  "broadcast": {
    "120363403215116621@g.us": ["alfred", "baerbel", "assistant3"]
  }
}
```

**结果：** 当 OpenAcosmi 在此聊天中回复时，将运行所有三个 agent。

### 处理策略

控制 agent 处理消息的方式：

#### 并行（默认）

所有 agent 同时处理：

```json
{
  "broadcast": {
    "strategy": "parallel",
    "120363403215116621@g.us": ["alfred", "baerbel"]
  }
}
```

#### 顺序

agent 按顺序处理（后者等待前者完成）：

```json
{
  "broadcast": {
    "strategy": "sequential",
    "120363403215116621@g.us": ["alfred", "baerbel"]
  }
}
```

### 完整示例

```json
{
  "agents": {
    "list": [
      {
        "id": "code-reviewer",
        "name": "Code Reviewer",
        "workspace": "/path/to/code-reviewer",
        "sandbox": { "mode": "all" }
      },
      {
        "id": "security-auditor",
        "name": "Security Auditor",
        "workspace": "/path/to/security-auditor",
        "sandbox": { "mode": "all" }
      },
      {
        "id": "docs-generator",
        "name": "Documentation Generator",
        "workspace": "/path/to/docs-generator",
        "sandbox": { "mode": "all" }
      }
    ]
  },
  "broadcast": {
    "strategy": "parallel",
    "120363403215116621@g.us": ["code-reviewer", "security-auditor", "docs-generator"],
    "120363424282127706@g.us": ["support-en", "support-de"],
    "+15555550123": ["assistant", "logger"]
  }
}
```

## 工作原理

### 消息流程

1. **入站消息** 到达 WhatsApp 群组
2. **广播检查**：Go Gateway 检查 peer ID 是否在 `broadcast` 中
3. **在广播列表中**：
   - 所有列出的 agent 处理该消息
   - 每个 agent 拥有独立的会话键和隔离上下文
   - agent 并行（默认）或顺序处理
4. **不在广播列表中**：
   - 正常路由（第一个匹配的 binding）

注意：广播组不绕过频道白名单或群组激活规则（提及/命令等）。它们仅改变消息符合处理条件时_运行哪些 agent_。

### 会话隔离

广播组中的每个 agent 维护完全独立的：

- **会话键**（`agent:alfred:whatsapp:group:120363...` vs `agent:baerbel:whatsapp:group:120363...`）
- **对话历史**（agent 看不到其他 agent 的消息）
- **工作区**（如已配置，使用独立沙箱）
- **工具访问**（不同的允许/拒绝列表）
- **记忆/上下文**（独立的 IDENTITY.md、SOUL.md 等）
- **群上下文缓冲区**（用于上下文的近期群消息）按 peer 共享，所有广播 agent 触发时看到相同上下文

每个 agent 可以拥有：

- 不同的人格
- 不同的工具访问（如只读 vs 读写）
- 不同的模型（如 opus vs sonnet）
- 不同的已安装技能

### 示例：隔离会话

在群组 `120363403215116621@g.us` 中，agents 为 `["alfred", "baerbel"]`：

**Alfred 的上下文：**

```
Session: agent:alfred:whatsapp:group:120363403215116621@g.us
History: [用户消息, alfred 的先前回复]
Workspace: /Users/pascal/openacosmi-alfred/
Tools: read, write, exec
```

**Bärbel 的上下文：**

```
Session: agent:baerbel:whatsapp:group:120363403215116621@g.us
History: [用户消息, baerbel 的先前回复]
Workspace: /Users/pascal/openacosmi-baerbel/
Tools: read only
```

## 最佳实践

### 1. 保持 Agent 聚焦

设计每个 agent 具有单一、明确的职责：

```json
{
  "broadcast": {
    "DEV_GROUP": ["formatter", "linter", "tester"]
  }
}
```

✅ **好：** 每个 agent 一个任务
❌ **差：** 一个通用的 "dev-helper" agent

### 2. 使用描述性名称

让每个 agent 的功能一目了然。

### 3. 配置不同的工具访问

仅给 agent 必要的工具。

### 4. 监控性能

多 agent 时，考虑：

- 使用 `"strategy": "parallel"`（默认）以提高速度
- 将广播组限制在 5-10 个 agent
- 为简单 agent 使用更快的模型

### 5. 优雅处理故障

agent 独立失败。一个 agent 的错误不会阻塞其他 agent：

```
消息 → [Agent A ✓, Agent B ✗ 错误, Agent C ✓]
结果: Agent A 和 C 回复, Agent B 记录错误
```

## 兼容性

### 平台支持

| 平台 | 状态 |
|------|------|
| WhatsApp | ✅ 已实现 |
| Telegram | 🚧 计划中 |
| Discord | 🚧 计划中 |
| Slack | 🚧 计划中 |

### 路由

广播组与现有路由并存：

```json
{
  "bindings": [
    {
      "match": { "channel": "whatsapp", "peer": { "kind": "group", "id": "GROUP_A" } },
      "agentId": "alfred"
    }
  ],
  "broadcast": {
    "GROUP_B": ["agent1", "agent2"]
  }
}
```

- `GROUP_A`：仅 alfred 回复（正常路由）
- `GROUP_B`：agent1 和 agent2 都回复（广播）

**优先级：** `broadcast` 优先于 `bindings`。

## 故障排查

### Agent 不响应

**检查：**

1. Agent ID 存在于 `agents.list` 中
2. Peer ID 格式正确（如 `120363403215116621@g.us`）
3. Agent 不在拒绝列表中

**调试：**

```bash
tail -f ~/.openacosmi/logs/gateway.log | grep broadcast
```

### 仅一个 Agent 响应

**原因：** Peer ID 可能在 `bindings` 中但不在 `broadcast` 中。

**修复：** 添加到 broadcast 配置或从 bindings 中移除。

### 性能问题

**如果多 Agent 时较慢：**

- 减少每组的 agent 数量
- 使用更轻量的模型（sonnet 代替 opus）
- 检查沙箱启动时间

## API 参考

### 配置 Schema

```go
// Go Gateway 配置结构 (backend/internal/config/schema.go)
type BroadcastConfig struct {
    Strategy string              `json:"strategy,omitempty"` // "parallel" | "sequential"
    Groups   map[string][]string // peerId -> []agentId
}
```

### 字段说明

- `strategy`（可选）：agent 处理方式
  - `"parallel"`（默认）：所有 agent 同时处理
  - `"sequential"`：agent 按数组顺序处理
- `[peerId]`：WhatsApp 群组 JID、E.164 号码或其他 peer ID
  - 值：应处理消息的 agent ID 数组

## 限制

1. **最大 agent 数**：无硬限制，但 10+ 个 agent 可能较慢
2. **共享上下文**：agent 看不到彼此的回复（设计如此）
3. **消息顺序**：并行回复可能以任意顺序到达
4. **速率限制**：所有 agent 计入 WhatsApp 速率限制

## 未来增强

计划功能：

- [ ] 共享上下文模式（agent 可看到彼此的回复）
- [ ] Agent 协调（agent 可以互相发信号）
- [ ] 动态 Agent 选择（根据消息内容选择 agent）
- [ ] Agent 优先级（某些 agent 先于其他 agent 回复）

## 另请参阅

- [多 Agent 配置](/tools/multi-agent-sandbox-tools)
- [路由配置](/channels/channel-routing)
- [会话管理](/concepts/sessions)
