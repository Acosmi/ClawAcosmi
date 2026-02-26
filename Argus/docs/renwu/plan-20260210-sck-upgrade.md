# ScreenCaptureKit 升级计划

## 背景与目标

当前 `go-sensory` 使用 CoreGraphics `CGWindowListCreateImage` 实现屏幕捕获，存在以下限制：

- Retina 缩放因子硬编码为 2.0
- 基于 ticker 轮询，非事件驱动
- 无法按窗口/应用过滤捕获目标

**目标**：升级到 Apple ScreenCaptureKit 框架，获得硬件加速、精确缩放、窗口级过滤等能力。CoreGraphics 实现保留作为回退方案。

## 阶段划分

### P1: 核心实现

- **目标**：新增 SCK 后端并支持切换
- **步骤**：
  - [x] Step A: 新建 `darwin_sck.go`（SCKCapturer 实现）
  - [x] Step B: 修改 `capture.go`（Backend 配置 + 工厂路由）
  - [x] Step C: 修改 `main.go`（`--backend` 参数）
- **验证方式**：编译通过 + 手动启动验证
- **预计工作量**：2~3 次对话

### P2: 验证与归档

- **目标**：功能回归测试 + 架构文档更新
- **步骤**：
  - [x] 回退测试（`--backend cg`）
  - [x] 更新 `docs/gouji/go-sensory.md`
- **预计工作量**：1 次对话

## 风险与注意事项

- ScreenCaptureKit 要求 macOS 12.3+
- 首次运行需要授予屏幕录制权限
- CGo + Objective-C 桥接代码需仔细管理内存

## 变更记录

| 日期 | 变更内容 |
|------|----------|
| 2026-02-10 | 创建初始计划 |
| 2026-02-10 | P1 + P2 全部完成，编译通过，IPC 测试全部通过 |
