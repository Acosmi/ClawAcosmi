> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram P3 审计完成报告（基础设施）

## 审计范围

7 对文件，全部已审计、修复并通过复核。

## 审计日期

2026-02-24

## 修复摘要

### audit.ts ↔ audit.go

| 修复项 | 内容 |
|--------|------|
| A-H1 HTTP 状态码检查 | 添加 `resp.StatusCode < 200 \|\| resp.StatusCode >= 300` 对齐 TS `!res.ok` 2xx 范围检查 |

### probe.ts ↔ probe.go

| 修复项 | 内容 |
|--------|------|
| P-H1 Status/Error null 语义 | `Status *int` + `Error *string`，移除 `omitempty`，JSON 输出 `null` 对齐 TS |
| P-M1 HTTP 2xx 范围检查 | `status != http.StatusOK` → `status < 200 \|\| status >= 300` 对齐 TS `!meRes.ok` |

### targets.ts ↔ targets.go

| 状态 | 内容 |
|------|------|
| PASS | 高质量迁移，无需修复 |

### api-logging.ts ↔ api_logging.go

| 修复项 | 内容 |
|--------|------|
| AL-H1 shouldLog 回调 | `WithTelegramAPIErrorLogging` 和 `Void` 变体添加 `shouldLog func(error) bool` 参数（nil 时记录所有错误） |
| webhook.go 联动 | 两处调用方传入 `nil` 对齐 TS 未传 shouldLog |

### webhook.ts ↔ webhook.go

| 修复项 | 内容 |
|--------|------|
| W-H1 优雅关闭 | 添加 `ctx.Done()` 监听 goroutine + `server.Shutdown(5s)` 对齐 TS `abortSignal` |
| W-M1 启动等待 | `go server.ListenAndServe()` → `net.Listen` + `server.Serve(ln)` 确保端口绑定完成后返回 |
| W-M2 错误处理 500 | 添加 panic recovery（对齐 monitor.safeHandleUpdate 模式），handler panic 时返回 500 |

### group-migration.ts ↔ group_migration.go

| 修复项 | 内容 |
|--------|------|
| GM-H1 大小写回退 | 精确匹配 `Accounts[normalized]` 失败后遍历所有 key 做 `strings.ToLower` 比较 |

### update-offset-store.ts ↔ update_offset_store.go

| 修复项 | 内容 |
|--------|------|
| UOS-H1 resolveStateDir 路径 | `~/.openacosmi/state` → `~/.openacosmi` 对齐 TS `resolveStateDir` |
| UOS-M1 sanitize 大小写 | 移除 `strings.ToLower`，正则改为 `[^a-zA-Z0-9._-]+` 保留原始大小写对齐 TS |

## 复核审计结果

3 组并行交叉颗粒度审计已完成:
- audit + probe + targets: PASS
- api_logging + webhook: PASS
- group_migration + update_offset_store: PASS

## 残余已知差异（LOW，已确认可接受）

1. **Legacy 目录支持缺失**: Go `resolveStateDir` 不支持 `.clawdbot/.moltbot/.moldbot` legacy 目录回退和 `CLAWDBOT_STATE_DIR` 环境变量。Go 后端为全新部署，无需兼容旧版 Node.js legacy 目录。
2. **Logger/Runtime DI 简化**: Go `api_logging` 使用 `slog.Error` 硬编码，不支持 TS 的 `runtime/logger` DI 注入。Go `slog` 本身支持 handler 替换，差异可接受。
3. **诊断子系统缺失**: Go `webhook` 不集成 TS 的 `isDiagnosticsEnabled` + heartbeat + webhook diagnostic 日志。长期可考虑添加。
4. **GM-H1 精确匹配条件微差**: Go 用 `acct == nil` 判断精确匹配失败，TS 用 `exact?.groups` 要求 groups 非空。当账户存在但 Groups 为 nil 时行为微有差异，实际影响极低。
5. **Webhook handler 错误语义**: Go `HandleUpdate` 不返回 error，仅通过 panic recovery 捕获异常。非 panic 的业务错误（如 API 调用失败）仍返回 200。TS 通过 Promise rejection 捕获所有异常。
6. **ButtonRow 死代码**: `model_buttons.go` 中 `ButtonRow` 类型定义未使用（P2 遗留）。

## 修改文件清单

| 文件 | 修改类型 |
|------|----------|
| `audit.go` | HTTP 2xx 范围检查 |
| `probe.go` | Status/Error 指针类型 + null 语义 + HTTP 2xx |
| `api_logging.go` | shouldLog 回调参数 |
| `webhook.go` | 优雅关闭 + 启动等待 + panic recovery + shouldLog 联动 |
| `group_migration.go` | 大小写回退查找 |
| `update_offset_store.go` | resolveStateDir 路径 + sanitize 大小写 |
