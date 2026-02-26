# argus-compound

## "24小时之眼" — 全天候屏幕感知智能体系统

基于 Go+Rust 混合架构构建的**纯视觉基础智能体（PVBA）**系统，具备全天候屏幕感知、深度视觉理解及自主桌面操作能力。

### 架构

| 服务 | 语言 | 职责 |
| --- | --- | --- |
| `rust-core` | Rust | SIMD 图像处理、屏幕捕获 (SCK/CG)、输入注入 (CGEvent)、零拷贝 SHM IPC |
| `go-sensory` | Go | 服务编排、VLM 路由、ReAct Agent、Pipeline 管线、FFI 调用 Rust |
| `web-console` | Next.js | 实时监控 & HITL 控制台（上帝模式） |

> **Go+Rust 混合架构**: CPU 密集型热路径（屏幕捕获、图像缩放、帧差分、SHM）已迁移至 Rust，Go 保留为服务编排层。Rust 通过 C ABI (`libargus_core.dylib`) 导出，Go 通过 CGO FFI 调用。

### 快速开始

```bash
# 1. 构建 Rust + Go（需要 Rust toolchain + Go 1.22+）
make build

# 2. 或者手动构建
cd rust-core && cargo build --release
cd go-sensory && go run cmd/server/main.go

# 3. 启动 Next.js 控制台
cd web-console && npm run dev

# 4. 运行测试 & 基准测试
make test
make bench
```

### 模块结构

```text
rust-core/src/           # Rust 核心库 (libargus_core.dylib)
├── capture.rs           # CG 屏幕捕获
├── capture_sck.rs       # SCK 事件驱动捕获
├── imaging.rs           # SIMD 图像缩放 (fast_image_resize)
├── input.rs             # CGEvent 输入注入
├── keyframe.rs          # 帧差分 + dHash
└── shm.rs               # POSIX 共享内存 IPC

go-sensory/internal/     # Go 服务层
├── agent/               # ReAct 循环 + UI 解析 (SoM)
├── analysis/            # 时序分析
├── api/                 # REST/WebSocket 端点
├── capture/             # 屏幕捕获 (Rust FFI: RustCapturer)
├── imaging/             # 图像缩放 (Rust FFI: RustResizeBGRA)
├── input/               # 输入虚拟化 (Rust FFI: RustInputController)
├── ipc/                 # SHM IPC (Rust FFI: RustShmWriter)
├── memory/              # ChromaDB 向量存储
├── metrics/             # Prometheus 指标
├── pipeline/            # 关键帧提取 (Rust FFI) + PII 过滤
├── skills/              # 技能执行器
└── vlm/                 # VLM 提供者路由
```

### 性能基准 (Apple Silicon arm64)

| 操作 | Go | Rust SIMD | 加速比 |
|------|-----|-----------|--------|
| 图像缩放 1080p→360p | 17.1ms | 4.1ms | **4.2×** |
| 帧差分 1080p | 2.7ms | 0.64ms | **4.2×** |
| SHM 写帧 720p | — | 47µs | **21K fps** |
