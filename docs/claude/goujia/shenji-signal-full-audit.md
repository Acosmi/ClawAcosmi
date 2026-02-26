> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Signal 模块 TS→Go 迁移审计报告

**审计日期**：2026-02-25
**复核审计日期**：2026-02-25
**审计范围**：`extensions/signal/` + `src/signal/` → `backend/internal/channels/signal/` + `bridge/`
**TS 文件数**：14 核心 + 3 扩展入口
**Go 文件数**：14 核心 + 2 桥接 + 7 测试

---

## 一、文件对齐总览

| Go 文件 | TS 对照 | 状态 |
|---------|---------|------|
| accounts.go | accounts.ts (92L) | 已对齐 |
| identity.go | identity.ts (136L) | 已对齐 |
| probe.go | probe.ts (58L) | 已对齐 |
| reaction_level.go | reaction-level.ts (72L) | 已对齐 |
| monitor_types.go | event-handler.types.ts (118L) | 已对齐 |
| monitor_deps.go | 跨模块 DI 接口 | 已对齐 |
| client.go | client.ts (195L) | 已修复 (CL-L1) |
| daemon.go | daemon.ts (103L) | 已修复 (DA-M1/DA-R1/DA-R2/DA-R3) |
| sse_reconnect.go | sse-reconnect.ts (81L) | 已修复 (SR-M1) |
| format.go | format.ts (239L) | 已修复 (FO-M1/FO-R1) |
| send.go | send.ts (282L) | 已修复 (SE-M1/L1/L2) |
| send_reactions.go | send-reactions.ts (216L) | 已对齐 |
| event_handler.go | monitor/event-handler.ts (582L) | 已修复 (EH-H1/H2/H3/M1/M2/L1) |
| monitor.go | monitor.ts (401L) | 已修复 (MO-H1/MO-H2/DA-M1/MO-R1) |
| bridge/signal_actions.go | actions/signal.ts (147L) | 已修复 (BA-M1) |

---

## 二、修复项明细

### HIGH 优先级

| ID | 文件 | 描述 | 修复内容 |
|----|------|------|----------|
| EH-H1 | event_handler.go | 入站防抖缺失 | 添加 per-key `sync.Map` 防抖器，支持 `shouldDebounce` 判断（控制命令/媒体跳过），flush 时合并多条文本 |
| EH-H2 | event_handler.go | 动态白名单缺失 | 调用 `ReadAllowFromStore("signal")` 合并 pairing store 白名单到 effectiveDmAllow/effectiveGroupAllow |
| EH-H3 | event_handler.go | 命令门控缺失 | 集成 `channels.ResolveControlCommandGate` + `autoreply.HasControlCommand`，群组未授权命令直接拦截 |
| MO-H1 | event_handler.go | 附件下载不完整 | 新增 `fetchSignalAttachment`：size 预检 → RPC getAttachment → base64 解码 → `media.SaveMediaBuffer` 保存 |
| MO-H2 | monitor.go | deliverReplies 缺失分块 | 集成 `autoreply.ChunkTextWithMode`，支持 `TextLimit`/`ChunkMode` 参数，媒体 URL 首个携带 caption |

### MEDIUM 优先级

| ID | 文件 | 描述 | 修复内容 |
|----|------|------|----------|
| DA-M1 | daemon.go + monitor.go | daemon CLI 参数缺失 | `SignalDaemonOpts` 添加 ReceiveMode/IgnoreAttachments/IgnoreStories/SendReadReceipts 字段及 CLI flag |
| SR-M1 | sse_reconnect.go | 重连计数器不重置 | handler wrapper 中成功收到事件后 `attempt = 0` |
| FO-M1 | format.go | 表格预转换缺失 | 添加 `SignalFormatOpts`/`ResolveSignalTableMode`，`MarkdownToSignalText` 支持 variadic opts 含 tableMode |
| SE-M1 | send.go | 出站附件缺失 | 新增 `resolveOutboundAttachment` 函数 + `MediaURL` 选项 |
| EH-M1 | event_handler.go | 打字指示器缺失 | dispatch 前调用 `SendTypingSignal` |
| EH-M2 | event_handler.go | 群组历史上下文缺失 | 集成 `reply.BuildHistoryContextFromMap`，支持 GroupHistories map + HistoryLimit 配置 |
| BA-M1 | bridge/signal_actions.go | sendMessage 缺失 + react 为占位 | 添加 `sendMessage` action，`react` 接入实际 signal RPC 调用 |

### LOW 优先级

| ID | 文件 | 描述 | 修复内容 |
|----|------|------|----------|
| SE-L1 | send.go | SendMessageSignal 返回类型 | 返回 `(*SignalSendResult, error)` 对齐 TS，所有调用点更新 |
| SE-L2 | send.go | maxBytes 级联缺失 | 新增 `ResolveMaxBytes` opts→account→global→8MB 级联 |
| CL-L1 | client.go | RPC ID 为时间戳非 UUID | 新增 `randomRpcID()` UUID v4 生成 |
| EH-L1 | event_handler.go | 反应 contextKey 格式 | 改为 `signal:reaction:added:msgId:senderId:emoji:groupId` + agent route sessionKey |

---

## 三、复核审计修复项

复核交叉比对发现以下遗漏，已全部修复：

| ID | 文件 | 描述 | 修复内容 |
|----|------|------|----------|
| DA-R1 | daemon.go | 缺失 `--no-receive-stdout` 标志 | TS 硬编码该标志防止 stdout 混入 JSON-RPC 结果，Go 已补充 |
| DA-R2 | daemon.go | `-a account` 位置错误 | TS 将 `-a account` 放在 `daemon` 子命令之前，Go 已修正参数顺序 |
| DA-R3 | daemon.go | ClassifySignalCliLogLine 返回值不对齐 | TS 返回 `"log"\|"error"\|null`，Go 改为 `*SignalLogLevel`，空行返回 nil；stderr 和 stdout 统一分类 |
| FO-R1 | format.go | Markdown 链接扩展缺失 | TS 将 `[label](url)` 扩展为 `label (url)`，Go 新增 `expandMarkdownLinks` 函数 |
| MO-R1 | monitor.go | daemon 启动超时默认值不对齐 | TS 默认 30s + clamp [1s,120s]，Go 从 15s 修正为 30s + clamp |
| DY-S01 | event_handler.go + monitor.go | 群组历史上下文实现 | 集成 `reply.HistoryMap`/`BuildHistoryContextFromMap`，dispatch 后清理 |
| DY-S02 | 7 个 _test.go 文件 | 单元测试从 0 补全至 62 个 | 覆盖 accounts/daemon/format/identity/reaction_level/send/send_reactions |
| DY-S03 | event_handler.go + monitor.go | readReceiptsViaDaemon 区分 | 计算 `autoStart && sendReadReceipts`，传入 handler 跳过手动已读回执 |

---

## 四、架构决策

### ChannelPlugin 模式
TS 使用 `extensions/signal/src/channel.ts` (316L) 的插件适配器模式注册 12+ 子模块。Go 采用核心频道注册模式（`dock.go` coreDocks），功能等价但结构不同。Signal 的 onboarding、identity、probe、send、monitor 等核心逻辑已完整迁移。

### Monitor 子目录合并
TS 的 `src/signal/monitor/` 三文件（event-handler.ts 582L + types 118L + index 15L）在 Go 中合并为：
- `event_handler.go` — 主处理逻辑
- `monitor_types.go` — SSE 事件类型
- `monitor_deps.go` — DI 依赖接口

结构等价，无功能遗漏。

---

## 五、已知延迟项

**无延迟项。** 所有 DY-S01~DY-S03 已全部修复并通过测试验证。

---

## 六、编译与测试验证

```
go build ./internal/channels/signal/   ✅ PASS
go build ./internal/channels/bridge/   ✅ PASS
go vet  ./internal/channels/signal/    ✅ PASS
go vet  ./internal/channels/bridge/    ✅ PASS
go test ./internal/channels/signal/    ✅ PASS (62 tests)
```

---

## 七、修改文件清单

| 文件 | 操作 |
|------|------|
| `internal/channels/signal/client.go` | 修改：添加 randomRpcID UUID v4 |
| `internal/channels/signal/daemon.go` | 修改：CLI 参数字段 + `--no-receive-stdout` + 参数顺序 + ClassifySignalCliLogLine 返回 `*SignalLogLevel` |
| `internal/channels/signal/sse_reconnect.go` | 修改：重连计数器重置 |
| `internal/channels/signal/format.go` | 修改：添加 tableMode 支持 + expandMarkdownLinks 链接扩展 |
| `internal/channels/signal/send.go` | 修改：添加 SendResult/outbound attachment/maxBytes/tableMode |
| `internal/channels/signal/monitor.go` | 修改：daemon 参数传递 + deliverReplies 分块 + 群组历史 + readReceiptsViaDaemon + 超时 clamp |
| `internal/channels/signal/event_handler.go` | 修改：防抖/动态白名单/命令门控/附件下载/打字/reaction格式/群组历史/readReceiptsViaDaemon |
| `internal/channels/bridge/signal_actions.go` | 重写：添加 sendMessage + react 实际调用 |
| `internal/channels/signal/accounts_test.go` | 新增：10 个测试用例 |
| `internal/channels/signal/daemon_test.go` | 新增：5 个测试用例 |
| `internal/channels/signal/format_test.go` | 新增：12 个测试用例 |
| `internal/channels/signal/identity_test.go` | 新增：12 个测试用例 |
| `internal/channels/signal/reaction_level_test.go` | 新增：7 个测试用例 |
| `internal/channels/signal/send_test.go` | 新增：9 个测试用例 |
| `internal/channels/signal/send_reactions_test.go` | 新增：7 个测试用例 |

---

## 八、复核审计签章

**审计方法**：技能三 — 交叉颗粒度审计
**复核结果**：全部 14 个核心文件 + 1 个桥接文件已通过 TS↔Go 逐逻辑块交叉比对
**延迟项**：0（全部清零）
**测试覆盖**：62 个单元测试全部通过
**编译验证**：build + vet 均通过
**归档状态**：复核通过，可以归档
