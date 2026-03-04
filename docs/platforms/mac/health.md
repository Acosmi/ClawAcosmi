---
summary: "macOS 应用健康检查和 Gateway 连接状态"
read_when:
  - 调试 macOS 应用的 Gateway 连接
title: "健康检查"
---

# 健康检查（macOS 应用）

macOS 应用定期对 Go Gateway 进行健康检查以维护连接状态。

## 检查机制

- 应用通过 WebSocket 发送 `health` 请求。
- 响应包含 Gateway 版本、运行时间和通道状态。
- 连接断开时显示菜单栏状态指示器。

## CLI 验证

```bash
openacosmi gateway call health --timeout 3000
openacosmi gateway status
openacosmi status
```

## 故障排除

- 检查 Gateway 是否运行：`systemctl --user status openacosmi-gateway`（Linux）或 `launchctl list | grep molt`（macOS）
- 检查端口占用：`lsof -i :18789`
- 查看日志：`openacosmi logs --follow`
