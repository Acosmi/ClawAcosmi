# Web Console 功能增强 — 新窗口上下文

> 创建: 2026-02-16 | 优先级: **P0 — 生产就绪**

---

## 系统定位

Argus-Sensory 定位为 **感知+执行服务**，为 IDE 及桌面智能体提供"眼睛"和"手"的能力，通过 MCP 协议集成到外部智能体中。**本系统不独立运行智能体逻辑**。

> [!IMPORTANT]
> Memory/Browser (原 Batch D4) 已决议移至独立项目开发，生产时再集成。
> MCP 工具层扩展暂挂起，优先完成 Web Console 生产化。

---

## 当前状态

### 后端 API 层 (`go-sensory/internal/api/`)

| 文件 | 端点 | 状态 |
|------|------|------|
| `ws_server.go` | 主服务入口 + CORS + 路由注册 | ✅ |
| `ws_frames.go` | `/ws/frames`, `/ws/frames/binary` — 帧推流 | ✅ |
| `ws_control.go` | `/ws/control` — WebSocket 命令通道 | ✅ |
| `hub.go` | WebSocket 客户端管理 | ✅ |
| `types.go` | Frame/Command/Status 类型定义 | ✅ |
| `action_handler.go` | `/api/action`, `/api/action/batch`, `/api/action/mouse` | ✅ |
| `pipeline_handler.go` | `/api/pipeline/stats`, `/api/pipeline/keyframes` | ✅ |
| `analysis_handler.go` | `/api/analyze`, `/api/health` | ✅ |
| `monitor_handler.go` | `/api/monitor`, `/api/monitor/observations` | ✅ |
| `windows_handler.go` | `/api/sensory/windows` 及排除管理 | ✅ |
| `dashboard.go` | 内嵌测试看板 | ✅ |

### 前端组件 (`web-console/src/`)

| 组件 | 功能 | 状态 | 待办 |
|------|------|------|------|
| `page.tsx` | 主编排页：实时画布 + 指标卡 + HITL 聊天 + 快捷操作 | ✅ 功能完整 | — |
| `SettingsPage.tsx` | VLM 配置 + 捕获参数 + 安全设置 | ✅ 功能完整 | — |
| `WindowManager.tsx` | 窗口排除列表管理 (嵌入 Settings) | ✅ 功能完整 | — |
| `TimelinePage.tsx` | 关键帧时间线 + 搜索 + 交互时间轴 | ⚠️ 有 UI 无数据源 | 对接 `/api/pipeline/keyframes` |
| **`TasksPage.tsx`** | 任务列表（状态 + 步骤 + 耗时） | ❌ 纯前端 state | **需对接后端 + 实时推送** |
| **`AnomalyPage.tsx`** | 异常告警列表 | ❌ 空数据 | **需对接 Monitor + 实时推送** |
| `LangSwitch.tsx` | 中英语言切换 | ✅ | — |

### 已有的后端数据源（可直接对接）

| 前端需求 | 后端端点 | 数据结构 |
|----------|----------|----------|
| 任务列表 | ⚠️ **无专用端点** | 当前任务在前端 `useState` 中临时管理 |
| 异常告警 | `GET /api/monitor/observations` | `{timestamp, description, severity, frame_no, ...}` |
| 关键帧 | `GET /api/pipeline/keyframes?n=N` | `{count, keyframes: [{frame_no, timestamp, reason, ...}]}` |
| 管线统计 | `GET /api/pipeline/stats` | `{processed_frames, keyframe_count, ...}` |
| 系统健康 | `GET /api/health` | `{status, vlm_ready, vlm_provider, pipeline_stats}` |

---

## 目标：Tasks + Anomaly 页面生产化

### TasksPage 增强

**问题**: 当前任务只存在于前端 `page.tsx` 的 `useState` 中，刷新即丢失，且无后端持久化。

**改造方向**:

1. **后端**: 新增 Task Manager + REST API (`/api/tasks`)
   - `GET /api/tasks` — 获取任务列表
   - `POST /api/tasks` — 创建任务
   - `PUT /api/tasks/{id}` — 更新状态
   - WebSocket 事件推送任务状态变更
2. **前端**:
   - 从 REST 拉取 + WebSocket 实时更新
   - 任务详情展开（步骤列表）
   - 任务操作（取消/重试）
   - 持久化到后端（刷新不丢失）

### AnomalyPage 增强

**问题**: `anomalies` 数组始终为空，未对接后端 Monitor 的 observations。

**改造方向**:

1. **前端对接**: 调用 `GET /api/monitor/observations` 获取数据
2. **数据映射**: Monitor observation → Anomaly 类型转换
3. **实时推送**: WebSocket 订阅新异常通知
4. **交互增强**:
   - 确认/静默/归档操作
   - 点击跳转到对应时间点（联动 Timeline）
   - 严重度过滤/排序

### TimelinePage 增强

**问题**: `keyframes` 数组始终为空，没有调用后端 API。

**改造方向**:

1. 对接 `GET /api/pipeline/keyframes?n=50`
2. 定时轮询或 WebSocket 推送新关键帧
3. 缩略图 base64 展示（后端已支持 JPEG 编码）

---

## 关键文件索引

### 后端 (Go)

| 文件 | 说明 |
|------|------|
| `internal/api/ws_server.go` | 主路由，`Server` 结构体 |
| `internal/api/types.go` | API 类型定义 |
| `internal/api/pipeline_handler.go` | 关键帧 + 管线统计 |
| `internal/api/monitor_handler.go` | Monitor observations |
| `internal/api/analysis_handler.go` | 异常分析 + 健康检查 |
| `internal/api/hub.go` | WebSocket 客户端广播 hub |
| `internal/pipeline/pipeline.go` | 管线核心逻辑 |
| `internal/pipeline/keyframe.go` | 关键帧提取器 |
| `internal/pipeline/monitor.go` | VLM 持续监控器 |

### 前端 (React/Next.js)

| 文件 | 说明 |
|------|------|
| `src/app/page.tsx` | 主页面编排 (296 行) |
| `src/components/TasksPage.tsx` | 任务页 (55 行) — **重点改造** |
| `src/components/AnomalyPage.tsx` | 异常页 (54 行) — **重点改造** |
| `src/components/TimelinePage.tsx` | 时间线页 (90 行) — 需对接 |
| `src/types/index.ts` | 共享类型定义 |
| `src/styles/subpages.css` | 子页面样式 (14.7KB) |
| `src/hooks/useBinaryFrameStream.ts` | 二进制帧流 Hook |

---

## 构建命令

```bash
# 后端
cd go-sensory && go build ./... && go vet ./...
cd go-sensory && go test -v ./internal/api/...

# 前端
cd web-console && npm run dev    # 开发服务器
cd web-console && npm run build  # 生产构建
```

## 端口

| 服务 | 端口 | 说明 |
|------|------|------|
| go-sensory API | 8090 | HTTP + WebSocket |
| web-console dev | 3000 | Next.js 开发服务器 |

---

## 使用方式

在新窗口中告诉 AI：

> 执行 Web Console 增强，上下文在 `docs/renwu/bootstrap-webconsole.md`
