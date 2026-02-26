# 全局审计进度总览 (Global Audit Index)

本索引用于追踪所有模块的审计与重构对齐进度。包含 `backend/internal`、`backend/pkg` 以及所有 `src/` 下的核心模块。

## 审计模块清单

### 超大型模块 (>30,000L) - 建议：2-3个子窗口

- [x] [agents](./global-audit-agents.md) (TS: 539, Go: 0 (Decentralized))
- [x] [channels & channel-impls](./global-audit-channels.md) (TS: ~40000, Go: 37052)
  *(涵盖 telegram, slack, discord, whatsapp, line, signal 等具体实现)*

### 大型模块 (10,000L - 30,000L) - 建议：单独窗口

- [x] [commands](./global-audit-commands.md) (TS: 28567, Go: 4811)
- [x] [gateway](./global-audit-gateway.md) (TS: 26457, Go: 21669)
- [x] [infra](./global-audit-infra.md) (TS: 23279, Go: 9219)
- [x] [autoreply](./global-audit-autoreply.md) (TS: 22028, Go: 17531) — **B+** (2026-02-22 复核修正: P1×2, P2×2, P3×2)
- [x] [cli](./global-audit-cli.md) (TS: 21105, Go: 1147)
- [x] [config](./global-audit-config.md) (TS: 14329, Go: 8114)
- [x] [browser](./global-audit-browser.md) (TS: 10478, Go: 3962)

### 中型模块 (2,000L - 10,000L) - 建议：1-2个/窗口

- [x] [memory](./global-audit-memory.md) (TS: 7001, Go: 5147)
- [x] [plugins](./global-audit-plugins.md) (TS: 5780, Go: 4410)
- [x] [web](./global-audit-web.md) (TS: 5696, Go: 2878)
- [x] [tui](./global-audit-tui.md) (TS: 4155, Go: 6105)
- [x] [security](./global-audit-security.md) (TS: 4028, Go: 3960)
- [x] [hooks](./global-audit-hooks.md) (TS: 3914, Go: 4703)
- [x] [cron](./global-audit-cron.md) (TS: 3767, Go: 3711)
- [x] [daemon](./global-audit-daemon.md) (TS: 3554, Go: 2924)
- [x] [media-understanding](./global-audit-media-understanding.md) (TS: 3436, Go: 待定)

### 小型基础模块 (<2,000L) - 建议：3个/窗口

- [x] [media](./global-audit-media.md) (TS: 1958, Go: ~4200)
- [x] [wizard](./global-audit-wizard.md) (TS: 1722, Go: 待定)
- [ ] [onboarding](./global-audit-onboarding.md) (TS: 9775, Go: 2651) — **C（需补全）** → [修复跟踪](./onboarding-fix-task.md)
- [x] [tts](./global-audit-tts.md) (TS: 1579, Go: 1881)
- [x] [log](./global-audit-log.md) (TS: 1503, Go: 562)
- [x] [markdown](./global-audit-markdown.md) (TS: 1461, Go: 1688)
- [x] [nodehost](./global-audit-nodehost.md) (TS: 1380, Go: 2795)
- [x] [acp](./global-audit-acp.md) (TS: 1196, Go: 2030)
- [x] [utils](./global-audit-utils.md) (TS: 821, Go: 263)
- [x] [terminal](./global-audit-terminal.md) (TS: 744, Rust: 776) — **A** (2026-02-23 全量复审: Rust `oa-terminal` 完整覆盖)
- [x] [canvas](./global-audit-canvas.md) (TS: 733, Go: 974)
- [x] [routing](./global-audit-routing.md) (TS: 646, Go: 448)
- [x] [process](./global-audit-process.md) (TS: 513, Go: 0) — **B** (2026-02-23 全量复审: Node.js 平台绑定，Go 原生替代)
- [x] [providers](./global-audit-providers.md) (TS: 411, Go: 370)
- [x] [plugin-sdk](./global-audit-plugin-sdk.md) (TS: 382, Go: 145)
- [x] [macos](./global-audit-macos.md) (TS: 343, Go: 0) — **A** (2026-02-23 全量复审: Go 二进制已完全替代)
- [x] [pairing](./global-audit-pairing.md) (TS: 523, Go: 256 infra/) — **C** (2026-02-23 新增: 部分覆盖但语义差异)
- [x] [polls](./global-audit-polls.md) (TS: 69, Go: 0) — **C** (2026-02-23 新增: 纯验证逻辑，安全延迟)
- [x] [sessions](./global-audit-sessions.md) (TS: 330, Go: 1683)
- [x] [linkparse](./global-audit-linkparse.md) (TS: 268, Go: 414)
- [x] [types](./global-audit-types.md) (TS: 165, Go: 3085)
- [x] [contracts](./global-audit-contracts.md) (TS: 0, Go: 452)
- [x] [retry](./global-audit-retry.md) (TS: 239, Go: 239)
- [x] [session](./global-audit-session.md) (TS: 0, Go: 150)
- [x] [i18n](./global-audit-i18n.md) (TS: 0, Go: 149) — **A** (2026-02-22 /fuhe 通过，D1-D6 已全量修复)
- [x] [outbound](./global-audit-outbound.md) (TS: ~4000, Go: 2193)
- [x] [ollama](./global-audit-ollama.md) (TS: 0, Go: 173)

---

## 修复计划文档

- [修复优先级计划](./post-audit-remediation-plan.md) — P0~P3 分层修复计划（W-FIX-0 至 W-FIX-8）
- [深度对比分析](./post-audit-deep-comparison.md) — 初稿与审计源的逐项交叉校验
- [执行跟踪](./post-audit-task.md) — 按窗口组织的 checklist（W-FIX-0~AR 全部 ✅）
- [Onboarding 补齐跟踪](./onboarding-fix-task.md) — OB-1~OB-13 初始化引导全量补齐（4 窗口）
- [后端 i18n 国际化](./backend-i18n-task.md) — W1~W5 全部完成 ✅
- [延期项明细](./deferred-items.md) — 历次窗口延期任务汇总
