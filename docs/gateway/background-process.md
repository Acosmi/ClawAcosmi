---
summary: "后台执行与进程管理"
read_when:
  - 添加或修改后台执行行为
  - 调试长时间运行的 exec 任务
title: "后台执行与 Process 工具"
---

# 后台执行 + Process 工具

> [!IMPORTANT]
> **架构状态**：exec/process 工具由 **Go Gateway** 的 Agent Runner（`backend/internal/agents/runner/`）管理。

OpenAcosmi 通过 `exec` 工具运行 shell 命令，并将长时间运行的任务保留在内存中。`process` 工具管理这些后台会话。

## exec 工具

关键参数：

- `command`（必需）
- `yieldMs`（默认 10000）：此延迟后自动转入后台
- `background`（bool）：立即后台运行
- `timeout`（秒，默认 1800）：超时后终止进程
- `elevated`（bool）：启用 elevated 模式时在宿主机运行
- 需要真实 TTY？设置 `pty: true`
- `workdir`、`env`

行为：

- 前台运行直接返回输出。
- 后台运行（显式或超时）返回 `status: "running"` + `sessionId` 和尾部输出。
- 输出保留在内存中直到会话被轮询或清除。
- 如果 `process` 工具被禁用，`exec` 同步运行并忽略 `yieldMs`/`background`。

## 配置

- `tools.exec.backgroundMs`（默认 10000）
- `tools.exec.timeoutSec`（默认 1800）
- `tools.exec.cleanupMs`（默认 1800000）
- `tools.exec.notifyOnExit`（默认 true）：后台 exec 退出时发送系统事件。

## process 工具

操作：

- `list`：运行中 + 已完成的会话
- `poll`：获取会话新输出（含退出状态）
- `log`：读取聚合输出（支持 `offset` + `limit`）
- `write`：发送 stdin（`data`，可选 `eof`）
- `kill`：终止后台会话
- `clear`：从内存中移除已完成的会话
- `remove`：运行中则 kill，否则 clear

注意事项：

- 仅后台会话保留在内存中。
- 进程重启后会话丢失（不持久化到磁盘）。
- `process` 按 agent 隔离，仅能看到该 agent 启动的会话。

## 示例

运行长任务后轮询：

```json
{ "tool": "exec", "command": "sleep 5 && echo done", "yieldMs": 1000 }
```

```json
{ "tool": "process", "action": "poll", "sessionId": "<id>" }
```

立即后台运行：

```json
{ "tool": "exec", "command": "go build ./...", "background": true }
```
