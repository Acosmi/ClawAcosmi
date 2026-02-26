# 编码规范 — Argus-Compound Go+Rust 混合架构

> 版本：v1.0 · 2026-02-15

---

## 一、通用规范

| 项目 | 要求 |
|------|------|
| **默认语言** | 交流、报告、文档说明均使用**中文**；代码注释和变量命名使用英文 |
| **文件命名** | 小写英文 + 短横线（kebab-case） |
| **组件命名** | 表达**意图**而非实现（`analyzeImage` 而非 `processData`） |
| **目录组织** | 按功能域组织（`auth/` 而非 `controllers/`） |
| **公共工具** | 统一放在 `utils/` 或 `shared/`，不分散在业务模块 |

---

## 二、Go 编码规范

### 2.1 包结构

- 使用 `internal/` 隐藏内部包，防止外部依赖内部实现
- 接口定义在**消费侧**（调用方定义接口，实现方满足接口）
- CGO 相关文件可豁免 300 行文件上限

### 2.2 错误处理

```go
// ✅ 正确：显式处理每个错误
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doSomething failed: %w", err)
}

// ❌ 错误：忽略错误
result, _ := doSomething()
```

### 2.3 CGO/FFI 绑定

```go
// 标准 CGO 头部
// #cgo LDFLAGS: -L${SRCDIR}/../../rust-core/target/release -largus_core
// #include "argus_core.h"
import "C"

// 必须在 defer 中释放 Rust 分配的内存
defer C.argus_free_buffer(ptr, C.size_t(size))
```

### 2.4 并发

- 使用 `context.Context` 传递取消信号
- channel 必须有明确的关闭方，避免泄漏
- 使用 `sync.WaitGroup` 或 `errgroup` 协调 goroutine

---

## 三、Rust 编码规范

### 3.1 项目结构

```
rust-core/
├── Cargo.toml          # workspace 根配置
├── src/
│   ├── lib.rs          # C ABI 导出入口
│   ├── capture.rs      # SCK 屏幕捕获
│   ├── imaging.rs      # SIMD 图像处理
│   ├── input.rs        # CGEvent 输入注入
│   ├── keyframe.rs     # 关键帧提取
│   └── shm.rs          # SHM IPC
└── include/
    └── argus_core.h    # C 头文件（与导出函数同步更新）
```

### 3.2 unsafe 使用约束

- `unsafe` 块必须加注释说明安全性保证
- 不裸用 `unsafe`，封装在安全的 pub 函数中
- FFI 边界函数内的 `unsafe` 必须做空指针检查

```rust
/// # Safety
/// `out_ptr` must be non-null and aligned.
#[no_mangle]
pub extern "C" fn argus_capture_frame(out_ptr: *mut *mut u8) -> i32 {
    if out_ptr.is_null() {
        return -1; // ERR_NULL_POINTER
    }
    // SAFETY: we checked out_ptr is non-null above
    unsafe {
        *out_ptr = allocate_frame();
    }
    0
}
```

### 3.3 命名与导出

- 所有 C ABI 导出函数使用 `argus_` 前缀
- `#[no_mangle] pub extern "C"` 修饰
- 返回 `i32` 错误码，0 = 成功，负数 = 错误
- 在 `include/argus_core.h` 中同步声明

### 3.4 内存管理

- Rust 分配的内存必须通过 `argus_free_buffer` 释放
- 禁止让 Go 侧 `free()` Rust 分配的内存
- 使用 `Box::into_raw` / `Box::from_raw` 管理堆内存传递

---

## 四、Git 提交规范

```
<类型>(<作用域>): <简要描述>

类型：feat | fix | refactor | docs | chore | test
作用域：go-sensory | rust-core | web-console | docs | pipeline | agent | vlm | mcp
```

示例：

```
feat(rust-core): 实现 SCK 屏幕捕获 C ABI 导出
fix(pipeline): 修复 FFI 调用后内存未释放
docs(gouji): 更新 pipeline 架构文档
```

---

## 五、代码质量门禁

| 检查项 | 命令 | 要求 |
|--------|------|------|
| Go 编译 | `go build ./...` | 零错误 |
| Go 静态分析 | `go vet ./...` | 零警告 |
| Rust 编译 | `cargo build --release` | 零错误 |
| Rust Clippy | `cargo clippy -- -D warnings` | 零警告 |
| 文件行数 | — | 单文件 ≤ 300 行（CGO 文件除外） |
| 函数行数 | — | 单函数 ≤ 50 行 |
