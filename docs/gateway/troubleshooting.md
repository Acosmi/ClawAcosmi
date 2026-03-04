---
summary: "Gateway 深度故障排除手册：频道、自动化、节点、浏览器"
read_when:
  - 故障排除中心引导你来此做深度诊断
  - 需要基于症状的稳定排查步骤
title: "故障排除"
---

# Gateway 故障排除

> [!IMPORTANT]
> **架构状态**：所有故障排除命令由 **Go CLI** 和 **Go Gateway** 实现。

深度排查手册。快速分诊请先看 [/help/troubleshooting](/help/troubleshooting)。

## 命令阶梯

按顺序运行：

```bash
openacosmi status
openacosmi gateway status
openacosmi logs --follow
openacosmi doctor
openacosmi channels status --probe
```

健康信号：`gateway status` 显示 `Runtime: running` + `RPC probe: ok`。

## 无回复

频道正常但无应答 — 检查路由和策略：

```bash
openacosmi channels status --probe
openacosmi pairing list <channel>
openacosmi config get channels
```

常见特征：`mention required`（群组提及规则）、`pairing request`（需审批）、`blocked`（过滤策略）。

## 控制 UI 连接问题

```bash
openacosmi gateway status
openacosmi gateway status --json
```

检查：URL 正确性、认证模式/token 匹配、设备身份要求。

## Gateway 服务未运行

```bash
openacosmi gateway status --deep
openacosmi doctor
```

常见特征：`set gateway.mode=local`、`refusing to bind ... without auth`、`EADDRINUSE`。

## 频道已连接但消息不流动

```bash
openacosmi channels status --probe
openacosmi pairing list <channel>
```

检查：DM 策略、群组白名单、频道 API 权限。

## 定时任务和心跳投递

```bash
openacosmi cron status
openacosmi system heartbeat last
```

常见特征：`scheduler disabled`、`heartbeat skipped` + `quiet-hours`。

## 节点工具失败

```bash
openacosmi nodes status
openacosmi nodes describe --node <id>
```

常见特征：`NODE_BACKGROUND_UNAVAILABLE`、`PERMISSION_REQUIRED`、`SYSTEM_RUN_DENIED`。

## 浏览器工具失败

```bash
openacosmi browser status
openacosmi browser profiles
```

常见特征：`Failed to start Chrome CDP`、`executablePath not found`。

## 升级后故障

1. **认证和 URL 覆盖行为变更**：检查 `gateway.mode`、`gateway.remote.url`。
2. **绑定和认证更严格**：非 loopback 绑定需配置认证。
3. **配对和设备身份变更**：检查待处理审批。

修复：

```bash
openacosmi gateway install --force
openacosmi gateway restart
```

相关：[配对](/gateway/pairing) · [认证](/gateway/authentication)
