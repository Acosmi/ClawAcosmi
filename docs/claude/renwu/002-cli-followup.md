> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# CLI 架构迁移 — 后续任务跟踪

- **创建日期**: 2026-02-25
- **前置**: `goujia/shenji-017-cli-architecture-audit.md`
- **全量汇总**: `goujia/shenji-020-cli-migration-full-summary.md`
- **ADR**: `docs/adr/001-rust-cli-go-gateway.md`

---

## P0: Rust CLI 补全剩余 15%

- [x] 审计 Rust CLI 与 TS CLI 命令覆盖差异
- [x] 识别 Rust CLI 缺失命令列表
- [x] 输出差异报告（技能二 第二步）→ `goujia/shenji-018-cli-gap-analysis.md`
- [x] 补全缺失命令（技能二 第四步，用户批准后）
  - [x] 新增 6 个 crate: oa-cmd-gateway, oa-cmd-logs, oa-cmd-memory, oa-cmd-cron, oa-cmd-config, oa-cmd-daemon
  - [x] 扩展 oa-cmd-agents: add, delete, set-identity
  - [x] 扩展 oa-cmd-channels: login, logout
  - [x] 所有新 crate 源文件实现完成（33 个 .rs 文件）
  - [x] 接入 oa-cli 入口：Commands enum + Args 结构体 + dispatch 路由
  - [x] 全量编译通过，1305 测试通过（+16 新测试）
- [x] 复核审计（技能三）— 40 个 action variant 全部 PASS，JSON flag 传播一致

## P1: CI/CD 流水线更新

- [x] 审计现有 CI/CD 配置文件
- [x] 更新 CI 默认构建产物为 `acosmi` + Rust `openacosmi`
  - [x] .github/workflows/gateway.yml — Go Gateway CI
  - [x] .github/workflows/cli-rust.yml — Rust CLI CI
  - [x] .github/workflows/release.yml — 统一发布流水线
  - [x] Dockerfile.gateway — Go Gateway Docker 镜像
  - [x] docker-compose.yml — 更新为 Go Gateway 为主服务
- [x] 输出变更报告
- [x] 执行变更（用户批准后）
- [x] 复核审计 — 8 个文件全部 PASS

## P1: 发布包调整

- [x] 审计现有发布配置（npm/brew/docker）
- [x] 输出改用 Rust 二进制的变更方案 → `goujia/shenji-019-release-config-audit.md`
- [x] 执行变更（方案 A + B 同时实现）
  - [x] **方案 A**: `scripts/install-rust-binary.mjs` — npm postinstall 下载 Rust 二进制
  - [x] **方案 A**: `package.json` — 添加 postinstall 钩子 + files 包含 bin/ 和脚本
  - [x] **方案 A**: `openacosmi.mjs` — 优先执行 Rust 二进制，失败 fallback TS CLI
  - [x] **方案 B**: `scripts/install-binary.sh` — macOS/Linux 纯二进制安装脚本
  - [x] **方案 B**: `scripts/install-binary.ps1` — Windows PowerShell 安装脚本
  - [x] **共用**: `release.yml` — 添加 SHA256SUMS.txt 校验和生成
  - [x] **文档**: `docs/install/installer.md` — 添加 install-binary.sh 文档段落
- [ ] 复核审计

## P2: Go CLI 代码冻结

- [x] 添加 CODEOWNERS / CI 保护规则阻止 `cmd/openacosmi/` 新 PR
- [x] 输出冻结方案
- [x] 执行变更（用户批准后）
  - [x] .github/CODEOWNERS — 保护 backend/cmd/openacosmi/
  - [x] backend/cmd/openacosmi/main.go — 添加弃用警告
  - [x] backend/Makefile — 重构默认构建目标
- [x] 复核审计 — CODEOWNERS、弃用警告、Makefile 全部 PASS

## P3: Go CLI 代码移除（一个发布周期后）

- [ ] 确认 Rust CLI 100% 覆盖
- [ ] 确认无外部依赖引用 Go CLI
- [ ] 删除 `cmd/openacosmi/` 及相关代码
- [ ] 复核审计
