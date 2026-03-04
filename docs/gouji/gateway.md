# 模块架构文档 — Gateway 核心层

> 最后更新：2026-02-26 | 代码级审计完成

## 1. 模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `internal/gateway/` |
| Go 源文件数 | 105 |
| Go 测试文件数 | 47 |
| 测试函数数 | 492 |
| 源码行数 | ~33,100 |

Gateway 是系统最大的模块，负责 HTTP 服务、WebSocket 通信、聊天状态管理、配置热重载、频道管理、设备配对、安装向导、OpenAI 兼容 API 等核心功能。

## 2. 文件索引 (105 个源文件)

### 2.1 服务器核心 (5 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `server.go` | 1517 | 网关服务器主入口、DI 组装、启动编排 |
| `server_http.go` | ~350 | HTTP 路由注册、中间件链 |
| `boot.go` | 476 | 引导/状态管理 |
| `http.go` | ~350 | HTTP 框架、Router、中间件 |
| `httputil.go` | ~100 | HTTP 工具函数 |

### 2.2 WebSocket (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `ws.go` | ~350 | WebSocket 客户端 + 自动重连 |
| `ws_server.go` | 516 | WebSocket 服务器 |
| `ws_log.go` | 641 | WebSocket 日志记录 |
| `ws_close_codes.go` | ~50 | WebSocket 关闭码定义 |

### 2.3 认证与安全 (5 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `auth.go` | ~300 | Token/密码/Tailscale 认证 |
| `net.go` | 432 | IP/HTTP 工具、Tailnet IP 查询 |
| `origin_check.go` | ~100 | Origin 检查 (CSRF 防护) |
| `tls_runtime.go` | ~100 | TLS 运行时配置 |
| `device_auth.go` | ~200 | 设备认证 |

### 2.4 聊天与事件 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `chat.go` | 455 | 聊天状态 + 事件处理器 |
| `broadcast.go` | ~250 | 广播系统 (scope guard + 背压) |
| `events.go` | ~300 | 节点事件处理 |
| `agent_run_context.go` | ~250 | Agent 运行上下文 + seq 计数 |

### 2.5 配置与通道 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `reload.go` | 541 | 配置热重载 (diff + 规则引擎) |
| `channels.go` | ~200 | 频道注册 + 工具策略接口 |
| `hooks.go` | ~250 | 钩子配置解析 |
| `hooks_mapping.go` | 513 | 钩子匹配 + Transform 管道 |

### 2.6 会话管理 (6 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `sessions.go` | 565 | 会话存储 + 主键解析 |
| `session_utils.go` | 554 | 会话工具 (标题推导/分类) |
| `session_utils_fs.go` | 508 | 会话文件系统操作 |
| `session_utils_types.go` | ~100 | 会话工具类型 |
| `session_metadata.go` | ~200 | 会话元数据 |
| `idempotency.go` | ~100 | 幂等性检查 |

### 2.7 设备与配对 (3 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `device_pairing.go` | 843 | 设备配对全流程 |
| `server_discovery.go` | ~250 | 服务发现 |
| `server_tailscale.go` | 535 | Tailscale 集成 |

### 2.8 远程审批 (5 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `remote_approval.go` | 418 | 远程审批核心逻辑 |
| `remote_approval_callback_verify.go` | ~150 | 回调验证 |
| `remote_approval_feishu.go` | ~200 | 飞书审批适配 |
| `remote_approval_dingtalk.go` | ~200 | 钉钉审批适配 |
| `remote_approval_wecom.go` | ~200 | 企业微信审批适配 |

### 2.9 权限升级 (3 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `permission_escalation.go` | 456 | 权限升级框架 |
| `escalation_audit.go` | ~200 | 升级审计 |
| `task_preset_permissions.go` | ~150 | 任务预设权限 |

### 2.10 OpenAI 兼容层 (3 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `openai_http.go` | 553 | OpenAI Chat Completions API |
| `openresponses_http.go` | 836 | OpenAI Responses API |
| `openresponses_types.go` | ~200 | Responses API 类型定义 |

### 2.11 频道桥接 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `channel_deps_adapter.go` | 612 | 频道依赖适配器 (DI) |
| `channel_monitor_start.go` | ~200 | 频道监控启动 |
| `channel_pairing_bridge.go` | ~150 | 频道配对桥接 |
| `server_channel_webhooks.go` | ~200 | 频道 Webhook 处理 |

### 2.12 Server Methods — API 处理器 (36 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `server_methods.go` | ~350 | 方法上下文 + 路由注册 |
| `server_methods_interfaces.go` | ~200 | 方法接口定义 |
| `server_methods_chat.go` | 512 | `chat.*` 处理器 |
| `server_methods_sessions.go` | 701 | `sessions.*` 处理器 |
| `server_methods_config.go` | 491 | `config.*` 处理器 |
| `server_methods_agents.go` | 576 | `agents.*` 处理器 |
| `server_methods_agent.go` | ~300 | `agent.*` 处理器 |
| `server_methods_agent_files.go` | ~200 | `agents.files.*` 处理器 |
| `server_methods_agent_rpc.go` | ~150 | Agent RPC 处理器 |
| `server_methods_nodes.go` | 682 | `node.*` 11 方法 |
| `server_methods_skills.go` | 609 | `skills.*` 处理器 |
| `server_methods_browser.go` | 438 | `browser.*` 处理器 |
| `server_methods_usage.go` | 824 | `usage.*` 处理器 (JSONL 解析) |
| `server_methods_memory.go` | 472 | `memory.*` UHMS 处理器 |
| `server_methods_send.go` | ~300 | `send.*` 处理器 |
| `server_methods_cron.go` | ~200 | `cron.*` 处理器 |
| `server_methods_devices.go` | ~200 | `devices.*` 处理器 |
| `server_methods_channels.go` | ~200 | `channels.*` 处理器 |
| `server_methods_models.go` | ~200 | `models.*` 处理器 |
| `server_methods_security.go` | ~200 | `security.*` 处理器 |
| `server_methods_logs.go` | ~200 | `logs.*` 处理器 |
| `server_methods_update.go` | ~200 | `update.*` 处理器 |
| `server_methods_system.go` | ~200 | `system.*` 处理器 |
| `server_methods_tts.go` | ~200 | `tts.*` 处理器 |
| `server_methods_stt.go` | ~100 | `stt.*` 处理器 |
| `server_methods_talk.go` | ~150 | `talk.*` 处理器 |
| `server_methods_mcp.go` | ~200 | MCP 协议处理器 |
| `server_methods_sandbox.go` | ~200 | `sandbox.*` 处理器 |
| `server_methods_exec_approvals.go` | ~200 | 执行审批处理器 |
| `server_methods_remote_approval.go` | ~200 | 远程审批处理器 |
| `server_methods_escalation.go` | ~200 | 权限升级处理器 |
| `server_methods_uhms.go` | ~200 | UHMS 记忆处理器 |
| `server_methods_rules.go` | ~200 | 规则处理器 |
| `server_methods_web.go` | ~200 | Web 处理器 |
| `server_methods_docconv.go` | ~100 | 文档转换处理器 |
| `server_methods_argus.go` | ~150 | Argus MCP 桥接处理器 |
| `server_methods_voicewake.go` | ~100 | 语音唤醒处理器 |
| `server_methods_task_presets.go` | ~100 | 任务预设处理器 |
| `server_methods_wizard.go` | ~200 | 安装向导处理器 |
| `server_methods_stubs.go` | ~100 | 未实现方法桩 |

### 2.13 安装向导 (6 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `wizard_onboarding.go` | 916 | Onboarding 主流程 |
| `wizard_finalize.go` | 479 | 向导完成 + 配置写入 |
| `wizard_session.go` | 455 | 向导会话管理 |
| `wizard_auth.go` | ~200 | 向导认证配置 |
| `wizard_gateway_config.go` | ~200 | 向导网关配置 |
| `wizard_helpers.go` | ~150 | 向导辅助函数 |

### 2.14 其他 (11 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `dispatch_inbound.go` | ~200 | `PipelineDispatcher` 管线桥接 |
| `delivery_context.go` | ~150 | 消息投递上下文 |
| `tools.go` | ~300 | 工具调用注册表 |
| `tools_invoke_http.go` | ~250 | 工具调用 HTTP 代理 |
| `protocol.go` | ~200 | 网关协议定义 |
| `transcript.go` | ~200 | 转录管理 |
| `maintenance.go` | ~150 | 维护模式 |
| `restart_sentinel.go` | ~100 | 重启哨兵 |
| `system_presence.go` | ~100 | 系统在线状态 |
| `server_browser.go` | ~200 | 浏览器控制服务器 |
| `server_plugins.go` | ~200 | 插件服务器 |
| `server_multimodal.go` | ~150 | 多模态处理 |
| `node_command_policy.go` | ~180 | 节点命令策略 |

## 3. 并发安全设计

| 组件 | 保护机制 |
|------|----------|
| `ChatRunRegistry` | `sync.Mutex` 保护 session 队列 |
| `ChatRunState` | `sync.Map` 用于 buffers/deltaSentAt |
| `ToolEventRecipientRegistry` | `sync.Mutex` + TTL 自动清理 |
| `ToolRegistry` | `sync.RWMutex` 保护工具注册表 |
| `Broadcaster` | `sync.RWMutex` 保护客户端列表 |
| `ConfigWatcher` | `sync.Mutex` + timer 回调加锁 |
| `GatewayState` | `sync.RWMutex` 保护阶段状态 |
| `AgentRunContextStore` | `sync.Map` + `sync.RWMutex` |
| `SessionStore` | `sync.RWMutex` 保护会话映射 |

## 4. 测试覆盖

| 指标 | 值 |
|------|------|
| 测试文件 | 47 |
| 测试函数 | 492 |
| 覆盖范围 | net, auth, broadcast, ws, chat, reload, events, tools, http, boot, sessions, config, agents, usage, browser, nodes, skills, cron, devices, security, wizard |

所有测试 `go test -race` 通过。
