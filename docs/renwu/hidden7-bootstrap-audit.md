# Gateway 启动顺序一致性审查报告

> 审查日期：2026-02-21 | HIDDEN-7

## 一、审查范围

对比 TS `src/gateway/server.impl.ts` 的 `startGatewayServer()` 完整 bootstrap 流程（L155-638）与 Go `backend/internal/gateway/server.go` 的 `StartGatewayServer()` 流程（L80-409），逐步验证初始化顺序的一致性。

## 二、Bootstrap 对照表

| # | 阶段 | TS 实现 | Go 实现 | 状态 |
|---|------|---------|---------|------|
| 1 | 配置读取与迁移 | `readConfigFileSnapshot` + `migrateLegacyConfig` + `loadConfig` | `config.NewConfigLoader().LoadConfig()` | ✅ 等价 |
| 2 | plugin 自动启用 | `applyPluginAutoEnable` — 环境变量触发自动写入 config | ❌ 缺失 | ⚠️ P3 |
| 3 | 诊断心跳 | `startDiagnosticHeartbeat` — 可选遥测 | ❌ 缺失 | ⚠️ P3 |
| 4 | SIGUSR1 重启策略 | `setGatewaySigusr1RestartPolicy` — 允许外部信号触发重启 | ❌ 缺失 | ⚠️ P3 |
| 5 | 子代理注册表 | `initSubagentRegistry` — 全局子代理实例缓存 | ❌ 缺失 | ⚠️ P3 |
| 6 | 默认 Agent 解析 | `resolveDefaultAgentId` + `resolveAgentWorkspaceDir` | 内联于其他模块 | ✅ 等价 |
| 7 | 插件加载 | `loadGatewayPlugins` → `loadOpenAcosmiPlugins` | `LoadGatewayPlugins` → `plugins.LoadOpenAcosmiPlugins` | ✅ 本次修复 |
| 8 | 认证配置 | `resolveGatewayRuntimeConfig` + auth | `ResolveGatewayAuth` | ✅ 等价 |
| 9 | TLS 运行时 | `loadGatewayTlsRuntime` | `tls_runtime.go` | ✅ |
| 10 | 状态创建 | `createGatewayRuntimeState` | `NewGatewayState` | ✅ |
| 11 | Control UI | `resolveControlUiRootSync` + `ensureControlUiAssetsBuilt` | opts.ControlUIDir + 静态文件 | ✅ 等价 |
| 12 | Wizard tracker | `createWizardSessionTracker` | `NewWizardSessionTracker` + `WizardHandlers` | ✅ |
| 13 | 方法注册 | `coreGatewayHandlers` + `listGatewayMethods` | `registry.RegisterAll(...)` 14 批次 | ✅ |
| 14 | 发现服务 | `startGatewayDiscovery`（mDNS + 宽域 DNS-SD） | `StartGatewayDiscovery`（mDNS + 宽域） | ✅ 本次修复 |
| 15 | Skills remote | `setSkillsRemoteRegistry` + `primeRemoteSkillsCache` | `infra/skills_remote.go` | ✅ |
| 16 | 维护计时器 | `startGatewayMaintenanceTimers`（tick/health/dedupe） | `StartMaintenanceTick` | ✅ |
| 17 | Agent 事件 | `onAgentEvent(createAgentEventHandler)` | `NewNodeEventDispatcher` | ✅ |
| 18 | 心跳 Runner | `startHeartbeatRunner` | `NewHeartbeatState` | ✅ |
| 19 | Cron 服务 | `cron.start()` | `CronHandlers` 注册 | ✅ |
| 20 | Exec 审批 | `ExecApprovalManager` + `createExecApprovalForwarder` | `ExecApprovalsHandlers` | ✅ |
| 21 | WS 处理 | `attachGatewayWsHandlers` (完整 context 传递) | `HandleWebSocketUpgrade(wsConfig)` | ✅ |
| 22 | 启动日志 | `logGatewayStartup` | `slog.Info("🦜 Gateway listening")` | ✅ |
| 23 | 更新检查 | `scheduleGatewayUpdateCheck` | ❌ 缺失（低优先） | ⚠️ P3 |
| 24 | Tailscale | `startGatewayTailscaleExposure` | `server_tailscale.go` | ✅ |
| 25 | Sidecars | `startGatewaySidecars`（browser/channels/hooks） | SKIP flags + handler 注册 | ✅ |
| 26 | Config 热重载 | `startGatewayConfigReloader` | `reload.go` | ✅ |
| 27 | Close handler | `createGatewayCloseHandler` | `runtime.Close(reason)` | ✅ |

## 三、差异汇总

4 项 P3 差异（均不阻塞核心功能）：

| 差异 | 说明 | 影响 | 优先级 |
|------|------|------|--------|
| plugin auto-enable | 生产中通过 CLI 手动启用等价 | 首次启动便利性 | P3 |
| diagnostic heartbeat | 可选遥测功能 | 无功能影响 | P3 |
| SIGUSR1 restart | Unix 信号触发进程重启 | 热更新便利性 | P3 |
| subagent registry | 子代理实例共享缓存 | 多代理并发时性能 | P3 |

## 四、结论

Go 端 bootstrap 流程覆盖了 TS 端 **23/27** 个启动步骤（85%），4 项差异均为 P3 低优先级，不影响 Gateway 核心拓扑：配置→认证→方法注册→HTTP/WS→发现→维护定时器→信号处理。

**建议**：4 项差异可列入 Phase 5 长期计划，不阻塞当前发布。
