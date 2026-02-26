> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 深度审计报告 #5：Discord Monitor P1 批次A

审计范围：provider, gateway-registry, reply-delivery, reply-context（共 4 对）
审计日期：2026-02-24

---

## 1. provider.ts (691L) ↔ monitor_provider.go (314L)

**对齐项**：token 校验、discordgo.New、ctx.Done graceful shutdown、botUserId 获取、applyDiscordConfig 基本流程

**WARNING (9项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-014 | `MonitorDiscordOpts` 缺少 mediaMaxMb/historyLimit/replyToMode 等运行时覆盖字段 | MEDIUM |
| W-015 | GuildPresences/GuildMembers 特权 intent 被无条件启用，TS 中是条件启用。可能导致 bot 无法连接 | **HIGH** |
| W-016 | name→ID 的 allowlist resolve 逻辑（约 180 行）完全缺失。用户配置 name 无法被正确匹配 | **HIGH** |
| W-017 | RegisterGateway/UnregisterGateway 未被 provider 调用 | MEDIUM |
| W-018 | slash command deploy/clear 逻辑完全缺失 | MEDIUM |
| W-019 | 30s HELLO timeout 僵尸连接检测缺失 | LOW |
| W-020 | abortSignal + waitForDiscordGatewayStop 完整关闭语义缺失 | LOW |
| W-021 | execApprovalsHandler 初始化和生命周期管理缺失 | MEDIUM |
| W-022 | Go 新增了 TS 中不存在的 GuildMember/Channel 事件处理器（扩展偏离） | LOW |

**INFO**：底层库差异 carbon vs discordgo、RuntimeEnv→slog.Logger、sync.RWMutex 并发保护（正确适配）

---

## 2. gateway-registry.ts (37L) ↔ monitor_gateway_registry.go (59L)

**对齐项**：Map 存储、sentinel key `\0__default__`、register/unregister/get/clear 全部对齐

**WARNING**：无。✅ 完全对齐

**INFO**：Go 添加 RWMutex 保护并发（正确适配）、GatewaySession 接口可能需扩展

---

## 3. reply-delivery.ts (81L) ↔ monitor_reply_delivery.go (97L)

**对齐项**：表格转换、分块、空 chunks fallback、replyTo 仅首 chunk

**WARNING (5项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-023 | TS 支持 ReplyPayload[] 数组迭代，Go 只处理单条文本 | MEDIUM |
| W-024 | media/attachment 发送逻辑完全缺失 | **HIGH** |
| W-025 | 直接调用 session 方法，缺少 TS 的 sendMessageDiscord 抽象层（retry/rate limit） | MEDIUM |
| W-026 | textLimit/chunkLimit 计算未对齐 | LOW |
| W-027 | Go 新增了 reaction 状态标记，TS 无此功能（扩展偏离） | LOW |

---

## 4. reply-context.ts (45L) ↔ monitor_reply_context.go (41L)

**对齐项**：Go 新增了 DiscordReplyCtx 存储结构（合理抽象）

**WARNING (2项)**：

| 编号 | 摘要 | 严重度 |
|------|------|--------|
| W-028 | resolveReplyContext 核心回复链解析逻辑缺失（获取被引用消息 + sender 身份 + envelope 格式化） | **HIGH** |
| W-029 | buildDirectLabel / buildGuildLabel 标签构造函数缺失 | MEDIUM |

**INFO**：文件职责不同 — TS=resolver（解析回复链），Go=store（存储回复状态）

---

## 跨文件汇总

| 严重度 | 数量 | 涉及文件 |
|--------|------|----------|
| **HIGH** | 4 | provider (W-015 intents, W-016 allowlist), reply-delivery (W-024 media), reply-context (W-028 resolve) |
| MEDIUM | 7 | provider (W-014/017/018/021), reply-delivery (W-023/025), reply-context (W-029) |
| LOW | 5 | provider (W-019/020/022), reply-delivery (W-026/027) |

**修复优先级建议**：
1. **P0 阻断性**：W-015（intents 硬编码可能导致连接失败）、W-016（name 无法解析为 ID）
2. **P1 功能缺失**：W-024（media 发送）、W-028（reply chain 解析）、W-023（multi-payload）
3. **P2 生命周期**：W-017（gateway 注册）、W-018（command deploy）、W-021（exec approvals）
4. **P3 健壮性**：W-019（zombie detection）、W-020（graceful shutdown）、W-026（textLimit）
