# Docker Sandbox 架构文档

> 最后更新：2026-02-26 | 代码级审计确认 | 7 源文件, 22 测试

## 一、模块概述

Docker Sandbox 模块负责管理 Agent 执行环境的容器化隔离。它提供容器生命周期管理（创建、启动、停止、清理）、配置哈希化、注册表持久化和运行时上下文（工具策略、状态查询）。

位于 `internal/agents/sandbox/`，被 `agents/runner/` 和 `agents/bash/` 消费。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件 | 大小 | 职责 |
|------|------|------|
| `docker/sandbox.ts` | ~12KB | 容器管理 + 生命周期 |
| `docker/types.ts` | ~3KB | 类型定义 |
| `docker/config.ts` | ~4KB | 配置解析 + hash |
| `docker/registry.ts` | ~3KB | JSON 注册表 |
| `docker/context.ts` | ~5KB | 运行时上下文 |

## 三、重构实现（Go）

### 文件结构

| 文件 | 行数 | 对应原版 |
|------|------|----------|
| `types.go` | ~80 | `types.ts` |
| `config.go` | ~120 | `config.ts` |
| `docker.go` | ~150 | `sandbox.ts` (Docker CLI) |
| `registry.go` | ~90 | `registry.ts` |
| `manage.go` | ~140 | `sandbox.ts` (生命周期) |
| `context.go` | ~100 | `context.ts` |

### 关键接口

- `SandboxConfig` — 容器配置（镜像/挂载/端口/环境变量）
- `ContainerRegistry` — JSON 文件注册表（path → containerID 映射）
- `SandboxContext` — 运行时状态 + 工具策略查询

### 隐藏依赖审计

| 类别 | 结果 | Go 等价方案 |
|------|------|-------------|
| npm 包黑盒行为 | ✅ | 无 npm 依赖，直接调用 Docker CLI |
| 全局状态/单例 | ✅ | 无全局状态 |
| 环境变量依赖 | ⚠️ | `DOCKER_HOST` 透传 |
| 文件系统约定 | ⚠️ | registry.json 路径约定已对齐 |
| 错误处理约定 | ✅ | Go error 返回 |

## 四、测试覆盖

| 测试类型 | 状态 | 说明 |
|----------|------|------|
| 编译验证 | ✅ | `go build` 通过 |
| 静态分析 | ✅ | `go vet` 通过 |
| 单元测试 | ⏳ | 延迟 BW1-D1（需 mock Docker CLI） |
