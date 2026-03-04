# plugins/ 架构文档

> 最后更新：2026-02-26 | 代码级审计确认 | 16 源文件, 30 测试

## 一、模块概述

`internal/plugins/` 负责 OpenAcosmi 插件系统的完整生命周期：类型定义、注册表、发现、安装/更新、运行时、加载。插件通过编译时注册（`RegisterBuiltinPlugin`）或未来 WASM/gRPC 边界加载。

## 二、原版实现（TypeScript）

### 源文件列表

| 文件 | 大小 | 职责 |
|------|------|------|
| `types.ts` | 8KB | 插件核心类型 |
| `registry.ts` | 12KB | 注册表 + Register* 方法 |
| `discovery.ts` | 10KB | 多源插件扫描 |
| `install.ts` | 17KB | 多源安装 |
| `update.ts` | 12KB | npm 批量更新 + 频道同步 |
| `loader.ts` | 14KB | 动态模块加载 + 配置校验 |
| `runtime/index.ts` | 13KB | 100+ 内部函数引用中心 |
| `runtime/types.ts` | 19KB | 127+ 函数类型别名 |
| `commands.ts` | 8KB | 命令注册 |
| `schema-validator.ts` | 5KB | JSON Schema 校验 |

### 核心逻辑摘要

TS 插件系统基于 Node.js 动态模块加载（`jiti`），runtime 对象暴露 127+ 内部函数引用供 JS 插件调用。

## 三、依赖分析

### 显式依赖图

| 依赖模块 | 类型 | 方向 | 用途 |
|----------|------|------|------|
| `pkg/types` | 值 | ↓ | PluginsConfig, PluginEntryConfig |
| `pkg/contracts` | 类型 | ↑ | PluginRegistry 接口 |
| `internal/hooks` | 值 | ↔ | 钩子桥接 |

### 隐藏依赖审计

| 类别 | 结果 | Go 等价方案 |
|------|------|-------------|
| npm 包黑盒行为 | ⚠️ jiti 动态加载 | 编译时 `RegisterBuiltinPlugin` 注册 |
| 全局状态/单例 | ⚠️ registryCache, activeRegistry | `sync.RWMutex` 保护 + `runtime_state.go` |
| 事件总线/回调链 | ✅ | — |
| 环境变量依赖 | ⚠️ OPENACOSMI_VERSION, OPENACOSMI_CONFIG_DIR | `os.Getenv` 读取 |
| 文件系统约定 | ⚠️ bundled/global/config 目录搜索 | `discovery.go` 多路径扫描 |
| 协议/消息格式 | ✅ | — |
| 错误处理约定 | ✅ | 函数返回 `Result` 结构体 |

## 四、重构实现（Go）

### 文件结构

| 文件 | 行数 | 对应原版 | 批次 |
|------|------|----------|------|
| `types.go` | 394 | types.ts | A2 |
| `plugin_api.go` | 83 | types.ts (PluginAPI) | A2 |
| `registry.go` | 540 | registry.ts | A2 |
| `config_state.go` | 241 | config-state.ts | A2 |
| `slots.go` | 160 | slots.ts | A2 |
| `commands.go` | 200 | commands.ts | A2 |
| `manifest.go` | 201 | manifest.ts | A2 |
| `schema.go` | 180 | schema-validator.ts | A2 |
| `http_path.go` | 50 | http-path.ts | A2 |
| `runtime_state.go` | 45 | runtime.ts (状态) | A2 |
| `discovery.go` | 480 | discovery.ts | B1 |
| `install.go` | 390 | install.ts | B1 |
| `install_helpers.go` | 175 | install.ts (辅助) | B1 |
| `update.go` | 375 | update.ts | B1 |
| `runtime.go` | 110 | runtime/index.ts | B3 |
| `loader.go` | 280 | loader.ts | B3 |

### 接口定义

```go
// 核心接口
type PluginRuntime interface {
    Version() string
    GetLogger(bindings map[string]interface{}) PluginLogger
}

// 默认实现
type DefaultPluginRuntime struct { ... }

// 注册表
type PluginRegistry struct {
    Plugins    []PluginRecord
    Tools      []PluginToolRegistration
    Hooks      []PluginHookRegistrationEntry
    // ... 10+ 注册类别
}

// 编译时注册
func RegisterBuiltinPlugin(id string, fn BuiltinPluginRegistrar)
```

## 五、差异对照

| 维度 | 原版 TS | 重构 Go |
|------|---------|---------|
| 模块加载 | `jiti` 动态 require | 编译时 `RegisterBuiltinPlugin` |
| 运行时 | 127+ 函数引用 DI 容器 | 仅 Version()+GetLogger()；内部包直接 import |
| npm 集成 | 原生 npm CLI | `os/exec` 封装 npm pack/install |
| 缓存 | `Map<string, Registry>` | `sync.RWMutex` + `map[string]*PluginRegistry` |

## 六、Rust 下沉候选

| 函数/模块 | 优先级 | 原因 |
|-----------|--------|------|
| (无) | — | 插件系统以 I/O 和配置为主，无计算密集路径 |

## 七、测试覆盖

| 测试类型 | 覆盖范围 | 状态 |
|----------|----------|------|
| 单元测试 | registry, commands, slots | ✅ 30 PASS |
| 编译验证 | go build + go vet | ✅ clean |
| 竞态检测 | go test -race | ✅ clean |
