> 📄 分块 01/08 — D-W0 | 索引：phase13-task-00-index.md
>
> **TS 源**：`src/node-host/runner.ts` + `src/infra/exec-approvals.ts`
> **Go 目标**：`backend/internal/nodehost/`

## 窗口 D-W0：P12 剩余项（最先执行）

> 参考：`gap-analysis-part4a.md` D-W0 节
> 范围：3 个 P12 遗留任务，集中在 `nodehost/`
> **完成度：~85%**（2026-02-18 审计）

- [x] **D-W0-T1**: `requestJSON` 实现
  - 文件：`runner.go` — `RequestFunc` 回调注入 + 降级兜底
  - [x] 实现 `requestJSON(method, params, result)` — 委托到注入的 `RequestFunc`
  - [x] 注入 `RequestFunc` 到 `NodeHostService`（`NewNodeHostService` 新增参数）
  - [x] nil 时降级为 `sendRequest`（fire-and-forget + warn log）
  - [ ] ⚠️ 超时处理 — 由调用方 `RequestFunc` 实现自行控制

- [x] **D-W0-T2**: `browser.proxy` 分支
  - 文件：`browser_proxy.go`（~270L 新建）
  - [x] 添加 `case "browser.proxy":` 到 `HandleInvoke`
  - [x] 解码 `BrowserProxyParams`
  - [x] `BrowserProxyHandler` 接口注入 + `SetBrowserProxy()` 方法
  - [x] Profile 白名单 + 默认 profile + `/profiles` 过滤
  - [x] 文件收集 + base64 + MIME 检测
  - [x] 通过 `sendInvokeResult` 返回响应

- [x] **D-W0-T3**: `allowlist` 评估移植
  - 文件：`allowlist_types.go` + `allowlist_parse.go` + `allowlist_eval.go`（~650L 新建）
  - [x] `EvaluateShellAllowlist()` — Shell 命令白名单（管道 + chain）
  - [x] `EvaluateExecAllowlist()` — Argv 命令白名单
  - [x] `RequiresExecApproval()` — 审批判断
  - [x] `RecordAllowlistUse()` / `AddAllowlistEntry()` — 白名单记录
  - [x] `AnalyzeShellCommand()` / `AnalyzeArgvCommand()` — 命令解析
  - [x] `tokenizeShellSegment()` / `splitShellPipeline()` / `splitCommandChain()` — Shell 分词
  - [x] `globToRegExp()` / `matchesPattern()` / `matchAllowlist()` — 模式匹配
  - [x] `isSafeBinUsage()` / `ResolveSafeBins()` — 安全命令判定
  - [x] `detectMacApp()` — **不存在于 TS 源码**（原任务文档误引用，已确认）
  - [x] Windows 分词 — `allowlist_win_parser.go`（`tokenizeWindowsSegment` + `findWindowsUnsupportedToken` + `AnalyzeWindowsShellCommand` + `IsWindowsPlatform`）
  - [x] `ResolveExecApprovalsFromFile()` — `allowlist_resolve.go`（完整 agent/defaults/wildcard 三层合并）
  - [x] `RequestExecApprovalViaSocket()` — `allowlist_resolve.go`（Unix socket IPC + 换行分隔 JSON 协议）
  - [x] `MinSecurity()` / `MaxAsk()` — `allowlist_resolve.go`（安全级别比较函数）

- [x] **D-W0 验证**：`go build` ✅ / `go test -race` ✅ / `go vet` ✅
