# 视觉理解 C+D 修复 Bootstrap 上下文

> 创建日期：2026-02-27
> 前置文档：audit-20260227-vision-cd.md

## 当前状态摘要

- 已完成：核心类型 + 算法骨架（action_risk, observation, buffer, observer, vla_client）
- 未完成：**所有集成接线** — P0×2 + P1×4 + P2×5
- 实际完成度：~45%

## 本次目标

修复 P0 + P1 项，使端到端功能可用。

## 待办清单

### Batch A: P0 阻断修复

- [ ] A1: `app-render.ts` 添加 `case "subagents"` 渲染 `renderSubAgents()`
- [ ] A2: `locales/en.ts` 添加 18 个英文 i18n 键

### Batch B: P1 集成接线

- [ ] B1: `bridge.go` 添加 ScreenObserver 字段 + Start/Stop 生命周期
- [ ] B2: `ArgusBridgeForAgent` 接口暴露 `ObservationBuffer()`
- [ ] B3: 后端 WS handler 添加 `subagent_ctl` 消息处理
- [ ] B4: AnthropicVisionClient 接入真实 API（或标记为 Phase 4）

### Batch C: P2 改进

- [ ] C1: 修复 SetGoal/SetInterval 数据竞争
- [ ] C2: 添加 subagents CSS 样式
- [ ] C3: 添加 screen_observer_test.go
- [ ] C4: 实现 ChangeMagnitude 像素差计算
- [ ] C5: 执行 go vet ./... 验证

## 关键文件索引

| 文件 | 用途 |
|------|------|
| `runner/action_risk.go` | 风险分级（已完成） |
| `runner/tool_executor.go` | 审批门（已完成） |
| `argus/observation.go` | 数据结构（已完成） |
| `argus/observation_buffer.go` | ring buffer（已完成） |
| `argus/vla_client.go` | VLA 接口（stub） |
| `argus/screen_observer.go` | 截图循环（孤立） |
| `argus/bridge.go` | **需接线** Observer |
| `runner/attempt_runner.go` | **需接线** Buffer |
| `ui/app-render.ts` | **需添加** render case |
| `ui/views/subagents.ts` | 管理视图（已完成） |

## 新窗口启动指令

```
/3-bootstrap
继续视觉理解 C+D 修复，上一轮已完成核心类型/算法骨架。
需修复 P0（render+i18n）和 P1（bridge/runner/WS integration）。
参考：docs/renwu/audit-20260227-vision-cd.md
```
