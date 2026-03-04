# Phase 2 → Phase 5 上下文文档

> 生成日期: 2026-02-13
> 目的: 记录 Phase 2 审计中发现的延迟至 Phase 5 实现的功能项及其完整 TS 参考

---

## 一、延迟项总览

| # | 项目 | 来源 | TS 文件 | Go 文件 | Phase 5 任务类型 |
| - | ---- | ---- | ------- | ------- | --------------- |
| D-1 | Transform 管道 | R2-M5/M6 | `hooks-mapping.ts` L137-332 | `hooks_mapping.go` | 新增功能 |
| D-2 | `HookMappingConfig` 嵌套 match 结构 | R2-M2 | `hooks-mapping.ts` L175-211 | `hooks_mapping.go` L16-40 | 配置兼容 |
| D-3 | `resolveSessionKey` 完整逻辑 | R2-U1/U2 | `http-utils.ts` L65-79 | `httputil.go` | 逻辑增强 |
| D-4 | `buildAgentMainSessionKey` + session-key 模块 | R2-U2 | `routing/session-key.ts` | 无 (需新建) | 新增模块 |
| D-5 | Channel 动态插件注册 | R2-H1 | `hooks.ts` | `hooks.go` | 架构增强 |
| D-6 | `validateAction` 专用函数 | R3-M6 | `hooks-mapping.ts` L302-313 | `hooks_mapping.go` | 代码质量 |

---

## 二、D-1: Transform 管道

### 2.1 功能描述

TS 的 hook mapping 支持为每个 mapping 配置一个 `transform` 模块。在基础 action 构建完成后, transform 函数可以:

1. 修改 action 的任意字段
2. 返回 `null` 跳过该 mapping (响应 204)
3. 切换 action 类型 (wake ↔ agent)

### 2.2 TS 实现参考

**数据流**: `matchMapping → buildActionFromMapping → loadTransform → transform(ctx) → mergeAction → validateAction`

**关键类型** (`hooks-mapping.ts` L26-100):

```typescript
type HookMappingTransformResolved = {
  modulePath: string;  // JS 模块绝对路径
  exportName?: string; // 导出名, 默认 "default" 或 "transform"
};

type HookTransformResult = Partial<{
  kind: "wake" | "agent";
  text: string; mode: string; message: string; wakeMode: string;
  name: string; sessionKey: string; deliver: boolean;
  allowUnsafeExternalContent: boolean; channel: string;
  to: string; model: string; thinking: string; timeoutSeconds: number;
}> | null;  // null = 跳过

type HookTransformFn = (ctx: HookMappingContext)
  => HookTransformResult | Promise<HookTransformResult>;
```

**核心函数** (`hooks-mapping.ts` L137-170):

```typescript
// applyHookMappings 中的 transform 管道
let override: HookTransformResult = null;
if (mapping.transform) {
  const transform = await loadTransform(mapping.transform);
  override = await transform(ctx);
  if (override === null) {
    return { ok: true, action: null, skipped: true };  // 204 skipped
  }
}
const merged = mergeAction(base.action, override, mapping.action);
```

**`mergeAction`** (`hooks-mapping.ts` L263-299):

- override 字段优先于 base 字段
- `kind` 可被覆盖 (wake→agent)
- boolean 字段 (deliver, allowUnsafe) 需 typeof 检查
- 非 boolean 字段用 `??` fallback

**`loadTransform`** (`hooks-mapping.ts` L315-325):

- 使用 ESM `import()` 动态加载模块
- 有 `transformCache` (L79) 缓存已加载模块

### 2.3 Go 实现建议

Go 不支持 ESM 动态 import, 建议方案:

1. **Go 插件**: 使用 `plugin.Open()` 加载 `.so` — 兼容性差
2. **内置 transform 注册表**: 通过 `RegisterTransform(name, fn)` 注册, 映射时用 name 引用
3. **Lua/Starlark 脚本**: 轻量级脚本引擎

### 2.4 当前 Go 状态

`HookMappingResolved` 已有 `TransformModule`/`TransformExport` 字段 (L62-63), 但 `buildMappingResult` 中未使用。`ApplyHookMappings` 直接返回 `buildMappingResult` 结果, 无 transform 管道。

---

## 三、D-2: HookMappingConfig 嵌套 match 结构

### 3.1 问题

TS 配置格式:

```json
{
  "match": { "path": "gmail", "source": "webhook" },
  "action": "agent"
}
```

Go 配置格式 (当前):

```json
{
  "matchPath": "gmail",
  "matchSource": "webhook",
  "action": "agent"
}
```

### 3.2 TS 参考

`hooks-mapping.ts` L175-211 (`normalizeHookMapping`):

```typescript
const matchPath = normalizeMatchPath(mapping.match?.path);
const matchSource = mapping.match?.source?.trim();
```

配置类型定义在 `config/config.ts` 的 `HookMappingConfig`:

```typescript
export type HookMappingConfig = {
  id?: string;
  match?: { path?: string; source?: string };
  action?: "wake" | "agent";
  // ... 其他字段扁平
};
```

### 3.3 Go 修复方案

```go
type HookMappingConfig struct {
    ID    string             `json:"id,omitempty"`
    Match *HookMappingMatch  `json:"match,omitempty"`  // 嵌套
    // 保留扁平字段作兼容 fallback
    MatchPath   string `json:"matchPath,omitempty"`
    MatchSource string `json:"matchSource,omitempty"`
    // ...
}

type HookMappingMatch struct {
    Path   string `json:"path,omitempty"`
    Source string `json:"source,omitempty"`
}
```

在 `ResolveHookMappings` 中: `match.path` 优先于 `matchPath`。

---

## 四、D-3 + D-4: resolveSessionKey 完整逻辑

### 4.1 问题

Go `httputil.go` 的 `ResolveSessionKey`:

```go
func ResolveSessionKey(sessionKey, channel string) string {
    if sessionKey != "" { return sessionKey }
    if channel != ""    { return channel }
    return "default"
}
```

TS `http-utils.ts` L65-79 (`resolveSessionKey`):

```typescript
function resolveSessionKey(params: {
  req: IncomingMessage;
  agentId: string;
  user?: string;
  prefix: string;
}): string {
  // 1. 显式 header
  const explicit = getHeader(req, "x-openacosmi-session-key")?.trim();
  if (explicit) return explicit;

  // 2. 构建 mainKey
  const user = params.user?.trim();
  const mainKey = user
    ? `${prefix}-user:${user}`     // 有用户: "prefix-user:alice"
    : `${prefix}:${randomUUID()}`; // 无用户: "prefix:uuid"

  // 3. buildAgentMainSessionKey
  return buildAgentMainSessionKey({ agentId, mainKey });
  // → "agent:main:prefix-user:alice"
}
```

### 4.2 依赖: `buildAgentMainSessionKey`

`routing/session-key.ts` L130-137:

```typescript
function buildAgentMainSessionKey(params: {
  agentId: string;
  mainKey?: string;
}): string {
  const agentId = normalizeAgentId(params.agentId);  // 小写+清理
  const mainKey = normalizeMainKey(params.mainKey);    // 小写, 默认 "main"
  return `agent:${agentId}:${mainKey}`;
}
```

### 4.3 session-key 模块完整功能

`routing/session-key.ts` (263 行) 包含:

| 函数 | 用途 | Phase 5 需要 |
| ---- | ---- | ------------ |
| `normalizeAgentId` | 清理 agent ID (小写, 去非法字符) | ✅ |
| `normalizeMainKey` | 清理 mainKey | ✅ |
| `buildAgentMainSessionKey` | 构建 `agent:{id}:{mainKey}` | ✅ |
| `buildAgentPeerSessionKey` | 构建 DM/group session key | Phase 6+ |
| `resolveAgentIdFromSessionKey` | 从 session key 提取 agent ID | Phase 5 |
| `classifySessionKeyShape` | 分类 key 格式 | Phase 5 |
| `toAgentStoreSessionKey` | 请求 key → 存储 key | Phase 5 |

### 4.4 Go 待建文件

建议创建 `backend/internal/routing/session_key.go`:

- `NormalizeAgentID(s string) string`
- `NormalizeMainKey(s string) string`
- `BuildAgentMainSessionKey(agentId, mainKey string) string`

然后更新 `httputil.go` 的 `ResolveSessionKey` 引用此模块。

---

## 五、D-5: Channel 动态插件注册

### 5.1 当前状态

Go `hooks.go` 硬编码:

```go
var validHookChannels = map[string]bool{
    "last": true, "new": true, "background": true,
}
```

TS 从 channel 插件注册表动态构建 (Phase 5+ 的 channel 插件系统)。

### 5.2 建议

Phase 5 实现 channel 插件注册后, 替换为:

```go
func RegisterHookChannel(name string) {
    validHookChannels[name] = true
}
```

---

## 六、D-6: validateAction

### 6.1 TS 实现

`hooks-mapping.ts` L302-313:

```typescript
function validateAction(action: HookAction): HookMappingResult {
  if (action.kind === "wake") {
    if (!action.text?.trim()) {
      return { ok: false, error: "hook mapping requires text" };
    }
    return { ok: true, action };
  }
  if (!action.message?.trim()) {
    return { ok: false, error: "hook mapping requires message" };
  }
  return { ok: true, action };
}
```

### 6.2 Go 现状

`buildMappingResult` 内联检查了 agent 的 message 为空, 但 wake 的 text 为空时有 fallback ("Webhook received"), 语义不同但可接受。建议 Phase 5 提取为独立函数以匹配 TS 代码结构。

---

## 七、Go 当前文件状态速查

| 文件 | 行数 | Phase 2 修改摘要 |
| ---- | ---- | --------------- |
| `hooks_mapping.go` | 428 | renderOptional + templateValueToString + Gmail preset |
| `hooks.go` | 243 | allowUnsafeExternalContent |
| `protocol.go` | 267 | PresenceEntry 16字段 + StateVersion + SessionDefaults |
| `server_http.go` | 301 | wake mode 响应 |
| `reload.go` | 541 | ResolveSettings 回调 |
| `httputil.go` | 92 | 待 Phase 5 增强 |
