> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Browser TS→Go 文件映射表

**更新日期**: 2026-02-25

## 核心层

| TS 文件 | 行数 | Go 文件 | 状态 |
|---------|------|---------|------|
| src/browser/config.ts | 273 | internal/browser/config.go | ✅ 已修复 (BR-H08,H09) |
| src/browser/constants.ts | 8 | internal/browser/constants.go | ✅ 基本对齐 |
| src/browser/chrome.ts | 342 | internal/browser/chrome.go | ✅ 已修复 (BR-H02,H03,H04) |
| src/browser/chrome.executables.ts | 625 | internal/browser/chrome_executables.go | 🟡 延迟 (BR-H05~H07) |
| src/browser/chrome.profile-decoration.ts | 198 | internal/browser/chrome_profile_decoration.go | ✅ **新建** (BR-H01) |
| src/browser/profiles.ts | 113 | internal/browser/profiles.go | 🟡 部分 |
| src/browser/profiles-service.ts | 187 | *(无)* | 🟡 缺 service factory |
| src/browser/cdp.ts | 454 | internal/browser/cdp.go | ✅ 已修复 (BR-H10,H11,H12) |
| src/browser/cdp.helpers.ts | 173 | internal/browser/cdp_helpers.go | ✅ 基本对齐 |
| *(无 TS 对照)* | — | internal/browser/cdp_session.go | ✅ **新建** (国际标杆改进) |
| *(无 TS 对照)* | — | internal/browser/actionability.go | ✅ **新建** (国际标杆改进) |
| src/browser/extension-relay.ts | 790 | internal/browser/extension_relay.go | ✅ 已修复 (BR-H13~H16, M11+M12) |
| src/browser/target-id.ts | 30 | *(无)* | 低优先 |
| src/browser/screenshot.ts | 57 | *(无)* | 低优先 |

## Playwright 工具层

| TS 文件 | 行数 | Go 文件 | 状态 |
|---------|------|---------|------|
| *(接口定义)* | — | internal/browser/pw_tools.go (522行) | ✅ 21 方法全部实现 (BR-M07+M08) |
| src/browser/pw-tools-core.interactions.ts | 546 | internal/browser/pw_tools_cdp.go (1471行) | ✅ **100%** (BR-H17~H24, M06, M15) |
| src/browser/pw-tools-core.shared.ts | 70 | internal/browser/pw_tools_shared.go | ✅ **完成** |
| src/browser/pw-tools-core.downloads.ts | 251 | internal/browser/pw_tools_cdp.go | ✅ 已修复 (BR-H25) |
| src/browser/pw-tools-core.storage.ts | 128 | internal/browser/pw_tools_cdp.go | ✅ 已修复 (BR-M02+M03) |
| src/browser/pw-tools-core.responses.ts | 123 | internal/browser/pw_tools_cdp.go | ✅ 已修复 (BR-H26) |
| src/browser/pw-tools-core.state.ts | 209 | internal/browser/pw_tools_state.go (268行) | ✅ **新建** (BR-M01) |
| src/browser/pw-tools-core.trace.ts | 37 | internal/browser/pw_tools_activity.go (499行) | ✅ **新建** (BR-M05) |
| src/browser/pw-tools-core.activity.ts | 68 | internal/browser/pw_tools_activity.go | ✅ **新建** (BR-M04) |
| src/browser/pw-tools-core.snapshot.ts | 205 | internal/browser/pw_tools_cdp.go | ✅ 已实现 |
| src/browser/pw-role-snapshot.ts | 427 | internal/browser/pw_role_snapshot.go | ✅ **95%** |
| src/browser/pw-session.ts | 629 | internal/browser/pw_playwright.go + pw_playwright_browser.go | ✅ BR-M07+M08 事件监听+roleRef缓存 |
| src/browser/pw-ai.ts + pw-ai-module.ts | 111 | internal/browser/pw_ai_loop.go + pw_ai_vision.go | ✅ **完成** |
| *(驱动工厂)* | — | internal/browser/pw_driver.go | ✅ **完成** |

## HTTP 路由层

| TS 文件 | 行数 | Go 文件 | 状态 |
|---------|------|---------|------|
| src/browser/server.ts | 109 | internal/browser/server.go (~25路由) | ✅ 已修复 (BR-M09) |
| src/browser/bridge-server.ts | 76 | *(合并至 server.go)* | ✅ 已修复 (BR-M09) |
| src/browser/control-service.ts | 88 | *(合并至 server.go)* | ✅ 已修复 (BR-M09) |
| src/browser/server-context.ts | 668 | internal/browser/session.go | 🟡 延迟 (BR-H27) |
| src/browser/routes/agent.act.ts | 541 | internal/browser/agent_routes.go (901行) | ✅ 已修复 (BR-H28, M09) |
| src/browser/routes/agent.snapshot.ts | 329 | *(合并至 agent_routes.go)* | ✅ 已实现 |
| src/browser/routes/agent.debug.ts | 151 | *(合并至 agent_routes.go)* | ✅ 已修复 (BR-M09) |
| src/browser/routes/agent.storage.ts | 435 | *(合并至 agent_routes.go)* | ✅ 已实现 |
| src/browser/client.ts | 337 | internal/browser/client.go | ✅ ~80% (BR-M10) |
| src/browser/client-fetch.ts | 110 | internal/browser/client_actions.go (331行) | ✅ 已修复 (BR-M10) |

## 新增文件（超越 TS 原始实现）

| Go 文件 | 行数 | 说明 | 参考来源 |
|---------|------|------|----------|
| cdp_session.go | ~250 | 持久 CDP WebSocket + EventBus | browser-use, Rod |
| actionability.go | ~130 | 5 点可操作性检查 | Playwright |
| chrome_profile_decoration.go | ~170 | Profile UI 定制 | TS chrome.profile-decoration.ts |
| pw_tools_state.go | 268 | 8 个页面仿真函数 (BR-M01) | TS pw-tools-core.state.ts |
| pw_tools_activity.go | 499 | 诊断+追踪 (BR-M04+M05) | TS activity.ts + trace.ts |
| browser-international-benchmark.md | — | 国际标杆研究报告 | 研究文档 |

## 量化统计

| 维度 | 初审 | HIGH修复后 | MEDIUM修复后 |
|------|------|--------|--------|
| Go 文件数 | 23 | 29 | 31 |
| Go 代码行数 | ~5,340 | ~7,944 | ~9,582 |
| TS 代码行数 | ~10,478 | ~10,478 | ~10,478 |
| 行数覆盖率 | 51% | 76% | ~91% |
| 功能对齐度 | ~40-45% | ~70-75% | ~85-90% |
| HTTP 路由 | ~13 | ~18 | ~25 |
| PW Tools 方法 | 9 实现 | 16 实现 | 21 全部实现 |
| Client Actions | ~30% | ~30% | ~80% |
