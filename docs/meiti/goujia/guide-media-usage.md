# oa-media 使用指南

> **包路径**: `backend/internal/media/`
> **更新日期**: 2026-03-01

---

## 快速开始

### 初始化子系统

```go
import "github.com/openacosmi/claw-acismi/internal/media"

sub, err := media.NewMediaSubsystem(media.MediaSubsystemConfig{
    Workspace:      workspaceDir,
    EnablePublish:  true,   // 启用发布工具
    EnableInteract: true,   // 启用互动工具
})
if err != nil {
    log.Fatal(err)
}
```

### 注册平台发布器

```go
// 注册微信公众号发布器
sub.RegisterPublisher(media.PlatformWeChat, wechatPublisher)

// 注册小红书发布器
sub.RegisterPublisher(media.PlatformXiaohongshu, xhsPublisher)

// 注册网站发布器
sub.RegisterPublisher(media.PlatformWebsite, websitePublisher)
```

### 获取工具列表

```go
// 获取所有工具
for _, tool := range sub.Tools {
    fmt.Printf("工具: %s — %s\n", tool.ToolName, tool.ToolDesc)
}

// 按名称获取
publishTool := sub.GetTool("media_publish")
```

### 构建系统提示词

```go
prompt := media.BuildMediaSystemPrompt(media.MediaPromptParams{
    Task:                "采集AI领域热点，生成公众号文章",
    Contract:            delegationContract,
    RequesterSessionKey: sessionKey,
})
```

---

## 典型工作流

### 流程 1: 热点采集 → 内容创作

```
用户下达任务 → 主智能体 spawn_media_agent
    ↓
oa-media 子智能体启动
    ↓
1. trending_topics(fetch) → 获取热点 TOP10
    ↓
2. 子智能体选择最佳话题
    ↓
3. web_search() → 补充素材（主系统工具）
    ↓
4. image() → 生成配图（主系统工具）
    ↓
5. content_compose(draft) → 创建草稿
    ↓
6. ThoughtResult 回报 → 包含 draft_id
    ↓
主智能体展示给用户 → 用户审批
    ↓
media_publish(approve) → media_publish(publish)
```

### 流程 2: 一稿多投

```
content_compose(draft, platform="wechat")    → 公众号草稿
content_compose(draft, platform="xiaohongshu") → 小红书草稿
content_compose(draft, platform="website")     → 网站草稿
    ↓
用户审批3个草稿
    ↓
media_publish(publish) × 3 → 逐平台发布
```

### 流程 3: 社交互动管理

```
social_interact(list_comments, note_id="xxx")
    ↓
子智能体用 LLM 生成回复
    ↓
social_interact(reply_comment, ...) × N（每次间隔 ≥5s）
    ↓
social_interact(list_dms) → reply_dm
    ↓
ThoughtResult 回报互动摘要
```

---

## 草稿生命周期

```
draft → pending_review → approved → published
  ↑         ↓
  └─ revise ┘  （修改后回退为 draft）
```

| 状态 | 触发动作 | 可执行操作 |
|------|---------|-----------|
| `draft` | `content_compose(draft)` | preview / revise / approve |
| `pending_review` | 手动设置 | approve |
| `approved` | `media_publish(approve)` | publish |
| `published` | `media_publish(publish)` | status（只读） |

---

## 扩展指南

### 添加新的热点数据源

```go
type MySource struct{}

func (s *MySource) Name() string { return "my_source" }
func (s *MySource) Fetch(ctx context.Context, category string, limit int) ([]media.TrendingTopic, error) {
    // 实现拉取逻辑
    return topics, nil
}

// 注册到聚合器
sub.Aggregator.AddSource(&MySource{})
```

### 添加新的平台发布器

```go
type MyPublisher struct{}

func (p *MyPublisher) Publish(ctx context.Context, draft *media.ContentDraft) (*media.PublishResult, error) {
    // 实现发布逻辑
    return &media.PublishResult{
        Platform: media.Platform("my_platform"),
        PostID:   "post-123",
        URL:      "https://example.com/post-123",
        Status:   "published",
    }, nil
}

// 注册
sub.RegisterPublisher(media.Platform("my_platform"), &MyPublisher{})
```

---

## 相关文档

| 文档 | 路径 |
|------|------|
| 架构文档 | `docs/meiti/goujia/arch-media-modules.md` |
| 设计文档 | `docs/xinshenji/design-media-subagent-2026-03-01.md` |
| 实施跟踪 | `docs/xinshenji/impl-tracking-media-subagent.md` |
| 技能（公众号） | `docs/meiti/skills/wechat-mp-content/SKILL.md` |
| 技能（小红书） | `docs/meiti/skills/xiaohongshu-content/SKILL.md` |
| 技能（多平台） | `docs/meiti/skills/media-publishing/SKILL.md` |
