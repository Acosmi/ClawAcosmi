# i18n 残留项启动上下文

> 用途：新窗口粘贴此文件内容作为初始 prompt

---

## 任务目标

完成前端 i18n 国际化改造的**最后 38 处残留硬编码字符串**。  
主体工作（Batch 1-7, ~250+ keys, 14 文件）已在前面的窗口完成。

## 关键文件路径

- **locale 文件**: `ui/src/ui/locales/zh.ts` / `en.ts`（已有 ~250+ keys）
- **i18n 基础设施**: `ui/src/ui/i18n.ts`（导出 `t()` 函数，支持 `t("key", { param })` 插值）
- **审计报告**: `docs/前端审计未改.md`（第三节逐行列出所有残留）

## 待处理文件清单 — ✅ 全部完成

### 1. `ui/src/ui/views/config-form.node.ts` (~11 处) — ✅ 已完成

### 2. `ui/src/ui/navigation.ts` (~8 处) — ✅ 已完成（分析前已 i18n）

### 3. `ui/src/ui/chat/grouped-render.ts` (~5 处) — ✅ 已完成

### 4. `ui/src/ui/views/markdown-sidebar.ts` (~3 处) — ✅ 已完成

### 5. `ui/src/ui/views/channels.nostr-profile-form.ts` (~3 处) — ✅ 已完成

### 6. `ui/src/ui/views/config-form.render.ts` (~2 处) — ✅ 已完成

### 7. `ui/src/ui/chat/tool-cards.ts` (~1 处) — ✅ 已完成

### 8. `ui/src/ui/views/usage.ts` 残留 (~5 处) — ✅ 已完成

## 工作模式

1. **添加 locale keys** → `zh.ts` + `en.ts`（文件末尾 `};` 前追加）
2. **替换字符串** → 多处用 `multi_replace_file_content`，单处用 `replace_file_content`
3. **构建验证** → `npx vite build`（在 `ui/` 目录下执行）
4. **注意事项**:
   - `usage.ts` L4367 有 `(t) =>` 变量遮蔽 i18n 的 `t()`，不要在那个回调内用 `t()`
   - 保持 `AllowMultiple: true` 当目标字符串在文件中有多处出现时
   - 每批替换后务必 build 验证

## Prompt 模板

```
请继续完成前端 i18n 国际化改造的残留项。
参考文件: docs/renwu/i18n-remaining-bootstrap.md
审计报告: docs/前端审计未改.md（第三节）
按优先级从高到低处理，每个文件处理完后 build 验证。
```
