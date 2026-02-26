# 视觉理解方案 C+D 审计报告

> 审计日期：2026-02-27
> 审计范围：本次新建/修改的 10 个文件 + 关键集成点

## 审计摘要

| 级别 | 数量 |
|------|------|
| P0（阻断） | 2 |
| P1（必修） | 4 |
| P2（建议） | 5 |

**真实完成度评估：~45%**

> [!CAUTION]
> 核心类型和算法已实现（~80% 代码骨架），但**关键集成接线全部缺失**，功能端到端不可用。

---

## 发现列表

### P0: 阻断项

| # | 文件 | 问题描述 | 影响 |
|---|------|----------|------|
| P0-1 | `app-render.ts` | **subagents 渲染分支缺失**：`renderSubAgents()` 已定义但 `app-render.ts` 中无 `case "subagents"` 调用。点击子智能体 Tab 显示空白。 | 前端功能完全不可用 |
| P0-2 | `locales/en.ts` | **英文 i18n 键全部缺失**：仅添加了 zh.ts 的 18 个键，en.ts 未添加任何对应键。英文环境下所有标签显示 key 原文。 | 国际化功能异常 |

### P1: 必修项

| # | 文件 | 问题描述 | 建议修复 |
|---|------|----------|----------|
| P1-1 | `bridge.go` | **ScreenObserver 未集成**：`Bridge` 结构体中无 ScreenObserver 字段，Start/Stop 未接入 Observer 生命周期。Observer 在 bridge 层完全孤立。 | 在 Bridge 中添加 `observer *ScreenObserver` 字段，Start 时创建并启动，Stop 时关闭 |
| P1-2 | `attempt_runner.go` | **主 Agent 未从 Buffer 读取**：`ArgusBridgeForAgent` 接口未暴露 `ObservationBuffer`，主 Agent 无法通过 `LatestKeyframe()` 获取视觉状态。 | 在 `ArgusBridgeForAgent` 接口中添加 `ObservationBuffer() *argus.ObservationBuffer` 方法 |
| P1-3 | 后端 WS handler | **`subagent_ctl` WS 命令未实现**：前端 `sendSubAgentCtl()` 发送 `type: "subagent_ctl"` 消息，但后端无对应 WS handler 处理此消息类型。前端控制指令会被静默丢弃。 | 在 gateway WS handler 中添加 `subagent_ctl` case |
| P1-4 | `vla_client.go` | **AnthropicVisionClient.Infer 是 stub**：始终返回 `{Action: "DONE"}`，未真正调用 Anthropic API。VLA 推理完全不工作。 | 接入 Anthropic Messages API（base64 图片 → tool_use） |

### P2: 建议项

| # | 文件 | 问题描述 | 建议修复 |
|---|------|----------|----------|
| P2-1 | `screen_observer.go` | **SetGoal/SetInterval 存在数据竞争**：`cfg.Goal` 和 `cfg.BaseInterval` 在主 goroutine 写、loop goroutine 读，无锁或 atomic 保护。`go vet -race` 会报警。 | 改用 `atomic.Value` 或 `sync.RWMutex` 保护 cfg 字段 |
| P2-2 | `views/subagents.ts` | **无 CSS 样式**：使用了 `subagents-view`、`subagent-card`、`subagent-row` 等 class，但未在任何 CSS 文件中定义这些样式。视图布局会失效。 | 在 index.css 或独立 CSS 中添加对应样式 |
| P2-3 | `screen_observer.go` | **无单元测试**：ScreenObserver 核心逻辑（截图循环、去重、VLA 调用）无测试覆盖。 | 添加 `screen_observer_test.go`，mock CaptureFunc 测试循环 |
| P2-4 | `screen_observer.go:172` | **ChangeMagnitude 硬编码 1.0**：注释说"简化版"，实际未实现像素差计算。所有变化帧都标记为关键帧，违背方案阈值设计。 | 实现基于缩略图像素差的变化幅度计算 |
| P2-5 | 验证步骤 | **未执行 `go vet`**：编码规范要求 go vet 零警告，本次未运行。 | 执行 `go vet ./...` 验证 |

---

## 集成完成度明细

| 数据通路 | 状态 | 说明 |
|----------|------|------|
| 风险分级 → 审批门 (D) | ✅ 已连通 | `executeArgusTool()` → `ClassifyActionRisk()` → `ShouldRequireApproval()` |
| 截图 → Buffer (C) | ⚠️ 孤立 | ScreenObserver 和 Buffer 代码完整，但未被任何上游代码创建或调用 |
| Buffer → 主 Agent (C) | ❌ 未连通 | `ArgusBridgeForAgent` 接口无 Buffer 暴露方法 |
| 前端 Tab → 视图渲染 | ❌ 未连通 | Tab 注册了但 render 分支缺失 |
| 前端 WS → 后端控制 | ❌ 未连通 | 前端发送 `subagent_ctl`，后端无 handler |
| 配置 → 运行时 | ⚠️ 部分 | `SubAgentConfig` 类型已定义，但无代码读取配置创建 Observer |

## 延迟项

以下项需推迟处理，建议登记到 `docs/renwu/deferred-items.md`：

- [ ] P0-1: `app-render.ts` 添加 subagents 渲染分支
- [ ] P0-2: `locales/en.ts` 添加 18 个英文 i18n 键
- [ ] P1-1: `bridge.go` 集成 ScreenObserver 生命周期
- [ ] P1-2: `ArgusBridgeForAgent` 暴露 ObservationBuffer
- [ ] P1-3: 后端 WS handler 支持 `subagent_ctl`
- [ ] P1-4: AnthropicVisionClient 接入真实 API
- [ ] P2-1: 修复 SetGoal/SetInterval 数据竞争
- [ ] P2-2: 添加 subagents CSS 样式
- [ ] P2-3: 添加 screen_observer_test.go
- [ ] P2-4: 实现 ChangeMagnitude 像素差计算
- [ ] P2-5: 执行 go vet 验证
