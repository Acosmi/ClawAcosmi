# 审计修复任务跟踪

> 创建日期：2026-02-16
> 状态：完成（Batch C 已延迟）
> 来源：[audit-20260216-global-deep.md](file:///Users/fushihua/Desktop/Argus-compound/docs/renwu/audit-20260216-global-deep.md)

## 背景与目标

2026-02-16 全局深度审计发现 3 个 P1 + 6 个 P2 问题。按修复难度分 3 个 Batch 执行。

## 任务清单

### Batch A: `accessibility.rs` 内存安全修复 (P1 × 2)

| 状态 | 编号 | 任务 | 关联发现 |
|------|------|------|----------|
| ✅ | A1 | `write_json_output` → `into_boxed_slice` + `Box::into_raw` | BUG-D2-01 |
| ✅ | A2 | `ax_get_children` → 重构消除 UAF 风险 | BUG-D2-02 |
| ✅ | A3 | `argus_request_screen_capture` unsafe 块修复 | STYLE-D2-03 |
| ✅ | A4 | `cargo build --release` 验证零 warning | — |

### Batch B: Go 侧快速修复 (P1 + P2)

| 状态 | 编号 | 任务 | 关联发现 |
|------|------|------|----------|
| ✅ | B1 | `router.go` defer body close 顺序修复 | BUG-D2-03 |
| ✅ | B2 | `dashboard.go` panic → log.Fatalf | STYLE-D2-02 |
| ✅ | B3 | `go build` + `go vet` 验证 | — |

### Batch C: P2 可选修复

| 状态 | 编号 | 任务 | 关联发现 |
|------|------|------|----------|
| ⏭️ | C1 | Gemini health key 从 URL 移除 | SEC-D2-01 |
| ⏭️ | C2 | VLM 错误路径 LimitReader | PERF-D2-01 |
| ⏭️ | C3 | CGO LDFLAGS 去重 | ARCH-D2-01 |

> **状态标记**：⬜ 待做 → 🔄 进行中 → ✅ 完成 → ⏭️ 跳过/延迟

## 完成后需更新的文档

- [x] `docs/renwu/audit-20260216-global-deep.md` — 标记已修复项
- [x] `docs/renwu/bootstrap-audit-fixes.md` — 标记已完成 Batch
- [x] `docs/renwu/deferred-items.md` — 登记跳过/延迟项（Batch C 全部延迟）

## 风险与注意事项

- `ax_get_children` 重构需保持 AX 遍历功能不变
- Batch C 中 Gemini API Key Header 传递需确认 Google API 是否支持
- 文件拆分 (STYLE-D2-01) 不在本计划范围

## 变更记录

| 日期 | 变更内容 |
|------|----------|
| 2026-02-16 | 初始创建，从 plan 重构为 task 格式 |
| 2026-02-17 | Batch A 全部完成 (A1-A4)，`accessibility.rs` 内存安全修复 |
| 2026-02-17 | Batch B 全部完成 (B1-B3)，Go 侧快速修复: `router.go` defer 顺序 + `dashboard.go` panic 替换 |
| 2026-02-17 | Batch C (C1-C3) 全部挂起为延迟项，登记至 `deferred-items.md`。任务关闭 |
