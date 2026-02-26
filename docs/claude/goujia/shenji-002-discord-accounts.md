> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 深度审计报告 #2：Discord `accounts.ts` ↔ `accounts.go`

- **TS 文件**: `src/discord/accounts.ts` (83L)
- **Go 文件**: `backend/internal/channels/discord/accounts.go` (226L)

---

## 1. 完美对齐的逻辑

| 模块 | 说明 |
|------|------|
| `ResolvedDiscordAccount` 类型 | Go struct 字段完整对应 TS type，`TokenSource` 使用自定义类型对应 TS `"env"\|"config"\|"none"` 联合类型 |
| `listConfiguredAccountIds` | TS `Object.keys(accounts).filter(Boolean)` ↔ Go map 迭代 + 空字符串过滤，等价 |
| `ListDiscordAccountIds` | 空配置时 fallback 到 `[DEFAULT_ACCOUNT_ID]`，两侧一致 |
| `ResolveDefaultDiscordAccountId` | 优先返回 default ID，fallback 到首个 ID，逻辑完全对齐 |
| `resolveAccountConfig` | 多层 nil guard (`cfg?.channels?.discord?.accounts`) 正确对应 TS 可选链 |
| `ResolveDiscordAccount` — enabled 判断 | `baseEnabled = channels.discord.enabled !== false`，Go 用 `*bool` 指针判空 + 取值，等价 TS `!== false`（undefined 视为 true） |
| `ResolveDiscordAccount` — name trim | TS `merged.name?.trim() \|\| undefined` ↔ Go `strings.TrimSpace` + 空串判断 |
| `ListEnabledDiscordAccounts` | `.map().filter()` ↔ Go for 循环 + append + enabled 过滤 |

## 2. 遗漏/偏离的逻辑

### 问题 1 (LOW)：排序方法差异
- **TS**: `ids.toSorted((a, b) => a.localeCompare(b))` — locale-aware 排序
- **Go**: `sort.Strings(ids)` — 字节序排序
- **影响**: 对纯 ASCII 账户 ID 无影响。若有 Unicode 账户 ID 可能排序不同

### 问题 2 (MEDIUM)：`mergeDiscordAccountConfig` 合并策略脆弱性
- **TS**: `{ ...base, ...account }` — 展开运算符自动合并所有字段
- **Go**: 逐字段手动合并 (accounts.go:90-176)，共 ~25 个字段
- **影响**: 若后续 `DiscordAccountConfig` 新增字段，TS 自动继承，Go 必须手动添加到 merge 函数
- **当前正确性**: 目前列举的字段覆盖完整，无遗漏

### 问题 3 (INFO — 改进)：Token 解析方式
- **TS**: `resolveDiscordToken(params.cfg, { accountId })` — 内部通过 `process.env.DISCORD_BOT_TOKEN` 获取环境变量
- **Go**: `ResolveDiscordToken(cfg, WithAccountID(id), WithEnvToken(os.Getenv("DISCORD_BOT_TOKEN")))` — 显式传入 env token
- **评估**: Go 的方式更好（显式依赖注入、更易测试），属于正向改进，无逻辑丢失

## 3. 外部依赖能力对照

| TS 依赖 | Go 对应 | 状态 |
|---------|---------|------|
| `../config/config.js` (`OpenAcosmiConfig`) | `pkg/types.OpenAcosmiConfig` | ✅ |
| `../config/types.js` (`DiscordAccountConfig`) | `pkg/types.DiscordAccountConfig` | ✅ |
| `../routing/session-key.js` (`DEFAULT_ACCOUNT_ID`, `normalizeAccountId`) | `account_id.go` (`defaultAccountID`, `NormalizeAccountID`) | ✅ |
| `./token.js` (`resolveDiscordToken`) | `token.go` (`ResolveDiscordToken`) | ✅ |

## 4. 结论

**accounts 模块审计通过**，无重大逻辑遗漏。仅有 2 个低风险注意项：
1. 排序方法差异（实际无影响）
2. 配置合并的字段枚举需维护同步（当前正确）

**状态**: 审计完成，无需修复
