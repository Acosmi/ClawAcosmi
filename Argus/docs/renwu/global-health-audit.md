# Argus-Compound 全局健康度审计报告

> 审计日期: 2026-02-15 | 审计范围: 全项目 Go+Rust 混合架构

---

## 一、架构合理性评估

### 1.1 总体架构评分：⭐⭐⭐⭐ (4/5)

项目采用 **「Go 做编排，Rust 做底盘」** 的混合架构，职责分界清晰：

| 层级 | 技术 | 评价 |
|------|------|------|
| 服务编排 (API/Agent/MCP/VLM) | Go | ✅ 合理，I/O 密集型天然适合 goroutine |
| CPU 热路径 (Capture/Imaging/Input/SHM) | Rust | ✅ 合理，消除 GC 抖动 + SIMD 加速 |
| FFI 桥接 | C ABI (CGO) | ✅ 行业标准方案，Discord/Cloudflare 同类实践 |
| 前端 | Next.js | ✅ 零影响，API 兼容 |
| 基础设施 | Docker (PG+Redis+Chroma) | ✅ 标准容器化 |

### 1.2 架构优点

1. **最小替换原则** — 仅 10 个 Rust 文件替换 CPU 热路径，未做全量重写
2. **增量可回滚** — SCK/CG 双后端策略，Rust 失败自动降级到 Go
3. **FFI 规范统一** — 所有导出函数使用 `argus_` 前缀 + `i32` 错误码
4. **内存管理清晰** — 统一通过 `argus_free_buffer` 释放 Rust 内存
5. **性能收益显著** — 图像缩放 4.2× faster，SHM 21K fps

### 1.3 架构风险点

| 风险 | 严重度 | 说明 |
|------|--------|------|
| 单一 dylib 膨胀 | ⚠️ 中 | 所有模块编译到 `libargus_core.dylib`，未按功能拆分 crate |
| 全局可变状态 | ⚠️ 中 | `capture_sck.rs` 使用 `OnceLock<Mutex>` 全局状态，限制多实例并发 |
| macOS 强依赖 | ⚠️ 中 | `screencapturekit`/`core-graphics` 锁死 macOS，Linux 部署需分层 |
| CGO 调用开销 | ⚡ 低 | 已评估 <3%，purego 方案暂不迁移 |

---

## 二、关键 Bug 与潜在问题

### 🔴 P0 — 严重：`capture_sck.rs` 内存分配器不匹配

**文件**: [capture_sck.rs](file:///Users/fushihua/Desktop/Argus-compound/rust-core/src/capture_sck.rs#L291)

```rust
// capture_sck.rs:291 — 使用 std::alloc::alloc 分配
let buf = unsafe {
    std::alloc::alloc(std::alloc::Layout::from_size_align_unchecked(len, 1))
};
```

```rust
// lib.rs:42 — argus_free_buffer 使用 Vec::from_raw_parts 释放
let _ = Vec::from_raw_parts(ptr, len, len);
```

> [!CAUTION]
> `std::alloc::alloc` 分配的内存用 `Vec::from_raw_parts(ptr, len, len)` 释放是 **未定义行为 (UB)**。`Vec` 要求 ptr 来自 `Vec::into_raw_parts`，且 capacity 必须匹配原始分配。这里 alignment=1 而 Vec 的 allocator 在 release 模式下可能使用不同 alignment，导致 **潜在的内存损坏或 SEGFAULT**。

**修复方案**:

```rust
// 应改为 Vec 分配方式，与 argus_free_buffer 对齐
let mut pixel_data = pixels.clone(); // Vec<u8>
let ptr = pixel_data.as_mut_ptr();
let len = pixel_data.len();
std::mem::forget(pixel_data);
```

### 🟡 P1 — 中等：`shrink_to_fit` 不保证 capacity == len

**影响文件**: `crypto.rs`, `pii.rs`, `metrics.rs`

多处使用 `vec.shrink_to_fit()` 后假设 `capacity == len`，但 Rust 文档明确指出 `shrink_to_fit` **不保证**收缩到精确大小。当 `capacity > len` 时，`argus_free_buffer(ptr, len)` 少释放了 `capacity - len` 字节，导致**内存泄漏**。

**修复方案**: 传递 capacity 而非 len，或使用 `into_boxed_slice`:

```rust
let boxed = json.into_boxed_slice();
let len = boxed.len(); // boxed slice 保证 capacity == len
let ptr = Box::into_raw(boxed) as *mut u8;
```

### 🟡 P2 — 中等：Metrics 计数缺失

以下模块调用了 Rust FFI 但**未递增**对应的 metrics 计数器：

| 模块 | 缺失的 `inc_*()` 调用 |
|------|----------------------|
| `capture.rs` | `inc_frames_captured()` — CG 路径未计数 |
| `capture_sck.rs` | `inc_frames_captured()` — SCK 路径未计数 |
| `imaging.rs` | `inc_resizes()` — resize 未计数 |
| `keyframe.rs` | `inc_keyframe_diffs()` — diff/hash 未计数 |
| `shm.rs` | `inc_shm_writes()` — write_frame 未计数 |

结果：`/metrics` 端点中 `rust_frames_captured_total` 等计数器**永远为 0**。

### 🟡 P3 — 中等：Makefile test 目标不完整

```makefile
# 当前 — 只测试 4 个模块
test: rust
    cd $(GO_DIR) && go test ... ./internal/imaging/ ./internal/pipeline/ ./internal/ipc/ ./internal/input/
```

**遗漏**: `./internal/crypto/`、`./internal/metrics/` 的测试未包含在 `make test` 中。

### 🟢 P4 — 低：CGO LDFLAGS 硬编码 Swift rpath

所有 Go FFI 文件硬编码了 Xcode Swift 路径：

```go
#cgo LDFLAGS: ... -Wl,-rpath,/Applications/Xcode.app/.../swift-5.5/macosx
```

不同 macOS/Xcode 版本可能路径不同，应改为动态检测或环境变量。

### 🟢 P5 — 低：`go.mod` 依赖极简

```go
module Argus-compound/go-sensory
go 1.25.7
require (
    golang.org/x/image v0.36.0 // indirect
    golang.org/x/net v0.49.0   // indirect
)
```

项目代码引用了 `encoding/json`、`net/http`、`context` 等标准库，但对于 VLM router（HTTP 客户端）、WebSocket server 等功能，**未见第三方网络/WS 库依赖**。需确认是否全部使用标准库实现。

---

## 三、模块级代码质量矩阵

### 3.1 Rust 模块 (rust-core/src/)

| 模块 | 行数 | 安全性 | 错误处理 | 内存管理 | 评级 |
|------|------|--------|----------|----------|------|
| `lib.rs` | 45 | ✅ | ✅ | 🔴 P0 见上 | B |
| `capture.rs` | 103 | ✅ | ✅ | ✅ Vec 方式 | A |
| `capture_sck.rs` | 364 | ⚠️ 全局状态 | ✅ | 🔴 alloc 不匹配 | C |
| `input.rs` | 355 | ✅ | ✅ | ✅ 无堆分配 | A |
| `imaging.rs` | 169 | ✅ | ✅ | ✅ Vec 方式 | A |
| `keyframe.rs` | 143 | ✅ | ✅ | ✅ 无堆分配 | A |
| `shm.rs` | 316 | ⚠️ unsafe Send/Sync | ✅ | ✅ Box 方式 | B |
| `crypto.rs` | 137 | ✅ | ✅ | 🟡 shrink_to_fit | B |
| `pii.rs` | 267 | ✅ | ✅ | 🟡 shrink_to_fit | B |
| `metrics.rs` | 112 | ✅ | ✅ | 🟡 shrink_to_fit | B |

### 3.2 Go FFI 模块 (go-sensory/internal/)

| 模块 | 文件 | 内存释放 | 错误处理 | 测试 | 评级 |
|------|------|----------|----------|------|------|
| `capture` | `rust_capture.go` | ✅ GoBytes+Free | ✅ | ❌ 无测试 | B |
| `imaging` | `rust_scaler.go` | ✅ defer Free | ✅ | ✅ 5/5 | A |
| `input` | `rust_input.go` | ✅ 无堆分配 | ✅ | ✅ 3/3 | A |
| `ipc` | `rust_shm.go` | ✅ Close() | ✅ | ✅ 3/3 | A |
| `pipeline` | `rust_keyframe.go` | ✅ 无堆分配 | ✅ | ✅ 8/8 | A |
| `pipeline` | `rust_pii.go` | ✅ defer Free | ✅ | ✅ 7/7 | A |
| `crypto` | `rust_crypto.go` | ✅ Free 双缓冲 | ✅ | ✅ 4/4 | A |
| `metrics` | `rust_metrics.go` | ✅ defer Free | ✅ | ✅ | A |

### 3.3 Go 业务模块

| 模块 | 文件数 | 职责 | 关键发现 |
|------|--------|------|----------|
| `agent` | 6 | ReAct 循环 | 无 panic recovery，VLM 超时无兜底 |
| `api` | 12 | HTTP/WS | `dashboard.go` 有 panic，registry 有 panic |
| `vlm` | 8 | 多 VLM 路由 | 支持 Gemini/Ollama/OpenAI，health check 完善 |
| `mcp` | 11 | MCP 服务 | 测试覆盖好，有 macOS 安全分级 |
| `pipeline` | 9 | 帧处理管线 | FFI 调用正确，输入验证充分 |

---

## 四、国际顶级视觉理解执行智能体对比

### 4.1 竞品概览

| 智能体 | 厂商 | 架构模式 | 核心技术 |
|--------|------|----------|----------|
| **CUA / Operator** | OpenAI | Screenshot → GPT-4o → Mouse/KB | 感知-推理-行动循环 |
| **Claude Computer Use** | Anthropic | Agent Loop + Pixel Counting | 截屏分析 → 坐标计算 → 执行 |
| **Project Mariner** | Google | Chrome 扩展 + Gemini VLM | 网页理解 + 多步任务规划 |
| **UI-TARS** | ByteDance | 端到端 VLM + GUI Grounding | 统一动作建模 + System-2 推理 |
| **OmniParser V2** | Microsoft | 图标检测 + OCR + Caption | 纯视觉 UI 解析为结构化 DOM |
| **Agent S3** | Simular AI | 模块化开源框架 | 经验学习 + 代码执行 |

### 4.2 Argus vs 竞品的差距与可借鉴点

#### ✅ Argus 已有的优势

| 优势 | 说明 |
|------|------|
| **原生系统级控制** | 通过 CGEvent/SCK 直接控制 macOS，延迟远低于浏览器方案 |
| **零拷贝 SHM** | 竞品多使用 HTTP 传输截图，Argus 用 SHM 零拷贝 |
| **多 VLM 路由** | 支持 Gemini/Ollama/OpenAI 故障切换，竞品多绑定单一模型 |
| **性能优化** | Rust SIMD 4.2× 加速，竞品 (Python) 无此优化层 |
| **隐私保护** | PII 过滤 + AES-GCM 加密，竞品 (CUA/Mariner) 依赖云端 |

#### 🔧 应从竞品借鉴的能力

| 能力 | 竞品参考 | Argus 当前差距 | 优先级 |
|------|----------|----------------|--------|
| **GUI Grounding** | OmniParser V2 | Argus 仅用 VLM 描述截屏，未做 UI 元素定位 | 🔴 高 |
| **Set-of-Mark 标注** | UI-TARS | `som_drawing.go` 存在但未集成 OmniParser | 🔴 高 |
| **System-2 推理** | UI-TARS | ReAct loop 是 System-1，缺少任务分解/反思 | 🟡 中 |
| **多 Agent 协调** | Claude | 单 Agent 循环，无子任务并行委派 | 🟡 中 |
| **沙箱隔离** | trycua/computer | 直接操作宿主机，无安全沙箱 | 🟡 中 |
| **自我纠错** | CUA + Agent S3 | 验证步骤简单，缺少自动重试策略 | 🟢 低 |
| **跨平台支持** | Open Computer Use | 仅 macOS，无 Linux/Windows | 🟢 低 |

### 4.3 战略建议

1. **短期 (1-2 周)**: 集成 OmniParser V2 做 GUI Grounding，提升 UI 元素定位精度
2. **中期 (2-4 周)**: 引入 System-2 推理 — 任务分解 + 中间验证 + 反思回溯
3. **长期 (1-2 月)**: 多 Agent 架构 — 主 Agent 委派子任务给专用 Agent 并行执行

---

## 五、修复优先级总览

| 优先级 | 问题 | 影响 | 建议 |
|--------|------|------|------|
| 🔴 P0 | `capture_sck.rs` 内存 UB | 运行时崩溃 | 立即修复：改用 Vec 分配 |
| 🟡 P1 | `shrink_to_fit` 内存泄漏 | 长运行泄漏 | 本周修复：改用 `into_boxed_slice` |
| 🟡 P2 | Metrics 计数器为零 | 可观测性缺失 | 本周修复：各模块加 `inc_*()` |
| 🟡 P3 | Makefile test 不完整 | CI 覆盖不全 | 本周修复：加入 crypto/metrics |
| 🟢 P4 | Swift rpath 硬编码 | 跨版本兼容 | 择机修复 |
| 🟢 P5 | go.mod 极简 | 需确认 | 择机确认 |

---

> 审计结论：项目 Go+Rust 混合架构**整体合理**，与 Discord/Cloudflare 等行业实践一致。但存在 1 个严重内存安全 Bug (P0) 需立即修复。与国际顶级视觉智能体相比，Argus 在系统级控制和性能方面有独特优势，但在 GUI Grounding 和高级推理方面仍有差距，建议优先集成 OmniParser V2。
