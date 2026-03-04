---
summary: "Agent 循环生命周期、事件流和等待语义"
read_when:
  - 需要了解 Agent 循环或生命周期事件的完整流程
title: "Agent 循环"
status: active
arch: go-gateway
---

# Agent 循环（OpenAcosmi）

> [!IMPORTANT]
> **架构状态**：Agent 循环由 **Go Gateway** 实现（`backend/internal/agents/runner/`）。
> Rust CLI 通过 `oa-gateway-rpc` 发送 `agent` / `agent.wait` RPC 请求触发和等待运行。

Agent 循环是 Agent 的完整运行：消息接入 → 上下文组装 → 模型推理 → 工具执行 → 流式回复 → 持久化。它是将一条消息转化为动作和最终回复的权威路径，同时保持会话状态一致。

在 OpenAcosmi 中，循环是每个会话的单一串行运行，在模型思考、调用工具和流式输出时发射生命周期和流事件。

## 入口点

- Gateway RPC：`agent` 和 `agent.wait`（Go Gateway，`backend/internal/gateway/server_methods_agent.go`）
- Rust CLI：`agent` 命令（`cli-rust/crates/oa-cmd-agent/`）

## 工作原理（高层）

1. `agent` RPC 验证参数、解析会话（sessionKey/sessionId）、持久化会话元数据，立即返回 `{ runId, acceptedAt }`。
2. `agentCommand` 运行 Agent：
   - 解析模型 + thinking/verbose 默认值
   - 加载 Skills 快照
   - 调用嵌入式 Agent 运行时
   - 如果嵌入式循环未发射生命周期结束/错误事件，则补充发射
3. 嵌入式 Agent 运行：
   - 通过按会话 + 全局队列序列化运行
   - 解析模型 + 认证 Profile 并构建会话
   - 订阅事件并流式传输助手/工具增量
   - 强制超时 → 超时则中止运行
   - 返回载荷 + 用量元数据
4. 事件桥接：
   - 工具事件 => `stream: "tool"`
   - 助手增量 => `stream: "assistant"`
   - 生命周期事件 => `stream: "lifecycle"`（`phase: "start" | "end" | "error"`）
5. `agent.wait`：
   - 等待 `runId` 的**生命周期 end/error** 事件
   - 返回 `{ status: ok|error|timeout, startedAt, endedAt, error? }`

## 队列 + 并发

- 运行按会话键（session lane）串行化，可选通过全局通道串行化。
- 防止工具/会话竞争，保持会话历史一致。
- 消息通道可选择队列模式（collect/steer/followup）供给此通道系统。参见 [命令队列](/concepts/queue)。

## 会话 + 工作区准备

- 工作区已解析并创建；沙箱运行可重定向到沙箱工作区根目录。
- Skills 已加载（或从快照重用）并注入到环境和提示词中。
- Bootstrap/上下文文件已解析并注入系统提示词报告。
- 获取会话写锁；在流式传输前打开并准备 `SessionManager`。

## 提示词组装 + 系统提示词

- 系统提示词由 OpenAcosmi 基础提示词、Skills 提示词、Bootstrap 上下文和按运行覆盖构建。
- 强制模型特定限制和压缩预留 token。
- 参见 [系统提示词](/concepts/system-prompt)。

## Hook 拦截点

OpenAcosmi 有两个 Hook 系统：

- **内部 Hook**（Gateway Hook）：命令和生命周期事件的事件驱动脚本。
- **插件 Hook**：Agent/工具生命周期和 Gateway 管道中的扩展点。

### 内部 Hook（Gateway Hook）

- **`agent:bootstrap`**：在系统提示词最终确定前构建 bootstrap 文件时运行。用于添加/移除 bootstrap 上下文文件。
- **命令 Hook**：`/new`、`/reset`、`/stop` 及其他命令事件。

参见 [Hooks](/automation/hooks)。

### 插件 Hook（Agent + Gateway 生命周期）

在 Agent 循环或 Gateway 管道内运行：

- **`before_agent_start`**：在运行开始前注入上下文或覆盖系统提示词。
- **`agent_end`**：在完成后检查最终消息列表和运行元数据。
- **`before_compaction` / `after_compaction`**：观察或标注压缩周期。
- **`before_tool_call` / `after_tool_call`**：拦截工具参数/结果。
- **`tool_result_persist`**：同步转换工具结果后写入会话转录。
- **`message_received` / `message_sending` / `message_sent`**：入站 + 出站消息 Hook。
- **`session_start` / `session_end`**：会话生命周期边界。
- **`gateway_start` / `gateway_stop`**：Gateway 生命周期事件。

参见 [插件](/tools/plugin#plugin-hooks)。

## 流式 + 部分回复

- 助手增量从 Agent 运行时流式传输并作为 `assistant` 事件发射。
- Block streaming 可在 `text_end` 或 `message_end` 时发射部分回复。
- Reasoning streaming 可作为单独流或 block 回复发射。
- 参见 [流式传输](/concepts/streaming)。

## 工具执行 + 消息工具

- 工具 start/update/end 事件在 `tool` 流上发射。
- 工具结果在日志/发射前进行大小和图像载荷的净化。
- 消息工具发送被跟踪以抑制重复的助手确认。

## 回复塑形 + 抑制

- 最终载荷由以下内容组装：
  - 助手文本（和可选的 reasoning）
  - 内联工具摘要（当 verbose + 允许时）
  - 模型错误时的助手错误文本
- `NO_REPLY` 被视为静默令牌，从出站载荷中过滤。
- 消息工具重复项从最终载荷列表中移除。
- 如果没有可渲染的载荷且工具出错，发射一个回退工具错误回复（除非消息工具已发送用户可见的回复）。

## 压缩 + 重试

- 自动压缩发射 `compaction` 流事件并可触发重试。
- 重试时重置内存缓冲区和工具摘要以避免重复输出。
- 参见 [压缩](/concepts/compaction)。

## 事件流

- `lifecycle`：由事件桥接发射（以及由 `agentCommand` 作为回退）
- `assistant`：来自 Agent 运行时的流式增量
- `tool`：来自 Agent 运行时的流式工具事件

## Chat 通道处理

- 助手增量被缓冲为 chat `delta` 消息。
- 在**生命周期 end/error** 时发射 chat `final`。

## 超时

- `agent.wait` 默认：30s（仅等待）。`timeoutMs` 参数可覆盖。
- Agent 运行时：`agents.defaults.timeoutSeconds` 默认 600s；在中止计时器中强制执行。

## 提前结束的情况

- Agent 超时（中止）
- AbortSignal（取消）
- Gateway 断连或 RPC 超时
- `agent.wait` 超时（仅等待，不停止 Agent）

## 代码位置

| 组件 | 位置 |
|------|------|
| Agent RPC 方法 | `backend/internal/gateway/server_methods_agent.go` |
| Agent 运行器 | `backend/internal/agents/runner/` |
| 会话队列 | `backend/internal/agents/runner/queue.go` |
| Rust CLI agent 命令 | `cli-rust/crates/oa-cmd-agent/` |
