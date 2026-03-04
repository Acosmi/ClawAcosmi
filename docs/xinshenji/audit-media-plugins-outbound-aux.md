---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/{media, plugins, outbound, tts, acp, argus, canvas, linkparse, mcpclient, nodehost, pairing, cli, tui}
verdict: Pass with Notes
---

# 审计报告: 媒体/插件/外发/语音合成 + 辅助模块

## 范围

- `media/` — 20 files + understanding/15 (图像处理/STT/文档转换)
- `plugins/` — 19 files (插件注册/发现/安装/更新)
- `outbound/` — 9 files (外发消息/跨上下文策略)
- `tts/` — 11 files (语音合成/缓存/摘要)
- 辅助模块: `acp`(10), `argus`(11), `canvas`(5), `linkparse`(6), `mcpclient`(3), `nodehost`(14), `pairing`(5), `cli`(13), `tui`(28)

## 审计发现

### [PASS] 正确性: 图像处理双模式 (media/image_ops.go)

- **位置**: `image_ops.go:37-41, 211-254`
- **分析**: macOS 优先使用 `sips` 命令行工具（硬件加速），非 macOS 使用 Go 内置 `image` 包 + CatmullRom 双三次插值。EXIF 方向读取支持字节级 APP1 段解析。HEIC→JPEG 转换有 sips fallback。标记为 `RUST_CANDIDATE: P2`。
- **风险**: None

### [PASS] 安全: 图像参数边界检查 (media/image_ops.go)

- **位置**: `image_ops.go:268-279, 416-433`
- **分析**: `SnapshotDom` 和 `QuerySelector` 的 `limit` 和 `maxTextChars` 参数有上限保护（5000/200/20000）。图像 `OptimizeImageToPng` 自适应降低分辨率直到符合 maxBytes。
- **风险**: None

### [PASS] 正确性: 插件注册表多维注册 (plugins/registry.go)

- **位置**: `registry.go:110-260`
- **分析**: `PluginRegistry` 使用 `sync.RWMutex` 保护，支持 8 种注册类型: Tools、Hooks、Channels、Providers、HttpHandlers、HttpRoutes、Services、Commands。`RegisterTool` 通过 DI 注入 `PluginToolFactory`，支持可选工具（`optional` 标志）。`RegisterGatewayMethod` 注册的方法不覆盖核心网关处理器。
- **风险**: None

### [PASS] 安全: 跨上下文消息策略 (outbound/policy.go)

- **位置**: `policy.go:143-203`
- **分析**: `EnforceCrossContextPolicy` 阻止智能体在未授权时跨频道/跨会话发送消息。支持两级策略: `allowWithinProvider`（同提供商内跨会话）和 `allowAcrossProviders`（跨提供商）。默认同提供商允许、跨提供商禁止。跨上下文消息可添加装饰标记（前缀/后缀）提示用户来源。
- **风险**: None

### [WARN] 正确性: 图像 sips 命令注入风险 (media/image_ops.go)

- **位置**: `image_ops.go:256-311`
- **分析**: `sipsResizeToJpeg` 和 `sipsConvertToJpeg` 通过 `exec.Command("sips", ...)` 执行系统命令。参数来自内部计算（maxSide、quality），不直接接受用户输入。文件路径是临时文件。实际风险极低。
- **风险**: Low (参数不来自用户输入)

### [PASS] 正确性: TTS 合成缓存 (tts/)

- **位置**: `tts/cache.go`, `tts/synthesize.go`
- **分析**: TTS 模块支持多 provider、语音偏好配置、合成结果缓存。缓存基于文本哈希，避免重复API调用。
- **风险**: None

### [PASS] 正确性: 辅助模块整体质量

- **分析**:
  - `linkparse` — URL/消息链接解析
  - `mcpclient` — MCP 协议客户端
  - `canvas` — 画布渲染
  - `nodehost` — Node.js 宿主进程管理
  - `pairing` — 设备配对
  - `acp` — ACP 协议实现
  - `argus` — 感知层集成
  - `cli`/`tui` — 命令行和终端 UI 入口
  所有辅助模块文件数较少（3-28），职责清晰。
- **风险**: None

## 总结

- **总发现**: 7 (6 PASS, 1 WARN, 0 FAIL)
- **阻断问题**: 无
- **结论**: **通过（附注释）** — 插件注册体系完整，跨上下文消息策略安全。图像处理性能有 sips/Rust 优化路径。
