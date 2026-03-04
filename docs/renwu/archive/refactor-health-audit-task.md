# 重构健康度全局审计 — 任务总表

> 创建: 2026-02-17
> 阶段: Phase 11 — 重构质量自检
> 方法论: `/refactor` 六步循环法 + 隐藏依赖审计

---

## 概述

对比 TS 原项目代码与 Go 重构代码，逐模块执行健康度审计。
每个模块在**独立窗口**中执行，使用对应的 bootstrap 文档恢复上下文。

> [!IMPORTANT]
> 此审计不是重新重构，而是**自检 + 修复**。重点是发现 Go 移植中的遗漏和偏差。

---

## 模块 A: Gateway 方法 (P0) — 窗口 1

> Bootstrap: `docs/renwu/phase11-gateway-methods-bootstrap.md`

- [x] 步骤 1 — 提取: 列出 TS `server-methods/` 41 文件的 export 清单
- [x] 步骤 2 — 依赖图: 各方法处理器的显式+传递依赖
- [x] 步骤 3 — 隐藏依赖审计 (7 项检查)
- [x] 步骤 4 — 差异分析: 逐方法对比 TS vs Go 行为
  - [x] chat.ts (22KB) vs server_methods_chat.go
  - [x] sessions.ts (15KB) vs server_methods_sessions.go
  - [x] usage.ts (27KB) vs server_methods_usage.go (⚠️ 27→3KB)
  - [x] agents.ts (15KB) vs server_methods_agents.go (⚠️ 15→3KB)
  - [x] send.ts (12KB) vs server_methods_send.go
  - [x] nodes.ts (17KB) — stub 状态确认
  - [x] 其余方法一致性检查
- [x] 步骤 5 — 修复发现的偏差（审计报告已记录，修复延至后续窗口）
- [x] 步骤 6 — 验证: `go build` + `go vet` + `go test`
- [x] 产出: `phase11-gateway-methods-audit.md`
- [x] 更新 `refactor-plan-full.md` Phase 11 模块 A 状态

---

## 模块 B: Session 管理 (P0) — 窗口 2

> Bootstrap: `docs/renwu/phase11-session-mgmt-bootstrap.md`

- [x] 步骤 1 — 提取: TS `session-utils.ts` + `config/sessions/` 的 export 清单
- [x] 步骤 2 — 依赖图: session 模块的依赖拓扑
- [x] 步骤 3 — 隐藏依赖审计 (7 项检查)
- [x] 步骤 4 — 差异分析:
  - [x] 存储模型: TS 磁盘 JSON vs Go 内存 Map (⚠️ 根因)
  - [x] `loadSessionEntry()` 全链路对比
  - [x] `resolveSessionStoreKey()` key 规范化
  - [x] Transcript 路径解析一致性
  - [x] `loadCombinedSessionStoreForGateway()` 合并逻辑
  - [x] `sessions.resolve` / `sessions.patch` RPC 行为
- [x] 步骤 5 — 修复建议（P0-P2 分级行动项）
- [x] 步骤 6 — 验证
- [x] 产出: `phase11-session-mgmt-audit.md`
- [x] 更新 `refactor-plan-full.md` Phase 11 模块 B 状态

---

## 模块 C: AutoReply 管线 (P1) — 窗口 3

> Bootstrap: `docs/renwu/phase11-autoreply-bootstrap.md`

- [x] 步骤 1 — 提取: TS `auto-reply/` 70+ 文件的核心 export 清单
- [x] 步骤 2 — 依赖图: autoreply → agents → gateway 依赖链
- [x] 步骤 3 — 隐藏依赖审计 (7 项检查)
- [x] 步骤 4 — 差异分析:
  - [x] commands-registry (17KB→6KB) 缺失命令盘点
  - [x] envelope (7KB→1KB) 封装逻辑完整性
  - [x] status (21KB→10KB) typing/状态广播
  - [x] chunk 分块算法等价性
  - [x] reply/ 子模块 (139→47 文件) 功能覆盖
  - [x] dispatch 入口消息分发链
- [x] 步骤 5 — 修复
- [x] 步骤 6 — 验证
- [x] 产出: `phase11-autoreply-audit.md`
- [x] 更新 `refactor-plan-full.md` Phase 11 模块 C 状态

---

## 模块 D: Agent Runner (P1) — 窗口 4

> Bootstrap: `docs/renwu/phase11-agent-runner-bootstrap.md`

- [x] 步骤 1 — 提取: TS `agents/` 核心 runner 文件的 export 清单
- [x] 步骤 2 — 依赖图: runner → models → llmclient 依赖链
- [x] 步骤 3 — 隐藏依赖审计 (7 项检查)
- [x] 步骤 4 — 差异分析:
  - [x] pi-embedded-runner (25 TS→9 Go) 核心逻辑
  - [x] model-fallback 降级/重试策略
  - [x] model-selection 模型选择逻辑
  - [x] model-auth 认证 Profile 轮换
  - [x] system-prompt (648L→278L) 构建逻辑
  - [x] bash-tools (2295L→185L) 工具执行
  - [x] pi-embedded-subscribe (618L) 流式订阅
- [x] 步骤 5 — 修复建议 (3 P0 + 5 P1 + 5 P2 行动项)
- [x] 步骤 6 — 验证 (`go build` + `go vet` + `go test` 全通过)
- [x] 产出: `phase11-agent-runner-audit.md`
- [x] 更新 `refactor-plan-full.md` Phase 11 模块 D 状态

---

## 模块 E: WS 协议 (P2) — 窗口 5

> Bootstrap: `docs/renwu/phase11-ws-protocol-bootstrap.md`

- [x] 步骤 1 — 提取: TS `ws-connection.ts` + `ws-types.ts` 的 export 清单
- [x] 步骤 2 — 依赖图: WS 帧类型和协议依赖
- [x] 步骤 3 — 隐藏依赖审计 (7 项检查)
- [x] 步骤 4 — 差异分析:
  - [x] 帧格式一致性 (connect/hello-ok/request/response/event)
  - [x] 心跳参数匹配 (ping 间隔/pong 超时)
  - [x] 重连策略 (前端 GatewayBrowserClient 参数)
  - [x] 错误码一致性 (4008/1006/1012)
  - [x] 连接数限制和背压处理
  - [x] WS 频繁断连 (~20s) 根因排查
- [x] 步骤 5 — 修复建议已记录
- [x] 步骤 6 — 验证 (go build/vet/test 通过)
- [x] 产出: `phase11-ws-protocol-audit.md`
- [x] 更新 `refactor-plan-full.md` Phase 11 模块 E 状态

---

## 模块 F: Config/Scope (P2) — 窗口 6

> Bootstrap: `docs/renwu/phase11-config-scope-bootstrap.md`

- [x] 步骤 1 — 提取: TS `config/` 88 非测试文件 + `agent-scope.ts` 的 export 清单
- [x] 步骤 2 — 依赖图: config → schema → defaults 依赖链
- [x] 步骤 3 — 隐藏依赖审计 (7 项检查) — 全部 ✅
- [x] 步骤 4 — 差异分析:
  - [x] schema.ts (1114L) → schema.go (166L) — ⚠️ UI Hints 缩减 (P2)
  - [x] io.ts (616L) → loader.go (694L) — ✅ 完整管线
  - [x] defaults.ts (470L) → defaults.go (473L) — ✅ 1:1 对等
  - [x] agent-scope.ts (192L) → scope.go (306L) — ✅ 12 函数全覆盖
  - [x] env-substitution.ts (134L) → envsubst.go (150L) — ✅ 完全等价
  - [x] 渠道特定配置 — 类型已移至 `pkg/types/`，非 config 包范围
  - [x] Zod schemas (3091L) → validator.go (193L) — ⚠️ struct tags 替代 (P2)
- [x] 步骤 5 — 修复: 无 P0/P1 项需修复，3 项 P2 延迟
- [x] 步骤 6 — 验证: `go build` + `go vet` + `go test -race` 全通过
- [x] 产出: `phase11-config-scope-audit.md`
- [x] 更新 `refactor-plan-full.md` Phase 11 模块 F 状态

---

## 已知问题（从调试中发现）

| # | 问题 | 模块 | 状态 |
|---|------|------|------|
| 1 | SessionStore 内存空存储 vs TS 磁盘读取 | B | ✅ 已修复 |
| 2 | ModelResolver 依赖缺失 | D | ✅ 已修复 |
| 3 | DeepSeek max_tokens 超限 | D | ✅ 已修复 |
| 4 | autoDetectProvider 缺失 | C | ✅ 已修复 |
| 5 | 用户消息不持久化 | B | ✅ 已修复 |
| 6 | WS 频繁断连 (~20s) | E | ⏳ 待查 |

---

## 完成标准

- [ ] 6 个模块全部审计完成
- [ ] 所有 ⚠️/❌ 项已修复或有明确方案
- [ ] `go build ./...` + `go vet ./...` + `go test ./...` 全部通过
- [ ] 6 份审计报告全部产出
- [ ] `refactor-plan-full.md` 更新 Phase 11 状态
