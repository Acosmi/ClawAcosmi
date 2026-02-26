---
document_type: Tracking
status: Draft
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: true
---

# 项目清理与 Git 初始化执行方案

> 目标：清除编译产物与历史数据 → 代码库纯净化 → 提交 Git → 模拟用户下载 → 验证初始化流程

---

## 1. 现状审计

### 1.1 体积分析

| 类别 | 路径 | 体积 | 占比 |
|------|------|------|------|
| Rust 编译缓存 | `cli-rust/target/` | 6.4 GB | 94% |
| Node.js 依赖 | `Argus/web-console/node_modules/` | 250 MB | 3.7% |
| Next.js 构建 | `Argus/web-console/.next/` | 52 MB | 0.8% |
| 前端依赖 | `ui/node_modules/` | 46 MB | 0.7% |
| Go 编译产物 | `backend/build/` | 43 MB | 0.6% |
| Argus .app | `Argus/build/` | 14 MB | 0.2% |
| **可清理总计** | | **~6.5 GB** | **96%** |
| **清理后预估** | | **~300 MB** | |

### 1.2 项目状态

| 检查项 | 状态 |
|--------|------|
| `.git` 目录 | ❌ 不存在（非 Git 仓库） |
| `.gitignore` | ❌ 不存在 |
| 敏感文件（`.env`/密钥） | ✅ 未发现（仅 `.env.example` 模板） |
| `STARTUP.md` | ✅ 已存在（176 行，版本号需更新） |
| CI 配置 | ✅ `.github/workflows/oa-sandbox-ci.yml` |

### 1.3 组件清单

| 组件 | 语言 | 入口 | 版本要求 |
|------|------|------|----------|
| Gateway | Go | `backend/cmd/acosmi/main.go` | Go 1.25.7+ |
| CLI | Rust | `cli-rust/` (31 crates) | Rust 1.85+ |
| 前端 UI | TypeScript | `ui/` (Vite + Lit) | Node 18+ |
| Argus Rust | Rust | `Argus/rust-core/` | Rust 1.85+ |
| Argus Go | Go | `Argus/go-sensory/` | Go 1.25.7+ |
| Argus Web | TypeScript | `Argus/web-console/` (Next.js) | Node 18+ |
| 移动端 | Swift/Kotlin | `apps/ios/`, `apps/android/` | — |
| Swabble | Swift | `Swabble/` (Swift Package) | — |

---

## 2. 决策清单

> 执行前须逐项确认。标 ★ 为推荐选项。

### 2.1 必删项（无需决策）

| # | 目标 | 体积 | 理由 |
|---|------|------|------|
| M1 | `cli-rust/target/` | 6.4 GB | 编译缓存，`cargo build` 可还原 |
| M2 | `Argus/web-console/.next/` | 52 MB | 构建缓存，`npm run build` 可还原 |
| M3 | `backend/build/` | 43 MB | Go 二进制，`make gateway` 可还原 |
| M4 | `Argus/build/` | 14 MB | .app 包，`make app` 可还原 |
| M5 | `Argus/web-console/node_modules/` | 250 MB | 依赖，`npm install` 可还原 |
| M6 | `ui/node_modules/` | 46 MB | 依赖，`npm install` 可还原 |
| M7 | 18 个 `.DS_Store` | < 1 MB | macOS 系统垃圾 |
| M8 | `docs/skills/general/date-time/body.tmp` | < 1 KB | 临时文件 |

### 2.2 需决策项

| # | 目标 | 体积 | 选项 |
|---|------|------|------|
| D1 | `docs/renwu/` (215 历史任务文件) | 2 MB | ★ 删除 / 保留 |
| D2 | `docs/v2renwu/` (8 旧版规划) | 32 KB | ★ 删除 / 保留 |
| D3 | `docs/claude/` (审计+跟踪+延迟项) | 676 KB | ★ 保留 / 删除 |
| D4 | `docs/skills/` (69 技能定义) | 10 MB | **必须保留** ⚠️ |
| D5 | `docs/zh-CN/` + `ja-JP/` + `.i18n/` | 3.1 MB | 保留 / ★ 删除 |
| D6 | `CLAUDE.md` (项目开发规范) | 8 KB | ★ 保留 / 删除 |
| D7 | `.claude/settings.local.json` | 4 KB | ★ 删除 / 保留 |
| D8 | `.claude/projects/*/memory/` | ~50 KB | ★ 归档后删除 / 直接删 |
| D9 | `Argus/.gemini/` | 8 KB | ★ 删除 / 保留 |
| D10 | `scripts/` (空目录) | 0 | ★ 删除 |
| D11 | 根目录中文散落文档 | ~20 KB | ★ 移入 docs/design/ / 删除 |
| D12 | `docs/前端审计未改.md` | ~10 KB | ★ 删除 / 保留 |

**⚠️ D4 关键警告**：`docs/skills/` 是后端运行时依赖。`LoadSkillEntries()` 在 Gateway 启动时扫描此目录加载 69 个技能定义。**删除会导致技能系统功能缺失。**

**⚠️ D8 注意**：`.claude/projects/*/memory/` 包含以下有价值的架构文档，建议先复制到 `docs/` 再删除：
- `MEMORY.md` — 项目总状态与进度
- `sandbox-architecture.md` — 沙箱架构决策记录
- `skill-store-architecture.md` — 技能商店桥接架构
- `sandbox-verification.md` — Skill 5 验证记录

---

## 3. 执行步骤

### Step 1: Memory 归档（如选择 D8 归档）

```bash
cd /Users/fushihua/Desktop/OpenAcosmi-rust+go

# 将有价值的 memory 文档复制到 docs/
mkdir -p docs/architecture/
cp ~/.claude/projects/-Users-fushihua-Desktop-OpenAcosmi-rust-go/memory/sandbox-architecture.md \
   docs/architecture/
cp ~/.claude/projects/-Users-fushihua-Desktop-OpenAcosmi-rust-go/memory/skill-store-architecture.md \
   docs/architecture/
cp ~/.claude/projects/-Users-fushihua-Desktop-OpenAcosmi-rust-go/memory/sandbox-verification.md \
   docs/architecture/
```

- [ ] 架构文档已归档到 `docs/architecture/`

### Step 2: 清除编译产物（节省 ~6.5 GB）

```bash
cd /Users/fushihua/Desktop/OpenAcosmi-rust+go

rm -rf cli-rust/target/
rm -rf Argus/web-console/.next/
rm -rf backend/build/
rm -rf Argus/build/
rm -rf Argus/web-console/node_modules/
rm -rf ui/node_modules/
```

- [ ] 编译产物已清除
- [ ] 依赖缓存已清除

### Step 3: 清除系统垃圾

```bash
find . -name ".DS_Store" -delete
rm -f docs/skills/general/date-time/body.tmp
```

- [ ] `.DS_Store` 已清除
- [ ] 临时文件已清除

### Step 4: 清除 AI 工具本地配置

```bash
rm -rf .claude/
rm -rf Argus/.gemini/
```

- [ ] `.claude/` 已删除
- [ ] `Argus/.gemini/` 已删除

### Step 5: 清理历史文档（按决策执行）

```bash
# D1: 历史任务记录
rm -rf docs/renwu/

# D2: 旧版规划
rm -rf docs/v2renwu/

# D5: 多语言文档（如决定删除）
# rm -rf docs/zh-CN/
# rm -rf docs/ja-JP/
# rm -rf docs/.i18n/

# D10: 空目录
rmdir scripts/

# D11: 根目录散落文档 → 整理
mkdir -p docs/design/
mv "Rust 沙箱设计方案文档.md" docs/design/ 2>/dev/null
mv "Argus/Rust 重构智能体架构规划.md" docs/design/ 2>/dev/null

# D12: 遗留文档
rm -f "docs/前端审计未改.md"
```

- [ ] 历史文档已按决策处理
- [ ] 散落文档已整理

### Step 6: 创建 `.gitignore`

在项目根目录创建：

```gitignore
# ===== 编译产物 =====
target/
build/
dist/
.next/
*.exe
*.out
*.dylib
*.so
*.o
*.a

# ===== 依赖 =====
node_modules/
vendor/

# ===== Python =====
__pycache__/
*.pyc
*.pyo
*.egg-info/
.venv/
venv/

# ===== IDE =====
.idea/
.vscode/
*.swp
*.swo

# ===== OS =====
.DS_Store
Thumbs.db

# ===== Docker 数据卷 =====
pgdata/
chromadata/

# ===== AI 工具本地配置 =====
.claude/
.gemini/

# ===== 环境与凭证 =====
.env
.env.local
.env.*.local
.env.development
.env.test
.env.production
openacosmi.config.json
*.pem
*.key
*.crt
*.p8
*.p12
*.pfx
*.jks
*.keystore

# ===== 临时文件 =====
*.tmp
*.bak
*.log
```

- [ ] `.gitignore` 已创建

### Step 7: 更新 `STARTUP.md`

修改环境要求部分：

| 字段 | 旧值 | 新值 |
|------|------|------|
| Go 版本 | `Go 1.21+` | `Go 1.25.7+` |
| Rust 版本 | `Rust (stable)` | `Rust 1.85+ (stable)` |
| 新增 | — | `Linux: libseccomp-dev >= 2.5.0（沙箱 seccomp 编译需要）` |

- [ ] `STARTUP.md` 环境要求已更新

### Step 8: 清理验证

```bash
# 体积检查（预期 ~300 MB）
du -sh .

# 文件总数
find . -type f | wc -l

# 敏感文件检查
grep -rl "eyJ" --include="*.json" --include="*.md" . 2>/dev/null
grep -rl "password\|secret\|apikey" --include="*.json" . 2>/dev/null

# 大文件检查（>5MB）
find . -type f -size +5M 2>/dev/null
```

- [ ] 体积在预期范围
- [ ] 无敏感文件
- [ ] 大文件仅为 `docs/skills/official/` 下的字体/schema（预期）

### Step 9: Git 初始化

```bash
cd /Users/fushihua/Desktop/OpenAcosmi-rust+go

git init
git add .
git status | head -50
git commit -m "init: OpenAcosmi polyglot monorepo (Go + Rust + TypeScript)"
```

- [ ] `git init` 成功
- [ ] `git add` 无异常
- [ ] 首次提交成功

---

## 4. 初始化流程测试

> 模拟用户 clone 后首次搭建环境。逐项验证每个组件可独立编译运行。

### 4.1 测试环境准备

```bash
mkdir -p /tmp/test-clone
cp -a /Users/fushihua/Desktop/OpenAcosmi-rust+go /tmp/test-clone/OpenAcosmi
cd /tmp/test-clone/OpenAcosmi
```

### 4.2 Go Gateway

```bash
cd /tmp/test-clone/OpenAcosmi/backend
go mod download
go build -o build/acosmi ./cmd/acosmi/
./build/acosmi --help
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| `go mod download` | 无错误 | [ ] |
| `go build` | 编译成功，生成二进制 | [ ] |
| `--help` 输出 | 显示命令帮助 | [ ] |

### 4.3 Rust CLI

```bash
cd /tmp/test-clone/OpenAcosmi/cli-rust
cargo check
cargo build --release
./target/release/openacosmi --help
./target/release/openacosmi sandbox list
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| `cargo check` | 31 crate 全部通过 | [ ] |
| `cargo build --release` | 编译成功（约 5 分钟） | [ ] |
| `--help` 输出 | 显示命令帮助 | [ ] |
| `sandbox list` | macOS 显示 seatbelt runner | [ ] |

### 4.4 前端 UI

```bash
cd /tmp/test-clone/OpenAcosmi/ui
npm install
npm run build
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| `npm install` | 无 npm ERR | [ ] |
| `npm run build` | Vite 输出 dist/ | [ ] |

### 4.5 Argus 子智能体

```bash
# Rust 核心
cd /tmp/test-clone/OpenAcosmi/Argus/rust-core
cargo build --release

# Go 感知服务
cd /tmp/test-clone/OpenAcosmi/Argus/go-sensory
go mod download
go build -o /tmp/argus-sensory ./cmd/server
/tmp/argus-sensory --help

# Web Console
cd /tmp/test-clone/OpenAcosmi/Argus/web-console
npm install
npm run build
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| Rust FFI `cargo build` | 生成 `libargus_core.dylib` | [ ] |
| Go 感知 `go build` | 编译成功 | [ ] |
| `--help` 输出 | 显示帮助 | [ ] |
| Web Console `npm run build` | Next.js 构建成功 | [ ] |

### 4.6 Docker 构建

```bash
cd /tmp/test-clone/OpenAcosmi
docker build -f Dockerfile.gateway -t openacosmi-gateway:test .
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| 多阶段构建 | 镜像生成成功 | [ ] |
| 镜像大小 | < 50 MB（alpine 精简） | [ ] |

### 4.7 集成启动

```bash
# 终端 1
cd /tmp/test-clone/OpenAcosmi/backend && make gateway-dev
# 预期日志: gateway started on :19001

# 终端 2
cd /tmp/test-clone/OpenAcosmi/ui && npm run dev
# 预期: Vite 启动在 localhost:26222

# 终端 3: 浏览器验证
open http://localhost:26222
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| Gateway 启动 | 无 panic，日志正常 | [ ] |
| UI 开发服务器 | Vite 启动成功 | [ ] |
| 浏览器访问 | 能看到 UI 界面 | [ ] |
| WebSocket 连接 | 前端连上 Gateway | [ ] |

### 4.8 技能系统

```bash
# Gateway 启动日志中观察:
# "skills: loaded N entries from docs/skills/"
# 预期 N >= 69
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| 技能加载 | >= 69 个技能正常加载 | [ ] |
| synced/ 空目录 | 无报错（未配置 nexus-v4 时正常跳过） | [ ] |

### 4.9 测试套件

```bash
cd /tmp/test-clone/OpenAcosmi/backend
make test          # Go 测试
make test-rust     # Rust 测试（需 OA_CLI_BINARY）
```

| 检查点 | 预期结果 | 状态 |
|--------|---------|------|
| Go 测试 | 全部通过 | [ ] |
| Rust 测试 | 125 测试全部通过 | [ ] |

---

## 5. 风险与注意事项

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| `docs/skills/official/` 含 9.8MB 二进制资产（字体/XML/PDF） | Git 仓库偏大 | 长期考虑 Git LFS |
| Rust 首次 release build 约 5 分钟 | 用户体验 | STARTUP.md 中注明 |
| macOS Argus 需 Xcode CLT + 签名证书 | 非 macOS 无法构建 | 跨平台说明 |
| Linux 需 `libseccomp-dev >= 2.5.0` | sandbox 编译失败 | 环境要求文档 |
| `go.mod` 要求 Go 1.25.7 | 较新版本 | 环境要求文档 |
| Dockerfile.gateway 基础镜像 `golang:1.23-alpine` 与 go.mod 1.25.7 不匹配 | Docker 构建可能失败 | 需更新 Dockerfile |

---

## 6. 清理后目录结构预览

```
OpenAcosmi-rust+go/          (~300 MB)
├── .github/workflows/       # CI 配置
├── .gitignore               # ★ 新建
├── CLAUDE.md                # 项目开发规范
├── STARTUP.md               # 启动指南（已更新版本号）
├── Dockerfile*              # 4 个 Docker 定义
├── docker-compose.yml       # 服务编排
│
├── backend/                 # Go Gateway
│   ├── cmd/acosmi/          #   主入口
│   ├── internal/            #   业务逻辑
│   ├── pkg/                 #   公共包
│   ├── go.mod / go.sum      #   依赖锁定
│   └── Makefile             #   构建脚本
│
├── cli-rust/                # Rust CLI (31 crates)
│   ├── crates/              #   workspace 成员
│   ├── Cargo.toml           #   workspace 定义
│   └── Cargo.lock           #   依赖锁定
│
├── ui/                      # 前端 (Vite + Lit)
│   ├── src/
│   └── package.json
│
├── Argus/                   # 视觉子智能体
│   ├── rust-core/           #   Rust FFI 库
│   ├── go-sensory/          #   Go MCP Server
│   ├── web-console/         #   Next.js 控制台
│   ├── .agent/              #   Agent 工作流定义
│   └── Makefile
│
├── apps/                    # 移动端 (iOS/Android/macOS)
├── Swabble/                 # Swift Package
│
├── docs/                    # 文档
│   ├── skills/              #   ★ 运行时技能定义 (必须保留)
│   ├── claude/              #   审计报告 + 跟踪文档
│   ├── architecture/        #   ★ 新建：归档的架构文档
│   ├── design/              #   ★ 新建：整理的设计文档
│   ├── SKILL-*.md           #   Skill 1-5 开发规范
│   └── ...                  #   其他技术文档
│
└── (无 target/ node_modules/ build/ .next/)
```

---

## 7. 执行时间预估

| 阶段 | 步骤 | 预估 |
|------|------|------|
| 准备 | Step 0 决策确认 | 5 min |
| 清理 | Step 1-5 删除 + 整理 | 10 min |
| 配置 | Step 6-7 gitignore + STARTUP.md | 5 min |
| 提交 | Step 8-9 验证 + git init | 5 min |
| 测试 | Step 10 Go + Rust + 前端 + Docker | 25 min |
| **总计** | | **~50 min** |
