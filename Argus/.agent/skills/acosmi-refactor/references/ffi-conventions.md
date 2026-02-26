# FFI 规范 — Go ↔ Rust 桥接约定

> 版本：v1.0 · 2026-02-15

---

## 一、函数命名

所有 Rust 导出到 C ABI 的函数遵循以下命名规范：

```
argus_<module>_<action>
```

| 模块 | 前缀 | 示例 |
|------|------|------|
| 屏幕捕获 | `argus_capture_` | `argus_capture_frame` |
| 图像处理 | `argus_image_` | `argus_image_resize`, `argus_image_encode` |
| 关键帧 | `argus_keyframe_` | `argus_keyframe_extract`, `argus_keyframe_diff` |
| 输入注入 | `argus_input_` | `argus_input_click`, `argus_input_type` |
| SHM IPC | `argus_shm_` | `argus_shm_create`, `argus_shm_write` |
| 无障碍 | `argus_ax_` | `argus_ax_list_elements`, `argus_ax_focused_app` |
| 内存管理 | `argus_` | `argus_free_buffer` |

---

## 二、函数签名规范

### 2.1 Rust 导出侧

```rust
#[no_mangle]
pub extern "C" fn argus_<module>_<action>(
    // 输入参数：值类型或 const 指针
    input: *const u8,
    input_len: usize,
    // 输出参数：可变指针
    out_result: *mut *mut u8,
    out_result_len: *mut usize,
) -> i32  // 返回错误码
```

### 2.2 Go 调用侧

```go
// #cgo LDFLAGS: -L${SRCDIR}/../../rust-core/target/release -largus_core
// #include "argus_core.h"
import "C"
import "unsafe"

func CallRust(input []byte) ([]byte, error) {
    var outPtr *C.uint8_t
    var outLen C.size_t
    
    rc := C.argus_module_action(
        (*C.uint8_t)(unsafe.Pointer(&input[0])),
        C.size_t(len(input)),
        &outPtr,
        &outLen,
    )
    if rc != 0 {
        return nil, fmt.Errorf("FFI call failed: error code %d", rc)
    }
    defer C.argus_free_buffer((*C.uint8_t)(outPtr), outLen)
    
    // 拷贝到 Go 管理的内存
    result := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
    return result, nil
}
```

---

## 三、错误码规范

| 错误码 | 含义 | 说明 |
|--------|------|------|
| `0` | 成功 | — |
| `-1` | 空指针错误 | 输入/输出指针为 null |
| `-2` | 无效参数 | 参数值超出合法范围 |
| `-3` | 内部错误 | Rust 内部 panic 或异常 |
| `-4` | 资源不可用 | 如屏幕捕获权限未授予 |
| `-5` | 内存分配失败 | 内存不足 |
| `-100..` | 模块特定错误 | 各模块自定义错误码 |

Rust 侧使用常量定义：

```rust
pub const ARGUS_OK: i32 = 0;
pub const ARGUS_ERR_NULL_PTR: i32 = -1;
pub const ARGUS_ERR_INVALID_PARAM: i32 = -2;
pub const ARGUS_ERR_INTERNAL: i32 = -3;
pub const ARGUS_ERR_UNAVAILABLE: i32 = -4;
pub const ARGUS_ERR_OOM: i32 = -5;
```

---

## 四、内存所有权协议

> **核心原则：谁分配，谁释放。**

### 4.1 Rust → Go 数据传递

```
Rust 分配 → 通过 out 指针传给 Go → Go 使用完毕后调用 argus_free_buffer 释放
```

```rust
// Rust 侧：分配并导出
let data = vec![0u8; size];
let ptr = Box::into_raw(data.into_boxed_slice());
unsafe { *out_ptr = (*ptr).as_mut_ptr(); }

// Rust 侧：释放函数
#[no_mangle]
pub extern "C" fn argus_free_buffer(ptr: *mut u8, len: usize) {
    if !ptr.is_null() {
        unsafe {
            let _ = Box::from_raw(std::slice::from_raw_parts_mut(ptr, len));
        }
    }
}
```

```go
// Go 侧：使用后释放
defer C.argus_free_buffer(outPtr, outLen)
```

### 4.2 Go → Rust 数据传递

```
Go 拥有内存 → 传 const 指针给 Rust → Rust 只读不持有 → Go 自行管理
```

- Rust 侧接收 `*const u8`，**不得**存储或释放此指针
- 如需保留数据，Rust 必须自行拷贝

### 4.3 禁止事项

- ❌ Go 侧使用 `C.free()` 释放 Rust 分配的内存
- ❌ Rust 侧释放 Go 传入的指针
- ❌ 跨 FFI 边界传递 Go 的 `[]byte` 切片头（需取 `&data[0]`）
- ❌ 在 Rust 侧持有 Go 传入指针的长期引用

---

## 五、C 头文件维护

`rust-core/include/argus_core.h` 必须与 Rust 导出函数保持同步：

```c
#ifndef ARGUS_CORE_H
#define ARGUS_CORE_H

#include <stdint.h>
#include <stddef.h>

// ===== 错误码 =====
#define ARGUS_OK             0
#define ARGUS_ERR_NULL_PTR  -1
#define ARGUS_ERR_INVALID   -2
#define ARGUS_ERR_INTERNAL  -3
#define ARGUS_ERR_UNAVAIL   -4
#define ARGUS_ERR_OOM       -5

// ===== 屏幕捕获 =====
int32_t argus_capture_frame(
    uint8_t **out_pixels,
    int32_t *out_width,
    int32_t *out_height
);

// ===== 图像处理 =====
int32_t argus_image_resize(
    const uint8_t *src, int32_t src_w, int32_t src_h,
    uint8_t *dst, int32_t dst_w, int32_t dst_h
);

// ===== 内存管理 =====
void argus_free_buffer(uint8_t *ptr, size_t len);

#endif // ARGUS_CORE_H
```

**规则**：每新增一个 `#[no_mangle]` 导出函数，必须同步在头文件中添加声明。

---

## 六、调试与日志

- Rust 侧错误信息输出到 `stderr`（使用 `eprintln!`），不干扰 Go 的 stdout
- 可选：通过 FFI 传递格式化的错误消息字符串
- 生产环境建议通过 `env_logger` 控制日志级别
