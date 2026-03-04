---
summary: "摄像头捕获（iOS/Android/macOS 节点）：照片（jpg）和短视频片段（mp4）"
read_when:
  - 添加或修改 iOS/Android/macOS 节点上的摄像头捕获
  - 扩展 agent 可访问的 MEDIA 临时文件工作流
title: "摄像头捕获"
---

> **架构提示 — Rust CLI + Go Gateway**
> 摄像头命令通过 Rust CLI（`openacosmi nodes camera`）发起，
> 经 Go Gateway 的 WebSocket 转发给节点设备处理。

# 摄像头捕获（agent）

OpenAcosmi 支持 agent 工作流中的**摄像头捕获**：

- **iOS 节点**（通过 Gateway 配对）：通过 `node.invoke` 捕获**照片**（`jpg`）或**短视频**（`mp4`，可选音频）。
- **Android 节点**（通过 Gateway 配对）：通过 `node.invoke` 捕获**照片**（`jpg`）或**短视频**（`mp4`，可选音频）。
- **macOS 应用**（作为 Gateway 节点）：通过 `node.invoke` 捕获**照片**（`jpg`）或**短视频**（`mp4`，可选音频）。

所有摄像头访问均受**用户控制设置**保护。

## iOS 节点

### 用户设置（默认开启）

- iOS 设置选项卡 → **摄像头** → **允许摄像头**（`camera.enabled`）
  - 默认：**开启**（缺少键视为已启用）。
  - 关闭时：`camera.*` 命令返回 `CAMERA_DISABLED`。

### 命令（通过 Go Gateway `node.invoke` 转发）

- `camera.list`
  - 响应载荷：
    - `devices`：`{ id, name, position, deviceType }` 数组

- `camera.snap`
  - 参数：
    - `facing`：`front|back`（默认：`front`）
    - `maxWidth`：数值（可选；iOS 节点默认 `1600`）
    - `quality`：`0..1`（可选；默认 `0.9`）
    - `format`：当前为 `jpg`
    - `delayMs`：数值（可选；默认 `0`）
    - `deviceId`：字符串（可选；来自 `camera.list`）
  - 响应载荷：
    - `format: "jpg"`
    - `base64: "<...>"`
    - `width`、`height`
  - 载荷保护：照片会重新压缩以将 base64 载荷控制在 5 MB 以内。

- `camera.clip`
  - 参数：
    - `facing`：`front|back`（默认：`front`）
    - `durationMs`：数值（默认 `3000`，上限 `60000`）
    - `includeAudio`：布尔值（默认 `true`）
    - `format`：当前为 `mp4`
    - `deviceId`：字符串（可选；来自 `camera.list`）
  - 响应载荷：
    - `format: "mp4"`
    - `base64: "<...>"`
    - `durationMs`
    - `hasAudio`

### 前台要求

与 `canvas.*` 类似，iOS 节点仅允许在**前台**执行 `camera.*` 命令。后台调用返回 `NODE_BACKGROUND_UNAVAILABLE`。

### CLI 辅助（临时文件 + MEDIA）

最简便的获取附件方式是通过 Rust CLI 辅助命令，将解码后的媒体写入临时文件并输出 `MEDIA:<path>`。

示例：

```bash
openacosmi nodes camera snap --node <id>               # 默认：前后摄像头（2 个 MEDIA 行）
openacosmi nodes camera snap --node <id> --facing front
openacosmi nodes camera clip --node <id> --duration 3000
openacosmi nodes camera clip --node <id> --no-audio
```

说明：

- `nodes camera snap` 默认捕获**两个**朝向，为 agent 提供双视角。
- 输出文件为临时文件（位于操作系统临时目录），除非自行封装处理。

## Android 节点

### Android 用户设置（默认开启）

- Android 设置页面 → **摄像头** → **允许摄像头**（`camera.enabled`）
  - 默认：**开启**（缺少键视为已启用）。
  - 关闭时：`camera.*` 命令返回 `CAMERA_DISABLED`。

### 权限

- Android 需要运行时权限：
  - `CAMERA`：用于 `camera.snap` 和 `camera.clip`。
  - `RECORD_AUDIO`：用于 `camera.clip` 中 `includeAudio=true` 时。

权限缺失时，应用会尽可能提示；如被拒绝，`camera.*` 请求以 `*_PERMISSION_REQUIRED` 错误失败。

### Android 前台要求

与 `canvas.*` 类似，Android 节点仅允许在**前台**执行 `camera.*` 命令。后台调用返回 `NODE_BACKGROUND_UNAVAILABLE`。

### 载荷保护

照片会重新压缩以将 base64 载荷控制在 5 MB 以内。

## macOS 应用

### 用户设置（默认关闭）

macOS 伴侣应用提供复选框：

- **设置 → 通用 → 允许摄像头**（`openacosmi.cameraEnabled`）
  - 默认：**关闭**
  - 关闭时：摄像头请求返回"Camera disabled by user"。

### CLI 辅助（node invoke）

使用 Rust CLI `openacosmi` 对 macOS 节点调用摄像头命令。

示例：

```bash
openacosmi nodes camera list --node <id>            # 列出摄像头 ID
openacosmi nodes camera snap --node <id>            # 输出 MEDIA:<path>
openacosmi nodes camera snap --node <id> --max-width 1280
openacosmi nodes camera snap --node <id> --delay-ms 2000
openacosmi nodes camera snap --node <id> --device-id <id>
openacosmi nodes camera clip --node <id> --duration 10s          # 输出 MEDIA:<path>
openacosmi nodes camera clip --node <id> --duration-ms 3000      # 输出 MEDIA:<path>（旧版参数）
openacosmi nodes camera clip --node <id> --device-id <id>
openacosmi nodes camera clip --node <id> --no-audio
```

说明：

- `openacosmi nodes camera snap` 默认 `maxWidth=1600`，除非手动覆盖。
- macOS 上 `camera.snap` 在预热/曝光稳定后等待 `delayMs`（默认 2000ms）再捕获。
- 照片载荷会重新压缩以将 base64 控制在 5 MB 以内。

## 安全性与实际限制

- 摄像头和麦克风访问会触发操作系统权限提示（需要 Info.plist 中的使用说明字符串）。
- 视频片段被限制（当前 `<= 60s`）以避免节点载荷过大（base64 开销 + 消息限制）。

## macOS 屏幕视频（系统级）

对于_屏幕_视频（非摄像头），使用 macOS 伴侣应用：

```bash
openacosmi nodes screen record --node <id> --duration 10s --fps 15   # 输出 MEDIA:<path>
```

说明：

- 需要 macOS **屏幕录制**权限（TCC）。
