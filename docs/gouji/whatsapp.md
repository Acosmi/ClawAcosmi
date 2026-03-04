# WhatsApp SDK 架构文档

> 最后更新：2026-02-26 | 代码级审计确认 | 17 源文件, 2 测试文件, 50 测试, ~2,893 行

## 一、模块概述

WhatsApp Web 频道适配器，负责 WhatsApp 消息收发。基于 Baileys (TS) 的功能移植到 Go，运行时 WebSocket 连接通过 DI 接口注入（[whatsmeow](https://github.com/tulir/whatsmeow) 或其他实现）。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件 | 大小 | 职责 |
|------|------|------|
| `src/whatsapp/normalize.ts` | 80L | 号码规范化 |
| `src/web/` | 40+ 文件 5696L | WhatsApp Web 全量逻辑 |

### 核心逻辑摘要

- Baileys WebSocket 连接 + QR 码登录
- 入站消息监控 + 去重 + 路由
- 出站消息发送（含轮询状态检查）
- 媒体处理（HEIC→JPEG、PNG 优化）
- 自动回复引擎集成
- auth store 凭证持久化

## 三、依赖分析

### 显式依赖图

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `pkg/types` | 类型 | ↓ | 共享类型（OpenAcosmiConfig、MarkdownTableMode） |
| `internal/config` | 值 | ↓ | 配置读取（OAuth 目录） |
| `internal/autoreply` | 类型 | ↓ | MsgContext 消息上下文 |
| `internal/autoreply/reply` | 函数 | ↓ | FinalizeInboundContext |
| `pkg/markdown` | 函数 | ↓ | ConvertMarkdownTables |
| `pkg/utils` | 函数 | ↓ | NormalizeE164, NormalizeWhatsAppTarget |

### 隐藏依赖审计

| 类别 | 结果 | Go 等价方案 |
|------|------|-------------|
| npm 包黑盒行为 | ⚠️ → ✅ | Baileys WebSocket → DI `WhatsAppMonitorDeps` 接口 |
| 全局状态/单例 | ⚠️ → ✅ | auth store `WAWebAuthDir` 模块级变量 |
| 事件总线/回调链 | ⚠️ → ✅ | DI `HandleInboundMessageFull` 回调管线 |
| 环境变量依赖 | ✅ | `WA_WEB_AUTH_DIR` 已实现 |
| 文件系统约定 | ✅ | creds.json 路径已实现 |
| 协议/消息格式 | ⚠️ → ✅ | `formatWhatsAppEnvelope` + `NormalizePollInput` |
| 错误处理约定 | ✅ | 重连策略 `reconnect.go` + 配对超时 |

## 四、重构实现（Go）

### 文件结构（17 文件）

| 文件 | 职责 | 状态 |
|------|------|------|
| `normalize.go` | 号码规范化（+/00 前缀处理） | ✅ |
| `accounts.go` | 多账户解析 | ✅ |
| `outbound.go` | 出站消息发送 + Markdown 表格转换 | ✅ WA-E |
| `media.go` | 本地/远程加载 + MIME 检测 + 优化管线 | ✅ WA-D |
| `heartbeat.go` | 心跳监控 | ✅ |
| `inbound.go` | 入站类型 + 去重 | ✅ |
| `monitor_deps.go` | **DI 依赖接口**（9 项注入点） | ✅ WA-A |
| `monitor_inbound.go` | **完整入站管线**（去重→策略→配对→路由→分发） | ✅ WA-B |
| `auto_reply.go` | 自动回复集成 + 分块投递 | ✅ WA-C |
| `login.go` | 登录骨架 | ⏳ 运行时 |
| `login_qr.go` | QR 状态管理 | ⏳ 运行时 |
| `active_listener.go` | 活跃监听器接口 | ⏳ 运行时 |
| `auth_store.go` | 凭证持久化 + 结构化日志 | ✅ WA-F |
| `reconnect.go` | 重连策略 | ✅ |
| `status_issues.go` | 状态诊断 | ✅ |
| `vcard.go` | vCard 解析 | ✅ |
| `whatsapp_test.go` | 33 个单元测试 | ✅ |

### 关键设计决策

1. **DI 模式**：`WhatsAppMonitorDeps` 包含 9 个函数指针（`ResolveAgentRoute`、`DispatchInboundMessage`、`RecordInboundSession`、`UpsertPairingRequest`、`ReadAllowFromStore`、`ResolveStorePath`、`ReadSessionUpdatedAt`、`ResolveMedia`、`EnqueueSystemEvent`），与 iMessage/Signal 保持一致
2. **媒体优化**：`MediaOptimizer` 接口允许 DI 注入不同实现（CGo HEIC、pngquant CLI）
3. **文本分块**：`ChunkReplyText` 支持 `length` / `newline` 两种模式，默认上限 4000 字符

## 五、差异对照

| 维度 | 原版 TS | 重构 Go |
|------|---------|---------|
| WA 客户端 | Baileys (npm) | DI 接口（whatsmeow 可选） |
| 入站管线 | monitor.ts 452L | `monitor_inbound.go` 350L + DI |
| 媒体优化 | HEIC→JPEG + PNG 压缩 | `MediaOptimizer` DI + `ClampImageDimensions` |
| Markdown 表格 | `convertMarkdownTables` | `pkg/markdown/tables.go` 已接入 |
| 日志 | console.log | `log/slog` 结构化日志 |

## 六、Rust 下沉候选

| 函数/模块 | 优先级 | 原因 |
|-----------|--------|------|
| 媒体优化管线 | P2 | HEIC 解码 + 图像缩放 |

## 七、测试覆盖

| 测试类型 | 覆盖范围 | 状态 |
|----------|----------|------|
| 单元测试 | 33 用例（表格转换、文本分块、配对回复、信封格式、允许列表、图像尺寸、媒体优化、辅助函数） | ✅ -race PASS |
| 集成测试 | 需真实 WA 账户 | ❌ 待 Phase 10 |
