# iMessage SDK 架构文档

> 最后更新：2026-02-26 | 代码级审计确认 | 13 源文件, ~2,982 行

## 一、模块概述

iMessage SDK 通过 JSON-RPC over stdio 与 `imsg rpc` CLI 子进程通信，提供 iMessage 消息的发送、接收监控和账户管理功能。位于 `backend/internal/channels/imessage/`。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件 | 行数 | 职责 |
|------|------|------|
| `targets.ts` | 234 | handle/target 规范化（电话号码/邮箱/群组解析） |
| `accounts.ts` | 91 | 多账户解析 + 配置合并 |
| `client.ts` | 245 | JSON-RPC 客户端（子进程管理 + 请求/响应匹配） |
| `probe.ts` | 107 | CLI 可用性探测 + RPC 支持检测 |
| `send.ts` | 141 | 消息发送（文本 + 媒体附件） |
| `constants.ts` | 3 | 常量定义 |
| `monitor/types.ts` | 41 | 入站消息载荷类型 |
| `monitor/runtime.ts` | 19 | 运行时环境 + allowList 规范化 |
| `monitor/deliver.ts` | 70 | 回复投递（分块 + markdown 表格转换） |
| `monitor/monitor-provider.ts` | 750 | 核心入站监控（25+ import 依赖） |

## 三、依赖分析

### 隐藏依赖审计

| 类别 | 结果 | 详情 |
|------|------|------|
| npm 包黑盒行为 | ✅ | 无第三方 npm 依赖 |
| 全局状态/单例 | ⚠️ H2 | probe cache 无 TTL（与 TS 一致） |
| 事件总线/回调链 | ⚠️ H3/H4 | 防抖器(Phase 6 桩)、回声检测(✅已修复) |
| 环境变量依赖 | ✅ | 仅 HOME 用于路径展开 |
| 文件系统约定 | ⚠️ H5/H6 | outbound/ 目录 + session store(Phase 6/7 桩) |
| 协议/消息格式 | ⚠️ H7/H8/H9 | markdown 表格、subscription ID(✅已修复)、文本分块(Phase 7) |
| 错误处理约定 | ⚠️ H10/H11 | 媒体日志(✅已修复)、优雅关闭(✅已修复) |

## 四、重构实现（Go）

### 文件结构

| 文件 | 行数 | 对应原版 |
|------|------|----------|
| `constants.go` | 6 | `constants.ts` |
| `targets.go` | 318 | `targets.ts` |
| `accounts.go` | 242 | `accounts.ts` |
| `client.go` | 360 | `client.ts` |
| `probe.go` | 138 | `probe.ts` |
| `send.go` | 246 | `send.ts` |
| `monitor_types.go` | 49 | `monitor/types.ts` |
| `monitor.go` | 400 | `monitor-provider.ts` + `deliver.ts` + `runtime.ts` |

### 核心接口

- `IMessageRpcClient` — JSON-RPC 客户端（管理 imsg rpc 子进程，H11 优雅关闭已实现）
- `SentMessageCache` — 发送消息缓存（用于 H4 回声检测）
- `MonitorIMessage()` — 入站监控入口（H1 watch.unsubscribe 已实现）
- `SendMessageIMessage()` — 消息发送（H10 LogError 回调已实现）

## 五、延迟项

| 编号 | Phase | 描述 |
|------|-------|------|
| IM-A | 6 | 入站消息分发管线（13 项依赖：防抖、群组策略、mention、命令门控、pairing store 等） |
| IM-B | 6 | 配对请求管理 |
| IM-C | 7 | 媒体附件下载 + 存储 |
| IM-D | 7 | Markdown 表格转换 + 文本分块 |
| IM-E | — | ✅ 已修复（H1/H8/H11 优雅关闭 + 订阅清理） |

## 六、Rust 下沉候选

无 — iMessage SDK 为 I/O 绑定（子进程通信），无计算密集型热点。
