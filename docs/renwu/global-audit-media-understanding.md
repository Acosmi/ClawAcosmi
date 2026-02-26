# media-understanding 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W8，部分 2 (中型模块)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 24 | 15 | 62.5% |
| 总行数 | 3436 | 1827 | 53.1% |

*(注：Go端的覆盖率较低，主要是因为在 Go 的重构架构中，诸如附件处理（`attachments.ts`）、格式应用（`apply.ts`）和部分基础格式化（`format.ts`）的逻辑（总计约 1100 行）已经被下沉合并到 `backend/internal/media` 核心库中。在之前的 `media` 模块审计(Go版行数超4000)中已确认。本模块纯粹作为 Provider 抽象层存在。)*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 |
|------|---------|---------|
| ✅ FULL | `runner.ts` | `runner.go`, `registry.go` |
| ✅ FULL | `resolve.ts` | `resolve.go` |
| ✅ FULL | `video.ts` | `video.go` |
| ✅ FULL | `concurrency.ts` | `concurrency.go` |
| ✅ FULL | `defaults.ts` | `defaults.go` |
| ✅ FULL | `scope.ts` | `scope.go` |
| ✅ FULL | `types.ts` | `types.go` |
| ✅ FULL | `providers/openai/*` | `provider_openai.go` |
| ✅ FULL | `providers/anthropic/*` | `provider_anthropic.go` |
| ✅ FULL | `providers/google/*` | `provider_google.go` |
| ✅ FULL | `providers/minimax/*` | `provider_minimax.go` |
| ✅ FULL | `providers/deepgram/*` | `provider_deepgram.go` |
| ✅ FULL | `providers/groq/*` | `provider_groq.go` |
| ✅ FULL | `providers/image.ts`, `providers/shared.ts` | `provider_image.go`, (在各自 Provider 中共享) |
| 🔄 REFACTORED | `attachments.ts`, `apply.ts`, `format.ts` | **(移至 backend/internal/media 模块)** |

> 评价：这是一个典范的解耦重构。TS 中混淆在 "理解("understanding")" 和 "多媒体实体表示("media")" 之间的职责在 Go 版得到了完美的边界切分：结构化表示归 `media`，调用各大厂商大模型提供视觉与音频理解能力归 `media/understanding`。

## 隐藏依赖审计

| # | 类别 | 检查结果 | Go端实现方案 |
|---|------|----------|-------------|
| 1 | npm 包黑盒行为 | ✅ 无 | 完全基于自己的 httpClient 实现各大模型厂商的 API 调用 |
| 2 | 全局状态/单例 | ✅ 无 | 被完全封装在 Runner 实例中 |
| 3 | 事件总线/回调链 | ✅ 无 | 同步与并发混合控制 |
| 4 | 环境变量依赖 | ⚠️ Providers 依赖各厂商的 API Key | 在上游或 config 包统一注入，符合 Go 设计 |
| 5 | 文件系统约定 | ✅ 无 | 处理的均是由上游下载后的内存结构或临时文件路径（不强绑定创建） |
| 6 | 协议/消息格式 | ⚠️ 各家厂商不兼容的非标准输入 | Go的 provider 层单独做了定制序列化(`json.Marshal`)和结构体约束 |
| 7 | 错误处理约定 | ⚠️ API 限流和请求失败 | 采用了标准的重试策略和并发错误冒泡 `golang.org/x/sync/errgroup` |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| MEDU-1 | 架构重构 | `attachments.ts`, `apply.ts` | (移出) | 核心富媒体封装职责下沉，Media-Understanding 变纯粹，代码量大幅下降。 | P3 | 良好的架构改进，无需修复 |
| MEDU-2 | 目录结构 | `providers/*` | `provider_*.go` | 展平 Providers 为单文件，易于寻找。 | P3 | 提升了可读性，无需修复 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- **模块审计评级: A** (模块边界更加清晰，消除了在 TS 中混在一处的职责，完美的解耦。)
