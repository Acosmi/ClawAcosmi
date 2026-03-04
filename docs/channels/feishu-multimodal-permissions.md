# 飞书多模态权限配置指南

> 本文档说明 OpenAcosmi 飞书频道多模态功能（图片/语音/文件下载）所需的权限配置步骤。

## 所需权限总览

| 权限 Scope | 说明 | 用途 |
|------------|------|------|
| `im:message` | 获取与发送单聊、群组消息 | 消息收发（基础功能） |
| `im:message:readonly` | 读取单聊、群组消息 | 读取消息内容获取 file_key |
| `im:resource` | 读取消息中的资源文件 | **多模态核心：下载图片/语音/文件** |

## 配置步骤

### 1. 进入飞书开放平台

- 国内版：[open.feishu.cn](https://open.feishu.cn)
- 国际版：[open.larksuite.com](https://open.larksuite.com)

### 2. 进入应用权限管理

1. 登录开发者后台
2. 选择你的机器人应用
3. 左侧导航：**权限管理** → **权限配置**

### 3. 添加权限

搜索并开通以下权限：

```
im:resource
im:message
im:message:readonly
```

或使用 JSON 批量导入：

```json
{
  "scopes": [
    "im:resource",
    "im:message",
    "im:message:readonly"
  ]
}
```

### 4. 创建版本并发布

1. **应用版本管理** → **创建版本**
2. 填写版本说明（如："添加多模态消息权限"）
3. 提交发布
4. 等待管理员审批（企业自建应用通常即时通过）

## API 接口说明

### 获取消息中的资源文件

```
GET /open-apis/im/v1/messages/{message_id}/resources/{file_key}?type={image|file}
```

| 参数 | 说明 |
|------|------|
| `message_id` | 消息 ID（从接收消息事件中获取） |
| `file_key` | 资源 key（从消息内容 JSON 解析） |
| `type` | `image`（图片）或 `file`（文件/音频/视频） |

### 认证方式

使用 `tenant_access_token`（租户级别令牌）：

```
Authorization: Bearer {tenant_access_token}
```

令牌通过 App ID + App Secret 获取：

```
POST /open-apis/auth/v3/tenant_access_token/internal
```

## 使用限制

| 限制项 | 说明 |
|--------|------|
| 文件大小 | ≤ 100MB |
| 表情包 | ❌ 不支持下载 |
| 合并转发消息 | ❌ 不支持获取子消息资源 |
| 卡片消息 | ❌ 不支持获取 |
| 受限消息 | ❌ 无法下载 |
| 格式转换 | ❌ API 不支持，需本地处理 |

## 前置条件

- [x] 应用已启用**机器人能力**
- [x] 机器人与目标消息在**同一会话**中
- [x] 已开通 `im:resource` 权限
- [x] 已发布包含新权限的版本

## file_key 获取方式

多模态消息中 `file_key` 的获取路径：

| 消息类型 | 内容字段 | key 字段 |
|----------|----------|----------|
| `image` | `content.image_key` | image_key |
| `audio` | `content.file_key` | file_key |
| `file` | `content.file_key` | file_key |
| `post`（富文本中图片） | `content.post.*.image_key` | image_key |

> 对应代码实现：`feishu/handler.go` 中的 `ExtractMultimodalMessage` 函数。

## 代码对应

| 代码文件 | 功能 |
|----------|------|
| `feishu/resource.go` | `DownloadResource()` — 调用资源下载 API |
| `feishu/handler.go` | `ExtractMultimodalMessage()` — 解析并提取 file_key |
| `feishu/client.go` | `FeishuClient` — SDK 客户端（含 AppID/AppSecret） |
| `gateway/server_multimodal.go` | `ProcessFeishuMessage()` — 多模态预处理 |

## 常见错误

| 错误码 | 原因 | 解决 |
|--------|------|------|
| `99991400` | 缺少 `im:resource` 权限 | 开通权限并发布新版本 |
| `99991401` | tenant_access_token 无效 | 检查 App ID / App Secret |
| `234043` | 合并转发/卡片消息不支持 | 过滤此类消息类型 |
| `230001` | message_id 与 file_key 不匹配 | 确保从同一消息中获取 |

## 钉钉/企微对比

| 平台 | 多模态能力 | 当前状态 |
|------|-----------|---------|
| 飞书 | 完整：图片/语音/文件/富文本 | ✅ Phase B 已实现 |
| 钉钉 | Stream SDK 有限制，需 HTTP API 补充 | 📋 Phase B+ 待实现 |
| 企微 | XML 回调需特殊解析 | 📋 Phase B+ 待实现 |
