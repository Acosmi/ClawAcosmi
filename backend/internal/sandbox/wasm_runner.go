//go:build cgo && sandbox_wasm
// +build cgo,sandbox_wasm

// =============================================================================
// 文件: backend/internal/sandbox/wasm_runner.go | 模块: sandbox | 职责: WebAssembly 沙箱执行引擎 (CGO 桥接)
// 审计: V12 2026-02-21 | 状态: ✅ 已审计
// =============================================================================

// Package sandbox provides the Go bridge to the Rust acosmi-sandbox Wasm execution engine.
//
// [C-2] 通过 acosmi-sandbox Rust crate 提供沙箱化 Wasm 执行能力
//
// Go 侧使用方法:
//
//	result, err := sandbox.ExecuteWasm(wasmBytes, "stdin input", nil)
//	if result.ExitCode == 0 {
//	    fmt.Println(result.Stdout)
//	}
package sandbox

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../rust/target/release -lacosmi_sandbox
#include <stdlib.h>
#include <stdbool.h>
#include <stdint.h>

extern char* acosmi_sandbox_execute(
    const uint8_t* wasm_ptr,
    size_t wasm_len,
    const char* stdin_data,
    uint64_t max_fuel,
    uint32_t max_memory_mb,
    uint64_t timeout_secs,
    char** out_error
);

extern int acosmi_sandbox_validate(
    const uint8_t* wasm_ptr,
    size_t wasm_len,
    char** out_error
);

extern void acosmi_sandbox_free_string(char* s);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"
)

// ExecutionResult mirrors the Rust ExecutionResult struct.
type ExecutionResult struct {
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	ExitCode      int32  `json:"exit_code"`
	FuelConsumed  uint64 `json:"fuel_consumed"`
	FuelExhausted bool   `json:"fuel_exhausted"`
	Error         string `json:"error,omitempty"`
}

// ExecutionOptions configures resource limits for Wasm execution.
type ExecutionOptions struct {
	// MaxFuel is the maximum instruction steps (0 = default: 1 billion).
	MaxFuel uint64
	// MaxMemoryMB is the maximum memory in MB (0 = default: 256 MB).
	MaxMemoryMB uint32
	// TimeoutSecs is the maximum execution time in seconds (0 = default: 300s).
	TimeoutSecs uint64
}

// ExecuteWasm runs a Wasm binary in the Rust sandbox.
func ExecuteWasm(wasmBytes []byte, stdinData string, opts *ExecutionOptions) (*ExecutionResult, error) {
	if len(wasmBytes) == 0 {
		return nil, errors.New("wasm bytes is empty")
	}

	wasmPtr := (*C.uint8_t)(unsafe.Pointer(&wasmBytes[0]))
	wasmLen := C.size_t(len(wasmBytes))

	var cStdin *C.char
	if stdinData != "" {
		cStdin = C.CString(stdinData)
		defer C.free(unsafe.Pointer(cStdin))
	}

	var maxFuel C.uint64_t
	var maxMemoryMB C.uint32_t
	var timeoutSecs C.uint64_t
	if opts != nil {
		maxFuel = C.uint64_t(opts.MaxFuel)
		maxMemoryMB = C.uint32_t(opts.MaxMemoryMB)
		timeoutSecs = C.uint64_t(opts.TimeoutSecs)
	}

	var cErr *C.char
	resultJSON := C.acosmi_sandbox_execute(
		wasmPtr, wasmLen, cStdin,
		maxFuel, maxMemoryMB, timeoutSecs,
		&cErr,
	)

	if resultJSON == nil {
		return nil, extractSandboxError(cErr, "wasm execution failed")
	}

	jsonStr := goStringFromSandbox(resultJSON)

	var result ExecutionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, errors.New("failed to parse execution result: " + err.Error())
	}

	return &result, nil
}

// ValidateWasm checks if a Wasm binary is valid without executing it.
func ValidateWasm(wasmBytes []byte) error {
	if len(wasmBytes) == 0 {
		return errors.New("wasm bytes is empty")
	}

	wasmPtr := (*C.uint8_t)(unsafe.Pointer(&wasmBytes[0]))
	wasmLen := C.size_t(len(wasmBytes))

	var cErr *C.char
	code := C.acosmi_sandbox_validate(wasmPtr, wasmLen, &cErr)
	if code != 0 {
		return extractSandboxError(cErr, "invalid wasm module")
	}
	return nil
}

// --- Internal helpers ---

func goStringFromSandbox(cstr *C.char) string {
	if cstr == nil {
		return ""
	}
	s := C.GoString(cstr)
	C.acosmi_sandbox_free_string(cstr)
	return s
}

func extractSandboxError(cErr *C.char, fallback string) error {
	if cErr != nil {
		msg := C.GoString(cErr)
		C.acosmi_sandbox_free_string(cErr)
		return errors.New(msg)
	}
	return errors.New(fallback)
}
