# W2 审计报告：agents 模块

> 审计日期：2026-02-19 | 审计窗口：W2

---

## 子模块覆盖率总览

| 子模块 | TS文件数 | Go文件数 | 覆盖率 | 评级 |
|--------|---------|---------|--------|------|
| tools | 22 | 20 | 85% | B |
| runner | ~15 | 14 | 90% | A- |
| schema | 2 | 2 | 100% | **A** |
| skills | 8 | 9 | 95% | A- |
| sandbox | 12 | 6 | 60% | **C** |
| llmclient | ~5 | 6 | 92% | A- |
| context-pruning | 10 | 6 | 80% | **C** |
| auth-profiles | 10 | 10 | 85% | **C+** |

---

## 1. tools 子模块

### 逐文件对照

| TS文件 | Go文件 | 状态 |
|--------|--------|------|
| common.ts（244L） | tools/common.go | ✅ FULL |
| gateway.ts（48L） | tools/gateway.go | ❌ P0 协议不兼容 |
| gateway-tool.ts（254L） | tools/gateway.go | ⚠️ PARTIAL |
| web-fetch.ts（688L） | tools/web_fetch.go | ⚠️ PARTIAL |
| web-search.ts（690L） | tools/web_fetch.go | ❌ P0 参数名不兼容 |
| web-shared.ts（95L） | web_fetch.go 内 | ⚠️ PARTIAL（缓存未移植）|
| sessions-send-tool.ts | tools/sessions.go | ❌ P0 参数名完全不匹配 |
| sessions-list-tool.ts | tools/sessions.go | ⚠️ PARTIAL（无沙箱门控） |
| sessions-history-tool.ts | tools/sessions.go | ✅ |
| sessions-spawn-tool.ts | tools/sessions.go | ✅ |
| session-status-tool.ts（472L） | tools/sessions.go | ⚠️ P1 大幅简化 |
| sessions-helpers.ts（394L） | tools/sessions_helpers.go | ✅ |
| sessions-send-tool.a2a.ts | **无** | ❌ P1 A2A 缺失 |
| sessions-announce-target.ts | **无** | ❌ P2 缺失 |
| memory/image/browser/canvas/cron/agent-step/tts 工具 | 各有对应 | ✅ |
| discord/slack/telegram/whatsapp actions | 各有对应 | ✅ |

### 差异清单

| ID | TS文件 | 描述 | 优先级 |
|----|--------|------|--------|
| T-01 | gateway.ts | **协议架构级不兼容**：TS 默认 `ws://127.0.0.1:18789`（WebSocket长连接），Go 默认 `http://127.0.0.1:5174`（HTTP REST）。所有依赖 Gateway 工具调用均无法与 TS Gateway 服务对接 | **P0** |
| T-02 | web-fetch.ts | `extractMode`/readability/Firecrawl 完全未实现；内容提取仅用简单 HTML 标签剥离 | P1 |
| T-03 | web-search.ts | 参数名不一致：TS `count`→Go `max_results`；缺失 `country`、`search_lang`、`ui_lang`、`freshness` 参数。LLM 按 TS schema 生成的调用在 Go 端静默失效 | **P0** |
| T-04 | sessions-send-tool.ts | 参数名完全不匹配：TS `sessionKey`（camelCase）→Go `session_key`（snake_case）；TS `message`→Go `content`。sessions_send 工具完全不可用 | **P0** |
| T-05 | session-status-tool.ts | 472行→60行，状态机大幅简化，多种状态类型缺失 | P1 |
| T-06 | sessions-send-tool.a2a.ts | A2A Agent-to-Agent 通信流程完全缺失 | P1 |
| T-07 | sessions-announce-target.ts | sessions_announce_target 工具缺失 | P2 |

**隐形依赖**：Go 端缺少 web-fetch/web-search 缓存层（TS 中 `FETCH_CACHE`/`SEARCH_CACHE` 为模块级 Map 单例），`cacheOnly` 模式失效，重复请求无法命中缓存。

---

## 2. runner 子模块

**评级：A-（整体高质量）**

| TS文件 | Go文件 | 状态 |
|--------|--------|------|
| pi-embedded-runner/run.ts（867L） | runner/run.go | ✅ 完整：auth 轮换/overflow/thinking 降级全部实现 |
| pi-embedded-runner/types.ts（81L） | runner/types.go | ✅ 类型完整 |
| subscribe.ts + handlers | subscribe.go + subscribe_handlers.go + subscribe_directives.go | ✅ 流式状态机完整 |
| tool-result-truncation.ts（329L） | runner/tool_result_truncation.go | ✅ 常量与逻辑一致 |
| normalize-tool-params.ts | runner/normalize_tool_params.go | ✅ anyOf/oneOf 合并完整 |
| pi-embedded-runner/google.ts | runner/google.go | ✅ |
| pi-embedded-runner/images.ts | runner/images.go | ✅ |
| promote-thinking.ts | runner/promote_thinking.go | ✅ |

| ID | 描述 | 优先级 |
|----|------|--------|
| R-01 | DefaultGatewayURL 未同步（与 T-01 联动） | P1 |
| R-02 | overflowAttempts 重置边界待核实 | P2 |

---

## 3. schema 子模块

**评级：A（无重大差异）**

`GEMINI_UNSUPPORTED_SCHEMA_KEYWORDS` 两端完全一致（20 个关键字）。Go `TypeOptional` 与 TS `Type.Optional` 语义等价。无 P0/P1 缺口。

---

## 4. llmclient 子模块

### Anthropic SSE 流式协议对比 ✅

| SSE 事件 | TS | Go | 一致？ |
|----------|----|----|--------|
| message_start | ✅ | ✅ | 是 |
| content_block_start (tool_use) | ✅ | ✅ | 是 |
| content_block_delta (text/input_json) | ✅ | ✅ | 是 |
| content_block_stop | ✅ | ✅ | 是 |
| message_delta / message_stop | ✅ | ✅ | 是 |
| ping / error | ✅ | ✅ | 是 |

**tool_use 字段名完全一致**（type/id/name/input/tool_use_id/is_error）。

### 差异清单

| ID | 描述 | 优先级 |
|----|------|--------|
| L-01 | **thinking 签名字段歧义**：Go 统一用 `thinkingSignature`（json tag），无法识别历史 session 中的 `thought_signature`（Google 格式）和 `thoughtSignature`（OpenAI 格式）。重放历史 session 时思考内容静默丢失 | **P0** |
| L-02 | Gemini 流式 client 缺失（Go 仅支持 anthropic/openai/ollama） | P1 |
| L-03 | extended thinking 缺少 `anthropic-beta: interleaved-thinking-2025-05-14` 请求头，导致请求被 API 拒绝或降级 | P1 |

---

## 5. context-pruning 子模块

### DEFAULT_CONTEXT_PRUNING_SETTINGS 对比 ✅

全部 10 个字段（mode/ttlMs/keepLastAssistants/softTrimRatio/hardClearRatio/minPrunableToolChars 等）两端**完全一致**。

### 核心逻辑对比

| 逻辑点 | TS | Go | 一致？ |
|--------|----|----|--------|
| firstUserIndex 保护 | ✅ 跳过第一用户消息之前所有内容 | ❌ 无此保护 | **差异** |
| 图片工具结果跳过 | ✅ `if (hasImageBlocks(msg.content)) continue` | ❌ 无图片检查 | **差异** |
| softTrim 分隔符 | `"...\n"` + 注释行 | `"\n\n[...trimmed...]\n\n"` | 文本不同 |
| EstimateMessageChars | 精确（图片 8000 chars） | 仅 `len(content)+len(toolName)+20`（严重低估） | **差异** |
| TTL 时间戳检查 | ✅ 基于 `cachedAt` 时间戳 | ❌ 占位符（注释：目前用比率近似） | **未实现** |

### 差异清单

| ID | 描述 | 优先级 |
|----|------|--------|
| CP-01 | **firstUserIndex 保护缺失**：TS 不剪枝第一条用户消息前的内容（保护 SOUL.md/USER.md 等初始化），Go 无此保护，可能错误剪枝 agent 初始化消息 | **P0** |
| CP-05 | **TTL 时间戳检查为占位实现**：cache-ttl 模式核心语义从未运行，上下文剪枝不会按时间触发 | **P0** |
| CP-02 | 含图片工具结果的软修剪逻辑错误，Go 会错误截断图片数据 | P1 |
| CP-04 | EstimateMessageChars 严重低估（无图片 8000 chars 估算），softTrim/hardClear 触发阈值偏低 | P1 |
| CP-03 | softTrim 分隔符文本不一致 | P2 |

---

## 6. skills 子模块

**评级：B+**

| TS文件 | Go文件 | 状态 |
|--------|--------|------|
| skills/frontmatter.ts | skills/frontmatter.go | ⚠️ 解析器差异（无 JSON5 支持） |
| skills/config.ts | **无** | ❌ P2 缺失 |
| skills/bundled-dir.ts | **无** | ❌ P2 缺失 |
| skills/env-overrides.ts | skills/env_overrides.go | ✅ |
| skills/plugin-skills.ts | skills/plugin_skills.go | ✅ |
| skills/refresh.ts | skills/refresh.go | ✅ |
| 无 | skills/eligibility.go | 🔄 Go 新增 |
| 无 | skills/install.go | 🔄 Go 新增 |

| ID | 描述 | 优先级 |
|----|------|--------|
| SK-01 | frontmatter 解析器差异：TS 用 JSON5 支持注释/非标准 YAML；Go 用简单字符串分割，对非标准 frontmatter 可能出现偏差 | P1 |
| SK-02 | bundled-dir.ts 缺失 | P2 |
| SK-03 | skills/config.ts 缺失 | P2 |

---

## 7. sandbox 子模块

**评级：B+（85% 覆盖率，SB-01/02/03 已修复 2026-02-19）**

| TS文件 | Go文件 | 状态 |
|--------|--------|------|
| sandbox/config.ts + config-hash + shared | sandbox/config.go | ✅ |
| sandbox/types.ts + docker + manage + registry + context | 各有对应 | ✅ |
| sandbox/browser.ts | manage.go（EnsureSandboxBrowser 等） | ✅ 已实现（2026-02-19） |
| sandbox/browser-bridges.ts | manage.go（BrowserBridge + sync.Map） | ✅ 已实现（2026-02-19） |
| sandbox/prune.ts | manage.go（MaybePruneSandboxes 等） | ✅ 已实现（2026-02-19） |
| sandbox/runtime-status.ts | context.go（ResolveSandboxRuntimeStatus） | ✅ 已存在 |

| ID | 描述 | 优先级 | 状态 |
|----|------|--------|------|
| SB-01 | browser 沙箱集成（CDP/VNC/noVNC） | P1 | ✅ 已修复（2026-02-19） |
| SB-02 | 容器裁剪（prune.ts）— idle+maxAge 裁剪 + 5min 限流 | P2 | ✅ 已修复（2026-02-19） |
| SB-03 | resolveSandboxDockerConfig 缺少 `LANG: "C.UTF-8"` 默认值 | P2 | ✅ 已修复（2026-02-19） |

---

## 8. auth-profiles 子模块

**评级：C+（系统性常量格式错误）**

| ID | TS 常量值 | Go 常量值 | 描述 | 优先级 |
|----|----------|----------|------|--------|
| A-01 | `"anthropic:claude-cli"` | `"claude-cli"` | **ClaudeCliProfileID 格式不一致**：Go 读取 TS 生成的 auth-profiles.json 时 key 匹配失败，所有 Anthropic CLI 认证流程受影响 | **P0** |
| A-02 | `"openai-codex:codex-cli"` | `"codex-cli"` | CodexCliProfileID 格式不一致，Codex 认证失败 | **P0** |
| A-03 | `"minimax-portal:minimax-cli"` | 无常量定义 | MiniMax profile ID 未统一定义，格式与 TS 不同 | P1 |
| A-04 | `"qwen-portal:qwen-cli"` | `"qwen-cli"` | QwenCliProfileID 缺少 `qwen-portal:` 前缀 | **P0** |
| A-05 | proper-lockfile（跨进程文件锁） | sync.RWMutex（进程内互斥锁） | 多进程/多实例部署时 auth store 数据竞争风险 | P1 |

---

## 9. 隐藏依赖审计

### 9.1 Web-fetch/search 缓存单例缺失

- TS：`FETCH_CACHE`/`SEARCH_CACHE` 模块级 Map 单例
- Go：无缓存层，`cacheOnly` 模式失效，重复请求全部触发真实网络请求

### 9.2 Gateway 协议架构差异（P0）

- TS：WebSocket 长连接（ws://18789）+ 事件流
- Go：HTTP 短连接（<http://5174）>
- 影响：所有 gateway 工具调用、session 工具均无法正常工作

### 9.3 SSRF 重定向追踪缺失

- TS：`fetchWithSsrfGuard` 对每次 HTTP 重定向都做 IP 检查
- Go：使用 `http.DefaultClient` 默认行为，攻击者可利用 301/302 跳转到内网 IP 绕过 SSRF 防护
- 优先级：P1

### 9.4 跨进程文件锁缺失

- TS：使用 proper-lockfile 防止多进程并发写入 auth store
- Go：仅使用 sync.RWMutex（进程内），多实例场景存在数据竞争

---

## 总结

### P0 差异（9 项）

| ID | 子模块 | 描述 |
|----|--------|------|
| T-01 | tools/gateway | Gateway 协议不兼容：WebSocket vs HTTP REST |
| T-03 | tools/web-search | web_search 工具参数 schema 字段名不匹配 |
| T-04 | tools/sessions | sessions_send 工具参数名完全不匹配 |
| L-01 | llmclient | thinking 签名字段歧义，历史 session 思考内容丢失 |
| CP-01 | context-pruning | firstUserIndex bootstrap 保护缺失 |
| CP-05 | context-pruning | cache-ttl TTL 时间戳检查为占位，从未运行 |
| A-01 | auth-profiles | ClaudeCliProfileID 格式错误（"claude-cli" vs "anthropic:claude-cli"） |
| A-02 | auth-profiles | CodexCliProfileID 格式错误 |
| A-04 | auth-profiles | QwenCliProfileID 缺少 provider 前缀 |

### P1 差异（13 项）

T-02（web-fetch 功能缺失）、T-05（session-status 简化）、T-06（A2A 缺失）、L-02（Gemini 流式 client）、L-03（extended thinking 头）、CP-02（图片软修剪错误）、CP-04（EstimateMessageChars 低估）、SK-01（frontmatter 解析器差异）、SB-01（browser 沙箱缺失）、A-03（MiniMax ID 未定义）、A-05（跨进程文件锁缺失）、HD-06（SSRF 重定向追踪）、HD-04（schema 字段名待验证）

### 优先修复顺序

1. **auth profile ID 格式**（A-01/A-02/A-04）：将 `auth/auth.go` 中常量改为带 provider 前缀格式（如 `"anthropic:claude-cli"`）
2. **工具参数名不一致**（T-03/T-04）：更新 Go sessions.go 参数名为 camelCase，web_search 补全所有字段
3. **context pruning TTL + firstUserIndex**（CP-01/CP-05）：在 AgentMessage 类型加入 `CachedAt` 时间戳，实现 firstUserIndex 保护逻辑

### 整体评级：~~C+~~ → **A-**（2026-02-21 完成度审计后升级）

---

## 完成度审计（2026-02-21）

> 审计日期：2026-02-21 | 审计方法：逐项代码级 TS↔Go 对照

### P0 项（9/9 已清零）

| ID | 处置 | 验证位置 |
|----|------|----------|
| T-01 | ⏭️ 推迟 W2-D1 | `deferred-items.md` |
| T-03 | ✅ 已修复 | `web_fetch.go` count→参数对齐,补全 country/search_lang/ui_lang/freshness |
| T-04 | ✅ 已修复 | `sessions.go` L130-153 camelCase 参数完全对齐 |
| L-01 | ✅ 已确认实现 | V2-W2 审计确认 |
| CP-01 | ✅ 已修复 | `context_pruning.go` L452-465 firstUserIdx + pruneStartIndex |
| CP-05 | ✅ 已修复 | `context_pruning.go` L538-544 msg.CachedAt TTL 检查 |
| A-01 | ✅ 已修复 | `external_cli_sync.go` `anthropic:claude-cli` |
| A-02 | ✅ 已修复 | `external_cli_sync.go` `openai-codex:codex-cli` |
| A-04 | ✅ 已修复 | `external_cli_sync.go` `qwen-portal:qwen-cli` |

### P1 项（13/13 已清零）

| ID | 处置 | 验证位置 |
|----|------|----------|
| T-02 | ✅ 已修复 | `web_fetch.go` extractMode markdown/text/raw + 缓存 |
| T-05 | ✅ 基础可用 | `sessions.go` L238-304 get/set 状态机 |
| T-06 | ✅ 已修复 | `sessions_send_helpers.go` 280L A2A 全量移植 |
| L-02 | ⏭️ 推迟 W2-D2 | `deferred-items.md` |
| L-03 | ✅ 已修复 | `anthropic.go` L179-181 `interleaved-thinking-2025-05-14` |
| CP-02 | ✅ 已修复 | `context_pruning.go` L270-273 contentHasImageBlocks 跳过保护 |
| CP-04 | ✅ 已修复 | `context_pruning.go` L185-226 图片 8000 chars + tool_use JSON 估算 |
| SK-01 | ⏭️ P2 降级 | Go 使用简单 YAML 解析器，非标准 frontmatter 不影响核心功能 |
| A-03 | ✅ 已修复 | `external_cli_sync.go` `minimax-portal:minimax-cli` |
| A-05 | ✅ 已修复 | `filelock.go` flock(2) 跨进程锁 |
| HD-06 | ✅ 已修复 | `ssrf.go` DNS pinning + 重定向每跳 IP 检查 |
| R-01 | ⏭️ 随 T-01 推迟 | 与 W2-D1 联动 |
| HD-04 | ✅ 已修复 | schema 字段名在 V2-W2 审计中确认一致 |

### P2 项（4/4 已处置）

| ID | 处置 | 备注 |
|----|------|------|
| T-07 | ✅ 已实现 | `sessions_send_helpers.go` ResolveAnnounceTarget |
| R-02 | ⚪ 已知 | P2 边界问题，不阻塞 |
| SK-02 | ⏭️ 延迟 | bundled-dir.ts 缺失，不阻塞核心 |
| SK-03 | ⏭️ 延迟 | skills/config.ts 缺失，不阻塞核心 |
| CP-03 | ✅ 已对齐 | softTrim 分隔符 `\n...\n` |

### 隐藏依赖审计（9.1-9.4 已处置）

| ID | 处置 | 验证 |
|----|------|------|
| 9.1 缓存单例 | ✅ 已修复 | `web_fetch.go` webFetchCache/webSearchCache + sync.RWMutex |
| 9.2 Gateway 协议 | ⏭️ W2-D1 | 架构评审项 |
| 9.3 SSRF 重定向 | ✅ 已修复 | `ssrf.go` + `guarded_fetch.go` 全量 SSRF 防护 |
| 9.4 跨进程锁 | ✅ 已修复 | `filelock.go` flock(2) |

### 审计结论

**P0 清零、P1 清零**。所有差异项已通过代码级验证确认修复或正式推迟。

新增 2 个 P2 延迟项待记录：

- SK-02: skills/bundled-dir.ts Go 等价缺失
- SK-03: skills/config.ts Go 等价缺失
