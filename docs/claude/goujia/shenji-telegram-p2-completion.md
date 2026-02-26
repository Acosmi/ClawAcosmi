> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram P2 审计完成报告（消息格式与媒体）

## 审计范围

9 对文件，全部已审计、修复并通过复核。

## 审计日期

2026-02-24

## 修复摘要

### format.ts ↔ format.go

| 修复项 | 内容 |
|--------|------|
| F-M1 headingStyle | `"bold"` → `"none"` 对齐 TS |
| F-M2 blockquotePrefix | `"> "` → `""` 对齐 TS |
| F-M3 TableMode 常量 | `text/html` → `off/bullets/code` 对齐 TS 类型定义 |
| F-H2 分块度量统一 | 单块优化从 `len([]rune)` 改为 `len(ir.Text)` 与 ChunkMarkdownIR 字节度量一致 |
| send.go 联动 | 更新 `mapGlobalTableMode` 使用新常量名 |
| send_table_mode_test.go 联动 | 更新测试用例使用新常量名 |

### caption.ts ↔ caption.go

| 状态 | 内容 |
|------|------|
| PASS | 基本完美对齐，无需修复 |

### download.ts ↔ download.go

| 修复项 | 内容 |
|--------|------|
| D-H2 maxBytes 溢出检测 | `io.LimitReader(maxBytes+1)` + 下载后 `len(data) > maxBytes` 报错（比 TS 更优：不下载全部数据） |
| D-H1 MIME 三级检测 | 改用 `media.DetectMime(Buffer+HeaderMime+FilePath)` 替代弱 fallback |
| 死代码清理 | 删除 `detectContentTypeFromPath`，移除 `strings` 未用 import |

### voice.ts ↔ voice.go

| 修复项 | 内容 |
|--------|------|
| V-M1 URL 扩展名解析 | 新增 `getVoiceFileExtension`：URL 时从 `url.Parse(path).Path` 提取扩展名 |

### inline-buttons.ts ↔ inline_buttons.go

| 修复项 | 内容 |
|--------|------|
| IB-H1 联合类型反序列化 | `TelegramCapabilitiesConfig` 添加 `UnmarshalJSON`（先尝试 `[]string`，后尝试 object） |
| IB-H2 数组优先级 | Tags 非空时先走数组分支；数组不含 inlinebuttons 返回 `"off"` |
| 审计修复: 空数组处理 | `len(caps.Tags) > 0` → `caps.Tags != nil`（空 JSON `[]` 也走数组分支） |
| 审计修复: 空账户回退 | `IsTelegramInlineButtonsEnabled` 添加 `len(ids) == 0` 时检查根级配置 |

### model-buttons.ts ↔ model_buttons.go

| 修复项 | 内容 |
|--------|------|
| MB-H1 正则大小写 | `mdlListRe` 添加 `(?i)` 标志支持大写 provider ID |
| MB-M2 rune 截断 | `truncateModelID` 改用 `[]rune` 操作 |
| MB-M1 pageSize 参数 | `BuildModelsKeyboard` 添加 `pageSize int` 参数（`<=0` 使用默认值） |

### reaction-level.ts ↔ reaction_level.go

| 状态 | 内容 |
|------|------|
| PASS | 完美对齐，无需修复 |

### sticker-cache.ts ↔ sticker_cache.go

| 修复项 | 内容 |
|--------|------|
| SC-M1 并发锁 | 新增 `stickerCacheMu sync.Mutex`，所有公开函数加锁保护 |
| SC-H2 描述常量 | 新增 `StickerDescriptionPrompt`/`StickerDescriptionMaxTokens`/`StickerDescriptionTimeoutMs` |

### sent-message-cache.ts ↔ sent_message_cache.go

| 状态 | 内容 |
|------|------|
| PASS | 高质量迁移，无需修复 |

## 复核审计结果

3 组并行交叉颗粒度审计已完成:
- format + caption + download: PASS
- voice + inline_buttons + model_buttons + reaction_level: FAIL（2 项已修复后 PASS）
- sticker_cache + sent_message_cache: PASS

## 残余已知差异（LOW，已确认可接受）

1. **Linkify 未实现**: Go 简化 Markdown 解析器不支持裸 URL 自动检测。Telegram 客户端自身支持 URL 检测，实际影响极低。长期可考虑引入 goldmark 替代。
2. **UTF-16 vs rune 长度**: caption.go 使用 `len([]rune)` 而非 UTF-16 code unit 计数，对 BMP 以外字符（emoji）的处理略有差异。
3. **DescribeStickerImage DI 重设计**: Go 使用 DI 模式替代 TS 内联的 vision provider 选择逻辑，返回 fallback 文本而非 null。已在 `bot_message_dispatch.go` 中直接使用 `deps.DescribeImage` 实现等价功能。
4. **ButtonRow 死代码**: `model_buttons.go` 中 `ButtonRow` 类型定义未使用。
5. **SentMessageCache 实例 vs 全局**: Go 使用实例方法替代 TS 模块级全局变量，需确保调用方正确管理单例。

## 修改文件清单

| 文件 | 修改类型 |
|------|----------|
| `format.go` | headingStyle + blockquotePrefix + TableMode 常量 + 分块度量 |
| `download.go` | maxBytes + media.DetectMime + 死代码清理 |
| `voice.go` | getVoiceFileExtension URL 解析 |
| `inline_buttons.go` | 数组优先级 + 空数组 + 空账户回退 |
| `model_buttons.go` | (?i) 正则 + rune 截断 + pageSize |
| `sticker_cache.go` | sync.Mutex + 描述常量 |
| `send.go` | TableMode 常量名更新 |
| `send_table_mode_test.go` | TableMode 常量名更新 |
| `pkg/types/types_telegram.go` | UnmarshalJSON 联合类型 |
