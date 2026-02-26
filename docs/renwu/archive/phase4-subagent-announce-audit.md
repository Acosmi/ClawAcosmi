# subagent-announce 模块隐藏依赖审计

**目标文件**: `src/agents/subagent-announce.ts` (573L)

## 依赖提取

- 显式依赖: 12 个模块
- 传递依赖(3层内): ~6 个
- 动态/条件依赖: 无
- 循环依赖: 无

## 隐藏依赖审计 (7 项)

| # | 类别 | 结果 | 说明 |
| --- | --- | --- | --- |
| 1 | npm 包黑盒行为 | ✅ 无 | 无第三方包 |
| 2 | 全局状态/单例 | ⚠️ | `subagent-announce-queue.ts` 模块级队列 Map。Go 方案: `sync.Mutex` + map |
| 3 | 事件总线/回调链 | ⚠️ | `waitForEmbeddedPiRunEnd` 使用 Promise 等待。Go 方案: channel + select + timeout |
| 4 | 环境变量依赖 | ✅ 无 | 不直接读取环境变量 |
| 5 | 文件系统约定 | ⚠️ | session store JSONL 文件路径拼接 (`storePath + sessionId + .jsonl`)。Go 方案: `filepath.Join` |
| 6 | 协议/消息格式约定 | ⚠️ | gateway RPC 调用协议 (`method: "agent"`, `method: "agent.wait"` 等)。Go 方案: 定义 `GatewayRPC` 接口，注入实现 |
| 7 | 错误处理约定 | ⚠️ | best-effort 错误吞没 (announce 失败不中断调用方)。Go 方案: 忠实复现，`log.Error` + 返回 false |

## 架构决策

`CallGateway` 和 `DeliveryContext` 在 Go 端**尚未实现**。采用 **依赖注入接口** 方案：

- 定义 `GatewayRPC` 接口
- 定义 `SessionStoreReader` 接口
- 定义 `EmbeddedRunTracker` 接口
- 所有自含逻辑（格式化、stats、system prompt）完整实现
