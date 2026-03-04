---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/{cmd, pkg, bridge} + cli-rust/ (35 crates + 4 libs)
verdict: Pass with Notes
---

# 审计报告: cmd/pkg/bridge + cli-rust — 启动入口/公共包/FFI桥接/Rust CLI

## 范围

- `backend/cmd/` — 2 entries: `acosmi`(1), `openacosmi`(46 files)
- `backend/pkg/` — 10 subdirs: types(35), i18n, log, markdown, mcpremote, media, polls, retry, utils, contracts
- `backend/bridge/` — 2 subdirs: inference(1), vision(1)
- `cli-rust/crates/` — 35 crates (oa-sandbox, oa-config, oa-infra, oa-cli, oa-gateway-rpc 等)
- `cli-rust/libs/` — 4 libs (nexus-core, openviking, openviking-rs, qdrant-engine)

## 审计发现

### [PASS] 架构: Go CLI 弃用 + Rust CLI 主力 (cmd/openacosmi/main.go)

- **位置**: `main.go:1-7, 31-38`
- **分析**: Go CLI (`openacosmi`) 已标记为 `DEPRECATED`，在 `PersistentPreRunE` 中输出弃用警告，引导用户使用 Rust CLI。Gateway 服务独立保留在 `cmd/acosmi`。架构决策记录在 `docs/adr/001-rust-cli-go-gateway.md`。
- **风险**: None

### [PASS] 正确性: CLI 预处理链 (cmd/openacosmi/main.go)

- **位置**: `main.go:30-101`
- **分析**: `PersistentPreRunE` 实现 6 步预处理链: i18n 初始化 → profile 解析(--dev/--profile) → banner 输出(doctor/completion 跳过) → 全局状态(verbose/yes) → config guard → plugin registry 按需加载。与 TS `preaction.ts` 对齐。
- **风险**: None

### [PASS] 正确性: pkg/types 类型完备性

- **位置**: `pkg/types/` (35 files)
- **分析**: 涵盖所有业务域的类型定义: agent, approvals, auth, browser, channels(discord/telegram/slack/signal/whatsapp/imessage/feishu/dingtalk/wecom/googlechat/msteams), cron, gateway, hooks, media, memory, messages, models, node_host, plugins, queue, sandbox, skills, tools, tts。每个频道有独立类型文件。
- **风险**: None

### [PASS] 架构: FFI Bridge 预留 (bridge/)

- **位置**: `bridge/inference/inference.go`(12L), `bridge/vision/vision.go`(23L)
- **分析**: 两个 FFI bridge 包均为 Phase 10+ 占位: `Available()` 返回 `false`。`vision.go` 包含注释的 cgo 编译标志（`#cgo CFLAGS/LDFLAGS`），预留了 Rust 视觉库的链接路径。架构规划清晰。
- **风险**: None

### [PASS] 安全: Rust Sandbox 跨平台隔离 (oa-sandbox)

- **位置**: `oa-sandbox/src/lib.rs`(88L), `oa-sandbox/src/platform.rs`(485L)
- **分析**: `SandboxRunner` trait 定义标准接口（name/available/run）。平台降级链:
  - **Linux**: Landlock+Seccomp (+Namespaces) → Docker
  - **macOS**: Seatbelt FFI (`sandbox_init_with_parameters`) → Docker
  - **Windows**: Restricted Token + Job Object + AppContainer → Docker
  
  运行时能力检测: user namespace、Landlock ABI 版本、seccomp-BPF、cgroups v2 delegation、macOS 版本(sysctl)、Seatbelt 可用性、AppContainer 支持。`#![allow(unsafe_code)]` 已声明并说明原因。
- **风险**: None (严格的 SAFETY 注释要求)

### [PASS] 正确性: Rust Config 模块对齐 (oa-config)

- **位置**: `oa-config/src/lib.rs`(15L)
- **分析**: 6 个子模块对齐 Go config 包: paths, includes, env_substitution, io, defaults, validation + sessions。与 Go 版本结构一致。
- **风险**: None

### [PASS] 正确性: Rust Crate 结构完整性

- **位置**: `cli-rust/crates/` (35 crates)
- **分析**: 35 个 crate 覆盖完整 CLI 功能: 命令入口(oa-cli, oa-cmd-*)、核心逻辑(oa-agents, oa-channels, oa-config, oa-infra, oa-routing, oa-types, oa-runtime)、专业功能(oa-sandbox, oa-coder, oa-daemon, oa-terminal, oa-gateway-rpc)。4 个 libs: nexus-core, openviking-rs, qdrant-engine。
- **风险**: None

### [WARN] 正确性: Go/Rust 类型同步风险

- **分析**: `pkg/types/` (35 Go files) 和 `oa-types` (Rust crate) 需要保持同步。目前无自动化同步机制（如代码生成或 protobuf 共享 schema）。
- **风险**: Medium
- **建议**: 考虑使用共享 schema（protobuf/JSON Schema）或代码生成工具保证 Go/Rust 类型一致性。

## pkg 其他子包简要审查

- `pkg/i18n` — 多语言支持（zh-CN/en-US）
- `pkg/log` — 结构化日志
- `pkg/markdown` — Markdown 解析/渲染
- `pkg/mcpremote` — MCP 远程桥接
- `pkg/retry` — 重试工具
- `pkg/utils` — 通用工具函数
- `pkg/contracts` — 接口契约
- `pkg/polls`/`pkg/media` — 投票/媒体工具

## 总结

- **总发现**: 8 (7 PASS, 1 WARN, 0 FAIL)
- **阻断问题**: 无
- **结论**: **通过（附注释）** — Go→Rust CLI 迁移路径清晰，Rust sandbox 跨平台隔离设计优秀。主要风险是 Go/Rust 类型同步。
