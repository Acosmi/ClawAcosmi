# AKS 部署分析

## 当前架构约束

Argus-Compound 高度依赖 macOS 系统框架:

- **CoreGraphics**: 屏幕捕获 (CG backend)
- **ScreenCaptureKit**: 事件驱动捕获 (SCK backend)
- **CGEvent**: 键鼠输入注入
- **AppKit**: 窗口管理

这些框架 **仅在 macOS 上可用**，无法在 Linux/AKS 容器中运行。

## 可部署组件分析

| 组件 | macOS 依赖 | 可容器化 | 说明 |
|------|-----------|---------|------|
| web-console | ❌ | ✅ | Next.js，平台无关 |
| VLM 路由 | ❌ | ✅ | HTTP/gRPC，仅网络 I/O |
| ReAct Agent | ❌ | ✅ | 纯 Go 逻辑 |
| ChromaDB Memory | ❌ | ✅ | 向量数据库客户端 |
| 屏幕捕获 | ✅ CG/SCK | ❌ | macOS 专有 |
| 输入注入 | ✅ CGEvent | ❌ | macOS 专有 |
| SIMD 图像缩放 | ❌ | ✅ | Rust `fast_image_resize` 跨平台 |
| SHM IPC | ❌ | ✅ | POSIX SHM，Linux 兼容 |
| PII Filter | ❌ | ✅ | Rust `regex`，跨平台 |
| AES-GCM | ❌ | ✅ | Rust `aes-gcm`，跨平台 |

## 建议方案: 分层部署

```
┌─────────────────────────────┐
│  AKS (云端)                  │
│  ┌─────────────────────┐    │
│  │ web-console (容器)   │    │
│  │ VLM 路由 (容器)      │    │
│  │ ReAct Agent (容器)   │    │
│  │ ChromaDB (容器)      │    │
│  └─────────────────────┘    │
└─────────────────────────────┘
          ▲ WebSocket/gRPC
          │
┌─────────────────────────────┐
│  macOS 客户端 (本地)         │
│  ┌─────────────────────┐    │
│  │ 屏幕捕获 (SCK/CG)   │    │
│  │ 输入注入 (CGEvent)   │    │
│  │ SHM IPC             │    │
│  └─────────────────────┘    │
└─────────────────────────────┘
```

## Rust dylib 容器化

对于部署到 AKS 的组件，需要交叉编译 Rust:

```bash
# Linux amd64 target
rustup target add x86_64-unknown-linux-gnu
cargo build --release --target x86_64-unknown-linux-gnu

# Dockerfile
FROM debian:bookworm-slim
COPY target/x86_64-unknown-linux-gnu/release/libargus_core.so /usr/lib/
COPY go-sensory /app/go-sensory
WORKDIR /app/go-sensory
CMD ["./server"]
```

> **注意**: 需要在 `Cargo.toml` 中条件编译排除 macOS 专有 crate (`screencapturekit`, `core-graphics`)

## 结论

- **短期**: 不需要 AKS 部署，系统设计为 macOS 本地全栈运行
- **中期**: 可将 web-console + VLM 路由独立容器化
- **长期**: 如需云端部署，采用分层架构 (客户端→云端 gRPC 通道)
