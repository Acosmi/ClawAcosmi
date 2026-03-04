---
summary: "在 OpenAcosmi 中使用 MiniMax M2.1"
read_when:
  - 使用 MiniMax 模型
  - 需要 MiniMax 设置指南
title: "MiniMax"
status: active
arch: rust-cli+go-gateway
---

# MiniMax

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - MiniMax 供应商默认配置定义在 **Go Gateway**（`backend/internal/agents/models/providers.go` — `minimax` / `minimax-portal`）
> - API Key 环境变量：`MINIMAX_API_KEY`（回退：`MINIMAX_PORTAL_API_KEY`）
> - MiniMax 模型常量定义在 **Rust CLI**（`cli-rust/crates/oa-cmd-onboard/src/auth/models.rs`）
> - 交互式配置通过 `openacosmi configure`（Rust CLI `oa-cmd-configure`）

MiniMax 是一家 AI 公司，开发了 **M2/M2.1** 模型系列。当前面向编程的版本是 **MiniMax M2.1**（2025 年 12 月 23 日发布），专为真实世界复杂任务构建。

来源：[MiniMax M2.1 发布说明](https://www.minimax.io/news/minimax-m21)

## 模型概览（M2.1）

MiniMax 在 M2.1 中的改进亮点：

- 更强的**多语言编程**能力（Rust、Java、Go、C++、Kotlin、Objective-C、TS/JS）。
- 更好的 **Web/App 开发**和美学输出质量（包括原生移动端）。
- 改进的**复合指令**处理，适用于办公工作流，基于交织推理和集成约束执行。
- **更简洁的回复**，更低的 token 消耗和更快的迭代循环。
- 更强的**工具/Agent 框架**兼容性和上下文管理（Claude Code、Droid/Factory AI、Cline、Kilo Code、Roo Code、BlackBox）。
- 更高质量的**对话和技术写作**输出。

## MiniMax M2.1 vs MiniMax M2.1 Lightning

- **速度：** Lightning 是 MiniMax 定价文档中的"快速"变体。
- **费用：** 定价显示相同的输入成本，但 Lightning 的输出成本更高。
- **编程计划路由：** Lightning 后端不直接在 MiniMax 编程计划中可用。MiniMax 自动将大多数请求路由到 Lightning，但在流量高峰时回退到常规 M2.1 后端。

## 选择设置方式

### MiniMax OAuth（编程计划）— 推荐

**适用场景：** 通过 OAuth 快速设置 MiniMax 编程计划，无需 API Key。

启用内置 OAuth 插件并认证：

```bash
openacosmi plugins enable minimax-portal-auth  # 如已加载则跳过
openacosmi gateway restart  # 如 Gateway 已运行则重启
openacosmi onboard --auth-choice minimax-portal
```

系统会提示选择端点：

- **Global** — 国际用户（`api.minimax.io`）
- **CN** — 中国大陆用户（`api.minimaxi.com`）

### MiniMax M2.1（API Key）

**适用场景：** 托管的 MiniMax 与 Anthropic 兼容 API。

通过 CLI 配置：

- 运行 `openacosmi configure`
- 选择 **Model/auth**
- 选择 **MiniMax M2.1**

```json5
{
  env: { MINIMAX_API_KEY: "sk-..." },
  agents: { defaults: { model: { primary: "minimax/MiniMax-M2.1" } } },
  models: {
    mode: "merge",
    providers: {
      minimax: {
        baseUrl: "https://api.minimax.io/anthropic",
        apiKey: "${MINIMAX_API_KEY}",
        api: "anthropic-messages",
        models: [
          {
            id: "MiniMax-M2.1",
            name: "MiniMax M2.1",
            reasoning: false,
            input: ["text"],
            cost: { input: 15, output: 60, cacheRead: 2, cacheWrite: 10 },
            contextWindow: 200000,
            maxTokens: 8192,
          },
        ],
      },
    },
  },
}
```

### MiniMax M2.1 作为回退（Opus 主力）

**适用场景：** 保持 Opus 4.6 为主力，MiniMax M2.1 作为故障转移。

```json5
{
  env: { MINIMAX_API_KEY: "sk-..." },
  agents: {
    defaults: {
      models: {
        "anthropic/claude-opus-4-6": { alias: "opus" },
        "minimax/MiniMax-M2.1": { alias: "minimax" },
      },
      model: {
        primary: "anthropic/claude-opus-4-6",
        fallbacks: ["minimax/MiniMax-M2.1"],
      },
    },
  },
}
```

### 可选：通过 LM Studio 本地运行（手动）

**适用场景：** 使用 LM Studio 进行本地推理。
在强力硬件（例如台式机/服务器）上使用 LM Studio 本地服务器运行 MiniMax M2.1 效果很好。

手动通过 `openacosmi.json` 配置：

```json5
{
  agents: {
    defaults: {
      model: { primary: "lmstudio/minimax-m2.1-gs32" },
      models: { "lmstudio/minimax-m2.1-gs32": { alias: "Minimax" } },
    },
  },
  models: {
    mode: "merge",
    providers: {
      lmstudio: {
        baseUrl: "http://127.0.0.1:1234/v1",
        apiKey: "lmstudio",
        api: "openai-responses",
        models: [
          {
            id: "minimax-m2.1-gs32",
            name: "MiniMax M2.1 GS32",
            reasoning: false,
            input: ["text"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 196608,
            maxTokens: 8192,
          },
        ],
      },
    },
  },
}
```

## 通过 `openacosmi configure` 配置

使用交互式配置向导设置 MiniMax，无需编辑 JSON：

1. 运行 `openacosmi configure`。
2. 选择 **Model/auth**。
3. 选择 **MiniMax M2.1**。
4. 根据提示选择默认模型。

## 配置选项

- `models.providers.minimax.baseUrl`：推荐 `https://api.minimax.io/anthropic`（Anthropic 兼容）；`https://api.minimax.io/v1` 可选（OpenAI 兼容）。
- `models.providers.minimax.api`：推荐 `anthropic-messages`；`openai-completions` 可选（OpenAI 兼容）。
- `models.providers.minimax.apiKey`：MiniMax API Key（`MINIMAX_API_KEY`）。
- `models.providers.minimax.models`：定义 `id`、`name`、`reasoning`、`contextWindow`、`maxTokens`、`cost`。
- `agents.defaults.models`：为允许列表中的模型设置别名。
- `models.mode`：保持 `merge` 以将 MiniMax 添加到内置模型旁。

## 注意事项

- 模型引用格式为 `minimax/<model>`。
- 编程计划用量 API：`https://api.minimaxi.com/v1/api/openplatform/coding_plan/remains`（需要编程计划密钥）。
- 如需精确成本追踪，请更新 `models.json` 中的定价值。
- MiniMax 编程计划推荐链接（9 折）：[https://platform.minimax.io/subscribe/coding-plan?code=DbXJTRClnb&source=link](https://platform.minimax.io/subscribe/coding-plan?code=DbXJTRClnb&source=link)
- 参见 [模型供应商概念](/concepts/model-providers) 了解供应商规则。
- 使用 `openacosmi models list` 和 `openacosmi models set minimax/MiniMax-M2.1` 切换模型。

## 故障排查

### "Unknown model: minimax/MiniMax-M2.1"

这通常意味着 **MiniMax 供应商未配置**（无供应商条目，且未找到 MiniMax 认证 profile/环境变量）。修复方法：

- 升级到 **2026.1.12**（或从源码 `main` 分支运行），然后重启 Gateway。
- 运行 `openacosmi configure` 并选择 **MiniMax M2.1**，或
- 手动添加 `models.providers.minimax` 配置块，或
- 设置 `MINIMAX_API_KEY`（或 MiniMax 认证 profile）以便供应商可以被注入。

确保模型 ID **区分大小写**：

- `minimax/MiniMax-M2.1`
- `minimax/MiniMax-M2.1-lightning`

然后使用以下命令验证：

```bash
openacosmi models list
```
