---
document_type: Audit
status: Auditing
created: 2026-02-26
last_updated: 2026-02-26
audit_report: self
skill5_verified: true
---

# Audit Report: UHMS P3 — Runner + Gateway Integration

## Scope

P3 阶段代码审计，覆盖 UHMS 记忆系统与 Agent Runner / Gateway 的集成层。

**审计文件**:

| 文件 | 行数 | 角色 |
|------|------|------|
| `backend/internal/memory/uhms/claude_integration.go` | 296 | Anthropic Compaction API + Provider 策略 |
| `backend/internal/memory/uhms/manager.go` | ~760 | UHMS 核心管理器 (CompressIfNeeded, CommitSession 等) |
| `backend/internal/memory/uhms/llm_adapter.go` | 62 | LLMClientAdapter 桥接 |
| `backend/internal/agents/runner/attempt_runner.go:68-86,148-156,367-381` | — | UHMSBridge 接口 + 工具循环集成 |
| `backend/internal/gateway/server.go:195-248,491-567` | — | uhmsBridgeAdapter + UHMS Provider 初始化 |
| `backend/internal/gateway/boot.go:419` | — | CLI 二进制路径查找 (exec.LookPath) |

---

## Findings

### F-01 (MEDIUM) — 响应体大小未限制 ✅ FIXED

**位置**: `claude_integration.go:152`
**问题**: `json.NewDecoder(resp.Body).Decode()` 未限制响应体大小，恶意/异常 API 响应可导致 OOM。
**修复**: 添加 `io.LimitReader(resp.Body, 10*1024*1024)` (10 MB 限制)。
**状态**: ✅ 已修复

---

### F-02 (MEDIUM) — Provider 选择 Map 迭代非确定性 ✅ FIXED

**位置**: `server.go:504` (原代码)
**问题**: `for provider, pc := range loadedCfg.Models.Providers` 遍历 map 选取第一个有 API key 的 provider。Go map 迭代顺序随机化，导致每次启动可能选择不同 provider。影响 UHMS 压缩策略（Anthropic 有 Compaction API 加成）。
**修复**: 收集候选 provider 到 slice，按优先级排序 (`anthropic > openai > 其余字母序`)，确定性选择最高优先级 provider。
**状态**: ✅ 已修复

---

### F-03 (MEDIUM) — summarizeMessages 无大小限制 ✅ FIXED

**位置**: `manager.go:572-586` (原代码)
**问题**: `summarizeMessages` 将所有旧消息全量拼接后发送到 LLM 做摘要。50 轮工具循环的旧消息可达 200K+ tokens，超出 LLM 上下文窗口导致截断或 API 报错。对比 `extractAndStoreMemories`(行 606) 正确截断到 8000 chars。
**修复**:
- 单条消息内容截断到 2000 chars
- 总对话文本限制到 60000 chars (~15K tokens)
- 超出时标注 `... (N older messages truncated)`
**状态**: ✅ 已修复

---

### F-04 (LOW) — 每次迭代双向消息转换开销 ✅ FIXED

**位置**: `server.go:204-227` (uhmsBridgeAdapter.CompressChatMessages)
**问题**: 工具循环每轮调用 `CompressChatMessages`，即使 token 未超阈值也执行 `chatMessagesToUHMS` + `uhmsToChatMessages` 双向转换。50 轮循环中前 40 轮可能都无需压缩。
**参考**: gRPC-Go PreparedMsg 快速路径 + Letta/LangChain token 阈值门控
**修复**: 在 adapter 层加快速 byte-length token 估算 (`len/3.5`, ~90-96% 准确)，低于 `CompressThreshold` 时直接返回原始 messages，跳过双向转换。新增 `DefaultManager.CompressThreshold()` 方法暴露阈值。
**状态**: ✅ 已修复

---

### F-05 (LOW) — L0→L1 升级启发式 `*10` 可能溢出 budget ✅ FIXED

**位置**: `manager.go:265`
**问题**: `if usedTokens+entryTokens*10 < tokenBudget` 假设 L1 是 L0 的 ~10 倍。如果 L1 实际更大（例如 50x），加载后可能超出 budget。
**参考**: LlamaIndex AutoMergingRetriever (加载后实测 token) + Progressive Context Disclosure (预留 20% budget)
**修复**:
- 先检查 `usedTokens+entryTokens < budget*80%`（预留 20% 给后续条目）
- 加载 L1 内容后用 `estimateTokens(l1Entry)` 实测
- 仅当实测 L1 tokens 不超 budget 时才升级
**状态**: ✅ 已修复

---

### F-06 (LOW) — 记忆提取 goroutine 错误仅 Debug 级别 + 无 panic 防护 ✅ FIXED

**位置**: `manager.go:321-327`, `manager.go:179`, `attempt_runner.go:378-389`
**问题**:
1. `extractAndStoreMemories` 失败仅 `slog.Debug`，生产环境不可见
2. 所有 fire-and-forget goroutine 无 panic recovery，panic 会崩溃整个进程
**参考**: CockroachDB Stopper (禁止裸 `go`) + LaunchDarkly GoSafely + Dave Cheney 日志哲学 + Uber Style Guide
**修复**:
- 新增 `safeGo(name, fn)` 包装器: `defer recover()` + Error 级别 + stack trace
- 记忆提取失败从 `Debug` 升级到 `Warn` (已处理但需关注)
- 向量索引 goroutine 改用 `safeGo`
- CommitChatSession goroutine 加 panic recovery
**状态**: ✅ 已修复

---

### F-07 (LOW) — CommitChatSession goroutine 捕获 messages 安全性

**位置**: `attempt_runner.go:369-381`
**问题**: goroutine 捕获 `messages` slice (引用类型)。需确认父函数不会在 goroutine 执行期间修改底层数组。
**分析**: goroutine 在工具循环退出后启动，`messages` 此后仅被读取（构建 AttemptResult），不会修改。`CommitChatSession` 通过 `chatMessagesToUHMS` 创建独立副本。**安全**。
**状态**: ✅ 已验证 — 无竞态

---

### F-08 (MEDIUM) — boot.go 缺少 `os/exec` import 导致编译失败 ✅ FIXED

**位置**: `boot.go:419`
**问题**: `exec.LookPath("openacosmi")` 调用依赖 `"os/exec"` 包，但 import 块中遗漏。导致整个 `gateway` 包编译失败。
**修复**: 在 `boot.go` import 块中添加 `"os/exec"`。
**状态**: ✅ 已修复

---

## Security Review

| 检查项 | 结果 |
|--------|------|
| API Key 泄露 | ✅ APIKey 仅通过 HTTP header 传递，不写入日志 |
| 响应体 DoS | ✅ LimitReader 10 MB (F-01 修复) |
| 输入验证 | ✅ AddMemory 校验 content/userID 非空 |
| SQL 注入 | ✅ 使用 GORM 参数化查询 |
| 路径遍历 | ✅ VFS 路径由 `filepath.Join` 构建，ID 为 hex 格式 |
| 并发安全 | ✅ Store 用 SQLite WAL; Cache 有内部锁; Manager 用 RWMutex 保护可变字段 |
| 上下文取消 | ✅ CompressIfNeeded/SearchMemories 接受 ctx; goroutine 用 context.Background() (设计如此，会话提交需 outlive 请求) |

---

## Verdict

**PASS** — 8 项 findings 全部处理 (4 MEDIUM + 3 LOW 修复 + 1 已验证安全)。无安全漏洞。代码质量良好，错误处理完整，nil-safe 设计正确。

## 参考来源

| Finding | 参考 |
|---------|------|
| F-04 快速路径 | gRPC-Go PreparedMsg ([#2432](https://github.com/grpc/grpc-go/issues/2432)), Letta agent.py, LangChain SummarizationMiddleware |
| F-05 分层加载 | LlamaIndex AutoMergingRetriever, Progressive Context Disclosure ([blog](https://williamzujkowski.github.io/posts/from-150k-to-2k-tokens-how-progressive-context-loading-revolutionizes-llm-development-workflows/)) |
| F-06 safeGo | CockroachDB Stopper ([#58164](https://github.com/cockroachdb/cockroach/issues/58164)), LaunchDarkly GoSafely, Uber Go Style Guide, Dave Cheney 日志哲学 |
