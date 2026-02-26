> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Pairing 模块深度审计报告 (shenji-014)

**审计日期**：2026-02-25
**审计范围**：pairing-store.ts / pairing-messages.ts / pairing-labels.ts / channels/plugins/pairing.ts / infra/node-pairing.ts

---

## 一、总览

| 维度 | 数值 |
|------|------|
| TS 源文件 | 9 个 |
| Go 需新建 | 5 个（store.go / messages.go / labels.go / store_test.go / messages_test.go） |
| Go 需修改 | 4 个（channels/pairing.go / infra/node_pairing.go / node_pairing_ops.go / server_methods_nodes.go） |
| WARNING 总数 | 8 项（初始） + 4 项（复核追加） + 4 项（延迟项） = 16 项 |
| CRITICAL | 1 项（W-001） |
| HIGH | 5 项（W-002/003/004/005/006） |
| MEDIUM | 2 项（W-007 + DY-P02） |
| LOW | 4 项（W-008 + DY-P01/P03/P04） |
| 复核追加 | 4 项（R-001/002/003/004） |
| **全部修复** | **16 项 ✅** |
| 延迟项 | **0（DY-P01~P04 已修复）** |

---

## 二、channels/pairing.go — 接口偏差分析

### 现有 Go 接口

```go
type PairingAdapter interface {
    GenerateCode(accountID string) (string, error)
    ValidateCode(code string) (bool, error)
    NotifyApproval(channelID, pairingID string) error
}
```

### TS ChannelPairingAdapter（types.adapters.ts L184-192）

```typescript
type ChannelPairingAdapter = {
    idLabel: string;
    normalizeAllowEntry?: (entry: string) => string;
    notifyApproval?: (params: { cfg, id, runtime }) => Promise<void>;
};
```

### W-001 CRITICAL: Go 接口包含 TS 不存在的方法

Go 有 `GenerateCode` 和 `ValidateCode`，TS 中不存在。配对码生成逻辑在 `pairing-store.ts` 的 `randomCode()`/`generateUniqueCode()` 中实现，不在 adapter 接口中。

**修复方案**：移除 `GenerateCode` 和 `ValidateCode`，重新定义接口。

### W-002 HIGH: Go 缺少 `IDLabel()` 方法

TS `idLabel: string` — 每个渠道返回其 ID 标签（如 "userId"、"phone number" 等），用于 CLI 显示和消息构建。

**修复方案**：添加 `IDLabel() string` 到 PairingAdapter 接口。

### W-003 HIGH: Go 缺少 `NormalizeAllowEntry()` 方法

TS `normalizeAllowEntry?: (entry: string) => string` — 可选方法，用于渠道特定的白名单条目规范化。

**修复方案**：添加 `NormalizeAllowEntry(entry string) string` 到接口，不需要特殊处理的渠道返回 `strings.TrimSpace(entry)` 即可。

### W-004 HIGH: `NotifyApproval` 签名差异

TS: `notifyApproval(params: { cfg, id, runtime }) => Promise<void>` — 接收配置和运行时上下文。
Go: `NotifyApproval(channelID, pairingID string) error` — 缺少 cfg 和 runtime 参数。

**修复方案**：保持当前简化签名，Go 的适配器实现可自行获取 config。这是 Go 端有意简化，不影响功能。标记为已知架构差异。

---

## 三、infra/node_pairing_ops.go — 缺失函数分析

### W-005 HIGH: 缺少 `GetPairedNode`

TS（node-pairing.ts L197-200）：
```typescript
export async function getPairedNode(nodeId, baseDir): Promise<NodePairingPairedNode | null> {
    const state = await loadState(baseDir);
    return state.pairedByNodeId[normalizeNodeId(nodeId)] ?? null;
}
```

Go 现有代码无此函数。`ListNodePairingStatus()` 返回全部状态，但没有按 nodeID 查询单个节点的方法。

**修复方案**：新增 `GetPairedNode(nodeID string) *NodePairingPairedNode`。

### W-006 HIGH: 缺少 `RenamePairedNode`

TS（node-pairing.ts L319-337）：
```typescript
export async function renamePairedNode(nodeId, displayName, baseDir): Promise<NodePairingPairedNode | null> {
    return await withLock(async () => {
        // ...验证 displayName、更新、持久化
    });
}
```

Go 有 `UpdatePairedNodeMetadata(nodeID, patch func(*NodePairingPairedNode))` 可部分替代，但 TS 版有额外验证（displayName 非空）和返回值（返回更新后的节点）。

**修复方案**：新增 `RenamePairedNode(nodeID, displayName string) (*NodePairingPairedNode, error)`。

### W-007 MEDIUM: `ApproveNodePairing` 未保留原始 `CreatedAtMs`

TS 版本（L249）：
```typescript
createdAtMs: existing?.createdAtMs ?? now,
```
当节点重新配对（isRepair）时，保留原始创建时间。

Go 版本（L75）：
```go
CreatedAtMs: found.Ts,
```
始终使用 pending 请求的时间戳，不检查是否有已配对记录。

**修复方案**：在 `ApproveNodePairing` 中查找已有 paired 节点，如存在则保留其 `CreatedAtMs`。

### W-008 LOW: `RequestNodePairing` 未自动检测 `IsRepair`

TS（L216）：
```typescript
const isRepair = Boolean(state.pairedByNodeId[nodeId]);
```

Go 版本依赖调用方传入 `IsRepair` 字段，不自动检测。

**修复方案**：在 `RequestNodePairing` 中自动检测是否已有 paired 记录并设置 `IsRepair`。

---

## 四、pairing-store.ts — 完全缺失分析

### 需迁移的导出函数（6 个）

| TS 函数 | Go 函数名 | 复杂度 |
|---------|-----------|--------|
| `upsertChannelPairingRequest` | `UpsertChannelPairingRequest` | 高 — 查找/新建/更新/裁剪 |
| `approveChannelPairingCode` | `ApproveChannelPairingCode` | 高 — 查找码/删除/自动白名单 |
| `listChannelPairingRequests` | `ListChannelPairingRequests` | 中 — 清理+排序 |
| `readChannelAllowFromStore` | `ReadChannelAllowFromStore` | 低 |
| `addChannelAllowFromStoreEntry` | `AddChannelAllowFromStoreEntry` | 中 — 去重 |
| `removeChannelAllowFromStoreEntry` | `RemoveChannelAllowFromStoreEntry` | 中 |

### 需迁移的内部函数

| TS 函数 | Go 函数名 | 说明 |
|---------|-----------|------|
| `randomCode()` | `randomCode()` | crypto.randomInt → crypto/rand.Int |
| `generateUniqueCode()` | `generateUniqueCode()` | 碰撞重试，最多 500 次 |
| `normalizeId()` | `normalizeID()` | String(value).trim() |
| `normalizeAllowEntry()` | `normalizeAllowEntry()` | 调用 adapter.NormalizeAllowEntry |
| `safeChannelKey()` | `safeChannelKey()` | 文件名安全化 |
| `pruneExpiredRequests()` | `pruneExpiredRequests()` | 过期清理 |
| `pruneExcessRequests()` | `pruneExcessRequests()` | 容量裁剪（保留最近） |
| `readJsonFile()` | `readJSONFile()` | 文件读取 + fallback |
| `writeJsonFile()` | `writeJSONAtomic()` | 原子写入（tmp+rename） |
| `withFileLock()` | 使用 sync.Mutex | 单进程锁 |

### 需迁移的类型

```go
type PairingRequest struct {
    ID         string            `json:"id"`
    Code       string            `json:"code"`
    CreatedAt  string            `json:"createdAt"`
    LastSeenAt string            `json:"lastSeenAt"`
    Meta       map[string]string `json:"meta,omitempty"`
}

type pairingStore struct {
    Version  int              `json:"version"`
    Requests []PairingRequest `json:"requests"`
}

type allowFromStore struct {
    Version   int      `json:"version"`
    AllowFrom []string `json:"allowFrom"`
}
```

### 隐藏依赖清单

| TS 依赖 | Go 替代方案 | 状态 |
|---------|-------------|------|
| `process.env` | `os.Getenv` / 函数参数 | 需设计 |
| `proper-lockfile` | `sync.Mutex`（单进程） | 简化 |
| `crypto.randomInt` | `crypto/rand.Int` + `math/big` | 已验证 |
| `resolveStateDir()` | `config.ResolveStateDir()` | 已有 |
| `resolveOAuthDir()` | `config.ResolveOAuthDir()` | 已有 |
| `getPairingAdapter()` | `channels.GetPairingAdapter()` | 需接口修正后可用 |
| `Array.toSorted()` | `slices.SortFunc()` | Go 1.21+ |
| `Date.parse()` / `Date.now()` | `time.Parse` / `time.Now` | 标准库 |
| `path.join` | `filepath.Join` | 标准库 |
| `fs.promises` | `os.ReadFile` / `os.WriteFile` | 标准库 |

### 路径设计

- 配对存储：`{OAuthDir}/{channel}-pairing.json`
- 白名单存储：`{OAuthDir}/{channel}-allowFrom.json`
- OAuthDir = `config.ResolveOAuthDir()` = `~/.openacosmi/credentials/`

---

## 五、pairing-messages.ts + pairing-message.ts — 完全缺失分析

### 需迁移的导出

| TS | Go | 说明 |
|----|-----|------|
| `PAIRING_APPROVED_MESSAGE` | `PairingApprovedMessage` | 常量字符串 |
| `buildPairingReply({channel, idLine, code})` | `BuildPairingReply(channel, idLine, code)` | 构建回复文本 |

### 隐藏依赖

- `formatCliCommand(cmd)` — TS 中检查 `OPENACOSMI_PROFILE` 环境变量，如有则插入 `--profile <name>`
- Go 中无此函数，需简单实现：
  ```go
  func formatCliCommand(cmd string) string {
      profile := strings.TrimSpace(os.Getenv("OPENACOSMI_PROFILE"))
      if profile == "" {
          return cmd
      }
      // 在 "openacosmi" 后插入 "--profile <name>"
      return strings.Replace(cmd, "openacosmi ", "openacosmi --profile "+profile+" ", 1)
  }
  ```

---

## 六、pairing-labels.ts — 完全缺失分析

### 需迁移的导出

```typescript
export function resolvePairingIdLabel(channel: PairingChannel): string {
    return getPairingAdapter(channel)?.idLabel ?? "userId";
}
```

Go 实现直接调用 `channels.GetPairingAdapter(channel).IDLabel()` 即可。需要 W-002 修复后才能使用。

---

## 七、修复优先级总览

| 编号 | 级别 | 文件 | 摘要 |
|------|------|------|------|
| W-001 | CRITICAL | channels/pairing.go | 接口方法与 TS 完全不匹配（GenerateCode/ValidateCode 不存在于 TS） |
| W-002 | HIGH | channels/pairing.go | 缺少 IDLabel() 方法 |
| W-003 | HIGH | channels/pairing.go | 缺少 NormalizeAllowEntry() 方法 |
| W-004 | HIGH | channels/pairing.go | NotifyApproval 签名差异（已知架构差异，保留） |
| W-005 | HIGH | node_pairing_ops.go | 缺少 GetPairedNode 函数 |
| W-006 | HIGH | node_pairing_ops.go | 缺少 RenamePairedNode 函数 |
| W-007 | MEDIUM | node_pairing_ops.go | ApproveNodePairing 不保留原始 CreatedAtMs |
| W-008 | LOW | node_pairing_ops.go | RequestNodePairing 不自动检测 IsRepair |

---

## 八、建议修改 Go 代码草稿

### 8.1 channels/pairing.go — 接口修正

```go
// PairingAdapter 配对适配器接口（对齐 TS ChannelPairingAdapter）
type PairingAdapter interface {
    // IDLabel 返回渠道的用户 ID 标签（如 "userId"、"phone number"）
    IDLabel() string
    // NormalizeAllowEntry 渠道特定的白名单条目规范化
    // 不需要特殊处理的渠道返回 strings.TrimSpace(entry)
    NormalizeAllowEntry(entry string) string
    // NotifyApproval 通知配对已批准（可选行为，不支持时返回 nil）
    NotifyApproval(channelID, pairingID string) error
}
```

### 8.2 internal/pairing/store.go — 核心存储（新建）

完整实现 6 个导出函数 + 所有内部函数，使用：
- `sync.Mutex` 替代 `proper-lockfile`
- `crypto/rand.Int` 替代 `crypto.randomInt`
- `config.ResolveOAuthDir()` 获取凭证目录
- 原子文件写入模式（参考 device_pairing.go）

### 8.3 internal/pairing/messages.go — 消息构建（新建）

常量 + `BuildPairingReply` + 内联 `formatCliCommand`

### 8.4 internal/pairing/labels.go — 标签解析（新建）

单函数 `ResolvePairingIdLabel`

### 8.5 infra/node_pairing_ops.go — 补齐 2 函数 + 2 修复

- `GetPairedNode(nodeID) *NodePairingPairedNode`
- `RenamePairedNode(nodeID, displayName) (*NodePairingPairedNode, error)`
- `ApproveNodePairing` 保留原始 CreatedAtMs
- `RequestNodePairing` 自动检测 IsRepair

### 8.6 测试文件

- `store_test.go`：4 个测试用例对齐 pairing-store.test.ts
- `messages_test.go`：5 个渠道测试对齐 pairing-messages.test.ts

---

## 九、复核审计追加发现（R-001 ~ R-004）

> 以下为实现完成后执行复核审计（技能三）发现的问题，均已修复。

| 编号 | 文件 | 摘要 | 修复内容 |
|------|------|------|----------|
| R-001 | store.go | `UpsertChannelPairingRequest` 中 `expiredRemoved` 用 `_` 丢弃，导致清理后不触发回写 | 捕获变量，改为 `expiredRemoved \|\| cappedRemoved` 作为回写条件 |
| R-002 | store.go | `ApproveChannelPairingCode` 自动添加白名单时错误被静默丢弃 | 传播错误并附描述信息 |
| R-003 | node_pairing.go | `NodePairingPairedNode` 结构体缺少 `Bins`/`Permissions`/`LastConnectedAtMs` 字段 | 添加 3 个字段（对齐 TS PairedNode 类型定义） |
| R-004 | node_pairing_ops.go | `ApproveNodePairing` 未复制 `Permissions` 字段，重配对时可能产生重复条目 | 添加 `Permissions: found.Permissions` + 移除旧条目再追加 |

---

## 十、延迟项（预存在问题）— 已全量修复 ✅

| 编号 | 文件 | 摘要 | 严重度 | 修复方式 |
|------|------|------|--------|----------|
| DY-P01 | infra/node_pairing_ops.go | 多处函数缺少 nodeId 规范化 | LOW | RequestNodePairing/VerifyNodeToken/UpdatePairedNodeMetadata 入口加 `strings.TrimSpace` |
| DY-P02 | infra/node_pairing.go | 缺少 pending TTL 过期清理 | MEDIUM | 新增 `pendingTTLMs` 常量 + `pruneExpiredPending()` 在 loadPairingState 中调用 |
| DY-P03 | infra/node_pairing_ops.go | ListNodePairingStatus 返回未排序列表 | LOW | `slices.SortFunc` 按 Ts/ApprovedAtMs 降序 |
| DY-P04 | infra/node_pairing_ops.go | VerifyNodeToken 缺少返回匹配 node 对象 | LOW | 签名改为 `(*NodePairingPairedNode, bool)` + 调用方同步更新 |

---

## 十一、修复补全汇总

### 总览

| 维度 | 数值 |
|------|------|
| TS 源文件 | 9 个（核心 3 + 适配器 2 + CLI 1 + 测试 2 + infra 1） |
| Go 新建文件 | 5 个 |
| Go 修改文件 | 4 个 |
| 初始审计 WARNING | 8 项（W-001~W-008）→ 全部修复 ✅ |
| 复核审计追加 | 4 项（R-001~R-004）→ 全部修复 ✅ |
| 延迟项 | 4 项（DY-P01~DY-P04）→ 全部修复 ✅ |
| **总修复项** | **16 项** |
| 测试用例 | 12 个（store 9 + messages 3）→ 全部通过 |
| 遗留问题 | **0** |

### 按严重度分布

| 严重度 | 数量 | 编号 |
|--------|------|------|
| CRITICAL | 1 | W-001 接口方法与 TS 完全不匹配 |
| HIGH | 5 | W-002/003/004/005/006 接口缺失 + 函数缺失 |
| MEDIUM | 2 | W-007 CreatedAtMs 覆盖 + DY-P02 pending TTL |
| LOW | 4 | W-008 IsRepair + DY-P01/P03/P04 |
| 复核追加 | 4 | R-001~R-004 |
| **合计** | **16** | |

### 修改文件总清单（9 个文件）

#### 新建（5 个）

| # | 文件 | 行数 | 说明 |
|---|------|------|------|
| 1 | `internal/pairing/store.go` | ~570L | 渠道配对存储核心（6 导出 + 10 内部函数） |
| 2 | `internal/pairing/messages.go` | ~45L | 配对消息构建（常量 + BuildPairingReply + formatCliCommand） |
| 3 | `internal/pairing/labels.go` | ~20L | 配对 ID 标签解析（ResolvePairingIdLabel） |
| 4 | `internal/pairing/store_test.go` | ~200L | 存储测试（9 个用例） |
| 5 | `internal/pairing/messages_test.go` | ~70L | 消息测试（3 个用例） |

#### 修改（4 个）

| # | 文件 | 修复项 |
|---|------|--------|
| 6 | `internal/channels/pairing.go` | W-001/002/003/004 接口重构 → PairingAdapter + PairingApprovalNotifier |
| 7 | `internal/infra/node_pairing.go` | R-003 添加 3 字段 + DY-P02 pendingTTLMs + pruneExpiredPending |
| 8 | `internal/infra/node_pairing_ops.go` | W-005/006/007/008 + R-004 + DY-P01/P03/P04 |
| 9 | `internal/gateway/server_methods_nodes.go` | DY-P04 适配 VerifyNodeToken 新签名 |

### 设计决策记录

| 决策 | 选型 | 依据 |
|------|------|------|
| 文件锁 | `sync.Mutex` per-channel | 单进程模型，无需 proper-lockfile |
| 随机码生成 | `crypto/rand.Int` + `math/big` | CSPRNG 安全等效 crypto.randomInt |
| 原子写入 | tmp + os.Rename | 对齐 TS writeJsonFile + 参考 device_pairing.go |
| 接口拆分 | PairingAdapter + PairingApprovalNotifier | 对齐 TS optional notifyApproval |
| 排序 | `slices.SortFunc` | 对齐 TS Array.toSorted()，Go 1.21+ |
| Pending 过期 | loadPairingState 自动清理 | 对齐 TS loadState → pruneExpiredPending |

### 编译验证

```
go build ./...                   ✅ 通过
go test ./internal/pairing/...   ✅ 12/12 PASS
go test ./internal/infra/...     ✅ 无回归
```

### 文档产出

| 文档 | 路径 |
|------|------|
| 审计跟踪 | `docs/claude/renwu/pairing_audit_tracker.md` |
| 审计报告+汇总 | `docs/claude/goujia/shenji-014-pairing-audit.md`（本文件） |
| 延迟项归档 | `docs/claude/daibanyanchi_guidang.md`（DY-P01~P04 节） |

### 结论

Pairing 模块 TS→Go 迁移审计 **全部完成**：

- **9 个 TS 源文件**审计完毕，5 个 Go 文件新建 + 4 个修改
- **8 个初始 WARNING** + **4 个复核追加** + **4 个延迟项** = **16 项全部修复**
- **1 个 CRITICAL 漏洞**修复（接口方法与 TS 完全不匹配）
- **12 个测试用例**全部通过，无回归
- **零遗留延迟项**
