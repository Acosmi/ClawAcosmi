# ⚠️ 生产前全局审计主索引（2026-02-19 深度审计版）

> **本文件为 2026-02-19 新一轮深度审计结果，覆盖所有模块，发现 43 项 P0 缺口和 7 项高危 Bug。**
> 审计范围：TS 1649 文件 ↔ Go 856 文件，并行 6 窗口 + 代码健康度审计
>
> **⛔ 生产上线结论：不满足条件（综合评级 D+）**

---

# 新审计结果（2026-02-19）

## 各模块审计状态

| 模块 | TS 文件 | Go 文件 | 覆盖率 | P0 | P1 | 评级 | 报告 |
|------|---------|---------|--------|----|----|------|------|
| gateway | 133 | 67 | 50% | 2 | 10 | **C** | [W1](./global-audit-W1.md) |
| security | 8 | 9 | 113% | 2 | 2 | **B** | [W1](./global-audit-W1.md) |
| config | 89 | 31 | 35% | 0 | 3 | **C-** | [W1](./global-audit-W1.md) |
| agents（全） | 233 | 144 | 62% | 9 | 13 | **C+** | [W2](./global-audit-W2.md) |
| channels（全） | 251 | 209 | 83% | 3 | 7 | **B+** | [W3](./global-audit-W3.md) |
| auto-reply | 121 | 90 | 74% | 5 | 5 | **C** | [W4](./global-audit-W4.md) |
| cron | 42 | 19 | 86% | 0 | 2 | **A** | [W4](./global-audit-W4.md) |
| daemon | 30 | 22 | 73% | 2 | 1 | **C** | [W4](./global-audit-W4.md) |
| hooks | 40 | 18 | 68% | 2 | 3 | **B** | [W4](./global-audit-W4.md) |
| infra | 120 | 43 | 36% | 4 | 10 | ~~D~~ → **B** | [W5](./global-audit-W5.md) |
| cli/commands | 312 | 30 | 9.6% | 3 | 5 | ~~D~~ → **C+** | [W5](./global-audit-W5.md) |
| media-understanding | 25 | 26 | 104% | 0 | 0 | **A** | [W5](./global-audit-W5.md) |
| memory | 28 | 23 | 82% | 0 | 4 | ~~B+~~ → **A-** | [W5](./global-audit-W5.md) |
| tui | 24 | 19+6test | 85% | 0 | 0 | **A** | [W5](./global-audit-W5.md) + [架构](../gouji/tui.md) + [任务](./phase5-tui-task.md) |
| browser | 52 | 12 | 23% | 1 | 4 | **C** | [W5](./global-audit-W5.md) |
| 代码健康 | — | — | — | 7高危Bug | 9中危 | **D** | [health](./global-audit-health.md) |

## P0 差异（43 项）分类

### 编译/启动阻断

1. ~~**go.mod `go 1.25.7` 不存在**~~ → **误判已撤销**。Go 1.25.7 是正式版，项目版本号正确，无需修改。
2. SQLite 驱动未声明 → memory 模块 panic（manager.go:63）
3. daemon/systemd-unit.ts 未移植 → Linux buildSystemdUnit() 调用崩溃

### 安全关键（7 项）

4. SSRF DNS pinning 完全缺失
5. TLS 自签名证书生成（infra/tls）缺失
6. Gateway TLS 完整运行时缺失
7. exec-safety.ts 命令执行安全策略缺失
8. WebSocket 客户端并发写竞态（H5）
9. control_ui_assets.go runWithTimeout 数据竞争（H1）
10. infra/net/ssrf.ts 重定向追踪缺失

### 身份/认证失效（7 项）

11. ClaudeCliProfileID 格式错误（"claude-cli" vs "anthropic:claude-cli"）
12. CodexCliProfileID 格式错误
13. QwenCliProfileID 格式错误
14. device-identity.ts 未移植（设备无法注册）
15. device-auth-store.ts 未移植（认证令牌无存储）
16. auth-choice.*.ts（14 文件）完全缺失
17. ~~OAuth 流程完全缺失~~ ✅ W-Q 实现 GitHub Copilot Device Flow + Qwen OAuth refresh

### 核心消息管线（9 项）

18. Gateway 协议不兼容（WebSocket vs HTTP）
19. web_search 工具参数名不兼容
20. sessions_send 参数名完全不匹配
21. outbound 投递链（4 文件）完全缺失 → 消息无法发出
22. auto-reply directive-handling 三文件缺失
23. auto-reply stage-sandbox-media 缺失（安全边界）
24. auto-reply untrusted-context 缺失
25. auto-reply streaming-directives 缺失
26. exec/directive 语义差异

### Agent 核心逻辑（5 项）

27. context-pruning firstUserIndex 保护缺失
28. cache-ttl TTL 时间戳检查为占位
29. thinking 签名字段多格式支持缺失（thought_signature 等）
30. bash exec 输出恒返回 nil（H6）
31. ssh_tunnel.go stderrPipe nil panic（H3）

### 基础设施（9 项）

32. gateway-lock.ts 缺失（多实例冲突）
33. home-dir.ts 核心路径解析缺失
34. Setup/onboarding 流程完全未实现
35. ~~config/sessions/metadata.ts 会话元数据衍生缺失~~ ✅ W-P 补全
36. /v1/responses OpenResponses 完整实现缺失
37. daemon/systemd-linger.ts 缺失（登出后停止运行）
38. hooks/session-memory 骨架未实现
39. hooks/llm-slug-generator 缺失
40. browser CDP relay goroutine 阻塞（H2）

### 功能缺失（3 项）

41. browser/pw-ai.ts AI 浏览自动化缺失
42. LINE 通道核心管线（3 文件）缺失
43. CDP relay goroutine 永久阻塞

## 修复优先级路线图

**第一阶段（立即）**：go.mod 版本修复 → SQLite 驱动 → WS 并发写 → systemd-unit 移植

**第二阶段（1-2周）**：auth profile ID 格式 → TLS 体系 → SSRF DNS pinning → device-identity/auth-store

**第三阶段（2-4周）**：Gateway 协议对齐 → 工具参数名对齐 → outbound 投递链 → directive 链 → exec 输出捕获

**第四阶段（4-6周）**：context-pruning → thinking 多格式 → ~~config/sessions 子模块~~ ✅ → session-memory → LINE 通道

**第五阶段（6-12周）**：~~Setup/OAuth~~ ✅ → Playwright 层 → TUI → Gemini client → infra 50+ 缺失项

## 报告文件索引

| 文件 | 模块 |
|------|------|
| [global-audit-W1.md](./global-audit-W1.md) | gateway + security + config |
| [global-audit-W2.md](./global-audit-W2.md) | agents 全模块 |
| [global-audit-W3.md](./global-audit-W3.md) | channels 全通道 |
| [global-audit-W4.md](./global-audit-W4.md) | auto-reply + cron + daemon + hooks |
| [global-audit-W5.md](./global-audit-W5.md) | infra + media + memory + cli + tui + browser |
| [global-audit-health.md](./global-audit-health.md) | 代码健康度 + Bug 清单 |
| [global-audit-tui.md](./global-audit-tui.md) | TUI 专项审计（22 项差异 + 7 类隐藏依赖） |
| [global-audit-tui-deps.md](./global-audit-tui-deps.md) | TUI 14 个外部依赖颗粒度审计 |
| [phase5-tui-project.md](./phase5-tui-project.md) | TUI 独立项目实施方案（15 文件 / ~3,740L / 7 窗口） |

---

# 旧审计历史（2026-02-19 之前）

> 以下为旧版审计摘要，已被上方新审计覆盖

---

## 审计结果总览

| 窗口 | 模块 | TS 文件/行 | Go 文件/行 | 行覆盖率 | 评级 | 差异 |
|------|------|-----------|-----------|---------|------|------|
| W1 | linkparse, markdown, tts, utils, process, types | 37/4,807 | 49+/7,063 | ~100% | **A** | 0 |
| W2 | security, routing, sessions, outbound, nodehost | 38/9,884 | 34+/10,154 | ~95% | **A-** | 1 P3 |
| W5 | config | 80/14,329 | 58/10,256 | 71.5% | **A-** | 0 |
| W6 | infra | 99/18,428 | 43/6,781 | 36.8%* | **B** | 2 P3 |
| W3 | memory, browser, canvas, daemon, cron | 123/25,333 | 60/11,160 | ~55%* | **B** | 1 P2 +2 P3 |
| W4 | hooks, plugins, acp, cli+cmds, tui | 397/64,717 | 57/12,232 | ~19%* | **B-** | 1 P3 |
| W7 | gateway | 133/26,457 | 67/20,819 | 78.7% | **A-** | 0 |
| W8 | agents | 233/46,991 | 144/34,957 | 74.4% | **B+** | 1 P3 |
| W9 | autoreply | 121/22,028 | 90/15,668 | 71.1% | **A-** | 0 |
| W10 | channels (×8) | ~337/42,028 | 209/34,545 | 82.2% | **A-** | 0 |

> \* 行覆盖率低不代表功能缺失 — infra 有 ~30 个 Node.js 特有工具文件、cli+cmds 使用 Cobra vs Commander.js 架构差异、browser 使用 CDP 替代 Playwright。

---

## 差异汇总（按优先级）

### 🔴 P0 关键差异：**0 项**

### 🟡 P1 重要差异：**0 项**

### 🟠 P1 重要差异：**~20 项**（深度审计新发现）

| ID | 模块 | 描述 |
|----|------|------|
| DEEP-1 | browser | 标签管理端点缺失（openTab/closeTab/focusTab/tabs/tabAction）|
| DEEP-2 | browser | Cookie/Storage 端点缺失（cookies/cookiesClear/storageGet/Set/Clear）|
| DEEP-3 | browser | 网络观测端点缺失（requests/responseBody/consoleMessages/pageErrors）|
| DEEP-4 | browser | 快照/调试端点缺失（snapshot/highlight/traceStart/traceStop）|

> 详细函数级清单见 [global-audit-deep-discrepancy.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-deep-discrepancy.md)

### 🟢 P2 可延迟差异：**~16 项**

| ID | 模块 | 描述 |
|----|------|------|
| DEEP-5 | browser | 设备模拟（setDevice/setGeolocation/setLocale/setTimezone 等 8 个）|
| DEEP-6 | browser | PDF/下载（pdfSave/download/waitForDownload）|
| DEEP-7 | browser | Profile 管理（createProfile/deleteProfile/resetProfile/profiles）|
| DEEP-8 | cli | onboard 交互式向导 (~18 TS 文件) |
| DEEP-9 | cli | auth-choice 认证选择流程 (~16 TS 文件) |

### ⚪ P3 代码风格/结构差异：**9 项**

| ID | 模块 | 描述 |
|----|------|------|
| W2-1 | outbound | directory-cache Map+TTL 简化为 DI 注入 |
| W56-1 | infra | update-runner/update-check 自更新机制未移植（Go 通常由包管理器更新） |
| W56-2 | infra | channel-activity/channel-summary 位置未确认 |
| W34-2 | browser | Playwright 高级 API → CDP 直接调用（设计决策） |
| W34-3 | cli | onboard wizard UX 细节（Commander→Cobra 架构差异） |
| W34-4 | tui | Ink.js 24 组件 → Bubble Tea 6 文件（框架差异） |
| W8-1 | agents | `stream/` 空包可清理 |
| — | config | Zod runtime → Go struct+tag（设计决策） |
| — | global | Ollama + i18n 延迟 Phase 13/14（已在 deferred-items.md） |

---

## 隐藏依赖审计汇总

| # | 类别 | 全模块结果 |
|---|------|-----------|
| 1 | npm 包黑盒行为 | ✅ 全部 Go 等价：Zod→validator, Playwright→CDP, Ink→BubbleTea, baileys→whatsmeow |
| 2 | 全局状态/单例 | ✅ 全部使用 sync.Map/sync.RWMutex/channel 替代 |
| 3 | 事件总线/回调 | ✅ Go channel + DI callback 替代 EventEmitter |
| 4 | 环境变量依赖 | ✅ os.Getenv 等价 |
| 5 | 文件系统约定 | ✅ 路径/锁/临时文件全覆盖 |
| 6 | 协议/消息格式 | ✅ WS/CDP/HTTP/SDK 全对齐 |
| 7 | 错误处理约定 | ✅ Go error wrapping 模式 |

---

## 审计报告索引

| 报告 | 路径 |
|------|------|
| W1 小型模块 | [global-audit-w1-small.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-w1-small.md) |
| W2 中型模块 A | [global-audit-w2-medium-a.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-w2-medium-a.md) |
| W5+W6 config+infra | [global-audit-w5w6-config-infra.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-w5w6-config-infra.md) |
| W3+W4 中型模块 B+C | [global-audit-w3w4-medium-bc.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-w3w4-medium-bc.md) |
| W7-W10 大型模块 | [global-audit-w7w10-large.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-w7w10-large.md) |
| 深度差异审计 | [global-audit-deep-discrepancy.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-deep-discrepancy.md) |
| TUI 专项审计 | [global-audit-tui.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-tui.md) |
| TUI 依赖颗粒度审计 | [global-audit-tui-deps.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-tui-deps.md) |
| TUI 独立项目方案 | [phase5-tui-project.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5-tui-project.md) |
| 隐藏依赖补全度审计 | [global-audit-hidden-deps.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-hidden-deps.md) |
| 隐藏依赖跟踪 | [hidden-deps-tracking.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/hidden-deps-tracking.md) |

---

## 结论

**全局审计评级：B（条件通过生产就绪评审）**

- TS 294,663 行 → Go 171,993 行（58.4% 行覆盖率），核心模块功能覆盖率 >95%
- **0 项 P0** — 无核心流程阻断
- **~20 项 P1** — browser 工具端点缺失（标签/Cookie/网络/快照）
- **~16 项 P2** — browser 设备模拟 + cli onboard/auth 向导
- **9 项 P3** — 框架选型差异
- **browser 模块评级降级为 C+** — 如果生产需要浏览器自动化工具，需优先补全
- 7 类隐藏依赖全部通过审计
- 编译 + 静态分析 + 竞态检测全通过
