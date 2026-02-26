# Phase 1 Bootstrap — Rust 外设微内核

> 创建时间: 2026-02-15
> 上下文来源: `docs/jiagou/改造Go+Rust 混合架构.md`

## 本轮任务

执行 **Phase 1: Rust 外设微内核**，目标是创建 `rust-core/` Cargo workspace，实现 SCK 屏幕捕获 + CGEvent 输入注入，编译为 `libargus_core.dylib`，并完成 Go 侧 FFI 绑定。

## Checklist

- [x] 创建 `rust-core/` Cargo workspace (`Cargo.toml` + `src/lib.rs`)
- [x] 实现 CG 屏幕捕获 (`core-graphics` crate) — Batch A
- [ ] 实现 CGEvent 输入注入 — Batch B
- [x] 定义 C ABI 导出函数 (`#[no_mangle] pub extern "C"`)
- [x] 生成 `include/argus_core.h` 头文件
- [x] 编译为 `libargus_core.dylib` (303KB)
- [x] Go 侧 FFI 绑定 (`RustCapturer` 实现 `Capturer` 接口)
- [ ] 单元测试：通过 Rust 捕获屏幕帧 — Batch B
- [ ] 单元测试：通过 Rust 执行鼠标点击 — Batch B
- [ ] 集成测试：Go 调用 Rust FFI 完整链路 — Batch B

## 里程碑

通过 Rust 成功捕获屏幕帧并执行鼠标点击，Go 侧通过 FFI 正常调用。

## 关键文件上下文

### 需要参考的现有 Go 实现

| 文件 | 说明 |
|------|------|
| `go-sensory/internal/capture/capture.go` | `Capturer` 接口定义 (11 methods) |
| `go-sensory/internal/capture/darwin_sck.go` | SCK CGO 实现 (916 行，Objective-C) |
| `go-sensory/internal/capture/darwin.go` | CG 后端 (Fallback) |
| `go-sensory/internal/input/input.go` | `InputController` 接口 (13 methods) |
| `go-sensory/internal/input/darwin.go` | CGEvent 输入实现 |
| `go-sensory/internal/ipc/shm_writer.go` | SHM IPC |

### 架构规划文档

| 文件 | 说明 |
|------|------|
| `docs/jiagou/改造Go+Rust 混合架构.md` | 完整架构方案（已批准） |

### 目标项目结构

```
rust-core/
├── Cargo.toml
├── src/
│   ├── lib.rs          # C ABI 导出
│   ├── capture.rs      # SCK 屏幕捕获
│   └── input.rs        # CGEvent 输入
└── include/
    └── argus_core.h    # C 头文件
```

### Rust Crate 依赖参考

```toml
[dependencies]
screencapturekit = "0.2"    # SCK 绑定
core-graphics = "0.23"      # CoreGraphics
enigo = "0.2"               # 跨平台输入
```

## 注意事项

1. **macOS 权限**: SCK 需要「屏幕录制」权限，CGEvent 需要「辅助功能」权限
2. **ObjC 生命周期**: 使用 `Retained<T>` 智能指针管理 IOSurface/CMSampleBuffer
3. **构建**: 需要 `cargo build --release` 生成 dylib，然后 Go 侧 `#cgo LDFLAGS` 链接
4. **Go 版本**: 当前 Go 1.25.7
5. **测试**: CI 使用 `macos-latest` runner（CGO 依赖）
