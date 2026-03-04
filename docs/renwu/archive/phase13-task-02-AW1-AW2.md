> 📄 分块 02/08 — A-W1 + A-W2 | 索引：phase13-task-00-index.md
>
> **TS 源**：`src/agents/tools/` + `src/agents/schema/`
> **Go 目标**：`backend/internal/agents/tools/` + `backend/internal/agents/schema/`

## 窗口 A-W1：工具基础层（含 agents/schema/）✅ 完成

> 参考：`gap-analysis-part4c.md` A-W1 节
> ⭐ 审计复核追加：`agents/schema/` (2文件 419L) 移入本窗口

- [x] **A-W1-T1**: 工具基础框架 → `common.go` (416L)
- [x] **A-W1-T2**: 工具 Schema + 注册 → `schema.go` (226L) + `registry.go` (203L)
- [x] **A-W1-T3**: 工具策略引擎 → `policy.go` (237L)
- [x] **A-W1-T4**: 工具辅助 → `display.go` (189L) + `callid.go` (196L) + `images.go` (137L)
- [x] **A-W1-T5**: 文件读取 + 频道桥接 + Gateway → `read.go` (259L) + `channel_bridge.go` (69L) + `gateway.go` (204L)
- [x] **A-W1-T6**: Agent 步骤 → `agent_step.go` (84L)
- [x] **A-W1-T7**: agents/schema/ → `typebox.go` (120L) + `clean_for_gemini.go` (463L)
- [x] **A-W1 验证**: `go build` + `go vet` + `go test -race` ✅ (19 + 16 = 35 tests)

---

## 窗口 A-W2：文件/会话/媒体工具 ✅ 完成

> 参考：`gap-analysis-part4c.md` A-W2 节

- [x] **A-W2-T1**: 图片工具 → `image_tool.go` (259L)
- [x] **A-W2-T2**: 记忆 + 网页抓取 → `memory_tool.go` (115L) + `web_fetch.go` (277L)
- [x] **A-W2-T3**: 消息 + 会话工具集 → `sessions.go` (263L) + `message_tool.go` (107L)
- [x] **A-W2-T4**: 节点/画布/Agent/定时/TTS/浏览器 → `nodes_tool.go` (147L) + `canvas_tool.go` (75L) + `cron_tool.go` (149L) + `tts_tool.go` (76L) + `browser_tool.go` (155L) + `agents_list_tool.go` (46L)
- [x] **A-W2-T5**: 会话辅助函数 → `sessions_helpers.go` (~360L) ← 原 AW2-D1
- [x] **A-W2-T6**: A2A 发送辅助 → `sessions_send_helpers.go` (~280L) ← 原 AW2-D2
- [x] **A-W2-T7**: 节点工具辅助 → `nodes_utils.go` (~240L) ← 原 AW2-D3
- [x] **A-W2 验证**: `go build` + `go vet` + `go test -race` ✅

> **完成度审计（最终 — 2026-02-18）**:
>
> - 25 Go 文件 (~4,572L) + 2 schema 文件 (583L) = **27 文件 ~5,155L**
> - TS 38 文件中：27 已完整移植，10 属 A-W3a（频道操作），1 为 TS 入口重导出
> - ~~原延迟 AW2-D1~D3~~ → **已全量补齐，0 延迟待办**
> - `go build ./...` + `go vet ./...` + `go test -race` 全局通过

✅ **本分块已全部完成 — 无延迟项、无残留**

---
