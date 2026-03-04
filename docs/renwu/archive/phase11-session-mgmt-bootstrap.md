# 模块 B: Session 管理 — 审计 Bootstrap

> 用于新窗口快速恢复上下文

---

## 新窗口启动模板

```
请执行 Session 管理模块的重构健康度审计。

## 上下文
1. 读取审计总表: `docs/renwu/refactor-health-audit-task.md`
2. 读取本 bootstrap: `docs/renwu/phase11-session-mgmt-bootstrap.md`
3. 读取 `/refactor` 技能工作流
4. 读取编码规范: `skills/acosmi-refactor/references/coding-standards.md`
5. 读取 `docs/renwu/deferred-items.md`
6. 控制输出量：预防上下文过载引发崩溃，需要大量输出时请逐步分段输出。
7. 任务完成后：请按要求更新 `refactor-plan-full.md` 和本模块的审计报告。

## 目标
对比 TS 原版的 Session 管理机制与 Go 移植，重点审计存储模型差异（磁盘 vs 内存）。

> **注意**: 具体审计步骤请严格参考 `docs/renwu/refactor-health-audit-task.md` 模块 B 章节。此文档仅提供上下文和文件索引。
```

---

## TS 源文件清单

| 文件 | 大小 | 职责 |
|------|------|------|
| `src/gateway/session-utils.ts` | 22KB | Session 加载、列表、查询、模型解析 |
| `src/gateway/session-utils.fs.ts` | ~15KB | Transcript 文件读写、预览提取 |
| `src/gateway/session-utils.types.ts` | ~5KB | 类型定义 |
| `src/config/sessions.ts` → `src/config/sessions/` | 13子文件 | SessionStore 磁盘持久化、SessionEntry 类型 |
| `src/gateway/chat-sanitize.ts` | ~5KB | 消息 envelope 清理 |
| `src/routing/session-key.ts` | ~3KB | Session key 解析 (`agent:xxx:main`) |

## Go 对应文件

| 文件 | 大小 | 对应 TS |
|------|------|---------|
| `gateway/sessions.go` | 3KB | SessionStore (⚠️ 纯内存) |
| `gateway/session_utils.go` | 7KB | `session-utils.ts` 部分 |
| `gateway/session_utils_fs.go` | 13KB | `session-utils.fs.ts` |
| `gateway/session_utils_types.go` | 6KB | 类型定义 |
| `gateway/transcript.go` | 9KB | Transcript 读写 |
| `agents/session/` | 6 文件 | SessionEntry 类型 |

## 关键审计点

1. **存储模型差异** ⚠️ 最关键:
   - TS: `loadSessionStore()` 从磁盘 JSON 文件读取 (`{storePath}`)
   - Go: `SessionStore` 是纯内存 `map[string]*SessionEntry`，启动即空
   - 需要: 实现磁盘持久化或启动时预加载

2. **Session Key 规范化**:
   - TS: `resolveSessionStoreKey()` 做复杂的 key 规范化 (`agent:xxx:main`)
   - Go: 是否等价实现？

3. **Transcript 路径解析**:
   - TS: `path.join(path.dirname(storePath), sessionId + ".jsonl")`
   - Go: `ResolveTranscriptPath()` 是否一致？

4. **Session 合并逻辑**:
   - TS: `loadCombinedSessionStoreForGateway()` 合并多 agent 的 store
   - Go: 是否实现？

## 已知问题

- `chat.send` 中已添加 session 自动创建（临时修复，审计应确认是否需要磁盘持久化）
- `AppendUserTranscriptMessage` 已添加
