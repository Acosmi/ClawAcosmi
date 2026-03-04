# 全局审计补全跟踪任务文档

> 创建日期：2026-02-19 | 基于全局审计 7 份报告 + deferred-items.md
>
> 总体评级：**B**（条件通过）| P0: 0 | P1: ~20 | P2: ~16 | P3: 9

---

## W1 小型模块（✅A — 无待办）

> 模块：linkparse, markdown, tts, utils, process, types
> 报告：[global-audit-w1-small.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-w1-small.md)

- [x] linkparse 6→5 Go ✅ FULL
- [x] markdown 6→6 Go ✅ FULL
- [x] tts 1→8 Go ✅ FULL（单文件合理拆分）
- [x] utils 10→散布 🔄 95%（boolean/account-id 内联重构）
- [x] process 5→散布 🔄 90%（child-process-bridge Go 原生支持）
- [x] types 9→30 Go ✅ FULL
- [x] 隐藏依赖审计 7/7 通过
- [x] **W1 差异清单：0 项**

---

## W2 中型模块 A（✅A- — 1 项 P3）

> 模块：security, routing, sessions, outbound, nodehost
> 报告：[global-audit-w2-medium-a.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-w2-medium-a.md)

- [x] security 8→9 Go ✅ FULL（+SSRF 防护）
- [x] routing 3→1+散布 🔄 95%（DI 回调模式）
- [x] sessions 6→7 Go ✅ FULL
- [x] nodehost 2→13 Go ✅ FULL
- [x] 隐藏依赖审计 7/7 通过（outbound directory-cache DI 替代）

### outbound 待跟踪

- [x] deliver.ts → deliver.go ✅
- [x] outbound-policy.ts → policy.go ✅
- [x] outbound-send-service.ts → send.go ✅
- [x] outbound-session.ts → session.go ✅
- [x] 14 个 TS 文件 → 🔄 REFACTORED 至 channels/gateway/autoreply
- [ ] **W2-1 (P3)**: directory-cache Map+TTL 简化为 DI 注入 — 确认生产无 TTL 需求

---

## W3 中型模块 B（⚠️ browser B — 2 项差异）

> 模块：memory, browser, canvas, daemon, cron

- [x] memory 28→21 Go 69.9% **A-**
- [x] canvas 2→4 Go 100%+ **A**
- [x] daemon 19→19 Go 71.2% **A**
- [x] cron 22→19 Go 98.5% **A**
- [ ] **W34-1 (P2)**: browser Agent AI 交互高级功能部分简化
- [ ] **W34-2 (P3)**: Playwright→CDP 直接调用（设计决策）

---

## W4 中型模块 C（⚠️ cli B- / tui B — 2 项差异）

> 模块：hooks, plugins, acp, cli+commands, tui

- [x] hooks 22→15 Go 94% **A**
- [x] plugins 29→16 Go 76.3% **A-**
- [x] acp 10→9 Go 100%+ **A**
- [ ] **W34-3 (P3)**: cli onboard wizard UX 细节
- [ ] **CLI-缺**: onboard 向导 ~18 TS (P2)
- [ ] **CLI-缺**: auth-choice ~16 TS (P2)
- [ ] **CLI-缺**: doctor 子模块 ~15 TS (P2)
- [ ] **CLI-缺**: configure 子模块 ~8 TS (P3)
- [ ] **CLI-缺**: dashboard + reset 命令 (P3)
- [ ] **W34-4 (P3)**: tui Ink.js 24→Bubble Tea 6（框架差异）

---

## W5 config（✅A- — 0 项差异）

> 80 TS/14,329L → 58 Go/10,256L | 功能覆盖 95%+

- [x] schema + zod → Go struct+tag ✅
- [x] io/loader + defaults + group-policy + includes + paths ✅
- [x] legacy 迁移 ×6 ✅ | validation ✅ | types ×30 ✅
- [x] sessions/store/cache-utils/commands/talk/merge-config ✅
- [x] 所有隐藏依赖已覆盖（Zod→validator）

---

## W6 infra（⚠️B — 2 项 P3）

> 99 TS/18,428L → 43 Go/6,781L | 核心功能 100%

- [x] exec-approvals + exec-approval-forwarder ✅
- [x] heartbeat-runner 全套 ✅
- [x] state-migrations ×8 ✅
- [x] bonjour + discovery ✅ | ports ✅
- [x] provider-usage ×12 → cost/ ✅
- [x] node-pairing + skills-remote + ssh-tunnel ✅
- [x] ~30 个 Node.js 特有工具文件无需 Go 等价
- [ ] **W56-1 (P3)**: update-runner/update-check 自更新未移植
- [ ] **W56-2 (P3)**: channel-activity/channel-summary 位置未确认

---

## W7 gateway（✅A- — 0 项差异）

> 133 TS/26,457L → 67 Go/20,819L | 78.7%

- [x] ~70 WS 方法全部注册
- [x] 核心文件全覆盖（device_pairing/usage/sessions/nodes/ws_log/openai_http/agents/reload/hooks_mapping）
- [x] Phase 11 深度审计 + Phase 13 D-W1 stub 全量实现

---

## W8 agents（⚠️B+ — 1 项 P3）

> 233 TS/46,991L → 144 Go/34,957L | 74.4%

- [x] bash/ 23 文件 ✅ | runner/ 22 文件 ✅ | tools/ 29 文件 ✅
- [x] models/ + auth/ + sandbox/ + skills/ + scope/ + llmclient/ ✅
- [x] 21 子包全覆盖
- [ ] **W8-1 (P3)**: `stream/` 空包可清理

---

## W9 autoreply（✅A- — 0 项差异）

> 121 TS/22,028L → 90 Go/15,668L | 71.1%

- [x] autoreply/根 39 文件 + reply/ 51 文件全覆盖
- [x] Phase 7 Batch D + Phase 8 + Phase 11 Batch E 全量移植

---

## W10 channels（✅A- — 0 项差异）

> ~337 TS/42,028L → 209 Go/34,545L | 82.2%

- [x] 8 频道 SDK 全量实现
- [x] discord 44→38 ✅ | telegram 40→36 ✅ | slack 43→37 ✅
- [x] imessage 12→13 ✅ | signal 14→14 ✅ | whatsapp 43→16 ✅ | line 21→9 ✅
- [x] channels 抽象层 77→20 ✅

---

## 🔴 深度差异审计 — browser（C+ → 最大差距区域）

> 报告：[global-audit-deep-discrepancy.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/global-audit-deep-discrepancy.md)

### P1 浏览器工具端点缺失 (~20 项)

- [ ] **DEEP-1**: 标签管理 — openTab/closeTab/focusTab/tabs/tabAction
- [ ] **DEEP-2**: Cookie/Storage — cookies/cookiesClear/storageGet/Set/Clear
- [ ] **DEEP-3**: 网络观测 — requests/responseBody/consoleMessages/pageErrors
- [ ] **DEEP-4**: 快照/调试 — snapshot/highlight/traceStart/traceStop

### P2 浏览器附加功能 (~15 项)

- [ ] **DEEP-5**: 设备模拟 — setDevice/setGeolocation/setLocale 等 8 个
- [ ] **DEEP-6**: PDF/下载 — pdfSave/download/waitForDownload
- [ ] **DEEP-7**: Profile 管理 — createProfile/deleteProfile/resetProfile

### P2 CLI 缺失

- [ ] **DEEP-8**: onboard 交互式向导 (~18 TS 文件)
- [ ] **DEEP-9**: auth-choice 认证选择 (~16 TS 文件)

---

## exec_tool.go 审计差异（9 项 GAP）

> 来源：[deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) Phase 13 审计

### P1 关键 (4 项)

- [ ] **GAP-1**: gateway/node 审批从异步 fire-and-forget → 同步阻塞
- [ ] **GAP-2**: node invokeBody 缺少 rawCommand/agentId/approved 等 6 字段
- [ ] **GAP-3**: node 分支缺少 system.run 命令支持检查
- [ ] **GAP-4**: node payload 解析缺少 .payload 层级和 error/success 字段

### P2 中等 (4 项)

- [ ] **GAP-5**: getWarningText() 未在结果 text 前缀
- [ ] **GAP-6**: emitExecSystemEvent 未被 gateway/node 分支调用
- [ ] **GAP-7**: gateway 分支缺少 allowlist-miss 最终 deny 检查
- [ ] **GAP-8**: onAbortSignal 未检查 backgrounded 状态就 kill

### P3 设计差异 (1 项)

- [ ] **GAP-9**: ExecAsk/ExecSecurity 在 bash 和 infra 包重复定义

---

## 项目级推迟项（deferred-items.md）

- [ ] **P11-1 (P3)**: Ollama 本地 LLM 集成
- [ ] **P11-2 (P3)**: 前端 View 文件 i18n 全量抽取
- [ ] **BW1-D1 (P2)**: Sandbox/Cost 单元测试基础设施
- [ ] **BW1-D2 (P2)**: Provider Fetch 外部 API 兼容性验证
- [ ] **BW1-D3 (P2)**: Provider Auth 模块 auth-profiles 对接

---

## 综合统计

| 优先级 | 数量 | 主要来源 |
|--------|------|----------|
| P0 | 0 | — |
| P1 | ~24 | browser 端点 ×20 + exec_tool GAP ×4 |
| P2 | ~23 | browser 附加 ×15 + CLI 向导 ×2 + exec GAP ×4 + deferred ×3 |
| P3 | ~11 | 框架差异 ×6 + infra ×2 + exec ×1 + Ollama ×1 + i18n ×1 |
| **合计** | **~58 项待办** | |

### 隐藏依赖审计: 7/7 全部通过 ✅

### 编译状态: `go build ./...` ✅ | `go vet ./...` ✅
