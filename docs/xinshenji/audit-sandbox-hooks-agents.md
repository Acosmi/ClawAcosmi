---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/{sandbox, hooks, agents} (~2300+ LOC core)
verdict: Pass with Notes
---

# 审计报告: sandbox/hooks/agents — 执行与钩子层

## 范围

- `backend/internal/sandbox/` — 10 files (沙箱Worker/Docker/Wasm/NativeBridge)
- `backend/internal/hooks/` — 18 files + gmail subdir (webhook钩子系统)
- `backend/internal/agents/` — 19 subdirs (auth/bash/tools/runner/models etc)

## 审计发现

### [PASS] 安全: Worker Panic 恢复 (sandbox_worker.go)

- **位置**: `sandbox_worker.go:225-232`
- **分析**: `runWorker` 使用 `defer recover()` 捕获 panic，避免单个任务 crash 导致整个 worker pool 崩溃。
- **风险**: None

### [PASS] 安全: pip 包名消毒 (sandbox_worker.go)

- **位置**: `sandbox_worker.go:463-471`
- **分析**: `executeDataProcessing` 对用户请求安装的 pip 包名进行字符白名单过滤（`[a-zA-Z0-9_\-.\[\],]`），移除非法字符。防止通过包名注入 shell 命令。
- **风险**: None

### [PASS] 正确性: 混合执行引擎 (sandbox_worker.go)

- **位置**: `sandbox_worker.go:340-354`
- **分析**: `HybridExecutor` 按 taskType 路由: `code_execution`→Docker, `data_processing`→Docker+Python, `code_interpreter`→Docker。每种类型配置独立的内存限制和超时。
- **风险**: None

### [PASS] 正确性: 原生沙箱 IPC 协议 (native_bridge.go)

- **位置**: `native_bridge.go:60-77, 283-331`
- **分析**: `NativeSandboxBridge` 通过 JSON-Lines IPC 与 Rust worker 通信。`Execute` 全程持有 mu 锁保证串行 IPC。支持状态机（init→starting→ready→degraded→stopped）、健康监控（定期 ping）、crash 自动重启（exponential backoff）。
- **风险**: None

### [PASS] 正确性: Worker 优雅关闭 (sandbox_worker.go)

- **位置**: `sandbox_worker.go:186-194`
- **分析**: `Stop()` 先 cancel context → 关闭 channel → `wg.Wait()` 等待所有 worker 退出。保证所有进行中的任务完成后再关闭。
- **风险**: None

### [WARN] 安全: 用户脚本 shell 注入风险 (sandbox_worker.go)

- **位置**: `sandbox_worker.go:477-481`
- **分析**: `executeDataProcessing` 通过 `python3 -c '<escaped_script>'` 在 shell 中执行用户脚本。单引号转义 (`'\''`) 在大多数情况下有效，但依赖 Docker 容器的隔离能力作为最终安全边界。这是可接受的设计——Docker 本身就是安全边界。
- **风险**: Low（受 Docker 隔离保护）

### [PASS] 正确性: Hooks 映射与匹配 (hooks.go)

- **位置**: `hooks.go:64-137`
- **分析**: 钩子系统支持路径匹配（`match.path`）、来源匹配（`match.source`）、动作类型（chat/inject/ignore）、渠道路由（`channel`/`to`）、模板渲染。配置解析完整。
- **风险**: None

### [WARN] 正确性: TaskStore 内存无限增长 (sandbox_worker.go)

- **位置**: `sandbox_worker.go:62-106`
- **分析**: `TaskStore` 使用内存 map 存储所有任务，无 TTL 清理机制。长时间运行后已完成任务会持续占用内存。
- **风险**: Low
- **建议**: 添加已完成任务的 TTL 自动清理（如 1 小时后删除）。

## 总结

- **总发现**: 8 (6 PASS, 2 WARN, 0 FAIL)
- **阻断问题**: 无
- **建议**:
  1. TaskStore 添加 TTL 清理 (Low)
  2. 用户脚本执行依赖 Docker 隔离，已在安全边界内 (Accepted)
- **结论**: **通过（附注释）** — 沙箱执行引擎设计合理，NativeBridge 与 Rust worker 的 IPC 实现健壮。
