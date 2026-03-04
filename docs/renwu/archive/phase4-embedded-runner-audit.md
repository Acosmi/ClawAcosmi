# runEmbeddedPiAgent 隐藏依赖审计

**目标**: `src/agents/pi-embedded-runner/run.ts` (867L)

## 隐藏依赖 (7 项)

| # | 类别 | 结果 | Go 方案 |
| --- | --- | --- | --- |
| 1 | npm 包黑盒 | ✅ 无 | — |
| 2 | 全局状态 | ⚠️ session/global lanes (并发控制) | `sync.Mutex` map |
| 3 | 事件总线 | ⚠️ `onPartialReply`/`onToolResult` 回调链 | Go callback func types |
| 4 | 环境变量 | ⚠️ `process.cwd()` 保存/恢复 | Go 不需要 (exec.Cmd.Dir) |
| 5 | 文件系统 | ⚠️ `mkdir -p workspace`, models.json 写入 | `os.MkdirAll` |
| 6 | 协议约定 | ⚠️ PI SDK session file JSONL 格式 | 接口注入 |
| 7 | 错误约定 | ⚠️ 多层 failover: auth→profile→thinking→model | 复用 `models.FailoverError` |

## 架构决策

867L 主函数拆分为 4 个 Go 文件：

1. `runner/run.go` — 主函数 + 类型定义 (已有 stub)
2. `runner/run_helpers.go` — 工具函数 (usage accumulator, refusal scrub)
3. `runner/run_attempt.go` — `runEmbeddedAttempt` 接口 + stub
4. `runner/run_auth.go` — auth profile 管理函数

不存在的重依赖采用接口注入：

- `AttemptRunner` 接口 (替代 runEmbeddedAttempt)
- `ModelResolver` 接口 (替代 resolveModel)
- `CompactionRunner` 接口 (替代 compactEmbeddedPiSessionDirect)
- `AuthProfileStore` 接口 (替代 auth store 操作)
