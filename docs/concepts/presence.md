---
summary: "在线状态（Presence）系统：条目生成、合并和显示"
read_when:
  - 修改 presence 发射或显示
title: "在线状态"
status: active
arch: go-gateway
---

# 在线状态（Presence）

> [!NOTE]
> **架构状态**：Presence 由 **Go Gateway** 管理（`backend/internal/gateway/`）。
> Rust CLI 通过 `system-presence` RPC 查询在线状态（`oa-cmd-system`）。

轻量级的最大努力在线状态视图，追踪 Gateway 实例和已连接客户端。

## 在线状态字段

- `instanceId`：唯一实例标识（去重键）
- `host`：主机名
- `ip`：IP 地址
- `version`：版本号
- `mode`：`gateway` | `cli` | `ui` | `node`
- `connectedAt`：连接时间
- `lastSeen`：最后活跃时间

## 生成来源

| 来源 | 描述 |
|------|------|
| Gateway 自身 | 启动时注册自身 |
| WebSocket 连接 | 客户端连接时注册 |
| `system-event` 信标 | 定期心跳更新 |
| Node 连接 | macOS/iOS/Android 节点注册 |

## 合并与去重

- 基于 `instanceId` 的键进行合并。
- 更新同一 `instanceId` 的条目替换旧值。
- TTL 过期：5 分钟未更新的条目自动移除。
- 最大 200 条在线状态条目。

## 为什么一次性 CLI 命令不产生 Presence

短命 CLI 连接（如 `openacosmi health`）在完成后立即断开。它们的 WebSocket 连接存在时间极短，通常在 presence 通知到达前就已关闭。

## 消费者

- macOS 应用的"Instances"标签页。
- Web UI 的实例列表。
- `openacosmi status` 命令。
