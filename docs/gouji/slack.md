# Slack SDK 架构文档

> 最后更新：2026-02-26 | 代码级审计确认 | 37 源文件, 4 测试文件, 58 测试, ~6,498 行

## 一、模块概述

Slack 频道适配器，负责与 Slack API 的完整交互：消息收发、事件处理、Socket Mode / HTTP 事件监听、线程管理、斜杠命令等。是文件数最多的频道 SDK（37 Go 文件）。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件 | 大小 | 职责 |
|------|------|------|
| `src/slack/` | 65 文件 | Slack 主 SDK |
| `src/slack/monitor/` | 20+ 文件 | 事件监听+消息处理管线 |

### 核心逻辑摘要

- Socket Mode 和 HTTP 事件双模式监听
- 消息 preflight（allowlist/mention/bot 过滤）→ 处理 → 回复投递管线
- 频道/用户缓存预加载 + API fallback
- 线程历史补全 + 流式编辑 + 状态反应
- 斜杠命令处理 + ephemeral 回复

## 三、依赖分析

### 显式依赖图

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `pkg/contracts` | 接口 | ↓ | 频道抽象层 |
| `pkg/types` | 类型 | ↓ | 共享类型 |
| `internal/config` | 值 | ↓ | 配置读取 |
| `internal/channels/bridge` | 值 | ↓ | 频道桥接 |
| `internal/routing` | 值 | ↓ | session key 路由 |

### 隐藏依赖审计

| 类别 | 结果 | Go 等价方案 |
|------|------|-------------|
| npm 包黑盒行为 | ✅ | `@slack/socket-mode` → `slack-go/slack` v0.17.3 `socketmode` 子包 |
| 全局状态/单例 | ⚠️ | 频道/用户缓存 → `sync.Map` |
| 事件总线/回调链 | ⚠️ | Socket Mode 事件分发 → channel + goroutine |
| 环境变量依赖 | ⚠️ | `SLACK_BOT_TOKEN`/`SLACK_APP_TOKEN` |
| 文件系统约定 | ✅ | — |
| 协议/消息格式 | ⚠️ | Events API 签名校验 + envelope ack |
| 错误处理约定 | ⚠️ | Slack API 速率限制 + `ok: false` 处理 |

## 四、重构实现（Go）

### 文件结构（37 文件）

**基础层：**

| 文件 | 职责 |
|------|------|
| `types.go` | Slack 核心类型 |
| `token.go` | Token 管理 + 验证 |
| `client.go` | HTTP API 客户端（含 `files.uploadV2` 3 步 API） |
| `accounts.go` | 多账户解析 |
| `targets.go` | 目标标识符处理 |
| `scopes.go` | OAuth scope 管理 |
| `probe.go` | API 可用性探测 |

**消息层：**

| 文件 | 职责 |
|------|------|
| `send.go` | 消息发送（Phase 7 媒体桩） |
| `format.go` | Markdown→mrkdwn 格式化（Phase 7 IR 桩） |
| `actions.go` | 消息动作处理 |
| `threading.go` | 线程上下文管理 |
| `threading_tool_context.go` | 工具调用线程上下文 |

**监控层（Phase 9 A5 完成）：**

| 文件 | 职责 | 状态 |
|------|------|----- |
| `monitor_deps.go` | DI 接口 (9 项注入点) | ✅ Phase 9 |
| `monitor_provider.go` | Socket Mode (`slack-go`) + HTTP 监听 | ✅ Phase 9 |
| `monitor_context.go` | 缓存 + API 回填 + dedup + auth.test | ✅ Phase 9 |
| `monitor_events_*.go` (5) | 消息/频道/成员/Pin/反应事件 | ✅ Phase 9 |
| `monitor_message_prepare.go` | 11 步入站过滤管线 | ✅ Phase 9 |
| `monitor_message_dispatch.go` | agent 路由 + MsgContext + auto-reply | ✅ Phase 9 |
| `monitor_replies.go` | 分块发送 + 反应状态 (⏳→✅) | ✅ Phase 9 |
| `monitor_slash.go` | 斜杠命令 + ephemeral | ✅ Phase 9 |
| `monitor_auth.go` | allowlist + pairing store | ✅ Phase 9 |
| `monitor_media.go` | Bot token 文件下载 | ✅ Phase 9 |
| `monitor_thread_resolution.go` | conversations.replies + 历史裁剪 | ✅ Phase 9 |

**辅助：**

| 文件 | 职责 |
|------|------|
| `resolve_channels.go` | 频道 ID→名称解析 |
| `resolve_users.go` | 用户 ID→名称解析 |
| `directory_live.go` | 目录实时查询 |
| `channel_migration.go` | 频道 ID 迁移 |
| `http_registry.go` | HTTP endpoint 注册 |
| `allow_list.go` | 允许列表管理 |
| `monitor_policy.go` | 频道监控策略 |
| `monitor_commands.go` | 命令注册 |
| `monitor_channel_config.go` | 频道配置 |

| 维度 | 原版 TS | 重构 Go |
|------|---------|---------|
| WebSocket | `@slack/socket-mode` | `slack-go/slack` v0.17.3 `socketmode` |
| 事件模型 | EventEmitter + 回调 | channel + goroutine |
| 格式化 | 自定义 IR | Markdown IR 管线 (部分接入) |
| 消息过滤 | prepare.ts | 11 步过滤管线 |
| 测试 | vitest | ✅ DF-C1 上传测试 + 待 Phase 10 集成 |

## 六、Rust 下沉候选

无（I/O 密集型，非计算密集型）。

## 七、测试覆盖

| 测试类型 | 覆盖范围 | 状态 |
|----------|----------|------|
| 单元测试 | `client_upload_test.go` — `UploadFileV2` 3 步 API mock | ✅ 6 PASS |
| 集成测试 | 需真实 Slack App | ❓ 待 Phase 10 |
| 编译检查 | `go build` + `go vet` | ✅ PASS |
