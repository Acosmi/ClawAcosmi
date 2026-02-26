// Package vision 提供视觉处理的 FFI 桥接层。
//
// 通过 extern "C" 接口调用 Rust 视觉处理内核，
// 负责帧处理、图像分析等高性能计算任务。
//
// 编译要求（Phase 10+ 启用）：
//   - Rust toolchain (rustup)
//   - cbindgen（自动生成 C 头文件）
//   - 可选：image crate
package vision

// 编译标志将在 Rust 库就绪后启用（Phase 10+）
// #cgo CFLAGS: -I${SRCDIR}/../../rust/acosmi-vision/include
// #cgo LDFLAGS: -L${SRCDIR}/../../rust/target/release -lacosmi_vision
// #include "vision_bridge.h"
// import "C"

// Available 返回 Rust 视觉引擎是否可用
func Available() bool {
	// TODO: Phase 10+ — 检查 Rust 库是否已编译
	return false
}
