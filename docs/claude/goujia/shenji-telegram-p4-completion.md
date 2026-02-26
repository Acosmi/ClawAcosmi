> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram P4 审计完成报告（Go 独有文件验证）

## 审计范围

5 个 Go 独有文件，验证 TS 源映射和行为对齐。

## 审计日期

2026-02-24

## 修复摘要

### http_client.go ↔ fetch.ts + proxy.ts

| 状态 | 内容 |
|------|------|
| PASS | 合理架构映射。Go 用 http.Transport 替代 undici ProxyAgent，SOCKS5 + HTTP/HTTPS 代理完整支持 |

### monitor_deps.go — Go 独有

| 状态 | 内容 |
|------|------|
| PASS | 有效 Go DI 文件，聚合 routing/auto-reply/pairing 等模块依赖，所有函数均有 TS 对照注释 |

### network.go ↔ allowed-updates.ts + network-config.ts + network-errors.ts

| 修复项 | 内容 |
|--------|------|
| NW-L1 死代码清理 | 移除未使用的 `recoverableErrorCodes` string map（仅 `recoverableSyscallErrnos` 被引用） |

### bridge/telegram_actions.go ↔ agents/tools/telegram-actions.ts

| 修复项 | 内容 |
|--------|------|
| TA-M1 ReadTelegramButtons 严格验证 | 签名改为 `([][]TelegramButton, error)`，添加 5 种 TS 对齐的验证（非数组行、非对象按钮、缺失字段、callback_data>64） |
| TA-M2 InlineButtonsScopeChecker | 新增可选接口，sendMessage/editMessage 通过 interface assertion 检查 inline buttons scope |
| TA-M3 ReactionLevelChecker | 新增可选接口，react 通过 interface assertion 检查 reaction level（在 actionGate 之前） |

### onboarding_telegram.go ↔ channels/plugins/onboarding/telegram.ts

| 修复项 | 内容 |
|--------|------|
| OT-H1 quickstartScore 对齐 | unconfigured=10（优先推荐新手）、configured=1，对齐 TS `configured ? 1 : 10` |
| OT-M1 selectionHint 对齐 | 添加 "recommended · configured" / "recommended · newcomer-friendly" 对齐 TS |
| OT-M2 API username 解析 | 新增 `resolveTelegramUserIDViaAPI` 通过 getChat API 解析 username→userID，PromptTelegramAllowFrom 添加重试循环 |
| 复核补丁: no-token fallback | token 为空时返回 "" 强制重试（对齐 TS 返回 null） |
| 复核补丁: allowFrom merge | 合并已有 allowFrom + 新解析 ID（对齐 TS 的 existingAllowFrom merge） |

## 复核审计结果

3 组并行交叉颗粒度审计已完成:
- onboarding_telegram.go (OT-H1 + OT-M1 + OT-M2): PASS（含 2 项复核补丁）
- bridge/telegram_actions.go (TA-M1 + TA-M2 + TA-M3): PASS
- network.go (NW-L1): PASS

## 残余已知差异（LOW，已确认可接受）

1. **http_client.go appliedAutoSelectFamily 单次锁**: TS 有全局缓存防止重复设置 autoSelectFamily，Go 无需此机制（Go net.Dialer 默认 Happy Eyeballs）。
2. **network.go Node 22 默认**: TS 对 Node 22+ 默认 `{value: false}`，Go 无此默认（Go 无 Node Happy Eyeballs 超时问题）。
3. **network.go collectErrorCandidates 遍历差异**: TS 遍历 .cause/.reason/.errors/.error(HttpError)，Go 使用 errors.Unwrap + multi-unwrap — Go 惯用等效方式。
4. **bridge/telegram_actions.go 扩展 actions**: Go 新增 forwardMessage/copyMessage/readMessages/poll/pin/admin 等 TS 无的 actions。
5. **onboarding_telegram.go 向导简化**: Go ConfigureTelegram 不支持 TS 的 shouldPromptAccountIds/accountOverrides/forceAllowFrom 等高级向导参数。
6. **bridge/telegram_actions.go sendSticker threading**: TS 传递 replyToMessageId/messageThreadId，Go 仅传 to/fileId。

## 修改文件清单

| 文件 | 修改类型 |
|------|----------|
| `network.go` | 死代码清理 (recoverableErrorCodes) |
| `bridge/telegram_actions.go` | ReadTelegramButtons 严格验证 + InlineButtonsScopeChecker + ReactionLevelChecker |
| `onboarding_telegram.go` | quickstartScore + selectionHint + API username 解析 + allowFrom merge |
