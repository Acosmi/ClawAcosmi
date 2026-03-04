# V2-W2 实施跟踪清单 (Agents)

> 关联审计报告: `global-audit-v2-W2.md`
> 评级: **C+** → **B** (核心阻断已解除)

## 任务目标

基于 V2 深度审计结果，集中修复 W2大板块 (Agents) 遗留的跨进程锁、工具幽灵化、网关协议不匹配等阻断性 P0/P1 BUG。

## 待办缺陷实施清单 (P0/P1)

### [P0] 核心链路阻断修复

- [x] **[T-R1] 解决工具幽灵化屏蔽问题** ✅ 2026-02-20
  - **位置**: `tools/registry.go`
  - **修复**: 13 个空桩函数全部接入已有 `Create*Tool()` 构造器，Sessions(5工具) / File / Browser / Canvas / Cron / Gateway / Memory(2工具) / Message / Nodes / TTS / Web(2工具) / Image 全量挂载。
- [x] **[L-01] 补齐 Thinking 签名反射分离** ✅ 审计关闭
  - **位置**: `runner/promote_thinking.go` (178L)
  - **状态**: 代码已完整实现 `SplitThinkingTaggedText` + `PromoteThinkingTagsToBlocks`，支持 `<thinking>/<think>/<thought>/<antthinking>` 多变体。审计漏检。
- [x] **[CP-05] 恢复 Cache TTL 拦截驱逐** ✅ 审计关闭
  - **位置**: `extensions/context_pruning.go` L527-553
  - **状态**: 已实现 `CachedAt` 时间戳 + TTL 硬清除逻辑。审计漏检。
- [x] **[CP-01] 完善 Context Pruning 拦截** ✅ 审计关闭
  - **位置**: `extensions/context_pruning.go` L454-464
  - **状态**: `firstUserIndex` 已实现，系统初始化消息（SOUL.md/USER.md）被完整保护。审计漏检。
- [ ] **[T-01] Gateway 网络底座统一** → 推迟
  - **原因**: 涉及系统架构决策（WS 18789 vs HTTP 5174 双端口设计），需专项架构窗口评审。
  - **已记录**: `deferred-items.md`

### [P1] 并发安全与边缘用例修复

- [x] **[S-01] 跨进程安全防护 (替换 proper-lockfile)** ✅ 2026-02-20
  - **位置**: `auth/filelock.go` (91L 新建) + `auth/auth.go` Save/Update 集成
  - **修复**: 基于 `flock(2)` 的跨进程排他锁，覆盖 `Save()` 和 `Update()` 的完整读-改-写周期。保留 `sync.RWMutex` 进程内保护。
  - **测试**: `filelock_test.go` (63L) — 排他访问 + 无锁解锁安全 + 目录自创建。
- [ ] **[M-01] 支持 Gemini 原生流式分块** → 推迟
  - **原因**: 需大量 Google API 调研和真实 API 凭证测试。
  - **已记录**: `deferred-items.md`

## 延期观察项

- [ ] Runner 中的独立沙箱调用健康度监控
- [ ] Workspace 目录下的读写竞争长期测试监控
