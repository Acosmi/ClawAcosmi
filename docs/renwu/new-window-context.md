# 新窗口上下文 — Go Gateway 端到端修复 + 权限系统

> 创建时间: 2026-02-23 02:00  
> 来源会话: 模型选择修复 + 权限系统设计

---

## 一、本次会话完成的 7 个核心修复

所有修复已编译通过（`go build` + `go vet`），DeepSeek 对话功能正常工作。

| # | 问题 | 根因 | 修复文件 | 关键行号 |
|---|------|------|---------|---------|
| 1 | 向导配置从不持久化 | `gateway.bind: "loopback"` 不在验证合法列表 → `WriteConfigFile` 静默失败 | `backend/internal/config/validator.go` | L253 |
| 2 | 模型选择忽略配置 | `autoDetectProvider()` 只查环境变量 | `backend/internal/autoreply/reply/model_fallback_executor.go` | L37-58 |
| 3 | API Key 不从配置读取 | `resolveAPIKey()` 只查环境变量 | `backend/internal/agents/runner/attempt_runner.go` | L229-244 |
| 4 | 向导保存模型无 provider 前缀 | 保存 `deepseek-chat` 而非 `deepseek/deepseek-chat` | `backend/internal/gateway/wizard_onboarding.go` | L385-393 |
| 5 | UI 转圈永不停止 | `runId` 不匹配：UI 用 UUID (`idempotencyKey`)，服务器用 `run_<timestamp>` | `backend/internal/gateway/server_methods_chat.go` | L226-230 |
| 6 | DeepSeek 工具调用报错 | `openaiMessage.Content` 的 `omitempty` 省略空 content 字段 | `backend/internal/agents/llmclient/openai.go` | L90-95, L116-183 |
| 7 | 配置路径混乱 | `~/.openclaw` 旧目录未迁移到 `~/.openacosmi` | 文件系统操作（已完成） |

### 数据目录迁移详情

```
~/.openclaw/openclaw.json → ~/.openacosmi/openacosmi.json  (已复制+重命名)
~/.openclaw → ~/.openclaw.migrated.bak  (已备份)
配置文件: workspace 路径已修正为 ~/.openacosmi/workspace
配置文件: gateway.bind 已修正为 "localhost"
配置文件: model.primary 已修正为 "deepseek/deepseek-chat"
```

---

## 二、当前智能体权限问题（待修复 P0）

### 问题

智能体只有只读权限，无法执行 bash 和写文件。

### 根因

`backend/internal/agents/runner/attempt_runner.go` **L172-175**：

```go
output, toolErr := ExecuteToolCall(ctx, tc.Name, tc.Input, ToolExecParams{
    WorkspaceDir: params.WorkspaceDir,
    TimeoutMs:    params.TimeoutMs,
    // AllowWrite 和 AllowExec 都没有设置！Go 零值 = false
})
```

`backend/internal/agents/runner/tool_executor.go` **L27-29**：

```go
type ToolExecParams struct {
    WorkspaceDir string
    TimeoutMs    int64
    AllowWrite   bool // 未被设置，默认 false
    AllowExec    bool // 未被设置，默认 false
    AllowNetwork bool // 预留
}
```

### P0 修复（最小改动）

将 `attempt_runner.go` L172 改为：

```go
output, toolErr := ExecuteToolCall(ctx, tc.Name, tc.Input, ToolExecParams{
    WorkspaceDir: params.WorkspaceDir,
    TimeoutMs:    params.TimeoutMs,
    AllowWrite:   resolveAllowWrite(r.Config),  // 从配置读取
    AllowExec:    resolveAllowExec(r.Config),   // 从配置读取
})
```

需要新增 `resolveAllowWrite()` / `resolveAllowExec()` 函数，读取 `config.Tools.Exec.Security`：

- `"full"` → AllowWrite=true, AllowExec=true (对应 L2)
- `"sandbox"` → AllowWrite=false, AllowExec=true (对应 L1)  
- `"off"` 或默认 → AllowWrite=false, AllowExec=false (对应 L0)

当前用户配置文件 `~/.openacosmi/openacosmi.json` 中 `tools.exec.security = "full"`。

### 配置类型路径

需要查找 `OpenAcosmiConfig.Tools.Exec.Security` 的类型定义：

```
backend/internal/config/types/ 目录（或 schema.go 中）
搜索: ToolsConfig / ExecConfig / Security
```

---

## 三、权限系统设计（已批准）

完整设计文档在：`~/.gemini/antigravity/brain/b614bf6f-764a-461e-8107-7a8401bccd1b/implementation_plan.md`

### 核心设计

- **3 级权限**: L0 只读 → L1 执行(沙盒) → L2 完全
- **4 种授权模式**: 手动设置 / 预设命令规则 / 智能体即时授权 / 任务级自动授权
- **自动降权**: 任务完成后自动恢复 L0
- **永久授权**: 需二次确认 + 风险提示

### 实施分期

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0 | `AllowWrite`/`AllowExec` 接入 `tools.exec.security` 配置 | **✅ 完成** |
| P1 | UI 安全设置页 + 永久授权开关 | 待规划 |
| P2 | 智能体即时授权 + UI 弹窗 + 自动降权 | 待规划 |
| P3 | Allow/Ask/Deny 命令规则引擎 | 待规划 |
| P4 | 远程聊天审批（飞书/微信卡片） | 待规划 |
| P5 | 任务级预设权限 | 待规划 |

---

## 四、沙箱组件（已从 Chat 项目复制）

路径: `backend/internal/sandbox/`（已复制到本项目，勿访问原项目）

| 文件 | 大小 | 用途 | 复用阶段 |
|------|------|------|---------|
| `docker_runner.go` | 6.6KB | Docker 安全沙箱 (`--read-only` + `--no-new-privileges` + `--network=none`) | L1 |
| `sandbox_worker.go` | 13.8KB | Worker Pool (Go channel 调度) | L1 |
| `container_pool.go` | 12KB | 容器池管理 | L1 |
| `ws_progress.go` | 4.8KB | WebSocket 进度推送 | L1 |
| `wasm_runner.go` | 4KB | Rust FFI Wasm 执行桥 | 未来 |
| `sandbox_test.go` | 5KB | 测试 | L1 |
| `container_pool_test.go` | 6.6KB | 测试 | L1 |

架构参考文档: `Acosmi/nexus-v4/docs/jiagou/sandbox-architecture.md`（仅供参考，不要修改该项目任何文件）

---

## 五、其他延迟项

完整列表在 `docs/renwu/deferred-items.md`，本次新增：

- **GW-WIZARD-D1**: Google OAuth 模式（向导仅支持 API Key）
- **GW-WIZARD-D2**: 简化向导缺少后续阶段（技能、频道等）
- **GW-LLM-D1**: 其他 provider 消息格式兼容性验证
- **GW-UI-D4**: 已连接实例列表始终为空

---

## 六、运行方式

```bash
# Gateway
cd /Users/fushihua/Desktop/Claude-Acosmi && pnpm gateway:go

# UI
cd /Users/fushihua/Desktop/Claude-Acosmi && pnpm ui:dev

# Dashboard URL
http://localhost:19001/?token=77023f6768acf269733f4b5236ca7d0acdc8379f5d44cac9c06cf2097f752118
```
