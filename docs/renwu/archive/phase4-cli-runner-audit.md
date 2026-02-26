# cli-runner 模块隐藏依赖审计

**目标文件**: `src/agents/cli-runner.ts` (363L) + `src/agents/cli-runner/helpers.ts` (549L) + `src/agents/cli-backends.ts` (158L)

## 依赖提取

- 显式依赖: 14 个模块
- 传递依赖(3层内): ~8 个
- 动态/条件依赖: 无
- 循环依赖: 无

## 隐藏依赖审计 (7 项)

| # | 类别 | 结果 | 说明 |
|---|------|------|------|
| 1 | npm 包黑盒行为 | ✅ 无 | 不依赖第三方 npm 包（仅 `node:child_process`） |
| 2 | 全局状态/单例 | ⚠️ | `CLI_RUN_QUEUE` — 模块级 `Map<string, Promise>` 用于序列化同一 backend 的 CLI 调用。Go 方案: 使用 `sync.Mutex` map 实现等价串行化 |
| 3 | 事件总线/回调链 | ✅ 无 | 纯同步/await 模式，无 EventEmitter |
| 4 | 环境变量依赖 | ⚠️ | `OPENACOSMI_CLAUDE_CLI_LOG_OUTPUT` 控制是否打印 CLI stdout/stderr。`backend.env` 和 `backend.clearEnv` 控制子进程环境。Go 方案: `os.Getenv` + `exec.Cmd.Env` |
| 5 | 文件系统约定 | ⚠️ | 临时图片写入 `os.tmpdir()` + cleanup 回调。Go 方案: `os.MkdirTemp` + `defer os.RemoveAll` |
| 6 | 协议/消息格式约定 | ⚠️ | CLI 输出解析依赖 JSON/JSONL 格式。`sessionIdFields` 指定 session ID 提取字段名。`resumeArgs` 中 `{sessionId}` 占位符替换。Go 方案: `encoding/json` + `strings.ReplaceAll` |
| 7 | 错误处理约定 | ⚠️ | 非零退出码 → `FailoverError`；stderr/stdout 错误文本 → `classifyFailoverReason()` → failover 状态码。Go 方案: 复用 `models.ClassifyFailoverReason` + `models.FailoverError` |

## 重构确认

- [x] 所有 ⚠️ 隐藏依赖已有明确 Go 等价方案
- [x] 依赖图中所有下游模块已有 Go 实现:
  - `models.NormalizeProviderId` ✅
  - `models.FailoverError` + `ClassifyFailoverReason` ✅
  - `helpers.IsFailoverErrorMessage` ✅
  - `workspace.ResolveRunWorkspaceDir` ✅
  - `types.CliBackendConfig` ✅
- [x] 无上游破坏性变更（新增文件）
