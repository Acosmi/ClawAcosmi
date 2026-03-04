---
summary: "CLI reference for `openacosmi webhooks` (webhook helpers + Gmail Pub/Sub)"
read_when:
  - You want to wire Gmail Pub/Sub events into OpenAcosmi
  - You want webhook helper commands
title: "webhooks"
status: active
arch: rust-cli
---

> [!NOTE]
> **架构状态：✅ 已适配** — Rust CLI crate (oa-cmd-webhooks) + Go Gateway stub 均已注册。
> 命令功能为 stub 实现，待后续补充完整业务逻辑。

# `openacosmi webhooks`

Webhook helpers and integrations (Gmail Pub/Sub, webhook helpers).

Related:

- Webhooks: [Webhook](/automation/webhook)
- Gmail Pub/Sub: [Gmail Pub/Sub](/automation/gmail-pubsub)

## Gmail

```bash
openacosmi webhooks gmail setup --account you@example.com
openacosmi webhooks gmail run
```

See [Gmail Pub/Sub documentation](/automation/gmail-pubsub) for details.
