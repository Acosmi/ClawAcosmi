---
summary: "全局语音唤醒词（Gateway 拥有）及跨节点同步机制"
read_when:
  - 修改语音唤醒词行为或默认值
  - 添加需要唤醒词同步的新节点平台
title: "语音唤醒"
---

> **架构提示 — Rust CLI + Go Gateway**
> 唤醒词存储和广播由 Go Gateway 管理，
> 通过 WebSocket RPC 方法 `voicewake.get`/`voicewake.set` 操作。

# 语音唤醒（全局唤醒词）

OpenAcosmi 将**唤醒词作为单一全局列表**，由 **Go Gateway** 拥有。

- **没有**按节点自定义唤醒词。
- **任何节点/应用 UI 均可编辑**该列表；更改由 Go Gateway 持久化并广播给所有连接方。
- 每个设备仍保持自己的**语音唤醒 启用/禁用**开关（本地 UX + 权限不同）。

## 存储（Gateway 主机）

唤醒词存储在 Gateway 机器上：

- `~/.openacosmi/settings/voicewake.json`

结构：

```json
{ "triggers": ["openacosmi", "claude", "computer"], "updatedAtMs": 1730000000000 }
```

## 协议

### 方法

- `voicewake.get` → `{ triggers: string[] }`
- `voicewake.set`，参数 `{ triggers: string[] }` → `{ triggers: string[] }`

说明：

- 触发词会被标准化（去除首尾空格、丢弃空值）。空列表回退到默认值。
- 为安全起见强制执行限制（数量/长度上限）。

### 事件

- `voicewake.changed` 载荷 `{ triggers: string[] }`

接收方：

- 所有 WebSocket 客户端（macOS 应用、WebChat 等）
- 所有已连接节点（iOS/Android），节点连接时也作为初始"当前状态"推送。

## 客户端行为

### macOS 应用

- 使用全局列表控制 `VoiceWakeRuntime` 触发器。
- 在语音唤醒设置中编辑"触发词"时调用 `voicewake.set`，然后依赖广播保持其他客户端同步。

### iOS 节点

- 使用全局列表进行 `VoiceWakeManager` 触发检测。
- 在设置中编辑唤醒词时调用 `voicewake.set`（通过 Gateway WebSocket），同时保持本地唤醒词检测的响应性。

### Android 节点

- 在设置中暴露唤醒词编辑器。
- 通过 Gateway WebSocket 调用 `voicewake.set`，使编辑在各处同步。
