# 主动消息机制与任务看板：架构审计与行业验证

基于对 OpenAcosmi 系统的深度代码级审计，以及针对所提供行业报告的网络调研验证，以下是我们的发现总结及已验证的落地升级路线。

## 1. 代码级架构审计（当前状态）

我们对核心的网关 (gateway)、广播 (broadcasting) 和后台任务 (background job) 组件进行了细粒度的审查。令人惊喜的是，系统其实已经具备了支持这些新功能的良好基础。

### 已识别的核心优势

- **强大的事件广播机制 (`broadcast.go`, `ws_server.go`)**：系统拥有一个成熟的 `Broadcaster`，能够根据 `sessionKey` 和权限向特定客户端推送实时事件（如 `chat.delta`、`agent` 工具调用和 `channel.message.incoming`）。将“任务状态”推送到前端看板的基础已经完全具备。
- **后台 Cron 引擎 (`cron/timer.go`, `cron/isolated_agent.go`)**：OpenAcosmi 已经拥有一个自定义的 Cron 服务，能够执行后台的 Agent 任务 (`SessionTargetIsolated`)。它在本地处理了深度的复杂性：故障退避、卡死任务超时（清理），最重要的是，它已经包含了**自动将结果投递**到特定频道的逻辑（通过 `RunSubagentAnnounceFlow`）。这证明系统已经为异步后台处理做好了准备。
- **任务频道抽象 (`server_methods_chat.go`)**：在正常的 `chat.send` 期间，如果智能体调用了工具，系统会自动懒加载创建一个 `taskSessionKey` (`task:<runID>`)，并将工具状态（`[工具]`、`[结果]`）广播到一个内部的 `task` 频道。这本质上已经是一个运行中的原型看板系统。

### 架构差距（缺失环节）

- **动态任务入队**：目前，长时间运行的智能体任务要么是同步的（在 `chat.send` 中阻塞用户），要么是刚性的定时 cron 任务。我们缺乏一个 `TaskManager` 队列，让用户可以说“执行这个大型研究任务”，然后系统立即返回 ACK（确认）并将工作转移到后台（等同于 LinkedIn 的异步模型）。
- **主动唤醒**：目前的 `cron` 任务是按照固定时间表触发的。为了匹配“睡眠计算器 (Sleep Calculator) / 决策引擎 (Decision Engine)”模式，智能体需要能够根据其推理动态地设定自己的下一次执行时间。

---

## 2. 行业最佳实践验证

我们进行了深度的实时网络调研，将原始报告中的各项声明与顶级科技公司和框架进行了交叉对比。这些概念得到了高度验证：

1. **OpenAI Agents SDK (睡眠与决策)**：✅ 已验证。高级自主智能体利用决策引擎来支配自己的执行循环，并在复杂步骤之间设定“睡眠”间隔，以智能地管理上下文和成本。
2. **Microsoft Copilot Studio (事件驱动的主动性)**：✅ 已验证。Copilot 使用 Power Automate 基于外部事件在 Teams 中触发主动的自适应卡片 (Adaptive Cards)。*契合度：* OpenAcosmi 的 `isolated_agent.go` 投递系统完美镜像了这种能力；我们只需要扩展外部事件触发器即可。
3. **LangGraph (环境智能体与流式传输)**：✅ 已验证。LangGraph 大力推崇在后台持续运行的“环境智能体 (Ambient Agents)”，利用自定义的流式事件（`"custom"`, `"updates"`）向用户保持状态通知。*契合度：* 我们的 `Broadcaster` 与这种模式是一对一映射的。
4. **Slack (事件驱动架构)**：✅ 已验证。行业标准是将触发器 (Events API) 与执行 (SQS/队列) 解耦。
5. **CrewAI & KaibanJS (任务编排)**：✅ 已验证。KaibanJS 利用字面意义上的看板 (Kanban) 方法，允许用户将 AI 智能体执行任务的过程进行可视化（待办 -> 进行中 -> 完成）。*契合度：* 这完全验证了我们构建专用 UI 看板的计划。
6. **LinkedIn Hiring Assistant (异步智能体)**：✅ 已验证。LinkedIn 的招聘助手纯粹以异步方式运行，在后台寻找候选人，并在找到高度匹配的候选人时主动通知招聘人员。*契合度：* 这正是我们想要实现的精准 UX（用户体验）。

---

## 3. 已验证的升级路线

初始报告中提出的架构非常扎实，并且与当前的 OpenAcosmi 代码库完美契合。

**推荐的实施步骤：**

1. **后端任务管理器扩展**：我们不应该从零开始构建一个全新的 `TaskManager`，而是**扩展功能强大的 `cron` 引擎**以支持“一次性异步任务 (One-Off Asynchronous Tasks)”。这可以复用我们现有的超时、退避和状态管理机制。
2. **新增 WebSocket 事件**：扩展 `events.go` 以提供类似 LangGraph 风格的后台环境流式传输：`task.queued`（入队）, `task.started`（开始运行）, `task.progress`（进度更新）, `task.completed`（完成）, `task.failed`（失败）。
3. **聊天管线修改 (`server_methods_chat.go`)**：修改 `chat.send` 处理器。如果一个命令被标记为“长任务”，则立即返回一个 ACK（确认），并将 Payload 推入增强后的后台任务队列中，而不是同步执行 `autoreply` 聊天管线。
4. **前端看板集成 (`gateway.ts`)**：更新前端的 `GatewayBrowserClient`，以监听新的 `task.*` 事件，并将状态映射到新的前端 React/Vue 看板 UI 组件中。
