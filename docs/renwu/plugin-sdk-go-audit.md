# Phase 1: Plugin SDK — Go 实现审计

## 总览

- Go 目录: `backend/internal/plugins/` — **19 文件, 4835 行**
- TS 参考: `src/plugins/` — 35 文件, 5780 行
- 覆盖率: **~84%**

## 文件映射

| Go 文件 | 行数 | TS 参考 | 已实现逻辑 |
|---------|------|---------|-----------|
| `types.go` | 394 | `types.ts` | 94 个类型: PluginLogger/Kind/Origin/ConfigUiHint/ConfigValidation/ToolContext/HookOptions/ProviderPlugin/CommandDefinition/Service 等 |
| `manifest.go` | 201 | `manifest.ts` | 解析 `openacosmi.plugin.json`, 含 PluginManifest/PackageChannel/PackageInstall 类型 |
| `registry.go` | 553 | `registry.ts` | PluginRegistry 全局注册表, RegisterTool/Hook/GatewayMethod/Channel/Provider/Command/Service, CreateAPI |
| `loader.go` | 334 | `loader.ts` | LoadOpenAcosmiPlugins 主入口, 缓存/规范化/启用状态/内置插件注册/配置校验 |
| `discovery.go` | 523 | `discovery.ts` | DiscoverPlugins 扫描: config→workspace→global→bundled, package.json 解析, ID推导 |
| `install.go` | 576 | `install.ts` | 5 种安装方式: FromNpmSpec/FromDir/FromFile/FromPath/FromPackageDir, ID校验/路径安全 |
| `install_helpers.go` | 150 | `install.ts` | 安装辅助: 归档解压/临时目录/npm pack |
| `update.go` | 543 | `update.ts` | UpdateNpmInstalledPlugins/SyncPluginsForUpdateChannel/RecordPluginInstall |
| `commands.go` | 267 | `commands.ts` | 命令注册/校验/匹配/执行, 保留命令表, 参数清理 |
| `config_state.go` | 196 | `config-state.ts` | NormalizePluginsConfig/ResolvePluginEnabled/EnablePlugin/DisablePlugin |
| `slots.go` | 164 | `slots.ts` | 互斥槽位管理: SlotKeyForPluginKind/ApplyExclusiveSlotSelection |
| `schema.go` | 123 | `schema-validator.ts` | JSON Schema 校验适配 |
| `runtime.go` | 92 | `runtime.ts` | 活跃注册表/运行时状态管理 |
| `runtime_state.go` | 50 | `runtime.ts` | 运行时状态序列化 |
| `plugin_api.go` | 83 | `types.ts` | PluginAPI 结构 + PluginRuntime 接口 + NullRuntime |
| `http_path.go` | 18 | `http-path.ts` | 插件 HTTP 路径前缀常量 |
| `commands_test.go` | 150 | - | 命令注册/匹配/校验测试 |
| `config_state_test.go` | 115 | - | 配置状态规范化测试 |
| `registry_test.go` | 110 | - | 注册表创建/工具注册测试 |

## 外围集成文件

| Go 文件路径 | 行数 | 逻辑 |
|------------|------|------|
| `cmd/openacosmi/cmd_plugins.go` | 52 | CLI: `oa plugins` 子命令入口 |
| `internal/cli/plugin_registry.go` | — | CLI 启动时加载插件注册表 |
| `internal/gateway/server_plugins.go` | 152 | Gateway 加载插件 HTTP 方法 |
| `internal/hooks/plugin_hooks.go` | ~130 | 插件钩子执行桥接 |
| `internal/config/plugin_auto_enable.go` | — | 新安装插件自动启用 |
| `internal/autoreply/commands_handler_plugin.go` | — | 聊天命令→插件命令路由 |
| `internal/agents/skills/plugin_skills.go` | — | 插件技能注册 |
| `pkg/types/types_plugins.go` | 40 | 配置类型定义 |
| `pkg/contracts/channel_plugin.go` | — | 频道插件契约接口 |
