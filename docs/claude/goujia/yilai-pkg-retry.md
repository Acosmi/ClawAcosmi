> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 外部依赖分析：`pkg/retry` 包

- **路径**: `backend/pkg/retry/retry.go`
- **授权范围**: 用户授权查看该包以确认 Jitter 实现

---

## Config 结构体

```go
type Config struct {
    MaxAttempts    int
    InitialDelay   time.Duration
    MaxDelay       time.Duration
    Multiplier     float64
    Jitter         bool                              // 布尔开关（非比例值）
    ShouldRetry    func(err error, attempt int) bool
    OnRetry        func(info RetryInfo)
    RetryAfterHint func(err error) time.Duration
    Label          string
}
```

## Jitter 实现细节

```go
if cfg.Jitter {
    // 添加 ±25% 随机抖动
    jitterRange := delay * 0.25
    delay += (rand.Float64()*2 - 1) * jitterRange
}
```

- **类型**: `bool`（开/关）
- **抖动幅度**: **±25%**（即 delay 在 75%~125% 范围浮动）
- **对比 TS**: TS 使用 `jitter: 0.1` 即 ±10% 抖动

## `resolveConfig` 函数

```go
func resolveConfig(cfg Config) Config {
    if cfg.MaxAttempts <= 0 { cfg.MaxAttempts = DefaultConfig.MaxAttempts }
    if cfg.InitialDelay <= 0 { cfg.InitialDelay = DefaultConfig.InitialDelay }
    if cfg.MaxDelay <= 0 { cfg.MaxDelay = DefaultConfig.MaxDelay }
    if cfg.Multiplier <= 0 { cfg.Multiplier = DefaultConfig.Multiplier }
    if cfg.MaxDelay < cfg.InitialDelay { cfg.MaxDelay = cfg.InitialDelay }
    return cfg
}
```

- 该函数在 `DoWithResult` 内部调用，会将零值字段回填为**包级默认值**
- 这意味着即使 `api.go` 整体替换了 retry config，零值字段不会导致无延迟重试
- **但**: 回填的是 `DefaultConfig` 而非 `discordAPIRetryDefaults`，两者可能不同

## `DoWithResult` 签名

```go
func DoWithResult[T any](ctx context.Context, cfg Config, fn func(attempt int) (T, error)) (T, error)
```

- 泛型函数，使用 Go 1.18+ 泛型
- 内部调用 `resolveConfig(cfg)` 处理默认值

## 对审计的影响

| Discord api.go 问题 | 影响评估 |
|---|---|
| Retry Config 整体替换 | 风险降低：`resolveConfig` 兜底零值，但默认值来源不同 |
| Jitter 差异 | 确认差异：10% vs ±25%，实际影响较小（重试间隔本身就有随机性） |
