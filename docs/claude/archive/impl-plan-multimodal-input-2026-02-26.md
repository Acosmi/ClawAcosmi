---
document_type: Archive
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-multimodal-input.md
skill5_verified: true
---

# 多模态输入全量补全 — 实施跟踪

## Online Verification Log

### 飞书多媒体消息 API
- **Query**: "飞书 发送图片消息 API" / "Lark send image message API"
- **Source**: https://open.feishu.cn/document/server-docs/im-v1/message/create
- **Key finding**: 两步流程 (先上传获取 key → 再引用 key 发消息); 图片 10MB, 文件 30MB; content 必须 JSON 字符串; 音频用 msg_type:"audio"+file_key 可内联播放
- **Verified date**: 2026-02-26

### MediaRecorder Web API + STT 格式兼容
- **Query**: "MDN MediaRecorder API audio recording" + "OpenAI Whisper API supported formats"
- **Source**: MDN + OpenAI API docs + Groq docs
- **Key finding**: Safari 18.4+ 起 webm;opus 全浏览器通用; OpenAI/Groq 直接接受 webm; fallback audio/mp4; 25MB 限制
- **Verified date**: 2026-02-26

### Feishu oapi-sdk-go/v3 SDK 上传方法
- **Query**: "larksuite oapi-sdk-go v3 upload image file"
- **Source**: https://github.com/larksuite/oapi-sdk-go
- **Key finding**: SDK 提供 Im.Image.Create() 和 Im.File.Create()
- **Verified date**: 2026-02-26

---

## 探索发现 (实施前)

### 关键断裂点
1. `MultimodalPreprocessor` 有 audio/document 占位符，从未调用 STTProvider/DocConverter
2. `GatewayState` 无 STTProvider/DocConverter 字段
3. `DispatchMultimodalFunc` 在 server.go 中已有设置机制
4. STT/DocConv provider 实现完整 (OpenAI/Groq/Azure/local-whisper + MCP/Builtin)
5. STT/DocConv config RPC 已存在 (stt.config.get/set, docconv.config.get/set)
6. 前端 ChatAttachment 类型已支持 audio category
7. 前端 chat.ts 已有 attachment 渲染预览 (renderSingleAttachmentPreview)
8. 前端 chat controller 已在 chat.send RPC 中发送 attachments 数组

### 现有可复用代码
- `media.NewSTTProvider(cfg)` / `media.NewDocConverter(cfg)` — 工厂函数
- `media.IsSupportedFormat(fileName)` — 格式检测
- `FeishuClient.DownloadFile()` / `DownloadImage()` — 资源下载
- `ChatAttachment` 类型 + `inferCategory()` — 前端分类
- `sendChatMessage()` 已支持 attachments content blocks

---

## Phase 1: STT/DocConv 管线连通 (~120 LOC Go)

- [x] 1.1 扩展 MultimodalPreprocessor 结构体 (+STTProvider, +DocConverter)
- [x] 1.2 替换 audio 占位符 → 调用 STTProvider.Transcribe()
- [x] 1.3 替换 document 占位符 → 调用 DocConverter.Convert()
- [x] 1.4 boot.go / server.go 注入 STT/DocConv provider 到 preprocessor

**文件**: `gateway/server_multimodal.go`, `gateway/boot.go` (或 server.go 中 preprocessor 创建处)

---

## Phase 2: stt.transcribe RPC 端点 (~50 LOC Go + ~30 LOC TS)

- [x] 2.1 新增 `stt.transcribe` RPC handler (server_methods_stt.go)
- [x] 2.2 前端 transcribeAudio() 控制器函数
- [x] 2.3 i18n keys (转录中/成功/失败)

**文件**: `gateway/server_methods_stt.go`, `ui/controllers/stt.ts` (或 chat.ts), `ui/locales/`

---

## Phase 3: 前端语音录制 UI (~200 LOC TS)

- [x] 3.1 VoiceRecorder 控制器 (MediaRecorder 封装)
- [x] 3.2 录音按钮 + 录音中 UI (chat.ts)
- [x] 3.3 app 状态字段 (voiceRecording, voiceRecordingDuration, voiceSupported)

**文件**: `controllers/voice-recorder.ts`, `views/chat.ts`, `app.ts`, `app-view-state.ts`, `locales/`

---

## Phase 4: 飞书多媒体上传 (~120 LOC Go)

- [x] 4.1 UploadImage() — POST /open-apis/im/v1/images
- [x] 4.2 UploadFile() — POST /open-apis/im/v1/files
- [x] 4.3 feishuFileType() MIME→file_type 映射

**文件**: `feishu/resource.go`

---

## Phase 5: 飞书多媒体发送 (~80 LOC Go)

- [x] 5.1 SendImageMessage / SendAudioMessage / SendFileMessage (client.go)
- [x] 5.2 FeishuSender 便捷方法 (sender.go)
- [x] 5.3 扩展 FeishuPlugin.SendMessage() + sendMediaMessage (plugin.go)

**文件**: `feishu/client.go`, `feishu/sender.go`, `feishu/plugin.go`

---

## Phase 6: 前端聊天富媒体渲染 (~100 LOC TS)

- [x] 6.1 消息 content block 渲染 (image → img, audio → audio player) — 已有实现
- [x] 6.2 CSS 样式 — 录音按钮 + 脉冲动画

**文件**: `views/chat.ts`

---

## Phase 7: chat.send 附件 STT/DocConv (~60 LOC Go)

- [x] 7.1 AttachmentParser 扩展: audio → STT, document → DocConv

**文件**: 后端 chat.send handler 或 AttachmentParser 实现

---

## 依赖关系

```
P1 (管线连通) → P7 (chat.send 附件) → 独立
P2 (stt RPC)  → P3 (录音 UI)       → 独立
P4 (飞书上传) → P5 (飞书发送)       → 独立
P6 (富媒体渲染)                     → 独立
```

**执行顺序**: P1 → P2 → P4 → P5 → P3 → P7 → P6

---

## 审计修复

- [x] C-01 CRITICAL: SSRF 防护 — validateMediaURL + 自定义 HTTP client (plugin.go)
- [x] H-01 HIGH: DownloadResource 添加 50 MB LimitReader (resource.go)
- [x] H-02 HIGH: getTenantAccessToken 添加 64 KB LimitReader (resource.go)
- [x] H-03 HIGH: Send*Message 改用 json.Marshal (client.go)
- [x] H-04 HIGH: getTenantAccessToken 改用 json.Marshal (resource.go)
- [x] H-05 HIGH: processAttachmentsForChat 添加 base64 大小限制 (server_methods_chat.go)
- [x] M-01: DispatchMultimodalFunc 添加 120s 超时 (server.go)
- [x] M-02: ProcessFeishuMessage 限制最多 10 个附件 (server_multimodal.go)
- [x] M-03: 移除 ImageBase64Blocks 死代码 (server_multimodal.go)
- [x] M-04: handleSTTTranscribe 先检查 base64 长度再解码 (server_methods_stt.go)
- [x] M-05: STT 错误响应不泄露内部细节 (server_methods_stt.go)
- [x] M-06: readLimited 超限返回错误而非静默截断 (plugin.go)
- [x] M-13: handleVoiceStart 错误时调用 cancel() 释放 MediaStream (app.ts)

---

## 验证清单

- [x] `go build ./...` 编译通过
- [x] `npx tsc --noEmit` 无新增错误
- [ ] P1: 飞书语音 → 转录文本 (非占位符)
- [ ] P1: 飞书文档 → markdown (非占位符)
- [ ] P2: 前端 stt.transcribe → 返回正确文本
- [ ] P3: 麦克风录音 → STT / 附件
- [ ] P4: UploadImage → 有效 image_key
- [ ] P5: agent 发图片到飞书 → 对方收到
- [ ] P6: 图片消息缩略图 + 音频播放器
- [ ] P7: UI 音频附件 → agent 收到转录
- [ ] STT 未配置 → 优雅降级
- [ ] DocConv 未配置 → 优雅降级
- [ ] 浏览器不支持 MediaRecorder → 隐藏录音按钮
- [ ] 现有功能回归无影响
