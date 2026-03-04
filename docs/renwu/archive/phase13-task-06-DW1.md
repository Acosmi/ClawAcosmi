> 📄 分块 06/08 — D-W1 | 索引：phase13-task-00-index.md
>
> **TS 源**：`src/gateway/server-methods/` → **Go**：`backend/internal/gateway/`

## 窗口 D-W1：Gateway Stub 全量实现（2 会话）

> 参考：`gap-analysis-part4b.md` D-W1 节
> 文件：`backend/internal/gateway/server_methods_stubs.go` → 拆分为多个实现文件
> 精确计数：**44 个 stub**（原计划 ~22 个，差距分析修正）

### 会话 D-W1a：G1~G4 组（cron/tts/skills/node）— ✅ 审计完成

- [x] **D-W1-G1**: cron.* (7 个方法) — ✅ 2026-02-18
  - TS: `src/gateway/server-methods/cron.ts` (227L)
  - Go 对接: `internal/cron/` 包（19 文件 3,711L，已完整）
  - [x] `cron.list` / `cron.status` / `cron.runs` / `cron.add` / `cron.update` / `cron.remove` / `cron.run`
  - 审计修复: FIND-1（默认 mode `force`→`now`）、FIND-2（patch 从 `params.patch` 子对象提取）、FIND-3（响应 key `entries`）
  - 输出：`server_methods_cron.go`

- [x] **D-W1-G2**: tts.* (6 个方法) — ✅ 已有实现（前序窗口）
  - TS: `src/gateway/server-methods/tts.ts` (157L)
  - Go 对接: `internal/tts/` 包（8 文件 1,881L，已完整）
  - [x] `tts.status` / `tts.providers` / `tts.enable` / `tts.disable` / `tts.convert` / `tts.setProvider`
  - 输出：`server_methods_tts.go`

- [x] **D-W1-G3**: skills.* (4 个方法) — ✅ 2026-02-18
  - TS: `src/gateway/server-methods/skills.ts` (216L)
  - Go 对接: `internal/agents/skills/` + `config/` + `infra/` + `routing/`
  - [x] `skills.status` — 全量实现：agent 解析 + BuildWorkspaceSkillSnapshot + 远程资格
  - [x] `skills.bins` — 全量实现：多工作区收集 + 去重排序
  - [x] `skills.update` — 全量实现：skillKey 参数 + enabled/apiKey/env 配置写入
  - [x] `skills.install` — 骨架实现（返回 not_implemented，需 D-W2 `InstallSkill` Go 函数）
  - 审计修复: FIND-4（全量 status）、FIND-5（skillKey 参数名）、FIND-6（WriteConfigFile）、FIND-7（workspace 解析）
  - 输出：`server_methods_skills.go` (280L)

- [x] **D-W1-G4**: node.*(11 个方法，含 pair.* 子方法) — ✅ 2026-02-18
  - TS: `src/gateway/server-methods/nodes.ts` (537L)
  - Go 对接: `internal/nodehost/` + `internal/gateway/device_pairing.go`
  - [x] `node.list` / `node.describe` / `node.invoke` / `node.invoke.result` / `node.event` / `node.rename`
  - [x] `node.pair.request` / `node.pair.list` / `node.pair.approve` / `node.pair.reject` / `node.pair.verify`
  - 审计修复: FIND-8（listDevicePairing+isNodeEntry）、FIND-9（command policy check）、FIND-10（参数归一化）
  - 新增文件: `node_command_policy.go` (181L) — 平台默认 allowlist + config overlay + deny list
  - 输出：`server_methods_nodes.go` (600L)

### 会话 D-W1b_stubs：G5~G7 组（device/exec.approval/其余10个）

- [x] **D-W1-G5**: device.* (5 个方法) — ✅ 已有实现（前序窗口）
  - TS: `src/gateway/server-methods/devices.ts` (190L)
  - Go 对接: `internal/gateway/device_pairing.go` (843L)
  - [x] `device.pair.list` / `device.pair.approve` / `device.pair.reject` / `device.token.rotate` / `device.token.revoke`
  - ⚠️ FIND-11（summarizeDeviceTokens 代码风格差异）— 低优先级，可延迟
  - 输出：`server_methods_devices.go`

- [x] **D-W1-G6**: exec.approval.* (4 个方法) — ✅ 已有实现（前序窗口）
  - TS: `exec-approvals.ts` (242L)
  - Go 对接: `server_methods_exec_approvals.go` (232L，已有部分)
  - [x] `exec.approval.request` / `exec.approval.resolve` / `exec.approvals.list` / `exec.approvals.resolve`
  - 输出：扩展已有文件

- [x] **D-W1-G7**: 其余 10 个方法 — ✅ 混合完成
  - [x] `voicewake.get` / `voicewake.set` — 语音唤醒
  - [x] `update.check` / `update.run` — 自动更新
  - [x] `browser.request` — ✅ 全量重写：FIND-12~16 修复（method/path 参数 + 方法验证 + node proxy + 文件持久化）
  - [x] `wake` / `talk.mode` — 唤醒/对话模式
  - [x] `web.login.start` / `web.login.wait` — WhatsApp QR 登录
  - 输出: `server_methods_browser.go` (600L)

- [x] **D-W1 验证**：`go build ./internal/gateway/... && go vet ./internal/gateway/...` ✅ 全通过

---

### D-W1 审计修复总结

| FIND | 文件 | 描述 | 状态 |
|------|------|------|------|
| 1 | cron.go | 默认 mode `force` | ✅ |
| 2 | cron.go | patch 从 `params.patch` | ✅ |
| 3 | cron.go | 响应 key `entries` | ✅ |
| 4 | skills.go | 全量 status 实现 | ✅ |
| 5 | skills.go | skillKey 参数名 | ✅ |
| 6 | skills.go | WriteConfigFile | ✅ |
| 7 | skills.go | workspace 解析 | ✅ |
| 8 | nodes.go | listDevicePairing 合并 | ✅ |
| 9 | nodes.go | command policy check | ✅ |
| 10 | nodes.go | 参数归一化 + callerId | ✅ |
| 11 | devices.go | summarizeDeviceTokens | ⏭️ 低优先级 |
| 12 | browser.go | method/path 参数 | ✅ |
| 13 | browser.go | method 验证 | ✅ |
| 14 | browser.go | node proxy + policy | ✅ |
| 15 | browser.go | 本地 dispatcher | ✅ |
| 16 | browser.go | 文件持久化 | ✅ |

**完成度：15/16（93.75%）**
