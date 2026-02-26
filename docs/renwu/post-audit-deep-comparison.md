# 全局审计后修复计划 — 深度对比分析

> 基于 `post-audit-remediation-plan.md`（初稿）与 39 个模块审计报告 + `deferred-items.md` 的逐项交叉验证
> 分析日期：2026-02-22 | 只读模式，不修改任何代码

## 一、统计汇总校正

初稿统计：

| 优先级 | 初稿数据 | 校正后数据 | 差异说明 |
|--------|---------|-----------|---------|
| 🔴 P0 | 1 | **1** | ✅ 一致 — CMD-1 |
| 🟡 P1 | 新5+旧2=7 | **新3+旧2=5** | ⚠️ 见下方详解 |
| 🟢 P2 | 新8+旧5=13 | **新5+旧5=10** | ⚠️ 见下方详解 |
| ⚪ P3 | ≥25 | **≥25** | 大致一致 |

---

## 二、P0 项校验（1 项）

### CMD-1: Sandbox CLI 命令群补全

| 维度 | 初稿 | 审计源 (`global-audit-commands.md`) | 校验结果 |
|------|------|-------------------------------------|---------|
| 优先级 | P0 | P0 | ✅ 一致 |
| 描述 | Go 端缺少 `sandbox explore/explain` | `sandbox*.ts, sessions.ts` 缺失端点命令 ❌ MISSING | ✅ 一致 |
| 修复方案 | 新建 `cmd_sandbox.go` 或扩展 `cmd_agent.go` | 需在 `cmd_agent.go` 增补或补充 `cmd_sandbox.go` | ✅ 一致 |

**结论**：P0 项无误。

---

## 三、P1 项逐项校验（初稿 7 项 → 校正 5 项）

### ✅ MEDIA-2 → P1 确认

| 维度 | 初稿 | 审计源 (`global-audit-media.md`) |
|------|------|----------------------------------|
| 优先级 | P1 | P1 |
| 描述 | `parse.go` 缺少 Markdown Fenced Code 边界检测 | 一致 |
| 方案 | 增加简单状态机检测 ``` 边界 | 一致 |

### ✅ MEDIA-1 → P1 确认

| 维度 | 初稿 | 审计源 |
|------|------|--------|
| 优先级 | P1 | P1 |
| 描述 | PDF 页面渲染转图片完全缺失 | 一致 |
| 方案 | 引入 `mutool`/`pdftocairo` 或 `pdfcpu` 扩展 | 一致 |

### ✅ W1-SEC1 → P1 确认

| 维度 | 初稿 | 审计源 (`deferred-items.md` L81-85) |
|------|------|--------------------------------------|
| 优先级 | P1 | P1 |
| 描述 | JSONC 配置解析失效 | 一致 |
| 备注 | **初稿正确指出这来自 `deferred-items.md`，非 `global-audit-security.md`**（security 审计报告 P0/P1/P2 均为 0） |

### ✅ W1-TTS1 → P1 确认

| 维度 | 初稿 | 审计源 (`deferred-items.md` L75-79) |
|------|------|--------------------------------------|
| 优先级 | P1 | P1 |
| 描述 | 长文本 TTS 缺少 LLM 智能摘要 | 一致 |
| 备注 | 初稿同时在 W-FIX-5 中列了 TTS-2（同一问题），**存在重复计数** |

### ✅ CMD-2 → P1 确认

| 维度 | 初稿 | 审计源 (`global-audit-commands.md` L63) |
|------|------|------------------------------------------|
| 优先级 | P1 | P1 |
| 描述 | OAuth CLI Web Flow 缺失 | 一致 |

### ⚠️ GW-3 → 初稿标 P1，但审计影响有限

| 维度 | 初稿 | 审计源 (`global-audit-gateway.md` L84) |
|------|------|------------------------------------------|
| 优先级 | P1 | P1 |
| 描述 | WS Close Codes 可能不一致 | 一致 |
| 备注 | 审计源措辞 "是否完全一致" 带不确定性。**建议先验证是否真实存在差异再定级**。若验证后一致则可降为 P3 |

### ⚠️ CMD-5 → 初稿标 P1，建议维持

| 维度 | 初稿 | 审计源 (`global-audit-commands.md` L66) |
|------|------|------------------------------------------|
| 优先级 | P1 | P1 |
| 描述 | `OPENACOSMI_STATE_DIR` 多级推断路径对齐 | 一致 |
| 备注 | 修复方案为"编写单元测试验证"，工作量 ~1h。可与 W-FIX-4 合并 |

### ❌ 初稿重复计数问题

> [!WARNING]
> **W1-TTS1 和 TTS-2 是同一个问题**，初稿分别在 W-FIX-3（P1）和 W-FIX-5（P2）中重复列出。
> 应合并为一项，保留 P1 优先级在 W-FIX-3 中处理，从 W-FIX-5 移除。

---

**P1 校正结论**：实际有效 P1 为 **5 项**（MEDIA-2, MEDIA-1, W1-SEC1, W1-TTS1, CMD-2），另有 2 项（GW-3, CMD-5）需验证后确认。

---

## 四、P2 项逐项校验（初稿 13 项 → 校正 10 项）

### 初稿 P2 确认项（与审计源一致）

| ID | 来源 | 校验 | 说明 |
|----|------|------|------|
| TTS-1 | `global-audit-tts.md` | ✅ | 硬编码 OpenAI 端点，不支持自定义 |
| LOG-1 | `global-audit-log.md` | ✅ | `--follow` 仅限本地文件轮询 |
| MEDIA-3 | `global-audit-media.md` | ✅ | 下载不支持自定义 Headers |
| BRW-2 | `global-audit-browser.md` | ✅ | `pw-ai` 视觉/AI 推理缺失 |
| MEM-2 | `global-audit-memory.md` | ✅ | node-llama→Ollama 接口确认 |
| GW-1 | `global-audit-gateway.md` | ✅ | Control UI 面板缺失 |

### 初稿 P2 来自 `deferred-items.md` 的项

| ID | 初稿描述 | `deferred-items.md` 校验 | 说明 |
|----|---------|-------------------------|------|
| W2-D2 / PHASE5-1 | Gemini SSE 分块 | ✅ L113-119 | 一致 |
| PHASE5-2 | `/v1/responses` 完整实现 | ✅ L386-390 | 一致，~12h |
| HEALTH-D4 | 图片工具缩放 | ✅ L267-270 | 一致 |
| PLG-1 | Hook 生命周期等效触发 | ✅ 来自 `global-audit-plugins.md` | 一致 |
| PLG-2 | C++ ABI → WASM/RPC 替代 | ✅ 来自 `global-audit-plugins.md` | 一致 |

### ❌ 初稿 P2 需要修正的项

**1. TTS-2 重复计数（应删除）**

TTS-2 与 W1-TTS1 完全相同（超长文本 LLM 摘要缺失），初稿标注"与 W1-TTS1 重叠，合并处理"但仍单独计数。**应从 P2 总数中删除。**

**2. TUI-1 和 TUI-2 来源核对**

> [!IMPORTANT]
> `global-audit-tui.md` 的差异清单中 TUI-1 和 TUI-2 均为 P3（架构重构差异），不是 P2。
> 初稿中的 TUI-1（~16h）和 TUI-2（~2h）实际来源是 `deferred-items.md` L340-354。
> 两份文档中 **TUI-1/TUI-2 的 ID 编号相同但含义完全不同**：
>
> | ID | `global-audit-tui.md` | `deferred-items.md` |
> |----|----------------------|---------------------|
> | TUI-1 | 组件架构 DOM→Elm 重构（P3，无需修复） | TUI 核心组件功能不完整（P2，需攻坚） |
> | TUI-2 | WS 回调→通道驱动（P3，无需修复） | 外部依赖细节遗漏（P2，粘合代码） |
>
> **建议**：为避免混淆，将 deferred-items 中的项重命名为 TUI-D1、TUI-D2（或保留当前命名但在修复计划中注明来源为 deferred-items 而非审计报告）。

---

**P2 校正结论**：删除重复的 TTS-2 后，有效 P2 为 **10 项**。TUI 的 2 项需要特别注意来源标注。

---

## 五、初稿遗漏项分析

以下是通过交叉审计发现的 **初稿未收录但审计源中存在的待办项**：

### 5.1 infra 审计未完成（高风险缺口）

> [!CAUTION]
> `global-audit-infra.md` 的"隐藏依赖审计"和"差异清单"两个关键章节标注为 **(待审计)**。
> 审计评级也标注为 **待定**。初稿将 infra 列为"未完成"是正确的，但未详拆其风险。

**已知 infra 缺失**（从逐文件对照中可确认）：

| 项 | 说明 | 建议优先级 |
|----|------|-----------|
| OTA 升级机制 | `update-*.ts` ❌ MISSING | P3（初稿已收录于 W-OPT-A） |
| 隐藏依赖未审计 | 84 个 TS 文件中的 npm/全局状态/环境变量依赖不明 | **P1 — 应优先完成审计** |

**建议**：在执行 W-FIX 序列前，先补完 infra 的隐藏依赖审计（预估 ~2h），避免修复过程遗漏关键依赖。

### 5.2 commands 中其他 MISSING 项未收录

`global-audit-commands.md` 标注了以下 ❌ MISSING：

| 项 | 初稿是否收录 | 说明 |
|----|-------------|------|
| `dashboard.ts`, `message.ts` | ❌ 未收录 | 缺失端点命令 |
| `sessions.ts` | ✅ 已收录于 CMD-1 | sandbox 一并处理 |

**建议**：`dashboard` 和 `message` 的缺失命令应评估是否需要纳入修复计划（至少标记为 P3）。

### 5.3 Cron 已知 Bug 未记录

`deferred-items.md` L169-171 记录了一个 **已知的 normalize 缺陷**：

> `NormalizeCronPayload()` 对 `kind` 字段执行 `strings.ToLower()`，导致 `"systemEvent"` → `"systemevent"` 无法匹配常量。

此 bug 虽然当前不影响正常功能（因调用方通常不直传 `kind`），但存在潜在风险。**初稿未将其纳入修复计划**。

**建议**：追加为 P3 项（~0.5h 修复，移除 `ToLower` 或改常量为小写）。

### 5.4 HEALTH-D1 安全缺口遗留

`deferred-items.md` L252-255 标注 HEALTH-D1（Skills 安装）虽已完成，但注意事项中存在 3 个未修的安全隐患：

1. `download` 安装无 SSRF 防护
2. `scanDirectoryWithSummary` 安全扫描未移植
3. `resolveBrewExecutable()` macOS 路径未移植

这些不在初稿的任何窗口中。**建议**：追加 SSRF 防护至 P2（与 security 模块关联），其余为 P3。

### 5.5 Gateway 审计报告重复总结段

`global-audit-gateway.md` 包含两个"总结"段（L86-92 和 L94-99），第二个标注"待定"。这是文档质量问题，不影响修复计划但应清理。

---

## 六、审计评级交叉校验

初稿的评级分布与各审计报告对照：

| 评级 | 初稿列出的模块 | 校验结果 |
|------|--------------|---------|
| **S** | autoreply, cli, config | ✅ 与审计报告一致 |
| **A** | 33 个模块 | ⚠️ 大部分一致，但 `infra` 未评级（待定），不应出现在 A 列表中 |
| **B** | commands | ✅ 一致 |
| **未完成** | infra | ✅ 一致 |

**校正**：初稿 A 级列表中不含 infra（正确），但 `autoreply` 审计报告中逐文件对照标注为"(待审计)"，实际仅完成了隐藏依赖审计和差异清单。虽然评级为 S，但严格来说逐文件对照的完成度有待商榷。

---

## 七、修正后行动摘要

### 初稿修改清单

| # | 修改项 | 类型 |
|---|--------|------|
| 1 | 删除 W-FIX-5 中的 TTS-2（与 W1-TTS1 重复） | 去重 |
| 2 | P1 总数从 7 改为 5（去重后），P2 总数从 13 改为 10 | 数据修正 |
| 3 | TUI-1/TUI-2 注明来源为 `deferred-items.md` 而非审计报告 | 来源标注 |
| 4 | 新增 infra 隐藏依赖审计补全窗口（W-FIX-0，P1） | 新增项 |
| 5 | 新增 SSRF 防护项（Skills 安装的 download 模式） | 新增 P2 |
| 6 | 新增 Cron normalize bug 为 P3 项 | 新增 P3 |
| 7 | 新增 dashboard/message CLI 缺失评估为 P3 | 新增 P3 |
| 8 | GW-3（WS Close Codes）标注"需先验证再定级" | 优化 |

### 建议修正后的执行顺序

```
W-FIX-0  → infra 隐藏依赖审计补全（新增，~2h）
W-FIX-1  → P0 Sandbox CLI（不变）
W-FIX-2  → P1 Media 缺陷（不变）
W-FIX-3  → P1 Security+TTS+OAuth（TTS-2 已合并入此窗口）
W-FIX-4  → P1 Gateway WS+Env（不变，可合并 W-FIX-1）
W-FIX-5  → P2 TTS+Log（移除 TTS-2，仅剩 TTS-1+LOG-1）
W-FIX-6  → P2 Media+Browser+SSRF（新增 SSRF 防护）
W-FIX-7  → P2 Gemini+OpenRS（不变）
W-FIX-8  → P2 TUI 攻坚（不变）
```

### 文档清理任务

- [ ] 清理 `global-audit-gateway.md` 重复的总结段
- [ ] 补完 `global-audit-infra.md` 隐藏依赖审计和差异清单
- [ ] 补完 `global-audit-autoreply.md` 逐文件对照
- [ ] 消除 TUI-1/TUI-2 在审计报告和 deferred-items 间的 ID 冲突
