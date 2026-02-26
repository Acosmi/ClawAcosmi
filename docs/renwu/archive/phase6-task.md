# Phase 6 任务清单 — CLI + 插件 + 钩子 + 守护进程

> 上下文：[phase6-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase6-bootstrap.md)
> 深度审计：[phase6-deep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase6-deep-audit.md)
> 延迟项：[deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)
> 最后更新：2026-02-14（C2 审计项修复 F1/F2/F3 完成，26 测试 PASS）

---

## 批次 A：无/低外部依赖（可立即开工）

### A1: 6.7 daemon/ 核心（105KB → `internal/daemon/`）✅

- [x] 类型+常量 → `daemon/types.go`, `daemon/constants.go`
- [x] 路径解析 → `daemon/paths.go`, `daemon/runtime_paths.go`
- [x] 服务环境 → `daemon/service_env.go`
- [x] 程序参数 → `daemon/program_args.go`
- [x] macOS launchd → `daemon/launchd_darwin.go`, `daemon/plist_darwin.go`
- [x] Linux systemd → `daemon/systemd_linux.go`
- [x] Windows schtasks → `daemon/schtasks_windows.go`
- [x] 服务主入口 → `daemon/service.go` + `platform_{darwin,linux,windows}.go`
- [x] 状态检查 → `daemon/inspect.go`
- [x] 配置审计 → `daemon/audit.go`
- [x] 诊断 → `daemon/diagnostics.go`
- [x] 节点服务 → `daemon/node_service.go`
- [x] 测试: 21 PASS (`go test -race`)
- [x] 编译验证: `go build ./...` + `go vet` clean

### A2: 6.4a plugins/ 类型+注册表（~45KB → `internal/plugins/`）✅

- [x] 核心类型 → `plugins/types.go`（30+ struct/const）
- [x] 插件 API → `plugins/plugin_api.go`（PluginAPI + PluginRuntime 接口）
- [x] 插件注册表 → `plugins/registry.go`（Registry + 所有 Register* 方法）
- [x] HTTP 路径 → `plugins/http_path.go`
- [x] 运行时状态 → `plugins/runtime_state.go`（全局单例）
- [x] Manifest 解析 → `plugins/manifest.go`
- [x] Schema 校验 → `plugins/schema.go`（内建 JSON Schema）
- [x] 配置状态 → `plugins/config_state.go`
- [x] 槽位管理 → `plugins/slots.go`
- [x] 命令注册 → `plugins/commands.go`（保留命令集 + 并发安全）
- [x] 测试: 30 PASS (`go test -race`)
- [x] 编译验证: `go build ./...` + `go vet` clean

### A3: 6.5a hooks/ 核心+加载（~35KB → `internal/hooks/` 扩展）✅

- [x] hook_types.go — Hook/HookEntry/HookMetadata/InternalHookEvent 等类型
- [x] internal_hooks.go — 事件 handler 注册/触发 + sync.RWMutex
- [x] frontmatter.go — HOOK.md frontmatter 解析 + metadata 提取
- [x] hook_config.go — eligibility 检查 + binary 探测 + 配置路径解析
- [x] workspace.go — 目录扫描 + hook entry 加载 + snapshot
- [x] status.go — 状态报告
- [x] bundled_dir.go — bundled hooks 目录解析
- [x] bundled_handlers.go — 4 个 bundled handler 静态注册骨架
- [x] loader.go — hook 加载编排
- [x] 测试: 35 PASS (`go test -race`)，含 8 新 A3 测试
- [x] 编译验证: `go build ./...` + `go vet` clean

---

## 批次 B：中等依赖 ✅

### B1: 6.4b plugins/ 发现+安装+更新（~40KB）✅

- [x] 插件发现 → `plugins/discovery.go`（~480L，对应 discovery.ts 10KB）
- [x] 插件安装 → `plugins/install.go`（~390L）+ `install_helpers.go`（~175L）
- [x] 插件更新 → `plugins/update.go`（~375L，含 UpdateChannel/PluginUpdateSummary 等类型）
- [x] bundled 目录 → 已并入 `plugins/discovery.go` 的 `ResolveBundledPluginsDir`
- [x] 编译验证: `go build ./...` + `go vet` + `go test -race` clean

### B2: 6.5b hooks/ Gmail 集成（~38KB）✅

- [x] Gmail 主入口 → `hooks/gmail/gmail.go`（~340L，配置解析+CLI 参数构建）
- [x] Gmail API 操作 → `hooks/gmail/ops.go`（~195L，setup 向导+服务运行）
- [x] Gmail 设置 → `hooks/gmail/setup.go`（~200L，gcloud/tailscale CLI 封装）
- [x] Gmail 推送 → `hooks/gmail/watcher.go`（~230L，进程生命周期+自动重启+定期续期）
- [x] Soul-evil 钩子 → `hooks/soul_evil.go`（~300L，purge 窗口+概率覆盖+SOUL.md 替换）
- [x] Go 依赖: 无额外依赖（通过 `gog` CLI 交互，未直接引入 Gmail API SDK）
- [x] 编译验证: `go build ./...` + `go vet` + `go test -race` clean

### B3: 6.4c plugins/ 运行时+桥接（~55KB）✅

- [x] 运行时核心 → `plugins/runtime.go`（~110L，DefaultPluginRuntime + 版本解析）
- [x] 插件加载器 → `plugins/loader.go`（~280L，发现→配置→注册 + 缓存）
- [x] 注册桥接 → `RegisterBuiltinPlugin()` 编译时注册机制（替代 TS 动态模块加载）
- [x] 命令注册 → 已在 A2 完成 `plugins/commands.go`
- [x] 频道适配器绑定 → 通过 `pkg/contracts/` 接口（已在 A2 完成）
- [x] 编译验证: `go build ./...` + `go vet` + `go test -race` clean

---

## 批次 C：依赖 Phase 4

### C1: 6.6a cron/ 服务层（~70KB → `internal/cron/`）✅

- [x] 类型定义 → `cron/types.go`（对应 types.ts 2.5KB）
- [x] Cron 表达式解析 → `cron/normalize.go`（对应 normalize.ts 13KB）
- [x] 调度计算 → `cron/schedule.go`
- [x] 持久化 → `cron/store.go`（对应 service/store.ts 15KB）
- [x] 定时器 → `cron/timer.go`（对应 service/timer.ts 16KB）
- [x] Job 管理 → `cron/jobs.go`（对应 service/jobs.ts 12KB）
- [x] CRUD 操作 → `cron/ops.go`
- [x] 运行日志 → `cron/run_log.go`
- [x] Go 依赖: `robfig/cron/v3`
- [x] 编译验证

> **注**: 17 个 Go 文件已在先前阶段完成，此处为状态订正。

### C2: 6.6b cron/ 独立 Agent 运行（~46KB）✅

> **包结构**: 扁平 `internal/cron/`（`isolated_agent.go` + `isolated_agent_helpers.go`），不创建 `cron/agent/` 子包

- [x] Agent 运行器 → `cron/isolated_agent.go`（596L, 对应 isolated-agent/run.ts 597L）
  - `RunCronIsolatedAgentTurn` 主编排函数（11 步流程）
  - `IsolatedAgentDeps` DI 接口（17 个依赖）
  - `NewRunIsolatedAgentJobFunc` 桥接 `CronServiceDeps`
- [x] 辅助函数 → `cron/isolated_agent_helpers.go`（226L）
  - session 创建、心跳过滤、hook 检测、payload 选择
- [x] 投递计划升级 → `cron/delivery.go` 升级到完整 TS 对等（+Source/Requested/legacy mode）
- [x] 隐藏依赖审计（7 类 / 12 项发现）
- [x] 编译验证: `go build ./...` + `go vet` clean
- [x] 文档更新: phase6-task.md + deferred-items.md + docs/gouji/cron.md

#### C2 审计项修复补全（2026-02-14）

- [x] **F1**: `buildSafeExternalPrompt` 完整移植 → `internal/security/external_content.go`（345L）
  - 12 regex prompt injection 检测 + Unicode 全角折叠 + 标记净化
  - 替换 `isolated_agent.go` 简化边界标记
- [x] **F2**: Model override 接入 `ResolveAllowedModelRef` DI（含兜底切割）
- [x] **F3**: `convertMarkdownTables` → `pkg/markdown/tables.go`（250L, 3 模式: off/bullets/code）
  - 替换 iMessage `send.go` + Discord `send_guild.go` TODO
  - 添加 `Markdown *MarkdownConfig` 字段到 `OpenAcosmiConfig`
- [x] 单元测试: `internal/security/` 17 PASS + `pkg/markdown/` 9 PASS = 26 PASS
- [x] 编译验证: `go build ./...` + `go vet` clean

---

## 批次 D：依赖 Gateway

### D1: 6.8 ACP 协议（36KB → `internal/acp/`）✅

- [x] 类型定义 → `acp/types.go` (290L, ACP 协议类型 + SDK 等价类型)
- [x] 协议翻译 → `acp/translator.go`（330L, ACP ⇔ Gateway 翻译器）
- [x] 客户端 → `acp/client.go` (230L, 子进程 + ndJSON 双向通信)
- [x] 服务端 → `acp/server.go` (250L, stdin/stdout ndJSON 请求分发)
- [x] 会话映射 → `acp/session.go` (142L, 内存 store + 三 map 索引)
- [x] 会话解析 → `acp/session_mapper.go` (115L, session key 解析 + Gateway RPC)
- [x] 事件映射 → `acp/event_mapper.go` (100L, 文本/附件提取 + 工具推断)
- [x] 元数据 → `acp/meta.go` (95L, ReadString/ReadBool/ReadNumber)
- [x] 命令 → `acp/commands.go` (35L, 26 个可用命令)
- [x] 单元测试 → `acp/acp_test.go` (250L, 11 test cases)
- [x] 编译验证: `go build ./...` + `go vet` + `go test -race` ✅

---

## 批次 E：CLI（全量 80+ 命令）

### E1: 6.1 CLI 框架（~50KB → `cmd/openacosmi/` + `internal/cli/`）✅

- [x] Cobra 主程序 → `cmd/openacosmi/main.go` (92L)
- [x] CLI 工具函数 → `internal/cli/utils.go` (83L)
- [x] 进度显示 → `internal/cli/progress.go` (132L)
- [x] 参数解析 → `internal/cli/argv.go` (125L)
- [x] 版本/Banner → `internal/cli/version.go` (48L) + `internal/cli/banner.go` (68L)
- [x] Gateway RPC → `internal/cli/gateway_rpc.go` (75L)
- [x] 编译验证: `go build ./...` + `go vet ./...` ✅

### E2: 6.2 核心命令（stub 已创建，业务逻辑待填充）✅

- [x] gateway 命令组 → `cmd/openacosmi/cmd_gateway.go` (80L, 4 sub-cmds)
- [x] agent 命令 → `cmd/openacosmi/cmd_agent.go` (112L, 6 sub-cmds)
- [x] status 命令 → `cmd/openacosmi/cmd_status.go` (33L)
- [x] setup/onboard → `cmd/openacosmi/cmd_setup.go` (37L)
- [x] models 命令组 → `cmd/openacosmi/cmd_models.go` (61L, 3 sub-cmds)
- [x] channels 命令 → `cmd/openacosmi/cmd_channels.go` (76L, 4 sub-cmds)
- [x] daemon 命令 → `cmd/openacosmi/cmd_daemon.go` (52L, 4 sub-cmds)
- [x] cron 命令 → `cmd/openacosmi/cmd_cron.go` (53L, 4 sub-cmds)
- [x] 编译验证 ✅

### E3: 6.3 辅助命令+完整 CLI（stub 已创建，业务逻辑待填充）✅

- [x] doctor 命令组 → `cmd/openacosmi/cmd_doctor.go` (26L)
- [x] skills 命令 → `cmd/openacosmi/cmd_skills.go` (43L, 3 sub-cmds)
- [x] hooks 命令 → `cmd/openacosmi/cmd_hooks.go` (36L, 2 sub-cmds)
- [x] plugins 命令 → `cmd/openacosmi/cmd_plugins.go` (51L, 4 sub-cmds)
- [x] browser 命令组 → `cmd/openacosmi/cmd_browser.go` (35L, 2 sub-cmds)
- [x] nodes 命令 → `cmd/openacosmi/cmd_nodes.go` (56L, 4 sub-cmds)
- [x] pairing/dns/ports → `cmd/openacosmi/cmd_infra.go` (88L)
- [x] security/update → `cmd/openacosmi/cmd_security.go` (41L)
- [x] acp/logs/docs/memory/misc → `cmd/openacosmi/cmd_misc.go` (259L, 14 sub-cmds)
- [x] 编译验证 ✅

---

## Phase 5A 遗留补全 ✅

> 隐藏依赖审计：[phase5a-hidden-dep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5a-hidden-dep-audit.md)

- [x] P2-D2: HookMappingConfig 嵌套 match → `Match *HookMatchFieldConfig` + `resolveMatchField`
- [x] P2-D5: Channel 动态插件注册 → 扩展 `validHookChannels` (11 entries) + `RegisterHookChannel()`
- [x] P3-D1: sessions.list Path 字段 → `ctx.Context.StorePath`
- [x] P3-D2: sessions.list Defaults 填充 → `getSessionDefaults()` + `models.ResolveConfiguredModelRef`
- [x] P3-D3: sessions.delete 主 session 保护 → `routing.BuildAgentMainSessionKey` 动态保护
- [x] P4-DRIFT4: API Key 环境变量映射 → 31 entries + `EnvApiKeyFallbacks` (5) + `ResolveEnvApiKeyWithFallback()`
- [x] P4-NEW5: 隐式供应商自动发现 → `implicit_providers.go` **[NEW]** (12 specs + Bedrock/Copilot stub)
- [x] P2-D1: Transform 管道 → `TransformFunc` / `RegisterTransform()` / `ApplyTransform()` (Override/Merge/Skip)

---

## 验证

- [x] 完整编译: `go build ./...`
- [x] 静态分析: `go vet ./...`
- [x] 单元测试: `go test -race ./internal/gateway/... ./internal/agents/models/...`
- [ ] 架构文档: `docs/gouji/` 各模块
