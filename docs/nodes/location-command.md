---
summary: "节点位置命令（location.get）、权限模式和后台行为"
read_when:
  - 添加节点位置支持或权限 UI
  - 设计后台定位与推送流程
title: "位置命令"
---

> **架构提示 — Rust CLI + Go Gateway**
> 位置命令通过 Rust CLI（`openacosmi nodes location`）发起，
> 经 Go Gateway WebSocket 转发给节点设备执行。

# 位置命令（节点）

## 概要

- `location.get` 是节点命令（通过 `node.invoke` 调用）。
- 默认关闭。
- 设置使用选择器：关闭 / 使用时 / 始终。
- 独立开关：精确位置。

## 为何使用选择器而非简单开关

操作系统权限是多级别的。应用内可展示选择器，但操作系统仍决定实际授权。

- iOS/macOS：用户可在系统提示/设置中选择**使用时**或**始终**。应用可请求升级，但操作系统可能要求前往设置。
- Android：后台定位是独立权限；Android 10+ 上通常需要在设置中操作。
- 精确位置是独立授权（iOS 14+ "精确"、Android "fine" vs "coarse"）。

UI 中的选择器驱动请求模式；实际授权存在于操作系统设置中。

## 设置模型

每个节点设备：

- `location.enabledMode`：`off | whileUsing | always`
- `location.preciseEnabled`：布尔值

UI 行为：

- 选择 `whileUsing` 请求前台权限。
- 选择 `always` 先确保 `whileUsing`，然后请求后台权限（或引导用户前往设置）。
- 如果操作系统拒绝请求的级别，回退到已授权的最高级别并显示状态。

## 权限映射（node.permissions）

可选。macOS 节点通过权限映射报告 `location`；iOS/Android 可能省略。

## 命令：`location.get`

通过 Go Gateway 的 `node.invoke` 调用。

参数（建议）：

```json
{
  "timeoutMs": 10000,
  "maxAgeMs": 15000,
  "desiredAccuracy": "coarse|balanced|precise"
}
```

响应载荷：

```json
{
  "lat": 48.20849,
  "lon": 16.37208,
  "accuracyMeters": 12.5,
  "altitudeMeters": 182.0,
  "speedMps": 0.0,
  "headingDeg": 270.0,
  "timestamp": "2026-01-03T12:34:56.000Z",
  "isPrecise": true,
  "source": "gps|wifi|cell|unknown"
}
```

错误码（稳定）：

- `LOCATION_DISABLED`：选择器为关闭状态。
- `LOCATION_PERMISSION_REQUIRED`：请求模式缺少权限。
- `LOCATION_BACKGROUND_UNAVAILABLE`：应用在后台但仅允许"使用时"。
- `LOCATION_TIMEOUT`：超时未获得定位。
- `LOCATION_UNAVAILABLE`：系统故障/无可用提供者。

## 后台行为（未来）

目标：模型可以在节点处于后台时请求位置，但仅在满足以下条件时：

- 用户选择了**始终**。
- 操作系统授予后台定位。
- 应用被允许在后台运行定位（iOS 后台模式 / Android 前台服务或特殊许可）。

推送触发流程（未来）：

1. Go Gateway 向节点发送推送（静默推送或 FCM 数据）。
2. 节点短暂唤醒并从设备请求位置。
3. 节点将载荷转发给 Gateway。

说明：

- iOS：需要始终权限 + 后台定位模式。静默推送可能被限流；预期间歇性失败。
- Android：后台定位可能需要前台服务；否则预期被拒绝。

## 模型/工具集成

- 工具接口：`nodes` 工具添加 `location_get` action（需要节点）。
- CLI：`openacosmi nodes location get --node <id>`。
- Agent 指南：仅在用户启用位置并理解范围时调用。

## UX 文案（建议）

- 关闭："位置共享已禁用。"
- 使用时："仅在 OpenAcosmi 打开时。"
- 始终："允许后台定位。需要系统权限。"
- 精确："使用精确 GPS 位置。关闭以共享近似位置。"
