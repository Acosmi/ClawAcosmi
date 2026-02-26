# W4 审计报告：auto-reply + cron + daemon + hooks

> 审计日期：2026-02-19 | 审计窗口：W4
> **修复完成日期**：2026-02-21 | W4 全量补全修复 — P0/P1 全部清零

---

## 各模块覆盖率

| 模块 | TS文件 | Go文件 | 文件覆盖率 | 行覆盖率 | 原评级 | 修复后评级 |
|------|--------|--------|-----------|---------|--------|-----------|
| auto-reply | 121 | 90 | ~96% | ~95% | **C** | **A** ✅ |
| cron | 42 | 19 | ~86% | ~98% | **A** | **A** ✅ |
| daemon | 30 | 22 | ~95% | ~94% | **C** | **A** ✅ |
| hooks | 40 | 18 | ~90% | ~94% | **B** | **A** ✅ |

---

## AUTO-REPLY 模块

### 逐文件对照（关键差异）

顶层文件：src/auto-reply/ 与 backend/internal/autoreply/ 对应关系良好，25 个核心文件均已迁移。

**reply/ 子目录（全部修复）：**

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| directive-handling.impl.ts | directive_handling_impl.go (392L) | ✅ **W4-02 已修** |
| directive-handling.auth.ts | directive_handling_auth.go (319L) | ✅ **W4-02 已修** |
| directive-handling.fast-lane.ts | directive_handling_fast_lane.go (160L) | ✅ **W4-02 已修** |
| stage-sandbox-media.ts | stage_sandbox_media.go (278L) | ✅ **W4-03 已修** |
| untrusted-context.ts | untrusted_context.go (41L) | ✅ **W4-04 已修** |
| streaming-directives.ts | streaming_directives.go (227L) | ✅ **W4-05 已修** |
| exec/directive.ts | exec_directive.go (219L) | ✅ **W4-01 语义修复（takeToken 有序扫描）** |
| queue/state.ts | queue_state.go | ✅ 已覆盖（加锁正确） |
| queue/drain.ts | queue_drain.go | ✅ 已覆盖 |
| queue/enqueue.ts | queue_enqueue.go | ✅ 已覆盖 |
| queue/cleanup.ts | queue_cleanup.go | ✅ 已覆盖 |
| inbound-sender-meta.ts | inbound_context.go（合并） | ✅ L12,43 |
| reply-payloads.ts | reply_payloads.go (236L) | ✅ **W4 修复 — 全 5 函数实现** |
| reply-tags.ts | reply_tags.go (42L) | ✅ **W4 修复 — extractReplyToTag 实现** |

**Go 新增文件（无 TS 直接对应，为架构重组）：**

- `queue_command_lane.go`：合并了 TS 的 `process/command-queue.ts` + `process/lanes.ts`
- `model_fallback_executor.go`：提炼自 agent-runner-execution.ts 的模型切换管线
- `queue_helpers.go`：从 TS `utils/queue-helpers.ts` 内联

### 差异清单（AUTO-REPLY 全部修复）

| ID | 问题 | 描述 | 优先级 | 状态 |
|----|------|------|--------|------|
| W4-01 | exec/directive 语义差异 | 修复：Go 使用有序 `takeToken` 扫描，遇非法 token 停止（L33, L179） | P0 | ✅ |
| W4-02 | directive-handling 三文件缺失 | 新建 `directive_handling_impl.go`(392L) + `_auth.go`(319L) + `_fast_lane.go`(160L) | P0 | ✅ |
| W4-03 | stage-sandbox-media 缺失 | 新建 `stage_sandbox_media.go`(278L)，含路径安全校验+SCP远端下载 | P0 | ✅ |
| W4-04 | untrusted-context 缺失 | 新建 `untrusted_context.go`(41L)，含 header 标记+normalize | P0 | ✅ |
| W4-05 | streaming-directives 缺失 | 新建 `streaming_directives.go`(227L)，含跨 chunk 暂存+Consume | P0 | ✅ |

---

## CRON 模块

**评级：A（覆盖率最高的模块）**

### 逐文件对照（全部文件均已对应） ✅

| TS 文件 | Go 对应 | 状态 |
|---------|---------|------|
| isolated-agent/run.ts | isolated_agent.go | ✅ |
| isolated-agent/helpers.ts | isolated_agent_helpers.go | ✅ |
| isolated-agent/session.ts + delivery-target.ts | isolated_agent_helpers.go（合并） | ✅ |
| service/ops.ts | ops.go（提升到顶层） | ✅ |
| service/jobs.ts / locked.ts / timer.ts | 对应顶层 Go 文件 | ✅ |
| parse.ts | parse.go（ISO-8601 解析完整） | ✅ |

### 差异清单

| ID | 问题 | 优先级 | 状态 |
|----|------|--------|------|
| W4-06 | `Start()` 中 `runMissedJobs` 等价调用 | P1 | ✅ `OnTimer` L82 收集 due+missed jobs，`Start` L27 调 `ComputeAllNextRunTimes` |
| W4-07 | TS async 锁 vs Go sync 锁 | P1 | ✅ Go 惯用法 `sync.Mutex` 语义正确 |
| W4-08 | 无协议一致性测试 | P2 | ⏭️ 延迟 |

---

## DAEMON 模块

**评级：A（全部修复）**

### 逐文件对照

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| systemd.ts | systemd_linux.go | ✅ 已覆盖（平台限定） |
| systemd-unit.ts | systemd_unit_linux.go (216L) | ✅ **W4-09 已修** |
| systemd-linger.ts | systemd_linger_linux.go (119L) | ✅ **W4-10 已修** |
| systemd-availability.ts | systemd_availability_linux.go (49L) | ✅ **W4-11 已修** |
| systemd-hints.ts | systemd_hints_linux.go (48L) | ✅ 已修（原 P2） |
| service-runtime.ts | types.go（内嵌） | ✅ |
| service-audit.ts | audit.go | ✅ |
| node-service.ts | node_service.go | ✅ |
| launchd.ts | launchd_darwin.go | ✅ 平台限定 |
| schtasks.ts | schtasks_windows.go | ✅ 平台限定 |

### 差异清单（DAEMON 全部修复）

| ID | 问题 | 描述 | 优先级 | 状态 |
|----|------|------|--------|------|
| W4-09 | systemd-unit 缺失 | 新建 `systemd_unit_linux.go`(216L)，含 build/parse/env render | P0 | ✅ |
| W4-10 | systemd-linger 缺失 | 新建 `systemd_linger_linux.go`(119L)，含 read/enable/isEnabled | P0 | ✅ |
| W4-11 | systemd-availability 缺失 | 新建 `systemd_availability_linux.go`(49L) | P1 | ✅ |

---

## HOOKS 模块

**评级：A（全部修复）**

### bundled hooks 覆盖检查

| TS bundled hook | Go Handler | 实现状态 |
|----------------|------------|---------|
| boot-md/handler.ts | `bootMdHandler` | ✅ 事件通知（boot checklist 由编排层调用，设计一致） |
| command-logger/handler.ts | `commandLoggerHandler` | ✅ **W4 修复 — JSONL 文件写入** |
| session-memory/handler.ts | `sessionMemoryHandler` (127L) | ✅ **W4-12/13 已修 — JSONL读+LLM slug+文件写** |
| soul-evil/handler.ts | `soulEvilHandler` (87L) | ✅ **W4-14 已修 — 连接 soul_evil.go** |

### 其他 hooks 文件

| TS 文件 | Go 文件 | 状态 |
|---------|---------|------|
| install.ts | hook_install.go (12.8KB) | ✅ **W4-15 已修** |
| installs.ts | hook_installs.go (2.1KB) | ✅ **W4-15 已修** |
| plugin-hooks.ts | — | ⏭️ P2 延迟（插件架构设计） |
| llm-slug-generator.ts | llm_slug_generator.go (84L) | ✅ **W4-13 已修** |

### Gmail 子模块 ✅

TS `src/hooks/gmail*.ts` 与 Go `backend/internal/hooks/gmail/`（gmail.go/ops.go/setup.go/watcher.go）对应完整，内容等价。

---

## 隐藏依赖审计

### 1. 全局队列状态并发安全 ✅

- TS：`FOLLOWUP_QUEUES = new Map<string, FollowupQueueState>()`（单线程，无竞争）
- Go：`followupQueues map` + `followupQueuesMu sync.RWMutex`（正确加锁）
- `FollowupQueueState` 内有独立 `mu sync.RWMutex`，双层锁设计正确

### 2. Worker Pool / 命令通道 ✅（含注意点）

- TS：`process/command-queue.ts` Promise 串行链
- Go：`queue_command_lane.go` goroutine + Mutex（`maxConcurrent=1` 保证串行）
- 跨模块循环依赖风险已通过包结构审核排除

### 3. 超时处理 ✅

- TS：`AbortController` + `AbortSignal`
- Go：`context.WithTimeout`（语义等价，惯用法正确）

### 4. session-memory 依赖链 ✅（已修复）

- Go 端 `sessionMemoryHandler` 完整实现：JSONL 读取 → LLM slug 生成 → 文件写入
- `llm_slug_generator.go` 提供 `GenerateSessionSlug` 函数

---

## 总结

| 模块 | P0 | P1 | P2 | 修复后评级 |
|------|----|----|----|-----------|
| auto-reply | 5→**0** | 5→**0** | 3→延迟 | **A** ✅ |
| cron | 0 | 2→**0** | 1→延迟 | **A** ✅ |
| daemon | 2→**0** | 1→**0** | 1→**已修** | **A** ✅ |
| hooks | 2→**0** | 3→**0** | 2→1延迟 | **A** ✅ |
| **合计** | **0** | **0** | **5→延迟** | **全 A** |

### P0/P1 全部清零 ✅

### P2 延迟项（→ deferred-items.md）

| ID | 描述 | 延迟原因 |
|----|------|----------|
| W4-08 | cron 协议一致性测试 | 需 E2E 测试基础设施 |
| W4-P2-hooks | plugin-hooks.ts 无 Go 对应 | 插件架构设计（Phase 13 范围） |
