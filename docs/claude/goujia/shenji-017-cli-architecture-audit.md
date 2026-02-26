> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# CLI 架构审计：实施完成报告

- **日期**: 2026-02-25
- **决策**: 采纳方案 B — Rust CLI + Go Gateway（各司其职）
- **ADR**: `docs/adr/001-rust-cli-go-gateway.md`

---

## 一、决策结论

```
Rust CLI (openacosmi) ──── WebSocket RPC ────→ Go Gateway (acosmi)
       用户交互层                                 服务端业务逻辑
```

否决「Go 调度 + Rust 执行」方案，原因：IPC 开销、Go CLI 仅 35% 完成、Rust 已有完整调度能力、双二进制分发复杂度、双基础设施维护成本。

---

## 二、已完成变更清单

### 2.1 Go CLI 弃用标记
**文件**: `backend/cmd/openacosmi/main.go`

| 变更项 | 内容 |
|--------|------|
| 包注释 | 添加 `DEPRECATED` 标记，引用 ADR |
| 运行时警告 | `PersistentPreRunE` 中输出弃用提示到 stderr |
| 影响 | 任何通过 Go CLI 执行的命令都会先看到弃用警告 |

### 2.2 Gateway 二进制增强
**文件**: `backend/cmd/acosmi/main.go`

| 变更项 | 内容 |
|--------|------|
| CLI flags | 新增 `-port`, `-control-ui-dir`, `-profile`, `-dev`, `-version` |
| 端口优先级 | `--port` flag > 配置文件 > 默认值 (19001) |
| Profile 支持 | `-dev` 和 `-profile` 隔离状态目录 |
| 自包含 | 无需依赖 Go CLI 即可完整启动 Gateway |

### 2.3 构建系统重构
**文件**: `backend/Makefile`

| 旧目标 | 新目标 | 说明 |
|--------|--------|------|
| `make build` → 编译 Go CLI | `make gateway` → 编译 Go Gateway | 默认产物改为 `build/acosmi` |
| `make build-rust` → 编译 Rust | `make cli` → 编译 Rust CLI | 命名更清晰 |
| `make install-rust` → 安装为 `openacosmi-rs` | `make install-cli` → 安装为 `openacosmi` | Rust 成为主 CLI |
| — | `make build-all` | Gateway + Rust CLI 一键编译 |
| — | `make test-all` | Go + Rust 全量测试 |
| — | `make build-go-cli-deprecated` | 旧 Go CLI 保留但带警告 |

### 2.4 架构决策记录 (ADR)
**文件**: `docs/adr/001-rust-cli-go-gateway.md`（新建）

| 章节 | 内容 |
|------|------|
| 背景 | 三套 CLI 对比表（TS / Go / Rust） |
| 决策 | Rust CLI + Go Gateway 职责划分 |
| 否决方案 | Go 调度 + Rust 执行的 6 项劣势分析 |
| 迁移策略 | Go CLI 18 个已实现命令的三类处理方式 |
| 后果 | 正面（单一实现、低维护）与负面（弃用代码） |

### 2.5 Rust CLI 架构文档更新
**文件**: `cli-rust/ARCHITECTURE.md`

| 变更项 | 内容 |
|--------|------|
| 新增章节 | "System Architecture: Rust CLI + Go Gateway" |
| 架构图 | Rust CLI ↔ WebSocket RPC ↔ Go Gateway |
| 定位声明 | 明确 Rust CLI 为唯一 CLI 二进制 |

---

## 三、文件影响矩阵

| 文件 | 操作 | 行数变化 |
|------|------|----------|
| `backend/cmd/openacosmi/main.go` | 修改 | +11 行（弃用注释 + 运行时警告） |
| `backend/cmd/acosmi/main.go` | 重写 | 52 → 84 行（增加 CLI flags） |
| `backend/Makefile` | 重写 | 94 → 89 行（重组构建目标） |
| `docs/adr/001-rust-cli-go-gateway.md` | 新建 | 85 行 |
| `cli-rust/ARCHITECTURE.md` | 修改 | +25 行（新增架构章节） |

---

## 四、后续建议（待执行）

| 优先级 | 事项 | 说明 | 跟踪 |
|--------|------|------|------|
| P0 | Rust CLI 补全剩余 15% | 对齐 TS CLI 100% 命令覆盖 | `renwu/002-cli-followup.md` |
| P1 | CI/CD 流水线更新 | 默认构建产物改为 `acosmi` + Rust `openacosmi` | `renwu/002-cli-followup.md` |
| P1 | 发布包调整 | npm/brew/docker 分发改用 Rust 二进制 | `renwu/002-cli-followup.md` |
| P2 | Go CLI 代码冻结 | 停止合入 `cmd/openacosmi/` 的新 PR | `renwu/002-cli-followup.md` |
| P3 | Go CLI 代码移除 | 经过一个发布周期后可安全删除 `cmd/openacosmi/` | `renwu/002-cli-followup.md` |
