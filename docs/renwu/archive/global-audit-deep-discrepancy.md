# 深度差异审计报告 — 高差异模块

> 审计日期：2026-02-19 | 审计类型：函数级深度对照
> 目标：排查 TS↔Go 存在巨大差异的区域

---

## 差异严重程度排序

| 排名 | 模块 | TS 导出函数 | Go 函数 | 函数覆盖率 | 严重程度 |
|------|------|-----------|---------|----------|---------|
| 🔴 1 | **browser** | ~170 | 32 | **18.8%** | **高** |
| 🟡 2 | cli+commands | 174 文件 | 124 Cobra 子命令 | **~71%** | **中** |
| 🟢 3 | infra | 310 | 248 | **80.0%** | **低** |
| 🟢 4 | outbound | 49 | 34 | **69.4%** | **低** |

---

## 🔴 browser — 最大差异区域

### 当前 Go 实现状态

**Go `server.go` 仅注册 6 个 HTTP 路由**:

```
POST /navigate     → handleNavigate
POST /screenshot   → handleScreenshot  
POST /evaluate     → handleEvaluate
GET  /status       → handleStatus
POST /launch       → handleLaunch
POST /close        → handleClose
```

### TS 有但 Go 完全缺失的浏览器工具端点 (~35 个)

| 分类 | 缺失的工具函数 | 优先级 |
|------|--------------|--------|
| **标签管理** | `browserOpenTab`, `browserCloseTab`, `browserFocusTab`, `browserTabs`, `browserTabAction` | P1 |
| **Cookie/存储** | `browserCookies`, `browserCookiesClear`, `browserCookiesSet`, `browserStorageGet`, `browserStorageSet`, `browserStorageClear` | P1 |
| **网络观测** | `browserRequests`, `browserResponseBody`, `browserConsoleMessages`, `browserPageErrors` | P1 |
| **设备模拟** | `browserSetDevice`, `browserSetGeolocation`, `browserSetLocale`, `browserSetTimezone`, `browserSetMedia`, `browserSetOffline`, `browserSetHeaders`, `browserSetHttpCredentials` | P2 |
| **PDF/下载** | `browserPdfSave`, `browserDownload`, `browserWaitForDownload` | P2 |
| **快照/调试** | `browserSnapshot`, `browserHighlight`, `browserTraceStart`, `browserTraceStop` | P1 |
| **权限/配置** | `browserClearPermissions`, `browserStart`, `browserStop` (完整版) | P2 |
| **Profile 管理** | `browserCreateProfile`, `browserDeleteProfile`, `browserResetProfile`, `browserProfiles` | P2 |

### Playwright/CDP 函数层（~60 个）

Go 使用 CDP 直接调用替代 Playwright 高级 API，但以下 CDP 操作层函数未实现：

| 分类 | 缺失函数 | 说明 |
|------|---------|------|
| **交互操作** | `clickViaPlaywright`, `hoverViaPlaywright`, `dragViaPlaywright`, `fillFormViaPlaywright`, `selectOptionViaPlaywright`, `typeViaPlaywright`, `pressKeyViaPlaywright`, `scrollIntoViewViaPlaywright` | 需用 CDP Input domain 实现 |
| **页面操作** | `navigateViaPlaywright`, `evaluateViaPlaywright`, `waitForViaPlaywright`, `resizeViewportViaPlaywright` | 部分在 handleNavigate/handleEvaluate |
| **快照** | `snapshotAiViaPlaywright`, `snapshotAriaViaPlaywright`, `snapshotDom`, `snapshotRoleViaPlaywright`, `formatAriaSnapshot`, `buildRoleSnapshotFromAiSnapshot` | ARIA/AI 快照系统完全缺失 |
| **文件/对话框** | `armDialogViaPlaywright`, `armFileUploadViaPlaywright`, `setInputFilesViaPlaywright`, `downloadViaPlaywright` | 对话框/文件交互缺失 |
| **Cookie/Storage CDP** | `cookiesGetViaPlaywright`, `cookiesSetViaPlaywright`, `cookiesClearViaPlaywright`, `storageGetViaPlaywright`, `storageSetViaPlaywright`, `storageClearViaPlaywright` | Cookie Storage CDP 操作缺失 |
| **截图/PDF** | `takeScreenshotViaPlaywright`, `screenshotWithLabelsViaPlaywright`, `pdfViaPlaywright`, `captureScreenshotPng` | 高级截图功能缺失 |
| **追踪** | `traceStartViaPlaywright`, `traceStopViaPlaywright` | 性能追踪缺失 |

### Agent AI 交互（~20 个）

| 缺失函数 | 说明 |
|---------|------|
| `browserAct` | AI Agent 浏览器自动操作入口 |
| `browserArmDialog` / `browserArmFileChooser` | 对话框/文件选择自动处理 |
| `registerBrowserAgentActRoutes` | Agent Act 路由注册 |
| `registerBrowserAgentSnapshotRoutes` | Agent 快照路由注册 |
| `registerBrowserAgentStorageRoutes` | Agent 存储路由注册 |
| `registerBrowserAgentDebugRoutes` | Agent 调试路由注册 |
| `createBrowserControlContext` | 控制上下文创建 |
| `getPwAiModule` / `requirePwAi` | Playwright AI 模块加载 |
| `ensureContextState` / `ensurePageState` | 页面状态管理 |
| `getProfileContext` / `resolveProfileContext` | Profile 上下文解析 |

### browser 差异总结

| 优先级 | 数量 | 描述 |
|--------|------|------|
| **P0** | 0 | — |
| **P1** | **~20** | 标签管理、Cookie/Storage、网络观测、快照/调试 |  
| **P2** | **~15** | 设备模拟、PDF/下载、Profile 管理 |
| **P3** | **~60** | Playwright 底层函数（可用 CDP 等价实现） |

---

## 🟡 cli+commands — 中等差异

### 实际覆盖状态（对初始评估的修正）

初始评估 "312 TS 文件 → 30 Go 文件" 造成了误导。实际：

- Go 注册了 **124 个 Cobra 子命令**（`Use:` 字段计数）
- TS 174 个命令文件中很多是 helper/shared 文件（非独立命令）
- **实际命令覆盖率 ~71%**（vs 初始估计的 7.4%）

### 已覆盖的命令

Go 已有的核心命令：`agent` (run/send/list/add/delete/identity), `browser` (status/open/close/sessions/extension/screenshot/navigate/click/type/evaluate), `channels` (list/status/configure), `cron` (list/start/stop/run/logs), `daemon` (install/start/stop/status/uninstall/restart/logs), `doctor` (check), `gateway` (start/stop/status/sessions/health), `hooks` (list/enable/disable/test/info/add/remove), `infra` (network/dns/heartbeat/pairing/system/state), `models` (list/set), `nodes` (list/add/remove/ssh/status/start/stop/restart), `plugins` (list/install/remove/update/enable/disable/info), `security` (audit/sandbox/explain), `setup`, `skills` (list/add/remove/info/update), `status` (status/events)

### 缺失的命令/功能模块

| 缺失 | TS 文件数 | 说明 | 优先级 |
|------|---------|------|--------|
| **onboard 向导** | ~18 个 | `onboard*.ts` 交互式首次设置向导 | P2 |
| auth-choice 系列 | ~16 个 | OAuth/API Key 认证选择流程 | P2 |
| doctor 子模块 | ~15 个 | 高级诊断检查 (state-integrity, legacy-config, workspace) | P2 |
| configure 子模块 | ~8 个 | 交互式配置向导 (wizard, channels, daemon) | P3 |
| dashboard | 1 个 | Web dashboard 命令 | P3 |
| reset | 1 个 | 重置命令 | P3 |

---

## 🟢 infra — 低差异（修正）

- TS 310 函数 vs Go 248 函数，**80% 函数覆盖率**
- 缺失的 ~62 个函数主要来自：
  - `update-*.ts` (×6) — 自更新机制，Go 不需要（包管理器更新）
  - `clipboard.ts`, `wsl.ts`, `is-main.ts` 等 Node.js 特有
  - `format-datetime.ts`, `format-duration.ts` — Go `time` 包原生
- **实际功能差异极小**，核心子系统（heartbeat, exec-approvals, state-migrations, provider-usage, bonjour, ports）100% 覆盖

---

## 🟢 outbound — 低差异（修正）

- TS 49 函数 vs Go 34 函数，**69.4% 函数覆盖率**
- 缺失函数分析：
  - `getChannelMessageAdapter` → Go 通过 DI 接口替代（无需适配器模式）
  - `formatGatewaySummary`, `formatOutboundPayloadLog` → logging 简化
  - `listConfiguredMessageChannels` → config 包直接提供
  - `resolveChannelTarget` → `outbound/session.go` 中的 per-channel resolve 函数覆盖
  - `executeSendAction`/`executePollAction` → `send.go` 中 Send/SendPoll 方法
- **Go 额外有 15 个 per-channel session resolver**（resolveDiscordSession 等）
- **实际功能差异极小**

---

## 修正后的全局审计评级

| 模块 | 初始评级 | 深度审计后评级 | 变化 |
|------|---------|-------------|------|
| browser | B | **C+** | ⬇️ 降级 — 函数级缺失严重 |
| cli+commands | B- | **B** | ⬆️ 升级 — 实际 124 命令 |
| infra | B | **B+** | ⬆️ 升级 — 80% 函数覆盖 |
| outbound | implicit A- | **A-** | 不变 |

## 结论

**browser 模块是唯一存在真正显著功能差异的区域**。Go 目前仅实现了最基本的 6 个路由端点，而 TS 有 ~40 个浏览器工具端点 + ~60 个底层 Playwright/CDP 操作 + ~20 个 Agent AI 交互函数。

其他高差异模块（cli, infra, outbound）经深度审计后确认差异主要来自架构选型或 Node.js 特有功能，实际功能覆盖远高于行数比率暗示的水平。

### 建议优先修复

1. **P1 browser 工具端点** (~20 个核心端点) — 如果生产需要浏览器工具
2. **P2 onboard/auth-choice 交互向导** — 改善首次用户体验
3. **P2 browser 设备模拟/PDF/下载/Profile** — 完善浏览器功能集
