---
document_type: Deferred
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: N/A
skill5_verified: true
---

# 持久沙箱 Worker — 延迟项

> Phase 1-3 + Phase 4.1 审计已完成并归档。以下为尚未完成的后续工作。
> 来源: `docs/claude/tracking/impl-plan-persistent-sandbox-worker.md` (已归档)

## 延迟项清单

### 高优先级

- [ ] **4.2 CI 集成**
  - 更新 `.github/workflows/oa-sandbox-ci.yml`
  - 添加 Worker 集成测试到各平台 CI job (macOS/Linux/Windows)
  - 需要设置 `OA_CLI_BINARY` 环境变量指向编译后的 CLI 二进制

- [ ] **4.3 文档更新**
  - README/API docs 更新（Worker 模块文档）
  - Go 层集成文档（NativeSandboxBridge 使用说明）

### 中优先级

- [ ] **Workspace 动态设置**
  - 当前 `boot.go` 用 `os.TempDir()` 作为默认 workspace
  - 需由 AttemptRunner 按会话动态设置 workspace 路径

- [ ] **Go 单元测试**
  - `native_bridge_test.go` — mock Worker 进程测试 IPC/健康检查/崩溃恢复

- [ ] **运行 Benchmark**
  - `OA_CLI_BINARY=... cargo bench --bench worker_bench` 获取实际 IPC 延迟数据
  - 验证 <1ms IPC 延迟目标

- [ ] **长时间运行内存测试**
  - 验证 Worker 进程不会内存泄漏（Skill 5 验证项 #6，Phase 3 未完成）

### 低优先级

- [ ] **2.4 Docker Worker 模式**
  - `docker run` 启动持久容器 + `docker exec` 替代 `docker run`
  - 适用于需要 Docker 隔离但想减少启动开销的场景

- [ ] **Linux/Windows Worker 沙箱实现**
  - 当前为占位 stub（打印 warning 继续）
  - Linux: 实现 Landlock + Seccomp `pre_exec` setup
  - Windows: 实现 Job Object + Restricted Token `CREATE_SUSPENDED` 模式
  - 需各平台 CI 验证

## 已完成项（归档参考）

| Phase | 状态 | 归档位置 |
|---|---|---|
| Phase 1: Worker 核心 | ✅ 完成 | `docs/claude/tracking/impl-plan-persistent-sandbox-worker.md` |
| Phase 2: 平台集成 | ✅ 完成 | 同上 |
| Phase 3: CLI + Go 集成 | ✅ 完成 | `docs/claude/tracking/impl-phase3-cli-go-integration.md` |
| Phase 4.1: 安全审计 | ✅ PASS | `docs/claude/audit/audit-2026-02-25-persistent-worker-phase3.md` |
