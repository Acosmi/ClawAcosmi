# OpenAcosmi 权限分级与审批系统设计规范

**版本**: 1.0
**日期**: 2026-02-27
**状态**: 正式设计文档

---

## 一、四层权限模型

### 1.1 总览

| 级别 | 代码值 | 名称 | 沙箱 | 网络 | 审批 | 审计 |
|---|---|---|---|---|---|---|
| L0 | `deny` | 只读 | 无 | 无 | 无需 | 无 |
| L1 | `allowlist` | 工作区受限 | ✅ | 无 | 无需 | 无 |
| L2 | `sandboxed` | 沙箱全权限 | ✅ | 无 | JIT 审批 | 无 |
| L3 | `full` | 裸机全权限 | ❌ | ✅ | JIT 审批 | `tool-audit.log` |

升级路径：`L0 → L1 → L2 → L3`，每步均需独立审批。
降级路径：任务完成或 TTL 到期后自动降回 base level，不可跨层跳降。

---

### 1.2 L0 — 只读（deny）

```
AllowWrite:   false
AllowExec:    false
SandboxMode:  false
Network:      无（exec 本身被禁止，不产生网络调用）
读权限:        工作区目录内（路径越界返回 permission denied）
审批:          无需
升级路径:      → L1（无需审批，用户在设置页直接调整 base level）
```

**说明**：L0 是最保守的默认模式。`read_file`、`list_dir`、`glob`、`search` 仅限工作区路径，
超出工作区的读取请求被应用层拦截并返回错误。

---

### 1.3 L1 — 工作区受限（allowlist）

```
AllowWrite:   true（限工作区目录）
AllowExec:    true（仅白名单命令，规则引擎 EvaluateCommand 过滤）
SandboxMode:  true（Docker 或 NativeSandbox）
Network:      无（--network=none）
审批:          无需（base level 直接配置）
升级路径:      → L2 需要 JIT 审批
```

**命令规则**：`CommandRule` 列表按优先级排序，`deny → ask → allow`，首个匹配项生效。
空规则列表时，exec 请求全部落入 `ask`（触发提权申请）。

---

### 1.4 L2 — 沙箱全权限 + 临时挂载区（sandboxed）

```
AllowWrite:   true（沙箱内任意路径 + 已授权挂载区）
AllowExec:    true（沙箱内任意命令，无规则过滤）
SandboxMode:  true（强制沙箱，内核边界隔离）
Network:      无（--network=none）

挂载（默认）:  工作区目录（始终挂载，读写）
挂载（可选）:  用户在审批卡片中逐条授权的宿主机目录（bind mount，支持 ro/rw）

TTL:          任务生命周期 + 5 分钟 grace，到期停容器，挂载自动释放
审批:          JIT 人工审批（见第三节）
升级路径:      → L3 需要 JIT 审批 + TTL ≤ 60 min
```

**挂载生命周期**：
- 审批通过 → 以指定 mount 参数启动沙箱容器
- 任务完成 / TTL 到期 → 停止容器 → 挂载自动消失（Docker volume 语义）
- 无需显式 umount，停容器即释放

**安全边界**：
- 内核隔离：命令出错影响范围仅限容器内
- 无网络：无法外泄数据、无法反弹 shell
- 挂载路径用户显式授权，agent 不可自行扩展挂载范围

---

### 1.5 L3 — 裸机全权限（full）

```
AllowWrite:   true（宿主机任意路径）
AllowExec:    true（任意命令）
SandboxMode:  false
Network:      全开（不限）
TTL:          最大 60 分钟，强制上限，不可延期
审批:          JIT 人工审批（见第三节）
审计:          每次工具调用写 tool-audit.log（见第五节）
降级:          任务完成 / TTL 到期 → 自动降回 base level
```

**说明**：L3 是最高权限，agent 直接在宿主机进程中运行。
默认 `maxAllowedLevel = "sandboxed"`，即系统默认不允许升到 L3。
需在配置中显式设置 `maxAllowedLevel = "full"` 才能请求 L3。

---

## 二、权限矩阵

| 操作 | L0 | L1 | L2 | L3 |
|---|---|---|---|---|
| 读工作区文件 | ✅ | ✅ | ✅ | ✅ |
| 读工作区外文件 | ❌ | ❌ | ✅（沙箱内） | ✅ |
| 写工作区文件 | ❌ | ✅ | ✅ | ✅ |
| 写工作区外文件 | ❌ | ❌ | ✅（沙箱内 + 授权挂载区） | ✅ |
| 执行命令（白名单内） | ❌ | ✅ | ✅ | ✅ |
| 执行命令（白名单外） | ❌ | ❌ | ✅（沙箱内） | ✅ |
| 访问挂载的宿主机目录 | ❌ | ❌ | ✅（审批授权路径） | ✅ |
| 网络访问 | ❌ | ❌ | ❌ | ✅ |
| 访问宿主机进程/设备 | ❌ | ❌ | ❌ | ✅ |

---

## 三、JIT 审批流程

### 3.1 审批请求结构

```go
type PendingEscalationRequest struct {
    ID             string         `json:"id"`             // "esc_xxxxxxxx"
    RequestedLevel string         `json:"requestedLevel"` // "sandboxed" | "full"
    Reason         string         `json:"reason"`
    RunID          string         `json:"runId,omitempty"`
    SessionID      string         `json:"sessionId,omitempty"`
    RequestedAt    time.Time      `json:"requestedAt"`
    TTLMinutes     int            `json:"ttlMinutes"`

    // L2 专用：临时挂载请求（工作区默认挂载不在此列）
    MountRequests  []MountRequest `json:"mountRequests,omitempty"`
}

type MountRequest struct {
    HostPath  string `json:"hostPath"`  // 宿主机绝对路径，如 "/home/user/data"
    MountMode string `json:"mountMode"` // "ro" 或 "rw"
}
```

### 3.2 审批流程

```
Agent 触发权限拒绝
  ↓
OnPermissionDenied 回调
  ↓ 检查: GetEffectiveLevel() >= neededLevel ? → 跳过（OS 级错误，非权限问题）
  ↓
RequestEscalation(level, reason, mountRequests?)
  ↓ 检查: baseLevel >= requestedLevel ? → 返回错误（已满足，无需申请）
  ↓ 检查: hasActive || hasPending ? → 返回错误（已有进行中）
  ↓ 检查: levelOrder(requestedLevel) > levelOrder(maxAllowedLevel) ? → 拒绝（超边界）
  ↓
广播 exec.approval.requested 事件（前端弹窗）
+ 异步推送飞书/钉钉审批卡片
+ 启动 approvalTimeout 定时器（默认 10 分钟，超时自动拒绝）
  ↓
用户点击「批准」
  ↓
ResolveEscalation(approve=true, ttlMinutes)
  ↓
创建 ActiveEscalationGrant（记录 level + expiresAt + mountRequests）
启动 TTL 定时器（到期调用 autoDeescalate）
广播 exec.approval.resolved 事件
推送审批结果卡片
  ↓
Agent 重试工具调用（使用新权限 + 新挂载配置）
```

### 3.3 审批超时与 TTL 解耦

| 参数 | 含义 | 默认值 |
|---|---|---|
| `approvalTimeoutMinutes` | 等待人工审批的超时（超时自动拒绝） | 10 分钟 |
| `ttlMinutes` | 授权生效后的有效时长 | L2: 任务时长+5, L3: ≤60 |

两者独立。超时拒绝 ≠ TTL 到期降权，语义不同。

### 3.4 降级触发条件

任意以下条件满足，立即执行 `autoDeescalate()`：

1. TTL 到期（`time.AfterFunc` 定时器）
2. 任务完成（`TaskComplete(runID)` 被调用）
3. 用户手动撤销（`ManualRevoke()`）
4. base level 被降低且低于 active grant level（`handleExecApprovalsSet` 后触发检查）

---

## 四、权限级别排序与边界

### 4.1 级别排序

```go
var levelOrder = map[ExecSecurity]int{
    ExecSecurityDeny:      0,
    ExecSecurityAllowlist: 1,
    ExecSecuritySandboxed: 2, // 新增
    ExecSecurityFull:      3,
}
```

### 4.2 权限边界（maxAllowedLevel）

```go
// EscalationManager 新增字段
maxAllowedLevel string // 默认 "sandboxed"，需显式配置才可设为 "full"
```

`RequestEscalation` 时校验：`levelOrder(requestedLevel) > levelOrder(maxAllowedLevel)` → 拒绝。
前端在设置页展示当前 maxAllowedLevel，用户可调整（需要本地确认操作，不走审批流程）。

### 4.3 工具执行参数映射

```go
// attempt_runner.go 中替换现有 AllowWrite/AllowExec 逻辑
switch ExecSecurity(secLvl) {
case ExecSecurityDeny:
    params.AllowWrite  = false
    params.AllowExec   = false
    params.SandboxMode = false
case ExecSecurityAllowlist:
    params.AllowWrite  = true   // 工作区目录（路径检查由 tool executor 执行）
    params.AllowExec   = true   // 配合 Rules 白名单过滤
    params.SandboxMode = true
case ExecSecuritySandboxed:
    params.AllowWrite  = true   // 沙箱内任意路径
    params.AllowExec   = true   // 无规则过滤
    params.SandboxMode = true
    params.MountPaths  = activeMounts // 从 ActiveEscalationGrant.MountRequests 取
case ExecSecurityFull:
    params.AllowWrite  = true
    params.AllowExec   = true
    params.SandboxMode = false
}
```

---

## 五、审计日志设计

### 5.1 双轨架构

```
~/.openacosmi/
  escalation-audit.log    权限生命周期（已有）
  tool-audit.log          L3 工具操作记录（新增）
```

### 5.2 escalation-audit.log（已有，维持现有结构）

记录事件：`request | approve | deny | expire | task_complete | manual_revoke`

关键字段：`timestamp, event, requestId, requestedLevel, reason, runId, sessionId, ttlMinutes`

### 5.3 tool-audit.log（新增）

**触发条件**：仅当 `secLvl == "full"`（L3）时写入。

**Schema**：

```go
type ToolAuditEntry struct {
    Timestamp    time.Time `json:"ts"`
    DurationMs   int64     `json:"durationMs"`
    RunID        string    `json:"runId"`
    SessionID    string    `json:"sessionId,omitempty"`
    EscalationID string    `json:"escalationId"`  // 关联 escalation grant ID
    ToolName     string    `json:"tool"`
    ArgsSummary  string    `json:"args"`           // 截断至 300 字符
    Outcome      string    `json:"outcome"`        // "ok" | "error" | "denied"
    IsError      bool      `json:"isError"`
}
```

**ArgsSummary 截断规则**：

| 工具 | 记录内容 |
|---|---|
| `bash` | 命令字符串前 300 字符 |
| `write_file` | 仅记录文件路径，**不记录文件内容** |
| `read_file` | 文件路径 |
| 其他 | `json.Marshal(args)` 截断至 300 字符 |

**存储规格**：JSON Lines，超过 10,000 行时轮转（保留最新 5,000 条），无外部依赖。

**关联查询**：

```
escalation-audit.log: { requestId: "esc_abc", event: "approve" }
                              ↕ escalationId 关联
tool-audit.log:        { escalationId: "esc_abc", tool: "bash", args: "..." }
                       { escalationId: "esc_abc", tool: "write_file", args: "/etc/hosts" }
escalation-audit.log: { requestId: "esc_abc", event: "expire" }
```

### 5.4 新增 API

```
security.tool_audit.list
  params:  { limit: int, runId?: string, tool?: string }
  returns: { entries: ToolAuditEntry[], total: int }
```

与现有 `security.escalation.audit` 对称，前端可并排展示"权限历史"和"操作历史"。

---

## 六、网络隔离策略

| 级别 | 实现方式 |
|---|---|
| L0 | 无 exec，不产生网络调用 |
| L1 | Docker `--network=none` / `unshare --net`（无默认路由） |
| L2 | Docker `--network=none` / `unshare --net` |
| L3 | 宿主机全网络，所有工具调用记录至 `tool-audit.log` |

L2 无网络的意义：沙箱内即使命令跑飞，也无法外泄数据或建立反向连接，攻击面极小。
需要网络访问时，升级至 L3 并接受审计约束。

---

## 七、Bug 修复清单

以下 Bug 须在新权限系统实施前修复：

### Fix 1：降级时撤销 active grant

**文件**：`server_methods_exec_approvals.go` → `handleExecApprovalsSet`

**问题**：写入新 base level 后未通知 `EscalationManager`，active grant 继续生效。

**修复**：写文件成功后，若 `active.Level` 的 levelOrder > 新 base level 的 levelOrder，
调用 `escMgr.ManualRevoke()`。

---

### Fix 2：OnPermissionDenied 前检查有效级别

**文件**：`server.go` → `OnPermissionDenied` 回调

**问题**：base level 已是 "full" 时，OS 级错误被误判为权限问题，触发不必要的审批弹窗。

**修复**：触发 `RequestEscalation` 前先检查 `GetEffectiveLevel() >= neededLevel`，
满足则直接返回，不走审批流程。

---

### Fix 3：RequestEscalation 前校验 base level

**文件**：`permission_escalation.go` → `RequestEscalation`

**问题**：base level 已满足请求级别时仍创建 pending 请求，造成逻辑混乱。

**修复**：函数入口处校验 `levelOrder(readBaseSecurityLevel()) >= levelOrder(level)`，
满足则返回 `fmt.Errorf("base level already satisfies requested level %q", level)`。

---

### Fix 4：新增 sandboxed 枚举 + 修复工具执行 switch

**文件**：`exec_approvals.go`，`attempt_runner.go`

**问题**：`allowlist` 和 `full` 的 `AllowWrite/AllowExec` 完全相同，
沙箱是唯一区别，若沙箱未运行则两者行为无差异。
缺少 L2 (`sandboxed`) 层级。

**修复**：
1. `exec_approvals.go`：新增 `ExecSecuritySandboxed ExecSecurity = "sandboxed"`，
   更新 `MinSecurity` 的 order map（sandboxed=2）。
2. `attempt_runner.go`：用 `switch` 替换现有 `allowlist || full` 逻辑（见第四节 4.3）。

---

## 八、TTL 参考值

| 级别 | 推荐 TTL | 强制上限 |
|---|---|---|
| L1 (allowlist) | base level 直接配置，无 TTL | — |
| L2 (sandboxed) | 任务生命周期 + 5 分钟 grace | 无强制上限（任务驱动） |
| L3 (full) | 用户申请时指定，默认 30 分钟 | **60 分钟** |

L3 的 60 分钟上限在 `ResolveEscalation` 中强制截断：
```go
if ttlMinutes > 60 {
    ttlMinutes = 60
}
```

---

## 附录：变更点速查

| 变更项 | 涉及文件 | 类型 |
|---|---|---|
| 新增 `ExecSecuritySandboxed` | `exec_approvals.go` | 枚举扩展 |
| 更新 `MinSecurity` order map | `exec_approvals.go` | 逻辑修改 |
| 新增 `MountRequest` 类型 | `exec_approvals.go` 或新文件 | 类型新增 |
| `PendingEscalationRequest` 加 `MountRequests` | `permission_escalation.go` | 结构扩展 |
| `ActiveEscalationGrant` 加 `MountRequests` | `permission_escalation.go` | 结构扩展 |
| `EscalationManager` 加 `maxAllowedLevel` | `permission_escalation.go` | 字段新增 |
| `RequestEscalation` 加边界检查 + base level 检查 | `permission_escalation.go` | Bug Fix 3 |
| `ResolveEscalation` 加 L3 TTL 上限截断 | `permission_escalation.go` | 逻辑修改 |
| `handleExecApprovalsSet` 加降级撤销 | `server_methods_exec_approvals.go` | Bug Fix 1 |
| `OnPermissionDenied` 加有效级别检查 | `server.go` | Bug Fix 2 |
| 工具执行 switch 替换 allowlist\|\|full 逻辑 | `attempt_runner.go` | Bug Fix 4 |
| `ToolAuditLogger` + `ToolAuditEntry` | `tool_audit.go`（新文件） | 新增 |
| `security.tool_audit.list` API | `server_methods_escalation.go` | 新增 |
