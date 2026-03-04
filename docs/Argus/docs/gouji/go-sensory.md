# go-sensory 架构文档

## 概述

go-sensory 是 Argus-compound 的**统一 Go 后端**，承载所有核心功能：屏幕采集、输入控制、VLM 模型路由、ReAct Agent 循环、时序分析、帧处理管线和指标监控。运行于 `:8090` 端口。

## 技术栈

- **语言**：Go 1.25
- **网络**：标准库 `net/http` + `golang.org/x/net/websocket`
- **屏幕采集**：ScreenCaptureKit（SCK）/ CoreGraphics（CG）via CGO
- **IPC**：共享内存（POSIX SHM）
- **VLM 代理**：HTTP 反向代理，支持 OpenAI 兼容和 Gemini 原生 API
- **向量存储**：ChromaDB HTTP REST（无 SDK 依赖）

## 目录结构

```text
go-sensory/
├── cmd/server/main.go              # 程序入口（CLI flags + 初始化 + 信号处理）
├── internal/
│   ├── agent/                      # ReAct Agent 循环 + UI 解析
│   │   ├── react_loop.go           # Observe→Think→Act→Verify 循环
│   │   ├── react_helpers.go        # 参数提取、JPEG 编码、markdown 解析
│   │   ├── ui_parser.go            # VLM-based UI 元素检测 + SoM 标注
│   │   └── types.go                # Action/Observation/TaskResult 等类型
│   ├── analysis/                   # 时序分析
│   │   └── temporal.go             # 多帧时序推理（TemporalAnalyzer）
│   ├── api/                        # HTTP/WebSocket API 层
│   │   ├── ws_server.go            # Server 结构体 + 路由注册 + REST 端点
│   │   ├── ws_frames.go            # WebSocket 帧流推送
│   │   ├── ws_control.go           # WebSocket 控制命令（输入注入）
│   │   ├── hub.go                  # WebSocket 客户端连接管理
│   │   ├── action_handler.go       # 输入动作 REST 端点
│   │   ├── analysis_handler.go     # 时序分析 REST 端点 + 健康检查
│   │   ├── pipeline_handler.go     # 管线统计 REST 端点
│   │   ├── windows_handler.go      # 窗口排除管理 REST 端点
│   │   ├── encoding.go             # 帧压缩、JPEG 编码
│   │   ├── types.go                # 请求/响应数据结构
│   │   └── dashboard.go            # 嵌入式 HTML 仪表盘
│   ├── capture/                    # 屏幕采集引擎
│   │   ├── capture.go              # Capturer 接口 + 工厂 + 配置
│   │   ├── darwin.go               # CoreGraphics 后端（CGO）
│   │   └── darwin_sck.go           # ScreenCaptureKit 后端（CGO）
│   ├── input/                      # 输入虚拟化（键盘/鼠标注入）
│   │   ├── input.go                # InputController 接口 + Key 类型
│   │   ├── darwin.go               # CGEvent 实现（CGO）
│   │   └── guardrails.go           # 安全护栏（坐标边界/速率限制）
│   ├── ipc/                        # 共享内存 IPC
│   │   └── shm_writer.go           # POSIX SHM 帧写入器（CGO）
│   ├── memory/                     # 向量存储
│   │   └── vector_store.go         # ChromaDB HTTP REST 客户端
│   ├── metrics/                    # 指标监控
│   │   └── metrics.go              # Prometheus 文本格式输出（零依赖）
│   ├── pipeline/                   # 帧处理管线
│   │   ├── pipeline.go             # 管线调度（Submit/Start/Stop）
│   │   ├── keyframe.go             # 关键帧提取（差分/SSIM）
│   │   └── pii_filter.go           # PII 过滤（人脸/文字脱敏）
│   ├── skills/                     # 技能执行器
│   │   └── executor.go             # 多步骤技能执行（click/type/hotkey/scroll/ground）
│   └── vlm/                        # VLM 模型路由模块
│       ├── provider.go             # Provider 接口 + ChatRequest/Response 类型
│       ├── config.go               # 配置加载（JSON 文件 / 环境变量）
│       ├── openai_provider.go      # OpenAI 兼容 HTTP 客户端（支持 SSE）
│       ├── gemini_provider.go      # Gemini 原生 API 适配器
│       ├── router.go               # HTTP 路由器 + Provider 管理
│       └── health.go               # 后台健康检查器
└── go.mod
```

## 核心组件

### Agent 模块 (`internal/agent/`)

- **ReActLoop**：实现 Observe→Think→Act→Verify 智能循环，全部调用均为进程内零网络开销
- **UIParser**：基于 VLM 的 UI 元素检测，支持 SoM（Set-of-Mark）标注和视觉定位
- **类型系统**：Action、Observation、TaskResult 等结构体定义

### 分析模块 (`internal/analysis/`)

- **TemporalAnalyzer**：多帧时序推理，接收 base64 图像序列 + 时间戳，通过 VLM 输出结构化分析

### API 层 (`internal/api/`)

- **Server**：统一 HTTP 服务器，集成 WebSocket 帧流、控制命令和 REST 端点
- **Hub**：WebSocket 客户端广播管理
- **ActionHandler**：单动作/批量动作/鼠标位置查询 REST 端点
- **AnalysisHandler**：时序分析 REST 端点 + 系统健康检查
- **PipelineHandler**：管线统计和关键帧查询端点

### 采集引擎 (`internal/capture/`)

- **Capturer 接口**：定义 Start/Stop/LatestFrame/Subscribe/DisplayInfo/ListWindows/SetExcludedWindows/ExcludeApp 方法
- **SCKCapturer**：ScreenCaptureKit 后端（macOS 13+，推荐），支持窗口排除和刷新率检测
- **CGCapturer**：CoreGraphics 后端（兼容旧系统），窗口排除方法返回 stub 错误

### 帧处理管线 (`internal/pipeline/`)

- **Pipeline**：异步帧处理调度，支持背压（通道满时丢弃旧帧）
- **KeyframeExtractor**：基于像素差分的关键帧提取
- **PIIFilter**：PII 脱敏过滤（人脸区域模糊化）

### VLM 模块 (`internal/vlm/`)

- **Provider 接口**：定义 ChatCompletion 和 ChatCompletionStream 方法
- **OpenAIProvider**：适配所有 OpenAI 兼容 API（Ollama/Qwen/Claude 等），支持 SSE 流式
- **GeminiProvider**：适配 Google Gemini 原生 API，内部做格式转换
- **Router**：管理多 Provider 实例，处理 HTTP 路由分发 + CRUD 配置

### 其他模块

- **InputController** (`input/`)：键盘/鼠标事件注入，via CGEvent CGO
- **ActionGuardrails** (`input/`)：安全边界检查（坐标范围/速率限制/屏幕区域保护）
- **ShmWriter** (`ipc/`)：POSIX 共享内存帧写入，用于进程间通信
- **KeyframeVectorStore** (`memory/`)：ChromaDB 语义检索，直接 HTTP REST 调用
- **ArgusMetrics** (`metrics/`)：Prometheus 文本格式指标，lock-free 计数器

## 数据流

```text
屏幕 → Capturer (SCK/CG) → Pipeline → KeyframeExtractor → VectorStore
          ↓                     ↓
     Hub (WebSocket)       PIIFilter
          ↓
     ShmWriter (IPC)

用户请求 → API Server → ReActLoop → VLM Router → Provider → LLM
                ↓            ↓
          InputController  UIParser → SoM 标注
```

## 接口清单

| 端点 | 方法 | 功能 |
| --- | --- | --- |
| `/` | GET | 嵌入式仪表盘 |
| `/ws/frames` | WS | 实时帧流（JSON + base64） |
| `/ws/frames/binary` | WS | 实时帧流（二进制 JPEG） |
| `/ws/control` | WS | 输入控制命令 |
| `/api/status` | GET | 系统状态 |
| `/api/display` | GET | 显示器信息 |
| `/api/capture/once` | GET | 单帧截图 |
| `/api/action` | POST | 输入动作（click/type/hotkey/scroll） |
| `/api/action/mouse` | GET | 鼠标位置查询 |
| `/api/action/batch` | POST | 批量动作执行 |
| `/api/analyze` | POST | 时序分析（多帧序列） |
| `/api/health` | GET | 系统健康检查 |
| `/api/pipeline/stats` | GET | 管线处理统计 |
| `/api/pipeline/keyframes` | GET | 关键帧列表查询 |
| `/v1/chat/completions` | POST | OpenAI 兼容聊天补全 |
| `/api/vlm/health` | GET | VLM 模块健康检查 |
| `/api/config/providers` | GET/POST | Provider 列表/创建 |
| `/api/config/providers/{name}` | PUT/DELETE/PATCH | Provider 更新/删除/激活 |
| `/api/windows` | GET | 列出所有可见窗口 |
| `/api/windows/exclude` | GET/POST/DELETE | 查询/设置/清除排除窗口列表 |
| `/api/windows/exclude/app` | POST | 按 Bundle ID 排除应用全部窗口 |
| `/metrics` | GET | Prometheus 指标 |

## 配置说明

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VLM_API_BASE` | (空) | OpenAI 兼容 API 端点 |
| `VLM_API_KEY` | (空) | API 密钥 |
| `VLM_MODEL` | (空) | 默认模型名 |
| `GEMINI_API_KEY` | (空) | Gemini API 密钥 |
| `GEMINI_MODEL` | `gemini-2.0-flash-exp` | Gemini 模型名 |
| `GO_SENSORY_PORT` | `8090` | 服务端口 |
| `GO_SENSORY_FPS` | `0`（自动） | 采集帧率（0 = 显示器刷新率 / 6） |
| `GO_SENSORY_SHM` | `true` | 共享内存 IPC 开关 |
| `ARGUS_API_KEY` | (空) | API 认证密钥 |
| `CHROMADB_URL` | `http://localhost:8000` | ChromaDB 服务地址 |

CLI flags: `-port`, `-fps`, `-backend`, `-shm`, `-open-browser`, `-vlm-config`

## 变更记录

| 日期 | 变更内容 | 操作人 |
| --- | --- | --- |
| 2026-02-11 | 新增窗口排除管理 API + FPS 自动检测 + VLM 配置始终初始化 | AI |
| 2026-02-11 | 全面重写：补充 agent/analysis/memory/metrics/pipeline/skills 六大模块文档 | AI |
| 2026-02-10 | P5: Python 全量迁移至 Go，删除 py-vision 和 openclaw-skills | AI |
| 2026-02-10 | P3: 新增 Action REST 端点 + 安全护栏迁移 | AI |
| 2026-02-10 | P1: 新增 `internal/vlm/` 模块，实现 VLM 代理和模型路由 | AI |
