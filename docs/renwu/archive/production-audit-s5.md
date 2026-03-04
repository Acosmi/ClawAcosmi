# S5 审计：Phase 7 辅助模块

> 审计日期：2026-02-18

---

## 模块对照

| TS 模块 | 文件/行 | Go 模块 | 文件/行 | 比率 |
|---------|---------|---------|---------|------|
| auto-reply/ | 121/22,028 | autoreply/ | 90/15,204 | ✅ 69% |
| memory/ | 28/7,001 | memory/ | 21/4,893 | ✅ 70% |
| security/ | 8/4,028 | security/ | 8/2,438 | ⚠️ 61% |
| browser/ | 52/10,478 | browser/ | 12/1,881 | ❌ 18% |
| media/ + media-understanding/ | 36/5,394 | media/ | 26/4,080 | ✅ 76% |
| tts/ | 1/1,579 | tts/ | 8/1,881 | ✅ 119% |
| markdown/ | 6/1,461 | pkg/markdown/ | 6/1,688 | ✅ 116% |
| link-understanding/ | 6/268 | linkparse/ | 5/414 | ✅ 154% |

## 隐藏依赖

- **auto-reply/** → channels/plugins/ → agent 工具链
- **browser/** → Puppeteer/Playwright (Node 依赖)
- **memory/** → LLM 调用 (向量化) + 文件系统
- **security/** → 沙箱 + 审批系统

## 关键缺失

| 模块 | TS 行数 | Go 行数 | 缺失估算 |
|------|---------|---------|----------|
| **browser/** | 10,478 | 1,881 | ~8,600L |

**browser/ 缺失详情**：

- TS 依赖 Puppeteer/Playwright (52 文件)
- Go 的 chromedp/rod 替代仅实现基本框架
- 缺失：页面截图/PDF生成/网页抓取的完整管线

## Phase 7 评估

**真实完成度：~65%**

- ✅ autoreply/memory/media/tts/markdown/linkparse 完整
- ⚠️ security (61%) 需检查审批逻辑完整性
- ❌ **browser/ (18%)** 是最大缺口 (~8,600L 缺失)
