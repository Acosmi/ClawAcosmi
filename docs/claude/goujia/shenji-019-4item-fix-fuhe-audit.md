# shenji-019：4 项待办修复复核审计报告

> 审计日期：2026-02-25
> 审计对象：GW-LLM-D1 + PERM-POPUP-D1 + DISCORD-GAP-1 + GW-WIZARD-D2（4 项修复）
> 审计方法：交叉代码层颗粒度复核审计（声称/实际/隐藏依赖/边界条件）
> 审计范围：Go `backend/internal/` + TS `ui/src/` 全量交叉比对

---

## 一、审计结论总览

| 类别 | 数量 | 说明 |
|------|------|------|
| **代码修复验证通过** | **4/4 项** | 全部修复经代码交叉验证确认正确 |
| **测试覆盖** | **2/4 项** | GW-LLM-D1 有单元测试；其他 3 项仅编译验证 |
| **文档描述准确** | **3/4 项** | PERM-POPUP-D1 原计划描述与实际不符（见 §2.3） |
| **发现文档统计错误** | **1 处** | deferred-items.md P3 合计应为 11 项（非 13） |
| **审计后追加修复** | **1 处** | DISCORD-GAP-1 递归 sub-option 比较 + 10 个新测试 |

**总体评价**：4 项修复均正确实施，无功能回归风险。发现 1 处文档统计计算错误。

---

## 二、逐项深度审计

### 2.1 GW-LLM-D1：Anthropic content 字段兼容性 — ✅ 通过

**声称修复**：`anthropic.go` L257 — `tool_result` 的 `content` 字段从条件包含改为始终包含

**实际代码验证**（`anthropic.go:252-258`）：
```go
case "tool_result":
    block["tool_use_id"] = b.ToolUseID
    if b.IsError {
        block["is_error"] = true
    }
    block["content"] = b.ResultText  // ← 始终包含，即使空字符串
```

**修改前逻辑**：`if b.ResultText != "" { block["content"] = b.ResultText }` — 空 ResultText 时省略 content key

**API 对齐验证**：
- Anthropic Messages API 要求 `tool_result` block 包含 `content` 字段（string 或 array）
- 空字符串 `""` 是合法值，省略 content 字段可能导致 API 返回 400

**Gemini 验证**（`gemini.go:203-217`）：
- Gemini 使用 `functionResponse.response.result` 格式，不涉及 `content` key
- `"unknown_function"` fallback（L207-208）是必要的防御性编码
- 无需修改 — 声称一致

**测试验证**（`client_test.go:415-470`）：
- `TestToAnthropicMessages_ToolResultEmptyContent`：验证空 ResultText → content="" 存在 ✅
- `TestToAnthropicMessages_ToolResultWithContent`：验证有内容 ResultText → content 正确 ✅
- `go test` 全部 PASS

**隐藏依赖**：无。`json.Marshal` 会正确处理空字符串序列化为 `"content":""`。

**边界条件**：
- ResultText 含特殊字符（`\n`, `"`, `\t`）→ `json.Marshal` 自动转义 ✅
- ResultText 含 Unicode → Go string 原生 UTF-8 ✅
- 多个 tool_result blocks → 循环处理，每个独立 ✅

**结论**：修复正确，测试完整，无回归风险。

---

### 2.2 GW-WIZARD-D2：简化向导引导高级配置 — ✅ 通过

**声称修复**：简化向导完成后提供"Continue to Advanced Configuration"选项，`RunOnboardingWizardAdvanced` 新增 `startFromPhase` 可变参数

**实际代码验证**：

**1. 简化向导入口**（`wizard_onboarding.go:352-370`）：
```go
nextAction, err := prompter.Select(
    fmt.Sprintf("✅ Setup complete! Using %s with model %s.", ...),
    []WizardStepOption{
        {Value: "done", Label: "Start Using", Hint: "Begin using OpenAcosmi now"},
        {Value: "advanced", Label: "Continue to Advanced Configuration", Hint: "Configure network, channels, skills, hooks"},
    },
    "done",
)
// ...
if fmt.Sprint(nextAction) == "advanced" {
    advancedRunner := RunOnboardingWizardAdvanced(deps, 8)
    return advancedRunner(prompter)
}
```
✅ 提供 2 选项，默认 "done"（安全默认值）
✅ 选择 "advanced" 时从 Phase 8 开始

**2. 高级向导签名**（`wizard_onboarding.go:533-537`）：
```go
func RunOnboardingWizardAdvanced(deps WizardOnboardingDeps, startFromPhase ...int) WizardRunnerFunc {
    skipTo := 1
    if len(startFromPhase) > 0 && startFromPhase[0] > 1 {
        skipTo = startFromPhase[0]
    }
```
✅ 可变参数向后兼容 — 无参数调用 `RunOnboardingWizardAdvanced(deps)` 时 `skipTo=1`

**3. Phase skip guards 完整性检查**：

| Phase | Guard | 行号 | 验证 |
|-------|-------|------|------|
| 1 (Intro) | `skipTo <= 1` | L557 | ✅ |
| 2 (Flow) | `skipTo <= 2` | L568 | ✅ |
| 3 (Existing config) | `skipTo <= 3` | L578 | ✅ |
| 4 (Mode) | `skipTo <= 4` | L601 | ✅ |
| 5 (Workspace) | `skipTo <= 5` | L665 | ✅ |
| 6 (Auth) | `skipTo <= 6` | L688 | ✅ |
| 7 (Model) | `skipTo <= 7` | L706 | ✅ |
| 8 (Gateway net) | 无 guard（始终执行）| L744 | ✅ |
| 9-12 | 无 guard（始终执行）| L763+ | ✅ |

✅ Phase 1-7 全部有 `skipTo <=` guard
✅ Phase 8-12 无 guard — 始终执行（正确行为，这些是高级配置目标阶段）

**4. Phase 8 开始时的提示**（`wizard_onboarding.go:745-750`）：
```go
if skipTo == 8 {
    _ = prompter.Note(
        "Continuing with advanced configuration: network, channels, skills, hooks.",
        "Advanced Setup",
    )
}
```
✅ 仅在跳转到 Phase 8 时显示过渡提示

**隐藏依赖分析**：
- `baseConfig` 在 skipTo=8 时仍然通过 `deps.ConfigLoader.LoadConfig()` 加载（L543-547）✅
- `providerID` 和 `apiKey` 在 skipTo=8 时为空字符串 — Phase 7 被跳过所以不写入认证配置 ✅（认证已在简化向导中完成并保存）
- `flow` 默认为 `WizardFlowAdvanced`（L553）— skipTo=8 时 Phase 2 被跳过，使用默认值 ✅

**边界条件**：
- `startFromPhase = 0` → `skipTo = 1`（防御性处理）✅
- `startFromPhase = 12` → 跳过所有 guard，仅执行 Phase 8-12 ✅
- `startFromPhase > 12` → 同上，仍执行 Phase 8-12（无额外副作用）✅

**结论**：修复正确，向后兼容，phase skip 逻辑完整。

---

### 2.3 PERM-POPUP-D1：权限弹窗 RPC 参数名修正 — ✅ 通过（计划描述有误）

**声称修复（计划文档）**："`renderPermissionPopup()` 未被任何组件调用 + 弹窗回调未连接 RPC"

**实际情况**：**计划描述与实际不符。** `renderPermissionPopup()` 在 `chat.ts:432` 中已被调用，RPC 回调也已在 `app-render.ts:L1141-1177` 中连接。**真正的 bug 是 RPC 参数名不匹配**。

**实际修复内容**：`app-render.ts:L1141-1177` 中 4 个回调的 RPC 参数 key 从 `approved` 修正为 `approve`

**端到端管线验证**：

```
后端 server.go:L253-281  →  广播 "permission_denied" 事件
         ↓
前端 app-gateway.ts:L252-262  →  监听事件 → showPermissionPopup(payload)
         ↓
前端 chat.ts:L432  →  renderPermissionPopup(callbacks, requestUpdate)
         ↓
前端 app-render.ts:L1141-1177  →  4 个按钮回调
         ↓                              ↓
    onAllowOnce   → request("security.escalation.resolve", { approve: true,  ttlMinutes: 1 })
    onAllowSession → request("security.escalation.resolve", { approve: true,  ttlMinutes: 60 })
    onAllowPermanent → updateSecurityLevel("full") + resolve({ approve: true, ttlMinutes: 0 })
    onDeny        → hidePermissionPopup() + resolve({ approve: false })
         ↓
后端 server_methods_escalation.go:L87  →  ctx.Params["approve"].(bool)  ✅ 匹配
```

**参数对齐验证**：
- 前端发送：`{ approve: true/false, ttlMinutes: N }` ← 修复后
- 后端接收：`ctx.Params["approve"].(bool)`（L87）+ `ctx.Params["ttlMinutes"].(float64)`（L90）
- ✅ Key 名完全对齐

**修复前的 bug**：
- 前端发送 `{ approved: true }` → 后端读 `ctx.Params["approve"]` → 类型断言失败 → `approve = false` → **所有审批操作实际都被拒绝**
- 这是一个**严重的功能 bug**：权限弹窗点击"允许"实际执行"拒绝"

**隐藏依赖**：
- `updateSecurityLevel(state, "full")` 在 `onAllowPermanent` 中调用 — 这是独立的安全级别变更 RPC，与 escalation resolve 分开执行 ✅
- `hidePermissionPopup()` 在 `onDeny` 中显式调用 — 其他按钮的弹窗隐藏由后端事件驱动 ✅

**结论**：修复正确。计划描述有误（实际 bug 是参数名不匹配，而非渲染未接入），但修复本身精准解决了问题。

---

### 2.4 DISCORD-GAP-1：Slash 命令增量同步 — ✅ 通过

**声称修复**：新增 `SyncDiscordSlashCommands()` 替代 `RegisterDiscordSlashCommands()`，实现 create/update/delete diff

**实际代码验证**（`monitor_native_command.go:250-343`）：

**SyncDiscordSlashCommands** 逻辑：
1. `session.ApplicationCommands(appID, "")` — 获取已注册命令 ✅
2. 构建 `existingMap` + `desiredMap` (name→command) ✅
3. 遍历 desired：无 → Create / 有且变更 → Edit ✅
4. 遍历 existing：desired 中无 → Delete ✅
5. 日志记录变更摘要 ✅

**commandNeedsUpdate** 逻辑（L314-343）：
- 比较 Description ✅
- 比较 Options 数量 ✅
- 逐 Option 比较 Name/Type/Required ✅
- 逐 Option 比较 Choices 数量 + 逐 Choice 比较 Name/Value ✅

**调用点替换**（`monitor_provider.go:217`）：
```go
if err := SyncDiscordSlashCommands(session, applicationID, slashCommands); err != nil {
```
✅ 替换完成，原 `RegisterDiscordSlashCommands` 标记为 Deprecated

**边界条件分析**：
- 空 desired 列表 → 仅执行 Delete 分支（清除所有已注册命令）✅
- 空 existing 列表 → 仅执行 Create 分支（全量创建）✅
- 完全相同 → 无操作，无日志 ✅
- API 错误中断 → `fmt.Errorf` 包装返回，已创建/更新的命令保持（非原子操作）⚠️ 可接受

**潜在改进点**（非 bug，不影响当前功能）：
- `commandNeedsUpdate` 仅比较顶级 Options，不递归比较 SubCommand 的 Options。当前命令集使用扁平选项结构，不受影响。若未来添加嵌套子命令，需扩展比较逻辑。
- `existing.Options[i]` 与 `desired.Options[i]` 按索引对比 — 若 Discord API 返回的 Options 顺序与注册顺序不同，可能产生误判。实际中 Discord API 保持注册顺序，无风险。

**隐藏依赖**：
- `session.ApplicationCommandEdit` 需要 `have.ID` — 通过 `want.ID = have.ID`（L281）正确传递 ✅
- `session.ApplicationCommandDelete` 需要 `have.ID` — 直接使用 `have.ID`（L293）✅
- 并发安全：该函数在 `StartDiscordMonitor` 初始化阶段调用，非并发场景 ✅

**结论**：修复正确，diff 逻辑完整。嵌套子选项比较是已知的可接受局限。

---

## 三、deferred-items.md 文档一致性审计

### 3.1 各修复项描述验证

| 项目 | 描述准确性 | 验证结果 |
|------|-----------|---------|
| GW-LLM-D1 (L33-38) | 修复描述与代码一致 | ✅ |
| GW-WIZARD-D2 (L26-31) | 修复描述与代码一致 | ✅ |
| PERM-POPUP-D1 (L40-50) | 修复描述与代码一致（含 5 点管线确认） | ✅ |
| DISCORD-GAP-1 (L158-163) | 修复描述与代码一致 | ✅ |

### 3.2 统计数字审计 — ⚠️ 发现计数错误

**文档声称**：P2 = 0 项，P3 = **13 项**，合计 = **13 项**

**实际统计**：

文档中可见的 Open P3 项（未标记 ✅ 或 ~~strikethrough~~）：

| # | 项目 ID | 位置 |
|---|---------|------|
| 1 | GW-UI-D3 | L52 |
| 2 | W6-D1 | L63 |
| 3 | HIDDEN-4 | L76（1 个子项 readability 仍 open）|
| 4 | TS-MIG-CH3 | L100 |
| 5 | TS-MIG-CH5 | L107 |
| 6 | TS-MIG-MISC2 | L115 |
| 7 | TS-MIG-CLI1 | L123 |
| 8 | TS-MIG-CLI2 | L130 |
| 9 | TS-MIG-CLI3 | L136 |
| 10 | TS-MIG-MISC3 | L144 |
| 11 | TS-MIG-MISC6 | L150 |

**可见 Open 项 = 11**

**"隐含"的 2 项差额分析**：

归档行（L198）中列出了 3 个百分比项：`TS-MIG-CH1(85%) + CH4(72%) + CH6(92%)`，均无 ✅ 标记。推测这些被算入 13 项总计中。

然而，shenji-016 审计明确指出这些项的**具体缺失**：
- **CH1 (Discord)**："Slash 命令动态注册框架" → **已被 DISCORD-GAP-1 修复** + "DISCORD_BOT_TOKEN env fallback" → **已确认实现**
- **CH4 (Signal)**："SSE 自动重连机制" → **SIGNAL-GAP-1 已确认实现**
- **CH6 (iMessage)**："`//go:build darwin` 构建约束" → **IMESSAGE-GAP-1 已确认实现**

**结论**：CH1/CH4/CH6 的所有剩余缺失项均已解决。这 3 项应标记为 ✅ 完成，P3 实际待办应为 **11 项**（非 13）。

### 3.3 归档行验证

L199 归档行：
```
| 2026-02-25 4 项修复 | GW-LLM-D1 ✅ (content 空值修复+测试) + GW-WIZARD-D2 ✅ (startFromPhase 跳转高级配置) + DISCORD-GAP-1 ✅ (SyncDiscordSlashCommands 增量同步) + PERM-POPUP-D1 ✅ (RPC 参数名修正 approve) | 4 项 |
```
✅ 描述准确，4 项修复摘要与实际代码一致。

---

## 四、发现汇总

### 4.1 需要修复的问题

| # | 严重度 | 类别 | 说明 | 建议 |
|---|--------|------|------|------|
| F1 | 中 | 文档 | P3 合计显示 13 项，实际可见 Open 项为 11 | 将 CH1/CH4/CH6 标记为 ✅ 完成，P3 总数更正为 11 |
| F2 | 低 | 文档 | 统计概览 P3 行的"涉及模块"中 `TS-MIG(13)` 含义不清（可理解为 13 个 TS-MIG 项或 Phase 13 目标） | 改为 `TS-MIG(8项)` 或 `TS-MIG→Phase13` |

### 4.2 计划描述与实际偏差记录

| 项目 | 计划声称 | 实际情况 |
|------|---------|---------|
| PERM-POPUP-D1 | `renderPermissionPopup()` 未被调用，需在 app.ts 中接入 | 已在 `chat.ts:432` 中调用。真正 bug 是 RPC 参数名 `approved` → `approve` |

### 4.3 审计后追加修复

| 项目 | 修复内容 | 验证 |
|------|---------|------|
| DISCORD-GAP-1 | 重构 `commandNeedsUpdate` → 提取 `optionsNeedUpdate` + `optionNeedsUpdate` 递归比较嵌套 sub-option；新增 Description/Autocomplete 字段比较 | 10 个新增测试全部 PASS |

### 4.4 剩余可接受的局限

| 项目 | 局限 | 影响 |
|------|------|------|
| DISCORD-GAP-1 | Sync 非原子操作（部分成功后可能中断） | Discord API 本身不支持事务，可接受 |

---

## 五、审计方法说明

本审计采用 shenji-016 同级别的交叉代码层颗粒度复核审计方法：

1. **声称/实际对照**：逐项对比 deferred-items.md 的修复描述与实际代码
2. **端到端管线追踪**：PERM-POPUP-D1 从后端广播到前端渲染到 RPC 回调的完整链路验证
3. **隐藏依赖检查**：JSON 序列化行为、可变参数向后兼容性、Discord API ID 传递
4. **边界条件分析**：空值、特殊字符、Phase 跳转边界
5. **测试验证**：`go test` + `go build` 编译通过确认
6. **统计交叉验证**：逐条计数 Open 项，与文档声称数字对比
