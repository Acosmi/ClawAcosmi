> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Signal 模块 TS→Go 迁移 — 任务完成汇总

**任务周期**：2026-02-25
**任务状态**：全部完成，复核审计通过，延迟项清零

---

## 一、任务背景

Signal 模块由 TS 重构 Go 完成度约 90%，剩余 10% 需补齐，同时对已完成部分进行复核审计。

- **TS 源文件**：14 核心 + 3 扩展入口（总计 ~2,800 行）
- **Go 目标文件**：14 核心 + 2 桥接（迁移前）→ 14 核心 + 2 桥接 + 7 测试（迁移后）
- **执行流程**：技能二 SOP → 审计报告 → 全量修复 → 复核审计（技能三）→ 文档链更新（技能四）

---

## 二、执行阶段总览

### 阶段 1：深度审计与差异报告

逐文件交叉比对 14 个 TS↔Go 文件对，输出审计报告，识别出：
- **HIGH** 级修复项 5 个
- **MEDIUM** 级修复项 7 个（含延迟项升级）
- **LOW** 级修复项 4 个

### 阶段 2：全量修复（HIGH + MEDIUM + LOW）

按优先级分批执行，每批修复后编译验证：

| 批次 | 修复项 | 涉及文件 |
|------|--------|----------|
| Batch 1 | CL-L1, DA-M1, SR-M1, FO-M1, SE-M1/L1/L2 | client.go, daemon.go, sse_reconnect.go, format.go, send.go |
| Batch 2 | SE-L1 调用点修复 | event_handler.go, monitor.go |
| Batch 3 | MO-H1, MO-H2 | event_handler.go (附件下载), monitor.go (deliverReplies 分块) |
| Batch 4 | EH-H1/H2/H3/M1/L1 | event_handler.go (防抖/白名单/命令门控/打字/reaction) |
| Batch 5 | BA-M1 | bridge/signal_actions.go (sendMessage + react 重写) |

### 阶段 3：延迟项消除（DY-S01~S03）

用户明确要求"不要延迟项"，全部修复：

| ID | 描述 | 修复方案 |
|----|------|----------|
| DY-S01 | 群组历史上下文 `buildPendingHistoryContextFromMap` 未实现 | 集成 `reply.HistoryMap` + `BuildHistoryContextFromMap`，monitor.go 创建 map 并计算 historyLimit，dispatch 后 ClearEntries |
| DY-S02 | 单元测试 TS 有 10 个测试文件，Go 为 0 | 新增 7 个 `_test.go` 文件共 62 个测试用例，覆盖全部可纯函数测试的模块 |
| DY-S03 | readReceiptsViaDaemon 未区分 | 计算 `autoStart && sendReadReceipts`，传入 handler，条件跳过手动已读回执 |

### 阶段 4：复核审计（技能三）

对全部 15 个文件对进行交叉颗粒度比对，发现 5 个额外遗漏：

| ID | 描述 | 严重程度 |
|----|------|----------|
| DA-R1 | daemon.go 缺失 `--no-receive-stdout` CLI 标志 | MEDIUM（会导致 stdout 混入不期望的输出） |
| DA-R2 | daemon.go `-a account` 参数应在 `daemon` 子命令之前 | MEDIUM（signal-cli 参数顺序敏感） |
| DA-R3 | ClassifySignalCliLogLine 返回值语义不对齐（Go 无 null，TS 空行返回 null） | LOW（stderr/stdout 统一分类） |
| FO-R1 | format.go 缺失 Markdown 链接扩展 `[label](url)` → `label (url)` | MEDIUM（链接在 Signal 中不可点击，需展开 URL） |
| MO-R1 | monitor.go daemon 启动超时默认 15s，TS 为 30s + clamp [1s, 120s] | LOW（可能导致慢启动环境误报超时） |

全部 5 项已修复并通过测试。

### 阶段 5：文档链更新（技能四）

| 文档 | 操作 |
|------|------|
| `goujia/shenji-signal-full-audit.md` | 更新：添加复核审计修复项（第三节）、延迟项清零（第五节）、测试验证（第六节） |
| `goujia/signal-ts-go-mapping.md` | 维持：15 个文件对全部状态"完成" |
| `daibanyanchi.md` | 更新：Signal 延迟项清零，添加归档记录 |
| `daibanyanchi_guidang.md` | 更新：DY-S01~S03 归档入库 |

---

## 三、修复项全量清单（21 项）

### 初轮审计修复（16 项）

| 优先级 | ID | 文件 | 描述 |
|--------|----|------|------|
| HIGH | EH-H1 | event_handler.go | 入站防抖 — per-key sync.Map 防抖器 |
| HIGH | EH-H2 | event_handler.go | 动态白名单 — ReadAllowFromStore 合并 |
| HIGH | EH-H3 | event_handler.go | 命令门控 — ResolveControlCommandGate 集成 |
| HIGH | MO-H1 | event_handler.go | 附件下载 — fetchSignalAttachment 完整流水线 |
| HIGH | MO-H2 | monitor.go | deliverReplies 分块 — ChunkTextWithMode 集成 |
| MEDIUM | DA-M1 | daemon.go + monitor.go | daemon CLI 参数字段 |
| MEDIUM | SR-M1 | sse_reconnect.go | 重连计数器重置 |
| MEDIUM | FO-M1 | format.go | 表格预转换 tableMode |
| MEDIUM | SE-M1 | send.go | 出站附件 resolveOutboundAttachment |
| MEDIUM | EH-M1 | event_handler.go | 打字指示器 SendTypingSignal |
| MEDIUM | EH-M2 | event_handler.go | 群组历史上下文 BuildHistoryContextFromMap |
| MEDIUM | BA-M1 | signal_actions.go | sendMessage + react 实际调用 |
| LOW | SE-L1 | send.go | SendMessageSignal 返回 (*SignalSendResult, error) |
| LOW | SE-L2 | send.go | ResolveMaxBytes 级联 |
| LOW | CL-L1 | client.go | randomRpcID UUID v4 |
| LOW | EH-L1 | event_handler.go | reaction contextKey 格式对齐 |

### 复核审计修复（5 项）

| ID | 文件 | 描述 |
|----|------|------|
| DA-R1 | daemon.go | 补充 `--no-receive-stdout` 标志 |
| DA-R2 | daemon.go | `-a account` 参数移至 daemon 子命令前 |
| DA-R3 | daemon.go | ClassifySignalCliLogLine 返回 `*SignalLogLevel`，空行返回 nil |
| FO-R1 | format.go | 新增 expandMarkdownLinks 链接扩展 |
| MO-R1 | monitor.go | 启动超时 15s→30s + clamp [1s,120s] |

---

## 四、测试覆盖

| 测试文件 | 用例数 | 覆盖模块 |
|----------|--------|----------|
| accounts_test.go | 10 | NormalizeAccountID, ListSignalAccountIds, ResolveSignalAccount, ResolveDefaultSignalAccountId |
| daemon_test.go | 5 | ClassifySignalCliLogLine（info/warn/error/heuristic/empty） |
| format_test.go | 12 | MarkdownToSignalText（inline styles/bold prefix/plain/empty/UTF-16/links），expandMarkdownLinks，utf16Len，MarkdownToSignalTextChunks |
| identity_test.go | 12 | ResolveSignalSender, FormatSignalSenderId, FormatSignalPairingIdLine, IsSignalSenderAllowed, IsSignalGroupAllowed, parseSignalAllowEntry |
| reaction_level_test.go | 7 | ResolveSignalReactionLevel（off/ack/minimal/extensive/default/unknown） |
| send_test.go | 9 | ParseTarget（recipient/group/username/prefix/empty），buildTargetParams，formatTextStyles，ResolveMaxBytes |
| send_reactions_test.go | 7 | normalizeSignalUUID, normalizeSignalRecipient, resolveReactionTargetAuthor, ResolveReactionTargets, IsSignalReactionMessage, ShouldEmitSignalReactionNotification, BuildSignalReactionSystemEventText |
| **合计** | **62** | |

---

## 五、编译验证

```
go build ./internal/channels/signal/   ✅ PASS
go build ./internal/channels/bridge/   ✅ PASS
go vet  ./internal/channels/signal/    ✅ PASS
go vet  ./internal/channels/bridge/    ✅ PASS
go test ./internal/channels/signal/    ✅ 62/62 PASS (0.010s)
```

---

## 六、修改文件总清单（15 个文件）

### 代码文件（8 个）

| 文件路径 | 操作摘要 |
|----------|----------|
| `internal/channels/signal/client.go` | 修改：randomRpcID UUID v4 |
| `internal/channels/signal/daemon.go` | 修改：CLI 参数 + --no-receive-stdout + 参数顺序 + ClassifySignalCliLogLine *SignalLogLevel |
| `internal/channels/signal/sse_reconnect.go` | 修改：重连计数器重置 |
| `internal/channels/signal/format.go` | 修改：tableMode + expandMarkdownLinks |
| `internal/channels/signal/send.go` | 修改：SendResult + outbound attachment + maxBytes + tableMode |
| `internal/channels/signal/monitor.go` | 修改：daemon 参数 + deliverReplies 分块 + 群组历史 + readReceiptsViaDaemon + 超时 clamp |
| `internal/channels/signal/event_handler.go` | 修改：防抖/白名单/命令门控/附件/打字/reaction/群组历史/readReceiptsViaDaemon |
| `internal/channels/bridge/signal_actions.go` | 重写：sendMessage + react 实际调用 |

### 测试文件（7 个，全部新增）

| 文件路径 | 用例数 |
|----------|--------|
| `internal/channels/signal/accounts_test.go` | 10 |
| `internal/channels/signal/daemon_test.go` | 5 |
| `internal/channels/signal/format_test.go` | 12 |
| `internal/channels/signal/identity_test.go` | 12 |
| `internal/channels/signal/reaction_level_test.go` | 7 |
| `internal/channels/signal/send_test.go` | 9 |
| `internal/channels/signal/send_reactions_test.go` | 7 |

---

## 七、文档链状态

| 文档 | 路径 | 状态 |
|------|------|------|
| 审计报告 | `goujia/shenji-signal-full-audit.md` | 已更新（含复核审计） |
| 文件映射表 | `goujia/signal-ts-go-mapping.md` | 已更新（15 对全部"完成"） |
| 待办延迟项 | `daibanyanchi.md` | 已清零 |
| 延迟项归档 | `daibanyanchi_guidang.md` | 已归档 DY-S01~S03 |
| 任务完成汇总 | `renwu/signal_task_completion_summary.md` | 本文档 |

---

## 八、结论

Signal 模块 TS→Go 迁移已 **100% 完成**：
- 14 个核心文件 + 1 个桥接文件全部通过交叉颗粒度复核审计
- 21 项功能修复（16 初轮 + 5 复核）全部实施
- 3 个延迟项全部清零
- 62 个单元测试全部通过
- 编译 + 静态分析全部通过
- 文档链完整更新，可以归档
