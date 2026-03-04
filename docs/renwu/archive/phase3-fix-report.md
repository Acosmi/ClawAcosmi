# Phase 3.1 审计修复报告

> 日期: 2026-02-12 | 范围: `internal/gateway`, `internal/infra`

## 修复统计

| 分类 | 数量 |
| ---- | ---- |
| P0 关键 Bug | 5 |
| 代码健康 | 2 (死代码+lint) |
| 新增模块 | 5 个文件 |
| 新增测试 | 14+ |

## 关键修复

### C1: SafeEqual 时间侧信道 (auth.go)

**问题**: 长度不等时 early return 泄露秘密长度。

```diff
-if len(a) != len(b) {
-    return false
-}
-return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
+maxLen := len(a)
+if len(b) > maxLen { maxLen = len(b) }
+if subtle.ConstantTimeEq(int32(len(a)), int32(len(b))) == 0 {
+    return false
+}
+padA, padB := make([]byte, maxLen), make([]byte, maxLen)
+copy(padA, []byte(a)); copy(padB, []byte(b))
+return subtle.ConstantTimeCompare(padA, padB) == 1
```

### C2: ToolRegistry 并发 (tools.go)

添加 `sync.RWMutex`，Register 写锁，Get/List 读锁。

### C3: reconnect goroutine 泄露 (ws.go)

递归 `go c.reconnect()` → `for { select ... }` 循环。

### C4: ConfigWatcher timer 竞态 (reload.go)

timer 回调内加 `w.mu.Lock()` 防止与 `Notify()`/`Stop()` 竞争。

### C5: DisallowUnknownFields (tools.go)

移除 `decoder.DisallowUnknownFields()`，与 TS 行为一致。

## 新增模块

| 文件 | 行数 | TS 来源 |
| ---- | ---- | ---- |
| `gateway/agent_run_context.go` | 108 | agent-events.ts |
| `gateway/sessions.go` | 113 | sessions.ts |
| `gateway/channels.go` | 168 | channels.ts + tool-strategy.ts |
| `infra/system_events.go` | 161 | system-events.ts |
| `infra/heartbeat_wake.go` | 140 | heartbeat-wake.ts |

## 验证

```
go build ./...                           ✅
go vet ./internal/gateway/... ./internal/infra/...  ✅
go test -race ./internal/gateway/...     ✅ PASS (1.284s)
go test -race ./internal/infra/...       ✅ PASS (2.790s)
```

---

# Phase 3.2 深度审计修复报告

> 日期: 2026-02-12 | 范围: `internal/gateway/chat.go`

## 修复统计

| 分类 | 数量 |
| ---- | ---- |
| 逻辑 Bug | 3 (verbose 映射 + 心跳抑制 + buffer key) |
| Lint 修复 | 2 (S1009 + tool verbose 适配) |
| 误报纠正 | 2 (TTL 清理 + Remove 逻辑) |

## Bug 修复

### BUG-1: NormalizeVerboseLevel 映射不一致 (chat.go)

Go 返回 `"compact"` 而 TS 返回 `"on"`；未知输入 Go 返回 `"compact"` 而 TS 返回 undefined。

修复：对齐 TS `auto-reply/thinking.ts:normalizeVerboseLevel`，返回 `"off"|"on"|"full"|""`。

### BUG-3: ShouldSuppressHeartbeat 未集成 (chat.go)

方法已存在但 `handleDelta`/`handleFinal` 从未调用。修复：在广播前插入 suppress 检查。

### BUG-5: Buffer key 使用 sessionKey (chat.go)

TS 使用 `clientRunId`，Go 使用 `sessionKey`，导致同 session 并发 run 互相覆盖 buffer。
修复：所有 `Buffers`/`DeltaSentAt` 操作改用 `runID` 作 key。

## 验证

```
go build ./...                           ✅
go vet ./internal/gateway/... ./internal/infra/...  ✅ 0 warnings
go test -race ./internal/gateway/...     ✅ PASS (1.283s)
go test -race ./internal/infra/...       ✅ PASS (2.789s)
```
