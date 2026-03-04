# 前端延迟项（Frontend Deferred Items）

> 独立于后端延迟项，记录前端 UI 层面的待办事项。

## FE-D5: 剩余 View 文件 i18n 字符串抽取

**来源**: Phase 10 Batch FE-D（i18n 基础设施）

本批次仅完成 MVP 字符串抽取（`navigation.ts`, `app-render.ts`），以下 view 文件仍包含硬编码英文字符串，需要在后续增量任务中逐步改造：

| # | 文件 | 预估字符串数 |
|---|------|------------|
| 1 | `views/overview.ts` | ~30 |
| 2 | `views/channels.ts` | ~25 |
| 3 | `views/sessions.ts` | ~15 |
| 4 | `views/config.ts` | ~40 |
| 5 | `views/debug.ts` | ~20 |
| 6 | `views/logs.ts` | ~15 |
| 7 | `views/cron.ts` | ~25 |
| 8 | `views/skills.ts` | ~20 |
| 9 | `views/nodes.ts` | ~15 |
| 10 | `views/agents.ts` | ~20 |
| 11 | `views/usage.ts` | ~30 |
| 12 | `views/instances.ts` | ~10 |
| 13 | `app-render.helpers.ts` (aria labels) | ~10 |

**建议策略**: 每次改造 2–3 个 view 文件，逐步扩展 `locales/zh.ts` 和 `locales/en.ts`。

## FE-D6: overview.ts 全页面 i18n 改造

**来源**: Phase 10 Batch FE-D 计划中列了 `overview.ts` 作为 view 改造模板，locale key 已预置但 view 文件本身尚未替换。

## FE-D7: 多语言格式化（日期/数字）

**来源**: Phase 10 Batch FE-D

目前数字和时间格式化未做本地化，可在后续批次中使用 `Intl.DateTimeFormat` / `Intl.NumberFormat` 处理。
