# Phase 2 深度审计报告 (Round 2)

> 审计日期: 2026-02-12 (Round 2 — 修复后复审)
> 审计视角: 高级全栈架构工程师 (10+ 年 Go/TS/Node 经验)
> 审计范围: Phase 2 — Gateway HTTP/协议层
> 前置: Round 1 审计的 22 项 P0/P1/P2 修复已全部完成并通过测试

---

## 一、审计总览 (修复后)

| 模块 | Go 文件 | TS 对标文件 | 继承度 | 风险等级 |
| ---- | ------- | ---------- | ------ | -------- |
| Protocol 类型 | `protocol.go` | `snapshot.ts` + `schema.ts` | 70% | 中 |
| Hooks 核心 | `hooks.go` | `hooks.ts` | 88% | 低 |
| Hooks 映射 | `hooks_mapping.go` | `hooks-mapping.ts` | 78% | 中 |
| HTTP 路由 | `server_http.go` | `server-http.ts` | 80% | 中 |
| HTTP 工具 | `httputil.go` | `http-utils.ts` | 65% | 高 |
| Config Reload | `reload.go` | `config-reload.ts` | 85% | 低 |

**综合继承度: ~78%** (Round 1 为 65%, 提升 13 个百分点)

---

## 二、Protocol 类型审计

### 2.1 已修复 ✅ (本轮确认)

- P-1: `SnapshotStateVersion` 已添加 (seq/ts/by)
- P-2: `PresenceEntry` 已扩展
- P-3: `SnapshotData` 已添加

### 2.2 残留问题 ⚠️

| # | 问题 | 详情 | 严重度 |
| - | ---- | ---- | ------ |
| P-R1 | `PresenceEntry` 字段与 TS Schema 不一致 | TS 实际字段: `host/ip/version/platform/deviceFamily/modelIdentifier/mode/lastInputSeconds/reason/tags/text/ts/deviceId/roles/scopes/instanceId` (16字段)。Go 版用了不同字段集: `ConnID/DisplayName/Role/Scopes/Platform/DeviceFamily/Version/Mode/ConnectedAt/Caps/ViewWidth/ViewHeight/Locale` (13字段)。`ConnID/DisplayName/Caps/ViewWidth/ViewHeight/Locale` 在 TS 中不存在; `host/ip/modelIdentifier/lastInputSeconds/reason/tags/text/deviceId/roles/instanceId` 在 Go 中缺失 | **高** |
| P-R2 | `SnapshotStateVersion` 字段不匹配 | TS `StateVersionSchema` 定义 `{presence: int, health: int}`, Go 版定义 `{seq, ts, by}` — 完全不同的语义 | **高** |
| P-R3 | `SnapshotData` 缺失字段 | TS `SnapshotSchema` 含 `uptimeMs/configPath/stateDir/sessionDefaults/health`, Go 版仅 `presence/state/version` | **中** |
| P-R4 | `SessionDefaultsSchema` 仍缺失 | TS: `{defaultAgentId, mainKey, mainSessionKey, scope}` | **中** |

---

## 三、Hooks 核心审计 (`hooks.go` vs `hooks.ts`)

### 3.1 已修复 ✅

- H-1: channel 验证已添加 (`validateHookChannel`)
- `resolveHooksConfig` / `extractHookToken` / `normalizeHookHeaders` / `normalizeWakePayload` 均正确

### 3.2 残留问题 ⚠️

| # | 问题 | 详情 | 严重度 |
| - | ---- | ---- | ------ |
| H-R1 | Channel 验证硬编码 vs TS 动态 | TS 用 `listChannelPlugins()` 动态构建合法 channel 集合 (含插件注册的 channel)。Go 硬编码 `{last, new, background}` — 缺少插件扩展性 | **中** |
| H-R2 | `resolveHookDeliver` 语义差异 | TS: `raw !== false`（任何非 false 值都视为 true，包括 undefined/null/number）。Go: `payload["deliver"].(bool)` 类型断言 — 当 `deliver` 为 `"true"` 字符串或 `1` 数字时 Go 返回 false，TS 返回 true | **中** |
| H-R3 | `HookAgentPayload` 缺少 `allowUnsafeExternalContent` | TS `HookAgentPayload` 无此字段但 `dispatchAgentHook` 参数中含它。Go 的 `HookAgentPayload` 也缺失此字段，mapping 结果通过 `payload map` 传递但 agent dispatch 时丢失 | **高** |
| H-R4 | `timeoutSeconds` 未取整 | TS: `Math.floor(timeoutRaw)`, Go: `int(ts)` — Go 的 `int()` 截断等效 floor 但 TS 还检查 `Number.isFinite()`。Go 未检查 Inf/NaN | **低** |

---

## 四、Hooks 映射审计 (`hooks_mapping.go` vs `hooks-mapping.ts`)

### 4.1 已修复 ✅

- M-1: Gmail preset 已添加
- M-2: TextTemplate 字段已添加
- M-3: AllowUnsafeExternalContent 已添加
- M-4: `query.xxx` 模板变量已支持
- M-5: `{{now}}` 模板变量已支持
- M-6: `payload.xxx` + 裸变量 fallback 已支持
- M-11: `normalizeMatchPath` 去前后斜线已对齐
- M-12: `matchSource` 从 `ctx.Source` 比较已对齐
- M-BUG-2: `GetByPath` 支持 `[0]` 方括号已添加

### 4.2 残留问题 ⚠️

| # | 问题 | 详情 | 严重度 |
| - | ---- | ---- | ------ |
| M-R1 | Gmail preset 字段与 TS 不一致 | TS gmail: `matchPath="gmail"`, `sessionKey="hook:gmail:{{messages[0].id}}"`. Go: `matchSource="gmail"`, `sessionKey=""`. TS 用 path 匹配; Go 用 source 匹配，且缺少 sessionKey 模板 | **高** |
| M-R2 | TS `HookMappingConfig` 使用嵌套 `match: {path, source}` | TS: `match.path` / `match.source`。Go 用扁平 `MatchPath` / `MatchSource` — JSON 结构不兼容, 无法解析 TS 格式配置 | **高** |
| M-R3 | `renderOptional` 缺失 | TS: 可选字段模板渲染结果为空则返回 `undefined`。Go: `buildMappingResult` 直接写入字段, 空字符串也会写入 payload | **中** |
| M-R4 | `validateAction` 缺失 | TS: 独立验证 wake 需要 text、agent 需要 message。Go: `buildMappingResult` 内联部分验证但逻辑不同 | **中** |
| M-R5 | `mergeAction` 缺失 | TS: transform 返回值与 base action 合并。Go 无 transform 故暂无影响, 但架构上缺少此抽象 | **低(注)** |
| M-R6 | mapping 结果 204 (skipped) 缺失 | TS: transform 返回 null → 响应 204。Go: 无此路径 | **低(注)** |
| M-R7 | 模板渲染: `GetByPath` 返回 nil 时行为差异 | TS: null/undefined → `""` (空字符串)。Go: `fmt.Sprint(nil)` → `"<nil>"` 字符串 | **高(BUG)** |
| M-R8 | 模板渲染: 非字符串值处理差异 | TS: 对象/数组 → `JSON.stringify()`。Go: `fmt.Sprint()` → Go 默认格式 (如 `map[key:value]`) | **中** |

---

## 五、HTTP 路由审计 (`server_http.go` vs `server-http.ts`)

### 5.1 已修复 ✅

- S-1/S-2/S-3/S-5/S-6/S-11 全部已修复通过

### 5.2 残留问题 ⚠️

| # | 问题 | 详情 | 严重度 |
| - | ---- | ---- | ------ |
| S-R1 | `/hooks/wake` 响应格式差异 | TS: `{ok:true, mode:"now"}`. Go: `{ok:true}` — 缺少 `mode` 字段 | **中** |
| S-R2 | mapping agent 分发缺少 `allowUnsafeExternalContent` 传递 | TS L257: `allowUnsafeExternalContent: mapped.action.allowUnsafeExternalContent`. Go: 通过 `NormalizeAgentPayload` 但 `HookAgentPayload` 无此字段 | **高** |
| S-R3 | mapping wake 分发未使用 TS 的 `buildActionFromMapping` 结构 | TS 直接用 mapping 结果的 `action` 对象。Go 重新走 `NormalizeWakePayload` — 架构差异导致 mapping 的 `mode` 可能被覆盖 | **中** |
| S-R4 | mapping 匹配后 channel 未二次验证 | TS L241-243: 对 mapping agent 结果再次调用 `resolveHookChannel` 验证。Go: 无此二次验证 | **中** |

---

## 六、HTTP 工具审计 (`httputil.go` vs `http-utils.ts`)

### 6.1 残留问题 ⚠️

| # | 问题 | 详情 | 严重度 |
| - | ---- | ---- | ------ |
| U-R1 | `ResolveSessionKey` 语义完全不同 | TS: 从 header `x-openacosmi-session-key` 提取, 无则用 `buildAgentMainSessionKey({agentId, mainKey})` 构建, mainKey 含 user/prefix/uuid。Go: 简单的 sessionKey→channel→"default" 三级 fallback | **高** |
| U-R2 | Go 缺少 `buildAgentMainSessionKey` 依赖 | TS 引用 `../routing/session-key.js` 模块。Go 无对应实现 | **高** |

---

## 七、Config Reload 审计 (`reload.go` vs `config-reload.ts`)

### 7.1 已修复 ✅

- R-1: `restartQueued` 已添加
- R-2: `DiffConfigPaths` 数组比较已添加

### 7.2 残留问题 ⚠️

| # | 问题 | 详情 | 严重度 |
| - | ---- | ---- | ------ |
| R-R1 | `DiffConfigPaths` 数组比较语义不同 | TS: 数组相等性用浅 `===` 逐项比较, 不递归。Go: 数组递归比较到子元素。行为不同但 Go 版更精细 | **低** |
| R-R2 | pending 重试不经过防抖 | TS: pending 时调 `schedule()` 重走防抖。Go: pending 时直接递归 `handleChange()`, 无防抖延迟 | **中** |
| R-R3 | `snapshot.valid` 验证缺失 | TS L304: 校验配置快照合法性后才 diff。Go: `LoadConfig` 返回 error 但无 `valid/issues` 结构化检查 | **中** |
| R-R4 | reload 后未更新 `settings` | TS L312: `settings = resolveGatewayReloadSettings(nextConfig)`. Go: `r.settings` 不更新 — 改了 debounce 后不生效 | **中** |

---

## 八、修复优先级行动计划

### P0 — 必须立即修复 (阻塞正确性)

1. **M-R7**: `GetByPath` 返回 nil 时 `fmt.Sprint(nil)` → `"<nil>"` BUG
2. **P-R1**: `PresenceEntry` 字段与 TS Schema 完全不一致
3. **P-R2**: `SnapshotStateVersion` 字段语义完全不同
4. **M-R1**: Gmail preset 匹配策略/sessionKey 不一致

### P1 — 高优先级

5. **H-R3**: `HookAgentPayload` + dispatch 添加 `allowUnsafeExternalContent`
6. **U-R1/U-R2**: `resolveSessionKey` 对齐 TS 完整逻辑
7. **M-R2**: `HookMappingConfig` JSON 结构对齐 (嵌套 `match`)
8. **S-R2**: mapping agent 分发传递 `allowUnsafeExternalContent`

### P2 — 中优先级

9. **S-R1**: wake 响应添加 `mode` 字段
10. **R-R2**: pending 重试走防抖
11. **R-R4**: reload 后更新 settings
12. **M-R3**: 添加 `renderOptional`
13. **M-R8**: 非字符串值用 `json.Marshal` 替代 `fmt.Sprint`
14. **S-R4**: mapping agent 结果二次验证 channel

### P3 — 低优先级/预期差异

15. H-R1: channel 动态插件注册 (Phase 5+)
16. R-R1: 数组比较策略可保持 Go 方式
17. M-R5/M-R6: transform 子系统 (架构决策)

---

## 九、总结

相比 Round 1, 综合继承度从 65% 提升至 **78%**。核心路由、token 验证、配置重载框架已基本对齐。

**关键发现:**

1. **Protocol 类型严重偏差** — `PresenceEntry` 和 `StateVersion` 的字段集与 TS 完全不同, 是直接参考了错误的文档
2. **`fmt.Sprint(nil)` → `"<nil>"` BUG** — 模板渲染中 `GetByPath` 返回 nil 会输出 `<nil>` 字符串
3. **`resolveSessionKey` 语义差距大** — Go 版是简化版, 缺少 header 提取和 `buildAgentMainSessionKey`
4. **Gmail preset 匹配策略错误** — TS 用 path 匹配, Go 用 source 匹配
5. **JSON 配置格式不兼容** — `HookMappingConfig` 的 match 字段结构不同

建议优先修复 P0 项 (4 项), 预计工作量 0.5 天。
