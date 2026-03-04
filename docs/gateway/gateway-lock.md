---
summary: "Gateway 单例保护：实例锁机制"
read_when:
  - 运行或调试 Gateway 进程
  - 排查单实例强制机制
title: "Gateway 锁"
---

# Gateway 锁

> [!IMPORTANT]
> **架构状态**：实例锁由 **Go Gateway**（`backend/internal/infra/gateway_lock.go`）实现，
> 使用 PID 文件锁 + 进程存活检测，支持陈旧锁自动清理。

最后更新：2026-03-01

## 目的

- 确保每个基础端口在同一宿主机上只运行一个 Gateway 实例。
- 通过 PID 检测自动清理崩溃/SIGKILL 留下的陈旧锁文件。
- 端口已被占用时快速失败并给出清晰错误。

## 机制

Go 实现使用 **PID 文件锁**（`gateway.lock`）：

1. 锁文件路径：`<stateDir>/gateway.lock`，写入当前进程 PID 和元数据（JSON 格式）。
2. 使用 `O_EXCL` 原子创建锁文件以保证独占。
3. 如果锁文件已存在：
   - 检查文件中记录的 PID 是否仍在运行（`kill(pid, 0)` / Windows 等效调用）。
   - 进程已死 → 自动删除陈旧锁并重新获取。
   - 进程仍活 → 返回 `ErrGatewayAlreadyRunning` 错误。
4. 锁文件超过 30 秒（`defaultStaleMs`）且无法解析时视为陈旧，自动清理。
5. 进程退出时调用 `unlock` 函数删除锁文件。

环境变量 `OPENACOSMI_ALLOW_MULTI_GATEWAY=1` 可跳过锁检查。

## 错误提示

- `gateway already running (pid <N>)` — 另一个 Gateway 进程正在运行。
- `创建锁文件失败: <path>` — 无法创建锁文件（权限或路径问题）。

## 运维注意事项

- 如果端口被其他进程占用，使用 `openacosmi gateway start --port <port>` 选择其他端口。
- macOS 应用在启动 Gateway 前仍维护自身的轻量 PID 检查；运行时锁由 Gateway 的文件锁强制执行。
