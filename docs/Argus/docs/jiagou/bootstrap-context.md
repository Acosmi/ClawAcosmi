# Go+Rust 混合架构改造 — 通用窗口上下文

> 在新窗口中使用: 告诉 AI **"执行 Phase X Batch Y，上下文在 `docs/jiagou/bootstrap-context.md`"**

---

## 项目信息

- **项目**: Argus-Compound（"24小时之眼"）
- **路径**: `/Users/fushihua/Desktop/Argus-compound`
- **架构**: Go+Rust 混合架构（Go 编排 + Rust 底盘）
- **架构文档**: `docs/jiagou/改造Go+Rust 混合架构.md`
- **任务清单**: `docs/jiagou/phase-task.md`

---

## 当前进度

| Phase | Batch | 状态 |
|-------|-------|------|
| Phase 1 | Batch A (Cargo + CG 捕获 + Go FFI) | ✅ 完成 |
| Phase 1 | Batch B (CGEvent 输入注入) | ✅ 完成 |
| Phase 1 | Batch C (SCK 升级) | ✅ 完成 |
| Phase 2 | Batch A (SIMD 图像缩放) | ✅ 完成 |
| Phase 2 | Batch B (关键帧提取) | ✅ 完成 |
| Phase 3 | (SHM IPC) | ✅ 完成 |
| Phase 4 | Batch A/B/C (验证+性能+文档) | ✅ 完成 |
| 延迟项 | PII/Crypto/Metrics/purego/AKS | ✅ 完成 |

> ⚠️ 每完成一个 Batch 后，更新此表和 `phase-task.md`

---

## 关键文件索引

### Rust 核心库 (`rust-core/`)

| 文件 | 说明 |
|------|------|
| `Cargo.toml` | 依赖: core-graphics 0.24, core-foundation 0.10 |
| `src/lib.rs` | C ABI 导出入口 |
| `src/capture.rs` | CG 屏幕捕获 (已实现) |
| `src/input.rs` | CGEvent 输入注入 (已实现) |
| `src/imaging.rs` | SIMD 图像缩放 (Phase 2, 已实现) |
| `src/keyframe.rs` | 关键帧差分+哈希 (Phase 2, 已实现) |
| `src/shm.rs` | SHM IPC 零拷贝帧传递 (Phase 3, 已实现) |
| `src/pii.rs` | PII 检测+脱敏 (延迟项, 已实现) |
| `src/crypto.rs` | AES-256-GCM 加密 (延迟项, 已实现) |
| `src/metrics.rs` | Rust 侧 Prometheus 指标 (延迟项, 已实现) |
| `include/argus_core.h` | C 头文件 (全部模块 API) |

### Go FFI 绑定 (`go-sensory/`)

| 文件 | 说明 |
|------|------|
| `internal/capture/rust_capture.go`  | `RustCapturer` 实现 `Capturer` 接口            |
| `internal/input/rust_input.go`      | `RustInputController` 实现 `InputController` 接口 |
| `internal/imaging/rust_scaler.go`   | Rust SIMD 图像缩放 FFI (`RustResizeBGRA`)       |
| `internal/pipeline/rust_keyframe.go`| Rust 关键帧差分+哈希 FFI (`RustCalcChangeRatio`) |
| `internal/ipc/rust_shm.go`          | Rust SHM IPC FFI (`RustShmWriter`)               |
| `internal/pipeline/rust_pii.go`     | Rust PII 过滤 FFI (`RustPIIFilter`)              |
| `internal/crypto/rust_crypto.go`    | Rust AES-GCM FFI (`RustAESEncrypt/Decrypt`)      |
| `internal/metrics/rust_metrics.go`  | Rust 指标 FFI (`GetRustMetrics`)                 |

### Go 原始实现 (参考)

| 文件 | 说明 |
|------|------|
| `internal/capture/capture.go` | `Capturer` 接口 (11 methods) |
| `internal/capture/darwin_sck.go` | SCK CGO 实现 (916 行) |
| `internal/capture/darwin.go` | CG CGO 实现 |
| `internal/input/input.go` | `InputController` 接口 (13 methods) |
| `internal/input/darwin.go` | CGEvent CGO 实现 |
| `internal/input/guardrails.go` | 安全护栏 |
| `internal/input/approval_gateway.go` | 审批网关 |
| `internal/pipeline/keyframe.go` | 关键帧提取 |
| `internal/pipeline/pipeline.go` | 管线编排 |
| `internal/imaging/` | 图像缩放 |
| `internal/ipc/shm_writer.go` | SHM IPC |

---

## 各 Phase 入口指引

### Phase 1 Batch B — CGEvent 输入注入

```
目标: 实现 rust-core/src/input.rs 中所有 argus_input_* 函数
参考: go-sensory/internal/input/darwin.go (CGEvent 实现)
头文件: rust-core/include/argus_core.h (API 已定义)
Go绑定: 创建 go-sensory/internal/input/rust_input.go
测试: 鼠标移动+点击, 键盘输入, Go FFI 调用
```

### Phase 1 Batch C — SCK 升级

```
目标: 引入 screencapturekit crate, 替代 CG 后端
参考: go-sensory/internal/capture/darwin_sck.go
新增: rust-core/src/capture_sck.rs
```

### Phase 2 Batch A — SIMD 图像缩放

```
目标: 引入 fast_image_resize, 实现 argus_resize_image
参考: go-sensory/internal/imaging/
新增: rust-core/src/imaging.rs
Go绑定: 替换 internal/imaging 调用
```

### Phase 2 Batch B — 关键帧提取

```
目标: Rust 实现帧差分哈希算法
参考: go-sensory/internal/pipeline/keyframe.go
新增: rust-core/src/keyframe.rs
Go绑定: Pipeline 调用 Rust 关键帧
```

### Phase 3 — SHM IPC

```
目标: memmap2 实现安全 SHM
参考: go-sensory/internal/ipc/shm_writer.go
新增: rust-core/src/shm.rs
Go绑定: 替换 internal/ipc/shm_writer.go
```

### Phase 4 — 集成验证

```
目标: 全链路测试 + 性能回归 + CI 更新
重点: 端到端、MCP链路、ReAct Agent、web-console 兼容
```

---

## ⚠️ 任务完成后必做

每完成一个 Batch 后，**必须依次执行以下更新**：

### 1. 更新任务清单

- 编辑 `docs/jiagou/phase-task.md`，将已完成项标记为 `[x]`
- 更新 `docs/jiagou/bootstrap-context.md` 中的「当前进度」表

### 2. 按技能工作流更新架构文档（参考 `.agent/skills/acosmi-refactor/SKILL.md` 步骤 5）

- 更新 `docs/gouji/<module>.md` — 受影响模块的架构文档
  - 如改动了 capture → 更新 `docs/gouji/go-sensory.md`
  - 如新增了模块 → 创建对应文档
- 更新 `docs/jiagou/改造Go+Rust 混合架构.md` — 标记已完成的 Phase/Batch
- 如有未完成项 → 登记到延迟待办文档

### 3. 验证

- `cargo build --release`（Rust 编译）
- `go build ./...`（Go 编译 + CGO 链接）
- `go vet ./...`（静态检查）

### 4. 窗口切换时

- 在切换前将上述更新全部完成
- 在新窗口中使用: `"执行 Phase X Batch Y，上下文在 docs/jiagou/bootstrap-context.md"`

---

## 构建命令

```bash
# Rust 构建
cd rust-core && cargo build --release
# 产物: target/release/libargus_core.dylib

# Go 构建 (自动链接 Rust dylib)
cd go-sensory && go build ./...

# Go 测试
cd go-sensory && go test -v ./...
```

## 编码规范

- Rust: C ABI 函数以 `argus_` 前缀，返回 `int32_t` 错误码
- Go FFI: `Rust` 前缀命名 (如 `RustCapturer`, `RustInputController`)
- 内存: Rust 分配 → 必须通过 `argus_free_buffer` 释放
- 日志: Rust 使用 `eprintln!` (stderr)，不污染 stdout
