> 📄 分块 03/08 — A-W3a + A-W3b | 索引：phase13-task-00-index.md
>
> **A-W3a TS 源**：`src/agents/tools/*-actions.ts` → **Go**：`backend/internal/agents/tools/`
> **A-W3b TS 源**：`src/agents/bash-tools.*` + `src/agents/subagent-*` → **Go**：`backend/internal/agents/bash/`

## 窗口 A-W3a：频道操作工具 ✅ 完成

> 参考：`gap-analysis-part4d.md` A-W3a 节

- [x] **A-W3a-T1**: Discord 操作工具 — `agents/tools/discord_actions.go`
- [x] **A-W3a-T2**: Slack 操作工具 — `agents/tools/slack_actions.go`
- [x] **A-W3a-T3**: Telegram 操作工具 — `agents/tools/telegram_actions.go`
- [x] **A-W3a-T4**: WhatsApp 操作工具 — `agents/tools/whatsapp_actions.go`
- [x] **A-W3a 验证**：`go build ./internal/agents/tools/...` ✅

---

## 窗口 A-W3b：Bash 执行链 + PTY ✅ 完成

> 参考：`gap-analysis-part4d.md` A-W3b 节

- [x] **A-W3b-T1**: Bash 命令执行 — `agents/bash/exec.go` (520L)
  - `executeBashCommand()` 主入口，Docker/本地分支，命令预处理，输出截断，超时，审批
- [x] **A-W3b-T2**: PTY 进程管理 — `agents/bash/process.go` (270L) + `pty_keys.go` (290L)
  - PTY 分配，键映射（55 命名键），Ctrl/Alt/Shift 修饰符，xterm CSI
- [x] **A-W3b-T3**: Docker 参数 + 进程注册表 — `agents/bash/shared.go` (310L) + `process_registry.go` (350L)
  - 沙箱环境构建，Docker exec 参数，线程安全进程注册表，TTL 清理
- [x] **A-W3b-T4**: 代码补丁 — `agents/bash/apply_patch.go` (380L) + `apply_patch_update.go` (230L)
  - `ApplyPatch` / `ApplyUpdateHunk`，4-pass 匹配，Unicode 标点规范化
- [x] **A-W3b-T5**: 子代理系统 — `agents/bash/subagent_registry.go` (350L) + `announce_queue.go` (230L)
  - 注册/释放/持久化，v1→v2 迁移，collect/individual/summarize 队列模式
  - `runner/subagent_announce.go` (320L) 已存在并验证
- [x] **A-W3b-T6**: 缓存追踪 — `agents/bash/cache_trace.go` (320L)
  - JSONL 追踪写入，SHA-256 摘要，稳定序列化，环境变量配置
- [x] **A-W3b 验证**：`go build ./internal/agents/bash/...` ✅ + `go vet` ✅

---
