# memory 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W5 (或后续分配窗口)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 27 | 22 | 81.5% |
| 总行数 | 7001 | 5147 | 73.5% |

## 逐文件对照

| 状态 | TS 文件 | Go 文件 | 备注 |
|------|---------|---------|------|
| 🔄 REFACTORED | `sqlite.ts` | `manager.go` | TS 的基础 sqlite 封装在 Go 中合并入了 manager 结构和 database 的基础管理。 |
| 🔄 REFACTORED | `openai-batch.ts` | `batch_openai.go` | 重命名并优化为了 `batch_openai.go`，包含相同的批量请求和状态处理。 |
| 🔄 REFACTORED | `node-llama.ts` | `embeddings_local.go` | 弃用 node-llama，使用 Ollama API 进行替代来实现本地 embedding。 |
| 🔄 REFACTORED | `session-files.ts` | `sync_sessions.go` | 提取 session 数据并合并到了同步 session 逻辑中。 |
| ✅ FULL | `sqlite-vec.ts` | `sqlite_vec.go` | SQLite Vec 扩展加载器，已完整实现。 |
| ✅ FULL | `provider-key.ts` | `provider_key.go` | 提供者 key 验证提取逻辑。 |
| ✅ FULL | `status-format.ts` | `status.go` | 状态及格式化常量等价实现。 |
| ✅ FULL | `types.ts` | `types.go` | 核心类型定义，完全覆盖。 |
| ✅ FULL | `embeddings-openai.ts` | `embeddings_openai.go` | OpenAI Embedding 实现完整。 |
| ✅ FULL | `memory-schema.ts` | `schema.go` | 数据库表结构和索引构建逻辑对齐。 |
| ✅ FULL | `embeddings-voyage.ts` | `embeddings_voyage.go` | Voyage AI Embedding 实现。 |
| ✅ FULL | `sync-memory-files.ts` | `watcher.go` / `sync_sessions.go` | 异步监控与文件同步功能转移至 watcher。 |
| ✅ FULL | `hybrid.ts` | `hybrid.go` | 关键词(BM25/FTS5)与向量相似度排名的混合搜索算法逻辑等价。 |
| ✅ FULL | `sync-session-files.ts` | `sync_sessions.go` | 批量 sessions 文件同步处理模块。 |
| ✅ FULL | `embeddings-gemini.ts` | `embeddings_gemini.go` | Gemini Embedding 支持。 |
| ✅ FULL | `manager-search.ts` | `search.go` | 主 Manager 中抽离出的搜索路由函数。 |
| ✅ FULL | `search-manager.ts` | `search_manager.go` | 统一抽象的独立 search-manager 对象。 |
| ✅ FULL | `embeddings.ts` | `embeddings.go` | Embedding Provider 主注册与工厂实现。 |
| ✅ FULL | `backend-config.ts` | `config.go` | 后端服务针对 Memory 模块专属 Config 参数。 |
| ✅ FULL | `internal.ts` | `internal.go` | 内部通用文件路径等帮助函数抽离。 |
| ✅ FULL | `batch-voyage.ts` | `batch_voyage.go` | Voyage 的批量接口逻辑支持。 |
| ✅ FULL | `batch-openai.ts` | `batch_openai.go` | OpenAI 批量端点接入。 |
| ✅ FULL | `batch-gemini.ts` | `batch_gemini.go` | Gemini 的并行限流批量操作。 |
| ✅ FULL | `qmd-manager.ts` | `qmd_manager.go` | QMD 文件结构的管理机制完整对齐。 |
| ✅ FULL | `manager.ts` | `manager.go` | 主入口类，生命周期与状态查询基本实现等价功能。 |
| ⚠️ PARTIAL | `headers-fingerprint.ts` | 无明显对应 | 提取请求头用于指纹的细颗粒度函数，Go可能不需要或放在 `internal` 中隐式处理。 |
| ⚠️ PARTIAL | `manager-cache-key.ts` | 无明显对应 | 缓存 Key 计算等辅助，可能已内联至 `manager.go` 或 `search.go`。 |

## 隐藏依赖审计

| # | 类别 | 结果 | 应对说明 |
|---|------|------|----------|
| 1 | npm包黑盒行为 | ⚠️ 存在 | TS 的 `sqlite-vec` 等底层 C 插件依赖，Go 中使用了 `modernc.org/sqlite` 及自定义扩展加载 `LoadSqliteVecExtension`。 |
| 2 | 全局状态/单例 | ✅ 正常 | `new Map()` 用于批处理状态在请求或临时生命周期中的追踪，Go 已使用 `map[string]struct` 等无锁或带锁的方式正确复现。 |
| 3 | 事件总线/回调链 | ✅ 无 | 无显式 EventEmitter 滥用。 |
| 4 | 环境变量依赖 | ⚠️ 存在 | `OPENACOSMI_DEBUG_MEMORY_EMBEDDINGS` 和 `OPENACOSMI_STATE_DIR`，在 Go 端应转为配置传入或提取至 `env_vars.go`。 |
| 5 | 文件系统约定 | ⚠️ 存在 | QMD Manager 使用 `os.homedir` 解析状态目录，并在大量 sync 流程需要读写目录，Go 端使用 `os` 库已完整重构相应的 `sync_*` 及 `watcher` 等策略，路径处理一致。 |
| 6 | 协议/消息格式 | ✅ 无 | 批量 embedding 上传时的 REST 与 jsonl 格式在对应的 `batch_*.go` 中已保证一致。 |
| 7 | 错误处理约定 | ✅ 正常 | 常见基于 promise 和 catch 的错误吞咽，在 Go 的 `manager.go` 中转换为带返回值的日志记录 `m.logger.Warn/Debug`，不会发生未捕获 Panic。 |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| 1 | 代码结构 | `sqlite.ts` | `manager.go` | TypeScript 存在单独的 sqlite 工具类实例化封装，Go 端直接在 `manager.go` 的方法中调用 `sql.Open` 隐式完成。 | P3 | 遵循现有 Go 实现即可。 |
| 2 | 后端模型 | `node-llama.ts` | `embeddings_local.go` | TS 使用基于 node 的 llama.cpp 绑定，Go 端转移到了更轻量友好的 Ollama API。 | P2 | 接口层面保留 `OllamaEmbed` 即可，用户接受。 |
| 3 | 缓存机制 | `manager-cache-key.ts` | 未直接体现 | 对应函数可能在 Go 端被直接作为结构体内联字符串拼接或简化掉了。 | P3 | 确认查询缓存是否在业务侧或上层生效即可。 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 1 项
- P3 差异: 2 项
- 模块审计评级: A

此模块的代码重构质量优异，大量的向量批处理和同步搜索逻辑在代码级已经完成了良好的对齐与转化。
