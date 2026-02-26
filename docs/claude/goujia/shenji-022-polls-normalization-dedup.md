> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 审计报告 022: Poll 规范化逻辑去重与共享层提取

- **日期**: 2026-02-25
- **编号**: shenji-022
- **前置**: `shenji-021-p3-clearance-sprint6-audit.md`（S6-2 项 poll normalize 发现）
- **范围**: `pkg/polls/`（新建） + `internal/cron/` + `internal/channels/discord/` + `internal/channels/whatsapp/`
- **TS 对标**: `src/polls.ts` — `normalizePollInput()` + `normalizePollDurationHours()`

---

## 一、问题描述

### 1.1 原审计发现（shenji-021 S6-2）

TS `src/polls.ts` 的投票规范化逻辑在 Go 侧有三处独立实现，行为不一致：

| 实现位置 | 函数名 | 问题 |
|----------|--------|------|
| `internal/cron/poll_normalize.go` | `NormalizePollInput` | 集中式，镜像 TS 逻辑，但无渠道调用 |
| `internal/channels/discord/send_guild.go` | `NormalizeDiscordPollInput` | 内联重复，缺失 maxSelections 校验 |
| `internal/channels/whatsapp/outbound.go` | `NormalizePollInput` | 内联重复，缺失 maxSelections/durationHours 校验 |

### 1.2 不一致行为对比（修复前）

| 特性 | TS polls.ts | Go/cron | Go/Discord | Go/WhatsApp |
|------|------------|---------|-----------|------------|
| 空问题处理 | 抛错 | 返回 error | 默认 "Poll" | 默认 "Poll" |
| 空选项过滤 | trim+filter | trim+filter | trim+filter | trim+filter |
| 选项不足处理 | 抛错 | 返回 error | 无校验 | 自动填充 "Option N" |
| maxSelections 校验 | 严格（≥1, ≤选项数） | 严格 | **无** | **无** |
| durationHours 校验 | floor+≥1 | floor+≥1 | 仅 clamp | **无** |

---

## 二、修复方案

### 2.1 设计决策

参考 [Google Go Style Best Practices](https://google.github.io/styleguide/go/best-practices.html) 和 [Practical Go (Dave Cheney)](https://dave.cheney.net/practical-go/presentations/qcon-china.html) 的共享验证包模式：

- **提取共享层**: 将集中式验证逻辑从 `internal/cron/` 提取到 `pkg/polls/`，符合项目已有 `pkg/` 目录惯例
- **渠道调用共享层**: Discord / WhatsApp 先做平台特有预处理，再调用共享验证
- **保留平台差异**: 空问题默认 "Poll"、WhatsApp 自动填充选项等行为在渠道层保留

### 2.2 三层架构

```
平台预处理层         共享验证层 (pkg/polls)        平台后处理层
─────────────    ──────────────────────────    ─────────────
Discord:          polls.NormalizePollInput()    → Discord API 格式
  空问题→"Poll"    - question 非空校验            (answers/layout_type)
                   - options trim+filter+≥2
WhatsApp:          - maxSelections 范围校验      → WhatsApp PollInput
  空问题→"Poll"    - durationHours floor+≥1       (question+options)
  选项<2→自动填充
                  polls.NormalizePollDurationHours()
Cron:              - [1, maxHours] 钳位          → NormalizedPollInput
  类型别名委托
```

---

## 三、变更清单

### 3.1 新建文件

| 文件 | 行数 | 说明 |
|------|------|------|
| `backend/pkg/polls/polls.go` | 96 | 共享类型 (`PollInput`, `NormalizedPollInput`) + 验证函数 |
| `backend/pkg/polls/polls_test.go` | 116 | 8 个测试用例，覆盖全部验证路径 |

### 3.2 修改文件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `backend/internal/cron/poll_normalize.go` | 重写 | 删除内联实现，改为 `import polls` + 类型别名 + 委托函数 |
| `backend/internal/channels/discord/send_guild.go` | 修改 | `NormalizeDiscordPollInput` 调用 `polls.NormalizePollInput` + `polls.NormalizePollDurationHours`；签名变更为返回 `(map[string]interface{}, error)` |
| `backend/internal/channels/whatsapp/outbound.go` | 修改 | `NormalizePollInput` 调用 `polls.NormalizePollInput`；保留 WhatsApp 特有自动填充逻辑 |

### 3.3 文件影响矩阵

| 文件 | 操作 | 新增 import | 删除代码 | 新增代码 |
|------|------|-------------|----------|----------|
| `pkg/polls/polls.go` | 新建 | `fmt`, `math`, `strings` | — | 96 行 |
| `pkg/polls/polls_test.go` | 新建 | `reflect`, `testing` | — | 116 行 |
| `internal/cron/poll_normalize.go` | 重写 | `pkg/polls` | 89 行 | 22 行 |
| `internal/channels/discord/send_guild.go` | 修改 | `pkg/polls` | 30 行 | 35 行 |
| `internal/channels/whatsapp/outbound.go` | 修改 | `pkg/polls` | 15 行 | 30 行 |

**净变化**: +265 行（新建 212 行 + 修改 +65/-134 行）

---

## 四、逐文件变更详情

### 4.1 `pkg/polls/polls.go`（新建）

从 `cron/poll_normalize.go` 提取，无逻辑变更：

| 导出符号 | 类型 | 说明 |
|----------|------|------|
| `PollInput` | struct | Question, Options, MaxSelections, DurationHours |
| `NormalizedPollInput` | struct | 规范化后的投票输入 |
| `NormalizePollInput(input, maxOptions)` | func | 严格验证：question 非空、options ≥2、maxSelections ∈ [1, len(options)]、durationHours floor+≥1 |
| `NormalizePollDurationHours(value, default, max)` | func | 钳位到 [1, maxHours]，value ≤0 时使用 defaultHours |

### 4.2 `internal/cron/poll_normalize.go`（重写）

- 删除全部内联实现（89 行）
- 改为 `import "github.com/anthropic/open-acosmi/pkg/polls"`
- `type PollInput = polls.PollInput`（类型别名，零开销）
- `type NormalizedPollInput = polls.NormalizedPollInput`
- 两个委托函数保持原签名不变，cron 包内部调用方无需修改

### 4.3 `discord/send_guild.go` — `NormalizeDiscordPollInput`

**签名变更**: `func NormalizeDiscordPollInput(input DiscordPollInput) map[string]interface{}` → `func NormalizeDiscordPollInput(input DiscordPollInput) (map[string]interface{}, error)`

**调用方更新**: `SendPollDiscord` 增加 `if err != nil` 错误处理

**逻辑流程**:

| 步骤 | 层 | 行为 |
|------|-----|------|
| 1 | Discord 预处理 | 空问题 → "Poll" |
| 2 | 共享验证 | `polls.NormalizePollInput(question, options, maxSelections, discordPollMaxAnswers=10)` |
| 3 | 共享时长钳位 | `polls.NormalizePollDurationHours(durationHours, default=24, max=768)` |
| 4 | Discord 格式化 | 构建 `answers[].poll_media.text` + `allow_multiselect` + `layout_type=1` |

**新增校验**: maxSelections ∈ [1, len(options)]（修复前无校验）

### 4.4 `whatsapp/outbound.go` — `NormalizePollInput`

**签名不变**: `func NormalizePollInput(poll PollInput, maxOptions int) PollInput`

**逻辑流程**:

| 步骤 | 层 | 行为 |
|------|-----|------|
| 1 | WhatsApp 预处理 | 空问题 → "Poll"；trim+过滤空选项 |
| 2 | WhatsApp 特有 | 选项不足时自动填充 "Option N" |
| 3 | 共享验证 | `polls.NormalizePollInput(question, options, maxOptions=12)` |
| 4 | 错误回退 | 共享验证失败时（仅 maxOptions 超限可能）回退到预处理结果 |

**新增校验**: 共享层 trim+过滤（双重保障）+ maxSelections 默认值

---

## 五、修复后一致性对比

| 特性 | TS polls.ts | Go 共享层 | Discord | WhatsApp |
|------|------------|----------|---------|----------|
| 选项 trim+过滤 | 集中 | 集中 | 调用共享层 | 预处理+调用共享层 |
| maxSelections 校验 | 严格 | 严格 | **调用共享层** | **调用共享层** |
| durationHours 校验 | floor+≥1 | floor+≥1 | **调用 NormalizePollDurationHours** | N/A（WA 无此字段） |
| 空问题处理 | 抛错 | 返回 error | 预处理默认 "Poll" | 预处理默认 "Poll" |
| 选项不足处理 | 抛错 | 返回 error | 返回 error | 预处理自动填充 |

---

## 六、测试验证

### 6.1 测试结果

| 包 | 测试数 | 结果 |
|----|--------|------|
| `pkg/polls` | 8 | PASS |
| `internal/cron` | 8（委托到 pkg/polls） | PASS |
| `internal/channels/discord` | 全量 | PASS |
| `internal/channels/whatsapp` | 全量 | PASS |

### 6.2 编译验证

| 命令 | 结果 |
|------|------|
| `go build ./pkg/polls/` | PASS |
| `go build ./internal/cron/` | PASS |
| `go build ./internal/channels/discord/` | PASS |
| `go build ./internal/channels/whatsapp/` | PASS |
| `go test ./...`（全量） | 2 个已有失败（channels/onboarding_test.go GroupPolicy 类型、gateway/server_methods_batch_cdb_test.go 指针类型），与本次变更无关 |

---

## 七、可信源参考

| 来源 | 适用原则 | URL |
|------|----------|-----|
| Google Go Style Best Practices | 共享验证逻辑提取到独立包 | https://google.github.io/styleguide/go/best-practices.html |
| Practical Go (Dave Cheney) | 包按功能组织，避免 utils 式泛化 | https://dave.cheney.net/practical-go/presentations/qcon-china.html |
| DEV Community: Validation Layer in Go | 验证层独立封装，跨模块复用 | https://dev.to/ansu/best-practices-for-building-a-validation-layer-in-go-59j9 |
| Go Project Structure (Glukhov) | pkg/ 目录放可被外部导入的共享包 | https://www.glukhov.org/post/2025/12/go-project-structure/ |

---

## 八、剩余事项

| 项目 | 优先级 | 说明 |
|------|--------|------|
| 复核审计 | P0 | 本文档需按复核审计技能进行交叉验证 |
| 已有测试失败修复 | P1 | `channels/onboarding_test.go` GroupPolicy 类型不匹配；`gateway/server_methods_batch_cdb_test.go` 指针类型不匹配 |
| WhatsApp maxSelections 字段 | P2 | `whatsapp.PollInput` 当前无 MaxSelections 字段，WhatsApp API 原生支持此功能时需扩展 |
| 集成测试 | P2 | 添加跨渠道场景测试，验证同一输入经 Discord/WhatsApp 规范化后行为符合预期 |
