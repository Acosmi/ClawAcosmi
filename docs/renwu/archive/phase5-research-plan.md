# 阶段五：长期补全 — 实施方案研究报告

> 生成日期：2026-02-19 | 目标：逐项研究 5.1~5.6 的细化实施方案
> 本文档为**方案研究报告**，不含代码实现

---

## 5.1 TUI 终端聊天客户端 — 已独立立项

> **本项已独立为单独项目文档**，详见 [`phase5-tui-project.md`](./phase5-tui-project.md)

| 维度 | 详情 |
|------|------|
| 方案 | bubbletea 完整重写（用户确认：性能优先） |
| 规模 | 12 个 Go 文件 / ~3000L |
| 工时 | ~30h（5-6 个窗口） |
| 依赖 | bubbletea + bubbles + lipgloss（已在 go.mod） |

---

## 5.2 Playwright AI 浏览自动化

### 现状分析

| 维度 | TS 端 | Go 端 |
|------|-------|-------|
| 文件数 | 14 个（pw-ai + pw-session + pw-tools-core 10 子模块） | 0（CDP 基础层有，Playwright 层完全缺失） |
| 操作数 | 40+ 个浏览器操作函数 | 无 |
| AI 循环 | 截图→识别→点击/输入 自动循环 | 无 |

**TS 端核心模块**：

- `pw-ai.ts` — 聚合导出（60L，纯 re-export）
- `pw-session.ts` — Playwright 会话管理（页面创建/切换/关闭）
- `pw-tools-core.ts` — 主入口，40+ 操作分散在 10 个子文件：
  - `.interactions.ts` — click/hover/type/drag/pressKey/fillForm
  - `.snapshot.ts` — snapshotAi/snapshotAria/snapshotRole/screenshotWithLabels
  - `.state.ts` — cookies/storage/设备/地理/离线/时区
  - `.downloads.ts` / `.responses.ts` / `.trace.ts` / `.activity.ts`

### 技术选型

| 方案 | 详情 | 推荐度 |
|------|------|--------|
| **A: playwright-go** | `github.com/playwright-community/playwright-go` v0.52，pre-1.0 但活跃 | ⭐⭐⭐⭐ |
| **B: 纯 CDP 直连** | 已有 CDP 基础层，直接用 `chromedp` | ⭐⭐ |
| **C: 混合方案** | 核心操作用 playwright-go，截图/AI 循环用 CDP | ⭐⭐⭐ |

**推荐方案 A**：直接使用 `playwright-go` ✅ 用户确认：视觉理解执行智能体待接入

**理由**：

1. TS 端使用 Playwright，Go 端用同一抽象层可保证 API 语义 1:1 对齐
2. `playwright-go` v0.52（2025-09）跟随 Playwright v1.49，覆盖全部 40+ 操作
3. pre-1.0 不影响生产使用（API 已稳定）
4. **用户确认**：项目有视觉理解执行智能体（Vision AI Agent）待接入，Playwright 是核心依赖

> [!CAUTION]
> **⚠️ 执行窗口必读 — Node.js 环境配置**
> `playwright-go` 需要 Node.js 运行时驱动 Playwright Server。
> **用户已确认**：Node.js 已安装在本项目之外的文件夹（原 TS 项目目录）。
> **执行前必须**：
>
> 1. 询问用户 Node.js 安装路径
> 2. 将该路径加入 `PATH` 环境变量，或设置 `PLAYWRIGHT_NODEJS_PATH` 指向 `node` 二进制
> 3. 验证：`node --version` 输出正常后再继续

### 实施策略（分 3 阶段）

| 阶段 | 内容 | 文件数 | 预估工时 |
|------|------|--------|----------|
| P1 | 会话管理 + 6 个核心操作（navigate/click/type/screenshot/snapshot/evaluate） | 3 | 6h |
| P2 | 剩余交互操作（hover/drag/fill/select/pressKey/scroll 等 15 个） | 2 | 4h |
| P3 | 高级功能（cookies/storage/trace/download/response 拦截/AI 循环） | 3 | 6h |
| **合计** | | **8 文件** | **~16h（3 个窗口）** |

---

## 5.3 Gemini 流式 client

### 现状分析

| 维度 | 详情 |
|------|------|
| Go llmclient 支持 | anthropic / openai / deepseek / ollama（4 个 provider） |
| Gemini 支持 | **完全缺失** — `StreamChat()` 的 switch 无 `"gemini"/"google"` 分支 |
| TS 端 | Gemini 流式通过 Google AI SDK SSE 实现 |
| Go 官方 SDK | `google.golang.org/genai`（2025-05 GA，生产就绪） |

**关键差异**：Go 端 `client.go:StreamChat()` 的 provider 路由不包含 gemini，所有 Gemini 模型调用会返回 `unsupported provider` 错误。

### 技术选型

| 方案 | 详情 | 推荐度 |
|------|------|--------|
| **A: 官方 genai SDK** | `google.golang.org/genai`，GA 版本，原生支持 SSE streaming | ⭐⭐⭐⭐⭐ |
| **B: 手动 HTTP SSE** | 自行解析 Gemini REST API 的 SSE 响应 | ⭐⭐ |
| **C: OpenAI 兼容模式** | Gemini 支持 OpenAI 兼容端点，复用 openaiStreamChat | ⭐⭐⭐ |

**推荐方案 A**：使用官方 `google.golang.org/genai` SDK

**理由**：

1. 2025-05 已 GA，Google 官方维护，长期稳定
2. 原生支持 `streamGenerateContent`（SSE）和 Live API（WebSocket 双向流）
3. 内置 function calling、safety settings、structured output
4. 避免 OpenAI 兼容层的功能损失（Gemini 特有能力如代码执行等不经 OpenAI 兼容层暴露）

### 实施方案

新建 `backend/internal/agents/llmclient/gemini.go`（~200L）：

- `geminiStreamChat()` — 使用 genai SDK 的 `GenerateContentStream`
- 将 `ChatRequest` 转换为 genai 请求格式（messages → contents）
- SSE 事件映射到统一 `StreamEvent`（text_delta/tool_use/finish）
- 模型映射：`gemini-2.5-flash` / `gemini-2.5-pro` 等
- 在 `client.go:StreamChat()` 添加 `case "gemini", "google":` 分支

**依赖**：`go get google.golang.org/genai`

### 工作量估算

| 内容 | 预估工时 |
|------|----------|
| `gemini.go` 核心实现 | 3h |
| 修改 `client.go` 路由 | 0.5h |
| 单元测试 | 1.5h |
| **合计** | **~5h（1 个窗口）** |

---

## 5.4 Gateway TailScale + mDNS 集成

### 现状分析

| 维度 | TS 端 | Go 端 |
|------|-------|-------|
| `tailscale.ts` | 496L：二进制查找、serve/funnel、whois 缓存、sudo fallback | **完全缺失** |
| `server-tailscale.ts` | 59L：gateway 暴露控制 | **完全缺失** |
| `bonjour.go` | — | 223L：框架完整，但 `BonjourRegistrar` 为 DI 接口，**无真实实现** |
| mDNS 广播 | 注释建议 `grandcat/zeroconf` | 未引入 |

### 技术选型

**Tailscale 集成**：

| 方案 | 详情 | 推荐度 |
|------|------|--------|
| **A: Go 调用 tailscale CLI** | `exec.Command("tailscale", ...)` — 与 TS 端一致 | ⭐⭐⭐⭐ |
| **B: tailscale Go SDK** | `tailscale.com/client/tailscale/v2` — HTTP API 客户端 | ⭐⭐⭐ |
| **C: Rust thin CLI wrapper** | Rust 二进制封装 tailscale 调用，FFI 或子进程调用 | ⭐⭐ |

**用户提问：CLI 用 Rust 性能是否更好？**

**联网验证结论**：

1. **官方 Tailscale CLI 是纯 Go 实现**（`tailscale.com/cmd/tailscale`），没有 Rust 版本
2. 我们调用的是**外部 tailscale 二进制**（exec.Command），不是自己实现 CLI
3. 性能瓶颈在**外部进程启动 + 网络通信**，Go vs Rust 封装层的差异 < 1ms（可忽略）
4. Rust 真正的优势场景是**高频调用/计算密集**，而 tailscale 操作是**低频 I/O**（serve/funnel 仅在 gateway 启动时调用一次）

**结论**：本项 CLI 封装用 Go 即可，Rust 无性能增益。但如果项目后续有 Rust CLI 工具链（如 Phase 10 Rust 迁移），可考虑将 tailscale/mDNS 操作合并进统一的 Rust CLI wrapper。

**推荐方案 A**：Go 调用 CLI（当前最优，与 TS 端行为一致）

**mDNS 集成**：

推荐引入 `github.com/grandcat/zeroconf`（Go 纯实现 mDNS/DNS-SD），实现 `BonjourRegistrar` 接口。

### 实施方案

1. **新建** `backend/internal/infra/tailscale.go`（~250L）：
   - `FindTailscaleBinary()` — 多路径查找（PATH → macOS 已知路径 → find）
   - `GetTailnetHostname()` / `ReadTailscaleStatusJSON()`
   - `EnableTailscaleServe/Funnel()` / `DisableTailscaleServe/Funnel()`
   - `ReadTailscaleWhoisIdentity()` — 含 TTL 缓存
   - `ExecWithSudoFallback()` — 权限不足时 sudo 重试

2. **新建** `backend/internal/gateway/server_tailscale.go`（~60L）：
   - `StartGatewayTailscaleExposure()` — serve/funnel 模式切换 + 退出清理

3. **实现** `BonjourRegistrar`（~80L）：
   - `zeroconfRegistrar` 结构体，实现 `Register()` / `Shutdown()`
   - 引入 `github.com/grandcat/zeroconf` 包

### 工作量估算

| 内容 | 预估工时 |
|------|----------|
| `tailscale.go`（CLI 封装 + whois 缓存） | 4h |
| `server_tailscale.go` | 1h |
| zeroconf Registrar 实现 | 2h |
| 测试 | 1.5h |
| **合计** | **~8.5h（1-2 个窗口）** |

---

## 5.5 OpenResponses /v1/responses 完整实现

### 现状分析

| 维度 | TS 端 | Go 端 |
|------|-------|-------|
| 文件 | `openresponses-http.ts`（915L） | `openai_http.go:HandleOpenAIResponses()`（~90L） |
| 功能 | 完整：auth + 文件/图像输入 + 历史重建 + 工具定义 + 流式 SSE | **仅代理转发**到 chat completions |
| Schema | `open-responses.schema.ts` 完整类型定义 | 无独立 schema |
| 规范 | 对齐 Open Responses 开放规范（2026-01） | 无 |

**关键差距**：Go 端 `HandleOpenAIResponses()` 将 `/v1/responses` 请求简单转换为 chat completions 格式再处理，缺失：

1. **文件/图像输入**（ContentPart 多模态解析）
2. **历史重建**（ItemParam → 对话历史 replay）
3. **工具定义**（extractClientTools + applyToolChoice）
4. **流式 SSE 事件**（response.created/output_item.added/content_part.delta/done）
5. **用量统计** (Usage 结构化返回)

### 技术分析

TS 端 `handleOpenResponsesHttpRequest()` 核心流程（915L）：

1. 认证 → Bearer token 验证
2. 解析 `CreateResponseBody`（含 input/model/tools/tool_choice/instructions 等）
3. `buildAgentPrompt()` — 将 ItemParam[] 历史重建为消息
4. `extractClientTools()` — 提取工具定义
5. 注入 agent pipeline → 订阅流式事件
6. SSE 输出：response.created → output_item.added → content_part.added → delta → done

### 推荐方案：完整重写 `openai_http.go` 的 Responses 部分

**实施分 2 阶段**：

| 阶段 | 内容 | 预估行数 |
|------|------|----------|
| P1 | Schema 类型 + prompt 构建 + 基础非流式 | ~200L |
| P2 | 流式 SSE + 工具调用 + 多模态输入 | ~300L |

**新建/修改文件**：

- **新建** `open_responses_schema.go` — CreateResponseBody/ResponseResource/OutputItem/ContentPart/Usage 类型
- **重写** `openai_http.go` 中 `HandleOpenAIResponses()` — 从代理转发改为原生实现
- **新建** `open_responses_prompt.go` — buildAgentPrompt（历史重建）+ extractClientTools

### 工作量估算

| 内容 | 预估工时 |
|------|----------|
| Schema 类型定义 | 2h |
| Prompt 构建 + 历史重建 | 3h |
| 流式 SSE 协议 | 3h |
| 工具调用 + 多模态 | 2h |
| 测试 | 2h |
| **合计** | **~12h（2 个窗口）** |

---

## 5.6 infra 剩余文件

### 现状分析

| 维度 | 数值 |
|------|------|
| TS infra 文件数（非测试） | ~120 个 |
| Go infra 文件数（非测试） | ~55 个 |
| 文件覆盖率 | ~46%（阶段 1-4 已从 36% 提升） |
| P1 级缺失 | ~15 个文件 |
| P2 级缺失 | ~20 个文件 |

### 分层优先级策略

已完成的阶段 1-4 修复了大部分 P0 缺失。剩余文件按**实际业务影响**分 3 批：

**第一批（P1，阻断部分功能）**：

| TS 文件 | 功能 | Go 实施建议 | 行数估算 |
|---------|------|-------------|----------|
| `ssh-config.ts` | ~/.ssh/config 解析 | 使用 `github.com/kevinburke/ssh_config` | ~60L |
| `update-check.ts` | git/npm 更新检测 | `exec.Command("git", "describe")` + HTTP check | ~80L |
| `exec-host.ts` | 执行宿主抽象 | 接口 + local 实现 | ~50L |
| `transport-ready.ts` | 传输层就绪事件 | channel + sync.WaitGroup | ~40L |
| `errors.ts` | infra 结构化错误 | 自定义 error 类型 | ~60L |
| `retry.ts` + `backoff.ts` | 重试/退避 | `cenkalti/backoff/v4` 或自写 | ~80L |

**第二批（P2，增强功能）**：

| TS 文件 | 功能 | 行数估算 |
|---------|------|----------|
| `clipboard.ts` | 剪贴板操作 | ~40L |
| `machine-name.ts` | 机器名获取 | ~30L |
| `archive.ts` | 文件归档 | ~50L |
| `widearea-dns.ts` | 广域 DNS | ~60L |
| `channel-activity.ts` | 频道活跃度 | ~50L |
| `session-cost-usage.ts` | 会话成本统计 | ~80L |
| `format-time/` | 时间格式化（3 文件）| ~100L |
| `dedupe.ts` | 去重工具 | ~30L |

**第三批（P3，可选）**：

- `brew.ts` / `wsl.ts` / `node-shell.ts` / `binaries.ts` — 平台特定，Go 端可能不需要
- `diagnostic-events/flags.ts` / `restart-sentinel.ts` — 内部诊断

### 推荐策略：批量模板化生产

多数 P2 文件为简单工具函数（30-80L），可在单个窗口中批量处理 5-8 个。

### 工作量估算

| 批次 | 文件数 | 预估工时 |
|------|--------|----------|
| 第一批（P1） | 6-8 | 8h（1-2 窗口） |
| 第二批（P2） | 8-10 | 6h（1 窗口） |
| 第三批（P3） | 5-7 | 4h（1 窗口） |
| **合计** | **~23 个文件** | **~18h（3-4 窗口）** |

---

## 优先级排序总表（修订版）

| 排序 | 项目 | 推荐方案 | 关键依赖 | 预估工时 | 理由 |
|------|------|----------|----------|----------|------|
| **1** | 5.3 Gemini 流式 | 官方 genai SDK | `google.golang.org/genai` | 5h | 工作量最小，解除 Gemini 完全不可用的阻断 |
| **2** | 5.5 OpenResponses | 完整重写 | 无新依赖 | 12h | 生产 API 端点功能严重缺失（仅代理转发） |
| **3** | 5.4 TailScale+mDNS | Go CLI 封装 + zeroconf | `grandcat/zeroconf` | 8.5h | 解除远程访问和局域网发现功能 |
| **4** | 5.1 TUI 完整版 | bubbletea 完整重写 | 已在 go.mod | **30h** | 性能优先，12 文件完整终端聊天客户端 |
| **5** | 5.6 infra 剩余 | 分 3 批 | 各批次不同 | 18h | 长尾补全，多数非核心路径 |
| **6** | 5.2 Playwright | playwright-go | `playwright-go` + Node.js | 16h | 视觉 AI Agent 核心依赖 |

### 总计（修订后）

| 指标 | 数值 |
|------|------|
| 总预估工时 | **~89.5h**（+17h） |
| 预计窗口数 | **15-18 个**（+3） |
| 新依赖 | 3 个（genai / zeroconf / playwright-go） |
| 新 Go 文件 | ~50 个（+10，TUI 增加） |

### 用户反馈汇总

| 项目 | 原方案 | 用户选择 | 变更 |
|------|--------|----------|------|
| 5.1 TUI | C（MVP 800L） | **A（完整 3000L）** | 工时 13h→30h，性能优先 |
| 5.2 Playwright | A（playwright-go） | **A（确认）** | 视觉 AI Agent 接入需要 |
| 5.3 Gemini | A（官方 genai） | **A（确认）** | 无变更 |
| 5.4 Tailscale | A（Go CLI） | **A + Rust 分析** | Go 已足够，Rust 无增益 |

> [!IMPORTANT]
> 建议按排序 1→6 执行。5.3 投入产出比最高建议优先启动。5.1 因升级为完整版，建议拆分为 5-6 个子窗口逐步执行。
