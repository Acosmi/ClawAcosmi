# 全局审计报告 — Node Host 模块

## 概览

| 维度 | TS (`src/node-host`) + (`src/infra/exec-*`) | Go (`backend/internal/nodehost`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 3+ | 14 | 逻辑完全拆片解耦 |
| 总行数 | ~1380+ | ~2795 | 100% 结构覆盖 |

### 架构演进

`nodehost`（节点宿主机）是负责安全执行本地系统命令并代理部分浏览器访问的组件（通过 WS 连接网关）。
在 TypeScript 原版中，核心逻辑高度集中于 `src/node-host/runner.ts` (1309 行) 和 `src/infra/exec-approvals.ts` 中。
在 Go 语言重构版中，这一单体结构被彻底解耦，按职责划分为 14 个文件：

- **执行与监控**: `exec.go`, `runner.go`, `invoke.go`.
- **安全检查**: `allowlist_eval.go`, `allowlist_parse.go`, `allowlist_resolve.go`, `allowlist_win_parser.go`.
- **环境与平台**: `sanitize.go`, `browser_proxy.go`.

## 差异清单

### P3 细微差异：代码组织结构

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| NODE-1 | **Allowlist 解析与 `cmd.exe` 处理** | TS 混杂在 `infra/exec-approvals.ts` 和 `runner.ts` 中，对 Windows `cmd.exe` 转发存在特定逻辑块。 | Go 专门抽象出了 `allowlist_win_parser.go` 和 `allowlist_eval.go`，并在 `runner.go` 中针对 Windows `cmd.exe` 绕路处理维持原判。 | **极佳的解耦重构 (P3)**，提升了可读性和跨平台测试性。无需修复。 |
| NODE-2 | **输出流缓冲区 (`stdout`/`stderr`) 截断** | TS 在 `child.stdout.on('data')` 手动累加 `outputLen`，到达 `OUTPUT_CAP=200_000` 后短路并标记 `truncated=true`。 | Go 封装了自定义的 `capBuffer`（实现了 `io.Writer` 接口），通过 `cmd.Stdout = &stdoutBuf` 自动短路并防爆存。 | **优雅的 Go 惯用法 (P3)**。结果对齐，内存管理更优。无需修复。 |
| NODE-3 | **`system.which` 的执行方式** | 由 `resolveExecutable` 扫描系统的 `PATH` 环境变量并追加平台特定的 `.exe` 扩展。 | 同样移植，完全利用 `os.Getenv("PATHEXT")` 取代了硬编码扩展的 fallback。 | 无需修复。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | `OPENACOSMI_NODE_EXEC_HOST` / `OPENACOSMI_NODE_EXEC_FALLBACK` 继承并使用。且都有对 `DYLD_` / `LD_` 等毒性环境变量的拦截。 | 对齐，安全拦截机制通过 (`sanitize.go`)。 |
| **2. 并发安全** | `cmd.Run()` 是阻塞操作。外面的超时利用传入带有 `context.WithTimeout` 的 `ctx` 取代了 TS 中的 `setTimeout` 并发杀内核进程。 | Go 版本没有并发泄露风险（TS 版可能在主进程忙碌时延迟发送 SIGKILL）。更好。 |
| **3. 第三方包黑盒** | 没有复杂的第三方。纯使用 Go 的 `os/exec`。 | 通过。 |

## 下一步建议

这是本次审计遇到的**重构最彻底、拆分最优雅**的功能模块之一。在高度保留所有业务特性（如代理 Browser Control、检查 Command Allowlist、支持安全截断）的情况下，展现了高质量的代码隔离。
当前节点宿主机状态优良，无需进行任何修复，可直接进行下一项审计任务。
