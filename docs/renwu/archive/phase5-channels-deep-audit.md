# Phase 5 深度审计报告 — 频道系统 (`src/channels/`)

> 审计日期：2026-02-13
> 审计范围：`src/channels/` 全量 75 个非测试 TS 文件（约 6000 行）

## 一、总体发现

初始审计严重低估了频道系统的规模和复杂度：

| 指标 | 初始估计 | 审计实际 | 倍率 |
|------|----------|----------|------|
| 核心代码量 | ~2100 行 | ~5300 行 | 2.5x |
| 核心模块数 | 个位数 | 75 个文件 | — |
| 隐藏依赖 | 0 | 6 项关键 | — |

## 二、四层架构分析

### 第 1 层：核心基础设施（必须移植）

| 文件 | 行数 | 状态 | 职责 |
|------|------|------|------|
| `dock.ts` | 457 | ❌ 未移植 | **频道行为中枢**：每个频道的能力、出站限制、流式默认值、AllowFrom 解析、@提及模式、线程处理 |
| `registry.ts` | 180 | ❌ 未移植 | 频道元数据（标签、文档路径、别名、排序） |
| `types.core.ts` | 332 | ⚠️ 部分 | 25+ 接口类型（ChannelCapabilities、ChannelMeta 等） |
| `types.adapters.ts` | 313 | ❌ 未移植 | 14 个适配器接口 |
| `types.plugin.ts` | 85 | ❌ 未移植 | ChannelPlugin 契约（23 个字段槽位） |

### 第 2 层：共享工具（必须移植）

| 文件 | 行数 | 职责 |
|------|------|------|
| `channel-config.ts` | 183 | 通用配置匹配算法（直接→父级→通配符→slug 五级回退） |
| `group-mentions.ts` | 409 | 各频道群组提及/工具策略解析 |
| `mention-gating.ts` | 60 | @提及门控逻辑 |
| `ack-reactions.ts` | 104 | 确认反应范围 + WhatsApp 专属逻辑 |
| `chat-type.ts` | 19 | ChatType 枚举 + 规范化 |
| `conversation-label.ts` | 70 | 对话标签解析 |
| `logging.ts` | 34 | 入站丢弃/打字/确认日志 |

### 第 3 层：插件辅助（随适配器移植）

`config-helpers.ts`(114)、`media-limits.ts`(26)、`config-schema.ts`、`config-writes.ts`、`directory-config.ts`、`helpers.ts`、`message-actions.ts` 等。

### 第 4 层：各频道适配器（~30 文件）

`plugins/normalize/`(6 文件)、`plugins/outbound/`(6 文件)、`plugins/onboarding/`(7 文件)、`plugins/actions/`(3 文件)。

## 三、6 项关键隐藏依赖

### 3.1 `dock.ts` — 频道行为中枢

**完全不在初始审计范围内。** 定义了 7 个频道各自的：

- `capabilities`（支持的聊天类型、投票、反应、媒体等）
- `outbound.textChunkLimit`（Discord 2000，其他 4000）
- `streaming.blockStreamingCoalesceDefaults`
- `config.resolveAllowFrom` / `formatAllowFrom`
- `groups.resolveRequireMention` / `resolveToolPolicy`
- `mentions.stripPatterns`（各频道的 @提及正则）
- `threading.resolveReplyToMode` / `buildToolContext`

### 3.2 Discord 工会→频道两级解析

`group-mentions.ts` L100-184：

1. 精确 guild ID 匹配 → 2. slug 匹配 → 3. 遍历所有 guild 的 slug → 4. `*` 通配符
频道层再做：channelId → channelSlug → `#slug` → normalizedSlug

### 3.3 Slack 频道 slug 规范化

`normalizeSlackSlug`：保留 `#@._+-`，其余替换为 `-`，多候选匹配。

### 3.4 Telegram 话题解析

`parseTelegramGroupId` 支持三种格式：

- `chatId:topic:topicId`
- `chatId:topicId`  
- 纯 `chatId`

配置级联：topicConfig → defaultTopicConfig → groupConfig → groupDefault(`*`)

### 3.5 频道配置五级匹配

`channel-config.ts` L82-164：direct → normalized → parent → normalizedParent → wildcard

### 3.6 ChannelPlugin 23 字段契约

每个频道插件最多实现 23 个适配器槽位（config、setup、security、groups、mentions、outbound、status、gateway、auth 等）。

## 四、对实施计划的影响

> [!IMPORTANT]
> 5B 阶段范围需从约 500 行修订为约 1500 行。5D 各频道适配器建议拆分到单独会话窗口。

### 修订后的执行顺序

1. ✅ **5A.1** 配置延迟项（F9、B3、C1、channel-capabilities）— 已完成
2. **5B.1** 移植 `types.core/adapters/plugin` → Go 接口
3. **5B.2** 移植 `registry.ts` → 频道注册表
4. **5B.3** 移植 `channel-config.ts` → 配置匹配算法
5. **5B.4** 移植 `dock.ts` → 频道行为中枢 ⚠️ 关键节点
6. **5C** 共享工具（chat-type、mention-gating、ack-reactions 等）
7. **5D** 各频道适配器（建议拆分会话）
