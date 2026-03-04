---
summary: "测试套件：单元测试/集成测试/端到端测试/实时测试及其覆盖范围"
read_when:
  - 在本地或 CI 中运行测试
  - 为模型/Provider Bug 添加回归测试
  - 调试 Gateway + Agent 行为
title: "测试"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# 测试

OpenAcosmi 采用 Rust + Go 混合测试体系：

- **Rust CLI 测试**：使用 `cargo test` 运行
- **Go Gateway 测试**：使用 `go test ./...` 运行
- **UI 测试**：使用 vitest 运行（TypeScript 前端部分保留）

本文档是"我们如何测试"的指南：

- 每个测试套件覆盖什么（以及刻意_不_覆盖什么）
- 常见工作流应运行哪些命令（本地、推送前、调试）
- 实时测试如何发现凭据并选择模型/Provider
- 如何为真实场景中的模型/Provider 问题添加回归测试

## 快速开始

日常使用：

- 完整门禁（推送前必须通过）：`cargo build && cargo test && cd backend && go test ./... && make build`

需要额外信心时：

- 覆盖率门禁：`cargo test` + `go test -cover ./...`
- 端到端测试：`make test-e2e`

调试真实 Provider/模型（需要真实凭据）时：

- 实时测试套件（模型 + Gateway 工具/图像探测）：`make test-live`

提示：如果只需要一个失败用例，优先通过下述允许列表环境变量缩小实时测试范围。

## 测试套件（各在哪里运行）

将测试套件理解为"真实度递增"（以及不稳定性/成本递增）：

### 单元 / 集成测试（默认）

- 命令：`cargo test`（Rust）+ `go test ./...`（Go）
- 范围：
  - 纯单元测试
  - 进程内集成测试（Gateway 认证、路由、工具、解析、配置）
  - 已知 Bug 的确定性回归测试
- 期望：
  - 在 CI 中运行
  - 无需真实密钥
  - 应快速且稳定

### 端到端测试（Gateway 冒烟测试）

- 命令：`make test-e2e`
- 范围：
  - 多实例 Gateway 端到端行为
  - WebSocket/HTTP 接口、节点配对和较重的网络操作
- 期望：
  - 在 CI 中运行（当在流水线中启用时）
  - 无需真实密钥
  - 比单元测试有更多活动部件（可能更慢）

### 实时测试（真实 Provider + 真实模型）

- 命令：`make test-live`
- 默认：通过设置 `OPENACOSMI_LIVE_TEST=1` **启用**
- 范围：
  - "这个 Provider/模型_今天_用真实凭据到底能不能用？"
  - 捕获 Provider 格式变更、工具调用异常、认证问题和速率限制行为
- 期望：
  - 设计上不具有 CI 稳定性（真实网络、真实 Provider 策略、配额、中断）
  - 消耗资金 / 使用速率限制
  - 优先运行缩小范围的子集而非"全部"
  - 实时运行会加载 `~/.profile` 以获取缺失的 API 密钥
  - Anthropic 密钥轮换：设置 `OPENACOSMI_LIVE_ANTHROPIC_KEYS="sk-...,sk-..."` 或多个 `ANTHROPIC_API_KEY*` 变量；测试会在速率限制时重试

## 应该运行哪个测试套件？

使用此决策表：

- 编辑逻辑/测试：运行 `cargo test` + `go test ./...`
- 修改 Gateway 网络 / WS 协议 / 配对：加上 `make test-e2e`
- 调试"我的 bot 挂了" / Provider 特定故障 / 工具调用：运行缩小范围的 `make test-live`

## 实时测试：模型冒烟测试（Profile 密钥）

实时测试分为两层，以便隔离故障：

- "直接模型"测试告诉我们 Provider/模型能否用给定密钥响应。
- "Gateway 冒烟"测试告诉我们完整的 Gateway+Agent 管道是否正常工作（会话、历史、工具、沙箱策略等）。

### 第一层：直接模型完成（无 Gateway）

- 目标：
  - 枚举发现的模型
  - 使用 `getApiKeyForModel` 选择有凭据的模型
  - 对每个模型运行小规模完成测试（以及需要时的定向回归）
- 启用方式：
  - `make test-live`（或直接调用测试时设置 `OPENACOSMI_LIVE_TEST=1`）
- 设置 `OPENACOSMI_LIVE_MODELS=modern`（或 `all`，其别名为 modern）来实际运行此套件
- 模型选择方式：
  - `OPENACOSMI_LIVE_MODELS=modern` 运行现代允许列表
  - `OPENACOSMI_LIVE_MODELS=all` 是现代允许列表的别名
  - `OPENACOSMI_LIVE_MODELS="openai/gpt-5.2,anthropic/claude-opus-4-6,..."` 逗号分隔列表
- Provider 选择方式：
  - `OPENACOSMI_LIVE_PROVIDERS="google,google-antigravity,google-gemini-cli"` 逗号分隔列表
- 密钥来源：
  - 默认：Profile 存储和环境变量回退
  - 设置 `OPENACOSMI_LIVE_REQUIRE_PROFILE_KEYS=1` 强制**仅使用 Profile 存储**
- 存在意义：
  - 将"Provider API 异常 / 密钥无效"与"Gateway Agent 管道异常"区分开来

### 第二层：Gateway + 开发 Agent 冒烟测试

- 目标：
  - 启动进程内 Gateway
  - 创建/打补丁 `agent:dev:*` 会话（每次运行覆盖模型）
  - 遍历有密钥的模型并断言：
    - "有意义的"响应（无工具）
    - 真实工具调用正常工作（read 探测）
    - 可选额外工具探测（exec+read 探测）
- 探测详情（便于快速解释失败原因）：
  - `read` 探测：测试在工作区写入随机文件，要求 Agent 读取并回显随机值。
  - `exec+read` 探测：测试要求 Agent 执行写入随机值到临时文件，然后读取回来。
  - 图像探测：测试附加生成的 PNG（猫 + 随机代码），期望模型返回 `cat <CODE>`。
- 启用方式：
  - `make test-live`（或设置 `OPENACOSMI_LIVE_TEST=1`）
- 模型选择方式：
  - 默认：现代允许列表
  - `OPENACOSMI_LIVE_GATEWAY_MODELS=all` 为现代允许列表别名
  - 或设置 `OPENACOSMI_LIVE_GATEWAY_MODELS="provider/model"` 缩小范围

提示：要查看您的机器上可测试的内容（以及准确的 `provider/model` ID），运行：

```bash
openacosmi models list
openacosmi models list --json
```

## 实时测试：Anthropic setup-token 冒烟测试

- 目标：验证 Claude Code CLI setup-token 或粘贴的 setup-token Profile 能否完成 Anthropic 提示。
- 启用方式：
  - `make test-live`（或设置 `OPENACOSMI_LIVE_TEST=1`）
  - `OPENACOSMI_LIVE_SETUP_TOKEN=1`
- Token 来源（选一）：
  - Profile：`OPENACOSMI_LIVE_SETUP_TOKEN_PROFILE=anthropic:setup-token-test`
  - 原始 Token：`OPENACOSMI_LIVE_SETUP_TOKEN_VALUE=sk-ant-oat01-...`

设置示例：

```bash
openacosmi models auth paste-token --provider anthropic --profile-id anthropic:setup-token-test
OPENACOSMI_LIVE_SETUP_TOKEN=1 OPENACOSMI_LIVE_SETUP_TOKEN_PROFILE=anthropic:setup-token-test make test-live
```

## 实时测试：CLI 后端冒烟测试

- 目标：验证 Gateway + Agent 管道使用本地 CLI 后端的表现，不触动默认配置。
- 启用方式：
  - `make test-live`（或设置 `OPENACOSMI_LIVE_TEST=1`）
  - `OPENACOSMI_LIVE_CLI_BACKEND=1`
- 默认值：
  - 模型：`claude-cli/claude-sonnet-4-5`
  - 命令：`claude`
  - 参数：`["-p","--output-format","json","--dangerously-skip-permissions"]`

示例：

```bash
OPENACOSMI_LIVE_CLI_BACKEND=1 \
  OPENACOSMI_LIVE_CLI_BACKEND_MODEL="claude-cli/claude-sonnet-4-5" \
  make test-live
```

### 推荐的实时测试配方

精确、显式的允许列表最快且最少抖动：

- 单模型，直接（无 Gateway）：
  - `OPENACOSMI_LIVE_MODELS="openai/gpt-5.2" make test-live`

- 单模型，Gateway 冒烟：
  - `OPENACOSMI_LIVE_GATEWAY_MODELS="openai/gpt-5.2" make test-live`

- 跨多个 Provider 的工具调用：
  - `OPENACOSMI_LIVE_GATEWAY_MODELS="openai/gpt-5.2,anthropic/claude-opus-4-6,google/gemini-3-flash-preview,zai/glm-4.7,minimax/minimax-m2.1" make test-live`

说明：

- `google/...` 使用 Gemini API（API 密钥）。
- `google-antigravity/...` 使用 Antigravity OAuth 桥（Cloud Code Assist 风格的 Agent 端点）。
- `google-gemini-cli/...` 使用本机上的本地 `gemini` 二进制文件（独立的认证和工具特性）。
- Gemini API vs Gemini CLI：
  - API：OpenAcosmi 通过 HTTP 调用 Google 托管的 Gemini API（API 密钥/Profile 认证）；大多数用户所说的"Gemini"就是这个。
  - CLI：OpenAcosmi 调用本地 `gemini` 二进制文件；它有自己的认证，行为可能不同（流式传输/工具支持/版本差异）。

## 凭据（禁止提交）

实时测试以与 CLI 相同的方式发现凭据。实际含义：

- 如果 CLI 正常工作，实时测试应该能找到相同的密钥。
- 如果实时测试提示"无凭据"，按调试 `openacosmi models list` / 模型选择的方式排查。

- Profile 存储：`~/.openacosmi/credentials/`（首选；测试中"Profile 密钥"的含义）
- 配置：`~/.openacosmi/openacosmi.json`（或 `OPENACOSMI_CONFIG_PATH`）

如果要依赖环境变量密钥（如在 `~/.profile` 中导出的），请在 `source ~/.profile` 后运行本地测试。

## 文档健全性检查

编辑文档后运行检查：`make docs-check`。

## 离线回归测试（CI 安全）

这些是"真实管道"回归测试但无需真实 Provider：

- Gateway 工具调用（模拟 OpenAI，真实 Gateway + Agent 循环）
- Gateway 向导（WS `wizard.start`/`wizard.next`，写入配置 + 强制认证）

## Agent 可靠性评估（技能）

已有一些 CI 安全测试作为"Agent 可靠性评估"：

- 通过真实 Gateway + Agent 循环的模拟工具调用。
- 端到端向导流程验证会话连接和配置效果。

技能方面仍缺少的内容（参见 [技能](/tools/skills)）：

- **决策判断：** 当技能列在提示中时，Agent 是否选择了正确的技能（或避免了无关技能）？
- **合规性：** Agent 是否在使用前阅读了 `SKILL.md` 并遵循必需的步骤/参数？
- **工作流契约：** 断言工具顺序、会话历史延续和沙箱边界的多轮场景。

未来评估应先保持确定性：

- 使用模拟 Provider 的场景运行器，断言工具调用 + 顺序、技能文件读取和会话连接。
- 一小套聚焦技能的场景（使用 vs 避免、门控、提示注入）。
- 仅在 CI 安全套件就绪后才添加可选的实时评估（通过环境变量门控）。

## 添加回归测试（指导）

当修复了在实时测试中发现的 Provider/模型问题时：

- 如果可能，添加 CI 安全的回归测试（模拟/存根 Provider，或捕获确切的请求形状转换）
- 如果本质上只能在实时环境中测试（速率限制、认证策略），保持实时测试范围精确，通过环境变量按需启用
- 优先定位能捕获 Bug 的最小层：
  - Provider 请求转换/回放 Bug → 直接模型测试
  - Gateway 会话/历史/工具管道 Bug → Gateway 实时冒烟或 CI 安全 Gateway 模拟测试
