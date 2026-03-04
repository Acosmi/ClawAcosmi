# Phase 5D.4 — WhatsApp SDK 隐藏依赖深度审计

> **审计时间**：2026-02-13  
> **审计范围**：14 Go 文件 + 跨模块（类型/上游消费者/未移植 TS 文件）  
> **审计方法**：`/refactor` 步骤 3 — 7 类隐藏依赖逐项检查  
> **Go 目录**：`backend/internal/channels/whatsapp/`  
> **TS 目录**：`src/web/`、`src/whatsapp/`、`src/channels/plugins/`

---

## 发现汇总

| 编号 | 发现 | 类别 | 严重度 | 状态 |
|------|------|------|--------|------|
| H1 | `types_whatsapp.go` 缺少 `Actions` 字段 | 全局状态 | 🔴 高 | ✅ **已修复** |
| H2 | `outbound.go` 缺少 `convertMarkdownTables()` | 协议/消息格式 | 🔴 高 | ⏳ Phase 6 (WA-E) |
| H3 | `media.go` 缺少图片优化管线 (HEIC→JPEG/PNG) | npm 包黑盒 | 🔴 高 | ⏳ Phase 6 (WA-D) |
| H4 | `auth_store.go` 缺少 `WA_WEB_AUTH_DIR` 常量 | 全局状态 | 🟡 中 | ✅ **已修复** |
| H5 | `auth_store.go` 缺少 `logWebSelfId()` | 日志 | 🟡 中 | ✅ **已修复** |
| H6 | `auth_store.go` 缺少 `pickWebChannel()` | 路由 | 🟡 中 | ✅ **已修复** |
| H7 | `outbound.go` + `auth_store.go` 缺少结构化日志 (correlationId) | 可观测 | 🟡 中 | ⚠️ **部分修复** |
| H8 | 16 个 TS 文件未逐一列入审计 | — | 🟢 信息 | ✅ **已补充记录** |

---

## 一、7 类隐藏依赖逐项检查

### 1. npm 包黑盒行为

| Go 文件 | 结果 | 说明 |
|---------|------|------|
| `login.go` | ❌ Phase 6 桩 | Baileys `makeWASocket()` WebSocket 握手 + Signal 协议 + QR 生成 |
| `login_qr.go` | ❌ Phase 6 桩 | Baileys QR 事件 + `qrcode-terminal` 渲染 |
| `auto_reply.go` | ❌ Phase 6 桩 | Baileys `messages.upsert` 事件监听 |
| `outbound.go` | ⚠️ | listener 返回值语义需 Phase 6 确认 |
| `media.go` | ⚠️ **H3** | `sharp` (libvips) 优化管线未移植 → WA-D |
| 其余 10 文件 | ✅ | 无 npm 包依赖 |

### 2. 全局状态/单例

| Go 文件 | 结果 | 说明 |
|---------|------|------|
| `active_listener.go` | ✅ | `sync.RWMutex` 保护的 `listeners` map |
| `login_qr.go` | ✅ | `sync.Mutex` 保护的 `activeLogins` map |
| `inbound.go` | ✅ | `sync.Mutex` 保护的去重缓存 |
| `auth_store.go` | ✅ **H4 已修复** | 补入 `var WAWebAuthDir = ResolveDefaultWebAuthDir()` |

### 3. 事件总线/回调链

| 结果 | 说明 |
|------|------|
| ❌ Phase 6 | `monitorWebChannel()`、`monitorWebInbox()` 事件订阅链未移植 |
| ✅ | `outbound.go` 通过接口直接调用 |

### 4. 环境变量依赖

| 结果 | 说明 |
|------|------|
| ✅ | 生产代码无 `process.env`。`OPENACOSMI_OAUTH_DIR` 通过 `config.ResolveOAuthDir()` 间接读取，Go 已有等价 |

### 5. 文件系统约定

| 结果 | 说明 |
|------|------|
| ✅ | 目录结构 `~/.openacosmi/oauth/whatsapp/{accountId}/` 完整等价 |
| ⚠️ **H7 部分修复** | `LogoutWeb()` + `MaybeRestoreCredsFromBackup()` 日志已补充；correlationId 需 Phase 6 日志基础设施 |

### 6. 协议/消息格式约定 🔴

| Go 文件 | 结果 | 说明 |
|---------|------|------|
| `outbound.go` | 🔴 **H2** | TS `sendMessageWhatsApp()` 调用 `convertMarkdownTables(text, tableMode)` → Go 完全缺失 |
| `normalize.go` | ✅ | JID/LID/Group 格式完整等价 |
| `vcard.go` | ✅ | vCard 解析完整等价 |

### 7. 错误处理约定

| 结果 | 说明 |
|------|------|
| ✅ | `DisconnectReason` 错误码 (401/440/408/515) 完整等价 |
| ⚠️ **H7** | `outbound.go` 缺 correlationId 结构化日志 |

---

## 二、跨模块扩展审计

### A. 类型定义缺失字段

| 字段 | TS 类型 | Go 现状 | 状态 |
|------|---------|---------|------|
| `actions` | `WhatsAppActionConfig` | 类型已定义但未嵌入 struct | ✅ **H1 已修复** — 补入 `Actions *WhatsAppActionConfig` |

### B. 上游消费者

仅 2 个 Go 文件导入 `channels/whatsapp`：

- `channels/normalize.go` — 委托 `NormalizeWhatsAppTarget()` ✅
- `channels/registry.go` — 频道注册常量 ✅

### C. 共享依赖

| 共享包 | 状态 |
|--------|------|
| `pkg/utils.NormalizeE164()` | ✅ 有实现 + 测试 |
| `internal/config.ResolveOAuthDir()` | ✅ |
| `pkg/types.WhatsAppConfig` | ✅ H1 修复后完整 |

### D. 未移植 TS 文件（Phase 6 整体延迟）

| 文件/目录 | 行数 | 职责 | deferred-items 归属 |
|-----------|------|------|-------------------|
| `session.ts` | 317 | Baileys socket 创建 + creds 保存 + Boom 错误解析 | WA-A 核心 |
| `qr-image.ts` | 132 | QR 矩阵 → PNG → Base64 DataURL | WA-A 子项 |
| `auto-reply/` | 11 文件 | monitor + 心跳 + deliver-reply + mentions + session-snapshot | WA-C |
| `inbound/` | 8 文件 | extract + access-control + monitor + media + send-api | WA-B |

> 这些文件全部依赖 Baileys 运行时，与 Phase 6 延迟策略一致。

---

## 三、已确认正确（无遗漏）

| Go 文件 | 与 TS 对比结果 |
|---------|---------------|
| `normalize.go` | ✅ JID/LID/Group 解析 + E164 完整等价 |
| `accounts.go` | ✅ 12 函数完整等价，配置级联合并 1:1 |
| `active_listener.go` | ✅ 接口 + 并发安全注册表完整等价 |
| `reconnect.go` | ✅ 策略合并 + 范围钳位完整等价 |
| `vcard.go` | ✅ FN/N/TEL 解析完整等价 |
| `inbound.go` | ✅ 类型 + 去重 + 文本辅助完整等价 |
| `status_issues.go` | ✅ 健康检查逻辑完整等价 |
| `heartbeat.go` | ✅ 收件人解析 + 去重完整等价 |

---

## 四、修复日志

| 时间 | 修复项 | 文件 | 说明 |
|------|--------|------|------|
| 2026-02-13 | H1 | `pkg/types/types_whatsapp.go` | 补入 `Actions *WhatsAppActionConfig` 字段 |
| 2026-02-13 | H4 | `auth_store.go` | 补入 `var WAWebAuthDir = ResolveDefaultWebAuthDir()` |
| 2026-02-13 | H5 | `auth_store.go` | 新增 `LogWebSelfId()` 函数 |
| 2026-02-13 | H6 | `auth_store.go` | 新增 `PickWebChannel()` 函数 |
| 2026-02-13 | H7 | `auth_store.go` | `MaybeRestoreCredsFromBackup` + `LogoutWeb` 补充 warn/info 日志 |
| 2026-02-13 | — | `docs/renwu/phase5d-task.md` | WhatsApp 桩表更正为 7 项 |
| 2026-02-13 | — | `docs/renwu/deferred-items.md` | 新增 WA-E、WA-F |

---

## 五、与 deferred-items.md 交叉引用

| deferred 编号 | 对应审计发现 | 状态 |
|--------------|-------------|------|
| WA-A | login.go + login_qr.go 桩 | ⏳ Phase 6 |
| WA-B | auto_reply.go 桩 | ⏳ Phase 6 |
| WA-C | ActiveWebListener 接口无实现 | ⏳ Phase 6 |
| WA-D | media.go 优化管线 (H3) | ⏳ Phase 6 |
| WA-E | outbound.go Markdown 表格转换 (H2) | ⏳ Phase 6 |
| WA-F | auth_store 辅助 + 日志 (H4-H7) | ✅ H4/H5/H6 已修复，H7 部分修复（correlationId 延迟 Phase 6） |
