---
summary: "Bun 运行时（已弃用）— 当前架构已不再需要 JavaScript 运行时"
read_when:
  - 查看旧版 Bun 相关信息
title: "Bun（已弃用）"
---

> [!WARNING]
> 当前 OpenAcosmi 已迁移至 **Rust CLI + Go Gateway** 架构，**不再需要 Bun 或 Node.js 运行时**。

# Bun（已弃用）

旧版 OpenAcosmi 可选择使用 Bun 作为本地运行时。当前架构已完全迁移至原生编译的 Rust CLI + Go Gateway，无需任何 JavaScript 运行时。

## 当前状态

- Gateway 由 Go 编译的原生二进制文件驱动
- CLI 由 Rust 编译的原生二进制文件驱动
- 前端 UI 构建产物为静态文件
- Bun 和 Node.js 均不再是运行时依赖

## 从源码构建

如需从源码构建，请使用：

```bash
make build        # 构建 Rust CLI + Go Gateway
make ui-build     # 构建前端 UI
```

详见 [安装](/install) 获取完整安装指南。
