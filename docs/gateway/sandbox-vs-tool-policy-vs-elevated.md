---
title: "沙箱 vs 工具策略 vs 提权"
summary: "工具为何被阻止：沙箱运行时、工具允许/拒绝策略和提权 exec 门控"
read_when: "遇到沙箱限制或工具/提权拒绝时需要准确的配置键。"
status: active
---

# 沙箱 vs 工具策略 vs 提权

> [!IMPORTANT]
> **架构状态**：工具策略由 **Go Gateway**（`backend/internal/agents/runner/tool_policy.go`）执行，
> 沙箱由 **Rust** 原生实现（`cli-rust/crates/oa-sandbox/`），提供 OS 级进程隔离。

三种相关但不同的控制：

1. **沙箱**（`agents.defaults.sandbox.*`）决定**工具在哪里运行**（Rust 原生 OS 隔离 vs 宿主机，Docker 仅作回退）。
2. **工具策略**（`tools.*`）决定**哪些工具可用/允许**。
3. **提权**（`tools.elevated.*`）是**仅限 exec 的逃逸舱口**。

## 快速调试

```bash
openacosmi sandbox explain
openacosmi sandbox explain --session agent:main:main
```

## 沙箱：工具在哪里运行

由 `agents.defaults.sandbox.mode` 控制。详见 [沙箱](/gateway/sandboxing)。

## 工具策略：哪些工具可调用

- **工具 Profile**：`tools.profile`（基础白名单）
- **全局/按 Agent 策略**：`tools.allow`/`tools.deny`
- **沙箱工具策略**（仅沙箱内生效）：`tools.sandbox.tools.allow`/`deny`

规则：`deny` 始终优先。`allow` 非空时其他被阻止。

### 工具组

```json5
{ tools: { sandbox: { tools: { allow: ["group:runtime", "group:fs"] } } } }
```

可用组：`group:runtime`、`group:fs`、`group:sessions`、`group:memory`、`group:ui`、`group:automation` 等。

## 提权：仅限 exec 的"在宿主机运行"

- 沙箱中 `/elevated on` 在宿主机运行 `exec`。
- 不授予额外工具，不覆盖工具策略。

门控：`tools.elevated.enabled` + `tools.elevated.allowFrom.<provider>`

## 常见修复

### "Tool X 被沙箱策略阻止"

- 禁用沙箱：`agents.defaults.sandbox.mode=off`
- 或在沙箱内允许工具：添加到 `tools.sandbox.tools.allow`

### "以为是主会话但被沙箱了"

`"non-main"` 模式下群组/频道键不是 main。使用 `sandbox explain` 查看。
