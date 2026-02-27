---
document_type: Audit
status: Completed
created: 2026-02-27
last_updated: 2026-02-27
scope: "多模态全链路复核审计：入站/出站规则 + 存储 + 格式处理 + GAP-2 实现"
verdict: PASS with 5 deferred items (2 MEDIUM, 3 LOW)
skill5_verified: true
---

# 复核审计报告：多模态全链路

## 审计目的

对多模态消息处理全链路进行 Skill 4 级别的复核审计，覆盖：
1. 各格式如何处理（入站/出站）
2. 入站规则（飞书 → 系统）
3. 出站规则（系统 → 飞书）
4. 存储问题（临时/持久/缓存）
5. GAP-2 实现的代码复核

---

## 第一部分：多模态格式处理矩阵

### 1.1 入站（飞书 → Agent）

| 消息类型 | 解析方法 | 处理流程 | 最终给 Agent 的格式 |
|---------|---------|---------|-------------------|
| `text` | `ExtractTextFromMessage` → JSON 解析 `{"text":"..."}` | 直接传递 | 纯文本 |
| `image` | `parseImageKey` → `image_key` | `DownloadImage` → `detectImageMediaType` → base64/描述 | `[图片: image/png, 123KB]` + 二进制数据 |
| `audio` | `parseAudioContent` → `file_key` + `duration` | `DownloadFile` → STT 转写（如配置）| 转写文字 或 `[语音: 15秒]` |
| `file` | `parseFileContent` → `file_key` + `fileName` + `fileSize` | `DownloadFile` → DocConv（如配置）| Markdown 或 `[文件: report.pdf, 2MB]` |
| `post` | `parsePostContent` → 遍历 `[[{tag,text}]]` | 提取纯文本（zh_cn 优先） | 纯文本 |
| `video` | — | 不处理 | `[视频附件, 暂不支持内容提取]` |

**入站处理链路**：
```
飞书 WebSocket → ExtractMultimodalMessage(handler.go)
  → DispatchMultimodalFunc(plugin.go:144)
  → ProcessFeishuMessage(server_multimodal.go:34)
      → client.DownloadImage/DownloadFile (resource.go)
      → STT 转写 (可选, server_methods_stt.go)
      → DocConv 转换 (可选, server_methods_chat.go:598)
  → 增强文本 → Agent 管线
```

### 1.2 出站（Agent → 飞书）

| 媒体类型 | 输入方式 | 处理流程 | 飞书消息类型 |
|---------|---------|---------|------------|
| 文本 | `send` RPC `message` 字段 | `SendTextMessage` | `text` |
| 图片 (URL) | `mediaUrl` / `mediaUrls` | SSRF 校验 → HTTP 下载 → `UploadImage` → `SendImageMessage` | `image` |
| 图片 (binary) | `mediaBase64` + `mediaMimeType` | **GAP-2 新增**：base64 解码 → `UploadImage` → `SendImageMessage` | `image` |
| 音频 (URL) | `mediaUrl` | SSRF 校验 → HTTP 下载 → `UploadFile(opus)` → `SendAudioMessage` | `audio` |
| 音频 (binary) | `mediaBase64` + `mediaMimeType=audio/*` | base64 解码 → `UploadFile` → `SendAudioMessage` | `audio` |
| 文件 (URL) | `mediaUrl` | SSRF 校验 → HTTP 下载 → `UploadFile` → `SendFileMessage` | `file` |
| 文件 (binary) | `mediaBase64` + `mediaMimeType` | base64 解码 → `UploadFile` → `SendFileMessage` | `file` |
| 视频 | `mediaUrl` | SSRF 校验 → HTTP 下载 → `UploadFile` → **SendFileMessage**（降级为文件） | `file` ⚠️ |
| 富文本 | `SendRichTextMessage` | 直接发送 post JSON | `post` |
| 卡片 | `SendCardMessage` | 直接发送 interactive JSON | `interactive` |

**出站处理链路**：
```
Agent send RPC (server_methods_send.go:58)
  ├── mediaBase64 路径（GAP-2 新增）:
  │   → base64.DecodeString → 大小校验(≤10MB)
  │   → ChannelMgr.SendMessage(OutboundSendParams{MediaData})
  │   → FeishuPlugin.SendMessage → sendMediaData
  │   → UploadImage/UploadFile → Send*Message
  │
  ├── mediaUrl 路径（原有）:
  │   → OutboundPipeline.Deliver (未 wired)
  │   → 或 ChannelSender.SendOutbound (未 wired)
  │   → 当前 fallback 到 stub 响应 ⚠️
  │
  └── 自动回复路径（inbound → reply）:
      → plugin.go:159 sender.SendText() 直接发送
```

---

## 第二部分：入站规则审计

### 2.1 安全防护

| 检查项 | 实现 | 状态 |
|--------|------|------|
| 消息去重 | `seenMessages` sync.Map, 5分钟 TTL | ✅ |
| 附件数量限制 | 最多 10 个 (server_multimodal.go:67) | ✅ |
| 下载大小限制 | 50 MB (resource.go:66) | ✅ |
| STT 输入大小 | 25 MB 解码后 (server_methods_stt.go:177) | ✅ |
| STT 超时 | 60 秒 (server_methods_stt.go:206) | ✅ |
| DocConv 超时 | 60 秒 (server_methods_chat.go:622) | ✅ |
| HTTP 响应体关闭 | 所有 `defer resp.Body.Close()` | ✅ |

### 2.2 发现的问题

**I-01 (MEDIUM): `resource.go:309` getTenantAccessToken 使用 `http.DefaultClient`**

```go
resp, err := http.DefaultClient.Do(req)  // ← 无超时！
```

- **影响**: slowloris 攻击可无限挂起 token 获取连接
- **对比**: `plugin.go:391` 的 `httpGetWithContext` 已修复为 30s 自定义 client
- **关联**: 与 M-08 同类问题，M-08 只修了 plugin.go，resource.go 遗漏
- **建议**: `resource.go` 中使用带超时的 client

**I-02 (MEDIUM): getTenantAccessToken 无缓存（M-11 遗留）**

- **位置**: `resource.go:289-330`
- **影响**: 每次 `DownloadResource`/`UploadImage`/`UploadFile` 都重新获取 token
- **场景**: 发送 1 张图片 = 1 次 token + 1 次上传 = 2 次 API 调用；高频场景触发飞书限流
- **注**: `remote_approval_feishu.go:112` 也有独立的 `getTenantAccessToken`，两份代码重复
- **建议**: 添加带过期检查的 token 缓存（飞书 token 有效期 2 小时）

**I-03 (LOW): 顺序处理附件，无并发**

- **位置**: `server_multimodal.go:75-159`
- **影响**: 10 个大音频文件 × 60s STT 超时 = 最长 10 分钟阻塞
- **建议**: 可考虑 `errgroup` 并发处理

---

## 第三部分：出站规则审计

### 3.1 安全防护

| 检查项 | URL 路径 | base64 路径 | 状态 |
|--------|---------|------------|------|
| SSRF 防护 | `validateMediaURL` 拦截私有/回环 IP | 不适用（无网络请求） | ✅ |
| 重定向限制 | 最多 5 次，每次重新校验 | 不适用 | ✅ |
| 大小限制 | 30 MB (readLimited) | 10 MB (handleSend) | ✅ |
| HTTP 超时 | 30 秒 | 不适用 | ✅ |
| Scheme 校验 | 仅 http/https | 不适用 | ✅ |
| 错误回退 | 媒体失败 → 降级为文字发送 | 同左 | ✅ |

### 3.2 发现的问题

**O-01 (LOW): OutboundPipeline 和 ChannelSender 均未 wired**

- **位置**: `server_methods.go:65,87` — 字段已声明但从未赋值
- **影响**: `send` RPC 中 mediaUrl 路径落入 stub 响应 `"outbound channel pipeline not connected"`
- **实际影响**: 仅 base64 路径（GAP-2 新增）通过 `ChannelMgr` 可工作；URL 路径当前不可用
- **建议**: 后续在 `StartGatewayServer` 中 wire OutboundPipeline 或直接统一用 ChannelMgr

**O-02 (LOW): 自动回复只发 SendText，不发媒体**

- **位置**: `plugin.go:156-171`
- **影响**: Agent 通过自动回复链路（inbound → reply）返回的文本中如含媒体 URL，不会自动上传发送
- **建议**: 低优先级，Agent 可通过 `send` RPC 主动发送媒体

---

## 第四部分：存储审计

### 4.1 飞书媒体存储模型

飞书的 `image_key` 和 `file_key` 是**飞书服务端管理的临时资源 ID**：

| 资源 | 来源 | 存储位置 | 生命周期 |
|------|------|---------|---------|
| `image_key` (入站) | 飞书消息解析 | 飞书服务端 | 消息存续期间 |
| `image_key` (出站) | `UploadImage` API 返回 | 飞书服务端 | 不确定（官方无明确说明） |
| `file_key` (入站/出站) | 消息解析 / `UploadFile` 返回 | 飞书服务端 | 同上 |
| 下载的二进制数据 | `DownloadResource` 结果 | **仅内存** | 函数返回即释放 |
| base64 解码数据 | `handleSend` 中 `DecodeString` | **仅内存** | RPC 处理完即释放 |

### 4.2 本系统存储特征

**关键发现：系统不持久化任何媒体文件。**

- ✅ 无临时文件写入：feishu 包中无 `tmp`/`TempDir`/`StorePath` 引用
- ✅ 无本地缓存：`image_key`/`file_key` 不缓存，每次即用即弃
- ✅ 内存控制：入站 50MB 上限，出站 URL 30MB / base64 10MB
- ✅ GC 安全：`[]byte` 由 Go GC 自动回收，无手动 `Close` 需求

### 4.3 存储相关风险

| 风险 | 评估 | 说明 |
|------|------|------|
| 内存峰值 | ⚠️ 可控 | base64 路径峰值 ~27MB（13.3MB base64 字符串 + 10MB 解码），可接受 |
| 磁盘占用 | ✅ 无 | 不写文件 |
| 飞书资源过期 | ⚠️ 低风险 | 上传的 image_key 有有效期，但即传即发，不缓存 |
| 并发内存 | ⚠️ 需关注 | N 个并发 send 各持 10MB = N×10MB 内存，建议未来加并发限制 |

---

## 第五部分：GAP-2 代码复核

### 5.1 `outbound.go` 变更

```go
MediaData     []byte    // ✅ zero value nil 不影响现有代码
MediaMimeType string    // ✅ zero value "" 不影响现有代码
```

**判定**: ✅ 向后兼容，无破坏性

### 5.2 `server_methods_send.go` 变更

| 检查项 | 结果 |
|--------|------|
| base64 解码错误处理 | ✅ 返回 BadRequest |
| 大小限制 (10MB) | ✅ 解码后校验 |
| ChannelMgr nil 检查 | ✅ `ctx.Context.ChannelMgr != nil` |
| channelID 守卫 | ✅ `channelID != "chat"` |
| context 传递 | ✅ `context.Background()` |
| 幂等去重 | ⚠️ 注意：base64 路径在幂等检查之后，受 `dedupeKey` 保护 |

**判定**: ✅ 安全

### 5.3 `plugin.go` 变更

| 检查项 | 结果 |
|--------|------|
| MediaData 优先于 MediaURL | ✅ 先检查 `len(params.MediaData) > 0` |
| client nil 检查 | ✅ `&& client != nil` |
| 降级逻辑 | ✅ 媒体发送失败 → fallthrough 到文字 |
| sendMediaData 与 sendMediaMessage 一致性 | ✅ 相同 switch 结构 |
| detectMediaCategory 复用 | ✅ 同一函数，传 mimeType 和 data |

**判定**: ✅ 正确

### 5.4 代码重复观察

`sendMediaData` 和 `sendMediaMessage` 的 switch 逻辑完全相同（image/audio/default），仅数据来源不同（binary vs HTTP response body）。可考虑合并为统一入口接受 `(data []byte, mimeType string)`。但当前分开更清晰，不做强制要求。

---

## 第六部分：判定

**PASS，含 5 项延迟项**

| 编号 | 类型 | 描述 | 优先级 | 关联 |
|------|------|------|--------|------|
| I-01 | 安全 | `resource.go` getTenantAccessToken 用 `http.DefaultClient` 无超时 | 🟡 MEDIUM | M-08 遗漏 |
| I-02 | 性能 | getTenantAccessToken 无缓存，高频触发限流 | 🟡 MEDIUM | M-11 遗留 |
| I-03 | 性能 | 附件顺序处理无并发 | 🟢 LOW | — |
| O-01 | 架构 | OutboundPipeline/ChannelSender 未 wired，URL 路径 stub | 🟢 LOW | — |
| O-02 | 功能 | 自动回复只发文字，不发媒体 | 🟢 LOW | — |

GAP-2 实现（mediaBase64 路径）审计 **PASS**，无安全/正确性问题。

---

## 参考来源

- `backend/internal/channels/feishu/plugin.go` — 出站发送 + sendMediaData
- `backend/internal/channels/feishu/resource.go` — 上传/下载 + token
- `backend/internal/channels/feishu/handler.go` — 入站消息解析
- `backend/internal/channels/feishu/client.go` — SDK 封装
- `backend/internal/channels/feishu/sender.go` — 发送便捷方法
- `backend/internal/channels/outbound.go` — OutboundSendParams 类型
- `backend/internal/gateway/server_methods_send.go` — send RPC 处理
- `backend/internal/gateway/server_multimodal.go` — 多模态预处理
- `backend/internal/gateway/server_methods_stt.go` — STT 配置/转写
- `backend/internal/gateway/server_methods_chat.go` — chat.send 附件处理
- `backend/internal/channels/channel_message.go` — ChannelMessage 类型
- `docs/claude/audit/audit-2026-02-27-gap2-mediabase64.md` — GAP-2 首次审计
- `docs/claude/audit/audit-2026-02-27-multimedia-feishu-outbound.md` — 出站能力审计
