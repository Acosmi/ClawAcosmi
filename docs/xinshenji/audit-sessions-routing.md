---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/{sessions, session, routing} (17 files, ~1600 LOC)
verdict: Pass with Notes
---

# 审计报告: sessions/session/routing — 会话与路由层

## 范围

- `backend/internal/sessions/` — 13 files (会话存储、元数据、重置、分组、路径)
- `backend/internal/session/` — 1 file (会话类型定义)
- `backend/internal/routing/` — 3 files (session key 管理、绑定)

## 审计发现

### [WARN] 正确性: SessionOrigin 类型重复定义

- **位置**: `sessions/store.go:18-28` vs `session/types.go:112-121`
- **分析**: `SessionOrigin` 在 `sessions` 和 `session` 两个包中各定义了一份，字段基本一致但命名微异（`AccountID` vs `AccountId`）。这可能导致序列化/反序列化行为不一致。
- **风险**: Medium
- **建议**: 统一到一处定义，消除类型冗余。

### [WARN] 正确性: FullSessionEntry vs SessionEntry 字段不同步

- **位置**: `sessions/store.go:57-140` vs `session/types.go:9-109`
- **分析**: 两个结构体都表示会话条目但有差异：`FullSessionEntry` 使用 `*int`/`*bool`/`*int64` 指针类型，`SessionEntry` 使用值类型。字段名也有差异（`SessionID` vs `SessionId`）。需要确认是有意设计（持久化 vs 运行时）还是偏移。
- **风险**: Medium
- **建议**: 如果是同一概念的两种表示，添加文档说明映射关系。

### [PASS] 资源安全: SessionStore 并发保护 (sessions/store.go)

- **位置**: `store.go:197-307`
- **分析**: `SessionStore` 使用 `sync.RWMutex` 保护：`LoadAll` 用 RLock 检查缓存，`Save`/`Update` 用 Lock。`Update` 方法实现了原子读-改-写（锁内读 → mutator → 锁内写）。缓存通过 mtime + TTL 双重校验失效。
- **风险**: None

### [PASS] 正确性: 原子文件写入 (sessions/store.go)

- **位置**: `store.go:275-284`
- **分析**: `Save` 和 `saveToDiskUnlocked` 都使用 tmp + rename 原子写入，文件权限 0600。临时文件名包含 PID + UUID 前缀避免冲突。失败时清理临时文件。
- **风险**: None

### [WARN] 正确性: Save 使用 0755 创建目录 vs 0700

- **位置**: `store.go:266`
- **分析**: `Save` 使用 `os.MkdirAll(dir, 0o755)`，而配置和设备身份模块统一使用 `0o700`。会话数据包含敏感信息（token 使用量、认证覆盖等），目录权限应更严格。
- **风险**: Low
- **建议**: 将 `0o755` 改为 `0o700`。

### [PASS] 正确性: UUID v4 生成 (sessions/store.go)

- **位置**: `store.go:181-193`
- **分析**: `generateUUID` 正确设置了 v4 版本位（`b[6] = (b[6] & 0x0f) | 0x40`）和 variant 2（`b[8] = (b[8] & 0x3f) | 0x80`）。使用 `crypto/rand.Read`。
- **风险**: None

### [PASS] 正确性: Session Key 解析与构建 (routing/session_key.go)

- **位置**: `session_key.go:53-318`
- **分析**: Session key 体系设计清晰：
  - 格式: `agent:<agentId>:<rest>`
  - 标准化: `NormalizeAgentID/AccountID` 使用正则校验 + fallback 清洗
  - 分类: `ClassifySessionKeyShape` 区分 agent/legacy/malformed
  - DM scope: 支持 `main/per-peer/per-channel-peer/per-account-channel-peer` 四级粒度
  - Identity links: `resolveLinkedPeerID` 支持跨渠道身份关联
- **风险**: None

### [PASS] 正确性: Agent ID 标准化安全 (routing/session_key.go)

- **位置**: `session_key.go:132-152`
- **分析**: `NormalizeAgentID` 限制长度 ≤ 64 字符，仅允许 `[a-z0-9_-]` 字符，去除前后 `-`。空值回退到 `"main"`。路径安全（可用作文件名/目录名）。
- **风险**: None

### [PASS] 正确性: Thread Session Key (routing/session_key.go)

- **位置**: `session_key.go:390-430`
- **分析**: `ResolveThreadSessionKeys` 通过 `:thread:` 后缀构建线程 session key。`ResolveThreadParentSessionKey` 使用 `strings.LastIndex` 查找 `:thread:` 或 `:topic:` 标记取父级。正确处理了嵌套线程场景。
- **风险**: None

## 总结

- **总发现**: 9 (6 PASS, 3 WARN, 0 FAIL)
- **阻断问题**: 无
- **建议**:
  1. 统一 `SessionOrigin` 类型定义，消除冗余 (Medium)
  2. 明确 `FullSessionEntry` vs `SessionEntry` 的关系和映射 (Medium)
  3. 会话存储目录权限应从 `0755` 改为 `0700` (Low)
- **结论**: **通过（附注释）** — Session key 体系设计优秀，存储层并发安全措施到位。主要问题在于类型定义冗余。
