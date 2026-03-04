# Phase 8 Bootstrap — Window 4 启动上下文

> 最后更新：2026-02-15（Window 3 完成后生成）

---

## Window 3 完成摘要

- 17 个命令处理器 Go 文件 + 1 测试文件（~2,200L）
- 15 DI 接口（GatewayCaller, ConfigManager, TTSService 等）
- 100 tests PASS（88 新 + 12 已有）
- `go build` + `go vet` + `go test` 全部通过

---

## Window 4 新窗口启动模板

```
@/refactor

当前阶段：Phase 8.2-W4 — 集成层 + 文档（820L TS + 修补项）

Window 1-3 均已完成（39 Go 文件 + 7 测试文件，100 tests 全部通过）。

请执行 Window 4：
1. 读取 `docs/renwu/phase8-task.md` 确认 W4 任务清单
2. 读取 `docs/renwu/deferred-items.md` 中 P7D-1, P7D-2, P7D-9, P7D-10 条目
3. 按 /refactor 六步循环执行：

W4-A: status.ts 移植（679L TS）→ `internal/autoreply/status.go`
- 对应 P7D-1
- 15+ 外部依赖（config/agents/channels/tts/memory/hooks/browser）
- 需 DI 接口 + stub 处理大量外部依赖

W4-B: skill-commands.ts 移植（141L TS）→ `internal/autoreply/skill_commands.go`
- 对应 P7D-2
- 依赖 agents/skills 模块完整接口

W4-C: 修补项
- P7D-9: sanitizeUserFacingText（`errors.ts` L403-446）→ API 错误消息友好化
- P7D-10: abort.go 完善 ABORT_MEMORY + tryFastAbort（`abort.ts` L20-205）

W4-D: 文档更新
- 更新 `docs/gouji/autoreply.md` 架构文档
- 更新 `docs/renwu/refactor-plan-full.md` Phase 8 完成状态
- 更新 `docs/renwu/deferred-items.md` 标记 P7D-1/2/9/10 完成
- 创建 Phase 8 完成审计报告
- 归档 Phase 8 相关文档到 archive/

Window 1-3 已创建的完整基础：
- 指令系统: directives.go → directive_parse.go → get_reply_directives.go
- Agent 执行: agent_runner.go → agent_runner_execution.go (DI)
- 回复链路: get_reply.go → get_reply_run.go → followup_runner.go
- 命令系统: commands_registry.go + 17 W3 处理器文件（15 DI 接口）
- 投递链路: reply_dispatcher.go → route_reply.go (DI)
- 注意: P8W2-D1~D5 + W3 DI 接口实现需在后续 Phase 处理
```

---

## 参考文档

- [phase8-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase8-task.md) — 任务清单
- [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) — P7D + P8W2 延迟项
- [autoreply.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/gouji/autoreply.md) — 架构文档
- [refactor-plan-full.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/refactor-plan-full.md) — 全局路线图
