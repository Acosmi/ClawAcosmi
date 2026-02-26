---
document_type: Audit
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: self
skill5_verified: true
---

# 审计报告: Argus 视觉子智能体架构 — Token 爆炸与性能回归

## 1. 审计范围

| 维度 | 内容 |
|------|------|
| 触发 | 用户报告：简单视觉任务（查看屏幕内容）需 6+ 分钟，上下文 38K+ tokens 被 Gemini API 截断 |
| 崩溃 | `tools_perception.go:562` nil pointer — VLM provider 未配置 |
| 核心文件 | `Argus/go-sensory/internal/mcp/tools_perception.go` (571 行) |
| | `backend/internal/agents/runner/attempt_runner.go` (530+ 行) |
| | `backend/internal/agents/runner/tool_result_truncation.go` (197 行) |
| | `backend/internal/agents/session/history.go` (225 行) |
| | `Argus/go-sensory/internal/vlm/router.go` (62 行) |
| | `Argus/go-sensory/cmd/server/main.go` (468+ 行) |
| 现象 | bodySize: 15,548 → 34,450 → 38,300 (3 次迭代线性增长) |
| | Gemini API: "unexpected EOF" → "context deadline exceeded" |
| | Argus 两次 SIGSEGV 崩溃 + 自动重启 |

## 2. 根因分析

### 2.1 即时崩溃: VLM Provider Nil Pointer (CRITICAL)

**调用链**:
```
main.go:408 → vlmProvider = nil (no env vars)
main.go:430 → PerceptionDeps{VLM: nil}
tools_perception.go:189 → callVLMWithImage(ctx, t.deps.VLM, ...)
tools_perception.go:562 → provider.ChatCompletion(ctx, req) ← SIGSEGV
```

**原因**: VLM 配置仅通过环境变量加载 (`VLM_API_BASE`, `GEMINI_API_KEY`, `OLLAMA_ENDPOINT`)。
若未设置，`vlm/router.go:52` 仅打印 Warning，不返回 error。nil provider 传入 `PerceptionDeps`，
6 个感知工具中 4 个调用 `callVLMWithImage()` 均无 nil 守卫。

**影响**: Argus 进程 panic → 自动重启 → 主智能体重试 → 再次 panic，形成死循环。
日志中可见两次完整 panic 堆栈 + 两次重启。

**受影响工具**:
- `describe_scene` (L189) — 本次崩溃点
- `read_text` (L315)
- `detect_dialog` (L371)
- `watch_for_change` (L502)

### 2.2 架构问题: 主智能体↔子智能体双重 Token 开销

**当前架构**:
```
用户消息 → Gemini (主智能体)
  → tool_call: argus_describe_scene
    → Argus MCP → 截屏 → VLM 分析 → 返回文本描述
  ← tool_result: "屏幕上有X、Y、Z..."
  → Gemini 再次理解文本描述 → 决定下一步
  → tool_call: argus_click(...)
  ← tool_result: "clicked"
  → Gemini 再调用截屏验证...
```

**问题分解**:

| 编号 | 问题 | 影响 |
|------|------|------|
| A-01 | **双重理解**: VLM 已理解截屏内容生成描述，主智能体再次从文字理解同一场景 | 重复 token 消耗 |
| A-02 | **线性累积**: `attempt_runner.go:131-344` 每次迭代追加 2 条完整消息，无滑动窗口 | bodySize 线性增长 |
| A-03 | **全量历史发送**: 每次 LLM 调用发送完整消息数组 `messages` | 第 k 次迭代 = 1+2k 条消息 |
| A-04 | **工具定义膨胀**: 21 个工具 × 每工具 schema → 系统 prompt 8,740 tokens | 固定开销巨大 |
| A-05 | **重试放大**: Argus panic → 主智能体收到 error → 重试 → 同一截屏任务触发 3 次完整请求 | 3 倍 token |
| A-06 | **无视觉上下文复用**: 每次 `describe_scene` 独立截屏+VLM，不缓存前次结果 | 冗余 VLM 调用 |

**实际 token 计算** (从日志):
```
迭代 0: inputTokens=4,080  outputTokens=81    → bodySize=15,548
迭代 1: inputTokens=8,683  outputTokens=65    → bodySize=34,450 (+122%)
迭代 2: inputTokens>10,000 (API 截断)         → bodySize=38,300 (+11%)
                                                → "unexpected EOF" × 3
```

### 2.3 性能问题: 单步延迟分析

| 阶段 | 耗时 | 瓶颈 |
|------|------|------|
| 截屏 + 缩放 | ~100ms | 可接受 |
| VLM 调用 (describe_scene) | N/A (崩溃) | nil provider |
| Gemini 主请求 (迭代 0) | ~12s (23:00:53→23:01:05) | 网络 + 推理 |
| Gemini 主请求 (迭代 1) | ~27s (23:01:11→23:01:37) | bodySize 翻倍 |
| Gemini 主请求 (迭代 2) | >90s (超时) | payload 过大 |
| Argus 重启 | ~1s × 2 | 自动恢复 |
| **端到端** | **>6 分钟** | **API 截断终止** |

## 3. 联网验证记录 (Skill 5)

### 3.1 Anthropic Claude Computer Use — 单模型视觉智能体
- **Query**: "Anthropic Claude Computer Use architecture single model vision agent"
- **Source**: https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool
- **Key finding**: Claude 采用单模型方案——同一 VLM 同时感知截屏和决策动作，无子智能体。提供 `token-efficient-tools` (减少 70% 输出 token) 和 `clear_tool_uses` (自动清理旧工具结果) 两个关键优化。推荐分辨率 XGA 1024x768。
- **Verified date**: 2026-02-25

### 3.2 Microsoft UFO — 双智能体 + Speculative Execution
- **Query**: "Microsoft UFO UI-Focused agent dual agent architecture speculative execution"
- **Source**: https://microsoft.github.io/UFO/ + https://arxiv.org/html/2504.14603v1
- **Key finding**: UFO 证明双智能体架构可高效运作，关键：(1) Shared Blackboard 替代序列化 tool_result, (2) Speculative Multi-Action 批量预测 N 个动作从一次视觉分析，**减少 51% LLM 调用**, (3) UIA 可访问性树为主、视觉为辅的混合输入。
- **Verified date**: 2026-02-25

### 3.3 Browser Use — 单快照保留 + 历史压缩
- **Query**: "browser-use open source agent context management history compression"
- **Source**: https://github.com/browser-use/browser-use (31K+ stars)
- **Key finding**: (1) **单快照保留**: 仅保留当前页面状态，丢弃历史页面截屏; (2) **MessageManager.maybe_compact_messages()**: 将历史动作压缩为摘要 memory 字段; (3) DOM 为主 (5-10K tokens/步)，截屏按需 (auto 模式)。压缩后 ~12,600 tokens vs 未压缩 43,000+ tokens (15 步后)。
- **Verified date**: 2026-02-25

### 3.4 Microsoft OmniParser — 结构化屏幕表示
- **Query**: "Microsoft OmniParser screen parsing structured representation token efficient"
- **Source**: https://github.com/microsoft/OmniParser + https://www.microsoft.com/en-us/research/articles/omniparser-v2
- **Key finding**: 用 YOLO 检测 + BLIP-2 描述将截屏转为结构化文本 (如 "[1] Submit 按钮 at (340,520)")。替代发送原始截屏图像，大幅减少 token。V2 延迟降低 60%，处理 0.6-0.8s。
- **Verified date**: 2026-02-25

### 3.5 Anthropic Context Engineering — Agent 循环最佳实践
- **Query**: "Anthropic context engineering AI agents sliding window compaction"
- **Source**: https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
- **Key finding**: Agent 循环必须实现: (1) 旧工具结果自动清理, (2) Prompt caching (静态部分 90% 成本下降), (3) 服务端压缩超出上下文限制时, (4) 自适应思考深度。核心原则: "永不在对话历史中累积完整工具输出"。
- **Verified date**: 2026-02-25

### 3.6 Apple Ferret-UI — AnyRes + Zoom 模式
- **Query**: "Apple Ferret-UI AnyRes zoom mechanism on-device UI understanding"
- **Source**: https://machinelearning.apple.com/research/ferret-ui-2
- **Key finding**: AnyRes 将截屏分为网格单元，提取全局低分辨率 + 局部高分辨率特征。Zoom 机制: 先低分辨率全局预测 → 裁剪感兴趣区域 → 高分辨率重新分析。减少每步图像 token。
- **Verified date**: 2026-02-25

## 4. 发现项 (Findings)

### F-01 [CRITICAL] VLM Provider Nil 无守卫导致 SIGSEGV

| 属性 | 值 |
|------|-----|
| 位置 | `tools_perception.go:562` `callVLMWithImage()` |
| 风险 | CRITICAL — 进程崩溃，触发重启循环 |
| 影响 | describe_scene / read_text / detect_dialog / watch_for_change 全部受影响 |

**现状**: `callVLMWithImage` 第一行即调用 `provider.ChatCompletion()`，无 nil 检查。
`main.go:408` 在 VLM 未配置时将 nil 传入 `PerceptionDeps.VLM`。

**建议修复**:
```go
// callVLMWithImage 入口处加 nil guard
func callVLMWithImage(ctx context.Context, provider vlm.Provider, ...) (string, error) {
    if provider == nil {
        return "", fmt.Errorf("VLM provider not configured; set VLM_API_KEY or GEMINI_API_KEY")
    }
    // ... existing code
}
```

### F-02 [HIGH] 主智能体 attempt loop 内无历史压缩

| 属性 | 值 |
|------|-----|
| 位置 | `attempt_runner.go:131-344` |
| 风险 | HIGH — 3 次迭代即触发 API 截断 |
| 量化 | 每迭代 +2 条完整消息，bodySize 增长 60-120% |

**现状**: `messages = append(messages, ...)` 在 tool loop 内线性累积。
Session 级 `TruncateByTokenBudget` 仅在 attempt 启动前执行一次，loop 内无裁剪。
`tool_result_truncation.go` 限制单条 30% context，但 N 条累积仍溢出。

**建议修复** (参考 Browser Use `maybe_compact_messages` 模式):
```
在 tool loop 内、每次 LLM 调用前:
1. 计算当前 messages 总 token
2. 若超过 context window 的 70%:
   a. 保留最新 2 条 tool 交互 (完整)
   b. 将更早的 tool 结果替换为摘要 (如 "[工具 argus_describe_scene 返回: 屏幕显示浏览器+终端，已定位目标按钮]")
   c. 或直接丢弃旧的 tool_result 内容，仅保留 "[已执行，结果已处理]"
```

### F-03 [HIGH] 视觉 tool 结果未在迭代间复用

| 属性 | 值 |
|------|-----|
| 位置 | `attempt_runner.go:246-253` (tool result 处理) |
| 风险 | HIGH — 同一场景重复截屏+VLM 调用 |

**现状**: 每次 `argus_describe_scene` 调用独立截屏 + 独立 VLM 推理。
若主智能体在 3 次迭代中调用 3 次 describe_scene (如日志所示)，产生 3 次完整的截屏+VLM 开销。

**建议修复** (参考 Browser Use 单快照保留):
```
实现 VisualContextCache:
- 缓存最近一次截屏结果 (描述文本 + 时间戳)
- TTL 5-10 秒 (屏幕未变化时复用)
- 新 describe_scene 调用先检查缓存
- 若截屏像素差异 < 阈值 → 返回缓存结果
```

### F-04 [HIGH] 双重理解开销 — 子智能体 VLM 分析 + 主智能体文字理解

| 属性 | 值 |
|------|-----|
| 位置 | 架构层面: attempt_runner ↔ Argus MCP |
| 风险 | HIGH — Token 开销翻倍，延迟翻倍 |

**现状**: Argus VLM 分析截屏 → 生成 ~200-500 字文字描述 → 返回给主智能体 → 主智能体再从文字"理解"同一场景。两个模型做同一件事。

**建议修复** (参考 Claude Computer Use 单模型模式 + UFO 混合输入):

**方案 A: 单模型直接视觉** (最优，参考 Claude/CUA):
```
若主模型 (Gemini) 支持多模态:
- 直接将截屏图像作为 message content 发给 Gemini
- 跳过 Argus VLM 文字描述中间层
- 一次推理完成: 看图 → 理解 → 决策 → 动作
```

**方案 B: 结构化表示替代文字描述** (次优，参考 OmniParser + UFO):
```
- Argus 不返回自然语言描述
- 而返回结构化 JSON: [{id:1, type:"button", label:"确认", bbox:[340,520,420,550]}, ...]
- 主智能体从结构化数据直接决策
- Token: ~200 vs ~500-1000 (自然语言描述)
```

### F-05 [MEDIUM] 21 个工具定义膨胀 System Prompt

| 属性 | 值 |
|------|-----|
| 位置 | `attempt_runner.go:510-530` |
| 风险 | MEDIUM — 固定 8,740 token 开销 |

**现状**: 每次 LLM 调用携带全部 21 个工具 schema (内建 + Argus 16 + Remote MCP)。
大部分工具在单次对话中不会用到。

**建议修复** (参考 Anthropic prompt caching + 动态工具选择):
```
1. 静态部分 (system prompt + 工具 schema) 启用 prompt caching
   - Gemini: 使用 cachedContent API
   - 减少重复编码开销
2. 动态工具子集:
   - 根据用户意图仅加载相关工具
   - 视觉任务: 仅 describe_scene + click + type + capture
   - 文本任务: 仅 read_text + run_shell
```

### F-06 [MEDIUM] Argus 崩溃重启循环无熔断

| 属性 | 值 |
|------|-----|
| 位置 | `server.go` Argus bridge 重启逻辑 |
| 风险 | MEDIUM — 无限重试放大 token 开销 |

**现状**: Argus panic → 自动重启 (1s backoff) → 主智能体重试 → 再次 panic。
日志中 2 次完整 panic+restart 循环。每次循环产生一次完整 LLM 调用。

**建议修复**:
```
1. 连续 N 次相同工具崩溃 → 标记该工具为 degraded
2. degraded 工具返回明确错误信息而非重试
3. 主智能体收到 degraded 状态后切换策略 (如 fallback 到纯文本交互)
4. 指数退避: 1s → 2s → 4s → 最终停止 (max 3 次)
```

### F-07 [LOW] describe_scene 固定 "high" detail 分辨率

| 属性 | 值 |
|------|-----|
| 位置 | `tools_perception.go:554` `Detail: "high"` |
| 风险 | LOW — 不必要的高 token 图像 |

**现状**: `callVLMWithImage` 硬编码 `Detail: "high"`。
高 detail 模式图像 token 是 low 模式的 ~4-8 倍。

**建议修复** (参考 Ferret-UI AnyRes + Browser Use detail 级别):
```
- 默认 "low" (XGA 1024x768, ~800 tokens)
- 仅在 locate_element 精确定位时用 "high"
- 或实现 zoom 模式: 低分辨率全局 → 裁剪目标区域高分辨率
```

## 5. 架构改进方案 (基于调研)

### 5.1 短期修复 (1-2 天，解决崩溃和最严重性能问题)

| 优先级 | 修复项 | 预期效果 |
|--------|--------|---------|
| P0 | F-01: callVLMWithImage 加 nil guard | 消除 SIGSEGV 崩溃循环 |
| P0 | F-06: Argus 重启加熔断 | 消除无限重试 token 放大 |
| P1 | F-02: tool loop 内加历史压缩 | bodySize 稳定在 ~15K 不增长 |

### 5.2 中期优化 (1-2 周，架构层面)

| 优先级 | 改进项 | 参考 | 预期效果 |
|--------|--------|------|---------|
| P1 | F-04A: Gemini 直接多模态输入 | Claude Computer Use | 消除双重理解，延迟减半 |
| P1 | F-03: 视觉结果缓存 (TTL 5-10s) | Browser Use 单快照 | 减少 60%+ VLM 调用 |
| P2 | F-05: Prompt caching + 动态工具子集 | Anthropic caching | 固定开销从 8.7K 降至 ~2K token |
| P2 | F-07: 自适应分辨率 | Ferret-UI AnyRes | 图像 token 降 4-8x |

### 5.3 长期架构演进 (参考路径)

**当前**: 主智能体 (Gemini) → MCP tool_call → Argus 子智能体 (VLM) → 文字描述 → 返回

**目标架构** (综合 Claude/UFO/Browser Use 最佳实践):

```
方案 1: 单模型直接视觉 (Claude Computer Use 模式)
  用户消息 + 截屏图像 → Gemini (单次推理) → 动作
  - 优点: 延迟最低，无中间层
  - 条件: Gemini 需支持 function calling + 视觉输入同一请求
  - 适合: Gemini-3-pro 已原生支持多模态

方案 2: 预处理 + 结构化输入 (OmniParser/UFO 模式)
  截屏 → Argus 快速解析 (YOLO/OCR, 无 VLM) → 结构化 JSON → Gemini 决策
  - 优点: 解析 <1s 且 token 极低 (~200)
  - 条件: 需实现 UI 元素检测器
  - 适合: 标准 GUI 操作

方案 3: 混合模式 (推荐)
  默认用方案 2 (快速、低 token)
  遇到非标准 UI (画布、动画、无可访问性标签) → 降级到方案 1
  - 兼顾效率和覆盖率
```

**Token 预算对比**:

| 场景 | 当前架构 | 方案 1 | 方案 2 | 方案 3 |
|------|---------|--------|--------|--------|
| 单步截屏理解 | ~4,000 输入 + VLM 调用 | ~1,800 | ~500 | ~500-1,800 |
| 3 步视觉任务 | ~10,000+ (爆炸) | ~5,400 | ~1,500 | ~1,500-5,400 |
| 10 步复杂任务 | API 截断 | ~18,000 | ~5,000 | ~5,000-18,000 |

## 6. 国际项目架构对比汇总

| 项目 | 架构 | 视觉输入 | 历史管理 | Token 效率 |
|------|------|---------|---------|-----------|
| **Claude Computer Use** | 单模型 | 直接截屏 (XGA) | auto clear + server compaction | token-efficient tools (-70%) |
| **OpenAI CUA/Operator** | 单模型 | 云端截屏 | 内部 CoT | RL 优化 |
| **Google Mariner** | 单模型 | Observe-Plan-Act | Plan 作为压缩上下文 | 原生多模态 |
| **Microsoft UFO** | 双智能体 | UIA 为主 + 选择性视觉 | Shared Blackboard | Speculative Exec (-51% 调用) |
| **OmniParser** | 预处理管线 | YOLO+BLIP→结构化文本 | N/A (管线) | ~200 token/步 |
| **Browser Use** | 单智能体 | DOM 为主 + auto 截屏 | 单快照 + 历史压缩 | 12.6K vs 43K (压缩) |
| **LaVague** | 世界模型+动作引擎 | 选择性视觉 | Short Term Memory | 视觉按需 |
| **Ferret-UI** | 单 VLM | AnyRes + Zoom | N/A (单步) | 低分辨率为主 |
| **当前 Argus** | **双智能体** | **每次全截屏 + VLM** | **无压缩** | **线性爆炸** |

**结论**: 当前架构是所有对比项目中唯一**无历史管理 + 无视觉复用 + 双重理解**的方案。

## 7. 判定

**Verdict: FAIL**

- F-01 CRITICAL: 生产环境 SIGSEGV 崩溃循环
- F-02/F-03/F-04 HIGH: 架构级 token 爆炸，简单任务不可用
- 6 项发现，0 项可接受

**建议**: P0 修复 (F-01, F-06) 立即执行；P1 修复 (F-02, F-03, F-04A) 本周内完成。

## 8. 参考来源

| # | 来源 | URL |
|---|------|-----|
| 1 | Anthropic Computer Use 文档 | https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool |
| 2 | Anthropic Token-Efficient Tool Use | https://docs.claude.com/en/docs/agents-and-tools/tool-use/token-efficient-tool-use |
| 3 | Anthropic Context Engineering | https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents |
| 4 | Microsoft UFO 文档 | https://microsoft.github.io/UFO/ |
| 5 | Microsoft UFO2 论文 | https://arxiv.org/html/2504.14603v1 |
| 6 | Microsoft OmniParser | https://github.com/microsoft/OmniParser |
| 7 | Browser Use | https://github.com/browser-use/browser-use |
| 8 | Apple Ferret-UI 2 | https://machinelearning.apple.com/research/ferret-ui-2 |
| 9 | OpenAI Computer-Using Agent | https://openai.com/index/computer-using-agent/ |
| 10 | Google Project Mariner | https://deepmind.google/models/project-mariner/ |
| 11 | SeeAct (ICML 2024) | https://arxiv.org/abs/2401.01614 |
| 12 | LaVague | https://github.com/lavague-ai/LaVague |
