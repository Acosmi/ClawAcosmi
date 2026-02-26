# Go Gateway 端到端修复 + 权限系统 — 实施跟踪文档

> 创建时间: 2026-02-23 02:20
> 来源: `docs/renwu/new-window-context.md`
> 联网验证: ✅ 已对照 OWASP LLM Top 10、Docker 官方安全指南、Britive JIT/ZSP 模式

---

## 验证摘要（联网可信源）

| 设计决策 | 行业对照 | 验证结论 |
|---------|---------|---------|
| L0/L1/L2 三级权限 | OpenAI Codex 3 级模型 (suggest/auto/full)、OWASP Least Privilege | ✅ 完全对齐 |
| Docker `--read-only` + `--no-new-privileges` | Docker 官方 Hardening Guide、CIS Benchmark | ✅ 行业标准 |
| 自动降权（任务后恢复 L0）| Britive JIT Access / Zero Standing Privileges (ZSP) | ✅ 最佳实践 |
| 永久授权需二次确认 | NIST Zero Trust、Human-in-the-Loop 模式 | ✅ 合规要求 |
| Allow/Ask/Deny 规则引擎 | ABAC/PBAC 策略引擎 (Cerbos, Auth0) | ✅ 行业趋势 |
| 远程审批（飞书/微信）| Mobile Approval Workflow (ServiceNow, Slack) | ✅ 企业级模式 |

---

## 一、已完成修复（7 项 ✅）

> 所有修复已通过 `go build` + `go vet`，DeepSeek 对话功能正常

| # | 问题 | 修复文件 | 状态 |
|---|------|---------|------|
| 1 | 向导配置不持久化 (`gateway.bind: "loopback"` 验证失败) | `validator.go` L253 | ✅ |
| 2 | 模型选择忽略配置 (`autoDetectProvider()` 只查环境变量) | `model_fallback_executor.go` L37-58 | ✅ |
| 3 | API Key 不从配置读取 | `attempt_runner.go` L229-244 | ✅ |
| 4 | 向导保存模型无 provider 前缀 | `wizard_onboarding.go` L385-393 | ✅ |
| 5 | UI 转圈永不停止 (runId 不匹配) | `server_methods_chat.go` L226-230 | ✅ |
| 6 | DeepSeek 工具调用报错 (content omitempty) | `openai.go` L90-95, L116-183 | ✅ |
| 7 | 配置路径混乱 (~/.openclaw → ~/.openacosmi) | 文件系统操作 | ✅ |

---

## 二、权限系统实施跟踪

### P0：AllowWrite/AllowExec 接入配置（✅ 完成 — 2026-02-23 实施通过）

> 行业对照：OWASP Least Privilege + OpenAI Codex 3-Tier — 静态配置接入

**目标**：将硬编码的 `AllowWrite=false, AllowExec=false` 改为从 `tools.exec.security` 配置动态读取。

**根因定位**：

| 文件 | 行号 | 问题 |
|------|------|------|
| `backend/internal/agents/runner/attempt_runner.go` | L172-175 | `ToolExecParams{}` 未设置 AllowWrite/AllowExec，Go 零值 = false |
| `backend/internal/agents/runner/tool_executor.go` | L27-29 | 结构体中 AllowWrite/AllowExec 默认 false |

**修复步骤**：

- [x] **P0-1**: 在 `attempt_runner.go` 新增 `resolveAllowWrite(cfg)` 和 `resolveAllowExec(cfg)` 函数
- [x] **P0-2**: 读取 `cfg.Tools.Exec.Security` 字段，映射：`"full"` → L2, `"allowlist"/"sandbox"` → L1, `"deny"/"off"/默认` → L0
- [x] **P0-3**: 修改 L172 `ExecuteToolCall` 调用，传入解析后的权限值
- [x] **P0-4**: 确认 `OpenAcosmiConfig.Tools.Exec.Security` 类型定义（`types_tools.go` L165: `string`）
- [x] **P0-5**: 编写 9 个单元测试验证 3 种安全级别映射 + 权限拒绝 + 边界情况（nil/empty/sandbox 别名）
- [x] **P0-6**: `go build` ✅ + `go vet` ✅ + `go test -race` 全部 PASS

**安全级别映射表**（已验证对齐 OWASP Least Privilege + OpenAI Codex 3-Tier）：

| 配置值 | 级别 | AllowWrite | AllowExec | 行业对照 |
|--------|------|-----------|----------|---------|
| `"off"` / 默认 | L0 只读 | false | false | Codex suggest mode |
| `"sandbox"` | L1 沙盒 | false | true | Codex auto mode |
| `"full"` | L2 完全 | true | true | Codex full mode |

> [!IMPORTANT]
> 当前用户配置 `~/.openacosmi/openacosmi.json` 中 `tools.exec.security = "full"`，P0 完成后智能体将获得完全权限。

---

### P1：UI 安全设置页 + 永久授权开关（✅ 完成 — 2026-02-23 复核通过）

> 行业对照：NIST Zero Trust — 永久授权需 Human-in-the-Loop 确认

**步骤**：

- [x] **P1-1**: 前端新增「安全设置」页面（`ui/src/ui/views/security.ts` + `controllers/security.ts`）
- [x] **P1-2**: 显示当前安全级别 (L0/L1/L2) + 切换控件
- [x] **P1-3**: 永久授权开关：启用时弹出风险提示 + 二次确认（输入 CONFIRM）
- [x] **P1-4**: 后端 API：`security.get` 聚合查询 + `exec.approvals.set` 写入
- [x] **P1-5**: 配置变更后热加载生效（复用 exec-approvals 写入路径）

---

### P2：智能体即时授权 + UI 弹窗 + 自动降权（✅ 完成 — 2026-02-23 实施通过）

> 行业对照：Britive JIT Access / ZSP — 任务级临时权限 + 自动过期

**步骤**：

- [x] **P2-1**: 后端提权管理器 `permission_escalation.go`（请求/审批/拒绝/TTL 定时器/自动降权）
- [x] **P2-2**: Gateway API 方法注册 `server_methods_escalation.go`（request/resolve/status/audit/revoke）
- [x] **P2-3**: TTL 自动降权（默认 30 分钟，定时器 + 任务完完成回调）
- [x] **P2-4**: 审计日志 `escalation_audit.go`（JSON Lines 格式 `~/.openacosmi/escalation-audit.log`）
- [x] **P2-5**: 前端弹窗 `escalation-popup.ts` + 控制器 `escalation.ts`（WebSocket 事件 `esc_` 前缀区分）
- [x] **P2-6**: i18n 翻译（en + zh 各 13 个 `security.escalation.*` 键）
- [x] **P2-7**: 13 个单元测试全部通过（含 `-race`）

---

### P3：Allow/Ask/Deny 命令规则引擎（已完成 ✅）

> 行业对照：ABAC/PBAC 策略引擎 (Cerbos, OPA)

**步骤**：

- [x] **P3-1**: 定义规则 schema — `CommandRule` 类型 + `ExecApprovalsDefaults.Rules` 字段
- [x] **P3-2**: 规则匹配引擎 — `command_rule_engine.go` (glob/前缀/子串/多段通配符) + 集成到 `tool_executor.go`
- [x] **P3-3**: 预设安全规则集 — 19 条内置规则 (10 deny, 3 ask, 6 allow)
- [x] **P3-4**: 用户自定义规则 CRUD — 后端 `security.rules.*` API + 前端 UI + i18n (en/zh)

**验证**: `go build` ✅ + `go vet` ✅ + 23 新测试 PASS + 回归测试 PASS

---

### P4-P5：远程审批 + 任务级预设权限（远期 ⚪）

| 阶段 | 内容 | 行业对照 |
|------|------|---------|
| **P4** | 飞书/微信/钉钉卡片审批 + WebHook 回调 | ServiceNow Mobile Approval |
| **P5** | 任务模板预绑定权限级别 | Terraform Sentinel Policy |

---

## 三、沙箱组件清单

> 路径: `backend/internal/sandbox/`（已从 Chat 项目复制）
> 安全验证: Docker `--read-only` + `--no-new-privileges` ✅ 对齐 CIS Docker Benchmark

| 文件 | 大小 | 用途 | 复用阶段 |
|------|------|------|---------|
| `docker_runner.go` | 6.6KB | Docker 安全沙箱 | L1 (P2) |
| `sandbox_worker.go` | 13.8KB | Worker Pool 调度 | L1 (P2) |
| `container_pool.go` | 12KB | 容器池管理 | L1 (P2) |
| `ws_progress.go` | 4.8KB | WebSocket 进度推送 | L1 (P2) |
| `wasm_runner.go` | 4KB | Rust FFI Wasm 桥 | 未来 |
| `sandbox_test.go` | 5KB | 测试 | L1 |
| `container_pool_test.go` | 6.6KB | 测试 | L1 |
