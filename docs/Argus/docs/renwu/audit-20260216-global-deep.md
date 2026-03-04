# Argus-Compound 全局深度审计报告

> 审计日期：2026-02-16
> 审计范围：全项目 Go+Rust 混合架构 — Rust Core (11 files) + Go (78 .go files)
> 审计维度：内存安全、逻辑 Bug、并发安全、错误处理、代码规范合规

---

## 审计摘要

| 级别 | 数量 | 变化 (vs 2/15 审计) |
|------|------|--------------------|
| P0（阻断） | 0 | 🟢 ↓1 (已修复) |
| P1（必修） | 3 | 🟡 新发现 2 项 |
| P2（建议） | 6 | 📊 |

### 编译状态

| 检查项 | 结果 |
|--------|------|
| `cargo build --release` | ✅ 编译通过，零 warning |
| `go build ./...` | ✅ 编译通过，linker dup rpath warnings |
| `go vet ./...` | ✅ 零警告 |

### 前次 P0/P1 修复确认

| 原编号 | 问题 | 状态 |
|--------|------|------|
| P0: `capture_sck.rs` alloc 不匹配 | 内存分配器 UB | ✅ 已修复 → 改用 Vec clone + mem::forget |
| P1: `shrink_to_fit` 内存泄漏 | `crypto.rs/pii.rs/metrics.rs` | ✅ 已修复 → 统一改用 `into_boxed_slice` |
| P2: Metrics 计数器为零 | 各模块缺失 `inc_*()` | ✅ 已修复 → 所有 FFI 函数已添加 metrics 调用 |

---

## 发现列表

### P1: 必修项

| # | 文件 | 问题描述 | 建议修复 |
|---|------|----------|----------|
| BUG-D2-01 | `accessibility.rs:write_json_output` (L377-379) | ✅ **已修复** ~~内存泄漏~~: 改用 `into_boxed_slice` + `Box::into_raw`。 | 2026-02-17 已修复 |
| BUG-D2-02 | `accessibility.rs:ax_get_children` (L247-280) | ✅ **已修复** ~~Use-After-Free 风险~~: 每个子元素 CFRetain，调用方 CFRelease。 | 2026-02-17 已修复 |
| BUG-D2-03 | `router.go:handleChatCompletions` (L104-109) | ✅ **已修复** ~~Body 关闭顺序问题~~: `defer req.Body.Close()` 已移到 `io.ReadAll` 之前。 | 2026-02-17 已修复 |

### P2: 建议项

| # | 文件 | 问题描述 | 建议修复 |
|---|------|----------|----------|
| STYLE-D2-01 | 20 个文件超 300 行 | **文件行数超标**: Go 16 files + Rust 4 files 超过 300 行限制。最严重: `darwin_sck.go` (1029L), `accessibility.rs` (660L), `tools_perception.go` (570L), `approval_gateway.go` (536L)。 | 按功能域拆分。如 `approval_gateway.go` 拆为 `risk_classifier.go` + `approval_gateway.go`；`accessibility.rs` 拆为 `ax_helpers.rs` + `accessibility.rs`。CGO 文件(`darwin_sck.go`)豁免。 |
| STYLE-D2-02 | `dashboard.go:17`, `registry.go:118` | ✅ **已修复** (dashboard.go) ~~启动阶段 panic~~: `dashboard.go` 已改为 `log.Fatalf`。`registry.go` 的 panic 保留（工具注册重复属编程错误，panic 语义合理）。 | 2026-02-17 已修复 (dashboard.go) |
| STYLE-D2-03 | `accessibility.rs:454-455` | ✅ **已修复** ~~Rust 2024 edition 兼容性~~: 已添加 `unsafe {}` 块。 | 2026-02-17 已修复 |
| PERF-D2-01 | `gemini_provider.go:113`, `openai_provider.go:92` | **错误路径 double-read Body**: 在 HTTP 错误时 `io.ReadAll(resp.Body)` 读取全部响应后丢弃 (`_ =`)，但 Body 已通过 `defer resp.Body.Close()` 确保关闭。这不是 bug 但是不必要的 IO。 | 使用 `io.LimitReader` 限制错误响应读取量，如 `io.ReadAll(io.LimitReader(resp.Body, 2048))`。 |
| SEC-D2-01 | `health.go:228` | **API Key 暴露于 URL**: Gemini health check 将 API Key 放在 query string `?key=xxx`。虽然是 Gemini 官方 API 格式，但 URL 查询参数可能被中间件/代理日志记录。 | 确认 Gemini API 是否支持 Header 传递 key；或在日志中过滤包含 key 的 URL。 |
| ARCH-D2-01 | Go CGO files | **Linker 重复 rpath**: 所有 CGO 文件产生 `ld: warning: duplicate -rpath '/usr/lib/swift' ignored` 和 `duplicate libraries: '-largus_core'`。虽不影响功能，但增加构建噪音。 | 统一 LDFLAGS 到单个共享文件或环境变量，避免每个 CGO 文件重复声明。 |

---

## 模块级代码质量矩阵

### Rust Core (rust-core/src/)

| 模块 | 行数 | 安全性 | 错误处理 | 内存管理 | Metrics | 评级 |
|------|------|--------|----------|----------|---------|------|
| `lib.rs` | 46 | ✅ | ✅ | ✅ Vec+boxed | N/A | A |
| `capture.rs` | 103 | ✅ | ✅ | ✅ Vec | ✅ | A |
| `capture_sck.rs` | 362 | ⚠️ 全局状态 | ✅ | ✅ Vec clone | ✅ | B+ |
| `input.rs` | 355 | ✅ | ✅ | ✅ 无堆分配 | N/A | A |
| `imaging.rs` | 170 | ✅ | ✅ | ✅ Vec+forget | ✅ | A |
| `keyframe.rs` | 144 | ✅ | ✅ | ✅ 无堆分配 | ✅ | A |
| `shm.rs` | 317 | ⚠️ unsafe Send/Sync | ✅ | ✅ Box | ✅ | B+ |
| `crypto.rs` | 135 | ✅ | ✅ | ✅ into_boxed_slice | ✅ | A |
| `pii.rs` | 264 | ✅ | ✅ | ✅ into_boxed_slice | ✅ | A |
| `metrics.rs` | 111 | ✅ | ✅ | ✅ into_boxed_slice | N/A | A |
| `accessibility.rs` | 672 | ✅ 已修复 | ✅ | ✅ into_boxed_slice + CFRetain | N/A | A |

### Go 业务模块 (go-sensory/internal/)

| 模块 | 关键文件 | 并发安全 | 错误处理 | 测试 | 评级 |
|------|----------|----------|----------|------|------|
| `agent` | react_loop.go (444L) | ✅ context 传递 | ✅ 优雅降级 | ⚠️ 无单测 | B |
| `api` | ws_server.go, hub.go | ✅ sync.RWMutex | ✅ | ✅ task_handler_test | A |
| `vlm` | router.go, health.go | ✅ goroutine 有 stop 机制 | ✅ 完善 fallback | ⚠️ 无单测 | B+ |
| `mcp` | server.go, registry.go | ✅ mu 保护 writer | ✅ JSON-RPC 错误码 | ✅ 6 test files | A |
| `input` | approval_gateway.go | ✅ sync.RWMutex | ✅ fail-closed | ✅ approval_test | A |
| `capture` | rust_capture.go, darwin_sck.go | ✅ Subscribe/Unsubscribe | ✅ | ⚠️ 无测试 | B |
| `pipeline` | rust_keyframe.go, rust_pii.go | ✅ | ✅ defer Free | ✅ 15+ tests | A |
| `crypto` | rust_crypto.go | ✅ | ✅ Free 双缓冲 | ✅ 4 tests | A |
| `ipc` | rust_shm.go | ✅ | ✅ Close() | ✅ 4 tests | A |
| `metrics` | rust_metrics.go | ✅ | ✅ defer Free | ✅ | A |

---

## 并发安全专项审查

| 组件 | goroutine | 停止机制 | 泄漏风险 |
|------|-----------|----------|----------|
| `HealthChecker.Start()` | 1 ticker goroutine | `stopCh` + `done` chan | ✅ 安全 |
| `HealthChecker.checkAll()` | N goroutine (per provider) | `sync.WaitGroup` | ✅ 安全 |
| `VLM streaming` | 1 goroutine per stream | channel + context | ✅ 安全 |
| `Hub.Register/Unregister` | 无额外 goroutine | `sync.RWMutex` | ✅ 安全 |
| `MCP Server.Run()` | 主 goroutine | `ctx.Done()` + reader close | ✅ 安全 |
| `Router.ReloadProviders()` | 无锁保护 | 依赖调用方序列化 | ⚠️ TOCTOU 风险（仅 CRUD API 触发，低概率） |

---

## 修复优先级总览

| 优先级 | 编号 | 问题 | 影响 | 修复建议 |
|--------|------|------|------|----------|
| ✅ 已修 | BUG-D2-01 | `accessibility.rs` 内存泄漏 | 长运行泄漏 | ✅ 改用 `into_boxed_slice` (2026-02-17) |
| ✅ 已修 | BUG-D2-02 | `accessibility.rs` UAF 风险 | 潜在 SEGFAULT | ✅ CFRetain 子元素 (2026-02-17) |
| ✅ 已修 | BUG-D2-03 | `router.go` body defer 顺序 | ReadAll panic 时 body 泄漏 | ✅ 调整 defer 位置 (2026-02-17) |
| 🟢 P2 | STYLE-D2-01 | 20 文件超 300 行 | 可维护性 | 拆分文件 |
| ✅ 已修 | STYLE-D2-02 | 2 处启动 panic | 生产稳定性 | ✅ dashboard.go 改为 log.Fatalf (2026-02-17) |
| ✅ 已修 | STYLE-D2-03 | Rust 2024 unsafe warning | 兼容性 | ✅ 添加 unsafe 块 (2026-02-17) |
| 🟢 P2 | PERF-D2-01 | 错误路径 double-read | 不必要 IO | LimitReader |
| 🟢 P2 | SEC-D2-01 | Gemini key in URL | 日志泄露 | Header 传递或日志过滤 |
| 🟢 P2 | ARCH-D2-01 | 重复 rpath/LDFLAGS | 构建噪音 | 统一声明 |

---

> **审计结论**：较 2/15 上次审计，原 P0 严重内存 UB 已修复，原 P1 `shrink_to_fit` 泄漏和 Metrics 缺失均已修复。新发现 `accessibility.rs` 模块存在 2 个 P1 级别内存安全问题（泄漏 + UAF 风险），该模块作为新功能（AX UI 元素枚举）聚集了全项目最多的 unsafe 代码。Go 侧整体健康，并发安全机制完善，FFI 内存释放规范。建议优先修复 `accessibility.rs` 的两个 P1 项。
