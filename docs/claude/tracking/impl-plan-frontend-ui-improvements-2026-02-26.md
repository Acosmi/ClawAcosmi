---
document_type: Tracking
status: Draft
created: 2026-02-26
last_updated: 2026-02-26
audit_report: Pending
skill5_verified: true
---

# 前端两项 UI 改进：任务处理卡片 + 频道快捷切换

## Online Verification Log

### Skeleton / Progress Indicator UX
- **Query**: skeleton screen loading indicator UX best practices
- **Source**: https://www.nngroup.com/articles/skeleton-screens/
- **Key finding**: Skeleton 和进度指示优于 spinner；左→右动画感知更快；2-10 秒操作用循环指示器
- **Verified date**: 2026-02-26

### Tab / Channel Switch UX
- **Query**: tabs channel switching UX design patterns
- **Source**: https://www.eleken.co/blog-posts/tabs-ux / https://getstream.io/blog/chat-ux/
- **Key finding**: 标签切换需即时无重载；频道切换需清晰导航；分类帮助用户更快定位会话
- **Verified date**: 2026-02-26

---

## 改动概览

| # | 改动 | 涉及文件 |
|---|------|----------|
| 1 | 任务处理动画卡片 | `grouped-render.ts`, `views/chat.ts`, `components.css`, `i18n.ts` |
| 2 | 频道会话快捷切换按钮 | `app-render.helpers.ts`, `components.css` |

---

## 改动 1：任务处理动画卡片

### 设计

将现有 reading-indicator（三点跳动）和 streaming group（流式文本）包装为一个**处理中卡片**：
- 卡片顶部：shimmer 脉冲圆点 + "任务处理中" 文字 + 已用时间（实时 elapsed）
- 卡片底部：流式输出文本（复用 `renderStreamingGroup` 渲染），无内容时显示三点动画
- 任务完成后卡片消失，显示正常消息

### 任务分解

- [ ] 1.1 在 `grouped-render.ts` 新增 `renderProcessingCard()` 函数
  - 参数：`stream: string | null`, `startedAt: number`, `onOpenSidebar?`, `assistant?`
  - 计算 elapsed time → `formatElapsed()`
  - 有流式内容时复用 `renderGroupedMessage()`，否则渲染 reading dots
- [ ] 1.2 在 `views/chat.ts` 的 `buildChatItems` 中替换原 stream/reading-indicator 渲染分支
  - 新增 `streamStartedAt` 状态（时间戳，发送消息时记录）
  - 用 `renderProcessingCard()` 替代直接渲染 reading-indicator 和 streaming group
  - 管理 `setInterval` timer：开始时启动，结束时清除（`disconnectedCallback` 兜底）
- [ ] 1.3 在 `components.css` 新增 `.chat-processing-card` 相关样式
  - `.chat-processing-card` 外壳：圆角、边框、overflow hidden
  - `__header`：flex 布局、muted 背景、底部分隔线
  - `__shimmer`：脉冲圆点动画 `processingPulse`（1.5s ease-in-out infinite）
  - `__time`：tabular-nums、margin-left auto
  - `__body`：内容区 padding
- [ ] 1.4 i18n key（如有国际化机制）：`chat.processing` / `chat.processingTime`

### CSS 关键样式

```css
.chat-processing-card {
  border-radius: 12px;
  border: 1px solid var(--border);
  overflow: hidden;
}
.chat-processing-card__header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: var(--bg-muted);
  border-bottom: 1px solid var(--border);
  font-size: 0.8rem;
  color: var(--muted);
}
.chat-processing-card__shimmer {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: var(--accent);
  animation: processingPulse 1.5s ease-in-out infinite;
}
@keyframes processingPulse {
  0%, 100% { opacity: 0.4; transform: scale(0.8); }
  50% { opacity: 1; transform: scale(1.2); }
}
.chat-processing-card__time {
  margin-left: auto;
  font-variant-numeric: tabular-nums;
}
.chat-processing-card__body {
  padding: 10px 14px;
}
```

### 核心函数签名

```typescript
// grouped-render.ts
export function renderProcessingCard(
  stream: string | null,
  startedAt: number,
  onOpenSidebar?: (msgId: string) => void,
  assistant?: AssistantIdentity,
): TemplateResult;

function formatElapsed(ms: number): string;
// → "0:05", "1:23", "12:05"
```

---

## 改动 2：频道会话快捷切换按钮

### 设计

在 `renderChatControls()` 的 session 下拉旁，条件性显示"← 网页"按钮：
- 仅当 `sessionKey` 匹配频道前缀（`feishu:`, `slack:`, `whatsapp:` 等）时显示
- 点击后切回主会话（`main` 或上次活跃的网页会话）
- 小 pill 按钮，带左箭头

### 任务分解

- [ ] 2.1 在 `app-render.helpers.ts` 的 `renderChatControls()` 中添加频道检测逻辑
  - 正则 `/^(feishu|slack|whatsapp|nostr|telegram|discord|signal|dingtalk|wecom|imessage|googlechat):/`
  - 条件渲染按钮，复用已有 session switch 逻辑
- [ ] 2.2 在 `components.css` 新增 `.chat-controls__back-btn` 样式
  - 小号 pill：`border-radius: 999px`, `padding: 2px 10px`, `font-size: 0.75rem`
  - 与 session select 对齐

### 实现伪代码

```typescript
// app-render.helpers.ts — renderChatControls() 内
const isChannelSession = /^(feishu|slack|whatsapp|nostr|telegram|discord|signal|dingtalk|wecom|imessage|googlechat):/.test(state.sessionKey);

// session select 后面插入：
${isChannelSession ? html`
  <button class="btn btn--sm chat-controls__back-btn"
    @click=${() => { /* 切回 main session, 复用已有 handler */ }}
    title="返回网页聊天">← 网页</button>
` : nothing}
```

---

## 验证方法

| 场景 | 预期 |
|------|------|
| 发送消息后 | 处理卡片出现，脉冲圆点 + "任务处理中" + elapsed time 每秒递增 |
| 流式文本到达 | 卡片 body 实时显示流式内容 |
| 任务完成 | 卡片消失，正常消息展示 |
| 切到飞书会话 | session 下拉旁出现"← 网页"按钮 |
| 点击"← 网页" | 切回主会话，按钮消失 |
| 在主会话时 | 不显示"← 网页"按钮 |
| `npm run build` | 无编译错误 |

---

## 风险 & 注意事项

- `setInterval` timer 需在组件卸载时清除，防止内存泄漏
- elapsed time 使用 `requestUpdate()` 刷新，避免强制全量 re-render
- 频道前缀列表需与 `channels.types.ts` 保持同步
- 处理卡片要正确继承 dark/light 主题变量
