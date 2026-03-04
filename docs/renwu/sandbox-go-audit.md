# Phase 2: 沙箱隔离 — Go 实现审计

## 总览

- Go 目录: `backend/internal/sandbox/` — **8 文件, 2266 行**
- TS 参考: `src/agents/sandbox/` — 12 文件, 1848 行
- 覆盖率: **122%** (Go 超越 TS, 含容器池+WASM)

## 文件映射

| Go 文件 | 行数 | 已实现逻辑 |
|---------|------|-----------|
| `sandbox_worker.go` | 583 | 任务调度器: TaskStore内存存储/TaskExecutor策略接口/Worker工作池(goroutine)/Submit提交/取消/超时/进度广播 |
| `container_pool.go` | 445 | Docker容器池: 预热warm/Acquire获取/Release归还/Destroy销毁/端口分配/健康检查/超时清理/状态查询 |
| `docker_runner.go` | 310 | Docker执行器: 实现TaskExecutor/docker exec/stdout+stderr流式捕获/超时控制/进度回调 |
| `wasm_runner.go` | 130 | WASM执行器: 实现TaskExecutor/wazero运行时/轻量沙箱(TS无此能力) |
| `ws_progress.go` | 160 | WebSocket进度推送: ProgressHub/客户端订阅/事件广播/断线清理 |
| `sandbox_test.go` | 165 | Worker/TaskStore 单元测试 |
| `container_pool_test.go` | 218 | 容器池 Acquire/Release/Cleanup 测试 |
| `sandbox_integration_test.go` | 105 | Docker 集成测试 |

## 外围集成文件

| Go 文件路径 | 行数 | 逻辑 |
|------------|------|------|
| `cmd/openacosmi/cmd_sandbox.go` | 590 | CLI: `oa sandbox run/status/list/stop/cleanup` |
| `internal/gateway/server_methods_sandbox.go` | — | Gateway HTTP: 提交/状态/WebSocket端点 |
| `internal/agents/sandbox/sandbox_test.go` | — | Agent级沙箱配置测试 |
| `internal/autoreply/reply/stage_sandbox_media.go` | — | 沙箱媒体文件暂存 |
| `pkg/types/types_sandbox.go` | 49 | 沙箱配置类型定义 |

## Go 超出 TS 的能力

| 能力 | Go | TS |
|------|----|----|
| 容器池预热 | ✅ ContainerPool | ❌ 每次新建 |
| WASM 沙箱 | ✅ wasm_runner | ❌ 仅 Docker |
| WebSocket 进度 | ✅ ProgressHub | ❌ 轮询 |
| 工作池并发 | ✅ Worker goroutine | ❌ 单线程 |
