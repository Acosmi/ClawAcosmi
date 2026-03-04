# 媒体运营子智能体（oa-media）实施方案

> **性质**：只读研究产出，分段增量撰写
> **日期**：2026-03-01

---

## 1. 项目目标

构建 `oa-media` 子智能体，在主智能体指挥体系下完成：

1. **热点采集** — 通过 web_search 工具获取实时热点
2. **图文制作** — LLM 生成文案 + image 工具生成配图
3. **审批确认** — 主系统确认后才发布（非自主发布）
4. **多平台发布** — 微信公众号、小红书、自有网站

---

## 2. 与现有架构的关系

### 2.1 子智能体接入方式

参照 `spawn_coder_agent` 模式，新增 `spawn_media_agent`：

```
主智能体 → spawn_media_agent(task_brief, scope, constraints)
    ↓
DelegationContract 创建 → 权限单调衰减校验
    ↓
独立 LLM Session（oa-media）
    ↓
ThoughtResult 返回 → announce 回主智能体
```

**关键文件参照**：

- `runner/spawn_coder_agent.go` → 模式模板
- `runner/delegation_contract.go` → 合约类型复用
- `tools/registry.go` → 工具注册

### 2.2 oa-media 与 oa-coder 的区别

| 维度 | oa-coder | oa-media |
|------|----------|----------|
| 核心能力 | 代码编辑/bash | 内容生成/平台发布 |
| 需要的工具 | read/write/bash | web_search/image/media_publish |
| 网络权限 | 通常 no_network | **必须允许网络**（调用平台 API）|
| 沙箱 | 可选 | 不需要 OS 沙箱 |
| 交互频率 | 单次委托 | 可能需要多轮（采集→生成→确认→发布）|

### 2.3 现有可复用组件

| 组件 | 位置 | 用途 |
|------|------|------|
| `DelegationContract` | `runner/delegation_contract.go` | 合约骨架直接复用 |
| `sessions_spawn` | `tools/sessions.go` | 子智能体生命周期 |
| `web_search` | `tools/web_search_bocha.go` | 热点采集数据源 |
| `web_fetch` | `tools/web_fetch.go` | 平台 API 能力 |
| `image` | `tools/image_tool.go` | 配图生成 |
| `cron` | `tools/cron_tool.go` | 定时发布任务 |
| `Channel Plugin` | `channels/channels.go` | 渠道插件接口模式 |

---

## 3. oa-media 工具集设计

oa-media 子智能体需要的专属工具：

### 3.1 热点采集工具 `trending_topics`

```
action: "fetch" | "analyze" | "list_sources"
source: "weibo" | "baidu" | "zhihu" | "douyin" | "general"
category: "tech" | "finance" | "entertainment" | "all"
limit: number (默认 10)
```

**实现方式**：组合 `web_search` + `web_fetch` 抓取各平台热搜 API。

### 3.2 内容生成工具 `content_compose`

```
action: "draft" | "preview" | "revise"
platform: "wechat" | "xiaohongshu" | "website"
topic: string
style: "informative" | "casual" | "professional"
include_image: boolean
```

**实现方式**：复用 LLM 自身能力 + `image` 工具生成配图。

### 3.3 平台发布工具 `media_publish`

```
action: "publish" | "schedule" | "draft_save" | "status"
platform: "wechat" | "xiaohongshu" | "website"
content: { title, body, images[], tags[] }
schedule_time: string (ISO 8601, 可选)
```

### 3.4 互动管理工具 `social_interact`

```
action: "reply_comment" | "reply_dm" | "list_comments" | "list_dms"
platform: "xiaohongshu"
note_id: string
message: string
auto_mode: boolean
```

---

---

## 4. 平台适配器设计

### 4.1 微信公众号适配器

**API 基础**：需认证服务号，获取 `AppID` + `AppSecret`。

**核心接口**：

| 功能 | API 路径 | 说明 |
|------|---------|------|
| 获取 access_token | `GET /cgi-bin/token` | 2h 有效期，需缓存 |
| 上传图片 | `POST /cgi-bin/media/uploadimg` | jpg/png ≤1MB |
| 新建图文草稿 | `POST /cgi-bin/draft/add` | 支持多图文 |
| 发布草稿 | `POST /cgi-bin/freepublish/submit` | 异步，返回 publish_id |
| 查询发布状态 | `POST /cgi-bin/freepublish/get` | 轮询结果 |
| 模板消息 | `POST /cgi-bin/message/template/send` | 通知类推送 |

**实现要点**：

- 新建 `backend/internal/channels/wechat_mp/` 目录
- 实现 `WeChatMPClient` 封装 access_token 缓存刷新
- 发布流程：上传图片 → 创建草稿 → **主智能体确认** → 提交发布
- Token 刷新需要 mutex 保护 + 提前 5 分钟过期策略

**配置项**（`config.yaml`）：

```yaml
channels:
  wechat_mp:
    enabled: true
    app_id: "wx..."
    app_secret: "..."
    token_cache_path: "_system/wechat_mp/token.json"
```

---

### 4.2 小红书适配器

**API 策略**：小红书官方 API 主要面向电商开放，内容发布需结合以下两条路径：

**路径 A — 官方 API（受限）**：

| 功能 | 权限 | 说明 |
|------|------|------|
| 笔记发布 | `note.create` | 需申请，审核严格 |
| 图片上传 | `image.upload` | 配合笔记发布使用 |
| 笔记管理 | `note.get/list` | 查询已发布笔记 |
| 私信线索 | 聚光 API | 仅广告主可用 |
| 评论获取 | 非官方 | 需爬取或第三方 SDK |

**路径 B — RPA 自动化（补充方案）**：

参考开源项目 `MediaCrawler`、`XHS Automate`：

- 使用 Playwright/Puppeteer 模拟浏览器操作
- Cookie 登录 → 自动发布 → 自动回复评论/私信
- 风险：平台检测反爬，需控制频率

**推荐实现**：

- Phase 1 先实现路径 B（RPA），快速可用
- Phase 2 申请官方 API，逐步替换 RPA 部分
- 新建 `backend/internal/channels/xiaohongshu/` 目录
- 浏览器自动化层通过 `browser_tool` 已有能力驱动

**小红书自动互动**：

```
评论自动回复：
  cron 定时（如每 30 分钟）→ 拉取新评论 → LLM 生成回复 → 发布

私信自动回复：
  Webhook 接收私信 → 转发至 oa-media session → LLM 生成回复 → 发送
```

---

### 4.3 自有网站适配器

**方式**：通过 REST API 或 CMS 接口发布内容。

**实现要点**：

- 新建 `backend/internal/channels/website/` 目录
- 通用 REST 发布器：POST JSON `{ title, content, images, tags }`
- 支持接入 WordPress REST API / Ghost CMS / 自研 CMS
- 配置化目标 URL 和认证方式（API Key / Bearer Token）

**配置项**：

```yaml
channels:
  website:
    enabled: true
    api_url: "https://your-site.com/api/posts"
    auth_type: "bearer"  # bearer | api_key | basic
    auth_token: "..."
    image_upload_url: "https://your-site.com/api/media"
```

---

## 5. 核心工作流

### 5.1 完整发布流程（指挥体系）

**角色定义**：

- **用户** = 管理员 — 发起任务、审批确认
- **主智能体** = 站长 — 细化任务、委托调度、汇报结果
- **子智能体（oa-media）** = 干活的 — 执行具体操作，可向站长请求帮助

```
① 用户发起: "帮我发一篇关于AI的文章到公众号"
    ↓
② 主智能体（站长）优化任务细则:
   → 分析需求，拟定具体方案:
     "计划从百度/微博热搜采集AI领域热点TOP5，
      生成800字公众号图文+配图，风格偏科技资讯"
   → 将方案提交用户确认
    ↓
③ 用户确认方案（或调整后确认）
    ↓
④ 主智能体委托子智能体:
   spawn_media_agent({
       task_brief: "采集AI领域热点TOP5，选最热话题生成800字公众号图文",
       scope: [{ path: "_media/drafts/", permissions: [read, write] }],
       constraints: { no_spawn: true }
   })
    ↓
⑤ 子智能体（oa-media）执行:
   a. trending_topics(source: "general", category: "tech")
   b. LLM 从热点中选题 + 生成文案
   c. image(action: "generate", prompt: "...")
   d. 将草稿写入 _media/drafts/{id}.json
   ⚡ 若遇到问题（如需更多权限/素材），→ 向主智能体发起对话请求帮助
   e. 返回 ThoughtResult { status: "completed", result: "草稿已就绪..." }
    ↓
⑥ 主智能体收到结果:
   → 审核草稿质量，整理摘要
   → 提交给用户审批: "草稿已完成，以下是预览..."
    ↓
⑦ 用户最终审批:
   → 确认 → 主智能体指示子智能体执行发布
   → 拒绝/修改意见 → 主智能体重新委托子智能体修改
```

### 5.2 小红书自动互动流程

```
cron 定时任务（每 30 分钟）:
    ↓
spawn_media_agent({
    task_brief: "检查小红书新评论和私信，智能回复",
    scope: [{ path: "_media/xhs/", permissions: [read, write] }]
})
    ↓
oa-media 子智能体:
  1. social_interact(action: "list_comments", platform: "xiaohongshu")
  2. 对每条新评论 → LLM 生成回复
  3. social_interact(action: "reply_comment", ...) 
  4. social_interact(action: "list_dms") → 回复私信
  5. ThoughtResult 返回处理摘要
```

---

## 6. 新增文件结构

```
backend/internal/
├── agents/
│   ├── runner/
│   │   └── spawn_media_agent.go       [NEW] 媒体子智能体生成工具
│   └── tools/
│       ├── trending_tool.go           [NEW] 热点采集工具
│       ├── content_compose_tool.go    [NEW] 内容生成工具
│       ├── media_publish_tool.go      [NEW] 平台发布工具
│       └── social_interact_tool.go    [NEW] 社交互动工具
├── channels/
│   ├── wechat_mp/                     [NEW] 微信公众号适配器
│   │   ├── client.go                  API 客户端 + token 管理
│   │   ├── publish.go                 草稿/发布流程
│   │   ├── config.go                  配置结构
│   │   └── plugin.go                  Plugin 接口实现
│   ├── xiaohongshu/                   [NEW] 小红书适配器
│   │   ├── rpa_client.go              RPA 浏览器自动化
│   │   ├── api_client.go              官方 API（Phase 2）
│   │   ├── interactions.go            评论/私信管理
│   │   ├── config.go                  配置结构
│   │   └── plugin.go                  Plugin 接口实现
│   └── website/                       [NEW] 自有网站适配器
│       ├── rest_client.go             通用 REST 发布器
│       ├── config.go                  配置结构
│       └── plugin.go                  Plugin 接口实现
└── media/                             [NEW] 媒体运营公共模块
    ├── draft.go                       草稿存储/格式化
    ├── trending.go                    热点源聚合
    └── types.go                       公共类型定义
```

---

## 7. 实施路线图

### Phase 0（基础设施，~2 天）

- [ ] 新建 `spawn_media_agent.go`（复制 spawn_coder_agent 模式）
- [ ] 新建 `media/types.go`（草稿/内容/热点数据结构）
- [ ] `channels.go` 新增 `ChannelWeChatMP` / `ChannelXiaohongshu` 频道 ID
- [ ] `registry.go` 注册频道元数据

### Phase 1（热点采集 + 内容生成，~3 天）

- [ ] 实现 `trending_tool.go`（聚合微博/百度/知乎热搜）
- [ ] 实现 `content_compose_tool.go`（LLM 文案 + 调用 image 工具）
- [ ] 草稿存储 `media/draft.go`（JSON 文件到 `_media/drafts/`）

### Phase 2（微信公众号发布，~3 天）

- [ ] `wechat_mp/client.go` — token 管理 + HTTP 封装
- [ ] `wechat_mp/publish.go` — 草稿 → 发布流程
- [ ] `media_publish_tool.go` — wechat 分支
- [ ] 在 registry 注册 + boot.go 初始化

### Phase 3（小红书 RPA，~5 天）

- [ ] `xiaohongshu/rpa_client.go` — Playwright 自动化
- [ ] `xiaohongshu/interactions.go` — 评论/私信自动回复
- [ ] `social_interact_tool.go` — 互动工具
- [ ] cron 定时互动任务配置

### Phase 4（自有网站 + 打磨，~2 天）

- [ ] `website/rest_client.go` — 通用 REST 发布
- [ ] 端到端测试
- [ ] 系统提示词调优

---

## 8. 验证计划

### 自动化测试

- `spawn_media_agent` 合约创建 + 权限衰减单元测试
- `trending_tool` mock 搜索结果的解析测试
- `wechat_mp/client` token 缓存/刷新逻辑测试
- `media_publish_tool` 各平台分支的参数校验测试

### 集成测试

- 主智能体 → spawn_media_agent → 热点采集 → 草稿生成 → announce 回报
- 微信公众号沙盒环境发布测试
- 小红书 RPA 在测试账号上的发布 + 互动验证

### 手动验证

- 用户对话中触发完整发布流程
- 确认"用户确认后才发布"的门控生效
- 验证 cron 驱动的小红书自动互动

---

## 9. 开源参考

| 项目 | 用途 |
|------|------|
| [MediaCrawler](https://github.com/NanmiCoder/MediaCrawler) | 小红书爬虫/自动化参考 |
| [XHS Automate](https://github.com/xhs-automate) | AI 内容生成 + 小红书发布 |
| [LangChain Social Media Agent](https://github.com/langchain-social-media-agent) | HITL 社媒发布工作流参考 |
| [n8n](https://n8n.io) | 多平台内容分发工作流参考 |
| [xiaohongshu SDK](https://github.com/jellyfrank/xiaohongshu) | Python 小红书 API SDK |

---

> **重要提醒**：
>
> - 所有发布操作必须经过主智能体确认（`ThoughtResult` → 用户审批 → 发布）
> - 小红书 RPA 需控制操作频率（建议每次间隔 ≥ 5 秒），防止账号风控
> - 微信公众号 API 有速率限制，需在 client 层实现 rate limiter
