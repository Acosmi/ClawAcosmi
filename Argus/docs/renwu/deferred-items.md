# 延迟项清单

> 创建日期：2026-02-17
> 状态：活跃
> 来源：审计修复任务 Batch C 挂起

## 延迟项

### 来自 audit-20260216-global-deep 审计报告 — Batch C (P2 可选修复)

| # | 原编号 | 描述 | 优先级 | 原因 |
|---|--------|------|--------|------|
| DF-AUD-01 | SEC-D2-01 (C1) | Gemini health check API Key 从 URL query string 移除，改用 Header 传递 | P2 | 需确认 Google Gemini API 是否支持 Header 方式传递 key |
| DF-AUD-02 | PERF-D2-01 (C2) | VLM 错误路径 `io.ReadAll` → `io.LimitReader` 限制读取量 | P2 | 非 bug，优化项 |
| DF-AUD-03 | ARCH-D2-01 (C3) | CGO LDFLAGS 去重，消除 linker duplicate rpath warnings | P2 | 不影响功能，构建噪音 |

### 来自 audit-20260216-global-deep 审计报告 — STYLE-D2-01

| # | 原编号 | 描述 | 优先级 | 原因 |
|---|--------|------|--------|------|
| DF-AUD-04 | STYLE-D2-01 | 20 个文件超 300 行，需按功能域拆分 | P2 | 独立重构任务，工作量大 |

## 变更记录

| 日期 | 变更内容 |
|------|----------|
| 2026-02-17 | 初始创建，登记 Batch C (C1-C3) + STYLE-D2-01 为延迟项 |
