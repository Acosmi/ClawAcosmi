---
document_type: Tracking
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: Pending
skill5_verified: true
---

# 权限与审批系统重新设计

> 综合：现有代码 Bug 分析 + 国际大厂最佳实践调研 + 新设计方案

---

## 一、现有 Bug 根因分析

### Bug 1：L2 降级到 L1 后仍然越权

**位置**：`server_methods_exec_approvals.go` → `handleExecApprovalsSet`

**根因**：`exec.approvals.set` 写入文件后，没有通知 `EscalationManager`。
若此时有活跃临时授权（`m.active != nil`，级别为 "full"），`GetEffectiveLevel()` 仍返回 "full"。

```
用户操作:
  1. 有活跃 escalation grant: level="full", expiresAt=+30min
  2. 用户在设置页把 base level 改为 "allowlist"
  3. handleExecApprovalsSet 写文件 → 成功
  4. 但 escMgr.active 仍是 "full" 的 grant (未撤销)
  5. 下次工具调用: SecurityLevelFunc() → GetEffectiveLevel() → 返回 "full"
  6. 权限未降级 ✗
```

**还有第二个问题**（AllowWrite/AllowExec 逻辑混乱）：

```go
// 当前代码 attempt_runner.go:282-284
AllowWrite: secLvl == "full" || secLvl == "allowlist",  // ← 两者都允许写!
AllowExec:  secLvl == "full" || secLvl == "allowlist",  // ← 两者都允许执行!
SandboxMode: secLvl == "allowlist",                     // ← 唯一区别只是沙箱
```

"allowlist" 和 "full" 在 AllowWrite/AllowExec 上完全相同，唯一差别是 SandboxMode。
若沙箱未运行或未正确配置，L1("allowlist") 与 L3("full") 实际行为无差异。

---

### Bug 2：L2（full 级）时仍弹出审批

**位置**：`server.go` → `OnPermissionDenied` 回调 (L796-808)

**根因**：`OnPermissionDenied` 自动调用 `RequestEscalation` 时，没有检查 base level 是否已经满足请求级别。

```go
// 当前代码 server.go:799-807
escLevel := "allowlist"
if tool == "bash" || tool == "write_file" {
    escLevel = "full"
}
// ← 没有先检查: if readBaseSecurityLevel() >= escLevel { return }
if err := escMgr.RequestEscalation(escId, escLevel, reason, ...); err != nil {
    slog.Debug("auto-escalation skipped (expected if already pending)", "error", err)
}
```

但 `RequestEscalation` 本身有检查 `m.active != nil`，如果有活跃 grant 会返回 error。
真正的问题是：**若 base level 已是 "full" 但工具因为 OS 级权限错误失败**，
`IsPermissionDeniedOutput` 可能会误判为应用层权限拒绝，然后触发不必要的审批流程。

---

### Bug 3：缺少 L2（沙箱内全权限）层级

**现有 3 层 vs 用户期望 4 层**：

| 用户期望 | 现有代码 | 是否存在 |
|---|---|---|
| L0: 只读 | `deny` | ✅ |
| L1: 工作区写 + 沙箱受限执行 | `allowlist` | ⚠️ 有但逻辑混乱 |
| L2: 沙箱内全权限 | ❌ 无 | **缺失** |
| L3: 裸机全权限 | `full` | ✅ |

现有 "allowlist" 层实际上是 L1（沙箱受限），
但 "full" 层没有沙箱（`SandboxMode=false`），直接等于 L3。
**L2（沙箱内全权限）完全缺失**。

---

## 二、国际大厂最佳实践（可信源验证）

### Online Verification Log

#### Claude Code 权限模型
- **Query**: Claude Code permission levels design sandboxing
- **Source**: https://www.anthropic.com/engineering/claude-code-sandboxing
- **Key finding**: 权限分为决策层（应用代码）和执行层（OS 内核/hypervisor）两层。
  内核层通过 macOS Seatbelt / Linux bubblewrap 强制执行，即使 Claude 被注入也无法绕过。
  网络通过本地 Unix socket 代理实现域名白名单，而非直接内核规则。
- **Verified date**: 2026-02-26

#### Google Cloud IAM PAM（即时访问）
- **Query**: Google Cloud IAM Privileged Access Manager temporary elevated access
- **Source**: https://cloud.google.com/iam/docs/temporary-elevated-access
- **Key finding**: JIT 模式：申请 → 人工审批 → 时限绑定授权 → 自动撤销。
  关键：TTL 到期自动撤销，无需人工记得撤销。支持二级审批链。
- **Verified date**: 2026-02-26

#### AWS Bedrock 代理三层权限架构
- **Query**: AWS Well-Architected AI agents least privilege GENSEC05
- **Source**: https://docs.aws.amazon.com/wellarchitected/latest/generative-ai-lens/gensec05-bp01.html
- **Key finding**: Permission Boundary = 硬上限 cap。有效权限 = Policy ∩ Boundary。
  即使 policy 配置错误放宽了权限，boundary 仍是绝对天花板，防止越权。
- **Verified date**: 2026-02-26

#### 沙箱执行平台 (E2B/Daytona/gVisor)
- **Query**: AI code execution sandbox permission tiers Firecracker gVisor
- **Source**: https://northflank.com/blog/how-to-sandbox-ai-agents
- **Key finding**: T4（Firecracker microVM）提供最高隔离（独立 kernel），
  适合不可信代码。L2 应在独立 VM kernel 内运行。权限降级 **必须通过重建进程/容器** 实现，
  不能在运行中修改权限。
- **Verified date**: 2026-02-26

#### MiniScope 最小权限框架（UC Berkeley）
- **Query**: MiniScope least privilege tool calling agents arXiv
- **Source**: https://arxiv.org/abs/2512.11147
- **Key finding**: 权限强制必须是机械性的（OS 级），而非基于 prompt 的建议。
  子智能体继承父级权限时，必须显式降到该任务需要的最低级别，
  不能自动继承父级的最高权限。
- **Verified date**: 2026-02-26

---

## 三、设计原则提炼（借鉴要点）

### 原则 1：层级是 OS 上下文，不是应用变量
> 来源：Anthropic Claude Code sandboxing + Linux capabilities 文档

权限层级不应存储为 Go 变量（`secLvl string`），而应通过进程/容器的安全上下文来体现。
层级转换 = 进程重建（新安全上下文），而非修改内存变量。

**当前代码的问题**：层级存在 `EscalationManager.active` 字段和 `exec-approvals.json` 文件中。
任何代码路径都可以绕过检查直接操作文件系统，没有 OS 级强制执行。

**短期务实方案**：在应用层做双重保证：
1. 应用层权限检查（现有逻辑，修复 Bug）
2. 对 L2 用专属沙箱上下文（已有 NativeSandbox）
3. 对 L3 明确标记为"裸机"并记录审计

### 原则 2：权限边界（Bounding Set）是绝对上限
> 来源：AWS IAM Permission Boundary + Linux capabilities(7)

设计一个 `PermissionBoundary`：即使临时授权被批准，也不能超过边界。
例如：当前 session 边界是 L2，则即使 agent 请求 L3，系统拒绝而非批准。

### 原则 3：降级必须撤销 active grants
> 来源：容器安全文档（capability bounding set 只能降不能升）

当 base level 降低时，高于新 base level 的 active grants 必须立即撤销。
对应当前 Bug 1 的修复。

### 原则 4：RequestEscalation 前检查是否已满足
> 来源：Progent fallback 机制

OnPermissionDenied 触发提权请求前，先检查 effective level 是否已满足需求。
若 base level 已是 "full"，工具失败是 OS 级错误而非权限问题，不应触发审批。

### 原则 5：交叉权限（requesting user ∩ agent）
> 来源：OSO HQ AI agent access control

代理只能行使 min(代理自身权限, 请求用户权限)，不能用代理权限绕过用户权限。
（当前系统无用户权限模型，暂记为 TODO）

---

## 四、新权限模型设计

### 4.1 四层模型定义

```
L0 — ReadOnly（只读）
  ExecSecurity: "deny"
  AllowWrite:   false
  AllowExec:    false
  SandboxMode:  false（无需沙箱，本身无危险操作）
  Network:      不涉及
  读权限:        工作区内只读（未来可扩展为 chroot 强制）
  升级路径:      → L1 需要审批

L1 — WorkspaceRestricted（工作区受限）
  ExecSecurity: "allowlist"
  AllowWrite:   true（工作区内）
  AllowExec:    true（仅白名单命令，其余 deny）
  SandboxMode:  true（Docker/NativeSandbox）
  Network:      关闭
  升级路径:      → L2 需要审批

L2 — SandboxedFull + 临时挂载区（沙箱内全权限，授权宿主机目录挂载）[新增]
  ExecSecurity: "sandboxed"（新增枚举值）
  AllowWrite:   true（沙箱内 + 已授权挂载区）
  AllowExec:    true（沙箱内任意命令）
  SandboxMode:  true（强制沙箱，内核边界隔离）
  Network:      无（--network=none）
  挂载:          工作区（固定）+ 用户审批的临时挂载路径（bind mount，随 TTL 撤销）
  TTL:          任务生命周期 + 5 min grace，挂载随 TTL 到期自动卸载
  升级路径:      → L3 需要 JIT 审批（TTL ≤ 60min）+ 工具操作审计

L3 — UnsandboxedFull（裸机全权限）
  ExecSecurity: "full"
  AllowWrite:   true（宿主机任意路径）
  AllowExec:    true（任意命令）
  SandboxMode:  false
  Network:      全开
  TTL:          最大 60 min，强制，不可延期
  审批:          JIT 人工审批，飞书/钉钉卡片，支持二级（未来）
  审计:          每次工具调用写 tool-audit.log（含 escalationId 关联，见第五节）
  降级:          任务完成立即降 → L1（不回 L2，保守策略）
```

### 4.2 网络策略（替代代理白名单）

网络代理白名单实现成本高，用以下策略替代：

```
L0  deny        → 无网络（AllowExec=false，根本不产生网络调用）
L1  allowlist   → 无网络（SandboxMode=true，Docker --network=none 或 network namespace 隔离）
L2  sandboxed   → 无网络（SandboxMode=true，Docker --network=none）[比代理更简单更安全]
L3  full        → 全网络 + 工具审计日志（见第五节）
```

**L2 网络隔离实现**：
- Docker 模式：`docker run --network=none ...`（一行参数，无需实现代理）
- NativeSandbox 模式：创建独立 network namespace（`unshare --net`），namespace 内无默认路由
- 语义：L2 是"沙箱内全文件系统权限，但不出网"。需要网络访问 → 升级到 L3（并触发 JIT 审批 + 审计）

**好处**：
1. L2 攻击面大幅缩小（无网络 = 无法外泄数据、无法反弹 shell）
2. 无需实现和维护代理服务
3. 用户理解更直接：L2=全本地, L3=全权限

---

### 4.4 新增枚举值

```go
// 在 exec_approvals.go 新增
ExecSecuritySandboxed ExecSecurity = "sandboxed"  // L2: 沙箱内全权限（无网络）

// 完整顺序（数值越大权限越高）
ExecSecurityDeny      = "deny"      // L0: 0
ExecSecurityAllowlist = "allowlist" // L1: 1
ExecSecuritySandboxed = "sandboxed" // L2: 2 [新增]
ExecSecurityFull      = "full"      // L3: 3

// 更新 MinSecurity 的 order map
order := map[ExecSecurity]int{
    ExecSecurityDeny:      0,
    ExecSecurityAllowlist: 1,
    ExecSecuritySandboxed: 2, // 新增
    ExecSecurityFull:      3,
}
```

### 4.3 修复后的工具执行参数逻辑

```go
// 修复 attempt_runner.go:282-284 的逻辑

switch secLvl {
case "deny":
    AllowWrite = false
    AllowExec  = false
    SandboxMode = false
case "allowlist":
    AllowWrite = true   // 工作区内（由 workspace dir 限制）
    AllowExec  = true   // 配合 Rules（白名单规则）
    SandboxMode = true
case "sandboxed":   // L2: 新增
    AllowWrite = true
    AllowExec  = true
    SandboxMode = true  // 强制沙箱
    // Rules 为空（沙箱内全权限，无需规则过滤）
case "full":
    AllowWrite = true
    AllowExec  = true
    SandboxMode = false // 裸机
}
```

### 4.4 权限边界（PermissionBoundary）

在 `GatewayState` 或 `EscalationManager` 增加 `MaxAllowedLevel` 概念：

```go
type EscalationManager struct {
    // ...现有字段...
    maxAllowedLevel string  // 权限边界：此 session 最高可升到的级别
                            // 默认 "sandboxed"（L2），手动配置可开放到 "full"
}

// RequestEscalation 时检查
if levelOrder(level) > levelOrder(m.maxAllowedLevel) {
    return fmt.Errorf("requested level %q exceeds session permission boundary %q", level, m.maxAllowedLevel)
}
```

---

## 五、审计日志设计

### 5.1 现有审计覆盖范围（不够）

现有 `EscalationAuditLogger` + `EscalationAuditEntry` 只记录权限生命周期事件：
`request → approve/deny → expire/task_complete/manual_revoke`

**L3 裸机模式下的缺口**：只知道"谁被授权了"，不知道"授权期间做了什么"。
AWS CloudTrail 和 Google Cloud Audit Logs 的核心是**记录每一次资源操作**，不只是授权事件。

### 5.2 双轨审计架构

```
~/.openacosmi/
  escalation-audit.log   ← 已有，记录权限生命周期（request/approve/deny/expire）
  tool-audit.log         ← 新增，记录 L3 下的每次工具调用
```

两个文件职责分离，沿用现有 JSON Lines 格式（每行一个 JSON 对象），风格统一。

### 5.3 ToolAuditEntry Schema

```go
// tool_audit.go（新文件，放在 gateway/ 包下）

// ToolAuditEntry 工具调用审计条目。
// 仅在 SecurityLevel == "full"（L3）时写入。
type ToolAuditEntry struct {
    // --- 时间 ---
    Timestamp  time.Time `json:"ts"`           // RFC3339Nano
    DurationMs int64     `json:"durationMs"`   // 工具执行耗时

    // --- 上下文 ---
    RunID        string `json:"runId"`
    SessionID    string `json:"sessionId,omitempty"`
    EscalationID string `json:"escalationId,omitempty"` // 关联的 escalation grant ID

    // --- 工具调用 ---
    ToolName string `json:"tool"`             // "bash" | "write_file" | "read_file" | ...
    ArgsSummary string `json:"args"`          // 截断到 300 字符，防止敏感数据膨胀

    // --- 结果 ---
    Outcome    string `json:"outcome"`        // "ok" | "error" | "denied"
    IsError    bool   `json:"isError"`
}
```

**ArgsSummary 截断规则**（安全设计）：
- bash 命令：记录命令字符串前 300 字符（`command` 字段）
- write_file：记录文件路径，**不记录文件内容**（内容可能含密钥）
- read_file：记录文件路径
- 其他工具：`json.Marshal(args)` 后截断到 300 字符

**Outcome 语义**：
- `"ok"` — 工具执行成功（exit code 0 或无 error）
- `"error"` — 工具执行失败（exit code != 0，或 Go 级 error）
- `"denied"` — 应用层权限拒绝（不应出现在 L3，但作为防御记录）

### 5.4 ToolAuditLogger 实现规格

与现有 `EscalationAuditLogger` 结构对齐，差异点：

```go
const (
    defaultToolAuditLogFile = "tool-audit.log"
    toolAuditMaxLines       = 10_000  // 超过时触发轮转
)

type ToolAuditLogger struct {
    mu       sync.Mutex
    filePath string
}

// Log 追加一条工具审计日志。仅在 level=="full" 时调用。
func (l *ToolAuditLogger) Log(entry ToolAuditEntry) { ... }

// ReadRecent 读取最近 N 条（最新在前）。供 security.tool_audit API 使用。
func (l *ToolAuditLogger) ReadRecent(limit int) ([]ToolAuditEntry, error) { ... }

// rotate 超过 maxLines 时，截断旧日志保留最新 maxLines 条。
// 在 Log() 内部调用（写锁已持有）。
func (l *ToolAuditLogger) rotateLocked() { ... }
```

**轮转策略**：写入前检查行数，超过 10,000 行则丢弃最旧的 5,000 行（保留后半段）。
用文件截断实现，不依赖外部轮转工具。

### 5.5 写入时机（在 attempt_runner.go 中）

```go
// 在 ExecuteToolCall 返回后，若 secLvl == "full"，写入审计
if secLvl == "full" && toolAuditLogger != nil {
    toolAuditLogger.Log(ToolAuditEntry{
        Timestamp:    time.Now(),
        DurationMs:   time.Since(toolStart).Milliseconds(),
        RunID:        params.RunID,
        SessionID:    params.SessionID,
        EscalationID: params.ActiveEscalationID, // 从 EscalationManager 传入
        ToolName:     tc.Name,
        ArgsSummary:  summarizeArgs(tc.Name, tc.Input, 300),
        Outcome:      outcomeOf(output, toolErr),
        IsError:      toolErr != nil,
    })
}
```

`EscalationID` 通过 `AttemptParams.ActiveEscalationID`（新增字段）从 `EscalationManager.active.ID` 传入，
把每次工具调用与批准它的 escalation grant 关联起来。

### 5.6 新增 API 端点

```
security.tool_audit.list   → 查询 tool-audit.log 最近 N 条
  params: { limit: int, runId?: string, tool?: string }
  返回: { entries: ToolAuditEntry[], total: int }
```

与现有 `security.escalation.audit` 对称设计，前端可并排展示两个日志视图：
"权限历史"（escalation）+ "操作历史"（tool）。

### 5.7 与 EscalationAuditEntry 的关联关系

```
EscalationAuditEntry { requestId: "esc_abc123", event: "approve", ... }
       ↑ 关联
ToolAuditEntry       { escalationId: "esc_abc123", tool: "bash", args: "rm -rf ...", outcome: "ok" }
ToolAuditEntry       { escalationId: "esc_abc123", tool: "write_file", args: "/etc/hosts", outcome: "ok" }
ToolAuditEntry       { escalationId: "esc_abc123", tool: "bash", args: "curl ...", outcome: "error" }
       ↓
EscalationAuditEntry { requestId: "esc_abc123", event: "expire", ... }
```

查询"这次 L3 授权期间执行了哪些操作"：
`tool-audit.log` 中 filter `escalationId == "esc_abc123"`。

---

## 六、Bug 修复清单

### Fix 1：降级时撤销 active grant（必须修复）

**文件**：`server_methods_exec_approvals.go`

在 `handleExecApprovalsSet` 成功写文件后：

```go
// 写文件成功后，检查是否需要撤销活跃 grant
if ctx.Context.EscalationMgr != nil {
    newLevel := extractNewSecurityLevel(fileMap)  // 从 fileMap 提取新级别
    status := ctx.Context.EscalationMgr.GetStatus()
    if status.HasActive && levelOrder(status.Active.Level) > levelOrder(newLevel) {
        // 活跃 grant 高于新 base level → 立即撤销
        ctx.Context.EscalationMgr.ManualRevoke()
        slog.Info("active grant revoked due to base level downgrade",
            "oldGrant", status.Active.Level, "newBase", newLevel)
    }
}
```

### Fix 2：OnPermissionDenied 前检查有效级别（必须修复）

**文件**：`server.go`

```go
OnPermissionDenied: func(tool, level, detail string) {
    // ... 广播事件（不变）...

    escMgr := state.EscalationMgr()
    if escMgr == nil {
        return
    }

    // ← 新增：检查当前 effective level 是否已满足需求
    currentLevel := escMgr.GetEffectiveLevel()
    escLevel := "allowlist"
    if tool == "bash" || tool == "write_file" {
        escLevel = "full"
    }
    if levelOrder(currentLevel) >= levelOrder(escLevel) {
        // 当前权限已满足，不触发审批（工具失败是 OS 级别问题，非权限问题）
        slog.Debug("OnPermissionDenied: current level already satisfies need, skipping escalation",
            "currentLevel", currentLevel, "neededLevel", escLevel)
        return
    }

    // ... 原有 RequestEscalation 逻辑...
},
```

### Fix 3：RequestEscalation 前校验 base level（防御性修复）

**文件**：`permission_escalation.go`

```go
func (m *EscalationManager) RequestEscalation(...) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // 新增：检查 base level 是否已满足请求
    baseLevel := readBaseSecurityLevel()
    if levelOrder(baseLevel) >= levelOrder(level) {
        return fmt.Errorf("base level %q already satisfies requested level %q", baseLevel, level)
    }

    // ... 原有逻辑...
}
```

### Fix 4：新增 "sandboxed" 层级 / 修复 AllowWrite/AllowExec 逻辑

**文件**：`exec_approvals.go` + `attempt_runner.go`

1. 在 `exec_approvals.go` 新增 `ExecSecuritySandboxed = "sandboxed"`
2. 更新 `MinSecurity` 的 order map（加入 sandboxed=2）
3. 修复 `attempt_runner.go` 中的 switch 逻辑（见 4.3 节）

---

## 六、审批流程优化

### 6.1 现有审批流程的问题

1. **超时 = 拒绝**：审批超时定时器设为 TTLMinutes，导致用户来不及审批就被拒绝
   - 建议：超时时间与 TTL 解耦，独立设置 `approvalTimeoutMinutes`（默认 5-10 分钟）

2. **单次审批全局生效**：批准一次后整个 session 都是高权限
   - 建议（可选增强）：审批时可选 "此任务" / "此 session" / "永久保存到预设"

3. **无权限边界**：任何 session 都可请求 L3
   - 建议：默认 `maxAllowedLevel = "sandboxed"`，L3 需要显式配置开启

### 6.2 L2 临时挂载区审批参数

L2 审批请求需新增挂载相关字段（扩展 `PendingEscalationRequest`）：

```go
type MountRequest struct {
    HostPath  string `json:"hostPath"`  // 宿主机目录，如 "/home/user/data"
    MountMode string `json:"mountMode"` // "ro"（只读）| "rw"（读写）
}

// 在 PendingEscalationRequest 新增字段
MountRequests []MountRequest `json:"mountRequests,omitempty"` // 仅 L2 使用
```

飞书/钉钉审批卡片展示：
- 路径列表：`/home/user/data (rw)`, `/opt/models (ro)`
- 用户可逐条接受或拒绝（接受全部才批准）
- 批准后：Docker 用 `--volume=<hostPath>:<mountPoint>:<mode>` 挂载
- TTL 到期：停止容器 → 挂载自动释放（无需显式 umount）

### 6.3 审批 TTL 合理值（来自 WorkOS/Google PAM 建议）

| 场景 | 推荐 TTL |
|---|---|
| L1 (allowlist) | 30 分钟，任务完成自动降 |
| L2 (sandboxed + 临时挂载) | 任务生命周期 + 5 分钟 grace，挂载同步卸载 |
| L3 (full) | 最大 60 分钟，强制，不可延期 |

---

## 七、关于 L0 "全局可读"的可靠性问题

用户提到"全局可读我觉得并不是很靠谱"。分析：

**当前实现**：`ExecSecurityDeny` 只是 `AllowWrite=false, AllowExec=false`。
读操作（read_file, list_dir, search, glob）没有任何限制，可以读取宿主机任意路径。

**问题**：
- 智能体可以 `read_file("/etc/passwd")` 或读取用户的私钥文件
- 无法阻止敏感文件泄露给 LLM（泄露到外部 API）

**国际标准做法**（Claude Code Sandboxing 文档）：
> macOS Seatbelt 只允许读取 CWD 和 `~/.claude`，Linux bubblewrap 绑定挂载只暴露工作区。

**务实建议**（渐进式）：
1. **短期**：在 L0 模式下，read_file / list_dir 检查路径是否在 `WorkspaceDir` 下，
   超出则返回 "permission denied: path outside workspace"
2. **中期**：L0 使用 read-only bind mount（Linux：`--bind-ro`），OS 级强制
3. **长期**：所有层级使用 OS 级沙箱（seccomp + namespace），如 Claude Code 做法

---

## 八、任务分解

- [ ] **Fix 1**: 降级时撤销 active grant（`server_methods_exec_approvals.go`）
- [ ] **Fix 2**: OnPermissionDenied 前检查 effective level（`server.go`）
- [ ] **Fix 3**: RequestEscalation 防御性校验 base level（`permission_escalation.go`）
- [ ] **Fix 4**: 新增 "sandboxed" 枚举 + 修复 AllowWrite/AllowExec switch（`exec_approvals.go` + `attempt_runner.go`）
- [ ] **Fix 5**: L0 模式下读取路径限制到 WorkspaceDir（渐进，可排期）
- [ ] **Enhancement**: 审批超时时间与 TTL 解耦（可排期）
- [ ] **Enhancement**: 权限边界 maxAllowedLevel（可排期）
- [ ] **Audit**: Fix 1-4 完成后触发 Skill 4 审计

---

## Online Verification Log（汇总）

| 主题 | 来源 | 关键发现 |
|---|---|---|
| Claude Code sandboxing | anthropic.com/engineering | OS 级强制执行，降低 84% 审批提示 |
| Google IAM PAM | cloud.google.com/iam | JIT + TTL 自动撤销 |
| AWS Permission Boundary | aws.amazon.com/wellarchitected | 有效权限 = Policy ∩ Boundary |
| Firecracker/gVisor | northflank.com | T4 microVM 最高隔离，权限降级必须重建容器 |
| MiniScope (UCB) | arxiv.org/abs/2512.11147 | OS 级机械强制，非 prompt 建议 |
| Linux capabilities | man7.org | bounding set 只降不升，进程边界 |
