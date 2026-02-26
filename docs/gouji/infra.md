# infra 基础设施架构文档

> 最后更新：2026-02-26 | 代码级审计完成

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/internal/infra/` |
| Go 源文件数 | 77 (含 cost/ 子包 14 + errors/ 子包 1) |
| Go 测试文件数 | 13 |
| 测试函数数 | 101 |
| 总行数 | ~12,900 |

`internal/infra/` 是系统底层基础设施模块，被 gateway、agents、autoreply 等上层模块依赖。

## 二、子系统分类 (77 个源文件)

### 2.1 心跳系统 (6 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `heartbeat.go` | 356 | 心跳调度器 (`HeartbeatRunner`)、agent 状态管理、活跃时段判断 |
| `heartbeat_delivery.go` | 144 | 心跳投递类型 + 配置 + DI 接口 |
| `heartbeat_delivery_run.go` | 94 | `RunHeartbeatOnce` + 批量投递 |
| `heartbeat_events.go` | 135 | 心跳事件发射 (`EmitHeartbeatEvent`) |
| `heartbeat_visibility.go` | 96 | 心跳抑制判断 (`ShouldSuppressHeartbeat`) |
| `heartbeat_wake.go` | 185 | coalesce+retry 心跳唤醒 (`HeartbeatWaker`) |

### 2.2 设备身份与安全 (5 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `device_identity.go` | 404 | ED25519 密钥对生成/加载、设备 ID 派生、签名/验签 |
| `device_auth_store.go` | 263 | 设备认证令牌存储 (JSON 持久化) |
| `tls_fingerprint.go` | 72 | TLS 证书指纹计算 |
| `tls_gateway.go` | 152 | TLS 网关配置 + 自签名证书生成 |
| `exec_safety.go` | 91 | 执行安全检查 (命令白名单) |

### 2.3 端口与网络 (5 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `ports.go` | 449 | 端口管理 (可用性探测/lsof 诊断/进程识别) |
| `gateway_lock.go` | 227 | 网关文件锁 (跨进程互斥) |
| `gateway_lock_unix.go` | 18 | Unix flock 实现 |
| `gateway_lock_windows.go` | 86 | Windows LockFileEx 实现 |
| `fetch_guard.go` | 193 | HTTP 请求限流守卫 |

### 2.4 服务发现 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `discovery.go` | 352 | 局域网网关发现 (dns-sd/avahi) |
| `bonjour.go` | 222 | mDNS 信标注册 + TXT 记录 |
| `bonjour_zeroconf.go` | 113 | zeroconf 库桥接 |
| `widearea_dns.go` | 333 | 广域 DNS 发现 |

### 2.5 节点配对 (2 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `node_pairing.go` | 120 | 配对类型 + JSON 持久化 |
| `node_pairing_ops.go` | 256 | Request/Approve/Verify 操作 |

### 2.6 审批系统 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `exec_approvals.go` | 306 | 执行审批 allowlist 管理 |
| `exec_approvals_ops.go` | 88 | 审批操作补全 |
| `approval_forwarder.go` | 273 | 审批事件 → 频道消息转发 |
| `approval_forwarder_ops.go` | 172 | Handle/Stop/Target 操作 |

### 2.7 状态迁移 (8 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `state_migrations_types.go` | 70 | 迁移类型定义 |
| `state_migrations_fs.go` | 103 | 迁移 FS 工具 |
| `state_migrations_keys.go` | 100 | session key 规范化 |
| `state_migrations_detect.go` | 114 | 旧目录检测 |
| `state_migrations_run.go` | 103 | 迁移编排 |
| `state_migrations_wa.go` | 42 | WhatsApp auth 迁移 |
| `state_migrations_statedir.go` | 119 | 状态目录迁移 |
| `state_migrations_store.go` | 149 | store 读写 + 合并 + JSON5 |

### 2.8 更新系统 (4 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `update_check.go` | 264 | 版本更新检查 |
| `update_runner.go` | 147 | 更新执行器 |
| `update_channels.go` | 128 | 更新渠道 (stable/beta/nightly) |
| `update_startup.go` | 116 | 启动时更新检查 |

### 2.9 平台适配 (5 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `platform_brew.go` | 98 | Homebrew 检测 + cask 路径 |
| `platform_brew_stub.go` | 15 | 非 macOS 桩 |
| `platform_wsl.go` | 55 | WSL 检测 |
| `platform_wsl_stub.go` | 15 | 非 Windows 桩 |
| `platform_clipboard.go` | 25 | 剪贴板工具 |

### 2.10 事件与运行时 (6 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `agent_events.go` | 154 | Agent 运行上下文全局注册表 |
| `system_events.go` | 165 | 会话级事件队列 (去重+上限) |
| `system_presence.go` | 106 | 系统在线状态 |
| `diagnostic_events.go` | 194 | 诊断事件收集 |
| `diagnostic_flags.go` | 111 | 诊断标志管理 |
| `channel_activity.go` | 117 | 频道活跃度追踪 |

### 2.11 基础工具 (17 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `retry.go` | 258 | 通用重试 (指数退避 + jitter) |
| `retry_policy.go` | 92 | 重试策略配置 |
| `restart.go` | 229 | 进程重启管理 |
| `archive.go` | 173 | tar.gz 压缩/解压 |
| `dedupe.go` | 135 | 消息去重 (TTL 缓存) |
| `env_vars.go` | 159 | 环境变量工具 |
| `exec_host.go` | 189 | 命令执行宿主 |
| `ssh_config.go` | 124 | SSH 配置解析 |
| `ssh_tunnel.go` | 270 | SSH 隧道管理 |
| `skills_remote.go` | 167 | 远程节点技能可用性 |
| `channel_summary.go` | 116 | 频道摘要生成 |
| `control_ui_assets.go` | 287 | 控制面板 UI 静态资源 |
| `canvas_host_url.go` | 112 | Canvas 宿主 URL 解析 |
| `transport_ready.go` | 112 | 传输就绪检查 |
| `voicewake.go` | 126 | 语音唤醒 |
| `warning_filter.go` | 64 | 警告过滤 |
| `runtime_guard.go` | 55 | 运行时守卫 |

### 2.12 小型工具 (11 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `home_dir.go` | 77 | 主目录解析 |
| `openacosmi_root.go` | 77 | 安装根目录 |
| `machine_name.go` | 89 | 机器名获取 |
| `path_env.go` | 85 | PATH 环境变量 |
| `git_commit.go` | 74 | Git 最新提交 |
| `iso639.go` | 69 | ISO 639 语言代码 |
| `shell_env.go` | 102 | Shell 环境加载 |
| `fs_safe.go` | 95 | 安全文件操作 |
| `json_file.go` | 39 | JSON 文件读写 |
| `os_summary.go` | 32 | 操作系统摘要 |
| `infra_time.go` | 9 | 时间工具 |

### 2.13 cost/ 子包 (14 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `types.go` | — | 费用统计类型定义 |
| `cost_summary.go` | — | 费用汇总计算 |
| `session_cost.go` | — | 会话级费用追踪 |
| `provider_types.go` | — | Provider 费用类型 |
| `provider_shared.go` | — | Provider 共享逻辑 |
| `provider_auth.go` | — | Provider 认证 |
| `provider_fetch.go` | — | Provider 费用拉取 |
| `provider_fetch_claude.go` | — | Claude 费用拉取 |
| `provider_fetch_codex.go` | — | Codex 费用拉取 |
| `provider_fetch_copilot.go` | — | Copilot 费用拉取 |
| `provider_fetch_gemini.go` | — | Gemini 费用拉取 |
| `provider_fetch_minimax.go` | — | MiniMax 费用拉取 |
| `provider_fetch_zai.go` | — | ZAI 费用拉取 |
| `provider_format.go` | — | 费用格式化 |

### 2.14 errors/ 子包 (1 文件)

| 文件 | 行数 | 职责 |
|------|------|------|
| `errors.go` | — | 结构化错误类型 (错误码 + 分类) |

## 三、测试覆盖

| 测试文件 | 覆盖范围 |
|----------|----------|
| `heartbeat_test.go` | 心跳调度 + 活跃时段 |
| `heartbeat_wake_test.go` | 唤醒 coalesce |
| `system_events_test.go` | 事件队列去重 |
| `agent_events_test.go` | Agent 注册/事件 |
| `ports_test.go` | 端口探测 + lsof 解析 |
| `retry_test.go` | 重试策略 + 退避 |
| `widearea_dns_test.go` | DNS 解析 |
| `bonjour_zeroconf_test.go` | mDNS 注册 |
| `update_test.go` | 更新检查 |
| `util_test.go` | 工具函数 |
| `cost/cost_test.go` | 费用统计 |
| `cost/provider_fetch_test.go` | Provider 拉取 |
| `errors/errors_test.go` | 错误类型 |
| **合计** | **101 个测试函数** |
