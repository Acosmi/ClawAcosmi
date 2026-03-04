# 公共工具包架构文档

> 最后更新：2026-02-26 | 代码级审计完成

本文档涵盖 `backend/pkg/` 和 `backend/internal/` 下的小型工具包。

## pkg/utils/ — 通用工具函数

| 属性 | 值 |
| ---- | ---- |
| 路径 | `backend/pkg/utils/` |
| 文件 | `utils.go`, `utils_test.go` |
| 行数 | ~263 |

提供常用工具函数：字符串处理、安全的类型转换、环境变量读取辅助等。

## pkg/retry/ — 重试逻辑库

| 属性 | 值 |
| ---- | ---- |
| 路径 | `backend/pkg/retry/` |
| 文件 | `retry.go`, `retry_test.go` |
| 行数 | ~257 |

通用重试执行器：

- **指数退避 + 抖动 (jitter)**
- 可配置最大重试次数和超时
- 支持自定义 `ShouldRetry(err) bool` 判断是否值得重试
- Context-aware：ctx 取消立即停止

## pkg/polls/ — 轮询工具

| 属性 | 值 |
| ---- | ---- |
| 路径 | `backend/pkg/polls/` |
| 文件 | `polls.go`, `polls_test.go` |
| 行数 | ~96 |

可取消的定时轮询执行器，用于定期检查外部状态（如 API 就绪、资源可用）。

## pkg/media/ — Web 媒体工具

| 属性 | 值 |
| ---- | ---- |
| 路径 | `backend/pkg/media/` |
| 文件 | `web_media.go`, `web_media_test.go` |
| 行数 | ~159 |

URL 媒体工具：

- 媒体类型检测 (MIME sniffing)
- 文件大小获取 (Content-Length)
- 与 `internal/media/` 的区别：此包是纯工具函数，不含业务逻辑

## internal/session/ — 会话类型定义

| 属性 | 值 |
| ---- | ---- |
| 路径 | `backend/internal/session/` |
| 文件 | `types.go` |
| 行数 | ~150 |

纯类型包，定义 `SessionEntry` 结构体。与 `internal/sessions/` (会话存储逻辑) 分离，避免循环依赖。

## internal/ollama/ — Ollama 占位

| 属性 | 值 |
| ---- | ---- |
| 路径 | `backend/internal/ollama/` |
| 文件 | 空（无 .go 文件）|

当前为空占位目录，Ollama 集成逻辑实际在 `internal/agents/llmclient/ollama.go` 中实现。
