# 视觉理解 C+D 修复 任务跟踪

> 创建日期：2026-02-27
> 状态：待启动
> 来源：[audit-20260227-vision-cd.md](file:///Users/fushihua/Desktop/OpenAcosmi-rust+go/docs/renwu/audit-20260227-vision-cd.md)
> 参考：[bootstrap-vision-cd.md](file:///Users/fushihua/Desktop/OpenAcosmi-rust+go/docs/renwu/bootstrap-vision-cd.md)、[视觉理解新方案.md](file:///Users/fushihua/Desktop/OpenAcosmi-rust+go/docs/claude/视觉理解新方案.md)

## 背景与目标

视觉理解 C（VLA 模型持续输出）+ D（审批集中主端）两个方案的代码骨架已完成约 45%。核心类型（`VisionObservation`、`ObservationBuffer`、`VLAClient`、`ScreenObserver`、`ActionRiskLevel`）和算法逻辑已编写，但**关键集成接线全部缺失**，端到端功能不可用。

**目标**：修复 P0 阻断项 + P1 集成接线，使功能端到端可用；完成 P2 改进项提升健壮性。

**真实完成度**：~45% → 修复后预期 ~90%

## 任务清单

### Batch A: P0 阻断修复（前端功能完全不可用）

| 状态 | 编号 | 任务 | 关联发现 | 涉及文件 |
|------|------|------|----------|----------|
| ✅ | A1 | `app-render.ts` 添加 `case "subagents"` 渲染分支，调用 `renderSubAgents()` | P0-1 | `ui/src/ui/app-render.ts` |
| ✅ | A2 | `locales/en.ts` 添加 18 个英文 i18n 键（对齐 `zh.ts` 中已有的 `subagents.*` 键） | P0-2 | `ui/src/ui/locales/en.ts` |

**A1 代码验证**：已确认 `app-render.ts` 中无 `case "subagents"` 分支。`renderSubAgents()` 已在 `views/subagents.ts` 中 export 定义（L64），但未被 `app-render.ts` 导入或调用。点击子智能体 Tab 后无内容渲染。

**A1 修复方案**：

1. 在 `app-render.ts` 顶部添加 `import { renderSubAgents } from "./views/subagents.ts";`
2. 在 view switch 语句中添加 `case "subagents": return renderSubAgents(this);`

**A2 代码验证**：已确认 `zh.ts` 包含 18 个 `subagents.*` 键（L39/57/84-99），`en.ts` 中搜索 `subagent` 返回 0 结果。英文环境下所有标签显示 key 原文。

**A2 修复方案**：在 `en.ts` 中添加 18 个对应英文翻译键，键名与 `zh.ts` 完全一致。

> **状态标记**：⬜ 待做 → 🔄 进行中 → ✅ 完成 → ⏭️ 跳过/延迟

---

### Batch B: P1 集成接线（端到端功能不可用）

| 状态 | 编号 | 任务 | 关联发现 | 涉及文件 |
|------|------|------|----------|----------|
| ⬜ | B1 | `bridge.go` 添加 `observer *ScreenObserver` 字段，Start/Stop 接入 Observer 生命周期 | P1-1 | `backend/internal/argus/bridge.go` |
| ⬜ | B2 | `ArgusBridgeForAgent` 接口暴露 `ObservationBuffer()` 方法 + `argusBridgeAdapter` 实现 | P1-2 | `backend/internal/agents/runner/attempt_runner.go`、`backend/internal/gateway/server.go` |
| ⬜ | B3 | 后端 WS handler (`ws_server.go`) 添加 `subagent_ctl` 消息类型分发 | P1-3 | `backend/internal/gateway/ws_server.go` |
| ⬜ | B4 | `AnthropicVisionClient.Infer` 接入真实 Anthropic Messages API（或标记 Phase 4 延迟） | P1-4 | `backend/internal/argus/vla_client.go` |

**B1 代码验证**：`bridge.go` `Bridge` 结构体（L79-94）字段为 `cfg/mu/state/client/cmd/tools/...`，无 `observer` 字段。`Start()` (L142-171) 和 `Stop()` (L393-439) 均不涉及 ScreenObserver。Observer 虽已完整实现（`screen_observer.go` 191 行），但在 bridge 层完全孤立。

**B1 修复方案**：

1. `Bridge` 结构体添加 `observer *ScreenObserver` 字段
2. `Start()` 成功后创建 `NewScreenObserver(cfg)` 并调用 `observer.Start()`
3. `Stop()` 中先 `observer.Stop()` 再关闭子进程
4. 添加 `Observer() *ScreenObserver` 公共方法供外部获取

**B2 代码验证**：`ArgusBridgeForAgent` 接口（`attempt_runner.go` L39-42）仅有 `AgentTools()` 和 `AgentCallTool()`。`argusBridgeAdapter`（`server.go` L115-157）仅适配这两个方法。主 Agent 无法通过任何路径获取 `ObservationBuffer`。

**B2 修复方案**：

1. `ArgusBridgeForAgent` 接口添加 `ObservationBuffer() interface{}` 方法（避免引入 argus 包依赖）
2. 或使用 runner 本地接口类型 `type ObservationReader interface { LatestKeyframe() ...; Last(n int) ... }`
3. `argusBridgeAdapter` 实现该方法，从 `bridge.Observer().Buffer()` 获取

**B3 代码验证**：`ws_server.go` `wsConnectionLoop()` (L99-499) 处理客户端消息。搜索 `subagent_ctl` 在整个 gateway 包返回 0 结果。前端 `sendSubAgentCtl()` 发送 `{"type":"subagent_ctl","payload":{...}}`，后端无对应 handler，消息被静默丢弃。

**B3 修复方案**：

1. 在 `wsConnectionLoop` 的消息类型分发中添加 `case "subagent_ctl":`
2. 解析 payload 中 `agent_id`/`action`/`value`
3. 根据 `action` 分别调用 `observer.SetGoal()`、`observer.SetInterval()` 等
4. 需要 `WsServerConfig` 持有 Argus Bridge 引用

**B4 代码验证**：`vla_client.go` `AnthropicVisionClient.Infer()` (L45-62) 包含 `// TODO: 实际调用 Anthropic Vision API`，始终返回 `{Action:"DONE", Reasoning:"stub"}`。现有 `llmclient` 包已有 Anthropic API 调用能力（`anthropic.go`），可复用 HTTP client 和认证逻辑。

**B4 修复方案（已确认 — 策略 A + 多 Provider）**：

- 接入真实 Anthropic Messages API（base64 图片 → vision prompt → 解析 VLMActionResult）
- 预留 Gemini、Qwen、本地 Ollama 多 provider 支持（工厂函数扩展 `NewVLAClient` switch）

---

### Batch C: P2 改进项（健壮性提升）

| 状态 | 编号 | 任务 | 关联发现 | 涉及文件 |
|------|------|------|----------|----------|
| ⬜ | C1 | `screen_observer.go` SetGoal/SetInterval 数据竞争修复（atomic 或 RWMutex） | P2-1 | `backend/internal/argus/screen_observer.go` |
| ⬜ | C2 | 添加 subagents 相关 CSS 样式（`.subagents-view`/`.subagent-card`/`.subagent-row` 等） | P2-2 | `ui/src/ui/index.css` 或独立 CSS |
| ⬜ | C3 | 添加 `screen_observer_test.go` 单元测试（mock CaptureFunc 测试循环/去重/VLA 调用） | P2-3 | `backend/internal/argus/screen_observer_test.go` [NEW] |
| ⬜ | C4 | 实现 ChangeMagnitude 像素差计算（替代当前硬编码 1.0） | P2-4 | `backend/internal/argus/screen_observer.go` |
| ⬜ | C5 | 执行 `go vet ./...` 验证零警告 | P2-5 | 全项目 |

**C1 代码验证**：`screen_observer.go` L103-104 `SetGoal()` 直接写 `o.cfg.Goal = goal`，L108-111 `SetInterval()` 直接写 `o.cfg.BaseInterval = d`。而 `loop()` goroutine (L114-135) 在 `captureOnce()` 中读取这些字段。主 goroutine 写 + loop goroutine 读 = 数据竞争。`go vet -race` 会报警。

**C1 修复方案**：

1. `Goal` 字段改用 `atomic.Value` 存储
2. `BaseInterval` 字段改用 `atomic.Int64` 存储（Duration 转 int64）
3. 或统一用 `sync.RWMutex` 保护 cfg 的可变字段

**C2 代码验证**：搜索项目 CSS 文件中 `.subagent` 相关选择器返回 0 结果。`subagents.ts` 使用了 `subagents-view`、`subagent-card`、`subagent-card__header`、`subagent-card__body`、`subagent-row` 等 class，均无样式定义。

**C3 代码验证**：`argus/` 目录现有 `bridge_test.go`(262L)、`observation_buffer_test.go`、`integration_test.go`、`codesign_test.go`，但无 `screen_observer_test.go`。

**C4 代码验证**：`screen_observer.go` L176-177 `obs.ChangeMagnitude = 1.0` 硬编码，注释"后续可用像素差计算"。所有变化帧都标记为关键帧，违背方案中"变化>2% 阈值"设计。

---

## 集成数据通路完成度跟踪

| 数据通路 | 修复前状态 | 修复项 | 修复后预期 |
|----------|-----------|--------|-----------|
| 风险分级 → 审批门 (D) | ✅ 已连通 | — | ✅ |
| 截图 → Buffer (C) | ⚠️ 孤立 | B1 | ✅ |
| Buffer → 主 Agent (C) | ❌ 未连通 | B2 | ✅ |
| 前端 Tab → 视图渲染 | ❌ 未连通 | A1 | ✅ |
| 前端 WS → 后端控制 | ❌ 未连通 | B3 | ✅ |
| 配置 → 运行时 | ⚠️ 部分 | B1+B3 | ✅ |
| VLA 推理 | ❌ stub | B4 | ⏭️ 或 ✅ |

## 完成后需更新的文档

- [ ] `docs/renwu/audit-20260227-vision-cd.md` — 标记已修复项
- [ ] `docs/renwu/bootstrap-vision-cd.md` — 标记已完成阶段
- [ ] `docs/renwu/deferred-items.md` — 登记 B4 延迟项（如选策略 B）

## 风险与注意事项

1. **B1 CaptureFunc 注入**：`ScreenObserver` 需要 `CaptureFunc` 截图函数，但 Bridge 当前通过 MCP 调用 `capture_screen` 工具。需确保 CaptureFunc 正确封装 MCP 调用或使用平台原生截图。
2. **B2 接口设计**：Go 惯例是消费侧定义接口。在 `runner` 包定义 `ObservationReader` 接口更合理，避免 runner→argus 直接依赖。
3. **B3 并发安全**：WS handler 中操作 Observer 需注意线程安全，SetGoal/SetInterval 必须先完成 C1 竞争修复。
4. **B4 API 成本**：Anthropic Vision API 每帧截图（1080p PNG ~1-3MB）约消耗 1000-2000 tokens，1fps 持续运行成本较高。建议仅在关键帧且用户手动启用时调用。
5. **C4 性能**：像素差计算如使用完整 PNG 解码会较慢（~10ms），建议生成 64×64 缩略图后计算。

## 验证计划

### 自动化测试

```bash
# 1. Go 编译验证
go build ./...

# 2. Go 静态检查
go vet ./...

# 3. 现有 argus 测试
go test ./backend/internal/argus/... -v

# 4. 新增 screen_observer 测试（C3 完成后）
go test ./backend/internal/argus/ -run TestScreenObserver -v

# 5. 数据竞争检测（C1 完成后）
go test ./backend/internal/argus/ -race -v
```

### 手动验证

1. 启动 Gateway，打开前端 UI，点击「子智能体」Tab → 应显示完整管理视图（A1）
2. 切换英文语言 → 标签应显示英文而非 key 原文（A2）
3. 开关视觉智能体 → WS 消息应到达后端并触发 Observer 启停（B1+B3）

## 变更记录

| 日期 | 变更内容 |
|------|----------|
| 2026-02-27 | 创建任务跟踪文档，基于审计报告 + 代码实际验证 |
