# 全局审计报告 — Ollama 模块

## 概览

| 维度 | TS | Go (`backend/internal/agents/llmclient/ollama.go`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 0 (依赖第三方库) | 1 | 本地 LLM 支持 |
| 总行数 | 0 | 173 | 100% 剥离第三方依赖 |

### 架构演进

在原版的 TypeScript 项目中，对于本地大模型推理引擎 **Ollama** 的支持并不在核心源码中，而是通过拉取依赖 `npm install @mariozechner/pi-ai` 来实现的。这意味着 Acosmi 在使用本地推理时，对协议的控制力较弱，且调试流式日志较为困难。

在 Go 重构中，`llmclient` 下全新手写了 `ollama.go` (173 行)：

1. **原生 NDJSON 解析 (`ollamaStreamChat`)**：Go 版不依赖任何第三方胖客户端 (如 `sashabaranov/go-openai` 中对 ollama 的 hack)，而是直接构造 JSON HTTP 请求，并利用 `bufio.Scanner` 对 Ollama Server 返回的 `/api/chat` 流式 NDJSON 进行了精准而高效的逐行 (Line-by-line) 解析。
2. **Context Timeout 适配**：由于 Ollama 是本地推理，往往冷启动或首字生成极慢。Go 代码中硬编码了一个非常贴心的兜底阈值：如果上层没有传入 Context 超时时间，将会默认给予 `10 分钟` 的宽容度 (`10 * time.Minute`)，彻底解决了 JS 版中常常因为底层库默认 60s 超时而导致生成中断的恶性 BUG。

## 差异清单

### P2 设计差异 (黑盒解耦)

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| OLL-1 | **流式协议支持** | 依赖 `@mariozechner/pi-ai` 发包。 | 原生用 `net/http` + `bufio.Scanner`。 | **架构极度轻量化 (P2)**。Go 版将庞大的外部依赖化解为了不足 200 行代码的标准库组合。无需修复。 |
| OLL-2 | **错误解析** | 黑盒抛出。 | 解析了 HTTP StatusCode，并拦截组装了可读的 `APIError` 结构体，支持自动重试判定 (`>= 500`)。 | 健壮性明显提升。无需修复。 |

## 隐藏依赖审计 (Step D)

执行了文本级别的全面结构探视：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | 回退到 `http://localhost:11434`，若 Config 未配置的话。标准行为。 | 安全。 |
| **2. 并发安全** | 每次调用独立分配 `http.Request`，利用自带的协程池。 | 极度安全。 |
| **3. 第三方包黑盒** | 纯标准库实现。不假外求。 | 优秀。 |

## 下一步建议

通过自己接管 Ollama 的 HTTP 流式协议流，Acosmi 成功摆脱了又一个不透明的第三方 Node module，实现了闭环可控。审计通过，安全结案。
