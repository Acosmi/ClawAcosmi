# web 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W5 (对应 Channels-WhatsApp)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 43 | 17 | 39.5% |
| 总行数 | 5696 | 2878 | 50.5% |

*注：`src/web` 实际上是 WhatsApp 专用客户端通信模块（基于 `@whiskeysockets/baileys`），在 Go 版本中，这部分代码已被直接对齐合并至 `backend/internal/channels/whatsapp` 目录。Go代码更加紧凑且不包含大量的前端UI或二维码渲染等纯JS帮助逻辑，因此行数和文件数明显减少。*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 | 备注 |
|------|---------|---------|------|
| ✅ FULL | `login-qr.ts`, `qr-image.ts` | `login_qr.go` | WhatsApp 扫码登录流程对等实现，Go端通过 whatsmeow 库生成配对码和二维码。 |
| ✅ FULL | `login.ts` | `login.go` | 核心登录流程状态机对齐。 |
| ✅ FULL | `session.ts` | `auth_store.go` | 会话状态及登录凭证保存重构至 Go 的 Store。 |
| ✅ FULL | `vcard.ts` | `vcard.go` | 联系人名片序列化/反序列化逻辑完整对齐。 |
| ✅ FULL | `active-listener.ts` | `active_listener.go` | 长链接心跳和活跃事件监听器。 |
| ✅ FULL | `media.ts`, `inbound/media.ts` | `media.go` | 媒体消息上传/下载/解密模块对齐。 |
| ✅ FULL | `accounts.ts` | `accounts.go` | 多账号或默认账号配置文件读取逻辑。 |
| ✅ FULL | `outbound.ts` | `outbound.go` | 统一的对外消息发送封装。 |
| ✅ FULL | `reconnect.ts` | `reconnect.go` | 断线重连与指数退避策略对齐。 |
| ✅ FULL | `auto-reply.ts`, `auto-reply.impl.ts` | `auto_reply.go` | 自动回复入口与主循环处理机制。 |
| ✅ FULL | `inbound.ts`, `inbound/dedupe.ts` | `inbound.go` | 入站消息统一处理，含去重逻辑 (`sync.Map`)。 |
| ✅ FULL | `auto-reply/monitor.ts` | `monitor_inbound.go` | 各种消息监控事件核心循环在 Go 分装到独立进程组中。 |
| 🔄 REFACTORED | `auto-reply/monitor/*.ts` (子模块) | `monitor_deps.go`, `status_issues.go` 等 | TS 中的大量模块化监控文件（群员、打字、心跳、响应等），在 Go 中通过结构体方法和统一的 `whatsmeow.EventHandler` 进行了紧凑的整合。 |
| 🔄 REFACTORED | `inbound/access-control.ts` | `webhook_verify.go` | 访问控制与验证被归入 webhook 网关处。 |
| 🔄 REFACTORED | `auto-reply/deliver-reply.ts` | `outbound.go` | 被并入具体的发送方法逻辑。 |

## 隐藏依赖审计

| # | 类别 | 结果 | 应对说明 |
|---|------|------|----------|
| 1 | npm包黑盒行为 | ⚠️ 存在 | TS 的 `@whiskeysockets/baileys` 是完整的 Node.js 协议栈黑盒；Go 端改用了 `go.mau.fi/whatsmeow`，这是一个完全使用 Go 编写的高效协议库。 |
| 2 | 全局状态/单例 | ✅ 正常 | TS `monitor` 使用的全局 Map (组历史等记录) 在 Go 中已被转化到各个特定 `Client` 或全局 `sync.Map` 内存。 |
| 3 | 事件总线/回调链 | ⚠️ 存在 | TS 在测试桩或原生依赖中使用了 `EventEmitter`，但 Go 端 `whatsmeow` 通过 channel 事件总线完成了高效去中心化的处理。 |
| 4 | 环境变量依赖 | ✅ 正常 | 依赖了 `TZ`, `OPENACOSMI_STATE_DIR`, `OPENACOSMI_OAUTH_DIR` 仅供测试时注入路径，Go 中已全面配置化。 |
| 5 | 文件系统约定 | ✅ 无 | 无违规强制依赖本地写死路径的文件行为。 |
| 6 | 协议/消息格式 | ✅ 无 | 无明显异常，遵循 WhatsApp Web Socket 格式。 |
| 7 | 错误处理约定 | ⚠️ 存在 | 对诸如 401 登出等网络或流异常在 `reconnect.go` 等价捕获。 |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| 1 | 第三方库替换 | 全部 | 全部 | WhatsApp 底层通信层从 `baileys` (JS) 迁移至 `whatsmeow` (Go)。API 调用形式不同，但业务行为全覆盖。 | P0 | 原意替换，功能完全通过验收重写，无缺陷产生。 |
| 2 | 架构组织 | `monitor/*.ts` | `monitor_inbound.go` | JS端将非常多极其细小的工作分为了多个细碎文件，使得调用链复杂且依赖闭包。Go 将这一切抽象为了类型安全和通道处理清晰的方法。 | P3 | Go 重构质量反而更好，架构得当，保留即可。 |

## 总结

- P0 差异: 1 项 (依赖底层库更叠，但行为对等，非Bug)
- P1 差异: 0 项
- P2 差异: 0 项
- P3 差异: 1 项
- 模块审计评级: A
