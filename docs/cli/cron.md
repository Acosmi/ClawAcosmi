---
summary: "定时任务调度（通过 Gateway scheduler 管理 cron job）"
read_when:
  - You want scheduled / recurring messages
  - You need wake-ups via the Gateway scheduler
title: "cron"
status: active
arch: go-backend
source: cmd_cron.go
---

> [!WARNING]
> **架构状态：⚠️ Go Gateway 端实现** — 对应 `backend/cmd/openacosmi/cmd_cron.go`。
> 此命令通过 Go CLI（已弃用）或 Gateway RPC 调用。Rust CLI 尚未迁移此子命令。

# `openacosmi cron`

Manage cron jobs for the Gateway scheduler.

Related:

- Cron jobs: [Cron jobs](/automation/cron-jobs)

Tip: run `openacosmi cron --help` for the full command surface.

Note: isolated `cron add` jobs default to `--announce` delivery. Use `--no-deliver` to keep
output internal. `--deliver` remains as a deprecated alias for `--announce`.

Note: one-shot (`--at`) jobs delete after success by default. Use `--keep-after-run` to keep them.

Note: recurring jobs now use exponential retry backoff after consecutive errors (30s → 1m → 5m → 15m → 60m), then return to normal schedule after the next successful run.

## Common edits

Update delivery settings without changing the message:

```bash
openacosmi cron edit <job-id> --announce --channel telegram --to "123456789"
```

Disable delivery for an isolated job:

```bash
openacosmi cron edit <job-id> --no-deliver
```

Announce to a specific channel:

```bash
openacosmi cron edit <job-id> --announce --channel slack --to "channel:C1234567890"
```
