> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Rust CLI 与 TS CLI 命令覆盖差异 — 深度审计报告

- **日期**: 2026-02-25
- **技能**: 技能二 第二步（输出深度审计与差异报告）
- **范围**: TS `src/cli/` vs Rust `cli-rust/crates/oa-cli/src/commands.rs`

---

## 一、总览

| 维度 | TS CLI | Rust CLI | 差距 |
|------|--------|----------|------|
| 顶层命令组 | 51 | 20 | **31 缺失** |
| 子命令总数 | ~200+ | 42 | **~160+ 缺失** |
| 实际有逻辑的命令 | ~200+ | 38（2 个 stub） | — |

---

## 二、Rust CLI 已实现（42 命令）

### 完整实现 (38)
- health, status, sessions, status-all, gateway-status
- channels: list, add, remove, resolve, capabilities, logs, status (7)
- models: list, set, set-image, aliases {list,add,remove}, fallbacks {list,add,remove,clear}, image-fallbacks {list,add,remove,clear} (15)
- agents: list (1)
- sandbox: recreate, explain (2)
- auth, configure, onboard, doctor
- agent (singular - send message)
- dashboard, docs, reset, setup, uninstall, message, completion

### Stub/部分实现 (2)
- auth — 仅 stub，实际向导逻辑未完成
- sandbox list — 返回空列表，Docker 集成未实现

---

## 三、缺失命令分析

### P0 — CLI 核心功能（用户日常操作必需）

| 命令组 | TS 子命令 | 说明 | 建议 |
|--------|-----------|------|------|
| **gateway** | run, start, stop, restart, status, install, uninstall, call, usage-cost, health, probe, discover (12) | CLI 控制 Gateway 生命周期 | 创建 `oa-cmd-gateway` crate |
| **daemon** | status, start, stop, restart, install, uninstall (6) | legacy alias → 复用 gateway 实现 | gateway 的 alias 子命令 |
| **logs** | follow, list, show, clear, export (5) | 查看 Gateway 日志 | 创建 `oa-cmd-logs` crate |
| **memory** | status, index, check, search (4) | Agent 记忆管理 | 创建 `oa-cmd-memory` crate |
| **cron** | status, list, add, edit, rm, enable, disable, runs, run (9) | 定时任务管理 | 创建 `oa-cmd-cron` crate |
| **config** | get, set, unset (3) | 配置文件直接操作 | 创建 `oa-cmd-config` crate |
| **agents** 扩展 | add, set-identity, delete (3) | Rust 仅有 list | 扩展 `oa-cmd-agents` |
| **models** 扩展 | status, scan, auth {login-github-copilot, add, login, paste-token, setup-token, order {get,set,clear}} (10) | 模型状态/扫描/认证 | 扩展 `oa-cmd-models` |
| **message** 扩展 | broadcast, poll, react, reactions, read, edit, delete, pin, unpin, pins, permissions, search, thread, emoji, sticker, discord-admin (16) | Rust 仅有基础 send | 扩展 `oa-cmd-supporting` message |
| **channels** 扩展 | login, logout (2) | Rust 缺少登录/登出 | 扩展 `oa-cmd-channels` |

**P0 小计**: ~70 个子命令

---

### P1 — 管理功能（运维必需）

| 命令组 | TS 子命令 | 说明 | 建议 |
|--------|-----------|------|------|
| **plugins** | list, install, uninstall, enable, disable, info, update (7) | 插件管理 | 创建 `oa-cmd-plugins` crate |
| **skills** | list, info, check, enable, disable, install, uninstall, update (8) | 技能管理 | 创建 `oa-cmd-skills` crate |
| **hooks** | list, create, delete, test, logs (5) | Webhook 钩子 | 创建 `oa-cmd-hooks` crate |
| **directory** | list, search, lookup, add, delete, update (6) | 联系人目录 | 创建 `oa-cmd-directory` crate |
| **security** | check, audit, permissions, allowlist, ban (5) | 安全管理 | 创建 `oa-cmd-security` crate |
| **update** | check, install, current (3) | CLI 更新 | 创建 `oa-cmd-update` crate |
| **sandbox** 扩展 | create, delete, run, eval (4) | Rust 仅有 list/recreate/explain | 扩展 `oa-cmd-sandbox` |

**P1 小计**: ~38 个子命令

---

### P2 — 高级功能（特定场景）

| 命令组 | TS 子命令 | 说明 | 建议 |
|--------|-----------|------|------|
| **browser** | 40+ (status, start, stop, tabs, screenshot, cookies, evaluate...) | 浏览器自动化 | 创建 `oa-cmd-browser` crate（大工程） |
| **nodes** | status, pairing, invoke, notify, canvas, camera, screen, location (~15+) | 节点管理 | 创建 `oa-cmd-nodes` crate |
| **devices** | list, pair, unpair, token {get,refresh,revoke} (6) | 设备管理 | 创建 `oa-cmd-devices` crate |
| **tui** | start, stop, status (3) | TUI 管理 | 创建 `oa-cmd-tui` crate |
| **system** | heartbeat, presence, events (3) | 系统事件 | 创建 `oa-cmd-system` crate |
| **dns** | lookup, resolve, check (3) | DNS 工具 | 可内置到 infra 或 doctor |
| **webhooks** | create, list, delete, test (4) | Webhook 管理 | 可合并到 hooks |
| **pairing** | discover, list, device, account (4) | 配对工具 | 创建 `oa-cmd-pairing` crate |
| **approvals** | list, approve, reject, revoke, pending (5) | 执行审批 | 创建 `oa-cmd-approvals` crate |
| **acp** | Agent Control Protocol 工具 | ACP 协议 | 创建 `oa-cmd-acp` crate |
| **node** | status, info, run (3) | 单节点控制 | 合并到 nodes |

**P2 小计**: ~90+ 个子命令

---

## 四、CI/CD 与发布配置审计

### 4.1 现状

| 配置 | 状态 | 说明 |
|------|------|------|
| GitHub Actions workflows | **缺失** | `.github/workflows/` 为空 |
| CODEOWNERS | **缺失** | 无代码保护规则 |
| Dockerfile | Node.js only | 不包含 Go Gateway 或 Rust CLI |
| docker-compose.yml | Node.js only | openacosmi-gateway 使用 Node.js |
| package.json | npm 发布 | 仅包含 TS 构建产物 |
| render.yaml | Docker 部署 | 指向 Node.js Dockerfile |
| Makefile (backend) | **已更新** | 已改为 Gateway + Rust CLI |
| Homebrew formula | **缺失** | 无 |
| Goreleaser | **缺失** | 无 |

### 4.2 需要创建/更新的文件

| 文件 | 操作 | 内容 |
|------|------|------|
| `.github/workflows/gateway.yml` | 新建 | Go Gateway 构建 + 测试 |
| `.github/workflows/cli-rust.yml` | 新建 | Rust CLI 构建 + 测试 + 跨平台发布 |
| `.github/workflows/release.yml` | 新建 | 统一发布流程 |
| `.github/CODEOWNERS` | 新建 | 保护 `cmd/openacosmi/` |
| `Dockerfile.gateway` | 新建 | Go Gateway Docker 镜像 |
| `docker-compose.yml` | 更新 | 使用 Go Gateway + Rust CLI |
| `package.json` | 更新 | npm postinstall 下载 Rust 二进制 |

---

## 五、建议执行计划

### 阶段 1: P0 核心命令补全（新建 6 crates + 扩展 4 crates）

新建 crate:
1. `oa-cmd-gateway` — gateway 生命周期（12 子命令）
2. `oa-cmd-logs` — 日志查看（5 子命令）
3. `oa-cmd-memory` — 记忆管理（4 子命令）
4. `oa-cmd-cron` — 定时任务（9 子命令）
5. `oa-cmd-config` — 配置操作（3 子命令）
6. `oa-cmd-daemon` — daemon legacy alias

扩展现有 crate:
7. `oa-cmd-agents` — 添加 add/set-identity/delete
8. `oa-cmd-models` — 添加 status/scan/auth
9. `oa-cmd-channels` — 添加 login/logout
10. `oa-cmd-supporting` — 扩展 message 子命令

### 阶段 2: CI/CD + 发布

11. 创建 GitHub Actions workflows
12. 创建 CODEOWNERS
13. 创建 Dockerfile.gateway
14. 更新 docker-compose.yml
15. 更新 npm 发布流程

### 阶段 3: P1 管理命令（新建 6 crates + 扩展 1）

16-22. plugins, skills, hooks, directory, security, update, sandbox 扩展

### 阶段 4: P2 高级功能

23+. browser, nodes, devices, tui, system, dns, webhooks 等
