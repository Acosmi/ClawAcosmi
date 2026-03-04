# Go Gateway 端到端修复 + 权限系统 — 索引文档

> 最后更新: 2026-02-23 03:14
> 联网验证: ✅ 已通过 OWASP / Docker / Britive 可信源验证

---

## 文档关系图

| 文档 | 用途 | 路径 |
|------|------|------|
| **本文** (索引) | 全局导航 + 状态总览 | `docs/renwu/new-window-context-index.md` |
| 原始上下文 | 会话原始记录 | `docs/renwu/new-window-context.md` |
| 实施跟踪 | P0-P5 阶段步骤跟踪 | `docs/renwu/gateway-permission-tracker.md` |
| 中国频道 SDK | 飞书/钉钉/企业微信集成跟踪 | `docs/renwu/china-channel-sdk-tracker.md` |
| 权限系统设计 | 完整设计文档 | `~/.gemini/.../b614bf6f/.../implementation_plan.md` |
| 延迟项汇总 | 全局延迟待办 | `docs/renwu/deferred-items.md` |

---

## 总状态概览

| 模块 | 项数 | 完成 | 待做 | 优先级 |
|------|------|------|------|--------|
| 端到端修复 | 7 | 7 ✅ | 0 | — |
| P0 权限接入 | 6 步 | 6 ✅ | 0 | — |
| P1 UI 设置页 | 5 步 | 5 ✅ | 0 | — |
| P2 即时授权 | 7 步 | 7 ✅ | 0 | — |
| P3 规则引擎 | 4 步 | 0 | 4 ⚪ | P3 |
| P4 远程审批 | — | 0 | — ⚪ | P4 |
| P5 任务级权限 | — | 0 | — ⚪ | P5 |
| 沙箱组件 | 7 文件 | 已复制 | 待集成 | L1 |
| 延迟项 | 4 项 | 0 | 4 | P2-P3 |

---

## 关键代码入口

| 组件 | 关键文件 | 行号 |
|------|---------|------|
| 权限守卫 | `backend/internal/agents/runner/tool_executor.go` | L23-30, L38-47 |
| 权限来源 | `backend/internal/agents/runner/attempt_runner.go` | L172-175 |
| 配置验证 | `backend/internal/config/validator.go` | L251-262 |
| 模型选择 | `backend/internal/autoreply/reply/model_fallback_executor.go` | L37-58 |
| API Key | `backend/internal/agents/runner/attempt_runner.go` | L229-244 |
| LLM 客户端 | `backend/internal/agents/llmclient/openai.go` | L90-95 |
| 向导入口 | `backend/internal/gateway/wizard_onboarding.go` | L274, L385 |
| 沙箱入口 | `backend/internal/sandbox/docker_runner.go` | 全文件 |

---

## 相关延迟项速查

| ID | 描述 | 优先级 | 详情 |
|----|------|--------|------|
| GW-WIZARD-D1 | Google OAuth 模式缺失 | P2 | `deferred-items.md` |
| GW-WIZARD-D2 | 简化向导缺后续阶段 | P2 | `deferred-items.md` |
| GW-LLM-D1 | 其他 provider 格式验证 | P2 | `deferred-items.md` |
| GW-UI-D4 | 已连接实例列表为空 | P2 | `deferred-items.md` |

---

## 联网验证结论

所有 6 项核心设计决策均通过行业可信源验证：

1. **3 级权限** → OWASP Least Privilege + OpenAI Codex 3-Tier ✅
2. **Docker 沙箱** → CIS Docker Benchmark + Docker Hardening Guide ✅
3. **自动降权** → Britive JIT/ZSP + NIST Zero Trust ✅
4. **二次确认** → Human-in-the-Loop (NIST SP 800-207) ✅
5. **规则引擎** → ABAC/PBAC (Cerbos, OPA) ✅
6. **远程审批** → ServiceNow/Slack Mobile Approval ✅
