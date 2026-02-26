---
document_type: Audit
status: PASS (with fixes applied)
created: 2026-02-25
last_updated: 2026-02-25
component: skill-store-bridge (Phase 1 REST)
scope: 7 files modified/created
---

# Audit Report: Skill Store Bridge Phase 1

## Scope

对 OpenAcosmi ↔ nexus-v4 技能商店桥接 Phase 1 (REST API) 的全部代码进行逐行审计。

### 审计范围

| 文件 | 类型 | 行数 | 审计 |
|------|------|------|------|
| `pkg/types/types_skills.go` | 修改 | 40 | Full |
| `internal/agents/skills/skill_store_client.go` | **新建** | 220 | Full |
| `internal/agents/skills/skill_store_sync.go` | **新建** | 256 | Full |
| `internal/gateway/server_methods_skills.go` | 修改 | 620 | 新增部分 Full |
| `internal/gateway/server_methods.go` | 修改 | 380 | 变更部分 Full |
| `internal/gateway/ws_server.go` | 修改 | 512 | 变更部分 Full |
| `internal/gateway/server.go` | 修改 | 820 | 变更部分 Full |

---

## 审计发现

### F-01: ZIP Path Traversal [CRITICAL → FIXED]

**Location:** `skill_store_sync.go:extractSkillFromZip()`
**Risk:** Critical — 远程代码执行 / 任意文件写入
**Issue:** `filepath.Dir(f.Name)` 直接使用 ZIP 条目路径，恶意 ZIP 可包含 `../../etc/` 路径实现目录遍历。
**Fix Applied:**
- 添加 `filepath.Clean()` + `filepath.IsLocal()` 校验，拒绝非本地路径
- 所有 `f.Name` 在使用前先经过 `cleanPath` 清洗
- 添加单文件大小限制 (5 MB)
- 使用 `io.LimitReader` 防止解压炸弹

**Verification:** 修复后，包含 `../` 的 ZIP 条目将触发 `"zip contains unsafe path"` 错误。

---

### F-02: HTTPS Not Enforced [CRITICAL → FIXED]

**Location:** `skill_store_client.go:NewSkillStoreClient()` + `Available()`
**Risk:** Critical — 中间人攻击可窃取 JWT token
**Issue:** 注释要求 HTTPS 但代码未校验，允许 HTTP URL 导致 token 明文传输。
**Fix Applied:**
- `Available()` 方法强制校验 `https://` 前缀
- 允许本地开发例外: `http://localhost` / `http://127.0.0.1`
- 不满足 HTTPS 要求时 `Available()` 返回 false，所有 API 调用被阻断

**Verification:** `Available()` 对 `http://evil.com` 返回 false。

---

### F-03: ZIP Size Limit Missing [HIGH → FIXED]

**Location:** `skill_store_sync.go:PullSkillToLocal()`
**Risk:** High — DoS（内存耗尽）
**Issue:** 无 ZIP 大小限制，恶意服务端可返回超大 ZIP 消耗内存。
**Fix Applied:**
- 添加 `maxZipSize = 10 MB` 总包限制
- 添加单文件 `UncompressedSize64 > 5 MB` 限制
- `io.LimitReader` 双重保护

**Verification:** 超过 10 MB 的 ZIP 触发 `"zip too large"` 错误。

---

### F-04: skillKey Input Validation [MEDIUM → ACCEPTED]

**Location:** `server_methods_skills.go:handleSkillsUpdate()` (L304-316)
**Risk:** Medium — config 文件注入
**Issue:** `skillKey` 参数未校验路径遍历字符（`/`, `\`, `..`）。
**Status:** **既有代码**，非本次变更引入。本次 P1 新增的 `skills.store.pull` 通过 `sanitizeKey()` 已防护。
**Recommendation:** 后续独立 PR 修复 `handleSkillsUpdate` 中的 skillKey 校验。

---

### F-05: Token Logging Risk [LOW → MITIGATED]

**Location:** `server.go:L540`
**Risk:** Low — 信息泄露
**Issue:** `slog.Info("gateway: skill store client configured", "url", store.URL)` — URL 被记录到日志。
**Mitigation:** Token 存储在独立字段 `store.Token`，不包含在 URL 中。日志仅记录 URL，不记录 token。
**Status:** 已验证安全，无需修改。

---

### F-06: TOCTOU Race in IsNew Detection [LOW → ACCEPTED]

**Location:** `skill_store_sync.go:L49-52`
**Risk:** Low — 逻辑正确性
**Issue:** `os.Stat()` 检查文件是否存在，随后 `os.WriteFile()` 写入。两步之间有竞态窗口。
**Impact:** 仅影响 `IsNew` 标志的准确性，不影响功能或安全。
**Status:** 可接受。批量拉取是串行执行，实际并发竞态概率极低。

---

## 安全检查清单

### 输入验证
- [x] HTTP API 参数（category, keyword）: 仅传递到 URL query，使用 `url.Values` 编码 ✅
- [x] skillIds 参数: 类型校验 `[]string`，逐个 TrimSpace ✅
- [x] page 参数: switch-case 白名单 ✅
- [x] skill ID: `url.PathEscape()` 编码 ✅
- [x] ZIP 路径: `filepath.IsLocal()` + `filepath.Clean()` ✅
- [x] 技能 key: `sanitizeKey()` 仅保留 `[a-z0-9-]` ✅

### 资源安全
- [x] HTTP 响应 body: `defer resp.Body.Close()` ✅
- [x] ZIP 文件句柄: `rc.Close()` 在 ReadAll 后 ✅
- [x] ZIP 大小限制: 10 MB 总包 + 5 MB 单文件 ✅
- [x] HTTP 超时: 30s ✅
- [x] 无 goroutine 泄漏（同步调用模式）✅

### 权限模型
- [x] `skills.store.browse` → readMethods ✅
- [x] `skills.store.refresh` → readMethods ✅
- [x] `skills.store.link` → readMethods ✅
- [x] `skills.store.pull` → adminExactMethods ✅
- [x] 与现有权限体系一致 ✅

### 错误处理
- [x] 所有 error 包装上下文 (`fmt.Errorf("...: %w", err)`) ✅
- [x] 错误消息不泄露 token ✅
- [x] client 不可用时返回 `ErrCodeServiceUnavailable` ✅
- [x] config 不可用时返回 `ErrCodeInternalError` ✅

### 并发安全
- [x] `SkillStoreClient` 无内部可变状态，天然线程安全 ✅
- [x] `BatchPull` 串行执行，无并发冲突 ✅
- [x] `GatewayMethodContext.SkillStoreClient` 只读引用 ✅

---

## 代码质量

| 指标 | 结果 |
|------|------|
| `go build ./...` | PASS ✅ |
| `go vet ./...` | PASS ✅ |
| `go test ./internal/agents/skills/...` | 5/5 PASS ✅ |
| 未使用导入 | 0 ✅ |
| 导出类型文档 | 所有导出类型均有注释 ✅ |
| 风格一致性 | 与现有代码风格一致 ✅ |

---

## Verdict: PASS

所有 Critical/High 发现已修复并验证。1 个 Medium 发现属于既有代码，已记录为后续修复项。

**修复统计:** 3 个发现已修复 (F-01 Critical, F-02 Critical, F-03 High), 2 个已接受 (F-04 Medium, F-06 Low), 1 个已缓解 (F-05 Low)。
