# 中国频道 SDK 集成跟踪

> 最后更新：2026-02-23

## SDK 版本信息

| 平台 | Go SDK 包 | 最新版本 | 官方仓库 |
|------|----------|---------|---------|
| 飞书/Lark | `github.com/larksuite/oapi-sdk-go/v3` | **v3.5.3** | [GitHub](https://github.com/larksuite/oapi-sdk-go) |
| 钉钉 | `github.com/open-dingtalk/dingtalk-stream-sdk-go` | **v0.9.1** | [GitHub](https://github.com/open-dingtalk/dingtalk-stream-sdk-go) |
| 企业微信 | `github.com/ArtisanCloud/PowerWeChat/v3` | **v3.4.38** | [GitHub](https://github.com/ArtisanCloud/PowerWeChat) |

---

## 当前进度

### Phase 1: UI 层 ✅ 已完成

- [x] 频道卡片 — `channels.wecom.ts`, `channels.dingtalk.ts`, `channels.feishu.ts`
- [x] 前端注册 — `channels.ts` 导入 + 渲染 + 默认排序
- [x] 后端注册 — `server_methods_channels.go` channelOrder/Labels/normalize
- [x] i18n — `en.ts` + `zh.ts` 频道名称、描述、专用字段

### Phase 2: 类型定义 ✅ 已完成

- [x] `pkg/types/types_feishu.go` — 添加 `FeishuConfig` + `FeishuAccountConfig`
  - `AppID`, `AppSecret`, `VerifyToken`, `EncryptKey`, `Domain`
- [x] `pkg/types/types_dingtalk.go` — 添加 `DingTalkConfig` + `DingTalkAccountConfig`
  - `AppKey`, `AppSecret`, `RobotCode`, `Token`, `AESKey`
- [x] `pkg/types/types_wecom.go` — 添加 `WeComConfig` + `WeComAccountConfig`
  - `CorpID`, `Secret`, `AgentID`(*int), `Token`, `AESKey`
- [x] `pkg/types/types_channels.go` — `ChannelsConfig` 添加 `Feishu`/`DingTalk`/`WeCom` 字段 + `channelsConfigKnownKeys`
- [x] `pkg/types/types.go` — `ChannelType` 常量添加 `ChannelFeishu`/`ChannelDingTalk`/`ChannelWeCom`
- [x] `internal/channels/channels.go` — `ChannelID` 常量添加 3 个中国频道
- [x] `internal/gateway/server_methods_channels.go` — `handleChannelsStatus` 添加 3 频道配置检测
- [x] `pkg/types/types_base_test.go` — `TestChannelTypeValues` 覆盖新常量

### Phase 3: SDK 安装 ✅ 已完成

- [x] `go get github.com/larksuite/oapi-sdk-go/v3@v3.5.3`
- [x] `go get github.com/open-dingtalk/dingtalk-stream-sdk-go@v0.9.1`
- [x] `go get github.com/ArtisanCloud/PowerWeChat/v3@v3.4.38`
- [x] `go mod tidy` — 依赖整理完成

### Phase 4: Channel Plugin 实现 ✅ 已完成（审计通过 2026-02-23）

#### 飞书 (`internal/channels/feishu/`) — oapi-sdk-go/v3 SDK

- [x] `config.go` — 配置解析 + 校验 + Feishu/Lark 域名切换
- [x] `client.go` — SDK 客户端封装 + 消息发送（text/post/interactive）
- [x] `sender.go` — 消息发送便捷封装
- [x] `handler.go` — 消息事件类型 + 文本提取
- [x] `webhook.go` — SDK EventDispatcher（自动验签/解密/URL challenge/去重）
- [x] `plugin.go` — Plugin 接口实现（ID/Start/Stop）

#### 钉钉 (`internal/channels/dingtalk/`) — dingtalk-stream-sdk-go

- [x] `config.go` — 配置解析 + 校验
- [x] `client.go` — Stream SDK 长连接 + recover 防护（防 SDK 内部 panic）
- [x] `handler.go` — SDK 回调数据解析
- [x] `sender.go` — HTTP API 消息发送（O2O + 群聊 + token 缓存）
- [x] `plugin.go` — Plugin 接口实现

#### 企业微信 (`internal/channels/wecom/`) — 标准库 crypto/aes

- [x] `config.go` — 配置解析 + 校验
- [x] `client.go` — HTTP API 客户端 + access_token 自动缓存
- [x] `handler.go` — AES-256-CBC 解密 + SHA1 签名验证 + XML 解析
- [x] `sender.go` — 消息发送（text/markdown/textcard）
- [x] `plugin.go` — Plugin 接口实现

#### Registry 更新

- [x] `registry.go` — chatChannelOrder + channelAliases + chatChannelMeta 添加 3 频道

### Phase 5: Gateway 集成 ✅ 已完成 (2026-02-23)

- [x] `boot.go` — GatewayState 添加 ChannelManager + accessor
- [x] `server.go` — 启动时从 config 读取频道配置并初始化 plugin
- [x] `server_methods_channels.go` — channels.status 合并运行时快照 + channels.logout 真实 Stop
- [x] `server_channel_webhooks.go` — 飞书 webhook + 企业微信回调 HTTP 路由
- [x] `ws_server.go` + `server_methods.go` — ChannelManager DI 注入
- [x] 消息路由 — outbound 配置 + MessageSender + bridge actions（Phase 7 完成）

### Phase 6: Schema + UI 表单 ✅ 已完成 (2026-02-23)

- [x] `config/schema.go` — knownChannelIDs 添加 feishu/dingtalk/wecom
- [x] `config/schema_hints_data.go` — 中英双语字段标签 + 帮助文本
- [x] UI 频道卡片 — 确认已使用 `t()` 国际化，zh/en locale 已有翻译
- [x] 向导 — 频道配置向导步骤添加中国平台（Phase 7 完成）

> 复核审计报告：[phase5-6-audit-report.md](phase5-6-audit-report.md)

### Phase 7: 消息路由管线 + 向导 ✅ 已完成 (2026-02-23)

- [x] `outbound.go` — feishu/dingtalk/wecom 出站配置
- [x] `channels.go` — `MessageSender` 可选接口 + `Manager.SendMessage`
- [x] `feishu/dingtalk/wecom plugin.go` — 实现 SendMessage
- [x] `bridge/feishu_actions.go` — FeishuActionDeps + HandleFeishuAction
- [x] `bridge/dingtalk_actions.go` — DingTalkActionDeps + HandleDingTalkAction
- [x] `bridge/wecom_actions.go` — WeComActionDeps + HandleWeComAction
- [x] `wizard-channel.ts` — 频道配置向导 (Apple 风格)
- [x] `en.ts` + `zh.ts` — wizard.channel.* i18n (22 keys × 2)

### Phase 8: 主动消息推送 + HMAC 验签 ✅ 已完成 (2026-02-23)

- [x] `permission_escalation.go` — 30 分钟审批超时自动拒绝 + 结果推送
- [x] `remote_approval.go` — `ApprovalResultNotification` + `ResultNotifier` + `NotifyResult`
- [x] `remote_approval_feishu.go` — 审批结果互动卡片（绿/红）
- [x] `remote_approval_dingtalk.go` — 审批结果 ActionCard
- [x] `remote_approval_wecom.go` — 审批结果 TextCard
- [x] `remote_approval_callback_verify.go` [NEW] — 钉钉 HMAC-SHA256 + 企微 SHA1+AES-256-CBC
- [x] Config 扩展 — 钉钉 `apiSecret` + 企微 `token`/`encodingAESKey`

## 配置示例

```json5
{
  "channels": {
    "feishu": {
      "appId": "cli_xxxxx",
      "appSecret": "xxxxx",
      "verifyToken": "xxxxx",
      "encryptKey": "xxxxx",
      "domain": "feishu"  // "feishu" 或 "lark"
    },
    "dingtalk": {
      "appKey": "dingxxxxx",
      "appSecret": "xxxxx",
      "robotCode": "xxxxx"
    },
    "wecom": {
      "corpId": "wwxxxxx",
      "secret": "xxxxx",
      "agentId": 1000002,
      "token": "xxxxx",
      "aesKey": "xxxxx"
    }
  }
}
```

## 技术要点

### 连接模式对比

| 平台 | 长连接（推荐） | HTTP 回调 | 需公网 IP |
| ---- | :---: | :---: | :---: |
| 飞书 | ✅ WebSocket (`oapi-sdk-go/v3/ws`) | ✅ | 长连接不需要 |
| 钉钉 | ✅ Stream (`dingtalk-stream-sdk-go`) | ✅ | 长连接不需要 |
| 企业微信 | ❌ 不支持 | ✅ | 需要 |

### 飞书

- **长连接模式**（✅ 推荐）：通过 `oapi-sdk-go/v3/ws` 包建立 WebSocket 全双工通道
  - 无需公网 IP、域名或内网穿透
  - 内置通信加密和鉴权，无需手动解密/验签
  - 每应用最多 **50 个并发连接**
  - 事件需在 **3 秒**内处理完毕，否则触发重试
  - 仅支持企业自建应用，不支持应用商店应用
  - 集群部署时只有一个客户端收到消息（不支持广播）
- **HTTP 回调模式**：传统 Webhook → 验证签名 → 解密 → 分发（需公网 IP）
- **消息发送**：REST API `POST /open-apis/im/v1/messages`
- **卡片消息**：支持 Interactive Card（推荐用于审批场景）
- **国际版**：域名切换 `open.larksuite.com` vs `open.feishu.cn`

### 钉钉

- **Stream 模式**（✅ 推荐）：长连接，无需公网 IP
  - 通过 `dingtalk-stream-sdk-go` 建立持久连接
  - 支持事件订阅 + 机器人消息接收 + 卡片回调
- **Outgoing 回调**：HTTP POST，需公网可达
- **消息发送**：`POST /v1.0/robot/oToMessages/batchSend`
- **群聊**：通过 conversationId 或 OpenConversationId

### 企业微信

- **回调模式**：HTTP 回调 → AES 解密 → XML 解析（需公网可达）
  - 企业微信暂不支持类似飞书/钉钉的长连接模式
- **消息发送**：`POST /cgi-bin/message/send` (应用消息)
- **Access Token**：需定时刷新（7200s 有效期）
- **群聊机器人**：Webhook URL 直接 POST JSON
