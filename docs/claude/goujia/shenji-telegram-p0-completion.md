> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram P0 审计完成报告

**审计范围**：P0 核心消息处理链（12 对文件）
**审计日期**：2026-02-24
**状态**：Phase 1 + Phase 2 修复完成，3 项延迟项记录

---

## 一、审计总览

| # | TS 文件 | Go 文件 | 审计结果 | 修复阶段 |
|---|---------|---------|----------|----------|
| 1 | accounts.ts | accounts.go | 已修复 | Phase 1 |
| 2 | token.ts | token.go | 基本对齐 | — |
| 3 | bot.ts | bot.go | 已修复 | Phase 1 |
| 4 | bot-message.ts | bot_message.go | 有缺口 | → DY-012 |
| 5 | bot-message-context.ts | bot_message_context.go | 已修复 | Phase 2 (#5) |
| 6 | bot-message-dispatch.ts | bot_message_dispatch.go | 已修复 | Phase 2 (#6) |
| 7 | bot-handlers.ts | bot_handlers.go | 已修复 | Phase 2 (#7) |
| 8 | bot-updates.ts | bot_updates.go | 有缺口 | → DY-013 |
| 9 | bot-access.ts | bot_access.go | 基本对齐 | — |
| 10 | bot-native-commands.ts | bot_native_commands.go | 已修复 | Phase 1 |
| 11 | send.ts | send.go | 已修复 | Phase 2 (#8) |
| 12 | monitor.ts | monitor.go | 有缺口 | → DY-014 |

**统计**：7 对已修复，2 对基本对齐（无需修复），3 对有残余缺口（记入延迟项）

---

## 二、Phase 1 CRITICAL 修复清单

### 2.1 bot.go — 默认值缺失
- **问题**：启动配置缺少 TS 版中的多项默认值
- **修复**：补全默认值初始化逻辑

### 2.2 accounts.go — binding 合并遗漏
- **问题**：`ListTelegramAccountIds` 未合并 routing binding 中绑定的账户
- **修复**：合入 `routing.ListBoundAccountIds` 结果

### 2.3 bot_native_commands.go — 重写
- **问题**：命令处理逻辑大量缺失
- **修复**：对齐 TS 完整命令集重写

---

## 三、Phase 2 HIGH 修复清单

### 3.1 Task #5 — bot_message_context.go
**对齐 TS bot-message-context.ts L168-185, L180-185, L330-340, L571**

修复内容：
1. **Agent 路由解析**：添加 `Deps.ResolveAgentRoute` 调用，在 context building 阶段（而非 dispatch 阶段）解析 agentID/sessionKey
2. **DM 线程 session key**：调用 `routing.ResolveThreadSessionKeys` 为 DM 线程生成独立 session key
3. **控制命令门控**：替换硬编码 `false`，改为 `autoreply.HasControlCommand` + `channels.ResolveControlCommandGate` 实际检测
4. **commandBody 规范化**：调用 `autoreply.NormalizeCommandBody` 去除 @botname 前缀

**新建共享模块**：`internal/channels/command_gating.go` — 从 imessage 包提取为共享实现

### 3.2 Task #6 — bot_message_dispatch.go
**对齐 TS bot-message-dispatch.ts L191-243, L311-328, L337-353**

修复内容：
1. **贴纸视觉理解**（步骤 5b）：分发前对未缓存贴纸调用 `Deps.DescribeImage` 获取描述，格式化后更新 context 并缓存
2. **空回复回退**（步骤 7b）：agent 返回 final 但无内容投递时，发送 "No response generated. Please try again."
3. **ACK 反应移除**（步骤 9）：回复投递后根据配置 `RemoveAckAfterReply` 异步移除 ACK 反应
4. **MsgContext 字段修正**：`Body`/`RawBody`/`From`/`To`/`CommandBody`/`GroupSubject` 等字段正确映射

### 3.3 Task #7 — bot_handlers.go
**对齐 TS bot-handlers.ts L60-64, L226-264, L776-836, L77-138**

修复内容：
1. **文本分片聚合**：Telegram 自动拆分 4096+ 字符消息为多条，添加基于时间间隔 + 消息 ID 间隔的重组逻辑
   - 常量：`textFragmentStartThreshold=4000`, `textFragmentMaxGapMs=1500`, `textFragmentMaxIDGap=1`
   - 最大 12 片、50000 字符上限
2. **Inbound debouncing**：同一用户短时间内连续发送的消息合并处理
   - 使用 `autoreply.ResolveInboundDebounceMs` 动态配置延迟
   - 媒体消息合并媒体引用列表

### 3.4 Task #8 — send.go
**对齐 TS send.ts L392-405, L456-472, L488-520, L552-566**

修复内容：
1. **Caption 拆分**：添加 `splitTelegramCaption`（1024 字符上限），超出部分作为独立后续文本消息
2. **asVoice 路由**：利用已有 `ResolveTelegramVoiceSend` 判断是否走 `sendVoice` API
3. **asVideoNote 路由**：视频 + `AsVideoNote=true` 时走 `sendVideoNote` API（不支持 caption，全部转后续文本）
4. **后续文本消息**：caption 拆分后的跟随文本作为独立消息发送，返回文本消息 ID 作为主消息

---

## 四、基本对齐文件（无需修复）

### 4.1 token.ts ↔ token.go
- 核心 token 解析优先级完整：accountTokenFile → accountBotToken → rootTokenFile → rootBotToken → env
- 微小差异：Go 缺少 `logMissingFile` 回调，不影响功能

### 4.2 bot-access.ts ↔ bot_access.go
- 三个核心函数完整对齐：`normalizeAllowFrom`、`isSenderAllowed`、`resolveSenderAllowMatch`
- 微小差异：缺少 `normalizeAllowFromWithStore`、`firstDefined` 辅助函数，当前无调用方依赖

---

## 五、延迟项（记入 daibanyanchi.md）

### DY-012: bot_message.go 配置解析函数缺失
- 缺少 `resolveBotTopicsEnabled`、`resolveGroupActivation`、`resolveGroupRequireMention`、`resolveTelegramGroupConfig`
- 缺少 `historyLimit`、`dmPolicy`、`ackReactionScope`、`telegramCfg` 传参
- 风险：中。影响群组话题、群组激活策略、历史限制等行为

### DY-013: bot_updates.go 去重与媒体组处理简化
- TTL 清理缺失（仅按 maxSize 清理，无过期机制）
- 缺少 `MEDIA_GROUP_TIMEOUT_MS` 与媒体组聚合逻辑
- `ResolveTelegramUpdateID` 过于简化
- 风险：中低。可能在高频更新时内存增长，媒体组可能重复处理

### DY-014: monitor.go Webhook 模式与重试机制缺失
- 仅实现 polling 模式，缺少 webhook 模式支持
- 缺少 Grammy 内置重试机制（maxRetryTime: 5min, exponential retryInterval）
- 缺少 HttpError 未捕获异常处理
- 风险：中。Webhook 模式为生产部署常用方式

---

## 六、新增/修改文件清单

| 操作 | 文件路径 |
|------|----------|
| 新建 | `internal/channels/command_gating.go` |
| 修改 | `internal/channels/telegram/accounts.go` |
| 修改 | `internal/channels/telegram/bot.go` |
| 修改 | `internal/channels/telegram/bot_native_commands.go` |
| 修改 | `internal/channels/telegram/bot_message_context.go` |
| 修改 | `internal/channels/telegram/bot_message_dispatch.go` |
| 修改 | `internal/channels/telegram/bot_handlers.go` |
| 修改 | `internal/channels/telegram/send.go` |

---

## 七、编译验证

- `go build ./internal/channels/telegram/...` ✅ 编译通过
- `go build ./...` 有 discord 包预先存在的编译错误（与本次审计无关）

---

## 八、后续工作建议

1. **优先处理 DY-012**（bot_message.go 配置解析），影响群组行为正确性
2. **P1-P3 审计**：完成 P0 后继续 P1（Bot 子系统）、P2（格式与媒体）、P3（基础设施）审计
3. **复核审计**：按技能三要求，对已修复文件进行逐行交叉比对复审
4. **集成测试**：编写针对修复点的单元/集成测试
