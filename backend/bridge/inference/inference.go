// Package inference 提供推理引擎的 FFI 桥接层。
//
// 通过 extern "C" 接口调用 Rust 推理内核（llama-cpp-rs），
// 负责本地模型加载、推理和资源管理。
package inference

// Available 返回 Rust 推理引擎是否可用
func Available() bool {
	// TODO: Phase 10+ — 检查 Rust 库是否已编译
	return false
}
