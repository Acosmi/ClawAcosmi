# Phase 13 — 功能补缺实施 任务清单

> 范围：基于 S1-S6 生产级审计 + gap-analysis 差距分析（9 篇文档）
>
> 审计复核日期：2026-02-18
>
> **执行顺序**：D-W0 → A-W1 → A-W2 → A-W3a → A-W3b → C-W1 → B-W1 → B-W2 → B-W3 → D-W1 → D-W1b → D-W2 → F-W1 → F-W2 → G-W1 → G-W2
>
> **参考文档**：
>
> - 差距分析：`brain/0466828e-*/gap-analysis-part1~4f.md`
> - 最终执行计划：`brain/0466828e-*/final-execution-plan.md`
> - 实施计划原稿：`brain/4cb3ce79-*/implementation_plan.md.resolved`

---

## 窗口总览（17 窗口 → 19 会话）

| 序号 | 窗口 | 会话数 | 内容摘要 | 优先级 |
|------|------|--------|----------|--------|
| 1 | D-W0 | 1 | P12 剩余项（requestJSON/allowlist/proxy） | 🔴 P0 |
| 2 | A-W1 | 1 | 工具基础层 + agents/schema/ | 🔴 P0 |
| 3 | A-W2 | 1 | 文件/会话/媒体工具 | 🔴 P0 |
| 4 | A-W3a | 1 | 频道操作工具（Discord/Slack/Telegram/WA） | 🔴 P0 |
| 5 | A-W3b | 1 | Bash/PTY/补丁/子代理 ⚠️高风险 | 🔴 P0 |
| 6 | C-W1 | **2** | 沙箱（16文件）+ security 补全 | 🔴 P0 |
| 7 | B-W1 | 1 | 计费/用量追踪（8文件） | 🔴 P0 |
| 8 | B-W2 | 1 | 迁移/配对/远程（含前置侦察） | 🔴 P0 |
| 9 | B-W3 | 1 | infra 补全（exec_approvals+heartbeat） | 🔴 P0 |
| 10 | D-W1 | **2** | Gateway 44 个 stub → 真实实现 | 🟡 P1 |
| 11 | D-W1b | 1 | Gateway 非stub方法补全（含前置侦察） | 🟡 P1 |
| 12 | D-W2 | **2** | Auth+Skills三件套+Extensions | 🟡 P1 |
| 13 | F-W1 | 1 | CLI 命令注册 | 🟡 P1 |
| 14 | F-W2 | 1 | TUI bubbletea 渲染 | 🟡 P1 |
| 15 | G-W1 | 1 | 杂项+autoreply补全+WS排查 | 🟢 P2 |
| 16 | G-W2 | 1 | LINE channel SDK 完整实现 🆕 | 🟢 P2 |

> **合计**：17 窗口 → **19 个会话**（C-W1/D-W1/D-W2 各拆 2 会话）
>
> ⚠️ **注意**：A 组（A-W1~A-W3b）与 B 组（B-W1~B-W3）无依赖关系，可并行推进。

---

## 依赖关系图

```
D-W0 → A-W1 → A-W2 → A-W3a
                    → A-W3b → C-W1（sandbox 依赖 bash-tools）

B-W1 → B-W2 → B-W3          ← 与 A 组可并行

D-W1 → D-W1b
D-W2（需 A-W1 工具框架 + D-W1 skills stub）

F-W1 → F-W2（CLI 先于 TUI）

G-W1 → G-W2（杂项先于 LINE）
```

---

## 每会话启动协议

每个新会话窗口开始时需提供：

1. 对应 `gap-analysis-part4*.md` 中的任务清单
2. 相关 TS 源文件路径
3. 当前 Go 代码位置
4. `/refactor` 工作流引用
5. 前序窗口的完成状态

---

## 验证策略（每窗口完成后执行）

```bash
cd backend && go build ./... && go vet ./... && go test -race ./...
```

---

## 窗口 D-W0：P12 剩余项（最先执行）

> 参考：`gap-analysis-part4a.md` D-W0 节
> 范围：3 个 P12 遗留任务，集中在 `nodehost/` 和 `exec/`

- [ ] **D-W0-T1**: `requestJSON` 实现
  - 文件：`backend/internal/nodehost/runner.go` L323-327（当前 stub）
  - TS 对应：`runner.ts` WS request-response 通信
  - [ ] 实现 `requestJSON(method, params, result)` — 通过 GatewayClient 发送请求并等待响应
  - [ ] 注入 `GatewayClient` 或 `RequestFunc` 到 `NodeHostService`
  - [ ] 添加超时处理（`context.WithTimeout`）
  - [ ] 用于 `skills.bins` 请求-响应对
  - ⚠️ **注意**：注入点在 gateway 启动序列，可能影响工作量估算（当前估 ~600L，实际可能偏高）

- [ ] **D-W0-T2**: `browser.proxy` 分支
  - 文件：`runner.go` `HandleInvoke` switch（L61-84）
  - [ ] 添加 `case "browser.proxy":`
  - [ ] 解码 `BrowserProxyParams`（URL + method + headers + body）
  - [ ] 调用 `internal/browser` 包的 HTTP 代理
  - [ ] 通过 `sendInvokeResult` 返回响应

- [ ] **D-W0-T3**: `allowlist` 评估移植
  - TS 来源：`runner.ts` L885-1160（~275L）
  - [ ] `evaluateShellAllowlist()` — 命令白名单匹配
  - [ ] `evaluateExecAllowlist()` — 执行白名单
  - [ ] `requiresExecApproval()` — 是否需要审批判断
  - [ ] `detectMacApp()` — macOS .app 检测
  - Go 目标：新建 `nodehost/allowlist.go` 或 `agents/exec/allowlist.go`
  - 依赖：`internal/security/` 审计规则

- [ ] **D-W0 验证**：`go build ./internal/nodehost/... && go test -race ./internal/nodehost/...`

---

## 窗口 A-W1：工具基础层（含 agents/schema/）

> 参考：`gap-analysis-part4c.md` A-W1 节
> ⭐ 审计复核追加：`agents/schema/` (2文件 419L) 移入本窗口（原计划在 D-W2，但 schema 是工具调用类型基础，与工具框架天然配套）

- [ ] **A-W1-T1**: 工具基础框架
  - TS: `common.ts` (244L)
  - Go 目标: `agents/tools/common.go`
  - [ ] `ReadStringParam()` / `ReadNumberParam()` / `ReadStringArrayParam()`
  - [ ] `ReadStringOrNumberParam()` / `ReadReactionParams()`
  - [ ] `CreateActionGate()` — 工具动作开关
  - [ ] `JsonResult()` / `ImageResult()` / `ImageResultFromFile()`

- [ ] **A-W1-T2**: 工具 Schema + 注册
  - TS: `pi-tools.schema.ts` (179L) + `openacosmi-tools.ts` (170L)
  - Go 目标: `agents/tools/schema.go` + `registry.go`
  - [ ] 工具 Schema 定义结构体（参数类型/约束/描述）
  - [ ] `RegisterTool()` / `GetTool()` / `ListTools()`

- [ ] **A-W1-T3**: 工具策略引擎
  - TS: `pi-tools.policy.ts` (339L) + `tool-policy.ts` (291L)
  - Go 目标: `agents/tools/policy.go`
  - [ ] `EvaluateToolPolicy()` — allow/deny 决策
  - [ ] `ResolveToolPermissions()` — 权限解析

- [ ] **A-W1-T4**: 工具辅助
  - TS: `tool-display.ts` (291L) + `tool-call-id.ts` (221L) + `tool-images.ts` (223L)
  - Go 目标: `agents/tools/display.go` + `callid.go` + `images.go`
  - [ ] `FormatToolName()` / `GenerateToolCallID()` / `SanitizeToolResultImages()`

- [ ] **A-W1-T5**: 文件读取 + 频道桥接 + Gateway
  - TS: `pi-tools.read.ts` (302L) + `channel-tools.ts` (121L) + `gateway-tool.ts` + `gateway.ts` (~200L)
  - Go 目标: `agents/tools/read.go` + `channel_bridge.go` + `gateway.go`

- [ ] **A-W1-T6**: Agent 步骤
  - TS: `agent-step.ts` (~150L)
  - Go 目标: `agents/tools/agent_step.go`

- [ ] **A-W1-T7**: agents/schema/ 新建（审计复核追加）
  - TS: `src/agents/schema/` (2文件 419L)
  - Go 目标: `internal/agents/schema/`
  - [ ] Agent Schema 定义/校验

- [ ] **A-W1 验证**：`go build ./internal/agents/tools/... && go test -race ./internal/agents/tools/...`

---

## 窗口 A-W2：文件/会话/媒体工具

> 参考：`gap-analysis-part4c.md` A-W2 节

- [ ] **A-W2-T1**: 图片工具
  - TS: `image-tool.ts` + `image-tool.helpers.ts` (~500L)
  - Go 目标: `agents/tools/image_tool.go`
  - [ ] 图片生成/编辑/分析工具定义 + 裁切/缩放/格式转换辅助

- [ ] **A-W2-T2**: 记忆 + 网页抓取
  - TS: `memory-tool.ts` (~350L) + `web-fetch.ts` + `web-fetch-utils.ts` (~500L)
  - Go 目标: `agents/tools/memory_tool.go` + `web_fetch.go`
  - [ ] `MemorySearchTool` / `WebFetchTool` / `web-search.ts` + `web-shared.ts` + `web-tools.ts` 辅助

- [ ] **A-W2-T3**: 消息 + 会话工具集
  - TS: 8 个 sessions-* 文件 (~1,700L) + `message-tool.ts` (~250L)
  - Go 目标: `agents/tools/sessions.go` + `message_tool.go`
  - [ ] `SessionsListTool` / `SessionsHistoryTool` / `SessionsSendTool`
  - [ ] `SessionsSpawnTool` / `SessionsAnnounceTarget` / `SessionStatusTool` / `MessageTool`

- [ ] **A-W2-T4**: 节点/画布/Agent/定时/TTS/浏览器
  - TS: 8 个文件 (~1,500L)
  - [ ] `AgentsListTool` / `NodesTool` + `NodesUtils` / `CanvasTool`
  - [ ] `CronTool` / `TTSTool` / `BrowserTool` + schema

- [ ] **A-W2 验证**：`go test -race ./internal/agents/tools/...`

---

## 窗口 A-W3a：频道操作工具

> 参考：`gap-analysis-part4d.md` A-W3a 节

- [ ] **A-W3a-T1**: Discord 操作工具 (~1,200L)
  - TS: 5 个文件（discord-actions.ts + guild/messaging/moderation/presence）
  - Go 目标: `agents/tools/discord_actions.go`
  - [ ] `discordSendMessage` / `discordEditMessage` / `discordDeleteMessage`
  - [ ] `discordReact` / `discordPin` / `discordCreateThread`
  - [ ] Guild 管理 / Moderation / Presence
  - 依赖：`channels/discord/` 包（已有 38 文件 6,211L）

- [ ] **A-W3a-T2**: Slack 操作工具 (~400L)
  - TS: `slack-actions.ts`
  - Go 目标: `agents/tools/slack_actions.go`
  - [ ] `slackSendMessage` / `slackEditMessage` / `slackDeleteMessage` / `slackReact` / `slackPin` / `slackUploadFile`

- [ ] **A-W3a-T3**: Telegram 操作工具 (~350L)
  - TS: `telegram-actions.ts`
  - Go 目标: `agents/tools/telegram_actions.go`
  - [ ] `telegramSendMessage` / `telegramEditMessage` / `telegramDeleteMessage` / `telegramReact` / `telegramSendPhoto` / `telegramSendDocument`

- [ ] **A-W3a-T4**: WhatsApp 操作工具
  - TS: `whatsapp-actions.ts`（行数较少）
  - Go 目标: `agents/tools/whatsapp_actions.go`
  - [ ] 基本消息发送/媒体发送

- [ ] **A-W3a 验证**：`go build ./internal/agents/tools/...`

---

## 窗口 A-W3b：Bash 执行链 + PTY（⚠️ 高风险）

> 参考：`gap-analysis-part4d.md` A-W3b 节
> ⚠️ 高风险：TS 依赖 PTY + Docker，Go 用 `os/exec` + `creack/pty`，是 Agent 执行动作的核心

- [ ] **A-W3b-T1**: Bash 命令执行 (1,630L)
  - TS: `bash-tools.exec.ts`
  - Go 目标: `agents/bash/exec.go`
  - [ ] `executeBashCommand()` — 主入口
  - [ ] Docker 沙箱模式 vs 本地模式分支
  - [ ] 命令预处理（危险命令检测/替换）
  - [ ] 输出截断 + token 限制
  - [ ] 超时控制 + 信号处理
  - [ ] 审批流程集成（需 `security/` 配合）

- [ ] **A-W3b-T2**: PTY 进程管理 (665L + 293L)
  - TS: `bash-tools.process.ts` + `pty-keys.ts`
  - Go 目标: `agents/bash/process.go` + `pty_keys.go`
  - [ ] PTY 分配（`creack/pty`）
  - [ ] 输入/输出流管理
  - [ ] 键映射（Ctrl-C/D/Z 等特殊键）
  - [ ] 窗口大小调整（`SIGWINCH`）

- [ ] **A-W3b-T3**: Docker 参数 + 进程注册表 (252L + 274L)
  - TS: `bash-tools.shared.ts` + `bash-process-registry.ts`
  - Go 目标: `agents/bash/shared.go` + `registry.go`
  - [ ] Docker 运行参数构建（挂载/网络/资源限制）
  - [ ] 全局进程注册表（Map 单例 + 清理）

- [ ] **A-W3b-T4**: 代码补丁 (~700L)
  - TS: `apply-patch.ts` + `apply-patch-update.ts`
  - Go 目标: `agents/bash/patch.go`
  - [ ] `ApplyPatch()` / `ApplyPatchUpdate()` / 冲突检测 + 回滚

- [ ] **A-W3b-T5**: 子代理系统 (~1,000L)
  - TS: `subagent-registry.ts` (430L) + `subagent-announce.ts` (572L)
  - Go 目标: `agents/tools/subagent.go`
  - [ ] 子代理注册/注销 / 子代理公告广播 / 任务队列管理

- [ ] **A-W3b-T6**: 缓存追踪 (294L)
  - TS: `cache-trace.ts`
  - Go 目标: `agents/tools/cache_trace.go`

- [ ] **A-W3b 验证**：`go build ./internal/agents/bash/... && go test -race ./internal/agents/bash/...`

---

## 窗口 C-W1：沙箱 + 安全补全（2 会话）

> 参考：`gap-analysis-part4e.md` C-W1 节

### 会话 C-W1a：Docker 沙箱核心

- [ ] **C-W1-T1**: Docker 沙箱（16 个 TS 文件 → ~6 个 Go 文件）
  - TS 目录: `src/agents/sandbox/` (1,848L)
  - Go 目标: `backend/internal/agents/sandbox/`（当前空目录）
  - [ ] `types.go` — 类型定义（types.ts + types.docker.ts + constants.ts）
  - [ ] `config.go` — 沙箱配置/哈希（config.ts + config-hash.ts + shared.ts）
  - [ ] `docker.go` — 容器管理/清理（docker.ts + manage.ts + prune.ts）
  - [ ] `runtime.go` — 运行时状态（runtime-status.ts + context.ts）
  - [ ] `workspace.go` — 工作区挂载/注册（workspace.ts + registry.ts）
  - [ ] `browser.go` — 浏览器桥接（browser.ts + browser-bridges.ts）
  - 关键函数：`CreateSandbox()` / `DestroySandbox()` / `PruneSandboxes()`
  - 关键函数：`ResolveSandboxConfig()` / `ComputeConfigHash()`
  - 关键函数：`ResolveSandboxRuntimeStatus()` — S6 审计发现的跨模块依赖
  - 关键函数：`MountWorkspace()` / `RegisterSandbox()`

### 会话 C-W1b：security/ 补全

- [ ] **C-W1-T2**: security/ 补全
  - 当前: 11 文件，S5 审计显示 61% 覆盖（2,438L vs TS 4,028L）
  - ⚠️ **注意**：`tool-policy.ts` (291L) 在 sandbox/ 中也有一份，需确定归属（已在 A-W1-T3 处理）
  - **审计复核：以下为逐函数对比确认的具体缺失项**：
  - [ ] **audit-extra.ts 缺失 8 个函数**（Go `audit_extra.go` 仅实现了前 6 个）：
    - [ ] `collectSmallModelRiskFindings()` — 小模型安全风险评估
    - [ ] `collectPluginsTrustFindings()` — 插件信任度评估
    - [ ] `collectIncludeFilePermFindings()` — include 文件权限检查
    - [ ] `collectStateDeepFilesystemFindings()` — 状态目录深度文件系统扫描
    - [ ] `collectExposureMatrixFindings()` — 暴露面矩阵分析
    - [ ] `readConfigSnapshotForAudit()` — 审计用配置快照读取
    - [ ] `collectPluginsCodeSafetyFindings()` — 插件代码安全扫描
    - [ ] `collectInstalledSkillsCodeSafetyFindings()` — 已安装技能代码安全扫描
  - [ ] **fix.ts 整个文件缺失** (455L)：
    - [ ] `fixSecurityFootguns()` — 安全自修复（chmod/icacls 批量修正）
    - [ ] `SecurityFixAction` / `SecurityFixResult` 类型定义
  - [ ] 补全缺失的审批决策路径

- [ ] **C-W1 验证**：`go build ./internal/agents/sandbox/... ./internal/security/... && go test -race ./...`

---

## 窗口 B-W1：计费与用量追踪

> 参考：`gap-analysis-part4e.md` B-W1 节

- [ ] **B-W1-T1**: 会话成本追踪 (1,092L)
  - TS: `src/infra/session-cost-usage.ts`
  - Go 目标: `internal/infra/cost/session_cost.go`
  - [ ] `TrackSessionCost()` / `GetSessionUsageSummary()` / `ResetSessionCost()`
  - [ ] Token 计数 + 定价计算逻辑

- [ ] **B-W1-T2**: Provider 用量 API (7 文件 ~900L)
  - TS: `provider-usage.*.ts`（OpenAI/Anthropic/Google/DeepSeek/等）
  - Go 目标: `internal/infra/cost/` 下多个文件
  - [ ] `FetchOpenAIUsage()` / `FetchAnthropicUsage()` / `FetchGoogleUsage()` / `FetchDeepSeekUsage()`
  - [ ] 其余 3 个 provider 适配器
  - [ ] 统一 `ProviderUsage` 接口

- [ ] **B-W1 验证**：`go build ./internal/infra/... && go test -race ./internal/infra/...`

---

## 窗口 B-W2：迁移/配对/远程（含前置侦察）

> 参考：`gap-analysis-part4e.md` B-W2 节
> ⭐ **审计复核追加前置步骤**：B-W2 开始前必须先确认 `exec-approval-forwarder` 现状，避免重复劳动

### 🔍 前置侦察（B-W2 开始前执行，约 15 分钟）

- [ ] **B-W2-PRE**: 检查 forwarder 现状

  ```bash
  find backend/internal/infra -name "*approval*" -o -name "*forwarder*"
  grep -r "forwarder\|ForwardApproval" backend/internal/infra/ --include="*.go" -l
  ```

  - S1 审计记录 `exec-approval-forwarder.ts` 状态为 "⚠️ 部分"
  - 根据侦察结果决定：全新实现 or 补全已有部分

### 主要任务

- [ ] **B-W2-T1**: 状态迁移 (970L)
  - TS: `src/infra/state-migrations.ts`
  - Go 目标: `internal/infra/migrations.go`
  - [ ] 版本号检测 + 迁移链执行
  - [ ] 各版本迁移函数（逐个对照 TS）
  - [ ] 文件系统操作（重命名/移动/格式转换）
  - [ ] 回滚保护

- [ ] **B-W2-T2**: 审批转发 (352L)
  - TS: `src/infra/exec-approval-forwarder.ts`
  - Go 目标: `internal/infra/approval_forwarder.go`（根据前置侦察结果决定工作量）
  - [ ] Gateway 双通道转发逻辑
  - [ ] 超时/重试处理

- [ ] **B-W2-T3**: 远程技能 (361L) + 节点配对 (336L)
  - TS: `skills-remote.ts` + `node-pairing.ts`
  - Go 目标: `internal/infra/skills_remote.go` + `node_pairing.go`
  - [ ] 远程技能发现 + 调用
  - [ ] 节点配对握手 + 验证

- [ ] **B-W2 验证**：`go build ./internal/infra/... && go test -race ./internal/infra/...`

---

## 窗口 B-W3：infra 已有文件补全

> 参考：`gap-analysis-part4a.md` B-W3 节

- [ ] **B-W3-T1**: exec_approvals.go 补全
  - 文件：`backend/internal/infra/exec_approvals.go`（当前 230L，TS 1,541L）
  - TS 来源：`src/infra/exec-approvals.ts`
  - [ ] `getExecApprovals()` — 获取审批记录列表
  - [ ] `setExecApproval()` — 设置审批结果
  - [ ] `resolveExecApproval()` — 解析审批状态
  - [ ] `cleanupExpiredApprovals()` — 过期审批清理
  - [ ] `forwardApprovalToGateway()` — 审批转发（依赖 gateway）
  - [ ] 持久化存储逻辑（TS 用文件系统）
  - 预估新增：~770L

- [ ] **B-W3-T2**: heartbeat.go 补全
  - 文件：`backend/internal/infra/heartbeat.go`（当前 356L，TS 1,030L）
  - TS 来源：`src/infra/heartbeat-runner.ts`
  - [ ] `deliverHeartbeat()` — 心跳发送到各频道
  - [ ] `resolveHeartbeatTargets()` — 心跳目标解析
  - [ ] `formatHeartbeatPayload()` — 负载格式化
  - [ ] outbound deliver 集成
  - 预估新增：~350L

- [ ] **B-W3 验证**：`go build ./internal/infra/... && go test -race ./internal/infra/...`

---

## 窗口 D-W1：Gateway Stub 全量实现（2 会话）

> 参考：`gap-analysis-part4b.md` D-W1 节
> 文件：`backend/internal/gateway/server_methods_stubs.go` → 拆分为多个实现文件
> 精确计数：**44 个 stub**（原计划 ~22 个，差距分析修正）

### 会话 D-W1a：G1~G4 组（cron/tts/skills/node）

- [ ] **D-W1-G1**: cron.* (7 个方法)
  - TS: `src/gateway/server-methods/cron.ts` (227L)
  - Go 对接: `internal/cron/` 包（19 文件 3,711L，已完整）
  - [ ] `cron.list` / `cron.status` / `cron.runs` / `cron.add` / `cron.update` / `cron.remove` / `cron.run`
  - 输出：新建 `server_methods_cron.go`

- [ ] **D-W1-G2**: tts.* (6 个方法)
  - TS: `src/gateway/server-methods/tts.ts` (157L)
  - Go 对接: `internal/tts/` 包（8 文件 1,881L，已完整）
  - [ ] `tts.status` / `tts.providers` / `tts.enable` / `tts.disable` / `tts.convert` / `tts.setProvider`
  - 输出：新建 `server_methods_tts.go`

- [ ] **D-W1-G3**: skills.* (4 个方法)
  - TS: `src/gateway/server-methods/skills.ts` (216L)
  - Go 对接: `internal/agents/skills/`（待 D-W2 补全）
  - [ ] `skills.status` / `skills.install` / `skills.update` / `skills.bins`
  - ⚠️ **注意**：需在 D-W2 完成 skills 补全后才能完整实现
  - 输出：新建 `server_methods_skills.go`

- [ ] **D-W1-G4**: node.*(11 个方法，含 pair.* 子方法)
  - TS: `src/gateway/server-methods/nodes.ts` (537L)
  - Go 对接: `internal/nodehost/` + `internal/gateway/device_pairing.go`
  - [ ] `node.list` / `node.describe` / `node.invoke` / `node.invoke.result` / `node.event` / `node.rename`
  - [ ] `node.pair.request` / `node.pair.list` / `node.pair.approve` / `node.pair.reject` / `node.pair.verify`
  - 输出：新建 `server_methods_nodes.go`

### 会话 D-W1b_stubs：G5~G7 组（device/exec.approval/其余10个）

- [ ] **D-W1-G5**: device.* (5 个方法)
  - TS: `src/gateway/server-methods/devices.ts` (190L)
  - Go 对接: `internal/gateway/device_pairing.go` (843L)
  - [ ] `device.pair.list` / `device.pair.approve` / `device.pair.reject` / `device.token.rotate` / `device.token.revoke`
  - 输出：新建 `server_methods_devices.go`

- [ ] **D-W1-G6**: exec.approval.* (4 个方法)
  - TS: `exec-approvals.ts` (242L)
  - Go 对接: `server_methods_exec_approvals.go` (232L，已有部分)
  - [ ] `exec.approval.request` / `exec.approval.resolve` / `exec.approvals.list` / `exec.approvals.resolve`
  - 输出：扩展已有文件

- [ ] **D-W1-G7**: 其余 10 个方法
  - [ ] `voicewake.get` / `voicewake.set` — 语音唤醒
  - [ ] `update.check` / `update.run` — 自动更新
  - [ ] `browser.request` — 浏览器请求
  - [ ] `wake` / `talk.mode` — 唤醒/对话模式
  - [ ] `web.login.start` / `web.login.wait` — WhatsApp QR 登录

- [ ] **D-W1 验证**：`go build ./internal/gateway/... && go test -race ./internal/gateway/...`

---

## 窗口 D-W1b：Gateway 非 Stub 方法补全（含前置侦察）

> 参考：`gap-analysis-part4b.md` D-W1b 节
> ⭐ **审计复核追加前置步骤**：protocol/ 工作量不确定，需先侦察再估算

### 🔍 前置侦察（D-W1b 开始前执行，约 15-30 分钟）

- [ ] **D-W1b-PRE**: protocol/ 实际缺口评估

  ```bash
  # TS 端：统计 protocol/ 文件列表
  find src/gateway/protocol -name '*.ts' | xargs wc -l | sort -n
  # Go 端：检查 gateway/ 中已内联的协议定义
  grep -r "type.*Protocol\|type.*Message\|type.*Packet" backend/internal/gateway/ --include="*.go" -l
  ```

  - TS `protocol/` 20 文件 2,800L vs Go `protocol.go` 270L
  - 判断：大量内联到 `gateway/*.go` → 实际缺口可能远小于 ~1,500L
  - 根据侦察结果调整 D-W1b-T4 工作量

### 主要任务

- [ ] **D-W1b-T1**: server_methods_config.go 补全
  - 当前 285L vs TS 460L
  - [ ] 逐函数对比缺失方法（配置更新/覆盖/插件配置相关）

- [ ] **D-W1b-T2**: server_methods_send.go 补全
  - 当前 227L vs TS 364L
  - [ ] 补全消息发送路由/多频道分发逻辑

- [ ] **D-W1b-T3**: server_methods_agent_rpc.go 补全
  - 当前 316L vs TS 515L
  - [ ] 补全 Agent RPC 剩余方法

- [ ] **D-W1b-T4**: protocol/ 补全（工作量由前置侦察决定）
  - 当前 `protocol.go` 270L vs TS `protocol/` 20 文件 2,800L
  - ⚠️ 可能部分已内联到 `gateway/*.go`，需深度对比确认实际缺口

- [ ] **D-W1b 验证**：`go build ./internal/gateway/... && go test -race ./internal/gateway/...`

---

## 窗口 D-W2：Auth + Skills + Extensions（2 会话）

> 参考：`gap-analysis-part4f.md` D-W2 节
> ⭐ **审计复核修正**：`agents/schema/` 已移入 A-W1，本窗口不再包含

### 会话 D-W2a：Auth Profiles 补全

- [ ] **D-W2-T1**: Auth Profiles 补全 (TS 15 文件 1,939L → Go 1 文件 344L)
  - TS 目录: `src/agents/auth-profiles/`
  - Go 目标: `internal/agents/auth/` 扩展
  - [ ] OAuth 流程：`initiateOAuth()` / `handleCallback()` / `refreshToken()`
  - [ ] 凭证存储：`saveCredential()` / `loadCredential()` / `deleteCredential()`
  - [ ] 配置文件管理：`listProfiles()` / `switchProfile()` / `createProfile()`
  - [ ] 使用量跟踪：`trackAuthUsage()`
  - 预计新增：~1,200L，5 个新 Go 文件

### 会话 D-W2b：Skills 三件套 + Extensions

- [ ] **D-W2-T2**: Skills 三件套（来自 deferred-items.md SKILLS-1/2/3）
  - SKILLS-1: `frontmatter.go` + `eligibility.go`
    - [ ] `ResolveOpenAcosmiMetadata()` / `ResolveSkillInvocationPolicy()` / `ShouldIncludeSkill()` / `HasBinary()`
  - SKILLS-2: `env_overrides.go` + `refresh.go` + `bundled_dir.go`
    - [ ] `ApplySkillEnvOverrides()` / `EnsureSkillsWatcher()` / `BumpSkillsSnapshotVersion()` / `ResolveBundledSkillsDir()`
  - SKILLS-3: `install.go` + Gateway stubs 填充
    - [ ] `InstallSkill()` / `UninstallSkill()` / `CheckSkillStatus()`
    - [ ] `BuildWorkspaceSkillCommandSpecs()` / `SyncSkillsToWorkspace()`

- [ ] **D-W2-T3**: pi-extensions (8 文件 1,019L)
  - TS 目录: `src/agents/pi-extensions/`
  - Go 目标: `internal/agents/extensions/`
  - [ ] 扩展点注册机制 / 扩展生命周期（加载/卸载/热更新）/ 扩展上下文隔离

- [ ] **D-W2 验证**：`go test -race ./internal/agents/auth/... ./internal/agents/skills/... ./internal/agents/extensions/...`

---

## 窗口 F-W1：CLI 命令注册

> 参考：`gap-analysis-part4f.md` F-W1 节

- [ ] **F-W1-T1**: 核心命令实现（估 ~30 个高频命令）
  - 当前: 18 个 `cmd_*.go`，大部分为骨架；TS: 174 个命令文件
  - [ ] 命令选项从 Commander.js → Cobra flags 映射
  - [ ] 子命令路由完善
  - [ ] cli-runner 集成（命令→Gateway WS 调用）
  - 策略：按 `cmd_*.go` 逐个文件补全

- [ ] **F-W1 验证**：`go build ./cmd/openacosmi/... && go test -race ./cmd/openacosmi/...`

---

## 窗口 F-W2：TUI 渲染（bubbletea）

> 参考：`gap-analysis-part4f.md` F-W2 节
> TUI 库决策：**bubbletea**（Elm Architecture，GitHub CLI 使用，可测试性强）

- [ ] **F-W2-T1**: bubbletea 框架搭建 — Model/Update/View 基础结构
  - Go 目标: `internal/tui/` 新目录 ~6 文件
- [ ] **F-W2-T2**: Setup wizard TUI 版（对应 TS clack-prompter）+ spinner/progress bar
- [ ] **F-W2-T3**: Agent 交互式对话 TUI

- [ ] **F-W2 验证**：`go build ./internal/tui/... && go test -race ./internal/tui/...`

---

## 窗口 G-W1：杂项 + autoreply 补全 + WS 排查

> 参考：`gap-analysis-part4f.md` G-W1 节
> ⭐ **审计复核修正**：LINE channel 已拆出为独立 G-W2，本窗口不再包含

- [ ] **G-W1-T1**: 原始杂项
  - [ ] `venice-models.ts` (393L) / `opencode-zen-models.ts` (316L) / `cli-credentials.ts` (607L)
  - [ ] channels/ 核心路由补全 (~2,000L)
  - [ ] `control-ui-assets.ts` (274L) / `infra/update-runner` (912L) / `infra/ssh-tunnel` (213L)

- [ ] **G-W1-T2**: autoreply 补全（差距分析发现的遗漏）
  - [ ] `commands_handler_bash.go` 55L → ~300L — /bash 执行逻辑
  - [ ] `status.go` 384L → ~500L — 状态管理补全
  - [ ] `get_reply_inline_actions.go` 185L → ~300L — 内联动作补全
  - ⚠️ **审计复核备注：依赖关系**：G-W1-T2 的 `commands_handler_bash.go` 是 autoreply 层的 /bash **命令处理器**（解析指令、权限检查），它需要调用 A-W3b 的 `agents/bash/exec.go` **执行引擎**（实际运行命令）。因此 **G-W1 必须在 A-W3b 完成后执行**，当前依赖图已满足此约束。

- [ ] **G-W1-T3**: WS 断连根因排查（Phase 11 遗留）
  - [ ] 可能原因：心跳超时/缓冲区溢出/并发写入竞态
  - [ ] 需结合日志分析 + 压力测试
  - ⚠️ **审计复核备注：优先级待评估** — 当前为 P2 最后执行，但如 WS 断连影响开发调试效率，可考虑提前到 B-W3 之后执行。建议在 A 组完成后评估实际断连频率再决定。

- [ ] **G-W1 验证**：`go build ./... && go test -race ./...`

---

## 窗口 G-W2：LINE Channel SDK 完整实现 🆕

> ⭐ **审计复核新增**：从 G-W1 拆出，独立成窗
> 原因：TS 5,964L → Go 91L（2%），工作量巨大，与 G-W1 杂项性质不同
> 参考：`production-audit-s3.md` + `deferred-items.md` NEW-7

- [ ] **G-W2-T1**: LINE Messaging API 集成
  - 当前: `channels/line/bot_message_context.go`（骨架，91L）；TS: **34 文件**（`src/line/`）
  - ⚠️ **审计复核修正**：TS 源目录为 `src/line/`（非 `src/channels/line/`），实际 34 文件（含测试），非测试文件约 21 个
  - SDK: `line-bot-sdk-go/v8` 或直接 REST API
  - [ ] 消息接收/发送基础流程 + Webhook 签名校验
  - [ ] 文字/图片/贴图消息类型处理 + 群组/私聊路由

- [ ] **G-W2-T2**: LINE 频道注册到频道路由
  - [ ] 注册到 `channels/` 核心路由层 + 集成 autoreply 管线

- [ ] **G-W2 验证**：`go build ./internal/channels/line/... && go test -race ./internal/channels/line/...`

---

## 附录 A：风险点汇总（审计复核识别）

| 风险 | 位置 | 处理方式 |
|------|------|---------|
| G-W1 工作量严重低估 | LINE channel ~5,964L | ✅ 已拆出为 G-W2 |
| D-W1b protocol/ 工作量不确定 | protocol.go 270L vs TS 2,800L | ✅ 已加前置侦察步骤 |
| B-W2 forwarder 状态不明 | exec-approval-forwarder.ts ⚠️部分 | ✅ 已加前置侦察步骤 |
| D-W0 requestJSON 注入点复杂 | gateway 启动序列 | ⚠️ 执行时评估实际工作量 |
| agents/schema/ 归属 | 原在 D-W2，工具类型基础 | ✅ 已移入 A-W1 |
| A 组与 B 组可并行 | 无依赖关系 | ✅ 已在总览注明 |
| G-W1-T2 依赖 A-W3b | bash 命令处理器需要执行引擎 | ✅ 已标注依赖关系 |
| WS 断连优先级 | 可能影响调试效率 | ⚠️ A组完成后重新评估 |

---

## 附录 B：显式延迟到下一轮的项目

> **审计复核备注**：以下项目在 gap-analysis-part2 中记录为遗漏项，经确认**有意不纳入本轮（Phase 13）**，延迟到 Phase 14+。

| ID | 内容 | 优先级 | 来源 | 延迟原因 |
|----|------|--------|------|----------|
| P11-1 | **Ollama 本地 LLM 集成** — 本地模型推理支持 | P3 | `deferred-items.md` | 非核心路径，TS↔Go 对齐优先 |
| P11-2 | **前端 i18n 全量抽取** — 国际化字符串提取 | P3 | `deferred-items.md` | 前端专属，后端重构无关 |
