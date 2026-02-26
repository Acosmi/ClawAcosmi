---
document_type: Tracking
status: Archived
created: 2026-02-25
last_updated: 2026-02-25
audit_report: docs/claude/audit/audit-2026-02-25-argus-mcp-bridge.md
skill5_verified: true
---

# Argus 视觉子智能体 MCP 桥接接入

> 将独立 Argus 视觉理解/执行子智能体（Go+Rust）通过 MCP 协议接入主系统网关。

## 在线验证摘要（Skill 5）

### MCP 协议
- **Query**: MCP protocol 2024-11-05 stdio transport
- **Source**: Argus/go-sensory/internal/mcp/server.go（一手源码）
- **Key finding**: 行分隔 JSON-RPC 2.0，10MB buffer，支持 initialize/tools-list/tools-call/ping
- **Verified date**: 2026-02-25

### Argus 工具清单
- **Source**: Argus MCP Server tools/list 实际响应
- **Key finding**: 16 个工具，4 分类（perception/action/shell/macos），3 风险等级
- **Verified date**: 2026-02-25

## 实施步骤

### 新建文件

- [x] `backend/internal/mcpclient/types.go` — MCP 协议类型定义
- [x] `backend/internal/mcpclient/client.go` — MCP stdio 客户端
- [x] `backend/internal/argus/bridge.go` — Argus 进程生命周期管理
- [x] `backend/internal/argus/skills.go` — MCP 工具到技能条目转换
- [x] `backend/internal/gateway/server_methods_argus.go` — argus.* RPC 方法

### 修改文件

- [x] `backend/internal/gateway/boot.go` — 添加 argusBridge 字段和初始化
- [x] `backend/internal/gateway/server_methods.go` — 添加 argus.* 权限规则
- [x] `backend/internal/gateway/server.go` — 注册 argus 方法 + 关闭逻辑
- [x] `backend/internal/gateway/ws_server.go` — 接入 ArgusBridge 到 methodCtx
- [x] `backend/internal/gateway/server_methods_skills.go` — 追加 Argus 技能条目

### 测试

- [x] `backend/internal/mcpclient/client_test.go` — 9 个测试用例，mock pipe 覆盖全公共 API
- [x] `backend/internal/argus/bridge_test.go` — 14 个测试用例，覆盖 IsAvailable/状态机/skills 转换
- [x] `backend/internal/argus/integration_test.go` — 端到端集成测试（需 ARGUS_BINARY_PATH）
- [x] 编译验证 `go build ./...` 零错误

### 验证结果

| 验证项 | 结果 |
|---|---|
| `go build ./...` | 零错误 |
| mcpclient 单元测试（9 用例） | 全部 PASS |
| argus bridge 单元测试（14 用例） | 全部 PASS |
| 集成测试 — MCP 握手 | PASS（协议版本 2024-11-05） |
| 集成测试 — 工具发现 | PASS（16 个工具） |
| 集成测试 — capture_screen 调用 | PASS |
| 集成测试 — mouse_position 调用 | PASS |
| 集成测试 — 技能条目映射 | PASS（4 分类全覆盖） |
| 集成测试 — 优雅关闭 | PASS |

## 架构决策

| 决策 | 理由 |
|---|---|
| MCP Client 而非直接 RPC | 复用 Argus 已有 MCP Server，零修改 Argus 代码 |
| 可选子系统模式 | 复用沙箱模式：二进制不存在时跳过，不影响网关启动 |
| 动态方法注册 | 从 tools/list 自动生成 argus.* RPC，新增工具无需修改网关 |
| 指数退避重启 | 1s→2s→4s...→60s cap，最多 5 次，平衡恢复速度与系统负载 |

## 待办 / Phase 2

- [ ] `argus.approval.resolve` 审批决策中继实现
- [ ] Argus ApprovalGateway 与网关 EscalationManager 对接
- [ ] 前端 Argus 工具面板 UI
