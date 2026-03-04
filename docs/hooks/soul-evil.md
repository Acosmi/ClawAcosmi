---
summary: "SOUL Evil Hook（用 SOUL_EVIL.md 替换 SOUL.md）"
read_when:
  - 需要启用或调整 SOUL Evil Hook
  - 需要净化窗口或随机概率的人格替换
title: "SOUL Evil Hook"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# SOUL Evil Hook

SOUL Evil Hook 在净化窗口期间或按随机概率将**注入的** `SOUL.md` 内容替换为 `SOUL_EVIL.md`。它**不会**修改磁盘上的文件。

## 工作原理

当 `agent:bootstrap` 运行时，此 Hook 可以在系统提示词组装之前替换内存中的 `SOUL.md` 内容。如果 `SOUL_EVIL.md` 缺失或为空，OpenAcosmi 会记录警告并保留正常的 `SOUL.md`。

Sub-agent 运行**不**在其引导文件中包含 `SOUL.md`，因此此 Hook 对 Sub-agent 无效。

## 启用

```bash
openacosmi hooks enable soul-evil
```

然后设置配置：

```json
{
  "hooks": {
    "internal": {
      "enabled": true,
      "entries": {
        "soul-evil": {
          "enabled": true,
          "file": "SOUL_EVIL.md",
          "chance": 0.1,
          "purge": { "at": "21:00", "duration": "15m" }
        }
      }
    }
  }
}
```

在 Agent 工作区根目录（与 `SOUL.md` 同级）创建 `SOUL_EVIL.md`。

## 选项

- `file`（字符串）：替代的 SOUL 文件名（默认：`SOUL_EVIL.md`）
- `chance`（数字 0–1）：每次运行使用 `SOUL_EVIL.md` 的随机概率
- `purge.at`（HH:mm）：每日净化开始时间（24 小时制）
- `purge.duration`（时长）：窗口持续时间（例如 `30s`、`10m`、`1h`）

**优先级：** 净化窗口优先于随机概率。

**时区：** 设置时使用 `agents.defaults.userTimezone`；否则使用主机时区。

## 注意事项

- 不会在磁盘上写入或修改任何文件。
- 如果 `SOUL.md` 不在引导列表中，此 Hook 不会执行任何操作。

## 另请参阅

- [Hooks](/automation/hooks)
