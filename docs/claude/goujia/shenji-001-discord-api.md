> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 深度审计报告 #1：Discord `api.ts` ↔ `api.go`

- **TS 文件**: `src/discord/api.ts` (137L)
- **Go 文件**: `backend/internal/channels/discord/api.go` (193L)

---

## 1. 完美对齐的逻辑

| 模块 | 说明 |
|------|------|
| API Base URL | 两侧均为 `https://discord.com/api/v10` |
| `parseDiscordApiErrorPayload` | JSON 花括号前缀/后缀守卫 + JSON 解析，逻辑等价 |
| `parseRetryAfterSeconds` | 先查 body payload，fallback 到 `Retry-After` header，`IsInf`/`IsNaN` 守卫等价 `Number.isFinite` |
| `formatRetryAfterSeconds` | `< 10` 用 1 位小数，`>= 10` 取整，逻辑完全一致 |
| `formatDiscordApiErrorText` | 空串判断、JSON 解析失败降级 `"unknown error"`、message trim、retry_after 拼接 — 全部对齐 |
| `DiscordApiError` | Go struct `*float64` 对应 TS `retryAfter?: number`，`Error()` 返回 Message |
| 错误消息格式 | `"Discord API %s failed (%d)%s"` 与 TS 模板字面量完全匹配 |
| 429 限速重试 | `ShouldRetry` 仅对 429 重试，`RetryAfterHint` 秒→毫秒换算正确 |
| Auth Header | `"Bot " + token` 两侧一致 |
| `context.Context` | Go 通过 `http.NewRequestWithContext(ctx, ...)` 正确接管 TS 隐式 async 上下文 |
| `discordAPIErrorPayload` struct | `Code *int`, `Global *bool` 使用指针 + `omitempty`，完美兼容 TS optional 的 `undefined` 语义 |

## 2. 遗漏/偏离的逻辑

### 问题 1 (MEDIUM)：HTTP Client 不可注入
- **TS**: `fetchDiscord(path, token, fetcher = fetch, options?)` — 第 3 参数允许注入自定义 fetch
- **Go**: 硬编码 `http.DefaultClient.Do(req)`，无注入点
- **影响**: 测试须依赖 `httptest.Server` 或全局替换 `http.DefaultTransport`；无法像 TS 那样支持自定义代理

### 问题 2 (HIGH)：Retry Config 合并策略不同
- **TS**: `resolveRetryConfig(defaults, options?.retry)` — 深度合并，只覆盖部分字段
- **Go**: `if opts.Retry != nil { retryCfg = *opts.Retry }` — 整体替换
- **缓解**: `pkg/retry.resolveConfig()` 会将零值字段回填为**包级**默认值（非 Discord 专属默认值），部分降低风险
- **残余风险**: 包默认值与 `discordAPIRetryDefaults` 可能不同

### 问题 3 (LOW)：Jitter 语义差异
- **TS**: `jitter: 0.1` — 10% 抖动比例
- **Go**: `Jitter: true` — 布尔开关，`pkg/retry` 实现为 **±25%** 随机抖动
- **差异**: 抖动幅度 10% vs ±25%（已从 `pkg/retry/retry.go:151-182` 确认）

## 3. 外部依赖能力对照

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `../infra/fetch.js` (`resolveFetch`) | `http.DefaultClient` | ⚠️ 功能降级（不可注入） |
| `../infra/retry.js` (`retryAsync`, `resolveRetryConfig`) | `pkg/retry` (`DoWithResult`, `resolveConfig`) | ⚠️ 缺少深度合并等价函数 |

## 4. 建议修复草稿

### 修复 1：HTTP Client 注入
```go
type DiscordFetchOptions struct {
    Retry  *retry.Config
    Label  string
    Client *http.Client // 新增
}

// FetchDiscord 内：
client := http.DefaultClient
if opts != nil && opts.Client != nil {
    client = opts.Client
}
resp, err := client.Do(req)
```

### 修复 2：Retry Config 深度合并
```go
func mergeRetryConfig(base retry.Config, override *retry.Config) retry.Config {
    if override == nil {
        return base
    }
    merged := base
    if override.MaxAttempts > 0 {
        merged.MaxAttempts = override.MaxAttempts
    }
    if override.InitialDelay > 0 {
        merged.InitialDelay = override.InitialDelay
    }
    if override.MaxDelay > 0 {
        merged.MaxDelay = override.MaxDelay
    }
    return merged
}
```

**状态**: 待用户指令后修复
