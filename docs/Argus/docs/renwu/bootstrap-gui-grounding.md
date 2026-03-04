# GUI Grounding 升级 Bootstrap

> ✅ **已完成** | 2026-02-15 | 来源: 全局健康度审计

## 任务目标

用 macOS 原生 AXUIElement API (Rust 实现) 替换 `ui_parser.go` 中的 VLM 元素检测，将 UI 元素识别从 ~2s 降至 <5ms。

## 关键文件

| 文件 | 状态 | 说明 |
|------|------|------|
| `docs/renwu/plan-20260215-gui-grounding.md` | ✅ 完成 | 完整升级方案 |
| `rust-core/src/accessibility.rs` | ✅ 新增 | AX 元素枚举 + C ABI 导出，467 行 |
| `go-sensory/internal/agent/rust_accessibility.go` | ✅ 新增 | Go FFI 绑定，127 行 |
| `go-sensory/internal/agent/ui_parser.go` | ✅ 改造 | AX 优先 + VLM fallback 双路检测 |
| `go-sensory/internal/agent/som_drawing.go` | 保留 | SoM 绘制原语，102 行 |
| `go-sensory/internal/agent/react_loop.go` | ✅ 改造 | 新增 SoM 模式 think() |
| `rust-core/src/lib.rs` | ✅ 已更新 | 已添加 `mod accessibility` |
| `rust-core/include/argus_core.h` | ✅ 已更新 | 已追加 AX 函数声明 |

## 执行顺序

1. **Batch A**: ✅ 新增 `rust-core/src/accessibility.rs` + C ABI 导出 + 更新 header
2. **Batch B**: ✅ 新增 `rust_accessibility.go` FFI 绑定 + 改造 `ui_parser.go`
3. **Batch C**: ✅ 改造 `react_loop.go` 集成 SoM 模式

## 遵循的规范

- 工作流: `.agent/workflows/1-refactor.md`
- 编码规范: `.agent/skills/acosmi-refactor/references/coding-standards.md`
- FFI 规范: `.agent/skills/acosmi-refactor/references/ffi-conventions.md`

## 新窗口启动指令

```
请读取 docs/renwu/bootstrap-gui-grounding.md 和 docs/renwu/plan-20260215-gui-grounding.md，按 /refactor 工作流执行 GUI Grounding 升级任务（Batch A → B → C）。
```
