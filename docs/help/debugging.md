---
summary: "调试工具：watch 模式、原始模型流、推理泄露追踪"
read_when:
  - 需要检查原始模型输出中的推理泄露
  - 需要在迭代中以 watch 模式运行 Gateway
  - 需要可重复的调试工作流
title: "调试"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# 调试

本页面涵盖流式输出的调试辅助工具，特别是当 Provider 将推理内容混入正常文本时的场景。

## 运行时调试覆盖

在聊天中输入 `/debug` 设置**仅运行时**的配置覆盖（内存中，不写入磁盘）。
`/debug` 默认禁用；通过 `commands.debug: true` 启用。
当需要切换不常用设置而不想编辑 `openacosmi.json` 时非常实用。

示例：

```
/debug show
/debug set messages.responsePrefix="[openacosmi]"
/debug unset messages.responsePrefix
/debug reset
```

`/debug reset` 清除所有覆盖，恢复到磁盘上的配置。

## Gateway watch 模式

要快速迭代，在文件监听器下运行 Go Gateway：

```bash
make gateway-watch
```

这等价于：

```bash
air -c .air.toml
```

在 `gateway-watch` 后添加任何 Gateway CLI 标志，它们会在每次重启时被传递。

## 开发配置 + 开发 Gateway（--dev）

使用开发配置来隔离状态，启动一个安全的、可丢弃的调试环境。有**两个** `--dev` 标志：

- **全局 `--dev`（Profile）：** 将状态隔离到 `~/.openacosmi-dev`，默认 Gateway 端口为 `19001`（派生端口随之偏移）。
- **`gateway --dev`：告诉 Gateway 在缺失时自动创建默认配置 + 工作区**（并跳过 BOOTSTRAP.md）。

推荐流程（开发 Profile + 开发引导）：

```bash
make gateway-dev
OPENACOSMI_PROFILE=dev openacosmi tui
```

如果还没有全局安装的 CLI，可以通过下载预编译二进制文件或从源码编译。

做了什么：

1. **Profile 隔离**（全局 `--dev`）
   - `OPENACOSMI_PROFILE=dev`
   - `OPENACOSMI_STATE_DIR=~/.openacosmi-dev`
   - `OPENACOSMI_CONFIG_PATH=~/.openacosmi-dev/openacosmi.json`
   - `OPENACOSMI_GATEWAY_PORT=19001`（浏览器/画布端口随之偏移）

2. **开发引导**（`gateway --dev`）
   - 如缺失则写入最小配置（`gateway.mode=local`，绑定回环地址）。
   - 设置 `agent.workspace` 为开发工作区。
   - 设置 `agent.skipBootstrap=true`（不加载 BOOTSTRAP.md）。
   - 如缺失则播种工作区文件：
     `AGENTS.md`、`SOUL.md`、`TOOLS.md`、`IDENTITY.md`、`USER.md`、`HEARTBEAT.md`。
   - 默认身份：**C3‑PO**（礼仪机器人）。
   - 开发模式下跳过渠道 Provider（`OPENACOSMI_SKIP_CHANNELS=1`）。

重置流程（全新重建）：

```bash
make gateway-dev-reset
```

注意：`--dev` 是一个**全局** Profile 标志，可能被某些运行器吞掉。
如需显式指定，使用环境变量形式：

```bash
OPENACOSMI_PROFILE=dev openacosmi gateway --dev --reset
```

`--reset` 会清除配置、凭据、会话和开发工作区（使用 `trash` 而非 `rm`），然后重新创建默认开发环境。

提示：如果非开发 Gateway 已在运行（launchd/systemd），请先停止它：

```bash
openacosmi gateway stop
```

## 原始流日志（OpenAcosmi）

OpenAcosmi 可以记录**过滤/格式化前的原始助手流**。
这是查看推理内容是否作为纯文本 delta 到达（或作为独立的思考块）的最佳方式。

通过 CLI 启用：

```bash
make gateway-watch -- --raw-stream
```

可选路径覆盖：

```bash
make gateway-watch -- --raw-stream --raw-stream-path ~/.openacosmi/logs/raw-stream.jsonl
```

等效环境变量：

```bash
OPENACOSMI_RAW_STREAM=1
OPENACOSMI_RAW_STREAM_PATH=~/.openacosmi/logs/raw-stream.jsonl
```

默认文件：

`~/.openacosmi/logs/raw-stream.jsonl`

## 原始 chunk 日志（pi-mono）

要捕获**被解析为块之前的原始 OpenAI 兼容 chunk**，pi-mono 暴露了一个独立的日志记录器：

```bash
PI_RAW_STREAM=1
```

可选路径：

```bash
PI_RAW_STREAM_PATH=~/.pi-mono/logs/raw-openai-completions.jsonl
```

默认文件：

`~/.pi-mono/logs/raw-openai-completions.jsonl`

> 注意：此功能仅在使用 pi-mono 的 `openai-completions` Provider 的进程中生效。

## 安全提示

- 原始流日志可能包含完整的提示词、工具输出和用户数据。
- 请将日志保存在本地，调试后删除。
- 如需分享日志，请先清除密钥和个人身份信息。
