# S2 审计：Phase 4 Agent 引擎（Part A — 根文件）

> 审计日期：2026-02-18 | TS agents/ 根 109 文件 23,154L → Go agents/ 根 1 文件 46L

---

## 关键映射：TS agents/ 根 → Go 子包

| TS 文件群 | TS 行数 | Go 位置 | 状态 |
|-----------|---------|---------|------|
| system-prompt*.ts | ~920 | agents/prompt/ 765L | ✅ |
| model-selection.ts + model-*.ts | ~2,500 | agents/models/ 1,940L | ✅ |
| pi-embedded-subscribe*.ts | ~1,800 | agents/runner/subscribe*.go | ⚠️ |
| compaction.ts | 373 | agents/compaction/ 251L | ⚠️ |
| cli-runner.ts | 362 | agents/runner/ 内含 | ✅ |
| agent-scope.ts | 192 | agents/scope/ 1,125L | ✅ |
| workspace.ts | 305 | agents/workspace/ 452L | ✅ |
| date-time.ts | 191 | agents/datetime/ 220L | ✅ |
| transcript*.ts | ~420 | agents/transcript/ 575L | ✅ |
| failover-error.ts | 234 | agents/helpers/ 内含 | ✅ |
| auth-*.ts | ~650 | agents/auth/ 344L | ⚠️ |

## ❌ 完全缺失（Go 中未找到）

| TS 文件 | 行数 | 功能 | 隐藏依赖 |
|---------|------|------|----------|
| **bash-tools.exec.ts** | 1,630 | Bash 命令执行 | PTY/Docker/审批 |
| **bash-tools.process.ts** | 665 | PTY 进程管理 | pty-keys/send-keys |
| **bash-tools.shared.ts** | 252 | Docker 参数/截断 | sandbox/* |
| **bash-process-registry.ts** | 274 | 进程注册表 | 全局状态 |
| **cli-credentials.ts** | 607 | CLI 凭证管理 | keychain |
| **skills-install.ts** | 571 | 技能安装管线 | brew/npm/go |
| **skills-status.ts** | 316 | 技能状态检查 | 文件系统 |
| **apply-patch.ts** | 503 | 代码补丁应用 | diff 算法 |
| **apply-patch-update.ts** | 199 | 补丁更新 | — |
| **venice-models.ts** | 393 | Venice 模型映射 | API |
| **opencode-zen-models.ts** | 316 | Zen 模型 | API |
| **pty-keys.ts** | 293 | PTY 键映射 | 平台 |
| **subagent-registry.ts** | 430 | 子代理注册表 | 全局状态 |
| **subagent-announce.ts** | 572 | 子代理公告 | queue |
| **cache-trace.ts** | 294 | 缓存追踪 | timer |
| **openacosmi-tools.ts** | 170 | 工具注册 | 类型 |
| **channel-tools.ts** | 121 | 频道工具 | 频道 |
| **pi-tools.policy.ts** | 339 | 工具策略 | 安全 |
| **pi-tools.read.ts** | 302 | 文件读取工具 | FS |
| **pi-tools.schema.ts** | 179 | 工具 schema | — |
| **tool-policy.ts** | 291 | 工具策略 | 安全模型 |
| **tool-display.ts** | 291 | 工具展示 | UI |
| **tool-images.ts** | 223 | 工具图片 | 媒体 |
| **tool-call-id.ts** | 221 | 工具调用 ID | UUID |

> **合计缺失约 ~9,000L** — 占 agents/ 根的 39%

## Phase 4A 评估

**agents/ 根文件真实完成度：~50%**

- ✅ 核心运行循环（runner/）、模型选择、提示词构建
- ❌ **bash-tools 整个子系统 (~2,800L)** 完全缺失
- ❌ **pi-tools 工具注册/策略/schema (~1,300L)** 缺失
- ❌ **skills-install/status (~900L)** 缺失
- ❌ **apply-patch 代码补丁 (~700L)** 缺失

---

## Phase 4B 审计：agents/ 子目录

### 子目录对照总表

| TS 子目录 | 文件/行 | Go 子目录 | 文件/行 | 比率 |
|-----------|---------|-----------|---------|------|
| tools/ | 38/10,106 | tools/ | **0/0** | ❌ 0% |
| pi-embedded-runner/ | 25/5,136 | runner/ | 22/5,592 | ✅ 109% |
| auth-profiles/ | 13/1,939 | auth/ | 1/344 | ⚠️ 18% |
| skills/ | 10/1,375 | skills/ | 1/253 | ⚠️ 18% |
| sandbox/ | 16/1,848 | sandbox/ | **0/0** | ❌ 0% |
| pi-embedded-helpers/ | 9/1,395 | helpers/ | 2/996 | ⚠️ 71% |
| pi-extensions/ | 8/1,019 | — | **0/0** | ❌ 0% |
| schema/ | 2/419 | — | **0/0** | ❌ 0% |
| cli-runner/ | 1/548 | runner/ 内含 | — | ✅ |

### 关键缺失模块详情

**tools/ (10,106L) — 🔴 最大缺口**

TS agents/tools/ 含 38 文件，是 Agent 的工具系统：

- 频道操作工具 (discord/slack/telegram/whatsapp actions)
- 文件操作工具 (read/write/search/glob)
- 代码工具 (apply-patch/edit)
- 消息工具 (message-tool/web-fetch)
- 隐藏依赖：直接 import 各频道 SDK → 循环依赖风险

**sandbox/ (1,848L) — 🔴 完全缺失**

Docker 沙箱系统，含：容器管理/挂载配置/安全策略/网络隔离

**pi-extensions/ (1,019L) — 🟡 缺失**

扩展点系统，允许注册自定义行为

### Phase 4 总评估

**agents/ 总体真实完成度：~35%**

| 分类 | TS 行数 | Go 行数 | 覆盖 |
|------|---------|---------|------|
| 根文件 | 23,154 | 内嵌各子包 | ~50% |
| 子目录 | 23,837 | 15,359 | ~50% |
| tools/ | 10,106 | 0 | 0% |
| sandbox/ | 1,848 | 0 | 0% |
| **合计** | **46,991** | **15,359** | **33%** |

> [!CAUTION]
> **tools/ (10,106L)** 和 **sandbox/ (1,848L)** 完全缺失。
> 这直接影响 Agent 执行任何工具调用的能力。
