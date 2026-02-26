# 新窗口启动上下文（继续修复用）

> 生成日期：2026-02-19 17:20
> 用途：在新对话窗口粘贴此文件内容，即可无缝继续修复工作

---

## 项目概况

- **项目**：OpenAcosmi（TS→Go 迁移后端）
- **代码库**：`/Users/fushihua/Desktop/Claude-Acosmi/backend/`
- **Go 版本**：1.25.7
- **关键文档目录**：`docs/renwu/`

## 审计文档索引（完整路径）

所有文档位于 `docs/renwu/` 目录下：

| 文件 | 绝对路径 | 用途 |
|------|---------|------|
| 审计主索引 | `docs/renwu/global-audit-index.md` | 43 项 P0 差异总览 + 评级 |
| 修复跟踪 | `docs/renwu/fix-plan-master.md` | 5 阶段修复进度（核心文档，每次修复后必须更新） |
| 隐藏依赖 | `docs/renwu/hidden-deps-tracking.md` | 环境变量/格式约定/npm 包等价（修复后须更新状态） |
| 代码健康 | `docs/renwu/global-audit-health.md` | 7 高危 + 9 中危 Bug 清单 |
| W1 审计 | `docs/renwu/global-audit-W1.md` | gateway + security + config |
| W2 审计 | `docs/renwu/global-audit-W2.md` | agents 全模块 |
| W3 审计 | `docs/renwu/global-audit-W3.md` | channels 消息通道 |
| W4 审计 | `docs/renwu/global-audit-W4.md` | auto-reply + cron + daemon + hooks |
| W5 审计 | `docs/renwu/global-audit-W5.md` | infra + media + memory + cli + tui + browser |
| 复核报告 | `docs/renwu/audit-quality-review.md` | 本轮代码验证复核结果 |
| 本文件 | `docs/renwu/continue-fix-context.md` | 新窗口启动上下文 |
| TUI 专项审计 | `docs/renwu/global-audit-tui.md` | TUI 22 项差异 + 7 类隐藏依赖 |
| TUI 依赖审计 | `docs/renwu/global-audit-tui-deps.md` | 14 个外部依赖颗粒度验证 |
| TUI 项目方案 | `docs/renwu/phase5-tui-project.md` | TUI 独立实施方案（15 文件 / 7 窗口） |

## 当前进度

- ✅ 阶段一（编译阻断）：4/4 完成
- ✅ 阶段二（安全+认证，6 窗口 A~F）：30/30 完成
- ✅ 阶段三（消息管线，6 窗口 G~L）：**28/28 完成（100%）**
  - ✅ G（Outbound 投递链）、H（Gateway 协议）、J（通道规范化）
  - ✅ I（LINE 通道 6 项）— 待 Phase 13 执行，参见 deferred-items
  - ✅ K（Browser CDP 4 项）— K-3 Playwright 延迟至阶段五
  - ✅ L（Context+错误处理 5 项）— 2026-02-19 代码审计确认全部完成
- 🔄 阶段四（功能补全）：18/18 完成（W-M/W-N/W-O/W-P/W-Q/W-R 全部完成）
- 🔄 阶段五：方案研究完成（见 `phase5-research-plan.md`），推荐顺序：5.3→5.5→5.4→5.1→5.6→5.2
  - ✅ **5.3 TUI W1（核心骨架）**：`model.go`(532L) + `gateway_ws.go`(713L) 编译通过 — 2026-02-20

## ✅ 优先修复清单状态（2026-02-19 代码审计确认全部完成）

> **审计结论**：以下全部 17 项在代码中已完成，仅 fix-plan-master.md 复选框滞后。已同步更新。

### 最高优先级（编译/运行时阻断）— ✅ 全部完成

```
1. H4 — SQLite 驱动缺失 ✅
   manager.go:13 已 import _ "modernc.org/sqlite"，L65 用 "sqlite"

2. H5 — WebSocket 客户端并发写竞态 ✅
   ws.go:46 有 writeMu sync.Mutex，pingLoop L238 加 writeMu.Lock()

3. H2 — CDP relay goroutine 永久阻塞 ✅
   extension_relay.go:154-157 共享 done channel + sync.Once
```

### 高优先级（阶段三未完成项）— ✅ 全部完成

```
4. K-2 — CDP Dial HandshakeTimeout ✅
   extension_relay.go:137 已有 HandshakeTimeout: 10*time.Second

5. E-1 — context-pruning firstUserIndex 保护 ✅
   context_pruning.go:452-465 完整 firstUserIdx 逻辑

6. L-3 — runner context 传播 ✅
   run.go:34 接收 ctx context.Context

7. L-5 — gmail watcher goroutine 泄漏 ✅
   watcher.go:216-229 Signal → 3s 超时 → Kill
```

### 中优先级（LINE 通道 + 阶段四）

```
8~13. I-1 ~ I-6（LINE 通道 6 项）✅ 全部完成
   I-1~I-5: Phase 13 G-W2 已完成（16 个 Go 文件已存在）
   I-6: 2026-02-19 修复 webhook 错误码 403→401

14. W-M — Setup/OAuth 流程
15. W-P — config/sessions 高级功能 ✅ (RFC 7396 + session-key + Zod 验证)
16. W-Q — providers（GitHub Copilot/Qwen OAuth）✅ (设备码 OAuth + token 交换 + 模型定义 + Qwen refresh)
17. W-R — sandbox browser + container prune ✅ (CDP wait + noVNC + bridge 复用 + idle prune + LANG env default)
```

## 编译验证命令

```bash
cd /Users/fushihua/Desktop/Claude-Acosmi/backend
go build ./...
go vet ./...
# Linux 交叉编译
GOOS=linux go build ./internal/daemon/...
```

## 每次修复后必须更新的文档

> **⚠️ 关键：每完成一项修复，必须同步更新以下文档，否则进度追踪会再次脱节。**

1. **`fix-plan-master.md`**：将对应任务的 `[ ]` 改为 `[✅]`，标注完成日期
2. **`hidden-deps-tracking.md`**：如果修复了环境变量/格式约定等隐藏依赖，将对应 `[ ]` 改为 `[✅]`
3. **`global-audit-health.md`**：如果修复了 H/M 编号的 Bug，在对应行标注 ✅
4. **`audit-quality-review.md`**：更新对应阶段的验证状态
5. **`global-audit-index.md`**：如有模块评级变化，更新模块评级表

## 注意事项

1. **阶段三已 100% 完成**：2026-02-19 代码审计确认全部完成，fix-plan-master.md 复选框已同步
2. **hidden-deps 环境变量**：34 个 TS 环境变量中 33 个 Go 端未读取，可作为批量任务处理
3. **每次修复后**运行 `go build ./...` 验证编译
4. **遵循 /safe 工作流**：逐步执行，避免上下文过载

---

*生成自：audit-quality-review.md 复核结果*
