# Phase 12 — 新窗口启动上下文

> 适用于：Phase 12 延迟项清除（6 窗口，16 项待办）

---

## 新窗口启动模板

在新窗口启动时，按顺序读取以下文件即可恢复上下文：

```text
1. skills/acosmi-refactor/references/coding-standards.md （跳过 Rust/FFI 章节）
2. docs/renwu/deferred-items.md                           — 延迟待办全量
3. docs/renwu/phase12-task.md                              — 当前窗口任务清单
4. 本文件（phase12-bootstrap.md）                          — 当前窗口启动上下文
```

然后确认：**当前要执行的窗口编号是什么？**

---

## 项目状态

- **Go 后端**：626 源文件，`go build`/`go vet` ✅，`go test` ✅（全绿）
- **移植完整度**：核心模块 100%，Phase 12 处理剩余 16 项
- **已归档**：215 项延迟待办已完成  
- **Phase 12 推迟到 Phase 13**：Ollama (P3) + i18n (P3)

---

## 窗口概览

| 窗口 | 主题 | 优先级 | TS 行数 | 关键 TS 源文件 |
| ---- | ---- | ------ | ------- | -------------- |
| W1 | node-host 远程执行 | 🔴 P0 | 1,382L | `src/node-host/config.ts`, `runner.ts` |
| W2 | block-streaming 管线 | 🟡 P1 | 554L | `src/agents/block-reply-pipeline.ts`, `block-streaming.ts`, `block-reply-coalescer.ts` |
| W3 | canvas-host 画布托管 | 🟢 P2 | 735L | `src/canvas-host/a2ui.ts`, `server.ts` |
| W4 | normalizeToolParameters 修复 | 🟢 P2 | ~200L | `src/agents/pi-tools.schema.ts` |
| W5 | 杂项差异与规范修复 | 🟢 P2 | ~100L | `model-selection.ts`, `pi-embedded-utils.ts`, `send_shared.go`, `memory/*.go` |
| W6 | 测试覆盖补充 | ⚪ P3 | ~400L | 无新 TS，仅 Go 端编写测试 |

---

## 窗口上下文

### W1: node-host 远程节点执行

**目标**：将 TS node-host 模块完整移植到 Go，替换 gateway `node.*` stub handlers

**TS 源文件**：

| 文件 | 行数 | 角色 |
| ---- | ---- | ---- |
| `src/node-host/config.ts` | 73L | NodeHostConfig 类型/加载/保存 |
| `src/node-host/runner.ts` | 1,309L | WS 连接/命令执行/浏览器代理/审批/环境消毒 |

**Go 目标**：`backend/internal/nodehost/` (新包)

**依赖关系**：

- 上游：gateway `node.*` 方法 → 需替换 stub
- 下游：`infra/exec-host`、`infra/exec-approvals`、`security/exec`、`agents/agent-scope`、`config/paths`

**注意事项**：

- `runner.ts` 超过 1,300L，必须拆分 2 个子任务执行
- 涉及 `child_process.spawn` → Go `os/exec`
- 涉及 WebSocket 客户端连接 → 需复用 gateway 的 WS 工具

---

### W2: block-streaming 管线

**目标**：移植 Agent 流式响应的 block 管线

**TS 源文件**：

| 文件 | 行数 | 角色 |
| ---- | ---- | ---- |
| `src/agents/block-reply-pipeline.ts` | 242L | block 流水线编排 |
| `src/agents/block-streaming.ts` | 165L | 流式 block 解析 |
| `src/agents/block-reply-coalescer.ts` | 147L | block 合并/去重 |

**Go 目标**：`backend/internal/agents/runner/` (扩展现有)

**依赖关系**：

- 上游：agent runner 的流式输出管线
- 下游：`autoreply/reply/block-streaming` (可能已有骨架)

---

### W3: canvas-host 画布托管

**目标**：在 Go gateway 中嵌入 Canvas 静态文件 + WS 服务

**TS 源文件**：

| 文件 | 行数 | 角色 |
| ---- | ---- | ---- |
| `src/canvas-host/a2ui.ts` | 219L | A2UI 静态资源 + WebView 桥接 |
| `src/canvas-host/server.ts` | 516L | HTTP+WS 服务器 + chokidar live-reload |

**Go 目标**：`backend/internal/canvas/` (新包)

**依赖关系**：

- 上游：gateway HTTP router → 注册 `/__openacosmi__/a2ui` 和 `/__openacosmi__/canvas` 路由
- 下游：`media/mime`、`config/paths`
- live-reload 使用 `fsnotify` 替代 `chokidar`

---

### W4: normalizeToolParameters 修复

**目标**：修复 AUDIT-1~5，5 处 Go↔TS 差异

**Go 目标文件**：`backend/internal/agents/runner/normalize_tool_params.go`

**TS 参考**：`src/agents/pi-tools.schema.ts`

**修改点**：

- AUDIT-1: `extractEnumValues` 添加 `"const" in record → [record.const]`
- AUDIT-2: `extractEnumValues` 递归处理嵌套 anyOf/oneOf
- AUDIT-3: `required` 合并改为仅在所有 variants 都需要时才保留
- AUDIT-4: 保留 `additionalProperties` 字段
- AUDIT-5: early-return 条件对齐 + fallback 回退逻辑

---

### W5: 杂项差异与规范修复

**目标**：修复 AUDIT-6、AUDIT-7、NEW-2、NEW-3

**修改点**：

| 项 | 文件 | 修改 |
| -- | ---- | ---- |
| AUDIT-6 | `agents/models/selection.go` | 添加 configuredProviders 分支 |
| AUDIT-7 | `agents/runner/promote_thinking.go` | 添加 hasThinkingBlock guard + trimStart |
| NEW-2 | `channels/discord/send_shared.go` | panic → error 返回 |
| NEW-3 | `memory/schema.go` + `manager.go` | 错误处理 + 日志 |

---

### W6: 测试覆盖补充

**目标**：为 4 个缺测试的包补充基础测试

| 项 | 包 | 测试重点 |
| -- | -- | -------- |
| NEW-4 | `tts/` | config/cache/directives 基础逻辑 |
| NEW-5 | `linkparse/` | URL 检测/格式化/边界情况 |
| NEW-6 | `routing/` | session key 解析/生成 |
| NEW-7 | `channels/line/` | 评估是否需要完整实现，记录决策 |

---

## 工作流提示

每个窗口严格按照 `/refactor` 工作流执行：

1. 步骤 0：加载上下文（本文件 + coding-standards）
2. 步骤 0.5：文件级预审
3. 步骤 1-3：提取 → 依赖图 → 隐藏依赖审计
4. 步骤 4-5：分析 → 重构
5. 步骤 6：验证（build/vet/test + TS↔Go 审计 + 文档更新）
