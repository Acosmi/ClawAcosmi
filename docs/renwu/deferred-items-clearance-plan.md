# 延迟项清除计划 — 量身定制版

> 制定日期：2026-02-23 | 基于联网可信源验证 + 国际大厂/顶级开源项目借鉴
>
> 本文档仅为计划报告，不含代码执行。

---

## 一、总体策略

将 25 项延迟待办按 **3 个 Sprint** 分批清除：

| Sprint | 聚焦 | 项数 | 预估工时 |
|--------|------|------|----------|
| **S1** | P2 核心流程修复（向导/权限/LLM） | 4 项 | ~6h |
| **S2** | P2 体验优化 + P3 基础设施 | 8 项 | ~12h |
| **S3** | P3 长尾补全（Playwright/infra/TUI） | 13 项 | ~30h |

**设计原则**：

- 每项方案均经联网可信源验证
- 借鉴 VS Code、Gravitee、Mozilla、TailScale 等大厂实践
- 所有方案量身适配 OpenAcosmi 的 Go 后端 + TS 前端架构

---

## 二、Sprint 1 — P2 核心流程修复（~6h）

### S1-1: PERM-POPUP-D1 权限弹窗 WebSocket 广播

> **可信源**：VS Code Extension Security Model — 3 层审批粒度（单次/会话/永久）

**借鉴**：VS Code 的 Trusted Domains 模式 + Roo Code 的 auto-approve 机制

**量身方案**：

```
attempt_runner.go                  server.go                    前端
  OnPermissionDenied 回调  ──→  WebSocket 广播事件  ──→  showPermissionPopup()
  (通过 AttemptParams           { type: "permission_denied",    3 层审批 UI
   的事件 channel)                tool, detail, level, runId }
```

**具体步骤**：

1. `attempt_runner.go`：为 `ToolExecParams.OnPermissionDenied` 注入实际回调，通过 `AttemptParams` 的事件 channel 发送 `permission_denied` 事件
2. `server.go`：在 dispatch 循环中消费该 channel，广播 WebSocket 事件
3. `app-gateway.ts`：监听 `permission_denied` 事件，调用已就绪的 `showPermissionPopup()`
4. 测试：手动发送需要 L2 权限的 bash 命令，验证弹窗弹出 + 3 层审批逻辑

**预估**：2h

---

### S1-2: GW-WIZARD-D2 简化向导缺少后续配置阶段

> **可信源**：Gravitee API Gateway — 渐进式配置披露（Progressive Disclosure）

**借鉴**：

- Gravitee：将 gateway 配置版本化，支持分阶段渐进配置
- Formbricks/Guidejar：向导完成后提供"继续高级配置"引导链接

**量身方案**：

- 保留当前 4 步简化向导作为快速启动路径
- 在简化向导完成页面添加「进入高级配置」按钮
- `RunOnboardingWizardAdvanced`（12 阶段）通过 UI 的 Security Tab 或单独入口触发
- 不改变首次体验的简洁性

**预估**：1.5h

---

### S1-3: GW-WIZARD-D1 Google provider 缺少 OAuth 模式

> **可信源**：Google AI Studio OAuth 文档 + OpenAI Go SDK v3 最佳实践

**量身方案**：

- 在 `getWizardProviders()` 中为 Google provider 增加认证模式选择步骤
- 方案 A（API Key）保持不变
- 方案 B（OAuth）：引导用户完成 Google Cloud Console 授权流程，存储 refresh token
- 参考 TS 端 `google-antigravity-auth` 扩展的逻辑移植

**预估**：1.5h

---

### S1-4: GW-LLM-D1 多 LLM provider content 字段兼容性

> **可信源**：Mozilla `any-llm-go` 统一接口模式 + Medium 多 provider tool calling 对比分析

**借鉴**：

- `any-llm-go`：规范化 streaming、error 语义、feature support 为一致接口
- Gemini tool calling 需要特殊的 `parts` 结构 + 参数解析

**量身方案**：

- 为 `anthropic.go`、`gemini.go` 编写 tool call message 格式的端到端测试用例
- 参照 `openai.go` 已修复的 `*string` content 字段模式，检查其他 client 是否有相同问题
- 在 `types.go` 中统一 `Message.Content` 序列化接口，确保 string/array 双模式兼容
- 利用现有 `client_test.go` 和 `gemini_test.go` 扩展覆盖

**预估**：1h

---

> Sprint 1 完成后 → P2 项从 4 项降至 0 项

## 三、Sprint 2 — P2 体验优化 + P3 基础设施（~12h）

### S2-1: GW-TOKEN-D1 Gateway 首次启动自动生成 token (P2)

> **可信源**：AWS Storage Gateway 首次配置 + Gravitee 零配置本地启动

**量身方案**：采用方案 C 为主 + 方案 A 为辅

- `--dev` 模式 + `localhost` 绑定时允许无 token 启动（零摩擦开发体验）
- 非 dev 模式首次启动时自动生成 `crypto/rand` 随机 token 并写入 `~/.openacosmi/config.json`
- 在终端输出 token 值并提示用户保存

**预估**：1h

---

### S2-2: GW-UI-D1 前端 chat 失败时显示错误弹窗 (P2)

> **可信源**：VS Code Notification API — 非侵入式通知 + 分级展示

**量身方案**：

- 监听 `chat.send` 的 error response（`res.ok === false`）
- 使用 toast 组件展示错误摘要（不打断用户流）
- 点击 toast 可展开完整错误详情
- 参考现有 `permission-popup.ts` 的模式创建 `error-toast.ts`

**预估**：1.5h

---

### S2-3: SANDBOX-D1 recreate 过滤 browser 容器 (P3)

**量身方案**：

- 在 `cmd_sandbox.go` 的 `--session`/`--agent` 过滤逻辑中，增加对 browser 容器的遍历
- 参照 TS `fetchAndFilterContainers()` 逻辑，同时获取 sandbox + browser 容器列表
- 仅在用户未显式指定 `--browser` 时生效

**预估**：1h

---

### S2-4: SANDBOX-D2 explain 支持 session store 查询 (P3)

**量身方案**：

- 引入 `internal/config/sessions` 模块的读取接口
- 实现 session key → channel 推断链（`normalizeExplainSessionKey` → `inferProviderFromSessionKey` → `resolveActiveChannel`）
- 在 explain 输出中追加 channel 上下文信息

**预估**：2h

---

### S2-5: W5-D1 Windows 进程检测优化 (P3)

> **可信源**：Microsoft `GetProcessTimes` API + Stack Overflow PID reuse detection 最佳实践

**借鉴**：Windows 进程精确识别的业界标准方案 = PID + CreationTime 组合唯一键

**量身方案**：

- 使用 `golang.org/x/sys/windows` 包
- `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` 获取句柄
- `GetProcessTimes()` 获取 `creationTime`，与锁文件中记录的 `startTime` 比对
- 匹配 → 进程存活；不匹配 → PID 已被复用，清理旧锁
- 移除当前 `tasklist` 方案，消除 fork 开销

**预估**：2h

---

### S2-6: HIDDEN-4 npm 包核心黑盒行为等价缺失 (P3)

> **可信源**：各领域推荐 Go 库

**量身方案**（3 个子项）：

| 子项 | TS 来源 | Go 推荐库 | 行动 |
|------|---------|-----------|------|
| `@mozilla/readability` | 网页内容提取 | `codeberg.org/readeck/go-readability/v2` | 引入库，封装 `HTMLToReadable()` 函数 |
| `bonjour-service` | mDNS 注册 | `github.com/grandcat/zeroconf` | 在现有 `bonjour.go`（223L）中填充实际 mDNS 注册逻辑 |
| `iso-639-1` | 语言代码映射 | 内建 map 常量（~180 条） | 创建 `internal/i18n/iso639.go` 静态映射表 |

**预估**：3h

---

### S2-7: HIDDEN-8 平台特定隐藏依赖 (P3)

> **可信源**：`aymanbagabas/go-nativeclipboard` (纯 Go，无 Cgo)

**量身方案**：

| 子项 | 方案 |
|------|------|
| macOS/Linux clipboard | 引入 `go-nativeclipboard`，纯 Go 实现，支持 text + image |
| macOS Homebrew 路径 | `exec.LookPath("brew")` + `brew --prefix` 解析 |
| Windows WSL 检测 | 读取 `/proc/version` 检查 `Microsoft` 关键字 |

**预估**：1.5h

---

### S2-8: W6-D1 TUI 渲染主题色彩微调 (P3)

**量身方案**：

- 对照 TS `syntax-theme.ts` 色彩表逐项拉平 `theme.go` 中的 Glamour 主题配色
- 重点校准 ANSI 色号映射和代码块高亮处理
- 纯视觉调整，无功能影响

**预估**：1h

---

> Sprint 2 完成后 → P2 清零，P3 从 21 项降至 13 项

## 四、Sprint 3 — P3 长尾补全（~30h，可拆分多窗口）

### S3-1: PHASE5-3 Gateway TailScale + mDNS 集成 (~8.5h)

> **可信源**：TailScale MagicDNS 官方文档 + `grandcat/zeroconf` + `mscheidegger/minidisc`

**借鉴**：

- TailScale：MagicDNS 处理 tailnet 内设备名解析，无需额外 mDNS
- Minidisc：TailScale 网络专用的零配置服务发现，gRPC/REST 服务注册
- `grandcat/zeroconf`：LAN 内 mDNS/DNS-SD（Bonjour/Avahi 兼容）

**量身方案**：

- **TailScale CLI 集成**：通过 `exec.Command("tailscale", "status", "--json")` 获取 tailnet 信息
- **Bonjour mDNS**：在现有 `bonjour.go`（已有 223L 框架）中引入 `grandcat/zeroconf`，实现 `BonjourRegistrar` 接口的实际注册逻辑
- **双模式**：LAN 模式用 zeroconf，TailScale 模式用 MagicDNS
- **2 个窗口**：窗口 1 做 TailScale CLI，窗口 2 做 mDNS

---

### S3-2: PHASE5-4 infra 剩余 50+ 缺失文件 (~18h)

**量身方案**：按优先级分 3 批

| 批次 | 文件 | 优先级依据 |
|------|------|-----------|
| 批次 A | `errors.ts` → error types, `exec-host.ts` → host exec | 被多处引用 |
| 批次 B | `ssh-config.ts`, `update-check.ts` | 远程功能依赖 |
| 批次 C | 其余长尾工具函数 | 按使用频率排序 |

**预估**：每批 ~6h，共 3 批

---

### S3-3: PHASE5-5 Playwright AI 浏览自动化 (~16h)

> **可信源**：`playwright-community/playwright-go` + Playwright MCP Protocol + Google DeepMind Project Mariner

**借鉴**：

- Playwright MCP：AI Agent 可通过 MCP 生成、执行、修复测试
- Playwright Go：支持 Chromium/Firefox/WebKit 的统一 API
- 需外挂 Node 环境运行 Playwright 浏览器引擎

**量身方案**（3 窗口）：

1. **窗口 1**：引入 `playwright-go`，实现基础 40+ 操作封装
2. **窗口 2**：实现 AI 引导浏览循环（截图 → Vision API 识别 → 点击/输入）
3. **窗口 3**：与现有 CDP 浏览器功能对接，提供统一的 browser tool 接口

---

### S3-4: W-FIX-7 OpenResponses 多模态支持（3 子项）

> **可信源**：OpenAI Responses API 官方文档 + Go SDK v3

**量身方案**：

| 子项 | 内容 | 方案 |
|------|------|------|
| OR-IMAGE | 图像输入提取 | 支持 URL / Base64 / file_id 三种模式，对齐 TS `extractImageContentFromSource` |
| OR-FILE | PDF/文件输入 | 支持 `input_file` 类型，调用 Files API 获取内容 |
| OR-USAGE | 实时 usage 聚合 | 替换 `emptyUsage()` 占位，从 `agentEvent` 收集 token usage 嵌入 `response.completed` |

**预估**：~4h

---

### S3-5: HEALTH-D4 图片工具缩放和转换 (P3)

**量身方案**：

- 引入 `github.com/disintegration/imaging` 库
- 在 `image_tool.go` 中实现 resize、crop、format conversion
- 支持 JPEG/PNG/WebP 互转

**预估**：~2h

---

### S3-6: HEALTH-D6 LINE 渠道 SDK 决策 (P3)

**现状**：功能已完整实现（16 个 Go 文件），仅未使用 `line-bot-sdk-go`
**量身方案**：

- **保持现状**（推荐）：当前直接 HTTP API 调用功能等价
- **可选**：后续如需 LINE API v3 新特性，再引入 SDK
- 标记为「设计决策 — 已确认」

**预估**：0h（仅文档标记）

---

### S3-7: GW-UI-D3 Vite 代理 ECONNREFUSED 静默 (P3)

**量身方案**：

- 在 `vite.config.ts` proxy 配置中添加 `configure` 钩子
- 捕获 `ECONNREFUSED` 错误，降级为 debug 日志
- 不影响 Gateway 重启后的自动恢复

**预估**：0.5h

---

## 五、全景时间线

```
Sprint 1 (P2 核心)     ████████████████░░░░░░░░░░░░░░  ~6h   → P2 清零
Sprint 2 (P2+P3 基础)  ░░░░░░░░████████████████████░░  ~12h  → P3 降至 13 项
Sprint 3 (P3 长尾)     ░░░░░░░░░░░░░░░░████████████████  ~30h → 全部清零
                       ──────────────────────────────────
                       总计 ~48h（约 6 个全天工作日）
```

## 六、可信源引用汇总

| 领域 | 可信源 | 借鉴要点 |
|------|--------|---------|
| 权限审批 | VS Code Extension Security | 3 层审批粒度 + Workspace Trust |
| 向导配置 | Gravitee API Gateway | 渐进式披露 + 版本化配置 |
| 多 LLM 兼容 | Mozilla any-llm-go | 统一接口规范化 provider 差异 |
| Windows 进程 | Microsoft GetProcessTimes API | PID + CreationTime 组合唯一键 |
| mDNS/服务发现 | grandcat/zeroconf + TailScale MagicDNS | LAN 用 zeroconf，tailnet 用 MagicDNS |
| 浏览器自动化 | Playwright Go + MCP Protocol | AI Agent 截图→识别→操作循环 |
| 内容提取 | readeck/go-readability/v2 | Mozilla Readability.js 的 Go 移植版 |
| 剪贴板 | aymanbagabas/go-nativeclipboard | 纯 Go 跨平台，无 Cgo 依赖 |
| 图像处理 | disintegration/imaging | Go 原生图片缩放/转换 |
| Responses API | OpenAI Go SDK v3 | input_image / input_file / usage 聚合 |

## 七、验证策略

每个 Sprint 完成后执行：

1. **单元测试**：`go test ./backend/...` 确保无回归
2. **TS↔Go 对照审计**：对修改的每个文件进行 TS 源对照，确认逻辑等价
3. **端到端验证**：启动 gateway + UI，手动走完关键流程
4. **deferred-items.md 更新**：将已完成项移入 `deferred-items-completed.md` 归档
