> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# CLI 架构迁移全量任务完成汇总

- **日期**: 2026-02-25
- **编号**: shenji-020
- **前置**: `shenji-017-cli-architecture-audit.md` → `shenji-018-cli-gap-analysis.md` → `shenji-019-release-config-audit.md`
- **ADR**: `docs/adr/001-rust-cli-go-gateway.md`
- **任务跟踪**: `renwu/002-cli-followup.md`

---

## 一、项目概要

### 1.1 决策背景

项目存在三套独立 CLI 实现（TS / Go / Rust），均通过 WebSocket RPC 与 Go Gateway 通信。经架构审计评估四个方案：

| 方案 | 描述 | 结论 |
|------|------|------|
| A | Go 调度 + Rust 执行 | **否决** — IPC 开销高、Go CLI 仅 35% 完成、双二进制分发复杂 |
| B | Rust CLI + Go Gateway | **采纳** — 各司其职，单一 CLI 实现 |
| C | Go 全栈，退役 Rust | **否决** — 丢失 52K LOC + 1289 测试 |
| D | 三套共存 | **否决** — 不可持续 |

### 1.2 采纳架构

```
Rust CLI (openacosmi) ──── WebSocket RPC ────→ Go Gateway (acosmi)
     用户交互层                                   服务端业务逻辑
  (5ms 启动, 4.3MB 二进制, 原生 TUI)            (goroutine + channel adapters)
```

### 1.3 迁移范围

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0 | Rust CLI 命令补全（对齐 TS CLI） | ✅ 完成 + 复核通过 |
| P1 | CI/CD 流水线更新 | ✅ 完成 + 复核通过 |
| P1 | 发布包调整（npm/安装脚本） | ✅ 完成，复核待执行 |
| P2 | Go CLI 代码冻结 | ✅ 完成 + 复核通过 |
| P3 | Go CLI 代码移除 | 待执行（需一个发布周期后） |

---

## 二、P0 命令补全 — 变更清单

### 2.1 新增 6 个 crate

| Crate | 子命令 | 子命令数 |
|-------|--------|----------|
| `oa-cmd-gateway` | run, start, stop, status, install, uninstall, call, usage-cost, health, probe, discover | 11 |
| `oa-cmd-logs` | follow, list, show, clear, export | 5 |
| `oa-cmd-memory` | status, index, check, search | 4 |
| `oa-cmd-cron` | status, list, add, edit, remove, enable, disable, runs, run | 9 |
| `oa-cmd-config` | get, set, unset | 3 |
| `oa-cmd-daemon` | status, start, stop, restart, install, uninstall (legacy alias) | 6 |

### 2.2 扩展 2 个 crate

| Crate | 新增子命令 | 新增文件 |
|-------|-----------|---------|
| `oa-cmd-agents` | add, delete, set-identity | `add.rs`, `delete.rs`, `set_identity.rs` |
| `oa-cmd-channels` | login, logout | `login.rs`, `logout.rs` |

### 2.3 CLI 入口集成 (`oa-cli/src/commands.rs`)

- 6 个新 `Commands` enum variant + 对应 `Args` 结构体
- 2 个现有 enum 扩展（`AgentsAction`, `ChannelsAction`）
- 6 个新 dispatch 函数
- JSON flag 传播模式一致（`json || args.json`）

### 2.4 文件清单

| 文件 | 操作 |
|------|------|
| `cli-rust/Cargo.toml` | 修改（workspace members） |
| `cli-rust/crates/oa-cli/Cargo.toml` | 修改（+6 依赖） |
| `cli-rust/crates/oa-cli/src/commands.rs` | 修改（enum + dispatch） |
| `cli-rust/crates/oa-cmd-gateway/Cargo.toml` | 新建 |
| `cli-rust/crates/oa-cmd-gateway/src/lib.rs` | 新建 |
| `cli-rust/crates/oa-cmd-logs/Cargo.toml` | 新建 |
| `cli-rust/crates/oa-cmd-logs/src/lib.rs` | 新建 |
| `cli-rust/crates/oa-cmd-memory/Cargo.toml` | 新建 |
| `cli-rust/crates/oa-cmd-memory/src/lib.rs` | 新建 |
| `cli-rust/crates/oa-cmd-cron/Cargo.toml` | 新建 |
| `cli-rust/crates/oa-cmd-cron/src/lib.rs` | 新建 |
| `cli-rust/crates/oa-cmd-config/Cargo.toml` | 新建 |
| `cli-rust/crates/oa-cmd-config/src/lib.rs` | 新建 |
| `cli-rust/crates/oa-cmd-daemon/Cargo.toml` | 新建 |
| `cli-rust/crates/oa-cmd-daemon/src/lib.rs` | 新建 |
| `cli-rust/crates/oa-cmd-agents/src/add.rs` | 新建 |
| `cli-rust/crates/oa-cmd-agents/src/delete.rs` | 新建 |
| `cli-rust/crates/oa-cmd-agents/src/set_identity.rs` | 新建 |
| `cli-rust/crates/oa-cmd-agents/src/lib.rs` | 修改 |
| `cli-rust/crates/oa-cmd-channels/Cargo.toml` | 修改（+oa-gateway-rpc 依赖） |
| `cli-rust/crates/oa-cmd-channels/src/login.rs` | 新建 |
| `cli-rust/crates/oa-cmd-channels/src/logout.rs` | 新建 |
| `cli-rust/crates/oa-cmd-channels/src/lib.rs` | 修改 |

**小计**: 12 新建文件 + 11 修改文件 = 23 个文件

### 2.5 编译修复

- `oa-cmd-channels` 缺失 `oa-gateway-rpc` 依赖 → 已补充
- `oa-cmd-agents` 3 个文件使用不存在的 `save_config` → 改为 `write_config_file`

---

## 三、P1 CI/CD — 变更清单

### 3.1 文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `.github/workflows/gateway.yml` | 新建 | Go Gateway 构建 + 测试 + lint |
| `.github/workflows/cli-rust.yml` | 新建 | Rust CLI check/clippy/test + 4 平台构建 |
| `.github/workflows/release.yml` | 新建 | 统一发布流水线（tag v* 触发） |
| `Dockerfile.gateway` | 新建 | Go Gateway 多阶段 Docker 镜像 |
| `docker-compose.yml` | 修改 | Gateway 为主服务，legacy 用 profiles 隔离 |

### 3.2 详细配置

**gateway.yml**:
- 触发: `backend/**` 变更
- 构建: `./cmd/acosmi`（非 Go CLI）
- 测试: `go vet` + `go test -race -cover`
- Lint: `golangci-lint`

**cli-rust.yml**:
- 触发: `cli-rust/**` 变更
- Check: `cargo check` + `clippy -D warnings`
- Test: `cargo test --workspace`
- Build: 4 平台目标（x86_64/aarch64 × linux/darwin）
- 缓存: `Swatinem/rust-cache@v2`

**release.yml**:
- 触发: tag push `v*`
- Gateway: 4 平台组合（linux/darwin × amd64/arm64），`CGO_ENABLED=0`
- CLI: 4 Rust 目标
- 发布: `softprops/action-gh-release@v2` + SHA256SUMS.txt

**Dockerfile.gateway**:
- 构建阶段: `golang:1.23-alpine`
- 运行阶段: `alpine:3.20`，非 root 用户 `acosmi`(uid 1000)
- 端口: 19001

**docker-compose.yml**:
- `gateway` 服务 → `Dockerfile.gateway`，端口 19001
- `gateway-legacy` 服务 → 原 Node.js `Dockerfile`，`profiles: ["legacy"]`（默认禁用）

**小计**: 4 新建文件 + 1 修改文件 = 5 个文件

---

## 四、P1 发布包 — 变更清单

### 4.1 方案 A: npm 包作为安装器

| 文件 | 操作 | 说明 |
|------|------|------|
| `scripts/install-rust-binary.mjs` | 新建 | npm postinstall 钩子：检测平台 → 从 GitHub Release 下载 Rust 二进制 |
| `package.json` | 修改 | 添加 `postinstall` 脚本 + `files` 包含 `bin/` 和安装脚本 |
| `openacosmi.mjs` | 修改 | 优先执行 Rust 二进制，失败 fallback 到 TS CLI |

**install-rust-binary.mjs 特性**:
- 平台映射: `darwin-arm64` → `aarch64-apple-darwin` 等
- 环境变量: `OPENACOSMI_SKIP_RUST_BINARY=1`（跳过下载）、`OPENACOSMI_BINARY_MIRROR`（自定义镜像）
- 下载到 `bin/openacosmi`，失败时静默降级（不阻断 npm install）

### 4.2 方案 B: 纯二进制安装脚本

| 文件 | 操作 | 说明 |
|------|------|------|
| `scripts/install-binary.sh` | 新建 | macOS/Linux Bash 安装脚本，直接下载 Rust 二进制 |
| `scripts/install-binary.ps1` | 新建 | Windows PowerShell 安装脚本 |

**install-binary.sh 特性**:
- 用法: `curl -fsSL https://openacosmi.ai/install-binary.sh | bash`
- 安装到 `~/.local/bin`（`--dir` 可配置）
- Flags: `--version`, `--dir`, `--no-onboard`, `--dry-run`, `--help`

### 4.3 共用变更

| 文件 | 操作 | 说明 |
|------|------|------|
| `.github/workflows/release.yml` | 修改 | 添加 `Flatten artifacts` + `Generate checksums`（SHA256SUMS.txt） |
| `docs/install/installer.md` | 修改 | 添加 `install-binary.sh` 文档段落 |

**小计**: 4 新建文件 + 3 修改文件 = 7 个文件

---

## 五、P2 Go CLI 冻结 — 变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `.github/CODEOWNERS` | 新建 | 保护 `backend/cmd/openacosmi/`，需 maintainers 审批 |
| `backend/cmd/openacosmi/main.go` | 修改 | 包注释 `DEPRECATED` 标记 + `PersistentPreRunE` 运行时弃用警告 |
| `backend/Makefile` | 重写 | 默认目标改为 Gateway，新增 `make cli`/`make build-all`/`make test-all` |

**Makefile 目标重组**:

| 旧目标 | 新目标 | 说明 |
|--------|--------|------|
| `make build` → 编译 Go CLI | `make gateway` → 编译 Go Gateway | 默认产物改为 `build/acosmi` |
| `make build-rust` | `make cli` | 编译 Rust CLI |
| `make install-rust` → `openacosmi-rs` | `make install-cli` → `openacosmi` | Rust 成为主 CLI |
| — | `make build-all` | Gateway + Rust CLI 一键编译 |
| — | `make test-all` | Go + Rust 全量测试 |
| — | `make build-go-cli-deprecated` | 旧 Go CLI 保留但带警告 |

**小计**: 1 新建文件 + 2 修改文件 = 3 个文件

---

## 六、全量文件影响矩阵

### 6.1 按操作类型汇总

| 操作 | 文件数 |
|------|--------|
| 新建 | 21 |
| 修改 | 17 |
| 重写 | 2 |
| **合计** | **40** |

### 6.2 完整文件清单

| # | 文件路径 | 操作 | 所属阶段 |
|---|---------|------|----------|
| 1 | `cli-rust/Cargo.toml` | 修改 | P0 |
| 2 | `cli-rust/crates/oa-cli/Cargo.toml` | 修改 | P0 |
| 3 | `cli-rust/crates/oa-cli/src/commands.rs` | 修改 | P0 |
| 4 | `cli-rust/crates/oa-cmd-gateway/Cargo.toml` | 新建 | P0 |
| 5 | `cli-rust/crates/oa-cmd-gateway/src/lib.rs` | 新建 | P0 |
| 6 | `cli-rust/crates/oa-cmd-logs/Cargo.toml` | 新建 | P0 |
| 7 | `cli-rust/crates/oa-cmd-logs/src/lib.rs` | 新建 | P0 |
| 8 | `cli-rust/crates/oa-cmd-memory/Cargo.toml` | 新建 | P0 |
| 9 | `cli-rust/crates/oa-cmd-memory/src/lib.rs` | 新建 | P0 |
| 10 | `cli-rust/crates/oa-cmd-cron/Cargo.toml` | 新建 | P0 |
| 11 | `cli-rust/crates/oa-cmd-cron/src/lib.rs` | 新建 | P0 |
| 12 | `cli-rust/crates/oa-cmd-config/Cargo.toml` | 新建 | P0 |
| 13 | `cli-rust/crates/oa-cmd-config/src/lib.rs` | 新建 | P0 |
| 14 | `cli-rust/crates/oa-cmd-daemon/Cargo.toml` | 新建 | P0 |
| 15 | `cli-rust/crates/oa-cmd-daemon/src/lib.rs` | 新建 | P0 |
| 16 | `cli-rust/crates/oa-cmd-agents/src/add.rs` | 新建 | P0 |
| 17 | `cli-rust/crates/oa-cmd-agents/src/delete.rs` | 新建 | P0 |
| 18 | `cli-rust/crates/oa-cmd-agents/src/set_identity.rs` | 新建 | P0 |
| 19 | `cli-rust/crates/oa-cmd-agents/src/lib.rs` | 修改 | P0 |
| 20 | `cli-rust/crates/oa-cmd-channels/Cargo.toml` | 修改 | P0 |
| 21 | `cli-rust/crates/oa-cmd-channels/src/login.rs` | 新建 | P0 |
| 22 | `cli-rust/crates/oa-cmd-channels/src/logout.rs` | 新建 | P0 |
| 23 | `cli-rust/crates/oa-cmd-channels/src/lib.rs` | 修改 | P0 |
| 24 | `.github/workflows/gateway.yml` | 新建 | P1-CI |
| 25 | `.github/workflows/cli-rust.yml` | 新建 | P1-CI |
| 26 | `.github/workflows/release.yml` | 新建+修改 | P1-CI + P1-发布 |
| 27 | `Dockerfile.gateway` | 新建 | P1-CI |
| 28 | `docker-compose.yml` | 修改 | P1-CI |
| 29 | `scripts/install-rust-binary.mjs` | 新建 | P1-发布 |
| 30 | `package.json` | 修改 | P1-发布 |
| 31 | `openacosmi.mjs` | 修改 | P1-发布 |
| 32 | `scripts/install-binary.sh` | 新建 | P1-发布 |
| 33 | `scripts/install-binary.ps1` | 新建 | P1-发布 |
| 34 | `docs/install/installer.md` | 修改 | P1-发布 |
| 35 | `.github/CODEOWNERS` | 新建 | P2 |
| 36 | `backend/cmd/openacosmi/main.go` | 修改 | P2 |
| 37 | `backend/cmd/acosmi/main.go` | 重写 | P2 |
| 38 | `backend/Makefile` | 重写 | P2 |
| 39 | `docs/adr/001-rust-cli-go-gateway.md` | 新建 | 架构文档 |
| 40 | `cli-rust/ARCHITECTURE.md` | 修改 | 架构文档 |

---

## 七、测试验证结果

### 7.1 编译验证

| 验证项 | 结果 |
|--------|------|
| `cargo check`（Rust 全 workspace） | ✅ PASS |
| `cargo test`（Rust 全 workspace） | ✅ 1305 tests PASS（+16 新测试） |
| Go 语法检查（CI/CD YAML） | ✅ PASS |
| Shell 脚本语法检查（install-binary.sh） | ✅ PASS |
| PowerShell 语法检查（install-binary.ps1） | ✅ PASS |

### 7.2 编译修复记录

| 问题 | 修复 |
|------|------|
| `oa-cmd-channels` 缺失 `oa-gateway-rpc` 依赖 | Cargo.toml 补充依赖 |
| `oa-cmd-agents` 3 个文件调用 `save_config`（不存在） | 改为 `write_config_file` |

---

## 八、复核审计结果汇总

### 8.1 P0 命令接线审计（交叉验证子代理）

| 检查项 | 验证数 | 结果 |
|--------|--------|------|
| Gateway enum → dispatch 映射 | 11/11 | ✅ PASS |
| Daemon enum → dispatch 映射 | 6/6 | ✅ PASS |
| Logs enum → dispatch 映射 | 5/5 | ✅ PASS |
| Memory enum → dispatch 映射 | 4/4 | ✅ PASS |
| Cron enum → dispatch 映射 | 9/9 | ✅ PASS |
| Config enum → dispatch 映射 | 3/3 | ✅ PASS |
| Channels enum → dispatch 映射 | 9/9 | ✅ PASS |
| Agents enum → dispatch 映射 | 4/4 | ✅ PASS |
| Cargo.toml 依赖完整性 | 8 crate | ✅ PASS |
| JSON flag 传播一致性 | 全量 | ✅ PASS |
| 参数类型匹配 | 全量 | ✅ PASS |
| 无遗漏 dispatch arm | 全量 | ✅ PASS |

**合计: 40 个 action variant × 8 命令组 — 全部 PASS，零差异。**

### 8.2 P1 CI/CD + P2 冻结审计（交叉验证子代理）

| 文件 | 检查项 | 结果 |
|------|--------|------|
| `.github/workflows/gateway.yml` | 触发路径、构建 acosmi、测试 + lint | ✅ PASS |
| `.github/workflows/cli-rust.yml` | 触发路径、check/clippy/test、4 平台构建 | ✅ PASS |
| `.github/workflows/release.yml` | tag 触发、双产物构建、创建 Release | ✅ PASS |
| `Dockerfile.gateway` | 多阶段构建、acosmi 入口、非 root 用户 | ✅ PASS |
| `docker-compose.yml` | Gateway 为主、legacy profiles 隔离 | ✅ PASS |
| `.github/CODEOWNERS` | 保护 cmd/openacosmi/ | ✅ PASS |
| `backend/cmd/openacosmi/main.go` | 弃用警告存在 | ✅ PASS |
| `backend/Makefile` | 默认构建 Gateway、deprecated 标记 | ✅ PASS |

**合计: 8 个文件 — 全部 PASS。**

### 8.3 P1 发布包审计

**状态**: 待执行（见剩余事项）

---

## 九、剩余事项

| 优先级 | 事项 | 说明 | 状态 |
|--------|------|------|------|
| P1 | 发布包复核审计 | 7 个文件的交叉代码层复核审计 | 待执行 |
| P3 | Go CLI 代码移除 | 确认 Rust CLI 100% 覆盖 + 无外部依赖引用后删除 `cmd/openacosmi/` | 待一个发布周期后 |
| P3 | Go CLI 代码移除 — 复核审计 | 删除操作的审计 | 待执行 |

---

## 十、关联文档索引

| 文档 | 编号 | 说明 |
|------|------|------|
| `goujia/shenji-017-cli-architecture-audit.md` | shenji-017 | 架构审计：决策 + 初始 5 文件变更 |
| `goujia/shenji-018-cli-gap-analysis.md` | shenji-018 | Rust vs TS CLI 命令覆盖差异报告 |
| `goujia/shenji-019-release-config-audit.md` | shenji-019 | 发布包配置调整方案 |
| `goujia/shenji-020-cli-migration-full-summary.md` | shenji-020 | **本文档** — 全量汇总 |
| `renwu/002-cli-followup.md` | — | 任务跟踪清单 |
| `docs/adr/001-rust-cli-go-gateway.md` | — | 架构决策记录 |
