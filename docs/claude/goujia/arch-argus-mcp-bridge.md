> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 架构设计文档：Argus 视觉子智能体 MCP 桥接

- **日期**: 2026-02-25
- **状态**: 已实现
- **范围**: `backend/internal/mcpclient/` + `backend/internal/argus/` + 网关集成
- **关联**: 汇总 `shenji-argus-mcp-bridge-full-summary.md` / 审计 `audit-2026-02-25-argus-mcp-bridge.md`

---

## 一、系统全景

### 1.1 定位

Argus 桥接是 OpenAcosmi 网关的**可选子系统**，将独立的 Argus 视觉子智能体通过标准 MCP 协议接入主系统，使主智能体获得屏幕感知与物理操作能力。

### 1.2 系统拓扑

```
┌──────────────────── OpenAcosmi 系统 ────────────────────┐
│                                                          │
│  ┌─────────────┐    WebSocket     ┌──────────────────┐  │
│  │   前端 UI   │◄──────RPC───────►│   Go Gateway     │  │
│  │  (browser)  │                  │                  │  │
│  └─────────────┘                  │  ┌────────────┐  │  │
│                                   │  │MethodRegistry│ │  │
│                                   │  │  ├ chat.*   │  │  │
│                                   │  │  ├ config.* │  │  │
│                                   │  │  ├ skills.* │  │  │
│                                   │  │  ├ sandbox.*│  │  │
│                                   │  │  └ argus.*  │◄─┼──── 本次新增
│                                   │  └────────────┘  │  │
│                                   │        │         │  │
│                                   │  ┌─────▼──────┐  │  │
│                                   │  │ ArgusBridge │  │  │
│                                   │  │ (状态机)    │  │  │
│                                   │  └─────┬──────┘  │  │
│                                   └────────┼─────────┘  │
│                                            │             │
│                               stdin/stdout │ (JSON-RPC)  │
│                                            │             │
│                                   ┌────────▼──────────┐  │
│                                   │  Argus MCP Server │  │
│                                   │  (独立进程)       │  │
│                                   │                   │  │
│                                   │  Go + Rust FFI    │  │
│                                   │  ├ ScreenCapture  │  │
│                                   │  ├ InputControl   │  │
│                                   │  ├ VLM Router     │  │
│                                   │  └ ApprovalGate   │  │
│                                   └───────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### 1.3 设计约束

| 约束 | 理由 |
|------|------|
| 零修改 Argus 代码 | Argus 独立开发/版本周期，避免耦合 |
| 可选子系统 | 二进制不存在时不影响网关启动 |
| 进程隔离 | Argus 含 CGo/Rust FFI，崩溃不影响网关 |
| 标准协议 | MCP (Model Context Protocol) 2024-11-05，可与其他 MCP 客户端互操作 |

---

## 二、模块划分

### 2.1 包依赖图

```
gateway (网关主包)
  ├── mcpclient (MCP 协议层 — 通用，不绑定 Argus)
  │     ├── types.go     — JSON-RPC 2.0 + MCP 类型定义
  │     └── client.go    — stdio 管道客户端
  │
  └── argus (Argus 业务层 — 特定于 Argus 子智能体)
        ├── bridge.go            — 进程生命周期 + 状态机
        ├── skills.go            — MCP 工具 → 技能条目转换
        ├── codesign_darwin.go   — macOS 签名（build tag: darwin）
        └── codesign_other.go    — 非 macOS no-op
```

**关键边界**: `mcpclient` 是通用 MCP 客户端库，未来可复用于接入其他 MCP Server（如 Claude Desktop 工具、第三方 MCP 服务等）。`argus` 包才是 Argus 专属逻辑。

### 2.2 网关集成点

```
boot.go
  GatewayState
    └── argusBridge *argus.Bridge        ← 新增字段

server_methods.go
  GatewayMethodContext
    └── ArgusBridge *argus.Bridge        ← 新增字段
  AuthorizeGatewayMethod()
    └── argus.* prefix rule              ← 新增规则

server.go
  StartGatewayServer()
    ├── registry.RegisterAll(ArgusHandlers())        ← 静态方法
    └── RegisterArgusDynamicMethods(registry, bridge) ← 动态方法

ws_server.go
  methodCtx
    └── ArgusBridge: cfg.State.ArgusBridge()  ← 接入 DI

server_methods_skills.go
  handleSkillsStatus()
    └── append argus entries to skillEntries  ← 技能列表合并
```

---

## 三、数据流

### 3.1 工具调用流（正常路径）

```
前端 UI                   Go Gateway                  Argus MCP Server
   │                          │                              │
   │  ws: {method:"argus.     │                              │
   │       capture_screen",   │                              │
   │       params:{quality:   │                              │
   │       "vlm"}}            │                              │
   │─────────────────────────►│                              │
   │                          │  1. AuthorizeGatewayMethod   │
   │                          │     → check scopeWrite       │
   │                          │                              │
   │                          │  2. handler lookup           │
   │                          │     → dynamic argus.capture  │
   │                          │       _screen handler        │
   │                          │                              │
   │                          │  3. bridge.CallTool()        │
   │                          │     → mcpclient.CallTool()   │
   │                          │                              │
   │                          │  stdin: {"jsonrpc":"2.0",    │
   │                          │   "method":"tools/call",     │
   │                          │   "params":{"name":          │
   │                          │   "capture_screen",          │
   │                          │   "arguments":{"quality":    │
   │                          │   "vlm"}}}                   │
   │                          │─────────────────────────────►│
   │                          │                              │ 执行截屏
   │                          │  stdout: {"jsonrpc":"2.0",   │ + JPEG 编码
   │                          │   "result":{"content":[      │ + base64
   │                          │   {"type":"text","text":     │
   │                          │   "data:image/jpeg;base64,   │
   │                          │   ..."}]}}                   │
   │                          │◄─────────────────────────────│
   │                          │                              │
   │  ws: {ok:true,           │  4. 解析 MCP content        │
   │   payload:{tool:         │     → 构建网关响应           │
   │   "capture_screen",      │                              │
   │   content:[...]}}        │                              │
   │◄─────────────────────────│                              │
```

### 3.2 健康检查流

```
Gateway (healthLoop)              Argus MCP Server
    │                                   │
    │  每 30s                           │
    │  stdin: {"method":"ping"}         │
    │──────────────────────────────────►│
    │  stdout: {"result":{}}            │
    │◄──────────────────────────────────│
    │  → 更新 lastPing, lastRTT        │
    │  → 连续 3 次失败 → degraded      │
    │  → 恢复 → ready                  │
```

### 3.3 启动流（含 macOS 签名）

```
NewGatewayState()
    │
    ├── 1. resolveArgusBinaryPath()
    │       ├── $ARGUS_BINARY_PATH?       → 使用
    │       ├── FindAppBundleBinary()?     → 方案 A（.app bundle）
    │       ├── ~/.openacosmi/bin/?        → 裸二进制
    │       └── PATH 查找                  → 裸二进制
    │
    ├── 2. argus.IsAvailable(path)?
    │       └── false → 跳过，网关正常启动
    │
    ├── 3. EnsureCodeSigned(path)          ← 方案 B 兜底
    │       ├── .app 内? → 跳过
    │       ├── 已签名? → 跳过
    │       ├── 有 "Argus Dev" 证书?
    │       │     └── codesign --force -s "Argus Dev"
    │       │         --identifier "com.argus.sensory.mcp"
    │       └── 无证书 → warn 日志，继续启动
    │
    ├── 4. bridge.Start()
    │       ├── exec.Command(path, "-mcp")
    │       ├── MCP Initialize (5s 超时)
    │       ├── MCP tools/list
    │       └── state → ready
    │
    └── 5. 启动后台循环
            ├── healthLoop (每 30s ping)
            └── processMonitor (退出重启)
```

---

## 四、状态机详解

### 4.1 Bridge 状态

```
状态         含义                     转入条件                    允许操作
─────────────────────────────────────────────────────────────────────────
init         初始创建                 NewBridge()                 Start()
starting     正在启动子进程           Start() 调用                等待
ready        MCP 握手完成，工作正常   spawnAndHandshake 成功      CallTool/Stop
degraded     健康检查失败或进程重启中 ping 连续 3 次失败 /        CallTool(降级)/Stop
                                      进程退出待重启
stopped      已关闭                   Stop() / 重启超限           无（需重新创建）
```

### 4.2 状态转换矩阵

| 当前状态 | 事件 | 目标状态 |
|----------|------|----------|
| init | Start() 成功 | ready |
| init | Start() 失败 | stopped |
| ready | ping 连续 3 次失败 | degraded |
| ready | 进程退出 | degraded → 重启 |
| ready | Stop() | stopped |
| degraded | ping 恢复 | ready |
| degraded | 重启成功 | ready |
| degraded | 重启失败 × 5 | stopped |
| degraded | Stop() | stopped |

---

## 五、MCP 协议适配

### 5.1 协议参数

| 参数 | 值 | 匹配 |
|------|-----|------|
| 协议版本 | `2024-11-05` | Argus server.go |
| 传输方式 | stdin/stdout 行分隔 JSON | Argus server.go |
| 最大消息 | 10MB | Argus scanner buffer |
| 客户端名称 | `openacosmi-gateway` | - |
| 服务端名称 | `argus-sensory` | Argus server.go |

### 5.2 方法映射

| MCP 方法 | 网关调用点 | 时机 |
|----------|-----------|------|
| `initialize` | `bridge.Start()` → `client.Initialize()` | 启动时一次 |
| `notifications/initialized` | `client.Initialize()` 内部 | 握手后通知 |
| `tools/list` | `bridge.Start()` → `client.ListTools()` | 启动时一次 |
| `tools/call` | `bridge.CallTool()` → `client.CallTool()` | 每次工具调用 |
| `ping` | `bridge.healthLoop()` → `client.Ping()` | 每 30s |

### 5.3 工具清单（16 个，4 分类）

| 分类 | 工具 | 风险 | 网关方法名 |
|------|------|------|-----------|
| perception | capture_screen, describe_scene, locate_element, read_text, detect_dialog, watch_for_change | low | argus.capture_screen 等 |
| action | click, double_click, type_text, press_key, hotkey, scroll, mouse_position | low-medium | argus.click 等 |
| shell | run_shell | high | argus.run_shell |
| macos | macos_shortcut, open_url | medium | argus.macos_shortcut 等 |

---

## 六、安全模型

### 6.1 三层防护

```
层级 1: 网关权限                     层级 2: MCP 协议隔离          层级 3: Argus ApprovalGateway
┌──────────────────────┐           ┌────────────────────┐       ┌────────────────────────┐
│ AuthorizeGatewayMethod│           │  进程边界隔离      │       │ 工具级风险评估          │
│ ├ argus.status → read│           │  stdin/stdout 管道  │       │ ├ perception → 自动允许 │
│ └ argus.* → write    │           │  Argus 崩溃不影响   │       │ ├ action → 需确认       │
│                      │           │  网关                │       │ └ shell → 始终需确认    │
└──────────────────────┘           └────────────────────┘       └────────────────────────┘
```

### 6.2 攻击面分析

| 向量 | 防护 |
|------|------|
| 恶意工具参数 | 网关不校验参数语义（透传），Argus 内部 JSON Schema 验证 |
| Argus 进程逃逸 | 进程隔离，Bridge 仅持有 stdin/stdout pipe |
| MCP 消息注入 | Client 仅接受匹配 request ID 的响应 |
| 二进制替换 | 方案 A 使用签名 .app bundle，方案 B 用证书签名验证 |

### 6.3 macOS TCC 授权

| 权限 | Info.plist Key | 用途 |
|------|----------------|------|
| 辅助功能 | `NSAccessibilityUsageDescription` | UI 元素检测、键鼠控制 |
| 屏幕录制 | `NSScreenCaptureUsageDescription` | 截屏、视觉分析 |

**持久化机制**:
- `.app` bundle: TCC 按 `CFBundleIdentifier` (`com.argus.compound`) 追踪
- 裸二进制: TCC 按 code signing identifier (`com.argus.sensory.mcp`) 追踪
- 两者均使用 `Argus Dev` 持久化证书，重新编译后授权不丢失

---

## 七、可扩展性

### 7.1 新增 Argus 工具

**零网关修改**: Argus 侧新增工具 → tools/list 响应多一个条目 → 网关重启后 `RegisterArgusDynamicMethods` 自动注册对应 `argus.<new_tool>` 方法。

仅需更新 `skills.go` 中的 `toolCategoryMap` 和 `toolRiskMap`（否则默认 category=unknown, risk=medium）。

### 7.2 接入其他 MCP Server

`mcpclient` 包是通用的，接入新 MCP Server 仅需：
1. 创建类似 `argus/bridge.go` 的进程管理器
2. 调用 `mcpclient.NewClient(stdin, stdout)` → Initialize → ListTools
3. 在网关注册对应方法

### 7.3 Phase 2 路线

```
Phase 1 (当前)          Phase 2                    Phase 3
─────────────────       ──────────────────         ──────────────────
MCP 桥接基础            审批对接                    多智能体协调
├ 工具调用转发           ├ approval.resolve         ├ 任务分配
├ 健康检查               │ → ApprovalGateway        ├ 上下文共享
├ 技能列表合并           ├ EscalationManager 对接   ├ 工具链组合
└ macOS 签名             └ 前端审批 UI              └ 多 Argus 实例
```

---

## 八、故障处理

| 故障场景 | Bridge 行为 | 用户影响 |
|----------|------------|----------|
| Argus 二进制不存在 | 跳过初始化，网关正常启动 | argus.status 返回 `available:false` |
| MCP 握手超时 | Start() 返回错误，state=stopped | 同上 |
| Argus 进程崩溃 | 指数退避重启（最多 5 次） | 短暂不可用，自动恢复 |
| MCP ping 超时 | 3 次后标记 degraded | CallTool 仍可调用（降级模式） |
| 重启超限 | state=stopped | argus.* 方法返回 503 |
| codesign 失败 | warn 日志，继续启动 | 功能正常，但重新编译后需重新授权 |

---

## 九、文件索引

### 源文件

| 路径 | 行数 | 职责 |
|------|------|------|
| `backend/internal/mcpclient/types.go` | 107 | MCP 协议类型 |
| `backend/internal/mcpclient/client.go` | 257 | MCP stdio 客户端 |
| `backend/internal/argus/bridge.go` | 423 | 进程生命周期管理 |
| `backend/internal/argus/skills.go` | 145 | 工具→技能转换 |
| `backend/internal/argus/codesign_darwin.go` | ~130 | macOS 签名逻辑 |
| `backend/internal/argus/codesign_other.go` | 15 | 非 macOS no-op |
| `backend/internal/gateway/server_methods_argus.go` | ~105 | argus.* RPC 方法 |

### 修改文件

| 路径 | 修改行数 | 修改点 |
|------|----------|--------|
| `backend/internal/gateway/boot.go` | ~50 | argusBridge 字段 + 初始化 + 签名 |
| `backend/internal/gateway/server_methods.go` | ~15 | ArgusBridge DI + 权限规则 |
| `backend/internal/gateway/server.go` | ~6 | 方法注册 + 关闭逻辑 |
| `backend/internal/gateway/ws_server.go` | ~1 | ArgusBridge 接入 methodCtx |
| `backend/internal/gateway/server_methods_skills.go` | ~20 | 技能列表合并 |

### 测试文件

| 路径 | 测试数 |
|------|--------|
| `backend/internal/mcpclient/client_test.go` | 9 |
| `backend/internal/argus/bridge_test.go` | 14 |
| `backend/internal/argus/codesign_test.go` | 4 |
| `backend/internal/argus/integration_test.go` | 2 |

### 文档

| 路径 | 类型 |
|------|------|
| `docs/claude/tracking/impl-argus-mcp-bridge-2026-02-25.md` | 任务追踪（已归档） |
| `docs/claude/audit/audit-2026-02-25-argus-mcp-bridge.md` | 审计报告（已归档） |
| `docs/claude/goujia/shenji-argus-mcp-bridge-full-summary.md` | 完成汇总 |
| `docs/claude/goujia/arch-argus-mcp-bridge.md` | 本文：架构设计 |
