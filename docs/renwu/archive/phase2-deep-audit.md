# Phase 2 深度审计报告（三轮合并版）

> 审计日期: 2026-02-12（三轮审计合并）
> 审计视角: 高级全栈架构工程师 (10+ 年 Go/TS/Node 经验)
> 审计范围: Phase 2 — Gateway HTTP/协议层
> 合并自: phase2-deep-audit.md (R1) + r2 + r3

---

## 一、审计总览

| 模块 | R1 继承度 | R2 继承度 | R3 继承度 |
|------|-----------|-----------|-----------|
| Protocol 类型 | 85% | 70%→修复 | ✅ 已对齐 |
| Hooks 核心 | 75% | 88% | 92% |
| Hooks 映射 | 60% | 78% | 85% |
| HTTP 路由 | 35% | 80% | 90% |
| HTTP 工具 | 80% | 65% | 65% |
| Config Reload | 80% | 85% | 90% |

**综合继承度演进: 65% → 78% → 86%**

---

## 二、Protocol 类型审计 (`protocol.go` vs `frames.ts` + `snapshot.ts`)

### 2.1 已正确继承 ✅

- `RequestFrame`, `ResponseFrame`, `EventFrame` 结构体字段与 TS Schema 1:1 对应
- `ErrorShape` 含 `code/message/details/retryable/retryAfterMs`，与 TS 一致
- `ConnectParamsFull` 覆盖了 TS `ConnectParamsSchema` 的所有字段
- `HelloOk` 含 `server/features/snapshot/auth/policy`，字段匹配
- 错误码常量覆盖全面

### 2.2 缺失项 ⚠️

| # | 缺失 | TS 位置 | 严重度 |
|---|------|---------|--------|
| P-1 | `StateVersion` 结构体缺失 | `snapshot.ts:38-42` StateVersionSchema 含 `presence` + `health` 两个 int 字段 | 中 |
| P-2 | `PresenceEntry` 字段不完整 | Go 版仅 5 字段, TS 版有 13 字段 (`host/ip/version/platform/deviceFamily/modelIdentifier/lastInputSeconds/reason/tags/text/ts/deviceId/roles/scopes/instanceId`) | 高 |
| P-3 | `SnapshotSchema` 完全缺失 | TS 定义了 `presence[]/health/stateVersion/uptimeMs/configPath/stateDir/sessionDefaults` | 中 |
| P-4 | `GatewayFrameSchema` 判别联合类型缺失 | TS 使用 `Type.Union([Req,Res,Event], {discriminator:"type"})` 做帧解析分发 | 低 |
| P-5 | `SessionDefaultsSchema` 缺失 | TS: `defaultAgentId/mainKey/mainSessionKey/scope` | 中 |

### 2.3 潜在 Bug 🐛

- **P-BUG-1**: `PresenceEntry.ConnectedAt` 在 Go 中存在但 TS 中不存在 (TS 用 `ts` 字段)。语义可能不一致。

---

## 三、Hooks 核心审计 (`hooks.go` vs `hooks.ts`)

### 3.1 已正确继承 ✅

- `ResolveHooksConfig` 逻辑与 TS `resolveHooksConfig` 基本一致
- `ExtractHookToken` 的 Bearer + X-OpenAcosmi-Token 优先级正确
- `NormalizeHookHeaders` 的大小写转换和多值 join 逻辑正确
- `NormalizeWakePayload` 的 text/mode 校验正确

### 3.2 缺失项 ⚠️

| # | 缺失 | TS 位置 | 严重度 |
|---|------|---------|--------|
| H-1 | `resolveHookChannel` 验证缺失 | TS L158-170: 验证 channel 值必须是 `"last"` 或已注册的 channel plugin ID | 高 |
| H-2 | `getHookChannelError` 错误信息缺失 | TS L156: 动态生成 "channel must be last\|telegram\|slack..." 错误消息 | 中 |
| H-3 | `resolveHookDeliver` 缺失 | TS L172-174: `raw !== false` 逻辑, Go 版直接用 `payload["deliver"].(bool)` | 低 |
| H-4 | `normalizeAgentPayload` 中 channel 验证被跳过 | TS: 调用 `resolveHookChannel` 后若返回 null 则报错, Go 版直接使用字符串 | 高 |
| H-5 | `HookAgentPayload` 缺少 `allowUnsafeExternalContent` 字段 | TS L148 定义了此字段, 用于 Gmail 安全策略 | 中 |

### 3.3 潜在 Bug 🐛

- **H-BUG-1**: Go `NormalizeAgentPayload` 对 `model` 的空值检查逻辑有偏差。TS 是 `modelRaw !== undefined && !model` 判断为错误, Go 用 `payload["model"] != nil && model == ""` — 当 JSON 中 `"model": null` 时 Go 行为不同。
- **H-BUG-2**: Go `NormalizeAgentPayload` 中 `timeoutSeconds` 不取 `Math.floor()`, TS 版取整。

---

## 四、Hooks 映射审计 (`hooks_mapping.go` vs `hooks-mapping.ts`)

### 4.1 已正确继承 ✅

- 基础的 `ResolveHookMappings` 处理 presets + custom mappings 的逻辑框架正确
- `matchPath` 的通配符 `/*` 支持正确
- `GetByPath` 嵌套取值支持 map + array 下标
- `RenderTemplate` 的 `{{expr}}` 正则替换基本正确
- `buildMappingResult` 正确构建 wake/agent payload

### 4.2 缺失项 ⚠️

| # | 缺失 | TS 位置 | 严重度 |
|---|------|---------|--------|
| M-1 | `gmail` preset 完全缺失 | TS L64-77: `hookPresetMappings.gmail` 定义了 Gmail webhook 映射 | 高 |
| M-2 | `textTemplate` 字段缺失 | TS L201: `HookMappingResolved` 含 `textTemplate` 用于 wake 文本模板 | 高 |
| M-3 | `allowUnsafeExternalContent` 字段缺失 | TS L17,203: Gmail 安全字段完全未移植 | 中 |
| M-4 | `query.xxx` 模板变量缺失 | TS L393-398: 支持 `{{query.key}}` 从 URL query 参数取值 | 中 |
| M-5 | `{{now}}` 模板变量缺失 | TS L387-389: 支持 `{{now}}` 输出当前 ISO 时间 | 低 |
| M-6 | `{{payload.xxx}}` 和裸 `{{xxx}}` 语义差异 | TS L399-402: `payload.xxx` 显式从 payload 取, 裸 `xxx` 也从 payload 取; Go 用 `body.xxx` 且不支持裸表达式 | 高 |
| M-7 | `transform` 整个子系统缺失 | TS L155-161,315-333: 支持动态加载 JS 模块做 transform, 含缓存机制 | 高(注) |
| M-8 | `mergeAction` 合并逻辑缺失 | TS L263-300: transform 返回的覆盖值与 base action 合并 | 高(注) |
| M-9 | `validateAction` 缺失 | TS L302-313: 独立的 action 验证函数 | 中 |
| M-10 | `renderOptional` 缺失 | TS L356-362: 可选字段的模板渲染 (空字符串返回 undefined) | 中 |
| M-11 | `normalizeMatchPath` 逻辑不同 | TS L345-354: 去前后斜线做标准化; Go 版保留前导斜线 | 高 |
| M-12 | `matchSource` 逻辑完全不同 | TS L219-224: 从 `ctx.payload.source` 字段匹配; Go 从 HTTP headers 检测。两者语义不同 | 极高 |

> **注**: M-7/M-8 标记为"高"但属预期差异 — Go 无法动态加载 JS 模块, 需要替代方案 (plugin/lua/wasm)。

### 4.3 潜在 Bug 🐛

- **M-BUG-1**: Go `matchPath` 中 pattern 不做 `normalizeMatchPath` 标准化, 但 TS 版对 `ctx.path` 和 `mapping.matchPath` 都做了斜线剥离。直接导致匹配不一致。
- **M-BUG-2**: Go `GetByPath` 仅支持 `items.0.name` 格式的数组下标, TS 版支持 `items[0].name` 方括号语法 (通过正则 `/([^.[\]]+)|(\[(\d+)\])/g` 解析)。

---

## 五、HTTP 路由审计 (`server_http.go` vs `server-http.ts`)

### 5.1 已正确继承 ✅

- `/health` 端点存在
- hooks 处理器的基本框架 (token 验证, body 读取, mapping 匹配)
- OpenAI / Tools Invoke placeholder stubs 存在
- Control UI 静态文件服务存在

### 5.2 缺失项 ⚠️

| # | 缺失 | TS 位置 | 严重度 |
|---|------|---------|--------|
| S-1 | hooks handler: 缺少 `?token=` query 参数拒绝 | TS L150-157: 明确拒绝 URL 中的 token, 防止泄露到日志 | 高 |
| S-2 | hooks handler: 直接路由 `wake` 和 `agent` 子路径缺失 | TS L193-213: `/hooks/wake` 和 `/hooks/agent` 有独立处理逻辑, 不走 mapping 系统 | 极高 |
| S-3 | hooks handler: 空 subPath 应返回 404 | TS L176-181: 当 subPath 为空时返回 404; Go 版走 mapping 匹配 | 中 |
| S-4 | hooks handler: mapping 返回 `skipped` (204) 缺失 | TS L228-232: transform 返回 null 时响应 204 | 中 |
| S-5 | hooks handler: agent 分发返回 `runId` 缺失 | TS L210,259: `dispatchAgentHook` 返回 runId, 响应 202 含 runId; Go 版响应 200 不含 runId | 高 |
| S-6 | hooks handler: body 解析错误区分 413 vs 400 | TS L185: `payload too large` 返回 413, 其他返回 400; Go 版统一 400 | 中 |
| S-7 | Canvas Host 路由完全缺失 | TS L358-378: A2UI, Canvas Host, Canvas WS 路由及授权 | 高(Phase3+) |
| S-8 | Slack HTTP 处理器缺失 | TS L331-333: `handleSlackHttpRequest` | 高(Phase3+) |
| S-9 | OpenResponses API 缺失 | TS L337-347: `/v1/responses` 的完整处理 | 高(Phase3+) |
| S-10 | WebSocket upgrade 处理缺失 | TS L412-450: `attachGatewayUpgradeHandler` | 中(Phase3+) |
| S-11 | 全局 try/catch 500 错误处理缺失 | TS L402-406: 顶层 catch 返回 500 | 中 |
| S-12 | Token 比较未使用常量时间比较 | Go 版 `SafeEqual` 在某些分支使用, 但 TS 版 `token !== hooksConfig.token` 是直接比较。Go 实际更安全, 但要注意一致性 | 低 |

---

## 六、Config Reload 审计 (`reload.go` vs `config-reload.ts`)

### 6.1 已正确继承 ✅

- `ReloadMode` 四种模式 (off/restart/hot/hybrid) 完全对应
- `baseReloadRules` 和 `tailReloadRules` 与 TS 完全一致 (逐条匹配)
- `BuildReloadPlan` 的 rule 匹配 + action 分发逻辑正确
- `applyReloadAction` 的 channel 动态前缀解析正确
- `ConfigWatcher` 的防抖机制正确
- `StartConfigReloader` 的 running/pending 状态机正确

### 6.2 缺失项 ⚠️

| # | 缺失 | TS 位置 | 严重度 |
|---|------|---------|--------|
| R-1 | `restartQueued` 防重复重启标志缺失 | TS L274,324,339: 确保重启只触发一次 | 高 |
| R-2 | `DiffConfigPaths` 不处理 array 浅比较 | TS L159-163: 数组用 `===` 逐项浅比较 | 中 |
| R-3 | TS 版 reload 后会更新 `settings` | TS L312: `settings = resolveGatewayReloadSettings(nextConfig)` | 中 |
| R-4 | TS 在 `mode="off"` 时仍然检测变更但不执行 | TS L319-322: 检测到 `mode=off` 后 log 并 return | 低 |
| R-5 | Go `ResolveReloadSettings` 的 debounce 下限是 50, TS 是 `Math.max(0, ...)` | TS L176: 允许 0ms debounce; Go 强制最小 50ms | 低 |
| R-6 | 缺少 `ConfigFileSnapshot.valid` 验证 | TS L303-308: 检查配置有效性后才 diff | 中 |

### 6.3 潜在 Bug 🐛

- **R-BUG-1**: Go `StartConfigReloader` 的 `handleChange` 在 pending 时递归调用自身，不像 TS 版通过 `schedule()` 重新走防抖。这意味着 Go 版的 pending 不会有防抖延迟。

---

## 七、HTTP 工具审计 (`httputil.go` vs `http-utils.ts` + `http-common.ts`)

### 7.1 已正确继承 ✅

- `ResolveAgentIDFromHeader` 检查 `X-OpenAcosmi-Agent-Id` + fallback `X-OpenAcosmi-Agent`
- `ResolveAgentIDFromModel` 正则匹配 `openacosmi:/` + `agent:` 前缀
- `ResolveAgentIDForRequest` 的优先级 (header > model > "main") 正确
- `WriteSSEData` 和 `SendNotFound` 逻辑正确

### 7.2 缺失项 ⚠️

| # | 缺失 | TS 位置 | 严重度 |
|---|------|---------|--------|
| U-1 | `resolveSessionKey` 缺失 | `http-utils.ts` L65-79: 从 header 或生成 session key | 高 |
| U-2 | `readJsonBodyOrError` 辅助函数缺失 | `http-common.ts` L33-44: 读取 body + 自动回错误 | 低 |

---

## 八、优先修复行动计划

### P0 — 必须立即修复 (阻塞正确性)

1. **S-2**: hooks handler 添加 `/hooks/wake` 和 `/hooks/agent` 直接路由
2. **M-12**: `matchSource` 修复为从 `payload.source` 匹配 (而非 HTTP headers)
3. **M-11**: `normalizeMatchPath` 对齐 TS 逻辑 (去前后斜线)
4. **S-1**: 添加 `?token=` query 参数拒绝逻辑
5. **R-1**: 添加 `restartQueued` 防重复重启标志

### P1 — 高优先级 (功能缺失)

6. **M-1**: 添加 `gmail` preset 映射
7. **M-2**: 添加 `textTemplate` 字段支持
8. **P-2**: 完善 `PresenceEntry` 全部 13 个字段
9. **M-6**: 模板表达式对齐 — 支持 `payload.xxx` 和裸 `xxx`
10. **H-1/H-4**: 添加 `resolveHookChannel` 验证
11. **S-5**: agent hook 分发返回 `runId` + 响应 202
12. **U-1**: 添加 `resolveSessionKey`
13. **M-BUG-2**: `GetByPath` 添加 `[0]` 方括号语法支持

### P2 — 中优先级 (合规/健壮性)

14. **P-1**: 添加 `StateVersion` 结构体
15. **P-3**: 添加 `SnapshotSchema` 类型
16. **M-4/M-5**: 模板添加 `query.xxx` 和 `now` 支持
17. **S-6**: body 错误区分 413 vs 400
18. **S-11**: 添加全局 500 错误处理
19. **R-2**: `DiffConfigPaths` 添加 array 浅比较
20. **R-3**: reload 后更新 settings

### P3 — 低优先级 / 预期差异

21. M-7/M-8: transform 子系统 (需架构决策)
22. S-7~S-10: Canvas/Slack/OpenResponses (Phase 3+ 范围)
23. H-3/R-5: 细微行为差异

---

## 九、三轮审计综合总结

| 指标 | R1 | R2 | R3 |
|------|----|----|-----|
| 综合继承度 | 65% | 78% | 86% |
| 发现 BUG 数 | 8 | 3 | 2 |
| 已修复比例 | 0% | 72% | 91% |

### R3 关键 BUG（新发现）

- **R3-M7**: `sessionKey` 模板未渲染 — Gmail preset 的 `hook:gmail:{{messages[0].id}}` 原样传递，所有 Gmail hook 共享 session
- **R3-M8**: `name`/`to`/`model`/`thinking` 字段也未渲染模板

### 当前残留项汇总

**P0（阻塞正确性）**: R3-M7 + R3-M8（`buildMappingResult` agent 分支模板渲染）

**P1（高优先级）**: U-R1/U-R2（`resolveSessionKey` 完整实现）

**P2（中优先级）**: R3-M1/M2/M4/M6（renderOptional、wake fallback、JSON结构、validateAction）

**Phase 5+ 延期**: M-R5/M-R6（transform 管道）、H-R1（channel 动态插件）

**结论**: 修复 R3-M7 和 R3-M8 后，Phase 2 继承度将达到 **90%+**，剩余项为 Phase 5+ 的 transform 管道和配置格式对齐，不阻塞当前阶段。
