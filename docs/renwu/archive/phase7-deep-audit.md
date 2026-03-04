# Phase 7 深度审计报告 — 辅助模块

> 最后更新：2026-02-15
> 审计方法：逐目录文件清单 + import 链分析 + 隐藏依赖交叉验证
> 基于 `phase4-9-deep-audit.md` Phase 7 项的深度展开

---

## 一、规模总览

### 1.1 Phase 7 TS 代码量真实统计

| 模块 | TS 非测试文件数 | 代码行数 | 全局审计预估 | 差异 |
|------|----------------|----------|-------------|------|
| `src/auto-reply/` | 121 | **22,028** | "210文件" | 首次精确量化 |
| `src/browser/` | 38 | **10,478** | "68文件" | 首次精确量化 |
| `src/memory/` | 28 | **7,001** | "43文件" | 首次精确量化 |
| `src/media-understanding/` | 25 | **3,436** | "21文件" | +4 文件 |
| `src/security/` | 8 | **4,028** | "13文件" | 首次精确量化 |
| `src/media/` | 11 | **1,958** | "19文件" | 首次精确量化 |
| `src/tts/` | 1 | **1,579** | "47KB单文件" | ✅ 准确 |
| `src/markdown/` | 6 | **1,461** | "8文件" | 首次精确量化 |
| `src/link-understanding/` | 6 | **268** | "7文件" | 首次精确量化 |
| **Phase 7 合计** | **~244** | **~52,237** | — | — |

> [!CAUTION]
> Phase 7 合计 **52K 行**，是 Phase 6 的 ~2 倍。7.1 auto-reply（22K 行）\
> 单独就超过整个 Phase 6 的 daemon+plugins+hooks+cron+ACP 代码量总和。

### 1.2 Go 端现状（2026-02-15）

| 目录 | 现有文件 | 状态 |
|------|---------|------|
| `internal/security/` | `external_content.go` (345L) + test | ✅ Phase 6 C2 修复 |
| `pkg/markdown/` | `tables.go` (250L) + test | ✅ Phase 6 C2 修复 |
| `internal/autoreply/` | 不存在 | 待创建 |
| `internal/memory/` | 不存在 | 待创建 |
| `internal/browser/` | 不存在 | 待创建 |
| `internal/media/` | 不存在 | 待创建 |
| `internal/tts/` | 不存在 | 待创建 |
| `internal/linkparse/` | 不存在 | 待创建 |

---

## 二、各子模块文件级清单

### 2.1 auto-reply/ — 121 文件，22,028 行 ⭐最大

**顶层文件（11 个）**：

| 文件 | 行数 | 职责 |
|------|------|------|
| `types.ts` | ~200 | 自动回复核心类型 |
| `chunk.ts` | ~300 | 消息分块策略（mode/limit） |
| `tokens.ts` | ~150 | token 计数工具 |
| `envelope.ts` | ~250 | 入站消息信封构建 |
| `inbound-debounce.ts` | ~200 | 消息防抖合并 |
| `group-activation.ts` | ~200 | 群组激活策略 |
| `model.ts` | ~150 | 模型解析 |
| `send-policy.ts` | ~200 | 发送策略 |
| `status.ts` | ~100 | 自动回复状态 |
| `heartbeat.ts` | ~250 | 心跳管理 |
| `thinking.ts` | ~100 | thinking mode 处理 |

**reply/ 子目录（~40 文件，核心管线）**：

| 文件 | 行数 | 职责 |
|------|------|------|
| `route-reply.ts` | ~400 | 回复路由分发（核心入口） |
| `get-reply-run.ts` | ~350 | 获取并执行回复 |
| `agent-runner-execution.ts` | ~500 | Agent 执行编排 |
| `agent-runner-helpers.ts` | ~300 | Agent 运行辅助 |
| `agent-runner-memory.ts` | ~200 | Agent 记忆管理 |
| `agent-runner-payloads.ts` | ~300 | Agent 载荷构建 |
| `provider-dispatcher.ts` | ~350 | Provider 调度器 |
| `commands.ts` | ~400 | 命令处理 |
| `directives.ts` + `directive-handling*.ts` | ~600 | 指令解析+处理 |
| `mentions.ts` | ~200 | @mention 检测 |
| `history.ts` | ~250 | 群聊历史管理 |
| `typing.ts` | ~100 | 打字状态 |
| `normalize-reply.ts` | ~200 | 回复格式化 |
| `followup-runner.ts` | ~300 | 跟进运行器 |
| 其余 ~25 文件 | ~6,000 | session, model-selection, threading 等 |

---

*(以下子模块将在后续段落补充)*

### 2.2 memory/ — 28 文件，7,001 行

| 文件 | 行数 | 职责 |
|------|------|------|
| `manager.ts` | **2,411** | 主管理器（⚠️ 需拆 5+ Go 文件） |
| `qmd-manager.ts` | **908** | QMD 索引管理（需独立 L2 审计） |
| `batch-gemini.ts` | 431 | Gemini 批量 Embedding |
| `batch-openai.ts` | 398 | OpenAI 批量 Embedding |
| `batch-voyage.ts` | 373 | Voyage 批量 Embedding |
| `internal.ts` | 315 | 内部工具函数 |
| `backend-config.ts` | 299 | 后端配置 |
| `embeddings.ts` | 254 | Embedding 抽象层 |
| `search-manager.ts` | 223 | 搜索管理器 |
| 其余 19 文件 | ~1,389 | sqlite, types, sync, session-files 等 |

### 2.3 browser/ — 38 文件，10,478 行

| 文件 | 行数 | 职责 |
|------|------|------|
| `extension-relay.ts` | **790** | Chrome 扩展中继（CDP） |
| `server-context.ts` | **668** | 标签页/上下文管理 |
| `chrome.executables.ts` | **625** | Chrome 路径发现 |
| `pw-session.ts` | **629** | Playwright 会话管理 |
| `pw-tools-core.interactions.ts` | 546 | 交互操作（点击/输入） |
| `cdp.ts` | 454 | CDP 协议封装 |
| `pw-role-snapshot.ts` | 427 | 角色快照 |
| `chrome.ts` | 342 | Chrome 进程管理 |
| `client.ts` | 337 | 客户端连接 |
| `client-actions-state.ts` | 295 | 客户端状态操作 |
| 其余 28 文件 | ~5,365 | config, constants, screenshots 等 |

### 2.4 media-understanding/ — 25 文件，3,436 行

| 文件 | 行数 | 职责 |
|------|------|------|
| `runner.ts` | ~600 | 媒体理解运行器（核心） |
| `providers/google/video.ts` | ~300 | Google Video 理解 |
| `providers/openai/audio.ts` | ~250 | OpenAI 音频转录 |
| `providers/deepgram/audio.ts` | ~200 | Deepgram 音频转录 |
| `providers/google/audio.ts` | ~200 | Google 音频转录 |
| 其余 20 文件 | ~1,886 | types, format, scope, concurrency 等 |

### 2.5 media/ — 11 文件，1,958 行

| 文件 | 行数 | 职责 |
|------|------|------|
| `fetch.ts` | ~350 | 远程媒体下载 |
| `store.ts` | ~300 | 媒体存储管理 |
| `image-ops.ts` | ~250 | 图片操作（缩放/转换） |
| `audio.ts` | ~200 | 音频处理 |
| `parse.ts` | ~200 | 媒体解析 |
| 其余 6 文件 | ~658 | mime, host, constants, input-files 等 |

### 2.6 security/ — 8 文件，4,028 行（已有 external_content.go 345L）

| 文件 | 行数 | 职责 |
|------|------|------|
| `audit-extra.ts` | **1,305** | 扩展审计规则 |
| `audit.ts` | **992** | 配置安全审计 |
| `skill-scanner.ts` | 441 | 技能文件安全扫描 |
| `fix.ts` | 541 | 安全问题自动修复 |
| `external-content.ts` | 282 | ✅ 已移植 |
| `windows-acl.ts` | 228 | Windows ACL |
| `audit-fs.ts` | 194 | 文件系统权限审计 |
| `channel-metadata.ts` | 45 | 频道元数据安全 |

### 2.7 tts/ — 1 文件，1,579 行

| 文件 | 行数 | 职责 |
|------|------|------|
| `tts.ts` | **1,579** | 完整 TTS 引擎（多 provider + 流式 + 缓存） |

### 2.8 markdown/ — 6 文件，1,461 行（已有 tables.go 250L）

| 文件 | 行数 | 职责 |
|------|------|------|
| `ir.ts` | ~400 | Markdown→IR 中间表示 |
| `render.ts` | ~350 | IR→频道格式渲染 |
| `tables.ts` | ~250 | ✅ 已移植 |
| `fences.ts` | ~200 | 代码围栏解析 |
| `code-spans.ts` | ~150 | 行内代码解析 |
| `frontmatter.ts` | ~111 | Frontmatter 解析 |

### 2.9 link-understanding/ — 6 文件，268 行

| 文件 | 行数 | 职责 |
|------|------|------|
| `runner.ts` | ~80 | 链接理解运行器 |
| `detect.ts` | ~50 | URL 检测 |
| `format.ts` | ~50 | 格式化输出 |
| `apply.ts` | ~40 | 应用理解结果 |
| `defaults.ts` | ~30 | 默认配置 |
| `index.ts` | ~18 | 入口 |

---

## 三、隐藏依赖链分析

### 3.1 auto-reply/ → agents/ 深度耦合 ⭐最关键

```
auto-reply/reply/agent-runner-execution.ts
  → agents/pi-embedded-runner/run.ts (runEmbeddedPiAgent)
  → agents/model-selection.ts (resolveConfiguredModelRef)
  → agents/model-fallback.ts (runWithModelFallback)
  → agents/auth-profiles/ (密钥轮换)
  → agents/skills/ (技能解析)
  → agents/system-prompt.ts (构建 system prompt)
  → routing/session-key.ts (session key)
  → infra/outbound/deliver.ts (投递)
  → security/external-content.ts (prompt 安全)
  → memory/ (memory-tool 集成)
```

> [!CAUTION]
> auto-reply 是 **整个系统的消息响应核心**。它不是辅助模块而是**业务主管线**。
> 每条入站消息最终都通过 auto-reply → agents → outbound 链路处理。

### 3.2 memory/ → 外部 Embedding API 链

```
memory/manager.ts (2,411L)
  → memory/embeddings.ts → embeddings-openai.ts (OpenAI)
  → memory/embeddings.ts → embeddings-gemini.ts (Gemini)
  → memory/embeddings.ts → embeddings-voyage.ts (Voyage)
  → memory/sqlite.ts → sqlite-vec.ts (向量搜索)
  → memory/qmd-manager.ts (908L, QMD 索引)
```

### 3.3 browser/ → playwright-core 深度绑定

```
browser/pw-session.ts → playwright-core (CDP 协议)
browser/pw-tools-core.*.ts → playwright Page/Frame API
browser/extension-relay.ts → Chrome DevTools Protocol
browser/server-context.ts → 标签页管理/上下文隔离
```

> [!WARNING]
> `playwright-core` → Go 替代 `chromedp`/`rod`，API 完全不同，需深度适配。

### 3.4 media-understanding/ → 多 Provider API 链

```
media-understanding/runner.ts
  → providers/openai/ (Whisper + GPT-4V)
  → providers/google/ (Gemini audio/video)
  → providers/anthropic/ (Claude vision)
  → providers/deepgram/ (语音转文字)
  → providers/groq/ (Groq audio)
  → providers/minimax/ (MiniMax)
  → media/ (fetch, store, mime)
```

---

## 四、七类隐式行为审计

| # | 类别 | auto-reply | memory | browser | media | security | tts | markdown |
|---|------|-----------|--------|---------|-------|----------|-----|----------|
| 1 | npm 包黑盒 | ✅ | ⚠️ sqlite-vec | ⚠️ playwright | ⚠️ sharp/canvas | ✅ | ⚠️ provider SDK | ✅ |
| 2 | 全局状态 | ⚠️ debouncer map | ⚠️ manager 单例 | ⚠️ session pool | ✅ | ✅ | ⚠️ 缓存 map | ✅ |
| 3 | 事件总线 | ⚠️ reply dispatch | ⚠️ batch queue | ⚠️ CDP events | ✅ | ✅ | ✅ | ✅ |
| 4 | 环境变量 | ✅ | ⚠️ API keys | ⚠️ CHROME_PATH | ✅ | ✅ | ⚠️ API keys | ✅ |
| 5 | 文件系统 | ⚠️ session 文件 | ⚠️ sqlite 文件 | ⚠️ profile 目录 | ⚠️ 媒体存储 | ⚠️ 权限审计 | ⚠️ 缓存文件 | ✅ |
| 6 | 协议/格式 | ⚠️ 消息格式 | ⚠️ embedding API | ⚠️ CDP 协议 | ⚠️ 多格式解码 | ✅ | ⚠️ 音频格式 | ⚠️ IR 格式 |
| 7 | 错误处理 | ⚠️ 降级策略 | ⚠️ API 重试 | ⚠️ 浏览器崩溃 | ⚠️ 格式兜底 | ⚠️ 修复回退 | ⚠️ provider 降级 | ✅ |

---

## 五、风险评级

### 🔴 高风险项（3 个）

1. **7.1 auto-reply** — 22K 行 + 深度耦合 agents，需 Phase 4 全部完成
2. **7.4 browser** — playwright→chromedp/rod API 不兼容，需大量适配
3. **7.2 memory** — manager.ts 2,411 行单文件拆分 + SQLite 向量搜索

### 🟡 中风险项（2 个）

4. **7.5 media-understanding** — 6 provider API 适配
5. **7.6 TTS** — 1,579 行单文件需拆分 + 多 provider

### 🟢 低风险项（2 个）

6. **7.3 security** — 已有 external_content.go 基础
7. **7.7 markdown+link** — 最小模块，无外部依赖
