# Bug 修复 Bootstrap — 审计发现修复

> 来源: `docs/renwu/global-health-audit.md` (2026-02-15)

## 待修复清单

### ✅ P0 — `capture_sck.rs:291` 内存分配器不匹配 (UB)

- **问题**: `std::alloc::alloc` 分配，`Vec::from_raw_parts` 释放 → 未定义行为
- **修复**: 改用 `pixels.clone()` (Vec) + `std::mem::forget` 方式，与 `argus_free_buffer` 对齐
- **文件**: `rust-core/src/capture_sck.rs` L289-296

### ✅ P1 — `shrink_to_fit` 不保证 capacity==len

- **问题**: 多处 `shrink_to_fit` 后传 `len` 给 `argus_free_buffer`，可能泄漏
- **修复**: 改用 `into_boxed_slice` + `Box::into_raw`，保证 capacity==len
- **文件**: `rust-core/src/crypto.rs`, `pii.rs`, `metrics.rs`

### ✅ P2 — Metrics 计数器永远为 0

- **问题**: `capture.rs`, `capture_sck.rs`, `imaging.rs`, `keyframe.rs`, `shm.rs` 未调用 `inc_*()`
- **修复**: 在各模块成功路径添加 `crate::metrics::inc_*()` 调用
- **文件**: 上述 5 个 `.rs` 文件

### ✅ P3 — Makefile test 遗漏模块

- **问题**: `make test` 未包含 `crypto` 和 `metrics` 测试
- **修复**: 在 Makefile test target 中添加 `./internal/crypto/` `./internal/metrics/`
- **文件**: `Makefile`

### ✅ P4 — Swift rpath 硬编码

- **问题**: 8 个 Go FFI 文件中 CGO LDFLAGS 硬编码了 `/Applications/Xcode.app/.../swift-5.5/macosx`，导致 Swift runtime 重复加载和类冲突警告
- **修复**: 替换为系统路径 `/usr/lib/swift`，消除 Swift 类重复加载警告
- **文件**: `rust_capture.go`, `rust_input.go`, `rust_scaler.go`, `rust_keyframe.go`, `rust_pii.go`, `rust_shm.go`, `rust_crypto.go`, `rust_metrics.go`

### ✅ P5 — go.mod 极简问题

- **问题**: `go.mod` 中依赖包标记为 `// indirect`，实际为直接依赖
- **修复**: `go mod tidy` 清理依赖关系
- **文件**: `go-sensory/go.mod`

## 验证步骤

```bash
cargo build --release        # Rust 编译
cargo clippy -- -D warnings  # 静态分析
cd go-sensory && go build ./...  # Go 编译
make test                    # 全量测试
```

## 新窗口启动指令

```
请读取 docs/renwu/bugfix-bootstrap.md，按 P0 → P1 → P2 → P3 顺序逐个修复，每个修复后运行验证步骤确认。
```
