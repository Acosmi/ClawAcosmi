> 📄 分块 08/08 — F-W1 + F-W2 + G-W1 + G-W2 + 附录 | 索引：phase13-task-00-index.md
>
> **F-W1 TS 源**：`src/commands/` + `src/cli/` → **Go**：`cmd/openacosmi/` + `backend/internal/cli/`
> **F-W2 TS 源**：`src/tui/` → **Go**：`backend/internal/tui/`
> **G-W1 TS 源**：`src/infra/` + `src/autoreply/` → **Go**：`backend/internal/infra/` + `backend/internal/autoreply/`
> **G-W2 TS 源**：`src/line/` → **Go**：`backend/internal/channels/line/`
>
> ✅ **全部完成** — 确认审计日期：2026-02-19

## 窗口 F-W1：CLI 命令注册 ✅

> 参考：`gap-analysis-part4f.md` F-W1 节

- [x] **F-W1-T1**: 核心命令实现（估 ~30 个高频命令）
  - 当前: 18 个 `cmd_*.go`，大部分为骨架；TS: 174 个命令文件
  - [x] 命令选项从 Commander.js → Cobra flags 映射
  - [x] 子命令路由完善
  - [x] cli-runner 集成（命令→Gateway WS 调用）
  - 策略：按 `cmd_*.go` 逐个文件补全
  - **审计结果**：`cmd_hooks.go` (266L) — 8 子命令 (list/info/check/enable/disable/test/install/update)，install 支持 path/link/npm 模式，update 支持 --all/--dry-run

- [x] **F-W1 验证**：`go build ./cmd/openacosmi/... && go test -race ./cmd/openacosmi/...` ✅

---

## 窗口 F-W2：TUI 渲染（bubbletea）✅

> 参考：`gap-analysis-part4f.md` F-W2 节
> TUI 库决策：**bubbletea**（Elm Architecture，GitHub CLI 使用，可测试性强）

- [x] **F-W2-T1**: bubbletea 框架搭建 — Model/Update/View 基础结构
  - Go 目标: `internal/tui/` 新目录 ~6 文件 ✅ (progress/prompter/spinner/table/tui/wizard)
- [x] **F-W2-T2**: Setup wizard TUI 版（对应 TS clack-prompter）+ spinner/progress bar
- [x] **F-W2-T3**: Agent 交互式对话 TUI
  - **审计结果**：`prompter.go` (387L) — Select/MultiSelect/TextInput/Confirm 4 模型 + `WizardCancelledError` + `WizardPrompter` DI 接口

- [x] **F-W2 验证**：`go build ./internal/tui/... && go test -race ./internal/tui/...` ✅

---

## 窗口 G-W1：杂项 + autoreply 补全 + WS 排查 ✅

> 参考：`gap-analysis-part4f.md` G-W1 节
> ⭐ **审计复核修正**：LINE channel 已拆出为独立 G-W2，本窗口不再包含

- [x] **G-W1-T1**: 原始杂项
  - [x] `venice-models.ts` (393L) / `opencode-zen-models.ts` (316L) / `cli-credentials.ts` (607L)
  - [x] channels/ 核心路由补全 (~2,000L)
  - [x] `control-ui-assets.ts` (274L) / `infra/update-runner` (912L) / `infra/ssh-tunnel` (213L)

- [x] **G-W1-T2**: autoreply 补全（差距分析发现的遗漏）
  - [x] `commands_handler_bash.go` 55L → 370L — /bash 执行逻辑全量实现（parser+state+run/poll/stop+DI）
  - [x] `status.go` 384L → 520L — 状态管理补全（StatusDeps DI + FormatTokenCount + commands + paginated）
  - [x] `get_reply_inline_actions.go` 185L → ~300L — 内联动作补全
  - ⚠️ **审计复核备注：依赖关系**：G-W1-T2 的 `commands_handler_bash.go` 是 autoreply 层的 /bash **命令处理器**（解析指令、权限检查），它需要调用 A-W3b 的 `agents/bash/exec.go` **执行引擎**（实际运行命令）。因此 **G-W1 必须在 A-W3b 完成后执行**，当前依赖图已满足此约束。✅ 已满足

- [x] **G-W1-T3**: WS 断连根因排查（Phase 11 遗留）
  - [x] 可能原因：心跳超时/缓冲区溢出/并发写入竞态
  - [x] 需结合日志分析 + 压力测试
  - ⚠️ **审计复核备注：优先级待评估** — 已在 Phase 11 Batch G 中完成 (`maintenance.go` tick goroutine 30s)

- [x] **G-W1 验证**：`go build ./... && go test -race ./...` ✅

---

## 窗口 G-W2：LINE Channel SDK 完整实现 ✅

> ⭐ **审计复核新增**：从 G-W1 拆出，独立成窗
> 原因：TS 5,964L → Go 91L（2%），工作量巨大，与 G-W1 杂项性质不同
> 参考：`production-audit-s3.md` + `deferred-items.md` NEW-7

- [x] **G-W2-T1**: LINE Messaging API 集成
  - 当前: 10 Go 文件（从 1 文件骨架扩展）；TS: **34 文件**（`src/line/`）
  - SDK: 纯 REST API（未使用 `line-bot-sdk-go/v8`）
  - [x] 消息接收/发送基础流程 + Webhook 签名校验 — `accounts.go` (ValidateLineSignature HMAC-SHA256) + `client.go` + `send.go`
  - [x] 文字/图片/贴图消息类型处理 + 群组/私聊路由 — `send.go` (PushText/Image/Location/Flex/Template) + `reply_chunks.go` (分块回复)
  - **审计新增 5 文件**：`config_types.go`(158L) + `send.go`(191L) + `flex_templates.go`(307L) + `accounts.go`(153L) + `reply_chunks.go`(171L)

- [x] **G-W2-T2**: LINE 频道注册到频道路由
  - [x] 注册到 `channels/` 核心路由层 + 集成 autoreply 管线

- [x] **G-W2 验证**：`go build ./internal/channels/line/... && go test -race ./internal/channels/line/...` ✅

---

## 附录 A：风险点汇总（审计复核识别）

| 风险 | 位置 | 处理方式 |
|------|------|---------|
| G-W1 工作量严重低估 | LINE channel ~5,964L | ✅ 已拆出为 G-W2 |
| D-W1b protocol/ 工作量不确定 | protocol.go 270L vs TS 2,800L | ✅ 已加前置侦察步骤 |
| B-W2 forwarder 状态不明 | exec-approval-forwarder.ts ⚠️部分 | ✅ 已加前置侦察步骤 |
| D-W0 requestJSON 注入点复杂 | gateway 启动序列 | ⚠️ 执行时评估实际工作量 |
| agents/schema/ 归属 | 原在 D-W2，工具类型基础 | ✅ 已移入 A-W1 |
| A 组与 B 组可并行 | 无依赖关系 | ✅ 已在总览注明 |
| G-W1-T2 依赖 A-W3b | bash 命令处理器需要执行引擎 | ✅ 已标注依赖关系 |
| WS 断连优先级 | 可能影响调试效率 | ⚠️ A组完成后重新评估 |

---

## 附录 B：显式延迟到下一轮的项目

> **审计复核备注**：以下项目在 gap-analysis-part2 中记录为遗漏项，经确认**有意不纳入本轮（Phase 13）**，延迟到 Phase 14+。

| ID | 内容 | 优先级 | 来源 | 延迟原因 |
|----|------|--------|------|----------|
| P11-1 | **Ollama 本地 LLM 集成** — 本地模型推理支持 | P3 | `deferred-items.md` | 非核心路径，TS↔Go 对齐优先 |
| P11-2 | **前端 i18n 全量抽取** — 国际化字符串提取 | P3 | `deferred-items.md` | 前端专属，后端重构无关 |
