# Phase 12 — 延迟项清除 任务清单

> 范围：16 项待办（不含 Ollama / i18n，已推 Phase 13）
>
> 窗口设计原则：每窗口 ≤ 2,000 行 TS 源码分析 + Go 编码，避免上下文过载

---

## 窗口划分

| 窗口 | 包含项 | TS 行数 | 主题 |
| ---- | ------ | ------- | ---- |
| W1 | NEW-8 | 1,382L | node-host 远程节点执行（P0） |
| W2 | P11-C-P1-4 | 554L | block-streaming 管线（P1） |
| W3 | NEW-9 | 735L | canvas-host 画布托管（P2） |
| W4 | AUDIT-1~5 | ~200L | normalizeToolParameters 5 项差异修复 |
| W5 | AUDIT-6, AUDIT-7, NEW-2, NEW-3 | ~100L | 模型选择 + thinking + panic + 错误静默 |
| W6 | NEW-4~7 | ~400L | 测试补充（tts/linkparse/routing）+ LINE 评估 |

> W1 最重（1,382L），需拆分 2 子任务：config(73L) + runner 前半(~650L) → runner 后半(~660L)

---

## 详细任务

### W1: node-host 远程节点执行运行时 (P0)

- [x] W1-1: 移植 `config.ts` (73L) → `internal/nodehost/config.go`
- [x] W1-2: 移植 `runner.ts` 前半 — 类型/常量/sanitizeEnv/SkillBinsCache/exec
- [x] W1-3: 移植 `runner.ts` 后半 — invoke/runner (HandleInvoke + system.run)
- [x] W1-4: gateway stubs 保留 + 注释更新
- [x] W1-5: 编写测试 + 验证 (nodehost 1.222s + gateway 9.396s, race)
- [x] W1-6: 深度审计 TS↔Go 对比 + 更新文档 + 架构文档

### W2: block-streaming 管线 (P1)

- [x] W2-1: 已实现 — 深度审计发现 5 项差异
- [x] W2-2: block-streaming / coalescer 已存在
- [x] W2-3: coalescer 改为 ReplyPayload API + 上下文切换 + 溢出拆分
- [x] W2-4: pipeline 增加 pendingKeys + buffer + payloadKey 含 mediaUrls
- [x] W2-5: 31 tests pass (race), build + vet clean
- [x] W2-6: 全量对齐 TS 完成

### W3: canvas-host 画布托管 (P2)

- [x] W3-1: a2ui.go (250L) — 静态文件服务 + live-reload 注入
- [x] W3-2: handler.go (423L) + server.go (168L) — HTTP/WS handler + 独立服务器
- [x] W3-3: host_url.go (113L) — URL 解析
- [x] W3-4: canvas_test.go (338L) — 12 tests pass (race)
- [x] W3-5: build + vet + 全量测试通过

### W4: normalizeToolParameters 5 项差异修复 (P2)

- [x] W4-1: AUDIT-1 — `extractEnumValues` 添加 const 处理
- [x] W4-2: AUDIT-2 — `extractEnumValues` 递归嵌套提取
- [x] W4-3: AUDIT-3 — required 合并改为 count 判断
- [x] W4-4: AUDIT-4 — 保留 `additionalProperties`
- [x] W4-5: AUDIT-5 — early-return 和 fallback 对齐
- [x] W4-6: 编写/补充测试 + 验证（14 tests pass, race）
- [x] W4-7: 深度审计 + 更新文档

### W5: 杂项差异与规范修复 (P2)

- [x] W5-1: AUDIT-6 — `BuildAllowedModelSet` 添加 configuredProviders 分支
- [x] W5-2: AUDIT-7 — `promoteThinkingTagsToBlocks` 添加 guard + trimStart
- [x] W5-3: NEW-2 — Discord `send_shared.go` 消除 2 处 panic → error 返回
- [x] W5-4: NEW-3 — `memory/` schema.go+manager.go 错误处理（4 处静默→日志）
- [x] W5-5: 验证 + 更新文档

### W6: 测试覆盖补充 (P3)

- [x] W6-1: NEW-4 — tts/ 包基础单元测试（15 tests pass, race）
- [x] W6-2: NEW-5 — linkparse/ 包检测/格式化测试（7 tests pass, race）
- [x] W6-3: NEW-6 — routing/ session_key 测试（11 tests pass, race）
- [x] W6-4: NEW-7 — channels/line/ 评估 → 推迟 Phase 13（骨架实现，需 LINE SDK 集成）
- [x] W6-5: 全量 `go test -race` 验证通过

---

## 验证标准

每个窗口完成后必须通过：

```bash
cd backend && go build ./... && go vet ./... && go test -race ./...
```

并执行 TS↔Go 逐文件深度审计，按技能工作流更新 `docs/gouji/` 架构文档。
