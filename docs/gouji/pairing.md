# 渠道配对模块架构文档

> 最后更新：2026-02-26 | 代码级审计完成 | 3 源文件, 2 测试文件, 9 测试, ~631 行

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/internal/pairing/` |
| Go 源文件数 | 3 |
| Go 测试文件数 | 2 |
| 测试函数数 | 9 |
| 总行数 | ~631 |
| 对齐 TS | `src/pairing/pairing-store.ts` (498L) |
| 依赖 | `internal/channels`, `internal/config` |

渠道配对模块，管理消息平台（Discord/Telegram/Signal 等）的**用户配对请求和白名单**。未经配对的用户发送消息时，收到包含配对码的引导消息；Bot 所有者通过 CLI 验证码批准后，用户被自动加入白名单。

## 二、文件索引

| 文件 | 行数 | 职责 |
|------|------|------|
| `store.go` | 573 | **核心**：配对请求 CRUD、白名单管理、JSON 持久化、过期清理、码碰撞防御 |
| `messages.go` | 42 | 配对消息模板构建 |
| `labels.go` | ~16 | 渠道 ID 到可读标签的映射 |

## 三、存储架构

每个渠道维护**两个 JSON 文件**，存储在 `config.ResolveOAuthDir()` 下：

```
~/.openacosmi/oauth/
├── discord-pairing.json      // 待处理配对请求
├── discord-allowFrom.json    // 已批准白名单
├── telegram-pairing.json
├── telegram-allowFrom.json
└── ...
```

### 文件格式

```json
// {channel}-pairing.json
{
  "version": 1,
  "requests": [
    {
      "id": "user123",
      "code": "AB3KW7NP",
      "createdAt": "2026-02-26T14:00:00.000Z",
      "lastSeenAt": "2026-02-26T14:05:00.000Z",
      "meta": { "username": "alice" }
    }
  ]
}

// {channel}-allowFrom.json
{
  "version": 1,
  "allowFrom": ["user123", "user456"]
}
```

## 四、配对码生成

```go
const pairingCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"  // 无 0O1I，人类友好
const pairingCodeLength   = 8
```

- **8 位字符**，使用 `crypto/rand` 密码学安全随机
- 去除易混淆字符 (0/O, 1/I) — 适合语音/截图传播
- `generateUniqueCode()` 确保与已有码不碰撞（最多 500 次重试）

## 五、核心操作

### UpsertChannelPairingRequest — 创建/更新配对请求

1. 加载 pairing JSON
2. 清理过期请求（TTL = 1 小时）
3. 如已存在 → 更新 `lastSeenAt` + meta，保留原码
4. 如不存在 → 生成唯一码、创建新请求
5. 容量限制：最多 3 个 pending 请求（保留最近访问的）
6. 原子写入 JSON

### ApproveChannelPairingCode — 通过验证码批准

1. 查找匹配码（大小写不敏感）
2. 从 pairing 列表移除
3. **自动** `AddChannelAllowFromStoreEntry()` 加入白名单
4. 返回被批准用户的 ID 和请求信息

### 白名单 CRUD

- `ReadChannelAllowFromStore()` — 读取（带规范化）
- `AddChannelAllowFromStoreEntry()` — 添加（带去重）
- `RemoveChannelAllowFromStoreEntry()` — 移除

## 六、并发安全

```go
// 按渠道分锁避免不同渠道互相阻塞
var storeLocks = make(map[string]*sync.Mutex)
```

- **全局锁注册表** + **按渠道分锁**（替代 TS 的 `proper-lockfile`）
- 同一渠道的 pairing 和 allow 各自独立加锁：`channel + ":pairing"`, `channel + ":allow"`
- **原子写入**：先写 `.tmp` 再 `os.Rename()`，防止崩溃导致数据损坏

## 七、安全措施

### 文件名安全化 (`safeChannelKey`)

```go
// 防路径遍历：去除 \ / : * ? " < > | 和 ..
re := regexp.MustCompile(`[\\/:*?"<>|]`)
safe := re.ReplaceAllString(raw, "_")
safe = strings.ReplaceAll(safe, "..", "_")
```

### 规范化 (`normalizeAllowEntry`)

- 调用渠道适配器的 `NormalizeAllowEntry()` 做渠道特定规范化
- 拒绝通配符 `*`

## 八、消息模板 (messages.go)

```go
const PairingApprovedMessage = "✅ OpenAcosmi access approved. Send a message to start chatting."
```

`BuildPairingReply(channel, idLine, code)` 构建引导文本，自适应 `OPENACOSMI_PROFILE` 环境变量：

```
OpenAcosmi: access not configured.

Your Discord ID: alice#1234

Pairing code: AB3KW7NP

Ask the bot owner to approve with:
openacosmi --profile myprofile pairing approve discord <code>
```

## 九、测试覆盖

| 测试文件 | 测试数 | 覆盖范围 |
|----------|--------|----------|
| `store_test.go` | ~6 | 配对 CRUD、过期清理、容量限制 |
| `messages_test.go` | ~3 | 消息格式化、profile 自适应 |
| **合计** | **9** | |
