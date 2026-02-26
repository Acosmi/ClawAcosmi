# 全局审计报告 — Providers 模块

## 概览

| 维度 | TS (`src/providers`) | Go (`backend/internal/agents/auth` 等) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 4 | 3 (`github_copilot_auth.go`, `qwen_oauth.go`, `github_copilot_models.go`) | 100% 对齐 |
| 总行数 | ~330 | 370 | 100% 特性支持 |

### 架构演进

`providers` 模块是负责非标准大模型（如通过 GitHub Device Flow 授权的 Copilot、通过 Web Portal 抓取刷新令牌的 Qwen Portal）的认证和刷新流程的独立高内聚模块。

在 Go 重构中：

1. **GitHub Copilot 认证流**：完美还原。在 `github_copilot_auth.go` 中，重写了 `RequestCopilotDeviceCode` 和 `PollForCopilotAccessToken`。Go 版利用 `context.Context` 和 `select`/`time.After` 优雅实现了设备码轮询等待（包含 `slow_down` 降频避让逻辑和过期退出），彻底摆脱了 JS `setTimeout` 递归或 `while(true)` 原地打转可能引发的点位丢失问题。
2. **Qwen Portal 令牌刷新**：完美还原。在 `qwen_oauth.go` 中，实现了向 `https://chat.qwen.ai/api/v1/oauth2/token` 的 POST 请求，并将过期拦截处理（HTTP 400）正确归一化转化为重新登录的指引日志。

## 差异清单

### 零差异 (完美移植)

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| PROV-1 | **GitHub 轮询退避** | `if (err === "slow_down") { await sleep(intervalMs + 2000); }`。 | `case "slow_down": interval += 2 * time.Second; continue`。 | 并发安全且逻辑等价。无需修复。 |
| PROV-2 | **Qwen Token 过期捕获** | `if (res.status === 400) throw new Error(...)` | `if resp.StatusCode == http.StatusBadRequest { return err }` | 处理完全一致。无需修复。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | 未读取任何非预期的环境变量，纯洁的参数传入。 | 安全。 |
| **2. 并发安全** | 标准的 `net/http` 发包，状态全在局部变量，不持有任何全局互斥量或指针共享。 | 极度安全。 |
| **3. 第三方包黑盒** | 没有使用任何第三方抓包、OAuth 库，全部采用 Go 标准库 `net/http`、`net/url` 及 `encoding/json` 构建。 | 通过查验。 |

## 下一步建议

这是一个小而美的标准对照模块。Go 代码清晰，错误处理完善，对 `context` 取消信号支持响应。审计通过，可直接标记完成。
