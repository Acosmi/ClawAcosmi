# purego 方案评估

## 背景

当前 Argus-Compound 通过 CGO 调用 Rust `libargus_core.dylib`。purego 是一种通过 `dlopen/dlsym` 直接调用动态库函数的方案，可以消除 CGO 开销。

## CGO vs purego 对比

| 维度 | CGO | purego |
|------|-----|--------|
| 调用开销 | ~200ns (goroutine 栈切换) | ~50ns (无栈切换) |
| 编译 | 需要 C 编译器 (gcc/clang) | 纯 Go 编译 |
| 交叉编译 | 困难 (需要目标平台 toolchain) | 简单 (CGO_ENABLED=0) |
| 调试 | cgo 堆栈不透明 | 更好的堆栈追踪 |
| 类型安全 | 编译时检查 | 运行时检查 |
| 指针传递 | 自动 pinning | 需手动管理 |
| macOS 框架 | 完整支持 | 需要手动 dlopen |

## 性能影响分析

当前热路径的 FFI 调用频率:

| 操作 | 每帧调用次数 | 每次耗时 | CGO 开销占比 |
|------|-------------|---------|-------------|
| Image Resize | 1 | 4.1ms | ~0.005% |
| Keyframe Diff | 1 | 0.64ms | ~0.03% |
| SHM Write | 1 | 47µs | ~0.4% |
| PII Filter | 0-1 | 7µs | ~2.9% |

> CGO 调用开销 (~200ns) 相比 Rust 侧计算时间几乎可忽略。仅 PII Filter 这类微秒级操作才有明显占比。

## 结论

**建议: 暂不迁移到 purego。**

理由:

1. CGO 开销占比 < 3%，性能收益极小
2. CGO 提供编译时类型安全，purego 需运行时检查
3. macOS CoreGraphics/CoreFoundation 框架通过 CGO 集成更稳定
4. purego 在 arm64 macOS 上的 ABI 兼容性仍有社区报告的边缘问题

**何时重新评估:**

- 如果 SHM 路径需要更高频率调用 (>10K ops/sec)
- 如果需要支持 CGO_ENABLED=0 的纯 Go 编译
- 如果需要 Linux/Windows 跨平台支持
