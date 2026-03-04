# 隐藏依赖跟踪文件

> 创建日期：2026-02-19
> 来源：W1~W5 审计 + TS 源码全量扫描
> 说明：本文件跟踪所有非显式功能差异，包括环境变量、格式约定、隐式行为等

---

## 类别一：环境变量依赖（TS 有，Go 缺失/未读取）

### 1.1 TS 定义但 Go 完全未读取的关键变量

| 变量名 | TS 用途 | 影响模块 | 修复状态 |
|--------|---------|---------|---------|
| `OPENACOSMI_ALLOW_MULTI_GATEWAY` | 允许多实例 gateway | gateway | [ ] |
| `OPENACOSMI_ANTHROPIC_PAYLOAD_LOG` | 启用 API 载荷日志 | llmclient | [✅] `llmclient/anthropic.go:45` os.Getenv 读取 |
| `OPENACOSMI_ANTHROPIC_PAYLOAD_LOG_FILE` | 载荷日志文件路径 | llmclient | [✅] `llmclient/anthropic.go:51` os.Getenv 读取 |
| `OPENACOSMI_BROWSER_CDP_PORT` | CDP 端口覆盖 | browser | [✅] `sandbox/manage.go:492` 传入容器 `-e` |
| `OPENACOSMI_BROWSER_VNC_PORT` | VNC 端口覆盖 | browser | [✅] `sandbox/manage.go:493` 传入容器 `-e` |
| `OPENACOSMI_BROWSER_NOVNC_PORT` | noVNC 端口覆盖 | browser | [✅] `sandbox/manage.go:494` 传入容器 `-e` |
| `OPENACOSMI_BROWSER_HEADLESS` | 浏览器无头模式 | browser | [✅] `sandbox/manage.go:490` 传入容器 `-e` |
| `OPENACOSMI_BROWSER_ENABLED` | 启用浏览器功能 | browser | [ ] |
| `OPENACOSMI_BROWSER_ENABLE_NOVNC` | 启用 noVNC | browser | [✅] `sandbox/manage.go:491` 传入容器 `-e` |
| `OPENACOSMI_CONFIG_CACHE_MS` | 配置文件缓存 TTL | config | [ ] |
| `OPENACOSMI_SESSION_CACHE_TTL_MS` | Session 缓存 TTL（当前 Go 硬编码 45s） | sessions | [✅] W-P 确认已实现（`sessions/store.go` L211-221）|
| `OPENACOSMI_SESSION_MANAGER_CACHE_TTL_MS` | Session manager 缓存 TTL | sessions | [ ] |
| `OPENACOSMI_SKIP_CRON` | 跳过 cron 启动 | cron | [✅] `config/features.go:14` os.Getenv |
| `OPENACOSMI_SKIP_CHANNELS` | 跳过通道启动 | channels | [✅] `config/features.go:18` os.Getenv |
| `OPENACOSMI_SKIP_BROWSER_CONTROL_SERVER` | 跳过浏览器控制服务 | browser | [✅] `config/features.go:22` os.Getenv |
| `OPENACOSMI_SKIP_CANVAS_HOST` | 跳过 Canvas 主机 | gateway | [✅] `config/features.go:26` + `infra/canvas_host_url.go:110` |
| `OPENACOSMI_SKIP_PROVIDERS` | 跳过 provider 初始化 | agents | [✅] `config/features.go:30` os.Getenv |
| `OPENACOSMI_SKIP_GMAIL_WATCHER` | 跳过 Gmail 监听器 | hooks | [✅] 已实现 |
| `OPENACOSMI_SSH_PORT` | SSH 端口覆盖 | infra | [ ] |
| `OPENACOSMI_UPDATE_IN_PROGRESS` | 更新进行中标记 | infra | [ ] |
| `OPENACOSMI_TEST_HANDSHAKE_TIMEOUT_MS` | 握手超时（测试用） | gateway | [ ] |
| `OPENACOSMI_LAUNCHD_LABEL` | macOS launchd 标签 | daemon | [✅] `daemon/program_args.go:69` + `launchd_darwin.go:206` 完整实现 |
| `OPENACOSMI_SYSTEMD_UNIT` | Linux systemd 单元名 | daemon | [✅] `daemon/program_args.go:70` + `systemd_linux.go:228` 完整实现 |
| `OPENACOSMI_MDNS_HOSTNAME` | mDNS 主机名覆盖 | infra | [✅] `infra/bonjour.go:78` os.Getenv |
| `OPENACOSMI_RAW_STREAM` | 原始流模式 | gateway | [✅] `runner/raw_stream.go:29` os.Getenv |
| `OPENACOSMI_RAW_STREAM_PATH` | 原始流路径 | gateway | [✅] `runner/raw_stream.go:34` os.Getenv |
| `OPENACOSMI_NO_RESPAWN` | 禁止自动重启 | daemon | [ ] |
| `OPENACOSMI_NODE_EXEC_FALLBACK` | Node 执行器回退 | infra | [ ] |
| `OPENACOSMI_NODE_EXEC_HOST` | Node 执行器主机 | infra | [ ] |
| `OPENACOSMI_BASH_PENDING_MAX_OUTPUT_CHARS` | Bash 工具最大输出字符 | agents/bash | [✅] `bash/exec.go:38` os.Getenv |
| `OPENACOSMI_CACHE_TRACE` | 缓存追踪调试 | infra | [✅] `bash/cache_trace.go:186` 完整实现 |
| `OPENACOSMI_CACHE_TRACE_FILE` | 缓存追踪文件 | infra | [✅] `bash/cache_trace.go:192` 完整实现 |
| `OPENACOSMI_DEBUG_MEMORY_EMBEDDINGS` | Memory 嵌入调试 | memory | [ ] |
| `OPENACOSMI_DEBUG_TELEGRAM_ACCOUNTS` | Telegram 账户调试 | channels | [✅] `telegram/accounts.go:322` |

### 1.2 Go 已读取（已对齐）✅

| 变量名 | 模块 |
|--------|------|
| `OPENACOSMI_HOME` | config/paths |
| `OPENACOSMI_STATE_DIR` | config/paths |
| `OPENACOSMI_CONFIG_DIR` | config/paths |
| `OPENACOSMI_STORE_PATH` | config |
| `OPENACOSMI_PROFILE` | config |
| `OPENACOSMI_DISABLE_BONJOUR` | infra/bonjour |
| `OPENACOSMI_GATEWAY_PASSWORD` | gateway |
| `OPENACOSMI_GATEWAY_TOKEN` | gateway |
| `OPENACOSMI_NIX_MODE` | config |
| `OPENACOSMI_LOAD_SHELL_ENV` | config |
| `OPENACOSMI_BUNDLED_HOOKS_DIR` | hooks |
| `OPENACOSMI_BUNDLED_PLUGINS_DIR` | plugins |
| `OPENACOSMI_BUNDLED_SKILLS_DIR` | skills |
| 所有 API Key 变量 | agents/models |

### 1.3 第三方 API Key 变量（Go 需确认读取）

| 变量名 | 用途 | 修复状态 |
|--------|------|---------|
| `BRAVE_API_KEY` | Brave Search | [✅] 已确认 Go 端读取 |
| `FIRECRAWL_API_KEY` | Firecrawl 爬虫 | [✅] 已确认 Go 端读取 |
| `PERPLEXITY_API_KEY` | Perplexity Search | [✅] 已确认 Go 端读取 |
| `XAI_API_KEY` | xAI/Grok | [✅] 已确认 Go 端读取 |
| `XIAOMI_API_KEY` | 小米 AI | [✅] 已确认 Go 端读取 |
| `XI_API_KEY` | ElevenLabs 备用 | [✅] 已确认 Go 端读取 |
| `Z_AI_API_KEY` | Z.ai | [ ] 缺失 (P3) |
| `OPENROUTER_API_KEY` | OpenRouter | [✅] 已确认 Go 端读取 |
| `CHUTES_CLIENT_ID/SECRET` | Chutes.ai OAuth | [ ] 缺失 (P3) |
| `CLAUDE_WEB_COOKIE` | Claude.ai web 会话 | [✅] 已确认 Go 端读取 |
| `CLAUDE_WEB_SESSION_KEY` | Claude.ai 会话 key | [✅] 已确认 Go 端读取 |
| `SHERPA_ONNX_MODEL_DIR` | 本地 ONNX 模型 | [ ] 缺失 (P3，本地模型) |
| `WHISPER_CPP_MODEL` | whisper.cpp 模型路径 | [ ] 缺失 (P3，本地模型) |

---

## 类别二：文件路径和格式约定

### 2.1 关键文件路径（双端必须一致）

| 文件路径（相对 OPENACOSMI_HOME） | 用途 | Go 覆盖状态 |
|-------------------------------|------|------------|
| `openacosmi.json` | 主配置文件（JSON5） | [✅] 已覆盖 |
| `auth-profiles.json` | auth profiles 存储 | [✅] 已覆盖但 key 格式错误（P0） |
| `device.json` | 设备 ED25519 身份 | [✅] `device_identity.go`（W-A）|
| `device-auth.json` | 设备认证令牌存储 | [✅] `device_auth_store.go`（W-A）|
| `gateway.lock` | 多实例锁文件 | [✅] `gateway_lock.go` + PID+死进程检测（W-A）|
| `sessions/{sessionKey}/` | Session 持久化目录 | [✅] `sessions/paths.go` + `sessions/store.go` 完整实现 |
| `sessions/{key}/transcript.jsonl` | 对话历史 JSONL | [✅] `infra/cost/session_cost.go:366` 使用 `transcript.jsonl` |
| `memory/` | 向量记忆存储目录 | [✅] 已覆盖 |
| `credentials/github-copilot.token.json` | Copilot API token 缓存 | [✅] `github_copilot_token.go`（W-Q）|
| `hooks/` | 用户自定义 hooks | [✅] 已覆盖 |
| `skills/` | 用户技能目录 | [✅] 已覆盖 |
| `plugins/` | 用户插件目录 | [✅] 已覆盖 |
| `logs/` | 日志目录 | [✅] `config/schema_hints_data.go:410` 引用 `$STATE_DIR/logs/` |

### 2.2 Session Key 格式约定（**P0 影响认证和消息路由**）

```
TS 格式：<channel>:<identifier>
示例：
  discord:123456789           → Discord 频道/用户
  telegram:987654321          → Telegram chat ID
  slack:T123/C456             → Slack team/channel
  imessage:user@example.com   → iMessage（email 需 lowercase）
  signal:+1234567890          → Signal E.164 格式
  whatsapp:1234567890@s.whatsapp.net → WhatsApp JID 格式
  web:session-uuid            → Web 会话
```

| 检查项 | 状态 |
|--------|------|
| Go 端 session key 解析使用相同格式 | [ ] 需验证 |
| iMessage email 地址 lowercase 规范化 | [ ] P1 缺失（W3-04） |
| Discord mention `<@123>` 展开 | [ ] P1 缺失（W3-05） |
| WhatsApp JID 格式 `@s.whatsapp.net` 后缀 | [ ] 需确认 |
| Signal E.164 规范化 | [ ] 需确认 |

### 2.3 Transcript JSONL 格式（session 历史格式）

```json
{"role": "user", "content": "...", "cachedAt": 1234567890}
{"role": "assistant", "content": [...], "cachedAt": 1234567890}
```

| 检查项 | 状态 |
|--------|------|
| `cachedAt` 字段是否在 Go 的 AgentMessage 中 | [✅] `agents/extensions/compaction_safeguard.go:60` — `CachedAt *time.Time` |
| content 数组格式（text/tool_use/tool_result blocks）与 Anthropic API 一致 | [✅] llmclient 对齐 |
| `thinkingSignature` / `thought_signature` 字段读取 | [✅] `llmclient/types.go:54,65` — 双字段 + 合并逻辑 L71 |

---

## 类别三：npm 包黑盒行为（Go 需等价实现）

| npm 包 | 功能 | Go 现状 | 等价方案 |
|--------|------|---------|---------|
| `proper-lockfile` | 跨进程文件锁（.lock 文件） | sync.RWMutex（进程内） | `github.com/gofrs/flock` 或自实现文件锁 |
| `grammy`（throttler 插件） | Telegram 速率限制（自适应） | 无 | 实现令牌桶限速 |
| `@mozilla/readability` | HTML → 可读文本提取 | 简单 HTML 标签剥离 | `github.com/go-shiori/go-readability` |
| `bonjour-service` | mDNS 服务注册/发现 | bonjour.go 占位 | `github.com/grandcat/zeroconf` |
| `sharp` | 高性能图片处理 | 无直接等价 | `github.com/disintegration/imaging` 或 libvips |
| `json5` | JSON5 解析（注释/尾逗号） | `tailscale/hujson` | [✅] 已覆盖 |
| `zod` | 运行时 schema 验证 | struct tags + validator | [✅] W-P 补全 broadcast + browser profile 跨字段验证 |
| `ws` | WebSocket 客户端/服务端 | gorilla/websocket | [✅] 已覆盖 |
| `p-queue` | Promise 串行队列 | channel + goroutine | [✅] queue_command_lane.go |
| `iso-639-1` | 语言代码验证 | 无 | 简单枚举即可 |
| `node-fetch` + `fetch-guard` | HTTP 客户端 + SSRF 防护 | net/http + security/ssrf.go | [✅] DNS pinning + 重定向逐跳检查已实现（W-B）|

---

## 类别四：全局单例 / 模块级状态

| 位置 | TS 单例 | Go 状态 | 风险 |
|------|---------|---------|------|
| `src/agents/tools/web-shared.ts` | `FETCH_CACHE = new Map()` | [✅] `webFetchCache` + sync.RWMutex + TTL 15min（W-C）| cacheOnly 已支持 |
| `src/agents/tools/web-shared.ts` | `SEARCH_CACHE = new Map()` | [✅] `webSearchCache` + sync.RWMutex + TTL 15min（W-C）| 已支持 |
| `src/agents/pi-embedded-runner/google.ts:205` | `compactionFailureEmitter = new EventEmitter()` | [✅] `runner/compaction_failure.go` Go channel 通知等价 | 已实现 |
| `src/agents/datetime/datetime.ts` | `cachedTimeFormat`（模块级）| cachedTimeFormat 全局无锁 | 竞态 M6，已记录 |
| `src/auto-reply/commands_registry.ts` | `cachedTextAliasMap` | 全局无锁 | 竞态 M7，已记录 |
| `src/infra/ssh-tunnel.ts` | SSH 连接池 | 每次新建连接 | 性能问题 |
| `src/gateway/server/ws-connection.ts` | 连接 nonce Set | [✅] `gateway/ws_server.go` `sync.Mutex` + Map，`ws_nonce_test.go` 有测试 | 已完成 |

---

## 类别五：事件总线 / 回调链

| TS EventEmitter 用法 | Go 等价 | 状态 |
|---------------------|---------|------|
| Discord `gateway.emitter.on("MESSAGE_CREATE")` | Go 回调函数注册 | [✅] 架构等价 |
| `compactionFailureEmitter.emit("fail", ...)` | [✅] `runner/compaction_failure.go:53` Go channel emit 等价 | [✅] 已实现 |
| `pi-embedded-runner` subscribe 回调链 | SubscribeParams 函数指针 | [✅] 等价实现 |
| auto-reply queue 事件 | goroutine + mutex | [✅] 等价实现 |

---

## 类别六：协议 / 消息字段命名约定

### 6.1 Gateway 协议帧（已对齐 ✅）

所有字段（type/id/method/params/ok/payload/error/event/seq/stateVersion）完全对齐。

### 6.2 工具调用 JSON Schema 字段（**部分不对齐**）

| 工具 | TS 参数名 | Go 参数名 | 状态 |
|------|----------|----------|------|
| sessions_send | `sessionKey`（camelCase） | [✅] `sessionKey` camelCase | [✅] 已修复 `sessions.go:131` |
| sessions_send | `message` | [✅] 参数名 `message` | [✅] 已修复 `sessions.go:143` |
| sessions_send | `label`, `agentId`, `timeoutSeconds` | [✅] 全部实现 | [✅] 已修复 `sessions.go:135-151` |
| web_search | `count` | [✅] `count` | [✅] 已修复 `web_fetch.go:362` |
| web_search | `country`, `search_lang`, `ui_lang`, `freshness` | [✅] 全部实现 | [✅] 已修复 `web_fetch.go:339-408` |
| web_fetch | `extractMode` | [✅] `markdown/text/raw` | [✅] 已修复 `web_fetch.go:206-235` |
| sessions_list | `sessionKey`（返回字段） | [✅] 返回 `key` 字段 | [✅] 已确认 `sessions.go:23` |

### 6.3 Auth Profile JSON 格式（**P0 格式不兼容**）

```json
// TS 生成的 auth-profiles.json
{
  "anthropic:claude-cli": { ... },
  "openai-codex:codex-cli": { ... },
  "qwen-portal:qwen-cli": { ... }
}

// Go 当前读取 key（错误！）
"claude-cli", "codex-cli", "qwen-cli"
```

| 检查项 | 状态 |
|--------|------|
| ClaudeCliProfileID 格式修复 | [✅] `"anthropic:claude-cli"`（W-A）|
| CodexCliProfileID 格式修复 | [✅] `"openai-codex:codex-cli"`（W-A）|
| QwenCliProfileID 格式修复 | [✅] `"qwen-portal:qwen-cli"`（W-A）|
| MinimaxCliProfileID 新增 | [✅] `"minimax-portal:minimax-cli"`（W-A）|
| 所有字面量引用核查 | [✅] 其余字面量均为 routing key，正确保留（W-A）|

### 6.4 Webhook 签名验证格式

| 通道 | 签名方式 | Go 状态 |
|------|---------|---------|
| Telegram | SHA-256 HMAC of payload | [✅] `hmac.Equal`（常量时间）`webhook.go:89` |
| Slack | `v0=` + HMAC-SHA256 | [✅] `hmac.Equal` `slack/monitor_provider.go:275` |
| LINE | Base64(HMAC-SHA256) | [✅] `hmac.Equal` `line/client.go:126`（错误码 403→401 待修 P2） |
| Discord | Ed25519 签名 | [❌] 未找到 Ed25519 签名验证（P1）|
| WhatsApp | HMAC-SHA256 | [❌] 未找到 HMAC-SHA256 验证（P1）|

---

## 类别七：错误处理约定

| TS 自定义 Error | 功能 | Go 等价 | 状态 |
|----------------|------|---------|------|
| `SsrfBlockedError` | SSRF 被阻断 | `fmt.Errorf("ssrf: ...")` | [✅] 语义等价 |
| `FailoverError` | 含 reason/status/provider/model | models.FailoverError struct | [✅] 已移植 |
| auth cooldown/disabledUntil 错误 | 认证冷却 | ProfileUsageStats.DisabledReason | [✅] 已移植 |
| `GatewayLockError` | 多实例锁冲突 | [✅] `infra/gateway_lock.go:31-44` 完整 struct + `Error()`/`Unwrap()` | [✅] 已实现 |
| infra `errors.ts` 中的结构化错误类型 | 基础设施错误 | 无专门类型 | [ ] P1 缺失 |

---

## 类别八：隐式启动顺序 / 生命周期依赖

TS 中的 gateway 启动顺序（`server.impl.ts` 中明确定义）：

```
1. 加载配置
2. 启动 shell env 探测
3. 启动 mDNS/Bonjour 注册
4. 启动 cron 调度器（除非 OPENACOSMI_SKIP_CRON）
5. 启动各通道连接（除非 OPENACOSMI_SKIP_CHANNELS）
6. 启动浏览器控制服务（除非 OPENACOSMI_SKIP_BROWSER_CONTROL_SERVER）
7. 启动 Canvas Host（除非 OPENACOSMI_SKIP_CANVAS_HOST）
8. 开始接受 WebSocket 连接
```

| 检查项 | Go 状态 |
|--------|---------|
| 启动顺序是否与 TS 一致 | [ ] 需核查 backend/cmd/acosmi/ |
| 各 SKIP_* 环境变量是否控制对应功能 | [✅] `config/features.go` 全部实现（文档已同步）|
| hello-ok 响应中 canvasHostUrl 字段注入 | [✅] `gateway/protocol.go:186` — `CanvasHostURL` 字段 |

---

## 类别九：平台特定隐藏依赖

| 平台 | TS 实现 | Go 状态 |
|------|---------|---------|
| macOS | launchd.ts + macos/（3文件）| launchd_darwin.go ✅ |
| Linux | systemd.ts + systemd-unit + systemd-linger | systemd_linux.go（部分）[ ] |
| Windows | schtasks.ts | schtasks_windows.go ✅ |
| macOS clipboard | `pbcopy`/`pbpaste` | 无（clipboard.ts 未移植）|
| Linux clipboard | `xclip`/`xdotool` | 无 |
| Homebrew 路径 | brew.ts 解析 prefix | 无（brew.ts 未移植）|
| WSL 检测 | wsl.ts | 无 |

---

## 修复优先级汇总

### 必须在阶段一/二修复（影响核心功能）

| # | 隐藏依赖 | 优先级 |
|---|─────────|--------|
| 1 | ~~auth profile ID 格式（`provider:name`）~~ | ~~**P0**~~ ✅ W-A 已修复 |
| 2 | session key 格式约定（各通道规范化） | **P0** |
| 3 | ~~`cachedAt` 字段添加到 transcript JSONL~~ | ~~**P0**~~ ✅ `compaction_safeguard.go:60` |
| 4 | ~~SSRF 重定向追踪（proper-lockfile → flock）~~ | ~~**P0**~~ ✅ `security/ssrf.go` DNS pinning + 重定向检查 |
| 5 | ~~`OPENACOSMI_SKIP_CRON/CHANNELS` 等 feature flag 环境变量~~ | ~~P1~~ ✅ `config/features.go` 全部实现 |
| 6 | ~~`OPENACOSMI_SESSION_CACHE_TTL_MS` 动态 TTL~~ | ~~P1~~ ✅ W-P 已确认实现 |
| 7 | ~~web-fetch/search 缓存层（Map 单例）~~ | ~~P1~~ ✅ `webFetchCache` + `webSearchCache` sync.RWMutex |
| 8 | ~~`compactionFailureEmitter` Go 等价通知~~ | ~~P1~~ ✅ `runner/compaction_failure.go` channel 等价 |
| 9 | ~~Telegram webhook 等时比较~~ | ~~P1~~ ✅ `hmac.Equal` 常量时间比较 |
| 10 | bonjour-service 真实 mDNS 注册 | P1 |

### 中期修复（影响完整性）

| # | 隐藏依赖 | 优先级 |
|---|---------|--------|
| 11 | `@mozilla/readability` → go-readability | P1 |
| 12 | `grammy` throttler → 令牌桶限速 | P1 |
| 13 | ~~所有 OPENACOSMI_SKIP_* 环境变量支持~~ | ~~P1~~ ✅ `config/features.go` 全部实现 |
| 14 | 第三方 API Key 变量（~~BRAVE/FIRECRAWL/XAI~~ ✅ 剩余 Z_AI/CHUTES P3）| ~~P1~~ ✅ 大部分已实现 |
| 15 | webhook 签名等时比较（~~Telegram/Slack/LINE~~ ✅ 剩余 Discord/WhatsApp P1）| P1 |
| 16 | 跨进程文件锁（proper-lockfile） | P1 (gateway_lock已实现，但agents模块Auth/Session仍用RWMutex) |
| 17 | ~~gateway hello-ok canvasHostUrl 注入~~ | ~~P2~~ ✅ `gateway/protocol.go:186` |
| 18 | ~~GatewayLockError 结构化错误类型~~ | ~~P2~~ ✅ `infra/gateway_lock.go:31-44` |
| 19 | 平台 clipboard 实现 | P2 |

---

*最后更新：2026-02-20（隐藏依赖文档补全 — 46 项 `[ ]` → `[✅]` 同步完成）*
*关联文件：[fix-plan-master.md](./fix-plan-master.md) | [global-audit-index.md](./global-audit-index.md) | [global-audit-hidden-deps.md](./global-audit-hidden-deps.md)*
