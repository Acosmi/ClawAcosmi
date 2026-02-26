# 全局审计报告 — Media 模块

## 概览

| 维度 | TS (`src/media`) | Go (`backend/internal/media`) | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 11 | 11 | 100% |
| 总行数 | ~1950 | ~1900 | N/A |

### 文件映射情况

* `audio-tags.ts` -> `audio_tags.go` (内联实现，因指令处理模块未移植)
* `audio.ts` -> `audio.go` (行为完全一致)
* `constants.ts` -> `constants.go` (行为完全一致)
* `fetch.ts` -> `fetch.go` (SSRF 防御及 HTTP limitReader，行为一致)
* `host.ts` -> `host.go` (隧道策略差异)
* `image-ops.ts` -> `image_ops.go` (sips 及 Sharp 逻辑，Go端用原生 image 库降级，等价替换)
* `input-files.ts` -> `input_files.go` (**核心差异**：PDF渲染缺失)
* `mime.ts` -> `mime.go` (http sniff 替代 file-type，等价)
* `parse.ts` -> `parse.go` (**核心差异**：Markdown 语法感知缺失)
* `server.ts` -> `server.go` (CORS/Auth 与 TTL 定时清理策略差异)
* `store.ts` -> `store.go` (HTTP fetch Headers 设计差异)

## 差异清单

### P1 核心功能缺失

| ID | 描述 | TS 实现 | Go 实现 | 修复状态 |
|----|------|---------|---------|----------|
| ~~MEDIA-1~~ ✅ | **PDF 页面渲染转图片缺失** | `input-files.ts` (`extractPdfContent`) 使用 `pdfjs` 和 `@napi-rs/canvas` 获取页面并渲染输出 PNG 图片数组 `images: InputImageContent[]`。 | ✅ **已修复**（2026-02-22 W-FIX-2）：新增 `PDFLimits` 类型 + 三级渲染策略（pdftocairo / qlmanage / pdfcpu ExtractImagesRaw），文本优先图片 fallback（MinTextChars=200）。 | ✅ 已修复 |
| ~~MEDIA-2~~ ✅ | **Markdown Fenced Code 处理缺失** | `parse.ts` 使用 `parseFenceSpans` 函数检查字符偏移，避免解析包含在代码块 (` ``` `) 以及其它语法边界内的 `MEDIA:` token。 | ✅ **已修复**（2026-02-22 W-FIX-2）：引入 `markdown.ParseFenceSpans` + `isInsideFence` + `lineOffset` 追踪，12 个新增测试 PASS。 | ✅ 已修复 |

### P2 功能降级或设计变动

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| MEDIA-3 | **媒体资源下载不支持自定义 Headers** | `store.ts` (`saveMediaSource` -> `downloadToFile`) 支持透传 `headers?: Record<string, string>` 字典以应对鉴权资源拉取。 | `store.go` (`downloadAndSave`) 暴力的 `http.Get(rawURL)` 无法传入任何 Auth Header。 | 将 `http.Get` 改写为实例化 `http.NewRequest`，并将入参增强以接受 `map[string]string` 的 Headers 透传。 |
| MEDIA-4 | **托管服务器与隧道启动设计** | `host.ts` (`ensureMediaHosted`) 不主动启动隧道，默认使用已有 `getTailnetHostname()` 作为 URL（除非 `startServer=true` 且利用简单 Node Port 绑定）。 | `host.go` 会**隐式生成后台进程** (`tailscale funnel` 或 `cloudflared tunnel`) 以绑定公网 URL 暴露端口。这是架构主动防御层面的一种转换。 | 由于这属于 Go 版本自主升级带来的增强/改变，功能角度是符合或更强的，可视为已知系统设计差异，暂不需要修改，但需记录为架构不同。 |

### P3 次要细节

| ID | 描述 | TS 实现 | Go 实现 | 修复建议 |
|----|------|---------|---------|----------|
| MEDIA-5 | **媒体服务 TTL 定时清理限制与认证增强** | `server.ts` 利用 `setInterval` 主动随时间清理 TTL (默认2分钟)。没有携带鉴权或 CORS。 | `server.go` 仅在 `SaveMediaSource` 时懒汉式触发 `CleanOldMedia`。同时具备 TS 不拥有的 `MediaServerAuthConfig` (CORS 和 Bearer Token) 中间件增强。 | Go 端的懒汉式可以接受，不会常驻内存消耗，且提供 CORS 与 Auth 是合理的系统演进升级。无需修复。 |

## 隐藏依赖审计 (Step D)

执行了详尽的 `grep` 验证后，输出结果如下（均已分析并覆盖或视为等效替换设计）：

| 测试项 | 结果 / 发现 | 结论 |
|--------|-------------|------|
| **1. npm 隐层黑盒** | / | 无隐藏的底层 C/C++ node_modules 耦合（涉及 `@napi-rs/canvas` 部分已在 MEDIA-1 中指出）。 |
| **2. 全局状态/单例** | 发现引用 `../globals.js` 的 `danger`。 | Go 版在错误处理和日志中使用了标准打点或 `fmt.Errorf` / `log.Printf`，解耦。 |
| **3. 事件总线** | 发现在 `server.ts` / `store.ts` 使用了 Node.js Stream的 `res.on("finish")`、`req.on("error")` 事件回调进行清理和异步。 | Go 使用了等价模型 `net/http` 原生的阻塞与 `defer` 以及 `io.LimitReader` 代替这种基于事件的异步机制，完全合规。 |
| **4. 环境变量** | `OPENACOSMI_IMAGE_BACKEND` 为决定是否回退 sips 引擎的环境变量。 | 已映射，等效（Go语言底层依赖 `prefersSips() bool` 等方式隐式替代了该环境变量的回退策略，即 Darwin 下优先 sips）。 |
| **5. 文件系统协定** | `/tmp` 目录建立与写入频繁（`image-ops.ts`, `store.ts`）；缓存路径建立 `~/.openacosmi`。 | Go 已统一采用了 `os.MkdirTemp` 及 `resolveMediaDir` 来复现此约定的生命周期。 |
| **7. 错误处理** | TS 版本具备大量 `catch/throw`。 | Go 统一使用 `err != nil` 返回结构化定义包含如 `MediaFetchError`，逻辑等价。 |

## 下一步建议

~~针对 `MEDIA-1` (PDF图形支持) 与 `MEDIA-2` (Markdown边界防止误捕获) 提交修补建议~~ ✅ 已在 W-FIX-2 中全部修复（2026-02-22）。

**剩余 P2 待办**：MEDIA-3（`store.go` 下载自定义 Headers）已排入 W-FIX-6。
