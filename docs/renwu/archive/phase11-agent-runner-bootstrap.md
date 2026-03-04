# 模块 D: Agent Runner — 审计 Bootstrap

> 用于新窗口快速恢复上下文

---

## 新窗口启动模板

```
请执行 Agent Runner 模块的重构健康度审计。

## 上下文
1. 读取审计总表: `docs/renwu/refactor-health-audit-task.md`
2. 读取本 bootstrap: `docs/renwu/phase11-agent-runner-bootstrap.md`
3. 读取 `/refactor` 技能工作流
4. 读取编码规范: `skills/acosmi-refactor/references/coding-standards.md`
5. 读取 `docs/renwu/deferred-items.md`
6. 控制输出量：预防上下文过载引发崩溃，需要大量输出时请逐步分段输出。
7. 任务完成后：请按要求更新 `refactor-plan-full.md` 和本模块的审计报告。

## 目标
对比 TS 原版 `src/agents/` 与 Go 移植 `backend/internal/agents/`。

> **注意**: 具体审计步骤请严格参考 `docs/renwu/refactor-health-audit-task.md` 模块 D 章节。此文档仅提供上下文和文件索引。
```

---

## TS 源文件 (核心 runner 相关, 排除测试)

| 文件群 | 大小 | 职责 |
|--------|------|------|
| `pi-embedded-runner/` | 32 子文件 | ⭐ 嵌入式 PI 执行器核心 |
| `pi-embedded-subscribe.ts` | 22KB | ⭐ 流式订阅与事件处理 |
| `pi-embedded-utils.ts` | 12KB | 工具/消息规范化 |
| `pi-embedded-block-chunker.ts` | 10KB | 块分割器 |
| `pi-embedded-helpers/` | 9 子文件 | 错误分类、消息清理 |
| `model-fallback.ts` | 11KB | ⭐ 模型降级/重试策略 |
| `model-selection.ts` | 13KB | ⭐ 模型选择逻辑 |
| `model-auth.ts` | 12KB | ⭐ 认证 Profile 轮换 |
| `model-scan.ts` | 14KB | 模型发现与扫描 |
| `model-catalog.ts` | 5KB | 模型目录 |
| `system-prompt.ts` | 27KB | ⭐ 系统 prompt 构建 |
| `bash-tools.exec.ts` | 54KB | ⭐⭐ Bash 工具执行 (最大文件) |
| `bash-tools.process.ts` | 21KB | 进程管理 |
| `agent-scope.ts` | 6KB | Agent 作用域解析 |
| `compaction.ts` | 11KB | 上下文压缩 |
| `cli-credentials.ts` | 17KB | CLI 认证管理 |
| `skills-install.ts` | 16KB | 技能安装管理 |
| `tool-policy.ts` | 8KB | 工具权限策略 |

## Go 对应文件 (`backend/internal/agents/`)

| 目录/文件 | 文件数 | 对应 TS |
|-----------|--------|---------|
| `runner/` | 11 文件 | `pi-embedded-runner/` 核心逻辑 |
| `models/` | 14 文件 | `model-*.ts` 系列 |
| `scope/` | 8 文件 | `agent-scope.ts` |
| `session/` | 6 文件 | session-* 相关 |
| `llmclient/` | 6 文件 | SDK 适配层 |
| `compaction/` | 2 文件 | `compaction.ts` |
| `transcript/` | 4 文件 | transcript 操作 |

## 关键审计点

1. **runner 文件数**: TS 32+子文件 → Go 11 文件，缩减严重
2. **bash-tools**: 54KB 最大文件，Go 端是否有等价实现？（可能在 `exec/`）
3. **model-fallback**: 降级/重试策略是否完整移植？
4. **auth-profile 轮换**: 多 API key 轮换逻辑
5. **system-prompt**: 27KB 系统 prompt 构建，Go 端如何处理？
6. **pi-embedded-subscribe**: 流式订阅处理 22KB，关键路径

## 已知问题

- `ModelResolver` 依赖注入已修复 (`model_resolver_env.go`)
- DeepSeek `max_tokens` 上限已修复 (`attempt_runner.go`)
- `autoDetectProvider` 已添加
