# 全局审计报告 — TTS 模块

## 概览

| 维度 | TS (`src/tts`) | Go (`backend/internal/tts`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 2 | 9 | 单文件拆分为多结构 |
| 总行数 | ~1580 | ~1880 | 100% 结构覆盖 |

### 文件映射与重构情况

本模块经历了一对多的精细化拆建。

* `src/tts/tts.ts` 单文件被完美拆分为：
  * `tts.go` (主入口点，缓存查阅和顺序回退机制)
  * `cache.go` (内存与临时文件调度清理)
  * `config.go` / `prefs.go` / `types.go` (配置与类型解耦)
  * `directives.go` (内联 `[[tts:xxx]]` 控制信令抽取和正则替换)
  * `provider.go` (厂商选择、Token提取、环境回退)
  * `synthesize.go` (三大 LLM 纯发音体对接)

整体核心流程、逻辑树以及缓存/Fallback 的机制两边保持高度一致。

## 差异清单

### P2 机制缺失：自定义 OpenAI 兼容服务端点

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| TTS-1 | **忽略了 `OPENAI_TTS_BASE_URL` 自定义端点** | `getOpenAITtsBaseUrl()` 会读取此环境变量。当用户配置了此变量时，由于被判定为自定义模型（例如本地部署的 Kokoro 或 LocalAI），**跳过 OpenAi Voice/Model 的严格白名单校验**。 | 在 `synthesize.go` 中，API 端点硬编码为 `"https://api.openai.com/v1/audio/speech"`，没有暴露覆盖选项，也并未跳过相关校验。 | **需修复 (P2)**。应该在 `ResolvedTtsConfig.OpenAI` 或直接在内部解析环境变量，将硬编码的 URL 替换，以支持越来越火的本地自建生态。 |

### P2 功能阉割：超长音频 LLM 智能总结

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| TTS-2 | **超长文本的摘要压缩策略** | 当长文超过 `maxLength` (如 1500) 且 `summarize` 为真时，会通过 `@mariozechner/pi-ai` 走一次 LLM 指令：`Summarize the text to approximately {targetLength} characters...` 作为语音输入。 | 在 `tts.go` 的第 55 行直接粗暴阶段：`text = string(runes[:params.Config.MaxTextLength])`，没有任何摘要处理直接截断导致语意突兀结尾。 | **需修复 (P2)**。需要从网关侧（或借助外部 Client）补充这一步摘要 Prompt 的中间代理环节。 |

### P3 次要细节

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| TTS-3 | **Edge TTS 连接方式** | 以 Node 库 (`node-edge-tts`) 直接实现该巨硬系的 WebSocket 握手拉流。 | Go 版中以 `exec.Command("edge-tts", ...)` 调起 Python 版的第三方跨系统 CLI。 | 考虑到 Edge 的签名极不且长变，Python版维护更紧密也是不错的脱壳代理，暂无大碍。 |

## 隐藏依赖审计 (Step D)

执行了详尽的 `grep` 环境探测：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. 环境变量** | `OPENACOSMI_TTS_PREFS`, `ELEVENLABS_API_KEY`, `XI_API_KEY`, `OPENAI_API_KEY` 全量对齐。 | `OPENAI_TTS_BASE_URL` 在 Go 版丢失，已被记为 P2 Gap。 |
| **2. API & fs 网络通信** | TS 使用 Node 原生 `fetch` 和 `fs` 对流的管线式落盘。 | Go 使用 `http.Request`, `bytes.Buffer`, `os.TempDir` 做到了一对一完美复刻音频的清理（Goroutine delay `TempFileCleanupDelay` vs JS `setTimeout` unref）。 |
| **3. 第三方包黑盒** | `node-edge-tts` (TS) vs Python `edge-tts` bash CLI (Go)。 | 实现手段差异，不阻碍业务逻辑。 |

## 下一步建议

TTS 的核心解耦和接口定义在 Go 版本体现得十分标准与清爽，但它遗漏了两个极为改善用户体验的核心要素（智能长文总结摘要 与 自建 LocalAI TTS 接驳）。必须创建对应 deferred 修复任务单。向导可推进至下一个模块。
