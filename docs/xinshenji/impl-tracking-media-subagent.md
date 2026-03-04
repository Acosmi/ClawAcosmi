# oa-media 子智能体 — 实施跟踪计划

> [!IMPORTANT]
> **文档目录约定**：本项目（oa-media 子智能体）产生的所有文档统一存放于 `docs/meiti/` 目录下，包括但不限于：
>
> - 审计报告（`audit-*.md`）
> - 任务跟踪（`task-*.md`）
> - 延迟待办（`deferred-items.md`）
> - Bootstrap 上下文（`bootstrap-*.md`）
> - 架构文档（`*.md`）
>
> **GitHub 仓库**：<https://github.com/Acosmi/Chat-Acosmi>

> [!CAUTION]
> **安全第一原则**：确保系统稳定，本次新增的子智能体在独立文件夹下完成开发和测试，全部验证通过后再接入主系统。禁止在主系统代码中直接开发，防止破坏现有代码逻辑结构导致系统崩溃。

> **基于**: `design-media-subagent-2026-03-01.md`
> **创建日期**: 2026-03-01
> **总工期估算**: ~15 工作日（Phase 0–4）

---

## 一、项目概述

构建 `oa-media` 媒体运营子智能体，复用现有 `spawn_coder_agent` + `DelegationContract` 委托模式，实现热点采集 → 内容生成 → 审批确认 → 多平台发布的完整链路。

**核心原则**：

- 所有发布必须经主智能体确认（HITL 人类在环）
- 权限单调衰减（子智能体不能获得比主智能体更多的权限）
- 工具注册与现有 `ToolRegistry` 模式保持一致

---

## 二、现有代码基准（已审计）

| 复用组件 | 文件路径 | 复用方式 |
|---------|---------|---------|
| 子智能体生成 | `runner/spawn_coder_agent.go` (244L) | 复制模板改造 |
| 委托合约 | `runner/delegation_contract.go` (515L) | 直接复用类型 |
| 工具注册表 | `tools/registry.go` (248L) | 新增 register 函数 |
| 频道插件接口 | `channels/channels.go` (227L) | 实现 `Plugin` 接口 |
| Web 搜索 | `tools/web_search_bocha.go` | 组合调用 |
| Web 抓取 | `tools/web_fetch.go` | 组合调用 |
| 图片生成 | `tools/image_tool.go` | 组合调用 |
| 定时任务 | `tools/cron_tool.go` | 互动定时驱动 |

**现有频道插件目录**（参照模式）: `discord/`, `telegram/`, `slack/`, `feishu/`, `wecom/` 等

---

## 三、子智能体运行时架构

> 参照 `spawn_coder_agent.go`，子智能体在被 spawn 时会获得独立的运行时配置：

**1. 专属系统提示词**（在 `spawn_media_agent.go` 中构建）

- 角色定义："你是 oa-media 媒体运营助手，负责热点采集、内容创作和平台发布"
- 能力边界：明确列出可用工具及其用途
- 行为准则：所有发布必须生成草稿等待审批，不可自主发布
- 平台规范：各平台内容格式/字数/图片限制

**2. 受限工具集**（权限单调衰减，不继承主智能体全部工具）

- 专属工具：`trending_topics`, `content_compose`, `media_publish`, `social_interact`
- 共享工具：`web_search`, `web_fetch`, `image`（从主智能体工具集中选取）
- **不包含**：`bash`, `read/write`（文件系统）, `gateway`, `memory` 等主智能体工具

**3. 不需要独立技能系统**

- Skills 是主智能体层面概念（`search_skills` + SKILL.md）
- 子智能体的全部能力通过工具定义 + 系统提示词约束
- 主智能体负责"什么时候派子智能体做什么事"的决策

---

## 四、分阶段实施任务清单

### Phase 0：基础设施搭建（~2 天）

#### P0-1: 新建 `spawn_media_agent.go`

- **文件**: `backend/internal/agents/runner/spawn_media_agent.go` [NEW]
- **参照**: `spawn_coder_agent.go`
- **任务**:
  - [x] 定义 `spawnMediaAgentInput` 结构体（`task_brief`, `scope`, `constraints`）
  - [x] 实现 `SpawnMediaAgentToolDef()` 返回 LLM 工具定义
    - name: `spawn_media_agent`
    - description: 描述媒体运营子智能体的能力范围
    - input_schema: 含 `task_brief`(string), `scope`([]ScopeEntry), `constraints`(object)
  - [x] 实现 `executeSpawnMediaAgent()` 函数
    - 创建 `DelegationContract`（constraints 中 `no_network=false`）
    - 构建专属系统提示词（媒体运营角色定义）
    - 通过 `SpawnSubagent` 回调启动独立 LLM session
    - 专属工具集: `trending_topics`, `content_compose`, `media_publish`, `social_interact`, `web_search`, `web_fetch`, `image`
  - [x] 实现 `formatMediaSpawnResult()` 格式化返回

#### P0-2: 新建 `media/types.go` 公共类型

- **文件**: `backend/internal/media/types.go` [NEW]
- **任务**:
  - [x] 定义 `TrendingTopic` 结构体

    ```
    Title, Source, URL, HeatScore, Category, FetchedAt
    ```

  - [x] 定义 `ContentDraft` 结构体

    ```
    ID, Title, Body, Images[], Tags[], Platform, Style, Status, CreatedAt, UpdatedAt
    ```

  - [x] 定义 `PublishResult` 结构体

    ```
    Platform, PostID, URL, Status, PublishedAt, Error
    ```

  - [x] 定义 `InteractionItem` 结构体

    ```
    Type(comment/dm), Platform, NoteID, AuthorName, Content, Timestamp
    ```

  - [x] 定义各种枚举常量
    - `Platform`: wechat / xiaohongshu / website
    - `ContentStyle`: informative / casual / professional
    - `DraftStatus`: draft / pending_review / approved / published

#### P0-3: 频道 ID 注册

- **文件**: `backend/internal/channels/channels.go` [MODIFY]
- **任务**:
  - [x] 在 `media/types.go` 中独立定义（避免修改主系统）:

    ```go
    ChannelWeChatMP    channels.ChannelID = "wechat_mp"
    ChannelXiaohongshu channels.ChannelID = "xiaohongshu"
    ```

  - [x] 注意: `website` 渠道使用现有 `ChannelWeb` 或新增 `ChannelWebsite`（集成时处理）

#### P0-4: 工具注册表扩展

- **文件**: `backend/internal/media/media_registry.go` [NEW]
- **注意**: 原计划修改 `tools/registry.go`，改为独立文件实现以遵循安全第一原则
- **任务**:
  - [x] `MediaToolsConfig` 配置结构体（含 DraftStore、TrendingAggregator 依赖注入）
  - [x] 工具名常量定义（`trending_topics`, `content_compose`, `media_publish`, `social_interact`）
  - [x] `DefaultMediaToolDefs()` 工具清单（含启用/禁用控制）
  - [x] `LogMediaToolsRegistration()` 启动日志
  - [x] `MediaToolExecutor` 接口占位

#### P0-5: 草稿存储模块

- **文件**: `backend/internal/media/draft_store.go` [NEW]
- **任务**:
  - [x] 实现 `DraftStore` 接口

    ```go
    Save(draft *ContentDraft) error
    Get(id string) (*ContentDraft, error)
    List(platform string) ([]*ContentDraft, error)
    UpdateStatus(id string, status DraftStatus) error
    ```

  - [x] 实现 `FileDraftStore`（JSON 文件存储到 `_media/drafts/`）
  - [x] 草稿 ID 生成策略（UUID）
  - [x] `validateID()` 路径遍历防护
  - [x] `sync.Mutex` 并发写入保护
  - [x] 8 项单元测试全部通过（含路径遍历测试）

#### P0-6: 热点源聚合模块

- **文件**: `backend/internal/media/trending.go` [NEW]
- **任务**:
  - [x] 定义 `TrendingSource` 接口

    ```go
    Fetch(ctx context.Context, category string, limit int) ([]TrendingTopic, error)
    Name() string
    ```

  - [x] 实现 `TrendingAggregator`（组合多个 source）
  - [x] 并发拉取 + 错误隔离（单源失败不影响其他源）
  - [x] 结果按 HeatScore 降序排序 + 全局 limit
  - [x] `FetchBySource()` 指定源拉取
  - [x] 6 项单元测试全部通过

---

### Phase 1：热点采集 + 内容生成（~3 天）

#### P1-1: 热点采集工具 `trending_tool.go`

- **文件**: `backend/internal/media/trending_tool.go` [NEW] ← 放在 media/ 包避免循环依赖
- **依赖**: `backend/internal/media/media_tool.go` [NEW] — MediaTool 本地类型定义
- **任务**:
  - [x] 实现 `CreateTrendingTool()` 返回 `*MediaTool`（不直接返回 `*AgentTool` 以避免 tools→channels→media 循环依赖）
  - [x] 工具 schema 定义:
    - name: `trending_topics`
    - actions: `fetch` / `analyze` / `list_sources`
    - params: `source`(weibo/baidu/zhihu/douyin/general), `category`(tech/finance/entertainment/all), `limit`(默认10)
  - [x] `fetch` action: 通过 `TrendingAggregator.FetchAll()` / `FetchBySource()` 拉取
    - 具体的热搜 API 源（微博/百度/知乎）在集成时通过 `TrendingSource` 接口注册
  - [x] `analyze` action: 格式化摘要输出（LLM 分析由 session 内完成）
  - [x] `list_sources` action: 返回已注册数据源清单
  - [x] 结果统一转换为 `[]TrendingTopic` → `jsonMediaResult()`
  - [x] 错误处理: 单个源失败不影响其他源
  - [x] 6 项单元测试全部通过

#### P1-2: 内容生成工具 `content_compose_tool.go`

- **文件**: `backend/internal/media/content_compose_tool.go` [NEW] ← 放在 media/ 包避免循环依赖
- **任务**:
  - [x] 实现 `CreateContentComposeTool()` 返回 `*MediaTool`
  - [x] 工具 schema 定义:
    - name: `content_compose`
    - actions: `draft` / `preview` / `revise` / `list`
    - params: `platform`, `title`, `body`, `tags`, `style`, `draft_id`, `revise_notes`
  - [x] `draft` action: 创建草稿 → DraftStore.Save()（LLM 通过 session 内自身能力生成文案）
  - [x] `preview` action: DraftStore.Get() → 格式化预览输出
  - [x] `revise` action: DraftStore.Get() → 更新 → DraftStore.Save()，状态重置为 draft
  - [x] `list` action: DraftStore.List() → 按 platform 过滤
  - [x] 平台特定约束（使用 `utf8.RuneCountInString()` 正确计算中文字符）:
    - 公众号: 标题 ≤64字符
    - 小红书: 标题 ≤20字符, 正文 ≤1000字
    - 网站: 无特殊限制
  - [x] 7 项单元测试全部通过（含平台约束边界测试）

#### P1-3: 草稿存储验证

- **文件**: `backend/internal/media/draft_store.go`（P0-5 中已完整实现）
- **状态**: ✅ P0-5 已完成全部功能，P1-3 无需额外代码
- **已验证**:
  - [x] `FileDraftStore.Save()` — JSON 序列化写入 + UUID 生成 + 路径遍历防护
  - [x] `FileDraftStore.Get()` — 读取并反序列化
  - [x] `FileDraftStore.List()` — 遍历目录，按 platform 过滤
  - [x] `FileDraftStore.UpdateStatus()` — 原地更新 status 字段
  - [x] `_media/drafts/` 目录自动创建
  - [x] `sync.Mutex` 并发写入保护
  - [x] 8 项单元测试通过

---

### Phase 2：微信公众号发布（~3 天）

#### P2-1: 微信公众号 API 客户端

- **文件**: `backend/internal/channels/wechat_mp/client.go` [NEW]
- **任务**:
  - [x] 定义 `WeChatMPClient` 结构体

    ```
    AppID, AppSecret string
    accessToken string
    tokenExpiry time.Time
    mu sync.Mutex
    httpClient *http.Client
    ```

  - [x] 实现 `GetAccessToken()`:
    - [x] 缓存 token，提前 5 分钟视为过期
    - [x] mutex 保护并发刷新
    - [x] 调用 `GET /cgi-bin/token?grant_type=client_credential&appid=...&secret=...`
    - [ ] 可选: token 持久化到 `_system/wechat_mp/token.json`
  - [x] 实现 `UploadImage(filePath string) (mediaID string, err error)`:
    - [x] `POST /cgi-bin/media/uploadimg`
    - [x] 校验: jpg/png, ≤1MB
  - [x] 实现 `doRequest()` 通用请求封装 + 错误码处理
  - [x] 实现 rate limiter（微信 API 有速率限制）

#### P2-2: 微信公众号发布流程

- **文件**: `backend/internal/channels/wechat_mp/publish.go` [NEW]
- **任务**:
  - [x] 实现 `CreateDraft(draft *media.ContentDraft) (draftMediaID string, err error)`:
    - [x] 上传图片素材 → 获取 media_id
    - [x] 构建图文消息 JSON（title, content, thumb_media_id）
    - [x] `POST /cgi-bin/draft/add`
  - [x] 实现 `SubmitPublish(draftMediaID string) (publishID string, err error)`:
    - [x] `POST /cgi-bin/freepublish/submit`
  - [x] 实现 `GetPublishStatus(publishID string) (*media.PublishResult, error)`:
    - [x] `POST /cgi-bin/freepublish/get`
    - [x] 轮询直到状态确定
  - [x] 实现完整发布链路: `PublishDraft(draft) → upload → create → submit → poll`

#### P2-3: 微信公众号配置

- **文件**: `backend/internal/channels/wechat_mp/config.go` [NEW]
- **任务**:
  - [x] 定义 `WeChatMPConfig` 结构体

    ```go
    Enabled        bool   `yaml:"enabled"`
    AppID          string `yaml:"app_id"`
    AppSecret      string `yaml:"app_secret"`
    TokenCachePath string `yaml:"token_cache_path"`
    ```

  - [x] 实现 `Validate()` 配置校验（`LoadConfig` 需集成时实现）

#### P2-4: 微信公众号 Plugin 实现

- **文件**: `backend/internal/channels/wechat_mp/plugin.go` [NEW]
- **任务**:
  - [x] 实现 `channels.Plugin` 接口:
    - `ID() → ChannelWeChatMP`
    - `Start(accountID)` → 初始化 client + 测试 token 获取
    - `Stop(accountID)` → 清理资源
  - [ ] 可选: 实现 `channels.MessageSender` 接口（支持模板消息推送，集成时实现）

#### P2-5: 平台发布工具 `media_publish_tool.go`

- **文件**: `backend/internal/agents/tools/media_publish_tool.go` [NEW]
- **任务**:
  - [x] 实现 `CreateMediaPublishTool()` 返回 `*MediaTool`（放在 media 包避免循环依赖）
  - [x] 工具 schema 定义:
    - name: `media_publish`
    - actions: `publish` / `approve` / `status`
    - params: `draft_id`, `platform`
  - [x] `publish` action — 路由到对应平台适配器:
    - [x] `wechat` → 通过 `MediaPublisher` 接口路由
    - [ ] `xiaohongshu` → Phase 3 实现
    - [ ] `website` → Phase 4 实现
  - [x] `approve` action — 审批门控（状态 → approved）
  - [ ] `schedule` action — 调用 cron 工具延迟发布（集成时实现）
  - [x] `status` action — 查询发布状态

#### P2-6: 启动注册

- **文件**: `backend/internal/media/bootstrap.go` [NEW] ← 独立模块，不修改主系统
- **文件**: `backend/internal/media/bootstrap_test.go` [NEW]
- **任务**:
  - [x] `MediaSubsystem` 聚合结构体 + `NewMediaSubsystem()` 工厂函数
  - [x] `buildMediaTools()` 工具实例化（trending + compose + publish + interact）
  - [x] `RegisterPublisher()` 平台发布器注入
  - [x] `GetTool()` / `ToolNames()` 工具查询
  - [x] 4 项单元测试全部通过
  - [ ] 集成时在 `registry.go` / `boot.go` 中调用 `NewMediaSubsystem()` 接入

---

### Phase 3：小红书 RPA 自动化（~5 天）

#### P3-1: RPA 浏览器自动化客户端

- **文件**: `backend/internal/channels/xiaohongshu/rpa_client.go` [NEW]
- **任务**:
  - [x] 定义 `XHSRPAClient` 结构体（Cookie 管理 + 频率控制）
  - [x] 实现 Cookie 登录机制:
    - [x] `LoadCookies()` 加载持久化 Cookie
    - [x] `CheckCookieValid()` 检测 Cookie 过期
  - [x] 实现 `Publish(ctx, draft)` — 实现 `media.MediaPublisher` 接口:
    - [ ] 浏览器自动化操作（集成阶段实现）
  - [x] 操作频率控制: 每个操作间隔 ≥5 秒
  - [x] 反检测措施: 随机延迟（0~2 秒 jitter）
  - [x] 错误截图目录配置 `_media/xhs/errors/`

#### P3-2: 评论/私信互动管理

- **文件**: `backend/internal/channels/xiaohongshu/interactions.go` [NEW]
- **任务**:
  - [x] 定义 `InteractionManager` 接口
  - [x] 实现 `RPAInteractionManager` 结构体
  - [x] 实现 `ListComments(ctx, noteID)` — 框架 + TODO 浏览器操作
  - [x] 实现 `ReplyComment(ctx, noteID, commentID, reply)` — 框架 + 去重标记
  - [x] 实现 `ListDMs(ctx)` — 框架 + TODO 浏览器操作
  - [x] 实现 `ReplyDM(ctx, userID, message)` — 框架 + 去重标记
  - [x] 已处理 ID 去重机制（`processed` map）
  - [x] 所有操作的频率限制（≥5 秒间隔）

#### P3-3: 社交互动工具 `social_interact_tool.go`

- **文件**: `backend/internal/agents/tools/social_interact_tool.go` [NEW]
- **任务**:
  - [x] 实现 `CreateSocialInteractTool()` 返回 `*MediaTool`（放在 media 包避免循环依赖）
  - [x] 定义 `SocialInteractor` 接口（消费侧定义）
  - [x] 工具 schema 定义:
    - name: `social_interact`
    - actions: `reply_comment` / `reply_dm` / `list_comments` / `list_dms`
    - params: `note_id`, `comment_id`, `user_id`, `message`
  - [x] 路由到 `SocialInteractor` 接口实现
  - [x] 8 项单元测试全部通过
  - [ ] `auto_mode=true` 时: LLM 自动生成回复内容（集成阶段）

#### P3-4: 定时互动 Cron 任务

- **文件**: 配置文件 / cron 注册代码 [MODIFY]
- **状态**: ⏭️ 延迟到集成阶段（需要主系统 cron 接入）
- **任务**:
  - [ ] 配置 cron 定时任务（每 30 分钟）
  - [ ] 任务内容: `spawn_media_agent(task: "检查小红书新评论和私信，智能回复")`
  - [x] 已回复评论/私信的去重机制（在 `interactions.go` 中已实现 `processed` map）

#### P3-5: 小红书 Plugin + 配置

- **文件**: `backend/internal/channels/xiaohongshu/config.go` [NEW]
- **文件**: `backend/internal/channels/xiaohongshu/plugin.go` [NEW]
- **任务**:
  - [x] `XiaohongshuConfig`: Enabled, CookiePath, AutoInteractInterval, RateLimitSeconds, ErrorScreenshotDir
  - [x] `Validate()` 配置校验
  - [x] `DefaultConfig()` 默认配置
  - [x] 实现 `channels.Plugin` 接口（ID/Start/Stop）
  - [x] `ConfigureAccount()` 注入配置 + 创建 RPA 客户端和互动管理器
  - [x] `GetClient()` / `GetInteractionManager()` 按账号获取
  - [ ] 在 `channels.Manager` 中注册（集成阶段）

---

### Phase 4：自有网站 + 打磨（~2 天）

#### P4-1: 通用 REST 发布器

- **文件**: `backend/internal/channels/website/rest_client.go` [NEW]
- **任务**:
  - [x] 定义 `WebsiteClient` 结构体
  - [x] 支持多种认证方式: Bearer Token / API Key / Basic Auth
  - [x] 实现 `Publish(draft *media.ContentDraft) (*media.PublishResult, error)`:
    - [x] POST JSON `{ title, content, images, tags }` 到配置的 API URL
    - [x] 支持图片先上传到 `image_upload_url` 再引用
  - [x] 支持 WordPress REST API / Ghost CMS / 自研 CMS
  - [x] 实现重试机制 + 超时配置
  - [x] 8 项单元测试通过（rest_client_test.go）

#### P4-2: 网站 Plugin + 配置

- **文件**: `backend/internal/channels/website/config.go` [NEW]
- **文件**: `backend/internal/channels/website/plugin.go` [NEW]
- **任务**:
  - [x] `WebsiteConfig`:

    ```go
    Enabled        bool   `yaml:"enabled"`
    APIURL         string `yaml:"api_url"`
    AuthType       string `yaml:"auth_type"` // bearer / api_key / basic
    AuthToken      string `yaml:"auth_token"`
    ImageUploadURL string `yaml:"image_upload_url"`
    ```

  - [x] 实现 `channels.Plugin` 接口
  - [x] 在 `media_publish_tool.go` 中补充 `website` 分支（通过动态平台注册实现）
  - [x] 6 项单元测试通过（plugin_test.go）

#### P4-3: 端到端集成

- **任务**:
  - [x] `media_publish_tool.go` 补充动态平台注册错误消息
  - [x] `publish_tool_test.go` 补充 `website` 平台测试
  - [ ] `media_publish_tool.go` 补充 `xiaohongshu` + `website` 发布分支 — 通过 `MediaPublisher` 接口动态注册实现
  - [ ] spawn_media_agent 系统提示词调优 — 延迟到集成阶段
  - [ ] 各平台错误码映射和用户友好消息 — 延迟到集成阶段

#### P4-4: 系统提示词设计

- **文件**: `backend/internal/media/system_prompt.go` [NEW] ← 独立于 runner 包，集成时调用
- **测试**: `backend/internal/media/system_prompt_test.go` [NEW]
- **审计**: `docs/meiti/audit-20260301-media-p44-system-prompt.md`
- **状态**: ✅ 已完成（272L 实现 + 145L 测试，8/8 PASS）
- **任务**:
  - [x] 定义 oa-media 角色: "你是一个媒体运营助手…"
  - [x] 明确能力边界和工具使用指南
  - [x] 强调: 所有发布必须生成草稿等待审批
  - [x] 各平台内容规范提示
  - [x] 12-section 架构（对标 coder 提示词，新增 HITL/社交互动/内容创作/质量标准段）
  - [x] ContractFormatter 接口避免循环依赖

---

## 四、验证计划

### 4.1 单元测试

| 测试文件 | 覆盖内容 | 命令 |
|---------|---------|------|
| `runner/spawn_media_agent_test.go` | 合约创建 + 权限衰减 | `go test ./internal/agents/runner/ -run TestSpawnMedia` |
| `tools/trending_tool_test.go` | mock 搜索结果解析 | `go test ./internal/agents/tools/ -run TestTrending` |
| `channels/wechat_mp/client_test.go` | token 缓存/刷新逻辑 | `go test ./internal/channels/wechat_mp/ -run TestToken` |
| `tools/media_publish_tool_test.go` | 各平台参数校验 | `go test ./internal/agents/tools/ -run TestMediaPublish` |
| `media/draft_test.go` | 草稿 CRUD 操作 | `go test ./internal/media/ -run TestDraft` |

### 4.2 集成测试

- [ ] **子智能体全链路**: 主智能体 → spawn_media_agent → 热点采集 → 草稿生成 → announce 回报
- [ ] **微信公众号沙盒**: 使用微信测试号进行发布测试
- [ ] **小红书 RPA**: 在测试账号上发布 + 互动验证

### 4.3 手动验证

- [ ] 用户对话中触发完整发布流程
- [ ] 确认"用户确认后才发布"的门控生效
- [ ] 验证 cron 驱动的小红书自动互动
- [ ] 各平台发布内容格式正确性

---

## 五、风险与前置依赖

| 风险/依赖 | 影响 | 缓解措施 |
|-----------|-----|---------|
| 微信服务号认证 | P2 无法进行 | 提前申请认证，先用测试号开发 |
| 小红书 API 权限 | P3 走 RPA 路径 | Phase 1 先 RPA，Phase 2 申请 API |
| 小红书反爬检测 | RPA 被封控 | 操作频率 ≥5s，随机延迟，备用账号 |
| 微信 API 速率限制 | 批量发布受限 | client 层 rate limiter |
| LLM 生成质量 | 内容不达标 | 系统提示词调优 + 人工审批兜底 |

---

## 六、新建文件清单（共 ~20 个文件）

| Phase | 文件 | 类型 |
|-------|------|------|
| P0 | `runner/spawn_media_agent.go` | 子智能体生成 |
| P0 | `media/types.go` | 公共类型 |
| P0 | `media/draft.go` | 草稿存储 |
| P0 | `media/trending.go` | 热点聚合 |
| P1 | `tools/trending_tool.go` | 热点工具 |
| P1 | `tools/content_compose_tool.go` | 内容生成工具 |
| P2 | `channels/wechat_mp/client.go` | 微信 API 客户端 |
| P2 | `channels/wechat_mp/publish.go` | 发布流程 |
| P2 | `channels/wechat_mp/config.go` | 配置 |
| P2 | `channels/wechat_mp/plugin.go` | Plugin 实现 |
| P2 | `tools/media_publish_tool.go` | 发布工具 |
| P3 | `channels/xiaohongshu/rpa_client.go` | RPA 客户端 |
| P3 | `channels/xiaohongshu/interactions.go` | 互动管理 |
| P3 | `channels/xiaohongshu/config.go` | 配置 |
| P3 | `channels/xiaohongshu/plugin.go` | Plugin 实现 |
| P3 | `tools/social_interact_tool.go` | 互动工具 |
| P4 | `channels/website/rest_client.go` | REST 发布器 |
| P4 | `channels/website/config.go` | 配置 |
| P4 | `channels/website/plugin.go` | Plugin 实现 |

**修改文件**: `channels/channels.go`, `tools/registry.go`, `config.yaml`, boot/server 初始化代码
