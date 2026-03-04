# 共享类型库架构文档

> 最后更新：2026-02-26 | 代码级审计完成 | 35 源文件 (含 1 测试), ~3,367 行

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/pkg/types/` |
| Go 文件数 | 35 |
| 总行数 | ~3,367 |
| 角色 | 项目全局共享类型库 |

**系统中被引用最广泛的包**。定义所有模块通用的配置结构体、Agent 定义、频道类型、模型配置等。几乎所有 `internal/` 和 `pkg/` 包都依赖此包。

## 二、文件索引（按领域分组）

### 核心配置

| 文件 | 行 | 定义 |
|------|---|------|
| `types.go` | ~130 | `OpenAcosmiConfig` 主配置结构体、日志级别 `LogLevel` |
| `types_openacosmi.go` | ~80 | 全局开关、feature flags |
| `types_gateway.go` | ~100 | 网关配置：端口、CORS、WebSocket |

### Agent 领域

| 文件 | 行 | 定义 |
|------|---|------|
| `types_agents.go` | ~150 | `AgentDefinition`、Agent 配置、工具策略 |
| `types_agent_defaults.go` | ~60 | Agent 默认模型/参数配置 |
| `types_models.go` | ~120 | `ModelDefinitionConfig`、Provider 枚举 |
| `types_skills.go` | ~80 | 技能定义、技能过滤规则 |
| `types_tools.go` | ~70 | Agent 工具配置 |

### 频道领域 (12 文件)

| 文件 | 覆盖平台 |
|------|----------|
| `types_channels.go` | 通用频道配置基础 |
| `types_telegram.go` | Telegram 特有字段 |
| `types_discord.go` + `_pluralkit.go` | Discord + PluralKit |
| `types_slack.go` | Slack OAuth + Socket Mode |
| `types_whatsapp.go` | WhatsApp Cloud API |
| `types_signal.go` | Signal signald |
| `types_imessage.go` | iMessage AppleScript |
| `types_feishu.go` | 飞书 |
| `types_dingtalk.go` | 钉钉 |
| `types_wecom.go` | 企业微信 |
| `types_googlechat.go` | Google Chat |
| `types_msteams.go` | Microsoft Teams |

### 功能模块

| 文件 | 行 | 定义 |
|------|---|------|
| `types_messages.go` | ~100 | 入站/出站消息结构体、媒体附件 |
| `types_media.go` | ~60 | 媒体处理配置 |
| `types_memory.go` | ~70 | UHMS 记忆系统配置 |
| `types_browser.go` | ~50 | 浏览器引擎配置 |
| `types_tts.go` | ~40 | TTS 语音合成配置 |
| `types_sandbox.go` | ~40 | 沙箱执行配置 |
| `types_node_host.go` | ~30 | Node.js 宿主配置 |
| `types_plugins.go` | ~60 | 插件系统配置 |
| `types_hooks.go` | ~50 | 钩子系统配置 |
| `types_cron.go` | ~40 | 定时任务配置 |
| `types_queue.go` | ~30 | 消息队列配置 |
| `types_approvals.go` | ~40 | 审批流程配置 |
| `types_auth.go` | ~50 | 认证/OAuth 配置 |

## 三、设计原则

1. **纯数据定义**：只包含 struct + const + 数据方法，无业务逻辑
2. **JSON tag 全覆盖**：所有字段带 `json` tag，支持配置文件序列化
3. **频道一文件**：每个平台单独文件，避免冲突
4. **零循环依赖**：`pkg/types` 不依赖任何 `internal/` 包
