> 📄 分块 07/08 — D-W1b + D-W2 | 索引：phase13-task-00-index.md
>
> **D-W1b TS 源**：`src/gateway/server-methods/` + `src/gateway/protocol/` → **Go**：`backend/internal/gateway/`
> **D-W2 TS 源**：`src/agents/auth-profiles/` + `src/agents/skills/` + `src/agents/pi-extensions/` → **Go**：`backend/internal/agents/auth/` + `backend/internal/agents/skills/` + `backend/internal/agents/extensions/`

## 窗口 D-W1b：Gateway 非 Stub 方法补全（含前置侦察）

> 参考：`gap-analysis-part4b.md` D-W1b 节
> ⭐ **审计复核追加前置步骤**：protocol/ 工作量不确定，需先侦察再估算

### 🔍 前置侦察（D-W1b 开始前执行，约 15-30 分钟）

- [x] **D-W1b-PRE**: protocol/ 实际缺口评估

  ```bash
  # TS 端：统计 protocol/ 文件列表
  find src/gateway/protocol -name '*.ts' | xargs wc -l | sort -n
  # Go 端：检查 gateway/ 中已内联的协议定义
  grep -r "type.*Protocol\|type.*Message\|type.*Packet" backend/internal/gateway/ --include="*.go" -l
  ```

  - TS `protocol/` 20 文件 2,800L vs Go `protocol.go` 270L
  - 判断：大量内联到 `gateway/*.go` → 实际缺口可能远小于 ~1,500L
  - 根据侦察结果调整 D-W1b-T4 工作量

### 主要任务

- [x] **D-W1b-T1**: server_methods_config.go 补全
  - 当前 285L vs TS 460L
  - [x] 逐函数对比缺失方法（配置更新/覆盖/插件配置相关）

- [x] **D-W1b-T2**: server_methods_send.go 补全
  - 当前 227L vs TS 364L
  - [x] 补全消息发送路由/多频道分发逻辑

- [x] **D-W1b-T3**: server_methods_agent_rpc.go 补全
  - 当前 316L vs TS 515L
  - [x] 补全 Agent RPC 剩余方法

- [x] **D-W1b-T4**: protocol/ 补全（工作量由前置侦察决定）
  - 当前 `protocol.go` 270L vs TS `protocol/` 20 文件 2,800L
  - ⚠️ 可能部分已内联到 `gateway/*.go`，需深度对比确认实际缺口

- [x] **D-W1b 验证**：`go build ./internal/gateway/... && go test -race ./internal/gateway/...`

---

## 窗口 D-W2：Auth + Skills + Extensions（2 会话）

> 参考：`gap-analysis-part4f.md` D-W2 节
> ⭐ **审计复核修正**：`agents/schema/` 已移入 A-W1，本窗口不再包含

### 会话 D-W2a：Auth Profiles 补全

- [x] **D-W2-T1**: Auth Profiles 补全 (TS 15 文件 1,939L → Go 1 文件 344L)
  - TS 目录: `src/agents/auth-profiles/`
  - Go 目标: `internal/agents/auth/` 扩展
  - [x] OAuth 流程：`initiateOAuth()` / `handleCallback()` / `refreshToken()`
  - [x] 凭证存储：`saveCredential()` / `loadCredential()` / `deleteCredential()`
  - [x] 配置文件管理：`listProfiles()` / `switchProfile()` / `createProfile()`
  - [x] 使用量跟踪：`trackAuthUsage()`
  - 预计新增：~1,200L，5 个新 Go 文件

### 会话 D-W2b：Skills 三件套 + Extensions

- [x] **D-W2-T2**: Skills 三件套（来自 deferred-items.md SKILLS-1/2/3）
  - SKILLS-1: `frontmatter.go` + `eligibility.go`
    - [x] `ResolveOpenAcosmiMetadata()` / `ResolveSkillInvocationPolicy()` / `ShouldIncludeSkill()` / `HasBinary()`
  - SKILLS-2: `env_overrides.go` + `refresh.go` + `bundled_dir.go`
    - [x] `ApplySkillEnvOverrides()` / `EnsureSkillsWatcher()` / `BumpSkillsSnapshotVersion()` / `ResolveBundledSkillsDir()`
  - SKILLS-3: `install.go` + Gateway stubs 填充
    - [x] `InstallSkill()` / `UninstallSkill()` / `CheckSkillStatus()`
    - [x] `BuildWorkspaceSkillCommandSpecs()` / `SyncSkillsToWorkspace()`

- [x] **D-W2-T3**: pi-extensions (8 文件 1,019L)
  - TS 目录: `src/agents/pi-extensions/`
  - Go 目标: `internal/agents/extensions/`
  - [x] 扩展点注册机制 / 扩展生命周期（加载/卸载/热更新）/ 扩展上下文隔离

- [x] **D-W2 验证**：`go test -race ./internal/agents/auth/... ./internal/agents/skills/... ./internal/agents/extensions/...`

---
