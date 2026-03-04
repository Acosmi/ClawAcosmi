---
summary: "macOS 应用首次引导流程"
read_when:
  - 设计 macOS 引导流程
  - 实现认证或身份设置
title: "引导（macOS 应用）"
sidebarTitle: "引导: macOS 应用"
status: active
arch: rust-cli+go-gateway
---

# 引导（macOS 应用）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> macOS 应用管理 Go Gateway 进程生命周期。

本文档描述**当前**的首次引导流程。目标是提供流畅的"第 0 天"体验：选择 Gateway 运行位置、连接认证、运行向导、让 Agent 完成自我引导。

<Steps>
<Step title="批准 macOS 警告">
<Frame>
<img src="/assets/macos-onboarding/01-macos-warning.jpeg" alt="" />
</Frame>
</Step>
<Step title="批准查找本地网络">
<Frame>
<img src="/assets/macos-onboarding/02-local-networks.jpeg" alt="" />
</Frame>
</Step>
<Step title="欢迎和安全提示">
<Frame caption="阅读显示的安全提示并做出相应决定">
<img src="/assets/macos-onboarding/03-security-notice.png" alt="" />
</Frame>
</Step>
<Step title="本地 vs 远程">
<Frame>
<img src="/assets/macos-onboarding/04-choose-gateway.png" alt="" />
</Frame>

**Go Gateway** 在哪里运行？

- **本机（仅本地）：** 引导可以运行 OAuth 流程并在本地写入凭证。
- **远程（通过 SSH/Tailnet）：** 引导**不会**在本地运行 OAuth；凭证必须存在于 Gateway 主机上。
- **稍后配置：** 跳过设置，保持应用未配置状态。

<Tip>
**Gateway 认证提示：**
- 向导现在即使在回环地址上也会生成 **token**，因此本地 WebSocket 客户端必须认证。
- 如果禁用认证，任何本地进程都可以连接；仅在完全可信的机器上使用。
- 对于多机器访问或非回环绑定，使用 **token**。
</Tip>
</Step>
<Step title="权限">
<Frame caption="选择要授予 OpenAcosmi 的权限">
<img src="/assets/macos-onboarding/05-permissions.png" alt="" />
</Frame>

引导请求以下 TCC 权限：

- 自动化（AppleScript）
- 通知
- 辅助功能
- 屏幕录制
- 麦克风
- 语音识别
- 摄像头
- 定位

</Step>
<Step title="CLI 安装">
  <Info>此步骤为可选</Info>
  应用可以安装全局 `openacosmi` Rust CLI，使终端工作流和 launchd 任务开箱即用。
  CLI 通过 `cargo install` 或预编译二进制安装。
</Step>
<Step title="引导对话（专用会话）">
  设置完成后，应用打开一个专用的引导对话会话，让 Agent 自我介绍并引导后续步骤。
  这使首次引导指导与正常对话分离。详见 [引导](/start/bootstrapping) 了解
  Agent 首次运行时在 Gateway 主机上发生的事情。
</Step>
</Steps>
