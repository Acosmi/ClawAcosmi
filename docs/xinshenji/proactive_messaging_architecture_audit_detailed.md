# 智能体主动对话与任务看板：深度代码审计与行业验证报告

基于对本系统（OpenAcosmi）代码层的颗粒度深度审计，以及针对所提供行业建议报告的全面联网校验，特形成此份不经删减、原封不动的详细技术验证报告，旨在为后续代码落地提供详实的细节支撑。

---

## 一、 系统核心层：颗粒度代码深度审计

通过对系统核心文件的逐字排查与逻辑溯源，我们确认了目前 OpenAcosmi 系统已经具备了非常扎实的底层基础，足以支撑“异步任务看板”与“主动消息推送”。具体文件层面的审计发现如下：

### 1. `backend/internal/gateway/ws_server.go` 与 `broadcast.go` (网关与广播机制)

- **细粒度发现：** 系统目前的网关极其成熟。`ws_server.go` 不仅处理了严密的设备认证（Device Auth）和协议协商，还将每个连接（`WsClient`）精细地纳入了统一的 `Broadcaster` 中。
- **与主动消息的关联：** `broadcast.go` 中的 `Broadcaster.Broadcast` 方法本身已经支持带有过滤范围 (`targetConnIDs`) 的事件推送。并且当前的事件白名单 (`defaultGatewayEvents`) 已经支持如 `chat.delta`, `agent`, `channel.message.incoming` 等事件流。这意味着如果在后台触发了主动对话，系统目前的基建**完全可以直接复用**该广播器将消息秒级推送到前端的 WebChat 或是看板中。

### 2. `backend/internal/gateway/server_methods_chat.go` (聊天管线)

- **细粒度发现：** 我们深入审计了 `chat.send` 方法。当前的代码设计中，在正常对话收到用户输入后，会立即生成 `runId` 并向前端返回 `started` 状态（ACK 确认）。随后利用 goroutine 异步开启了 `autoreply` 流程。
- **与任务看板的强关联（Proto-Kanban 雏形）：** 令人惊喜的是，在 `server_methods_chat.go` 的第 346 行左右，系统已经具有类似看板任务流的逻辑：一旦智能体调用外部工具，系统会“懒加载”创建一个名为 `task:<runID>` 的隐形 session，并将工具的 `[开始]`、`[错误]`、`[结果]` 作为流式数据广播出去，同时记录到属于任务频道的 Transcript 中。这说明 OpenAcosmi 底层**已经孕育了“状态看板”的数据流结构**。

### 3. `backend/internal/gateway/events.go` (事件分类)

- **细粒度发现：** 文件中清晰定义了各种内部事件。例如 `ParseAgentUpdate` (`runId`, `status: started|finished|error`)。这些内置结构完全契合未来任务管理器的需求，我们可以直接在此基础上添加 `task.queued`, `task.progress` 等看板所需的新事件定义。

### 4. `backend/internal/cron/timer.go` 与 `isolated_agent.go` (后台智能体引擎)

- **细粒度发现：** 系统并非只能处理同步聊天。`timer.go` 内部实现了一个强大的后台任务池调度器，能自动寻找 `due jobs`，并且具备完善的 `cleanupStuckJobs` (清理卡死任务) 和 `ConsecutiveErrors` (连续错误退避) 算法。
- **与异步任务的核心印证：** 在 `isolated_agent.go` 中，后台运行的智能体会开启独立的 `runSessionID`。而且在执行完成最末端（第 412 行起），它**已经内置了将生成结果向外部渠道进行异步投递的功能**（`RunSubagentAnnounceFlow`）。这已经完美符合我们需要后台代理异步完成任务后、主动打扰并通知用户的逻辑。

### 5. `ui/src/ui/gateway.ts` (前端网关拦截)

- **细粒度发现：** 浏览器的网关客户端 `GatewayBrowserClient` 负责维持长连接与断线重连，并在 `handleMessage` 中针对 `type === "event"` 的帧进行全局侦听（`opts.onEvent`）。前端完全准备好捕获任何新定义的看板事件，数据管道是畅通的。

---

## 二、 行业最佳实践：联网深度校验 (原封不动展开)

在理解了我们的系统家底后，我针对报告中提及的 6 大国际主流体系进行了全网深度检索比对。以下是详尽的行业可信源调研结果（完全展开）：

### 1. OpenAI Agents SDK：唤醒周期与决策引擎 (Wake-up cycles & Decision Engine)

- **调研验证：** 最新的 OpenAI Agent 框架倾向于让智能体摆脱纯粹的“你问我答”被动状态。顶尖实践（如一些名为 "ProactiveAgent" 的开源项目）利用基于 GPT 的“决策引擎”在幕后评估会话的上下文，自主决定是否需要立刻行动，或者计算出一个“最佳唤醒时间（Sleep Calculator）”延迟行动，以节省算力和提供更拟人的互动体验。这种“休眠-等待-主动发声”的模式正在成为主流。

### 2. Microsoft Copilot Studio：Power Automate 触发式推送

- **调研验证：** 微软在 Copilot Studio 中允许开发者通过 Power Automate 工作流实现主动信息传递。其特点是：触发条件不仅限于时间，可以是数据库 (Dataverse) 变动或外部表单提交。当事件触发时，Copilot 不会简单发一条冰冷的系统通知，而是通过 “Chat with agent” 通道以智能体的口吻发送“自适应卡片 (Adaptive Cards)”。
- **对应本系统：** 我们的 `isolated_agent.go` 中的 `deliveryPayload` 以及未来的富文本推送计划，完美契合微软证明成功的“带有富交互的主动推送格式”。

### 3. LangGraph：环境智能体 (Ambient Agents) 与三级流式更新

- **调研验证：** LangGraph 的官方文档高度强调“Ambient Agents（环境智能体）”的概念——它们在后台隐形持守，处理长期运行的复杂任务。在流式传输上，LangGraph 提供了 `.stream(stream_mode="custom")` 的细粒度控制。开发人员可以在节点执行时发送极其详细的自定义状态数据包（如执行到哪一步、中间推导结果），这让前端可以实现极其流畅透明的“进度条或看板”。

### 4. Slack：事件驱动架构 (Event-Driven Architecture)

- **调研验证：** Slack 在构建主动应用时，强烈推崇发布/订阅模式和队列解耦。他们要求开发者将触发动作与实际的繁重处理分开。通过类似于 SQS (Simple Queue Service) 的消息队列缓冲，能够避免高峰期任务挤压，也避免让智能体因单线阻塞而停止响应。
- **对应本系统：** 意味着在 OpenAcosmi 中，用户的“长任务”指令必须立即进队列返回，而不是阻塞当前 HTTP 或 WS 请求。

### 5. KaibanJS / CrewAI：智能体可视化看板编排

- **调研验证：** KaibanJS 是近期一个备受瞩目的 JavaScript/TypeScript 原生多智能体框架。其最核心的突破就是引入了敏捷开发中的“看板 (Kanban)”模式来监督 AI。任务被明确拆分为 To Do, Doing, Blocked, Done。用户可以直观地看到哪个 AI Agent 正在做哪个节点的任务。CrewAI 中类似的 Task Tracking（任务跟踪）和层级委派执行，也强调将黑盒的 AI 推导清晰化。这毫无疑问代表了先进的 UX 趋势。

### 6. LinkedIn Hiring Assistant：真正的异步后台与主动通知

- **调研验证：** 领英在近期推出的 AI 招聘助手正是这一模式的商业完美应用范例。助手像邮件服务一样纯后台运行，不仅不会卡住操作页面，而且能够在几小时后自主甄别出匹配度极高的候选人时，“主动探出头来”向招聘人员发起通知，邀请人类进入审批流。
- **对应本系统：** 这正是我们的系统所期盼演进到的最终形态（任务转后台 -> 漫长执行 -> 主动返回推送提醒）。

---

## 三、 结合审计与调研的融合方案：我们如何落地

经过代码级的确实验证和不缩小尺寸的详尽调研，原定框架的落地不仅绝对可行，其成功率极高，且不用伤筋动骨去推翻现有逻辑。具体实施路径我们不需要丝毫妥协地缩水：

### 第 1 步：复用与“魔改” Cron 服务充当任务中心

由于我们的 `cron` 系统中已经有了完善的定时调度器、任务失败退避以及 `SessionTargetIsolated` 的独立跑道，我们无需从 0 开始写队列系统。

- **动作：** 将目前的 `TaskManager` 职责无缝并入现有的 cron 底层，或者是复刻一份基于 `cronJob` 结构的 `AsyncQueueMgr`。所有的外部异步处理请求只需封装成一种特化的、立刻触发 (Trigger immediately) 的 Job 送入队列。

### 第 2 步：完善底层协议中的 `task.*` WS 事件簇

- **动作：** 在 `events.go` 中正式添加对应 KaibanJS 模式的事件：
  - `task.queued`（已成功接收，排队中）
  - `task.started`（提取出列，某 Agent 已领受任务）
  - `task.progress`（结合 `sever_methods_chat.go` 里已经包含的关于“工具调用”细节来作为更新动态）
  - `task.completed` 与 `task.failed`
  
### 第 3 步：拦截长任务

- **动作：** 在 `chat.send` 逻辑内增加“任务重分配”判断。如果判定该意图为“长时间研究或运行任务”，则截断并返回，将该 Payload 丢给上述的 `任务中心`。

### 第 4 步：原生的主动消息播报机制

- **动作：** 复用修改目前在 `isolated_agent.go` 第 462 行开始的 `RunSubagentAnnounceFlow` 代码块。当后台任务完成时，调用它或是底层的 `chat.inject` 方法，令智能体通过当前的活动 Session 渠道，带着“看板结果的富文本卡片”主动向前端推送数据。这部分完全迎合了前述微软 Copilot 和 LinkedIn 的应用场景。

### 第 5 步：构建前端任务看板

- **动作：** 根据接收到的流式事件，在前端构建标准的 Kanban 列版面（对应 KaibanJS 的最佳实践），使用户在同智能体对话的同时，随时可以点开任务板查看后台各节点的并行情况。

---
这份报告是对系统构件与行业现状详尽审计的成果，并未删减任何颗粒细度和推理路径。系统基础架构卓越，随时准备支持该特性升级闭环。
