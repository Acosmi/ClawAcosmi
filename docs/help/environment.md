---
summary: "OpenAcosmi 环境变量加载位置和优先级顺序"
read_when:
  - 需要了解哪些环境变量被加载及其顺序
  - 调试 Gateway 中缺失的 API 密钥
  - 记录 Provider 认证或部署环境配置
title: "环境变量"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# 环境变量

OpenAcosmi 从多个来源获取环境变量。规则是**不覆盖已有的值**。

## 优先级（从高到低）

1. **进程环境** — Gateway 进程从父 shell / 守护进程继承的环境变量。
2. **当前工作目录下的 `.env`** — dotenv 默认行为，不覆盖已有值。
3. **全局 `.env`** — 位于 `~/.openacosmi/.env`（即 `$OPENACOSMI_STATE_DIR/.env`），不覆盖已有值。
4. **配置文件 `env` 块** — 在 `~/.openacosmi/openacosmi.json` 中，仅当环境变量缺失时应用。
5. **可选的登录 shell 导入** — 通过 `env.shellEnv.enabled` 或 `OPENACOSMI_LOAD_SHELL_ENV=1` 启用，仅对缺失的期望键生效。

如果配置文件完全不存在，则跳过第 4 步；Shell 导入仍在启用时运行。

## 配置文件 `env` 块

两种等效方式设置内联环境变量（均不覆盖已有值）：

```json5
{
  env: {
    OPENROUTER_API_KEY: "sk-or-...",
    vars: {
      GROQ_API_KEY: "gsk-...",
    },
  },
}
```

## Shell 环境导入

`env.shellEnv` 运行您的登录 shell 并仅导入**缺失的**期望键：

```json5
{
  env: {
    shellEnv: {
      enabled: true,
      timeoutMs: 15000,
    },
  },
}
```

等效环境变量：

- `OPENACOSMI_LOAD_SHELL_ENV=1`
- `OPENACOSMI_SHELL_ENV_TIMEOUT_MS=15000`

## 配置中的环境变量替换

可以在配置字符串值中使用 `${VAR_NAME}` 语法直接引用环境变量：

```json5
{
  models: {
    providers: {
      "vercel-gateway": {
        apiKey: "${VERCEL_GATEWAY_API_KEY}",
      },
    },
  },
}
```

详见 [配置：环境变量替换](/gateway/configuration#env-var-substitution-in-config)。

## 路径相关环境变量

| 变量                   | 用途                                                                                                                                                                          |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `OPENACOSMI_HOME`        | 覆盖所有内部路径解析使用的主目录（`~/.openacosmi/`、agent 目录、会话、凭据）。适用于以专用服务用户运行 OpenAcosmi 的场景。 |
| `OPENACOSMI_STATE_DIR`   | 覆盖状态目录（默认 `~/.openacosmi`）。                                                                                                                                         |
| `OPENACOSMI_CONFIG_PATH` | 覆盖配置文件路径（默认 `~/.openacosmi/openacosmi.json`）。                                                                                                                       |

### `OPENACOSMI_HOME`

设置后，`OPENACOSMI_HOME` 将替代系统主目录（`$HOME`）用于所有内部路径解析。这使得无头服务账户可以实现完整的文件系统隔离。

**优先级：** `OPENACOSMI_HOME` > `$HOME` > `USERPROFILE` > 系统默认主目录

**示例**（macOS LaunchDaemon）：

```xml
<key>EnvironmentVariables</key>
<dict>
  <key>OPENACOSMI_HOME</key>
  <string>/Users/kira</string>
</dict>
```

`OPENACOSMI_HOME` 也可以设置为带波浪号的路径（如 `~/svc`），使用前会通过 `$HOME` 展开。

## 相关文档

- [Gateway 配置](/gateway/configuration)
- [FAQ：环境变量和 .env 加载](/help/faq#env-vars-and-env-loading)
- [模型概览](/concepts/models)
