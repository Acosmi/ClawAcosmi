---
summary: "macOS 菜单栏应用行为和 UI 元素"
read_when:
  - 实现菜单栏功能
  - 修改菜单栏 UI 或状态指示器
title: "菜单栏"
---

# 菜单栏（macOS 应用）

## 概览

OpenAcosmi macOS 应用是菜单栏常驻应用。主要交互通过点击菜单栏图标触发。

## 菜单栏图标状态

- **空闲**：正常图标（已连接到 Go Gateway）
- **活跃**：动画图标（agent 正在处理）
- **语音**：语音耳朵动画（Talk 模式或语音唤醒激活）
- **断开**：灰色图标（未连接 Gateway）
- **错误**：带警告标记的图标

## 菜单项

- **打开聊天**：显示 WebChat 窗口
- **Talk**：切换 Talk 模式
- **Canvas**：显示/隐藏 Canvas 面板
- **设置**：打开设置窗口
- **状态**：显示 Gateway 和节点连接信息
- **退出**：退出应用（Gateway 通过 launchd 继续运行）

## 行为

- 应用启动时自动连接到 Go Gateway（本地或远程）。
- 退出应用不会停止 Gateway 服务（launchd 管理）。
- 支持多 profile 运行（`OPENACOSMI_PROFILE`）。
- 单实例限制：相同 bundle ID 的另一个实例运行时自动退出。
