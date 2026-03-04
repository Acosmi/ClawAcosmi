---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/memory (23 files + 3 subdirs, ~4000+ LOC)
verdict: Pass with Notes
---

# 审计报告: memory — 记忆/向量搜索模块

## 范围

- **目录**: `backend/internal/memory/`
- **文件数**: 23 顶层文件 + 3 子目录 (go-api/166, uhms/23, local-proxy/15)
- **核心文件**: `manager.go`(570L), `embeddings.go`(254L), `search.go`(291L), `hybrid.go`, `config.go`, `schema.go`, `types.go`

## 审计发现

### [PASS] 安全: 路径遍历防护 (manager.go)

- **位置**: `manager.go:232-240`
- **分析**: `ReadFile` 方法在读取文件前做路径安全检查：`filepath.Abs` 解析绝对路径后检查是否以工作区目录为前缀（`strings.HasPrefix(abs, wsAbs)`）。有效防止 `../../../etc/passwd` 类路径遍历攻击。
- **风险**: None

### [PASS] 资源安全: Manager 生命周期管理 (manager.go)

- **位置**: `manager.go:20-108, 554-569`
- **分析**: `Manager` 使用 `sync.RWMutex` 保护所有操作。`Close()` 设置 `closed` 标志、停止 watcher、关闭 DB。所有公开方法在入口检查 `closed` 状态避免操作已关闭资源。
- **风险**: None

### [PASS] 正确性: 混合搜索实现 (manager.go + search.go)

- **位置**: `manager.go:112-219`
- **分析**: `Search` 实现 4 步混合搜索:
  1. 向量搜索（optional，需 provider）
  2. FTS5 关键词搜索
  3. `MergeHybridResults` 合并（向量权重 0.7 / 关键词权重 0.3）
  4. 分数过滤 + 截断
  
  向量搜索失败时优雅降级为纯关键词搜索。Over-fetch 3x 后合并保证结果质量。
- **风险**: None

### [PASS] 正确性: 向量搜索双模式 (search.go)

- **位置**: `search.go:65-169`
- **分析**: `SearchVector` 根据 sqlite-vec 扩展可用性自动选择:
  - Native: 使用 `vec_distance_cosine` SQL 函数（高性能）
  - Fallback: 全量加载 chunks 后在内存中计算余弦相似度（无需扩展）
  
  Fallback 模式性能较差但保证功能可用。
- **风险**: None

### [PASS] 正确性: Embedding 向量归一化 (embeddings.go)

- **位置**: `embeddings.go:62-84`
- **分析**: `SanitizeAndNormalizeEmbedding` 处理 NaN（`v != v` 技巧）后执行 L2 归一化。幅度过小（<1e-10）时不归一化，避免除零。
- **风险**: None

### [PASS] 正确性: SQLite WAL 模式 (manager.go)

- **位置**: `manager.go:72-75`
- **分析**: 启用 WAL（Write-Ahead Logging）模式提升并发读取性能。失败不阻塞（warn 日志）。
- **风险**: None

### [WARN] 正确性: 同步期间使用写锁阻塞读取 (manager.go)

- **位置**: `manager.go:296-297`
- **分析**: `Sync` 方法持有 `mu.Lock()`（写锁）整个同步过程。对于大型工作区，同步可能耗时较长（文件扫描+embedding生成+DB写入），期间所有 `Search`/`ReadFile` 调用被阻塞。
- **风险**: Medium
- **建议**: 考虑仅在 DB 写入阶段持写锁，文件扫描和embedding生成阶段使用无锁操作。

### [WARN] 正确性: Embedding HTTP请求使用 DefaultClient (embeddings.go)

- **位置**: `embeddings.go:248`
- **分析**: `embeddingHTTPPost` 使用 `http.DefaultClient` 发送 embedding 请求。DefaultClient 无超时设置，无连接池限制，可能导致资源泄漏。
- **风险**: Medium
- **建议**: 使用带超时配置的自定义 `http.Client`。

### [PASS] 正确性: 事务写入 (manager.go)

- **位置**: `manager.go:401-465`
- **分析**: 同步过程使用 `BeginTx` 开启事务，先删除旧 chunks 再插入新数据，保证原子性。事务失败时跳过该文件继续处理其他文件。
- **风险**: None

## 总结

- **总发现**: 9 (7 PASS, 2 WARN, 0 FAIL)
- **阻断问题**: 无
- **建议**:
  1. 同步期间锁粒度优化，避免长时间阻塞读取 (Medium)
  2. Embedding HTTP 请求应使用带超时的自定义 Client (Medium)
- **结论**: **通过（附注释）** — 混合搜索实现完整（向量+FTS5），路径遍历防护到位。主要改进空间在锁粒度和 HTTP Client 配置。
