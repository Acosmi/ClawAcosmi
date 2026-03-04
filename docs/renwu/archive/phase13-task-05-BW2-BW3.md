> 📄 分块 05/08 — B-W2 + B-W3 | 索引：phase13-task-00-index.md
>
> **B-W2 TS 源**：`src/infra/state-migrations.ts` + `src/infra/exec-approval-forwarder.ts` + `src/infra/skills-remote.ts` + `src/infra/node-pairing.ts` → **Go**：`backend/internal/infra/`
> **B-W3 TS 源**：`src/infra/exec-approvals.ts` + `src/infra/heartbeat-runner.ts` → **Go**：`backend/internal/infra/`

## 窗口 B-W2：迁移/配对/远程 ✅ 完成

> 完成日期：2026-02-18 | 13 个 Go 文件 + 1 新建（store）

- [x] **B-W2-T1**: 状态迁移 → 7+1 files: `state_migrations_{types,fs,keys,detect,run,wa,statedir,store}.go`
- [x] **B-W2-T2**: 审批转发 → 2 files: `approval_forwarder.go` + `approval_forwarder_ops.go`
- [x] **B-W2-T3**: 远程技能 → `skills_remote.go` | 节点配对 → `node_pairing.go` + `node_pairing_ops.go`
- [x] **B-W2 验证**：`go build` + `go vet` + `go test -race` ✅

### ✅ B-W2 隐藏依赖审计 — 全量补全

| TS 文件 | 隐藏依赖 | FIX | 状态 |
|---------|----------|-----|------|
| `state-migrations.ts` | routing/sessions 接线 + JSON5 + store 操作 | FIX-1/2 | ✅ |
| `exec-approval-forwarder.ts` | 消息构建 + 目标解析 + session filter + 投递 | FIX-3 | ✅ |
| `skills-remote.ts` | RecordNodeInfo + DescribeNode + RefreshNodeBins | FIX-5 | ✅ |
| `node-pairing.ts` | 10+ 字段扩展 + UpdatePairedNodeMetadata | FIX-4 | ✅ |

---

## 窗口 B-W3：infra 已有文件补全 ✅ 完成

> 完成日期：2026-02-18 | 3 个 Go 文件

- [x] **B-W3-T1**: `exec_approvals_ops.go` — `NormalizeExecApprovals`, `AddAllowlistEntry`, `RecordAllowlistUse`, `RequiresExecApproval`
  - 注：`MinSecurity`/`MaxAsk` 已在 `nodehost/allowlist_resolve.go` 和 `bash/exec_security.go` 中实现，无需重复
- [x] **B-W3-T2**: `heartbeat_delivery.go` + `heartbeat_delivery_run.go` — `RunHeartbeatOnce`, `NormalizeHeartbeatReply`, config helpers
  - 注：跨包依赖（auto-reply, channels, outbound）使用接口注入避免循环引用
- [x] **B-W3 验证**：`go build` + `go vet` + `go test -race` ✅

### ✅ B-W3 隐藏依赖审计 — 全量补全

| TS 文件 | 隐藏依赖 | FIX | 状态 |
|---------|----------|-----|------|
| `exec-approvals.ts` | 已在 `nodehost/allowlist_*.go` | — | ✅ 无需额外 |
| `heartbeat-runner.ts` | channel adapter + thinking token + batch 执行 | FIX-6 | ✅ |

> 架构文档：[infra.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/gouji/infra.md)

---
