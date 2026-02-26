# Go+Rust 混合架构改造 — 全量任务清单

> 创建: 2026-02-15 | 架构文档: `docs/jiagou/改造Go+Rust 混合架构.md`

---

## Phase 1: Rust 外设微内核（屏幕捕获 + 输入注入）

### Batch A — Cargo 基建 + CG 屏幕捕获 ✅

- [x] 创建 `rust-core/` Cargo workspace
- [x] 实现 CG 屏幕捕获 (`core-graphics` crate)
- [x] 定义 C ABI 导出函数 (`#[no_mangle] pub extern "C"`)
- [x] 生成 `include/argus_core.h` 头文件
- [x] 编译为 `libargus_core.dylib` (303KB)
- [x] Go 侧 FFI 绑定 (`RustCapturer` 实现 `Capturer` 接口)

### Batch B — CGEvent 输入注入 ✅

- [x] 实现 `input.rs`: `argus_input_click`, `argus_input_mouse_move`
- [x] 实现 `input.rs`: `argus_input_double_click`, `argus_input_scroll`
- [x] 实现 `input.rs`: `argus_input_key_down/up/press`, `argus_input_type_char`
- [x] 实现 `input.rs`: `argus_input_get_mouse_pos`
- [x] Go 侧 FFI 绑定: `RustInputController` 实现 `InputController` 接口
- [x] 集成测试: Go 调 Rust FFI 完整链路 (3/3 tests pass)

> 注: scroll 函数使用 raw CoreGraphics FFI (core-graphics 0.24 不暴露 scroll API)

### Batch C — SCK 升级 ✅

- [x] 引入 `screencapturekit` crate 替代 CG 后端
- [x] 实现事件驱动帧回调 (替代轮询)
- [x] 双后端策略: SCK 优先，CG Fallback

> 注: 窗口级过滤功能延迟，继续由 Go ObjC 实现承载

**Phase 1 里程碑**: Go 通过 Rust FFI 完成屏幕捕获 + 鼠标键盘操作闭环

---

## Phase 2: 图像管线 Rust 化（SIMD 加速）

### Batch A — 图像缩放 ✅

- [x] 引入 `fast_image_resize` crate (v5.5)
- [x] 实现 `imaging.rs`: `argus_image_resize` (BGRA → SIMD 缩放)
- [x] 支持多种缩放算法 (Lanczos3, Bilinear, Nearest)
- [x] C ABI 导出 + 头文件更新 (`argus_image_resize`, `argus_image_calc_fit_size`)
- [x] Go 侧 FFI 绑定: `rust_scaler.go` (`RustResizeBGRA`, `RustCalcFitSize`)
- [x] 性能基准测试: Go `x/image` vs Rust SIMD — Rust 4.2× faster (Phase 4完成)

### Batch B — 关键帧提取 ✅

- [x] 实现 `keyframe.rs`: 帧差分 + dHash 算法
- [x] 实现像素变化率计算 (`argus_keyframe_diff`)
- [x] C ABI 导出: `argus_keyframe_diff`, `argus_keyframe_hash`
- [x] Go FFI 封装: `rust_keyframe.go` (`RustCalcChangeRatio`, `RustFrameHash`)
- [x] 性能基准测试: 关键帧提取延迟对比 — Rust 4.2× faster (Phase 4完成)

**Phase 2 里程碑**: 图像处理性能提升 5x+，Go Pipeline 无缝调用 Rust

---

## Phase 3: SHM IPC Rust 化（零拷贝帧传递） ✅

- [x] 引入 `memmap2` crate
- [x] 实现 `shm.rs`: 共享内存区域创建/写入/读取
- [x] C ABI 导出: `argus_shm_create`, `argus_shm_write_frame`, `argus_shm_destroy`
- [x] 头文件更新
- [x] Go 侧 FFI 绑定: `rust_shm.go` (`RustShmWriter`)
- [x] 跨进程读取验证 (SimulateReaderConsume)

**Phase 3 里程碑**: 跨进程零拷贝帧传递正常工作

---

## Phase 4: 集成验证 + 清理

### Batch A — 全链路验证 ✅

- [x] 集成测试: Resize → Keyframe → Hash → SHM 全链路 (2 tests)
- [x] imaging 单测 (`rust_scaler_test.go`) — 5/5 PASS
- [x] keyframe 单测 (`rust_keyframe_test.go`) — 8/8 PASS
- [x] Rust SHM 测试 (`rust_shm_test.go`) — 3/3 PASS

### Batch B — 性能回归 ✅

- [x] Benchmark 图像缩放: Rust 4.1ms vs Go 17.1ms (4.2× faster)
- [x] Benchmark 帧差分: Rust 0.64ms vs Go 2.7ms (4.2× faster)
- [x] Benchmark SHM 写帧: 47µs/frame (21K fps)

### Batch C — 文档 + CI ✅

- [x] 创建 `Makefile` 统一 Rust/Go 构建
- [x] 更新 `ci.yml` 增加 Rust build + clippy 步骤
- [x] 更新 `README.md` 混合架构文档 + 性能表
- [x] 更新 `phase-task.md` 标记完成
- [x] 更新 `bootstrap-context.md` 进度表

**Phase 4 里程碑**: 全系统在 Go+Rust 混合架构下稳定运行，CI 绿色

---

## GUI Grounding 升级: AX 原生检测 ✅

> 来源: 全局健康度审计 | 完成: 2026-02-15

### Batch A — Rust AXUIElement 模块 ✅

- [x] 新增 `rust-core/src/accessibility.rs` — 3 个 C ABI 函数 (467 行)
  - `argus_ax_list_elements` — 按 PID 枚举 UI 元素
  - `argus_ax_element_at_position` — 坐标处元素查询
  - `argus_ax_focused_app` — 前台 App 元素树
- [x] 更新 `lib.rs` + `argus_core.h`
- [x] 验证: `cargo build --release` + `cargo clippy -- -D warnings`

### Batch B — Go FFI + UIParser 改造 ✅

- [x] 新增 `rust_accessibility.go` — Go FFI 绑定 (127 行)
- [x] 改造 `ui_parser.go` — AX 优先 + VLM fallback 双路检测
- [x] 更新 `main.go` — 注入 RustAccessibility 客户端
- [x] 验证: `go build ./...` + `go vet ./...`

### Batch C — ReAct Loop SoM 集成 ✅

- [x] 改造 `react_loop.go` — 新增 `thinkSoM()` + `thinkDirect()` 双路模式
- [x] 验证: `make test` 全部通过，零回归

**GUI Grounding 里程碑**: UI 元素检测从 VLM (~2s) 切换为 AX 原生 (<5ms), 500× 提速

> 后续增强: 见 `docs/renwu/plan-20260215-gui-grounding-v2.md` (.pkg 封装 + 权限引导 + 混合策略)

---

## 延迟项 (Deferred)

### 已完成 ✅

- [x] PII Filter Rust 化 — `pii.rs` 7 patterns, 7/7 tests PASS, Rust 1.28× faster
- [x] AES-GCM 加密 Rust 化 — `crypto.rs` encrypt/decrypt, 4/4 tests PASS (含 1MB round-trip)
- [x] Prometheus 指标从 Rust 侧采集 — `metrics.rs` 6 atomic counters + JSON export
- [x] `purego` 方案评估 — 结论: 暂不迁移 (CGO 开销 <3%), 详见 `purego-evaluation.md`
- [x] AKS 部署方案 — 分层架构 (macOS 客户端 + AKS 云端), 详见 `aks-deployment.md`

### 待办 (P3)

- [ ] 跨平台 AX 适配 — Windows UIA + Linux AT-SPI2，远期规划
- [ ] .app bundle + .pkg 安装器 — 见 `plan-20260215-gui-grounding-v2.md` Batch A
- [ ] 统一权限检查 — 见 `plan-20260215-gui-grounding-v2.md` Batch B
- [ ] 混合 AX 策略 (Electron + Chrome) — 见 `plan-20260215-gui-grounding-v2.md` Batch C

### 已决议延迟/移出

- [→] **Memory/Browser (D4)** — 记忆系统有独立项目，不在本系统单独开发，生产时集成。详见原计划。
- [→] **MCP 工具层扩展** — 暂挂起，优先生产化。

> **系统定位说明** (2026-02-16): Argus-Sensory 定位为感知+执行服务，为 IDE 及桌面智能体
> 提供"眼睛"和"手"的能力，通过 MCP 协议集成到外部智能体。本系统不独立运行智能体。
