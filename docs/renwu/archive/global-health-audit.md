# 全局代码健康度审计报告

> **审计日期**：2026-02-18
> **范围**：Go 后端全量（626 源文件 + 48 pkg 文件 + 19 cmd 文件）vs TS 原项目（1,658 非测试文件）
> **方法**：`go build` / `go vet` / `go test` + panic/错误忽略扫描 + TS↔Go 模块覆盖对比

---

## 一、构建与静态分析

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | ✅ 零错误 |
| `go vet ./...` | ✅ 零警告 |
| `go test ./...` | ❌ **1 FAIL** — `TestStubHandlers_AllRespond` |

### BUG-1: `TestStubHandlers_AllRespond` 失败

- **文件**: `internal/gateway/server_methods_batch_cdb_test.go:319`
- **原因**: W9 实现 wizard 后，`wizard.start` 从 `StubHandlers()` 移到 `WizardHandlers()`，但测试仍期望它在 stub 列表中
- **修复**: 从测试的 `testMethods` 列表中移除 `"wizard.start"`
- **优先级**: 🔴 P0（CI 阻塞）

---

## 二、代码规范违规

### 2.1 业务代码中的 panic（违反编码规范）

| 文件 | 行号 | 代码 |
|------|------|------|
| `channels/discord/send_shared.go` | L71 | `panic(fmt.Sprintf(...))` |
| `channels/discord/send_shared.go` | L173 | `panic("emoji required")` |

> 编码规范明确禁止在业务代码中使用 `panic`。应改为 `return fmt.Errorf(...)` 并上层处理。

### 2.2 被忽略的错误（`_ = err`）

共 ~20 处，多数为合理忽略（端口关闭/目录创建等），但以下值得关注：

| 文件 | 问题 |
|------|------|
| `infra/discovery.go:195,248` | `Port, _ = strconv.Atoi(m[2])` — 格式错误时 Port 默认 0 |
| `memory/schema.go:133` | `_, _ = db.Exec(ALTER TABLE...)` — DDL 失败静默 |
| `memory/manager.go:407,435` | `_, _ = tx.Exec(DELETE/INSERT...)` — 事务操作失败静默 |
| `infra/exec_approvals.go:221` | `_, _ = rand.Read(buf)` — 安全敏感操作 |

---

## 三、测试覆盖

| 指标 | 数值 |
|------|------|
| 测试文件总数 | 157 |
| 有测试的包 | 35 |
| **缺测试文件的包** | **17** |

### 缺测试文件的关键包

| 包 | 风险评估 |
|----|----------|
| `hooks/gmail` | ⚠️ 外部集成，应有 mock 测试 |
| `linkparse` | ⚠️ 链接检测解析，边界情况多 |
| `media/understanding` | ⚠️ 多 provider 路由逻辑 |
| `routing` | ⚠️ session key 路由（340L） |
| `tts` | ⚠️ 8 文件无任何测试 |
| `session` (单独) | 低风险，仅类型定义 |
| `contracts` | 低风险，仅接口定义 |
| `ollama` | 低风险，Phase 12 推迟项 |

---

## 四、TS↔Go 模块覆盖对比

> 以下为每个 TS 顶层模块的移植状态

| TS 模块 | TS 文件数 | Go 对应包 | 移植状态 |
|---------|-----------|-----------|----------|
| `agents/` | 233 | `agents/*` (10 子包) | ✅ 核心完成 |
| `commands/` | 174 | `autoreply/commands_*` + `cli/cmd_*` | ✅ 完成 |
| `cli/` | 138 | `cli/` + `cmd/openacosmi/` | ✅ Cobra 替代 |
| `gateway/` | 133 | `gateway/` (83 文件) | ✅ 核心完成 |
| `auto-reply/` | 121 | `autoreply/` + `autoreply/reply/` | ✅ 完成 |
| `infra/` | 120 | `infra/` (11 文件) | ✅ 完成 |
| `config/` | 89 | `config/` + `pkg/types/` | ✅ 完成 |
| `channels/` | 77 | `channels/` (7 子包) | ✅ 完成 |
| `browser/` | 52 | `browser/` (8 文件) | ✅ 完成 |
| `discord/` | 44 | `channels/discord/` | ✅ 完成 |
| `slack/` | 43 | `channels/slack/` | ✅ 完成 |
| `web/` | 43 | `channels/whatsapp/` | ✅ 完成 |
| `telegram/` | 40 | `channels/telegram/` | ✅ 完成 |
| `plugins/` | 29 | `plugins/` (16 文件) | ✅ 完成 |
| `memory/` | 28 | `memory/` (20 文件) | ✅ 完成 |
| `media-understanding/` | 25 | `media/understanding/` | ✅ 完成 |
| `tui/` | 24 | — | ⏭️ **CLI 已用 Cobra 替代** |
| `hooks/` | 22 | `hooks/` (12 文件) | ✅ 完成 |
| `cron/` | 22 | `cron/` (17 文件) | ✅ 完成 |
| `line/` | 21 | `channels/line/` | ⚠️ 骨架级 |
| `daemon/` | 19 | `daemon/` (21 文件) | ✅ 完成 |
| `signal/` | 14 | `channels/signal/` | ✅ 完成 |
| `imessage/` | 12 | `channels/imessage/` | ✅ 完成 |
| `media/` | 11 | `media/` (11 文件) | ✅ 完成 |
| `utils/` | 10 | `pkg/utils/` | ✅ 完成 |
| `terminal/` | 10 | — | ⏭️ CLI 替代 |
| `logging/` | 10 | `pkg/log/` | ✅ 完成 |
| `acp/` | 10 | `acp/` (9 文件) | ✅ 完成 |
| `types/` | 9 | `pkg/types/` (29 文件) | ✅ 完成 |
| `security/` | 8 | `security/` (7 文件) | ✅ 完成 |
| `wizard/` | 7 | `gateway/wizard_*` | ✅ W9 完成 |
| `sessions/` | 6 | `sessions/` (7 文件) | ✅ 完成 |
| `process/` | 5 | `agents/runner/` 内含 | ✅ 嵌入 |
| `providers/` | 4 | `agents/llmclient/` | ✅ 完成 |
| `pairing/` | 3 | `gateway/device_pairing.go` | ✅ W6 完成 |
| `canvas-host/` | 2 | — | ⏭️ 独立功能 |
| `node-host/` | 2 | — | ⏭️ 远程执行 Phase 12+ |
| `link-understanding/` | — | `linkparse/` | ✅ 完成 |
| `markdown/` | — | `pkg/markdown/` | ✅ 完成 |
| `tts/` | — | `tts/` (8 文件) | ✅ 完成 |
| `shared/` | 1 | — | ✅ 嵌入其他包 |
| `compat/` | 1 | — | ✅ N/A（TS 兼容层） |
| `plugin-sdk/` | 1 | `plugins/` 内含 | ✅ 嵌入 |
| `test-helpers/` | 2 | — | ✅ N/A（测试工具） |
| `test-utils/` | 2 | — | ✅ N/A |

---

## 五、已知待修复项（来自 deferred-items.md）

| ID | 问题 | 优先级 |
|----|------|--------|
| P11-C-P1-4 | block-streaming 管线（554L TS 缺 Go） | 🔴 P1 |
| AUDIT-1 | `extractEnumValues` 缺 const 处理 | 🟡 P2 |
| AUDIT-2 | `extractEnumValues` 缺递归嵌套提取 | 🟡 P2 |
| AUDIT-3 | `required` 合并应基于 count 判断 | 🟡 P2 |
| AUDIT-4 | 缺 `additionalProperties` 保留 | 🟡 P2 |
| AUDIT-5 | early-return 和 fallback 差异 | 🟡 P2 |
| AUDIT-6 | `BuildAllowedModelSet` 缺 configuredProviders | 🟡 P2 |
| AUDIT-7 | `promoteThinkingTagsToBlocks` 缺 guard | 🟡 P2 |
| P11-1 | Ollama 集成 | ⚪ P3（Phase 12） |
| P11-2 | 前端 i18n（275 key） | ⚪ P3（Phase 12） |

---

## 六、本次审计新发现汇总

| 编号 | 类别 | 问题 | 优先级 | 修复建议 |
|------|------|------|--------|----------|
| NEW-1 | 🐛 Bug | `TestStubHandlers_AllRespond` 失败 | 🔴 P0 | 移除 `"wizard.start"` 条目 |
| NEW-2 | ⚠️ 规范 | Discord `send_shared.go` 含 2 处 panic | 🟡 P2 | 改为 error 返回 |
| NEW-3 | ⚠️ 健壮性 | `memory/` 3 处 DDL/DML 错误静默 | 🟡 P2 | 加日志或返回 error |
| NEW-4 | 📝 覆盖 | `tts/` 8 文件零测试 | 🟢 P3 | 添加基础单元测试 |
| NEW-5 | 📝 覆盖 | `linkparse/` 零测试 | 🟢 P3 | 添加检测/格式化测试 |
| NEW-6 | 📝 覆盖 | `routing/` 零测试 | 🟢 P3 | 添加 session key 测试 |
| NEW-7 | ⏭️ 缺失 | `canvas-host/` (2 TS 文件) 未移植 | ⚪ 评估 | 独立功能，可推迟 |
| NEW-8 | ⏭️ 缺失 | `node-host/` (2 TS 文件) 未移植 | ⚪ 评估 | 远程执行，Phase 12+ |
| NEW-9 | ⏭️ 缺失 | `channels/line/` 仅骨架 | ⚪ 评估 | LINE SDK 集成延后 |

---

## 七、健康度评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 编译健康 | 10/10 | build + vet 零问题 |
| 测试健康 | 8/10 | 1 fail + 17 包缺测试 |
| 代码规范 | 9/10 | 2 处 panic + 少量忽略错误 |
| 移植完整度 | 9/10 | 核心模块 100%，3 个小模块未移植 |
| 文档完整度 | 10/10 | `docs/gouji/` + `docs/renwu/` 完善 |
| **综合** | **9.2/10** | 仅需小修即达生产就绪 |
