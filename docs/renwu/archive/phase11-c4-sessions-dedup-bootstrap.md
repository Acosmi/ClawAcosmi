# Phase 11 C4: `internal/sessions/` 类型与函数去重 — 新窗口上下文

> 本文件提供独立窗口执行 C4 清理任务所需的全部上下文。

## 任务目标

将 `internal/sessions/` 包中与 `gateway/session_utils.go`、`routing/session_key.go`、`internal/session/types.go` 重复的类型和函数消除，减少代码冗余。

## 关键发现

> [!TIP]
> **无外部 import** — 扫描确认 `internal/sessions` 包没有被 `internal/sessions` 以外的任何 Go 包 import。所有外部代码都直接使用 `internal/session`（单数）或 `gateway` 包。

## 文件清单

**`internal/sessions/` (8 源 + 7 test)**：

| 文件 | 行数 | 内容 | 操作建议 |
|------|------|------|----------|
| `sessions.go` | 199 | 精简 `SessionEntry`(5字段) + `SessionKind` + `GroupKeyParsed` + 8 函数 | **删除重复，保留独有** |
| `sessions_test.go` | 133 | 上述函数的测试 | 迁移到对应包的测试文件 |
| `main_session.go` | 149 | `SessionScopeConfig` + `AgentListEntry` + 4 函数 | **保留**（独有功能） |
| `main_session_test.go` | — | 上述函数测试 | 保留 |
| `group.go` | — | 群组会话解析 | 检查是否与 `gateway` 重复 |
| `metadata.go` | — | 会话元数据 | 检查独有性 |
| `paths.go` | — | 会话路径解析 | 检查独有性 |
| `reset.go` | — | 会话重置逻辑 | 检查独有性 |
| `store.go` | — | 独立 store 实现 | 检查是否与 `gateway/sessions.go` 重复 |
| `transcript.go` | — | 转录文件管理 | 检查是否与 `gateway/transcript.go` 重复 |

## 类型冲突

```go
// internal/sessions/sessions.go — 精简版（5 字段）
type SessionEntry struct {
    SessionID   string `json:"sessionId,omitempty"`
    DisplayName string `json:"displayName,omitempty"`
    Subject     string `json:"subject,omitempty"`
    ChatType    string `json:"chatType,omitempty"`
    UpdatedAt   *int64 `json:"updatedAt,omitempty"`  // ← 指针
}

// internal/session/types.go — 完整版（50+ 字段）
type SessionEntry struct {
    SessionKey  string `json:"sessionKey"`
    // ... 50+ 字段 ...
    UpdatedAt   int64  `json:"updatedAt,omitempty"`  // ← 值类型
}
```

## 函数重复对照

| `sessions.` 函数 | 等效实现 | 差异 |
|---|---|---|
| `DeriveSessionTitle(entry, msg)` | `gateway.DeriveSessionTitle(entry, msg)` | sessions 版用 `*int64` 的 `FormatSessionIDPrefix` |
| `ClassifySessionKey(key, entry)` | `gateway.ClassifySessionKey(key, entry)` | gateway 版返回 `string`，sessions 版返回 `SessionKind` |
| `ParseGroupKey(key)` | `gateway.ParseGroupKey(key)` | 类型 `GroupKeyParsed` vs `GroupKeyParts`，逻辑相同 |
| `IsCronRunSessionKey(key)` | `gateway.IsCronRunSessionKey(key)` | 完全相同 |
| `ParseAgentSessionKey(key)` | `gateway.parseAgentSessionKeySimple(key)` | 公开 vs 私有，类型 `AgentSessionKey` vs `agentKeyParsed` |
| `NormalizeMainKey(key)` | `gateway.NormalizeMainKey` / `routing.NormalizeMainKey` | sessions 版多处理 `"default"` → `"main"` |
| `NormalizeAgentID(id)` | `gateway.NormalizeAgentId(id)` | 完全相同 |
| `TruncateTitle(text, max)` | `gateway.TruncateTitle(text, max)` | sessions 版用 `utf8.RuneCountInString` |

## 执行步骤建议

1. **确认包内引用**：检查 `main_session.go`、`group.go`、`store.go` 等文件是否引用 `sessions.SessionEntry`
2. **替换 `SessionEntry`**：将包内用到精简版的地方改为使用 `session.SessionEntry`（注意 `UpdatedAt` 从 `*int64` → `int64`）
3. **删除重复函数**：`sessions.go` 中的 8 个函数，保留在 `sessions_test.go` 中的测试迁移到 `gateway/session_utils_test.go`
4. **检查独有文件**：`group.go`、`metadata.go`、`paths.go`、`reset.go`、`store.go`、`transcript.go` — 如有独有逻辑则保留
5. **编译验证**：`go build ./...` + `go vet ./...` + `go test -race ./...`

## 风险点

- `UpdatedAt` 类型变更：从 `*int64`→`int64`，所有 nil 判断需改为 `> 0` 判断
- `sessions.NormalizeMainKey` 额外处理了 `"default"` → `"main"`，需确认其他两个版本是否需要同步
- `SessionKind` 类型 vs 裸 `string` — 如果保留 `SessionKind`，需要放到共享位置

## 参考文件

- **主跟踪**: [phase11-fix-task.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase11-fix-task.md) → C4
- **延迟待办**: [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) → P11-B5
- **Go 源码**:
  - [sessions/sessions.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/sessions/sessions.go) (待清理)
  - [session/types.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/session/types.go) (完整版)
  - [gateway/session_utils.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/session_utils.go) (等效函数)
  - [gateway/sessions.go](file:///Users/fushihua/Desktop/Claude-Acosmi/backend/internal/gateway/sessions.go) (SessionStore)
