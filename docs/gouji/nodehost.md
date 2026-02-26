# Node Host 远程节点执行模块

> 最后更新：2026-02-26 | 代码级审计确认 | 13 源文件
> **TS 原版**：`src/node-host/config.ts` (73L) + `src/node-host/runner.ts` (1,309L)
> **重构完成**：Phase 12 W1 + Phase 13 D-W0 (2026-02-18)

## 模块职责

Node Host 是一个 **WS 客户端进程**，运行在远程机器上，通过 WebSocket 连接到 Gateway，接收命令执行请求并返回结果。

```
远程机器                               本地 Gateway
┌──────────────┐                    ┌─────────────┐
│ NodeHostSvc  │───WS connect──→   │  WS Server  │
│              │                    │             │
│ HandleInvoke │◀─ node.invoke.req─│  UI Client  │
│    │         │                    │             │
│    ├─ run    │─ node.invoke.res─→│             │
│    ├─ which  │─ node.event     ─→│             │
│    ├─ exec   │                    │             │
│    └─ proxy  │                    │             │
└──────────────┘                    └─────────────┘
```

## 文件结构

| 文件 | 行数 | 职责 |
|------|------|------|
| `config.go` | ~115 | node.json 配置管理（nodeId/token/gateway） |
| `types.go` | ~125 | 类型定义 + 常量（OutputCap=200KB, 请求/响应结构） |
| `sanitize.go` | ~110 | 环境变量消毒（PATH 仅追加 + 黑名单过滤） |
| `skill_bins.go` | ~60 | TTL 90s 缓存 skills.bins 查询 |
| `exec.go` | ~160 | os/exec 命令执行 + which 查找 + capBuffer |
| `invoke.go` | ~170 | 载荷解析 + 结果构建 + 安全级别解析 |
| `runner.go` | ~355 | NodeHostService + HandleInvoke 命令分派 |
| `browser_proxy.go` | ~270 | browser.proxy 代理处理（profile 白名单/文件收集） |
| `allowlist_types.go` | ~45 | 白名单评估类型定义 |
| `allowlist_parse.go` | ~490 | Shell 分词/管道/chain 解析 + glob→regexp + platform 分派 |
| `allowlist_eval.go` | ~360 | 评估/审批/记录函数（含 platform 参数） |
| `allowlist_win_parser.go` | ~110 | Windows Shell 分词（`tokenizeWindowsSegment` 等 4 函数） |
| `allowlist_resolve.go` | ~270 | `ResolveExecApprovalsFromFile` + `RequestExecApprovalViaSocket` + `MinSecurity`/`MaxAsk` |
| `nodehost_test.go` | ~365 | 单元测试覆盖所有模块 |

## 核心接口

```go
type NodeHostService struct {
    config      *Config
    skillBins   *SkillBinsCache
    sendRequest func(method string, params interface{}) error
    requestFunc RequestFunc  // 请求-响应回调
    browserProxy       BrowserProxyHandler  // 可选注入
    browserProxyConfig BrowserProxyConfig
}

// HandleInvoke 分派: system.run / system.which / execApprovals / browser.proxy
func (s *NodeHostService) HandleInvoke(payload interface{})
```

## 支持的命令

| 命令 | 功能 |
|------|------|
| `system.run` | 执行 shell 命令（超时/输出截断/事件通知） |
| `system.which` | 在 PATH 中查找可执行文件 |
| `system.execApprovals.get` | 获取执行审批配置（OCC 快照） |
| `system.execApprovals.set` | 更新执行审批配置（baseHash 乐观并发） |
| `browser.proxy` | 浏览器代理（profile 白名单 + 文件收集） |

## 安全机制

- **环境变量消毒**：`NODE_OPTIONS`/`PYTHONHOME`/`DYLD_*`/`LD_*` 等危险变量被阻止
- **PATH 策略**：仅允许在原始 PATH 后追加，禁止替换
- **输出上限**：`capBuffer` 限制 stdout/stderr 各 200KB
- **屏幕录制拒绝**：`needsScreenRecording=true` 直接返回 UNAVAILABLE

## 与 Gateway 的关系

- Gateway `node.*` stubs（`server_methods_stubs.go`）为 **UI 客户端** 调用的管理接口
- `nodehost` 包实现的是 **node 端 WS 客户端** 逻辑
- 两者职责不同：gateway stubs 保留直到 node registry 完整实现

## 依赖

- `internal/config` — `ResolveStateDir()` 解析 `~/.openacosmi/` 路径
- `internal/infra` — `ExecApprovals` 读写 + 快照 + 乐观并发控制
- `github.com/google/uuid` — nodeId 生成

## 待完善（低优先级遗留）

> 所有原计划遗留项已在 Phase 13 D-W0 全量补全。

- ~~`requestJSON` stub~~ ✅ 已完成（RequestFunc 注入）
- ~~`browser.proxy` 命令~~ ✅ 已完成（BrowserProxyHandler 接口注入）
- ~~完整 allowlist 评估流程~~ ✅ 已完成（19 函数移植）
- ~~Windows Shell 分词~~ ✅ 已完成（`allowlist_win_parser.go` — 4 函数）
- ~~`ResolveExecApprovalsFromFile()`~~ ✅ 已完成（三层合并：defaults → wildcard → agent）
- ~~`RequestExecApprovalViaSocket()`~~ ✅ 已完成（Unix socket IPC + 换行分隔 JSON）
- `detectMacApp()` — **TS 源码中不存在**（原任务文档误引用，已确认）
