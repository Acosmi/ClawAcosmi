---
document_type: Audit
status: Complete
created: 2026-02-28
scope: ui/src/ (166 TS files + 22 CSS files) + Argus/ (go-sensory/79, rust-core/11, web-console/26)
verdict: Pass with Notes
---

# 审计报告: ui (前端) + Argus (感知层)

## 范围

- `ui/src/ui/` — 47 顶层 TS 文件 + 9 子目录 (views/62, controllers/35, chat/15, locales/2)
- `ui/src/styles/` — 22 CSS 文件
- `Argus/go-sensory/` — 79 internal Go 文件 (感知采集)
- `Argus/rust-core/` — 11 Rust 文件 (capture/crypto/PII/input/metrics/imaging)
- `Argus/web-console/` — 26 文件 (Web控制台)
- `Argus/wails-console/` — 12 文件 (桌面应用)

## 审计发现

### [PASS] 架构: LitElement Web Component (ui/app.ts)

- **位置**: `app.ts:120-744`
- **分析**: `OpenAcosmiApp` 继承 `LitElement`，使用 `@customElement("openacosmi-app")` + `@state()` 响应式属性。50+ 方法组织为: 连接管理、聊天发送/中止、会话管理、主题/i18n、向导配置、频道切换、通知、语音等。`createRenderRoot` 返回 `this`（无 Shadow DOM），允许全局 CSS。
- **风险**: None

### [PASS] 安全: localStorage 类型安全加载 (ui/storage.ts)

- **位置**: `storage.ts:21-98`
- **分析**: `loadSettings` 对 localStorage 数据做全字段类型检验: `typeof` 校验 + 枚举白名单（theme: "light"/"dark"/"system"，locale: "zh"/"en"）+ 范围限制（splitRatio: 0.4-0.7）。JSON 解析失败返回 defaults。Token 存储在 localStorage 中（Web 控制台场景可接受，非高安全场景）。
- **风险**: None

### [PASS] 正确性: WebSocket 连接协议自适应 (ui/storage.ts)

- **位置**: `storage.ts:23-28`
- **分析**: 根据页面 `location.protocol` 自动选择 `ws://` 或 `wss://`。开发模式（Vite on 26222）通过 Vite proxy 转发 `/ws`，生产模式由 gateway 直接服务。
- **风险**: None

### [PASS] 正确性: 前端代码组织 (ui/)

- **位置**: 整体
- **分析**: 清晰的职责拆分:
  - `app-render.ts`(61KB) — 渲染逻辑
  - `app-gateway.ts`(18KB) — WS 通信
  - `app-view-state.ts`(14KB) — 视图状态管理
  - `app-settings.ts`(15KB) — 设置面板
  - `views/`(62 files) — 各视图面板
  - `controllers/`(35 files) — 业务控制器
  - `chat/`(15 files) — 聊天渲染逻辑
- **风险**: None

### [PASS] 架构: Argus 双语言感知层

- **位置**: `Argus/go-sensory/`(79 files), `Argus/rust-core/`(11 files)
- **分析**: Argus 采用 Go+Rust 混合架构:
  - **Rust core**: 高性能核心 — capture(屏幕捕获), capture_sck(ScreenCaptureKit), crypto(加密), pii(隐私擦除), imaging(图像处理), input(输入采集), metrics(度量), accessibility(无障碍), shm(共享内存), keyframe(关键帧)
  - **Go sensory**: 感知编排层 — 79 个文件处理采集调度、数据流、API 服务
  - **Web console**: Argus Web 可视化控制台
  - **Wails console**: 桌面版控制台（Wails 框架）
- **风险**: None

### [WARN] 正确性: app-render.ts 单文件过大

- **位置**: `app-render.ts` (61251 bytes)
- **分析**: 61KB 的单文件包含所有渲染逻辑。虽然 `app.ts` 已做了部分拆分（lifecycle/gateway/scroll/settings/channels/chat），但渲染本身仍在单文件中。
- **风险**: Low
- **建议**: 按视图/面板拆分为独立渲染函数文件。

### [WARN] 安全: Token 存储在 localStorage

- **位置**: `storage.ts:8`
- **分析**: Gateway token 存储在 `localStorage`。对于本地控制台场景（localhost 访问）风险可控。但如果通过 Tailscale/公网暴露，有 XSS → token 蛀取风险。
- **风险**: Low（本地场景可接受）
- **建议**: 如果支持公网访问，考虑使用 HttpOnly cookie 或 session token。

## 总结

- **总发现**: 7 (5 PASS, 2 WARN, 0 FAIL)
- **阻断问题**: 无
- **结论**: **通过（附注释）** — 前端架构清晰（LitElement + 职责拆分），Argus 双语言感知层设计合理。
