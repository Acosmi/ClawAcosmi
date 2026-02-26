---
document_type: Deferred
status: Draft
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: false
---

# Argus macOS: Accessibility API 优先策略

## 背景

Argus 视觉子智能体当前使用本地视觉模型理解截屏内容、判断 UI 元素位置。
macOS 提供了 Accessibility API，可以直接获取元素位置、标签、状态，
大部分场景下比视觉模型更快更精确。

## 优化方案

### 优先用 Accessibility API 的工具

| 工具 | 当前方式 | 优化后 |
|------|----------|--------|
| `locate_element` | 视觉模型截屏分析 | 优先 Accessibility 查询，fallback 视觉模型 |
| `click` / `double_click` | CGEvent 坐标 | 可结合 Accessibility 元素定位 |
| `read_text` | OCR 截屏 | 优先 Accessibility 读取文本，fallback OCR |

### 仍需视觉模型的工具

| 工具 | 原因 |
|------|------|
| `describe_scene` | 需要理解画面整体内容，Accessibility 无法"看" |
| `capture_screen` | 截屏本身就是视觉操作 |
| `watch_for_change` | 监控视觉变化 |
| `detect_dialog` | 可混合：Accessibility 检测窗口 + 视觉确认 |

### 视觉模型仍必要的场景

- 浏览器内网页内容（DOM Accessibility 暴露不完整）
- 自定义渲染 UI（游戏、Canvas、Electron）
- 无障碍支持差的第三方 app
- 图片/视频内容理解

## 预期收益

- 减少视觉模型调用次数 → 降低延迟
- 元素定位更精确（像素级 vs 视觉估算）
- 减少对 GPU/模型推理的依赖

## 实施优先级

低 — 当前架构可用，属于性能优化。
