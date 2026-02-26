> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 任务 002：Slack 媒体处理 + Slash 命令 + Providers 深度审计

## 涉及文件清单

### 缺口① Slash 命令
- [x] TS: `src/slack/monitor/slash.ts` (630L) ↔ Go: `backend/internal/channels/slack/monitor_slash.go` (~576L)

### 缺口② 媒体处理
- [x] TS: `src/slack/monitor/media.ts` (209L) ↔ Go: `backend/internal/channels/slack/monitor_media.go` (~345L)

### 缺口③ Providers
- [x] TS: `src/providers/github-copilot-auth.ts` (185L) → Go: `backend/internal/providers/github_copilot_auth.go`
- [x] TS: `src/providers/github-copilot-token.ts` (133L) → Go: `backend/internal/providers/github_copilot_token.go`
- [x] TS: `src/providers/github-copilot-models.ts` (42L) → Go: `backend/internal/providers/github_copilot_models.go`
- [x] TS: `src/providers/qwen-portal-oauth.ts` (55L) → Go: `backend/internal/providers/qwen_portal_oauth.go`

### 关联 Go 依赖文件（已读取审查）
- [x] `backend/internal/channels/slack/monitor_context.go` (+UseAccessGroups)
- [x] `backend/internal/channels/slack/allow_list.go`
- [x] `backend/internal/channels/slack/monitor_replies.go`
- [x] `backend/internal/channels/slack/monitor_thread_resolution.go`
- [x] `backend/internal/channels/slack/monitor_channel_config.go`
- [x] `backend/internal/channels/slack/monitor_policy.go`
- [x] `backend/internal/autoreply/reply/inbound_context.go`
- [x] `backend/internal/autoreply/templating.go`
- [x] `backend/internal/autoreply/commands_registry.go`
- [x] `backend/internal/media/store.go`
- [x] `backend/internal/infra/json_file.go`
- [x] `backend/internal/agents/auth/oauth.go`
- [x] `backend/internal/security/channel_metadata.go`
- [x] `backend/pkg/types/types_models.go`

## 审计状态
- 第一步：静默提取与对比 — ✅ 完成
- 第二步：深度审计报告 — ✅ 完成
- 第三步：精准补全 — ✅ 完成（用户已确认）
- 第四步：复核审计 — ✅ 完成（3 个并行子 agent 交叉审计）
- 第五步：编译验证 — ✅ `go build` 通过
- 第六步：文档链更新 — ✅ 完成

## 复核审计结果
- **monitor_media.go ↔ media.ts**: ✅ 完全对齐
- **monitor_slash.go ↔ slash.ts**: ✅ 核心逻辑对齐，延迟项记录至 DY-031~033
- **providers/*.go ↔ providers/*.ts**: ✅ 常量/算法/类型完全匹配，CLI 层延迟 DY-034

## 延迟项
- DY-031: Native 命令注册循环（需入口层支持）
- DY-032: Reply Delivery 高级选项（需 dispatcher 接口）
- DY-033: 交互式参数菜单呈现流（需 action 回调路由）
- DY-034: Providers CLI 交互层（需 cobra 命令层）
- DY-035: Slack 测试覆盖不足
