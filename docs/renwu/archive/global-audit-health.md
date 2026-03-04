# 代码健康度审计报告

> 审计日期：2026-02-19
> 审计范围：`backend/internal/` + `backend/bridge/` + `backend/cmd/`
> Go 文件总数：1021（含测试），生产代码 goroutine 启动点：58 处

---

## ✅ 编译版本说明（已更正）

### H7：~~go.mod 声明 `go 1.25.7`~~ — **审计误判，已撤销**

**更正说明**：Go 1.25.7 是 2025 年下半年发布的正式稳定版本，当前最新为 Go 1.26.0。审计时因知识截止日期（2025-08）而误判此版本不存在。**`go 1.25.7` 版本号完全正确，无需修改。**

### H4：SQLite 驱动缺失 — memory 模块运行时必然 panic

**文件**：`backend/internal/memory/manager.go:63`
```go
sql.Open("sqlite3", dbPath)  // go.mod 中无 sqlite3 驱动声明
```
- 运行时 panic：`unknown driver "sqlite3"`
- 需添加 `github.com/mattn/go-sqlite3` 或 `modernc.org/sqlite`

---

## 1. Goroutine 泄漏风险

| 文件 | 行号 | 问题描述 | 风险级别 |
|------|------|---------|---------|
| `internal/infra/control_ui_assets.go` | 267-282 | `runWithTimeout` 超时后 Kill 进程，子 goroutine 继续写 `output` 变量，主 goroutine 已返回并读该变量，**数据竞争** | 高 |
| `internal/browser/extension_relay.go` | 153,178 | CDP relay 双向 goroutine，一端出错直接 `return` 而不 `close(done)`，对端 goroutine 永久阻塞在 `ReadMessage` | 高 |
| `internal/hooks/gmail/watcher.go` | 220 | `Stop()` 超时 3s 后不再跟踪等待进程的 goroutine，进程不退出时该 goroutine 泄漏 | 中 |
| `internal/agents/bash/exec_tool.go` | 562, 962 | 审批请求 goroutine 使用 `context.Background()`，外部无法取消 | 中 |
| `internal/gateway/ws_server.go` | 312 | ping 保活 goroutine 仅靠 `WriteMessage` 失败退出，无显式 stop channel | 低 |

**核心竞态代码（`control_ui_assets.go:267`）：**
```go
func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, error) {
    done := make(chan error, 1)
    var output []byte   // 主/子 goroutine 共享，无锁
    go func() {
        output, runErr = cmd.CombinedOutput() // 子 goroutine 写
        done <- runErr
    }()
    select {
    case <-time.After(timeout):
        _ = cmd.Process.Kill()
        return string(output), ... // 超时时主 goroutine 读 — 数据竞争！
    }
}
```

---

## 2. 资源未关闭

| 文件 | 行号 | 问题描述 | 风险级别 |
|------|------|---------|---------|
| `internal/infra/ssh_tunnel.go` | 202 | `stderrPipe, _ := cmd.StderrPipe()` 错误忽略，pipe 为 nil 时 L212 的 `stderrPipe.Read(buf)` 触发 **nil panic** | 高 |
| `internal/browser/extension_relay.go` | 135-136 | CDP Dial 使用 `websocket.Dialer{}` 无 HandshakeTimeout，可能永久阻塞 | 中 |
| `internal/infra/cost/provider_fetch.go` | 84 | `fetchJSON` 使用 `http.DefaultClient`（无超时），连接池在慢响应下耗尽 | 中 |

---

## 3. 错误处理问题

统计：`err != nil` 检查 **2083 处**（整体健康），`_ =` 错误忽略约 40 处。

**高危忽略项：**

| 文件 | 行号 | 忽略的错误 | 风险 |
|------|------|----------|------|
| `internal/infra/node_pairing_ops.go` | 34,76,98,136 | `_ = savePairingState(s)` 配对状态持久化失败静默忽略，重启后配对丢失 | 高 |
| `internal/infra/approval_forwarder_ops.go` | 162 | `_ = f.cfg.DeliverFunc(t, text)` 审批消息投递失败无日志 | 高 |
| `internal/infra/cost/session_cost.go` | 130 | `_ = json.Unmarshal(tc, &toolCall)` 成本统计数据静默丢失 | 中 |
| `internal/infra/discovery.go` | 195,248 | `beacon.Port, _ = strconv.Atoi(m[2])` 端口解析失败时 Port=0 | 中 |

**`context.TODO()` 使用：**
- `internal/autoreply/reply/followup_runner.go:99` — `RunReplyAgent(context.TODO(), ...)` 在 agent 执行期间无法取消

---

## 4. 竞态条件风险

| 文件 | 行号 | 问题描述 | 风险 |
|------|------|---------|------|
| `internal/agents/datetime/datetime.go` | 34,62-70 | `cachedTimeFormat` 包级全局变量，`ResolveUserTimeFormat` 无锁读写（nil 检查+赋值非原子） | 中 |
| `internal/autoreply/commands_registry.go` | 336,341-366 | `cachedTextAliasMap` 全局变量无锁，并发调用 `GetTextAliasMap()` 存在双重写入竞态 | 中 |
| `internal/gateway/ws.go` | 130-137, 233 | `Send()` 释放 mu 后调用 `conn.WriteMessage`，同时 `pingLoop` 调用 `conn.WriteControl`，**gorilla/websocket 不允许并发写** | **高** |

---

## 5. Nil 解引用风险

| 文件 | 行号 | 问题描述 | 风险 |
|------|------|---------|------|
| `internal/infra/ssh_tunnel.go` | 202,212 | `stderrPipe` 可能为 nil，直接 `.Read()` 触发 panic | 高 |
| `internal/agents/bash/exec_process.go` | 313-318 | `getStdoutPipe`/`getStderrPipe` **恒返回 nil**，exec 工具进程 stdout/stderr 完全无法捕获（功能性 Bug） | 高 |
| `internal/gateway/server.go` | 375 | `os.Exit(1)` 跳过所有 defer，资源未清理 | 中 |

---

## 6. 超时/上下文管理（系统性问题）

**根本问题**：`internal/agents/tools/` 所有工具的 `Execute` 函数签名为 `func(toolCallID string, args map[string]any)` — **没有 context 参数**，无法传播取消信号。

| 文件 | 行号 | 问题 | 风险 |
|------|------|------|------|
| `internal/agents/runner/run.go` | 42 | `ctx := context.Background()` 整个 agent runner 无法外部取消 | 高 |
| `internal/agents/runner/run.go` | 412 | 压缩操作 `context.Background()` 无超时 | 中 |
| `internal/agents/tools/sessions.go` | 60,104,147,185,238,251 | 6 处 session 操作均用 `context.Background()` | 中 |
| `internal/agents/tools/image_tool.go` | 126,177 | 图片生成/编辑用 `context.Background()` | 中 |
| `internal/agents/tools/tts_tool.go` | 62 | TTS 合成无法取消 | 中 |

---

## 7. WebSocket 协议兼容性

**服务端（ws_server.go）：**
- 帧格式、握手流程、协议版本协商与 TS 版一致 ✅
- 心跳：服务端每 30s 发 PingMessage，pong 收到后重置 90s 读超时 ✅
- ❌ **Origin 检查未生效**：`CheckBrowserOrigin()` 函数已实现但从未被调用，upgrader 使用 `CheckOrigin: func(r *http.Request) bool { return true }`，任意来源可连接

**客户端（ws.go）：**
- 断线重连：指数退避正确实现 ✅
- ❌ **并发写 Bug**（H5）：`Send()` 与 `pingLoop` 同时写连接，违反 gorilla/websocket 规范

---

## 8. 配置格式兼容性

- JSON 字段名使用 camelCase，与 TS 版一致 ✅
- 必填字段通过 `go-playground/validator` 验证 ✅
- 配置支持 JSON5（通过 `tailscale/hujson`） ✅

---

## 9. TODO/FIXME 清单

| 文件 | 行号 | 内容 | 影响范围 |
|------|------|------|---------|
| `internal/agents/tools/registry.go` | 158-202 | 8 处 TODO — sessions/browser/memory/message/nodes/tts/web_fetch/image 工具未实现 | **所有 agent 工具** |
| `internal/agents/tools/image_tool.go` | 229,253 | 图片缩放/格式转换未实现 | image_tool |
| `internal/gateway/server_methods_skills.go` | 180 | `skills.install` 返回 not-implemented | skills 安装 |
| `internal/gateway/server_methods_nodes.go` | 414 | 节点命令策略未从 config 读取 | 节点安全 |
| `internal/gateway/server_methods_browser.go` | 227,335 | 浏览器模式/media store 未集成 | browser |
| `internal/autoreply/reply/followup_runner.go` | 99 | 使用 Stub 桩实现 | followup runner |
| `internal/channels/line/bot_message_context.go` | 63 | LINE 渠道未完整实现 | LINE 渠道 |
| `internal/infra/bonjour.go` | 177 | Bonjour watchdog 为空占位 | mDNS 健康 |

---

## 10. 依赖分析

**v0.x.x 不稳定版本：**

| 依赖 | 版本 | 风险 |
|------|------|------|
| `github.com/bwmarrin/discordgo` | v0.29.0 | Discord 渠道核心 |
| `github.com/pdfcpu/pdfcpu` | v0.11.1 | PDF 处理 |
| `github.com/slack-go/slack` | v0.17.3 | Slack 渠道核心 |

**伪版本（pseudo-version）：**
- `github.com/tailscale/hujson v0.0.0-20250605163823-992244df8c5a` — 依赖特定 commit，上游删除风险
- `github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f` — 2021 年 commit，4 年无更新

**致命缺失：**
- SQLite 驱动未在 go.mod 声明，memory 模块运行时必然 panic

---

## 高危 Bug 汇总（7 项）

| # | 位置 | 问题 | 修复优先级 |
|---|------|------|-----------|
| ~~H7~~ | ~~`backend/go.mod:3`~~ | ~~`go 1.25.7` 不存在~~ → **误判已撤销，版本正确** | ✅ 无需修复 |
| **H4** | `memory/manager.go:63` | SQLite 驱动缺失，运行时 panic | 🔴 立即 |
| **H5** | `gateway/ws.go:130,233` | WebSocket 客户端并发写竞态 | 🔴 立即 |
| **H3** | `infra/ssh_tunnel.go:202` | `stderrPipe` nil 导致 panic | 🟠 高 |
| **H1** | `infra/control_ui_assets.go:267` | `runWithTimeout` 数据竞争 | 🟠 高 |
| **H6** | `agents/bash/exec_process.go:313` | exec 进程输出完全丢失（恒返回 nil） | 🟠 高 |
| **H2** | `browser/extension_relay.go:153,178` | CDP relay goroutine 永久阻塞 | 🟠 高 |

## 中危 Bug（9 项）

| # | 描述 |
|---|------|
| M1 | `node_pairing_ops.go` 配对状态静默丢失 |
| M2 | `approval_forwarder_ops.go` 审批消息投递失败无日志 |
| M3 | `agents/runner/run.go:42` agent runner 无法外部取消 |
| M4 | 工具层系统性缺失 context 传播（tools/ 全部 Execute 函数） |
| M5 | `followup_runner.go:99` `context.TODO()` 在长时 agent 操作 |
| M6 | `datetime.go:34,62` `cachedTimeFormat` 并发竞态 |
| M7 | `commands_registry.go:336` `cachedTextAliasMap` 并发竞态 |
| M8 | `ws_server.go` WebSocket Origin 检查实现存在但未生效 |
| M9 | `gmail/watcher.go:220` 清理 goroutine 泄漏 |

---

## 代码健康总评

| 维度 | 评级 |
|------|------|
| 编译可用性 | **C**（SQLite 驱动缺失；go 1.25.7 版本误判已撤销） |
| 并发安全 | **D**（WS 客户端并发写、全局变量竞态） |
| 错误处理 | **C**（覆盖率尚可，关键路径有忽略） |
| 上下文管理 | **D**（工具层系统性缺失 context） |
| 资源管理 | **C**（主要资源有 defer，goroutine 部分失控） |
| 功能完整性 | **D**（多处 TODO 桩、exec 输出完全失效） |
| 依赖管理 | **D**（虚构 Go 版本、缺失 SQLite 驱动） |
| **综合评级** | **D** |

**最优先修复顺序：**
H7(go.mod) → H4(SQLite 驱动) → H5(WS 并发写) → H3(stderrPipe nil) → H1(数据竞争) → H6(exec 输出丢失) → H2(CDP goroutine 泄漏)
