> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# P3 延迟项清除 Sprint 6 复核审计

- **日期**: 2026-02-25
- **编号**: shenji-021
- **前置**: `shenji-019-4item-fix-fuhe-audit.md`（P3 11→9 修正）
- **目标**: 验证 Sprint 6 清除的 7 项 P3 延迟项 + 2 项架构阻塞更新
- **方法**: 逐项读取 Go 代码 + TS 源文件交叉对比，验证逻辑对齐

---

## 审计范围

| 编号 | 项目 | 类型 | 初审 | 复核 | 发现问题 |
|------|------|------|------|------|---------|
| S6-1 | HIDDEN-4 go-readability | 代码实现 | ✅ | ✅ | 无 |
| S6-2 | TS-MIG-MISC6 poll normalize | 代码实现 | ⚠️ | ✅ 已修正 | 3 处 TS 对齐差异 |
| S6-3 | TS-MIG-CLI2 profile 验证 | 代码实现 | ✅ | ✅ | `main.go` 调用方已修正 |
| S6-4 | TS-MIG-MISC2 macOS 平台 | 代码实现 | ⚠️ | ✅ 已修正 | 双重转义 bug + 注释不准确 |
| S6-5 | TS-MIG-MISC3 设备配对标记 | 文档标记 | ✅ | ✅ | 无 |
| S6-6 | TS-MIG-CLI3 wizard 确认 | 文档标记 | ✅ | ✅ | 无 |
| S6-7 | TS-MIG-CH3 Telegram 补全 | 代码+文档 | ✅ | ✅ | TS 无 sendMediaGroup（Go 新增能力） |
| — | TS-MIG-CH5 WhatsApp 阻塞 | 文档更新 | ✅ | ✅ | 无 |
| — | TS-MIG-CLI1 Rust CLI 替代 | 文档更新 | ✅ | ✅ | 无 |

---

## 复核发现与修正

### 发现 #1：`notify_darwin.go` 双重转义 bug（严重）

- **问题**：`fmt.Sprintf` 使用 `%q`（自动添加引号+转义），但调用前已执行 `escapeAppleScript()` 手动转义，造成双重转义。例如 `hello "world"` 会变成 `"hello \\\"world\\\""` 而非预期的 `"hello \"world\""`。
- **影响**：含双引号或反斜杠的通知标题/正文会显示错误。
- **修正**：`%q` → `"%s"`，手动拼接引号，保留 `escapeAppleScript()` 处理内部转义。
- **验证**：`go build ./internal/daemon/...` 通过。

### 发现 #2：`poll_normalize.go` TS 对齐差异 3 处（中等）

- **差异 A**：Go 原版做 case-insensitive 去重（`strings.ToLower`），TS 只做 `trim()` + `filter(Boolean)` 不去重。
  - **修正**：移除 case-insensitive 去重，对齐 TS 行为。
- **差异 B**：Go 原版函数签名 `NormalizePollInput(options []string, maxOptions int)` 只处理选项数组。TS 是完整的 `PollInput` 对象验证（含 question/maxSelections/durationHours）。
  - **修正**：重写为 `NormalizePollInput(input PollInput, maxOptions int) (NormalizedPollInput, error)`，新增 `PollInput`/`NormalizedPollInput` 结构体，完整对齐 TS 验证逻辑。
- **差异 C**：Go 原版 `NormalizePollDurationHours(hours, min, max)` 签名与 TS `normalizePollDurationHours(value, {defaultHours, maxHours})` 不同。TS 下限硬编码为 1，有 defaultHours 参数。
  - **修正**：签名改为 `NormalizePollDurationHours(value, defaultHours, maxHours)`，对齐 TS 语义。
- **测试**：从 5 个增至 8 个，覆盖 question 验证/选项不足/maxSelections 越界/durationHours floor 等场景。

### 发现 #3：`platform_darwin.go` 注释不准确（轻微）

- **问题**：注释声称"对应 TS src/macos/accessibility.ts"，但 TS `src/macos/` 实际包含 `relay.ts`(82L 入口)、`gateway-daemon.ts`(224L 守护进程)、`relay-smoke.ts`(37L) + 测试。无 `accessibility.ts` 或 `notification.ts`。
- **影响**：审计溯源误导。
- **修正**：注释改为"注：TS src/macos/ 中无直接对应文件（TS 侧为 daemon 入口），此为 Go 侧新增平台能力"。`deferred-items.md` MISC2 描述同步修正。

### 发现 #4：Telegram `send_media_group.go` — TS 无对应文件（信息性）

- **发现**：TS 代码库无 `send-media-group.ts`。TS Telegram 模块仅处理媒体组*接收*聚合（`bot-handlers.ts`），不做媒体组*发送*。
- **影响**：Go `SendMediaGroup()` 是新增能力，非 TS 迁移。
- **处置**：功能正确且有价值（Telegram Bot API 支持 sendMediaGroup），保留实现，文档已标注。

---

## 逐项验证明细

### S6-1: HIDDEN-4 go-readability ✅（复核通过）

- **文件**: `backend/internal/agents/tools/web_fetch.go:18` — `import readability "codeberg.org/readeck/go-readability/v2"`
- **函数**: `htmlToReadableMarkdown(rawHTML, pageURL string) string`（L496-518）
  - `readability.FromReader()` 提取正文 → `article.RenderHTML()` → `htmlToSimpleMarkdown()` 转 Markdown
  - 失败时 fallback 到 `htmlToSimpleMarkdown(rawHTML)`
  - 标题提取：`article.Title()` 非空时拼为 `# title`
- **测试**: 3 个测试验证正文提取、空 HTML fallback、无效 URL 处理
- **TS 对照**: `web-fetch-utils.ts extractReadableContent()` — `@mozilla/readability + linkedom` → Go 使用 `go-readability` 等价替代

### S6-2: TS-MIG-MISC6 poll normalize ✅（复核后修正通过）

- **新文件**: `backend/internal/cron/poll_normalize.go`（~90L，重写后）
- **结构体**: `PollInput` + `NormalizedPollInput`（对齐 TS types）
- **函数对照**:

| TS | Go | 对齐状态 |
|----|----|---------|
| `normalizePollInput(input, options)` | `NormalizePollInput(input, maxOptions)` | ✅ 完整对齐 |
| `input.question.trim()` + 非空校验 | `strings.TrimSpace` + `== ""` error | ✅ |
| `map(trim).filter(Boolean)` | `TrimSpace` + `!= ""` 过滤 | ✅ |
| `cleaned.length < 2` error | `len(cleaned) < 2` error | ✅ |
| `maxOptions` 上限校验 | `maxOptions > 0 && len > max` error | ✅ |
| `Math.floor(maxSelections)` + 范围 | `maxSelections` 默认 1 + 范围校验 | ✅ |
| `Math.floor(durationHours)` + `>= 1` | `math.Floor` + `< 1` error | ✅ |
| `normalizePollDurationHours(value, {default, max})` | `NormalizePollDurationHours(value, default, max)` | ✅ |
| `Math.min(Math.max(base, 1), max)` | `clamp [1, max]` | ✅ |

- **测试**: 8 个测试全部通过

### S6-3: TS-MIG-CLI2 profile 验证 ✅（复核通过）

- **正则**: `^[a-zA-Z0-9_-]+$`，长度 1-64
- **安全性**: 阻止 `../escape`、`has/slash`、`has.dot` 等路径穿越
- **签名变更**: `string` → `(string, error)`
- **调用方**: `cmd/openacosmi/main.go:52` 已修正为 `profile, profErr := cli.ResolveProfile(os.Args)`
- **全项目影响**: `go build ./...` 通过，无其他调用方

### S6-4: TS-MIG-MISC2 macOS 平台 ✅（复核后修正通过）

- **`CheckAccessibilityPermission()`**: osascript 探测 `System Events` 进程列表，无权限时 `err != nil` → 返回 false
- **`SendNotification()`**: 修正后使用 `"%s"` 手动拼接 + `escapeAppleScript()` 防注入
- **构建约束**: 两个文件均有 `//go:build darwin`
- **TS 对照修正**: TS `src/macos/` 实际是 daemon 入口（relay.ts + gateway-daemon.ts），非 Accessibility/通知。Go 侧为新增平台能力。

### S6-5: TS-MIG-MISC3 设备配对标记 ✅（复核通过）

- Go `internal/pairing/`（5 文件）+ `gateway/device_pairing.go`（843L）完整实现
- TS `src/pairing/`（5 文件 497L）功能重复
- 不删除 TS 文件以避免破坏构建

### S6-6: TS-MIG-CLI3 wizard 确认 ✅（复核通过）

- Go `wizard_*.go`（8 文件 3,200L）+ `setup_*.go`（14 文件）
- TS `src/wizard/`（10 文件）+ `configure.wizard.ts`（19KB）= ~2,209L
- 架构重设计：双路径（CLI setup + Gateway HTTP wizard）覆盖更广

### S6-7: TS-MIG-CH3 Telegram 补全 ✅（复核通过）

- **`send_media_group.go`**: sendMediaGroup API JSON 调用，1-10 项限制，速率限制集成
- **TS 对照**: TS 无 sendMediaGroup 发送功能（仅有接收聚合），Go 为新增能力
- **已有覆盖确认**:
  - 长轮询: `monitor.go:114-186` `startPollingMode()` — getUpdates + offset + 指数退避 + maxRetryTime
  - Webhook: `webhook.go:59-175` `StartTelegramWebhookServer()` — HTTP server + secret + 优雅关闭
  - 限速: `ratelimit.GlobalTelegramLimiter()` 令牌桶（等价 grammy throttler）
  - 媒体组接收: `bot_handlers.go:381-415` `addToMediaGroup()` + `flushMediaGroup()` 聚合

---

## 统计汇总

| 指标 | 值 |
|------|-----|
| 复核发现数 | 4（1 严重 + 1 中等 + 1 轻微 + 1 信息性） |
| 已修正数 | 3（双重转义 + poll 对齐 + 注释修正） |
| 新增/修改 Go 文件 | 5（poll_normalize.go 重写 + notify_darwin.go 修正 + platform_darwin.go 注释 + poll_normalize_test.go 重写 + deferred-items.md 修正） |
| 测试变化 | 5 → 8（poll）+ 其余不变 |
| P3 待办 | 仍为 2（复核未改变结论） |
| `go build ./...` | ✅ 通过 |
| `go test` 全量 | ✅ 通过 |

---

## 结论

复核审计发现 3 处需修正的问题，已全部修复并验证通过：

1. **`notify_darwin.go` `%q` 双重转义**（严重）→ 改为 `"%s"` + 手动转义
2. **`poll_normalize.go` TS 对齐缺失**（中等）→ 完整重写，新增 PollInput 结构体、验证逻辑、8 个测试
3. **`platform_darwin.go` 注释溯源错误**（轻微）→ 修正 TS 来源描述

Sprint 6 最终结果不变：**P3 待办 9 → 2**（CH5 WhatsApp 架构阻塞 + CLI1 Rust CLI 替代）。
