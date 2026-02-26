> 📄 分块 04/08 — C-W1 + B-W1 | 索引：phase13-task-00-index.md
>
> **C-W1 TS 源**：`src/agents/sandbox/` + `src/security/` → **Go**：`backend/internal/agents/sandbox/` + `backend/internal/security/`
> **B-W1 TS 源**：`src/infra/session-cost-usage.ts` + `src/infra/provider-usage.*.ts` → **Go**：`backend/internal/infra/cost/`

## 窗口 C-W1：沙箱 + 安全补全（2 会话）✅ 完成

> 参考：`gap-analysis-part4e.md` C-W1 节

### 会话 C-W1a：Docker 沙箱核心 ✅

- [x] **C-W1-T1**: Docker 沙箱（16 个 TS 文件 → 6 个 Go 文件）
  - TS 目录: `src/agents/sandbox/` (1,848L)
  - Go 目标: `backend/internal/agents/sandbox/`
  - [x] `types.go` — 类型定义（types.ts + types.docker.ts + constants.ts）
  - [x] `config.go` — 沙箱配置/哈希（config.ts + config-hash.ts + shared.ts）
  - [x] `docker.go` — 容器管理/清理（docker.ts + manage.ts + prune.ts）
  - [x] `context.go` — 运行时状态 + 工具策略（runtime-status.ts + context.ts）
  - [x] `registry.go` — 注册表持久化（registry.ts + workspace.ts）
  - [x] `manage.go` — 容器/浏览器生命周期（manage.ts + browser.ts + prune.ts）
  - 关键函数已实现：`CreateSandbox` / `DestroySandbox` / `PruneSandboxes` / `ResolveSandboxConfig` / `ComputeConfigHash` / `ResolveSandboxRuntimeStatus`

### 会话 C-W1b：security/ 补全 ✅

- [x] **C-W1-T2**: security/ 补全
  - **audit_extra.go 缺失函数 — 全部补全**：
    - [x] `collectSmallModelRiskFindings()` — 已在前序窗口实现
    - [x] `collectPluginsTrustFindings()` — 已在前序窗口实现
    - [x] `collectIncludeFilePermFindings()` — 已在前序窗口实现
    - [x] `collectStateDeepFilesystemFindings()` — 已在前序窗口实现
    - [x] `collectExposureMatrixFindings()` — 已在前序窗口实现
    - [x] `readConfigSnapshotForAudit()` — 本窗口新增
    - [x] `collectPluginsCodeSafetyFindings()` — 已在前序窗口实现
    - [x] `collectInstalledSkillsCodeSafetyFindings()` — 已在前序窗口实现
  - **audit_extra.go 新增辅助函数**：
    - [x] `extensionUsesSkippedScannerPath()` — 本窗口新增
    - [x] `readPluginManifestExtensions()` — 本窗口新增
  - **fix.go 全量重写 (~480L)**：
    - [x] `FixSecurityFootguns()` — 完整编排逻辑
    - [x] `safeChmod()` / `safeAclReset()` — chmod + icacls
    - [x] `applyConfigFixes()` / `setGroupPolicyAllowlist()` — 配置修复
    - [x] `chmodCredentialsAndAgentState()` — 凭证保护
    - [x] `SecurityFixAction` / `SecurityFixResult` 类型
  - [x] 审批决策路径 — 已在 `audit.go` 中集成

- [x] **C-W1 验证**：`go build` ✅ | `go vet` ✅ | `go test ./internal/security/...` ✅

---

## 窗口 B-W1：计费与用量追踪 ✅ 完成

> 参考：`gap-analysis-part4e.md` B-W1 节

- [x] **B-W1-T1**: 会话成本追踪
  - TS: `src/infra/session-cost-usage.ts`
  - Go: `internal/infra/cost/types.go` + `session_cost.go` + `cost_summary.go`
  - [x] 成本类型 + JSONL 日志 + 时间序列
  - [x] Token 计数 + usage summary

- [x] **B-W1-T2**: Provider 用量 API（12 个 TS 文件 → 11 个 Go 文件）
  - [x] 类型系统重写 — `provider_types.go`（对齐 TS `UsageWindow`）
  - [x] 共享工具 — `provider_shared.go`（IDs + ClampPercent + parseISO）
  - [x] 认证解析 — `provider_auth.go`（8 供应商 env + auth-profiles）
  - [x] 路由 + 并发加载 — `provider_fetch.go`
  - [x] Claude/Anthropic 适配器 — `provider_fetch_claude.go`
  - [x] GitHub Copilot 适配器 — `provider_fetch_copilot.go`
  - [x] Google Gemini 适配器 — `provider_fetch_gemini.go`
  - [x] OpenAI Codex 适配器 — `provider_fetch_codex.go`
  - [x] z.ai 适配器 — `provider_fetch_zai.go`
  - [x] MiniMax 适配器 — `provider_fetch_minimax.go`（启发式解析）
  - [x] 格式化输出 — `provider_format.go`

- [x] **B-W1 验证**：`go build` ✅ | `go vet` ✅

### 延迟项（→ deferred-items.md）

- BW1-D1: Sandbox/Cost 单元测试基础设施 (P2)
- BW1-D2: Provider Fetch 外部 API 兼容性 (P2)
- BW1-D3: Provider Auth 模块依赖 (P2)

### 架构文档

- [x] `docs/gouji/sandbox.md` — ✅ 已创建
- [x] `docs/gouji/cost.md` — ✅ 已创建

---
