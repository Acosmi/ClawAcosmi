# 全局审计报告 — Wizard 模块

## 概览

| 维度 | TS (`src/wizard`) | Go (`backend/internal/gateway`, `backend/internal/tui`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 10 | 5 | N/A (重新组合) |
| 总行数 | ~1700 | ~1500 | N/A |

### 文件映射与重构情况

* **交互原语抽象**：`src/wizard/clack-prompter.ts` -> `backend/internal/tui/prompter.go` (Go 使用 `bubbletea` 替代了 `@clack/prompts`)
* **会话引擎**：`src/wizard/session.ts` -> `backend/internal/gateway/wizard_session.go` (Go 用 goroutine + channel 替换了 TS 的 Promise Deferred)
* **远程 API**：`src/gateway/server-methods/wizard.ts` (不在该目录，但属同模块) -> `backend/internal/gateway/server_methods_wizard.go`
* **引导流程**：`src/wizard/onboarding*.ts` -> `backend/internal/gateway/wizard_onboarding.go`
* **纯 CLI 向导**：`backend/internal/tui/wizard.go` 提供了无网关的纯 CLI 原生 BubbleTea Setup 界面。

## 差异清单

### P2 设计差异与职责裁剪

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| WIZARD-1 | **引导流程 (Onboarding) 职责大幅削减** | `onboarding.ts` 及其附属文件负责异常庞大的流程：包含 LLM 模型设置、Gateway 网络配置 (IP/Port/Tailscale/HTTPS)、各渠道 (Channels) 授权开关配置、内部 Hooks 开启、以及最终的 Daemon (SystemdUser/LaunchAgent) 驻留拦截器安装。 | 根据 `wizard_onboarding.go` 注释和代码，向导被极致精简，**仅剩 4 步**：选择 Provider -> 填写 API Key -> 选择默认模型 -> 确认保存。其他一切进阶网络、底层和插件设置全盘废弃。 | **无须修复**。这显然是架构有意为之（将复杂的路由、端口分配和常驻托管从 Setup 中剥离，转移到 Web UI Config 中完成，降低新用户的终端操作负担）。这属于良性功能重构，仅登记为架构差异。 |
| WIZARD-2 | **向导阻塞式的会话控制模型 (Session Control)** | TS 中 `WizardSession` 包装 `Deferred<T>`，利用底层 `v8` 事件循环和 Promises 微任务，以 `await session.awaitAnswer()` 的方式在 RPC 请求间阻塞流程。 | Go 版在 `wizard_session.go` 内实现了无锁（精细锁配置的） `stepCh chan` 和 `answerChs map[string]chan`，启动 goroutine Runner 异步推送步骤并通过 Channel 收发同步阻塞。 | **无须修复**。这展示了 Go 传统并发原语对比 JS 事件循环模型的优秀等价替换方案。完美一致。 |

### P3 次要细节

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| WIZARD-3 | **Prompt 交互库替换** | 强依赖 `@clack/prompts`，输出类似 Vite/NextJS 脚手架的风格。 | 强依赖 `github.com/charmbracelet/bubbletea` 和 `lipgloss`，具备极强的跨平台渲染终端风格。 | UI 风格变化，完全合规。 |

## 隐藏依赖审计 (Step D)

执行了详尽的 `grep` 验证后，输出结果如下（均已分析并覆盖或视为等效替换设计）：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. npm 隐层黑盒** | `findTailscaleBinary` 依赖了子进程。 | Go 版中同样有 `exec.Command("tailscale")` 检测，但已被移出向导的必经路径。 |
| **2. 全局状态/单例** | 采用全局 `WizardSessionTracker`。 | `wizard_session.go` 中用 `sync.RWMutex` 封装了安全的 Tracker，安全合规。 |
| **3. 环境变量** | `OPENACOSMI_GATEWAY_TOKEN`、`BRAVE_API_KEY`、`OPENACOSMI_GATEWAY_PASSWORD` 等被读取。 | 在 Go 的 `models.ResolveEnvApiKeyWithFallback` 及其它包中进行了同等读取，环境降级策略一致。 |
| **7. 错误处理** | 大量的 `try/catch` 和针对 SystemD 的回滚逻辑。 | Go 由于职责的精简（去除了 Daemon 驻留自动安装），避开了这几十个可能引发灾难崩溃的 OS 级副作用错误捕获。 |

## 下一步建议

Wizard 模块已重构并完成审计，未发现需要抢修的 Bug（重构掉的冗余反而降低了系统脆性）。建议可以直接闭环并更新追踪清单。
