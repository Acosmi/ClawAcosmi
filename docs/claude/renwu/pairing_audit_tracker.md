> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Pairing 模块审计跟踪

## 审计统计
- TS 源文件：9 个（核心 3 + 适配器 2 + CLI 1 + 测试 2 + infra 1）
- Go 现有文件：4 个（channels/pairing.go + infra/node_pairing.go + node_pairing_ops.go + config/paths.go）
- 新建 Go 文件：5 个（store.go + messages.go + labels.go + store_test.go + messages_test.go）
- 修改 Go 文件：3 个（channels/pairing.go + infra/node_pairing.go + infra/node_pairing_ops.go）
- WARNING 总数：8 项（初始） + 4 项（复核审计追加） = 12 项
- 全部修复：12 项 + 4 延迟项 = 16 项 | 延迟项：0（DY-P01~P04 已修复）

## 文件映射

| TS 源文件 | Go 目标文件 | 状态 |
|-----------|-------------|------|
| pairing/pairing-store.ts (498L) | internal/pairing/store.go | ✅ 已实现 |
| pairing/pairing-messages.ts (21L) + channels/plugins/pairing-message.ts (3L) | internal/pairing/messages.go | ✅ 已实现 |
| pairing/pairing-labels.ts (7L) | internal/pairing/labels.go | ✅ 已实现 |
| channels/plugins/pairing.ts (70L) | internal/channels/pairing.go | ✅ 接口已修正 |
| infra/node-pairing.ts (337L) | infra/node_pairing.go + node_pairing_ops.go | ✅ 补齐+修复 |
| pairing/pairing-store.test.ts | internal/pairing/store_test.go | ✅ 9 测试通过 |
| pairing/pairing-messages.test.ts | internal/pairing/messages_test.go | ✅ 3 测试通过 |
| cli/pairing-cli.ts (151L) | — | ℹ️ CLI 层，不在本次范围 |

## 任务清单

### 任务 1：新建 internal/pairing/store.go ✅
- [x] PairingRequest / pairingStore / allowFromStore 类型
- [x] 常量：pairingCodeLength / pairingCodeAlphabet / pairingPendingTTLMs / pairingPendingMax
- [x] randomCode / generateUniqueCode（crypto/rand.Int）
- [x] normalizeID / normalizeAllowEntry / safeChannelKey
- [x] pruneExpiredRequests / pruneExcessRequests（slices.SortFunc）
- [x] File I/O: readJSONFile / writeJSONAtomic（原子写入）
- [x] UpsertChannelPairingRequest
- [x] ApproveChannelPairingCode（含自动白名单+错误传播）
- [x] ListChannelPairingRequests
- [x] ReadChannelAllowFromStore
- [x] AddChannelAllowFromStoreEntry
- [x] RemoveChannelAllowFromStoreEntry

### 任务 2：新建 internal/pairing/messages.go ✅
- [x] PairingApprovedMessage 常量
- [x] BuildPairingReply 函数
- [x] formatCliCommand 辅助函数（含 OPENACOSMI_PROFILE 处理）

### 任务 3：新建 internal/pairing/labels.go ✅
- [x] ResolvePairingIdLabel 函数

### 任务 4：修改 internal/channels/pairing.go ✅
- [x] W-001 CRITICAL: 移除 GenerateCode/ValidateCode，重构接口
- [x] W-002 HIGH: 添加 IDLabel() 方法
- [x] W-003 HIGH: 添加 NormalizeAllowEntry() 方法
- [x] W-004 HIGH: NotifyApproval 改为可选接口 PairingApprovalNotifier

### 任务 5：修改 infra/node_pairing.go + node_pairing_ops.go ✅
- [x] W-005 HIGH: 新增 GetPairedNode
- [x] W-006 HIGH: 新增 RenamePairedNode
- [x] W-007 MEDIUM: 修复 ApproveNodePairing 保留原始 CreatedAtMs
- [x] W-008 LOW: RequestNodePairing 自动检测 IsRepair
- [x] 复核追加: NodePairingPairedNode 添加 Bins/Permissions/LastConnectedAtMs 字段
- [x] 复核追加: ApproveNodePairing 复制 Permissions 字段
- [x] 复核追加: ApproveNodePairing 移除旧条目防重复

### 任务 6：新建测试 ✅
- [x] store_test.go — 9 个测试（upsert/expire/cap/approve/allowFrom/safeChannelKey）
- [x] messages_test.go — 3 个测试（5 渠道回复/无 profile/常量）

### 任务 7：集成确认 ✅
- [x] pairing.go 适配器接口与新 store 集成正确
- [x] go build ./... 通过
- [x] go test ./internal/pairing/... 通过（12/12）
- [x] go test ./internal/infra/... 通过（无回归）

## WARNING 编号索引

### 初始审计
- W-001 CRITICAL: channels/pairing.go GenerateCode/ValidateCode 不存在于 TS ✅ 已修复
- W-002 HIGH: channels/pairing.go 缺少 IDLabel() ✅ 已修复
- W-003 HIGH: channels/pairing.go 缺少 NormalizeAllowEntry() ✅ 已修复
- W-004 HIGH: NotifyApproval 签名差异 → 改为可选接口 ✅ 已修复
- W-005 HIGH: node_pairing_ops.go 缺少 GetPairedNode ✅ 已修复
- W-006 HIGH: node_pairing_ops.go 缺少 RenamePairedNode ✅ 已修复
- W-007 MEDIUM: ApproveNodePairing 不保留原始 CreatedAtMs ✅ 已修复
- W-008 LOW: RequestNodePairing 不自动检测 IsRepair ✅ 已修复

### 复核审计追加修复
- R-001: store.go UpsertChannelPairingRequest expiredRemoved 未用于 write-back 条件 ✅ 已修复
- R-002: store.go ApproveChannelPairingCode auto-add 错误应传播 ✅ 已修复
- R-003: node_pairing.go NodePairingPairedNode 缺失 Bins/Permissions/LastConnectedAtMs 字段 ✅ 已修复
- R-004: node_pairing_ops.go ApproveNodePairing 未复制 Permissions + 重配对重复 ✅ 已修复

## 延迟项（预存在问题）— 已全量修复 ✅

| 编号 | 摘要 | 严重度 | 状态 |
|------|------|--------|------|
| DY-P01 | node_pairing_ops: 多处函数缺少 nodeId 规范化（TrimSpace） | LOW | ✅ 已修复 |
| DY-P02 | node_pairing.go: 缺少 pending TTL 过期清理（TS PENDING_TTL_MS=5min） | MEDIUM | ✅ 已修复 |
| DY-P03 | ListNodePairingStatus: 返回未排序列表（TS 按 ts/approvedAtMs 降序） | LOW | ✅ 已修复 |
| DY-P04 | VerifyNodeToken: 缺少返回匹配的 node 对象（TS 返回 { ok, node }） | LOW | ✅ 已修复 |

## 修复报告索引
- shenji-014: Pairing 模块深度审计报告（W-001~W-008）

## 修改文件清单（8 个文件）

### 新建（5 个）
1. `internal/pairing/store.go` — 渠道配对存储核心（~570L）
2. `internal/pairing/messages.go` — 配对消息构建（~45L）
3. `internal/pairing/labels.go` — 配对 ID 标签解析（~20L）
4. `internal/pairing/store_test.go` — 存储测试（~200L）
5. `internal/pairing/messages_test.go` — 消息测试（~70L）

### 修改（4 个）
6. `internal/channels/pairing.go` — PairingAdapter 接口重构
7. `internal/infra/node_pairing.go` — NodePairingPairedNode 添加 3 字段 + pendingTTL 清理
8. `internal/infra/node_pairing_ops.go` — 添加 2 函数 + 4 修复 + DY-P01~P04 修复
9. `internal/gateway/server_methods_nodes.go` — 适配 VerifyNodeToken 新签名
