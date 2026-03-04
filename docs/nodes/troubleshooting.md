---
summary: "节点故障排除：配对、前台要求、权限和工具失败"
read_when:
  - 节点已连接但 camera/canvas/screen/exec 工具失败
  - 需要理解节点配对与审批的思维模型
title: "节点故障排除"
---

> **架构提示 — Rust CLI + Go Gateway**
> 故障排除命令由 Rust CLI 执行，Go Gateway 管理节点连接状态。
> 节点主机执行审批由 Go Gateway 的 `nodehost` 包处理（`backend/internal/nodehost/allowlist_eval.go`）。

# 节点故障排除

当节点在状态中可见但节点工具失败时使用本页。

## 命令排查步骤

```bash
openacosmi status
openacosmi gateway status
openacosmi logs --follow
openacosmi doctor
openacosmi channels status --probe
```

然后运行节点专用检查：

```bash
openacosmi nodes status
openacosmi nodes describe --node <idOrNameOrIp>
openacosmi approvals get --node <idOrNameOrIp>
```

正常信号：

- 节点已连接且配对角色为 `node`。
- `nodes describe` 包含你正在调用的能力。
- 执行审批显示预期的模式/允许列表。

## 前台要求

`canvas.*`、`camera.*` 和 `screen.*` 在 iOS/Android 节点上仅限前台使用。

快速检查和修复：

```bash
openacosmi nodes describe --node <idOrNameOrIp>
openacosmi nodes canvas snapshot --node <idOrNameOrIp>
openacosmi logs --follow
```

如果看到 `NODE_BACKGROUND_UNAVAILABLE`，将节点应用切到前台并重试。

## 权限矩阵

| 能力 | iOS | Android | macOS 节点应用 | 典型失败码 |
| ---- | --- | ------- | ------------- | --------- |
| `camera.snap`、`camera.clip` | 摄像头（+ 视频需麦克风） | 摄像头（+ 视频需麦克风） | 摄像头（+ 视频需麦克风） | `*_PERMISSION_REQUIRED` |
| `screen.record` | 屏幕录制（+ 麦克风可选） | 屏幕捕获提示（+ 麦克风可选） | 屏幕录制 | `*_PERMISSION_REQUIRED` |
| `location.get` | 使用时或始终（取决于模式） | 前台/后台定位（取决于模式） | 位置权限 | `LOCATION_PERMISSION_REQUIRED` |
| `system.run` | 不适用（节点主机路径） | 不适用（节点主机路径） | 需要执行审批 | `SYSTEM_RUN_DENIED` |

## 配对与审批的区别

这是两个不同的控制层：

1. **设备配对**：此节点能否连接到 Go Gateway？
2. **执行审批**：此节点能否运行特定的 shell 命令？

快速检查：

```bash
openacosmi devices list
openacosmi nodes status
openacosmi approvals get --node <idOrNameOrIp>
openacosmi approvals allowlist add --node <idOrNameOrIp> "/usr/bin/uname"
```

如果配对缺失，先批准节点设备。
如果配对正常但 `system.run` 失败，修复执行审批/允许列表。

Go 实现：`backend/internal/nodehost/allowlist_eval.go`（审批规则评估）。

## 常见节点错误码

- `NODE_BACKGROUND_UNAVAILABLE` → 应用在后台；切到前台。
- `CAMERA_DISABLED` → 节点设置中摄像头开关已关闭。
- `*_PERMISSION_REQUIRED` → 操作系统权限缺失/被拒绝。
- `LOCATION_DISABLED` → 位置模式为关闭。
- `LOCATION_PERMISSION_REQUIRED` → 请求的位置模式未被授权。
- `LOCATION_BACKGROUND_UNAVAILABLE` → 应用在后台但仅有"使用时"权限。
- `SYSTEM_RUN_DENIED: approval required` → exec 请求需要显式批准。
- `SYSTEM_RUN_DENIED: allowlist miss` → 命令被允许列表模式阻止。

## 快速恢复循环

```bash
openacosmi nodes status
openacosmi nodes describe --node <idOrNameOrIp>
openacosmi approvals get --node <idOrNameOrIp>
openacosmi logs --follow
```

如果仍然无法解决：

- 重新批准设备配对。
- 重新打开节点应用（前台）。
- 重新授予操作系统权限。
- 重新创建/调整执行审批策略。

相关文档：

- [节点总览](/nodes/index)
- [摄像头捕获](/nodes/camera)
- [位置命令](/nodes/location-command)
- [Exec 审批](/tools/exec-approvals)
- [Gateway 配对](/gateway/pairing)
