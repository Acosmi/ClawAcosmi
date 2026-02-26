package browser

// pw_ai_stub.go — pw-ai 功能实现说明
//
// TS 对照: browser/pw-ai.ts + pw-ai-module.ts
//
// Go 端实现分布:
//
//  1. ARIA 角色快照: pw_role_snapshot.go (✅ FULL)
//  2. AI 视觉循环: pw_ai_loop.go — observe→plan→act 引擎 (✅ FULL)
//     - AIBrowseLoop: Google DeepMind Mariner 风格的自动化循环
//     - AIPlanner 接口: 委托给 Agent Runner LLM 管道
//  3. Vision 提示构建: pw_ai_vision.go (✅ FULL)
//     - BuildVisionPrompt: 多模态 LLM Vision 提示构建
//     - AnnotateSnapshotForAI: ARIA 快照标注
//     - 支持 en/zh 双语提示模板
//  4. 核心浏览器操作: pw_tools_cdp.go (✅ FULL) + pw_playwright.go (✅ FULL)
//  5. 驱动管理: pw_driver.go — CDP/Playwright 双驱动管理器 (✅ FULL)
//  6. BrowserController 桥接: pw_playwright_browser.go (✅ FULL)
//
// 参考审计: global-audit-browser.md BRW-2
// 完成于: Phase 5-5 Sprint 3 (2026-02-23)
