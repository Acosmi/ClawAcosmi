---
summary: "安装 OpenAcosmi 并在几分钟内开始首次对话。"
read_when:
  - 首次从零开始设置
  - 想要最快速到达可用的聊天
title: "快速开始"
status: active
arch: rust-cli+go-gateway
---

# 快速开始

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - **Rust CLI**（`openacosmi`）：用户交互、命令解析、TUI 渲染
> - **Go Gateway**（`acosmi`）：服务端逻辑、通道适配、Agent 执行

目标：从零开始到首次可用的聊天，最小化设置步骤。

<Info>
最快聊天方式：打开 Control UI（无需设置通道）。运行 `openacosmi dashboard`
并在浏览器中聊天，或打开 `http://127.0.0.1:18789/`（
<Tooltip headline="Gateway 主机" tip="运行 OpenAcosmi Gateway 服务的主机。">Gateway 主机</Tooltip>）。
文档：[Dashboard](/web/dashboard) 和 [Control UI](/web/control-ui)。
</Info>

## 前置条件

- Go 1.22+ 和 Rust 1.75+（从源码构建）
- 或使用预编译二进制

<Tip>
检查版本：`go version`、`rustc --version`、`openacosmi --version`
</Tip>

## 快速设置 (CLI)

<Steps>
  <Step title="安装 OpenAcosmi（推荐）">
    <Tabs>
      <Tab title="macOS/Linux">
        ```bash
        curl -fsSL https://openacosmi.ai/install.sh | bash
        ```
      </Tab>
      <Tab title="Windows (PowerShell)">
        ```powershell
        iwr -useb https://openacosmi.ai/install.ps1 | iex
        ```
      </Tab>
      <Tab title="从源码构建">
        ```bash
        # 构建 Rust CLI
        cd cli-rust && cargo build --release
        # 构建 Go Gateway
        cd backend && make build
        ```
      </Tab>
    </Tabs>

    <Note>
    其他安装方式和要求：[安装](/install)。
    </Note>

  </Step>
  <Step title="运行引导向导">
    ```bash
    openacosmi onboard --install-daemon
    ```

    向导配置认证、Gateway 设置和可选通道。
    详见 [引导向导](/start/wizard)。

  </Step>
  <Step title="检查 Gateway">
    如果已安装服务，它应该已经在运行：

    ```bash
    openacosmi gateway status
    ```

  </Step>
  <Step title="打开 Control UI">
    ```bash
    openacosmi dashboard
    ```
  </Step>
</Steps>

<Check>
如果 Control UI 加载成功，你的 Gateway 已准备就绪。
</Check>

## 可选检查和扩展

<AccordionGroup>
  <Accordion title="前台运行 Gateway">
    适用于快速测试或排错。

    ```bash
    openacosmi gateway --port 18789
    ```

    或直接运行 Go Gateway：

    ```bash
    cd backend && make gateway-dev
    ```

  </Accordion>
  <Accordion title="发送测试消息">
    需要已配置的通道。

    ```bash
    openacosmi message send --target +15555550123 --message "Hello from OpenAcosmi"
    ```

  </Accordion>
</AccordionGroup>

## 常用环境变量

如果以服务账户运行或想自定义配置/状态位置：

- `OPENACOSMI_HOME` 设置内部路径解析的主目录。
- `OPENACOSMI_STATE_DIR` 覆盖状态目录。
- `OPENACOSMI_CONFIG_PATH` 覆盖配置文件路径。

完整环境变量参考：[环境变量](/help/environment)。

## 深入了解

<Columns>
  <Card title="引导向导（详情）" href="/start/wizard">
    完整 CLI 向导参考和高级选项。
  </Card>
  <Card title="macOS 应用引导" href="/start/onboarding">
    macOS 应用的首次启动流程。
  </Card>
</Columns>

## 你将拥有的

- 运行中的 Go Gateway
- 已配置的认证
- Control UI 访问或已连接的通道

## 下一步

- DM 安全和审批：[配对](/channels/pairing)
- 连接更多通道：[通道](/channels)
- 高级工作流和源码构建：[设置](/start/setup)
