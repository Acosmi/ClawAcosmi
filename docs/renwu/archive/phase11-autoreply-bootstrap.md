# 模块 C: AutoReply 管线 — 审计 Bootstrap

> 用于新窗口快速恢复上下文

---

## 新窗口启动模板

```
请执行 AutoReply 管线模块的重构健康度审计。

## 上下文
1. 读取审计总表: `docs/renwu/refactor-health-audit-task.md`
2. 读取本 bootstrap: `docs/renwu/phase11-autoreply-bootstrap.md`
3. 读取 `/refactor` 技能工作流
4. 读取编码规范: `skills/acosmi-refactor/references/coding-standards.md`
5. 读取 `docs/renwu/deferred-items.md`
6. 控制输出量：预防上下文过载引发崩溃，需要大量输出时请逐步分段输出。
7. 任务完成后：请按要求更新 `refactor-plan-full.md` 和本模块的审计报告。

## 目标
对比 TS 原版 `src/auto-reply/` 与 Go 移植 `backend/internal/autoreply/`。

> **注意**: 具体审计步骤请严格参考 `docs/renwu/refactor-health-audit-task.md` 模块 C 章节。此文档仅提供上下文和文件索引。
```

---

## TS 源文件 (核心, 排除测试)

| 文件 | 大小 | 职责 |
|------|------|------|
| `status.ts` | 21KB | ⭐ 状态广播、typing 指示 |
| `chunk.ts` | 15KB | ⭐ 流式分块逻辑 |
| `commands-registry.ts` | 15KB | 命令注册中心 |
| `commands-registry.data.ts` | 17KB | 内置命令数据 |
| `dispatch.ts` | 3KB | 消息分发入口 |
| `envelope.ts` | 7KB | 消息封装/拆封 |
| `heartbeat.ts` | 5KB | 心跳/typing 定时器 |
| `templating.ts` | 6KB | 模板渲染 |
| `thinking.ts` | 7KB | 思考过程处理 |
| `command-auth.ts` | 8KB | 命令权限检查 |
| `skill-commands.ts` | 4KB | 技能相关命令 |
| `send-policy.ts` | 1KB | 发送策略 |
| `inbound-debounce.ts` | 3KB | 入站消息去抖 |
| `reply/` | 139 子文件 | ⭐ Reply 子模块 (完整回复管线) |

## Go 对应文件 (`backend/internal/autoreply/`)

| 文件 | 大小 | 对应 TS |
|------|------|---------|
| `status.go` | 10KB | `status.ts` (21KB → 10KB ⚠️) |
| `chunk.go` | 12KB | `chunk.ts` |
| `commands_registry.go` | 6KB | `commands-registry.ts` (15KB → 6KB ⚠️) |
| `commands_data.go` | 6KB | `commands-registry.data.ts` (17KB → 6KB ⚠️) |
| `dispatch.go` | 1KB | `dispatch.ts` |
| `envelope.go` | 1KB | `envelope.ts` (7KB → 1KB ⚠️) |
| `heartbeat.go` | 5KB | `heartbeat.ts` |
| `templating.go` | 3KB | `templating.ts` |
| `thinking.go` | 9KB | `thinking.ts` |
| `skill_commands.go` | 5KB | `skill-commands.ts` |
| `reply/` | 47 子文件 | `reply/` (139 → 47 ⚠️) |

## 关键审计点

1. **命令注册表缩减**: 17KB → 6KB，哪些命令缺失？
2. **Envelope 大幅缩减**: 7KB → 1KB，消息封装逻辑是否完整？
3. **Status 逻辑缩减**: 21KB → 10KB，typing 指示和状态广播是否完整？
4. **Reply 子模块**: 139 → 47 子文件，缺失的 92 个文件中有多少是测试文件 vs 功能文件？
5. **Dispatch 入口**: 消息分发链与 TS 原版是否一致？
6. **Streaming 分块**: chunk.go 与 chunk.ts 的切割算法是否等价？

## 已知问题

- `autoDetectProvider` 已在 `model_fallback_executor.go` 添加
