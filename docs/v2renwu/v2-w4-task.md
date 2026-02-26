# V2-W4 实施跟踪清单 (Auto-reply / Cron / Daemon / Hooks)

> 关联审计报告: `global-audit-v2-W4.md`
> 模块评级: **CRON (A) / HOOKS (B→A) / AUTO-REPLY (C+→A) / DAEMON (C→A)**
> 最后更新: 2026-02-20

## 任务目标

基于 V2 深度审计结果，跟踪 W4 大板块的缺口修复情况。

## 实施清单 (待修复验证清单)

### [P0] 阻断级缺陷

- [x] **[W4-09] daemon/systemd-unit 缺失（编译断裂）**: ✅ 交叉审计确认 `systemd_unit_linux.go` (216L) 已全量实现 `buildSystemdUnitContent` + `parseSystemdExecStart` + `parseSystemdEnvAssignment`。`plist_darwin.go` (181L) 也已实现。**非缺失，审计报告误判。**
- [x] **[W4-10] daemon/systemd-linger 缺失**: ✅ 交叉审计确认 `systemd_linger_linux.go` (119L) 已全量实现 `ReadSystemdUserLingerStatus` + `EnableLinger` + `IsLingerEnabled`。**非缺失，审计报告误判。**
- [x] **[W4-02/05] auto-reply/directive 三端管线未移植**: ✅ 交叉审计确认 7 个文件全部存在：`directive_handling_auth.go` (319L) + `directive_handling_fast_lane.go` (160L) + `directive_handling_impl.go` (392L) + `directive_parse.go` + `directive_persist.go` + `directive_shared.go` + `streaming_directives.go`。**非缺失，审计报告误判。**
- [x] **[W4-03/04] auto-reply 沙箱暂存安全边界**: ✅ `untrusted_context.go` (41L) 已实现 `AppendUntrustedContext`。**非缺失，审计报告误判。**
- [x] **[W4-12/13] hooks/session-memory 工具链瘫痪**: ✅ `bundled_handlers.go` 中 `sessionMemoryHandler` (298L) 已完整实现 JSONL 读取 + LLM slug 生成 + Markdown 输出。**非骨架，审计报告误判。**

### [P1] 次要级缺陷

- [x] **[W4-01] exec/directive 扫描语义差异**: ✅ `exec_directive.go` L179 注释确认已修复为 TS 遇错即停语义 (D-1 fix)。
- [x] **[W4-07] cron/status 同步锁差异**: ✅ **确认为合理设计差异**。Go `sync.Mutex` 替代 TS Promise 在 goroutine 模型下是正确选择，无性能风险。不需代码修改。
- [x] **[W4-14] hooks/soul-evil 配置断连**: ✅ **已修复**。`soulEvilHandler` 已接通 `ApplySoulEvilOverride`，完整读取 workspaceDir/hookConfig/bootstrapFiles 上下文并执行替换。

### [补全] 审计新发现缺口

- [x] **[AUDIT-NEW] daemon/systemd-hints**: ✅ 新建 `systemd_hints_linux.go` (47L)，实现 `IsSystemdUnavailableDetail` + `RenderSystemdUnavailableHints`。
- [x] **[AUDIT-NEW] hooks/install pipeline**: ✅ 新建 `hook_install.go` (~310L) + `hook_installs.go` (~70L)，实现 `InstallHookFromDir` + `InstallHookPackageFromDir` + `InstallHooksFromPath` + `RecordHookInstall`。

## 验证结果

- ✅ `go build ./...` — 编译通过
- ✅ `go vet ./...` — 零告警
- ✅ `go test -race ./internal/hooks/...` — 全部通过
- ✅ `go test -race ./internal/daemon/...` — 全部通过

## 隐藏依赖审计与重构验证补充

- [x] 确保处理 NPM 黑盒无引入，保持现有的零黑盒政策。
- [x] 确保 auto-reply 中依赖环境变量和路径的隐式调用继续使用 `os.Getenv` 及沙箱挂载来完成边界控制。
- [x] 审查协程和 `sync.Mutex` 引入处的内存泄露及锁竞争，尤其在群组状态及队列锁区域。

## 最终评级

| 模块 | 原评级 | 新评级 | 说明 |
|------|--------|--------|------|
| **CRON** | A | **A** | 稳定，同步锁差异确认合理 |
| **HOOKS** | B | **A** | soul-evil 已接通，install 管线已补齐 |
| **AUTO-REPLY** | C+ | **A** | 所有 directive/untrusted-context/session-memory 均已全量实现 |
| **DAEMON** | C | **A** | systemd-unit/linger/hints 全部就位 |
