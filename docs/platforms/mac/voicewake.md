---
summary: "macOS 语音唤醒和 Push-to-Talk 实现细节"
read_when:
  - 处理语音唤醒或 PTT 路径
title: "语音唤醒"
---

# 语音唤醒与 Push-to-Talk

## 模式

- **唤醒词模式**（默认）：始终在线的 Speech 识别器等待触发词。匹配时开始捕获，显示覆盖层并显示部分文本，静默后自动发送。
- **Push-to-Talk（按住右 Option 键）**：按住右 Option 键立即捕获——无需触发词。覆盖层在按住期间显示；释放后短暂延迟再完成发送。

## 运行时行为（唤醒词）

- Speech 识别器位于 `VoiceWakeRuntime` 中。
- 触发仅在唤醒词和下一个词之间有**明显停顿**（约 0.55s 间隔）时触发。
- 静默窗口：说话时 2.0s，仅听到触发词时 5.0s。
- 硬性停止：120s 防止失控会话。
- 会话间防抖：350ms。
- 覆盖层由 `VoiceWakeOverlayController` 驱动。
- 发送后，识别器干净重启以监听下一个触发。

## 生命周期不变量

- 语音唤醒启用且权限已授予时，唤醒词识别器应处于监听状态（Push-to-Talk 捕获期间除外）。
- 覆盖层可见性（包括通过 X 按钮手动关闭）不得阻止识别器恢复。

## Push-to-Talk 详情

- 热键检测使用全局 `.flagsChanged` 监视器检测**右 Option**（`keyCode 61` + `.option`）。仅观察事件（不拦截）。
- 捕获管道位于 `VoicePushToTalk`：立即启动 Speech，流式部分文本到覆盖层，释放时调用 `VoiceWakeForwarder`。
- Push-to-Talk 启动时暂停唤醒词运行时以避免音频冲突；释放后自动重启。
- 权限：需要麦克风 + Speech；检测事件需要辅助功能/输入监控权限。

## 用户设置

- **语音唤醒** 开关：启用唤醒词运行时。
- **按住 Cmd+Fn 说话**：启用 Push-to-Talk 监视器（macOS 26 以下禁用）。
- 语言和麦克风选择器、电平计、触发词表、测试器（仅本地，不转发）。
- 麦克风选择器在设备断开时保留上次选择，显示断开提示，临时回退到系统默认。
- **音效**：触发检测和发送时的提示音；默认使用 macOS "Glass" 系统声音。

## 转发行为

- 语音唤醒启用时，转写文本被转发给活跃的 Go Gateway/agent（与 macOS 应用其他功能使用相同的本地/远程模式）。
- 回复传递到**上次使用的主 provider**（WhatsApp/Telegram/Discord/WebChat）。
