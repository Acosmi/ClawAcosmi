# GUI Grounding 增强 Bootstrap

> 新窗口上下文 | 前置: GUI Grounding 升级已完成 (AX 原生检测)

## 任务目标

在 GUI Grounding 基础上完成三项增强：

1. ~~**.app + .pkg 封装** — 独立应用 bundle，支持安装引导和权限预设~~ ✅
2. ~~**统一权限检查** — 启动时检测 Screen Recording + Accessibility，引导用户授权~~ ✅
3. ~~**混合 AX 策略** — Electron 强制 AX + Chrome Web 区域深层枚举~~ ✅

## 执行顺序

1. ~~**Batch A**: .app bundle 结构 + Info.plist + Makefile target + .pkg 安装器~~ ✅
2. ~~**Batch B**: Rust `argus_check_permissions()` + Go 启动时权限引导~~ ✅
3. ~~**Batch C**: AXManualAccessibility 强制开启 + Web 区域深层递归~~ ✅

## 关键文件

| 文件 | 状态 | 说明 |
|------|------|------|
| `Makefile` | ✅ 已更新 | 新增 `make app` / `make pkg` target |
| `scripts/package/Info.plist` | ✅ 已创建 | 应用 bundle 配置 + 权限声明 |
| `scripts/package/build-pkg.sh` | ✅ 已创建 | .pkg 构建脚本 |
| `rust-core/src/accessibility.rs` | ✅ 已改造 | 增加权限检查函数 + AXManualAccessibility |
| `rust-core/include/argus_core.h` | ✅ 已更新 | 追加新函数声明 |
| `go-sensory/cmd/server/main.go` | ✅ 已改造 | 启动时统一权限检查 |
| `go-sensory/internal/agent/rust_accessibility.go` | ✅ 已改造 | 增加权限检查 FFI |

## 前置成果 (已完成)

- `accessibility.rs` — 3 个 C ABI 函数，AX 元素枚举 + JSON 导出
- `rust_accessibility.go` — Go FFI 绑定
- `ui_parser.go` — AX 优先 + VLM fallback 双路检测
- `react_loop.go` — SoM 模式 think()
- `main.go` 中已有 `detectBundleVLMConfig()` 检测 .app bundle 路径

## 延迟项

- 跨平台适配 (Windows UIA / Linux AT-SPI2) — P3，远期规划

## 遵循的规范

- 工作流: `.agent/workflows/1-refactor.md`
- 编码规范: `.agent/skills/acosmi-refactor/references/coding-standards.md`
- FFI 规范: `.agent/skills/acosmi-refactor/references/ffi-conventions.md`

## 新窗口启动指令

```
请读取 docs/renwu/bootstrap-gui-grounding-v2.md 和 docs/renwu/plan-20260215-gui-grounding-v2.md，按 /refactor 工作流执行 GUI Grounding 增强任务（Batch A → B → C）。
```
