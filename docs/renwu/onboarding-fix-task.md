# Onboarding 初始化引导 — 全量补齐修复跟踪

> 来源：2026-02-22 引导页审计 → [审计报告](./global-audit-onboarding.md)
> 评级：**C（需补全）** → 目标 **A**
> TS 总量：9,775L / 40 文件 → Go 现有：2,651L / 14 文件（覆盖率 27%）
> 预估工时：~32-40h，拆 4 窗口

---

## 窗口 1：P1 核心流程（~12h）

- [x] **OB-1** (P1)：Onboarding Finalization ✅ 2026-02-22
  - [x] 新建 `gateway/wizard_finalize.go` (340L)
  - [x] 新建 `gateway/wizard_helpers.go` (337L)
  - [x] 移植 `finalizeOnboardingWizard`（daemon 提示 + gateway probe + TUI/Web hatch）
  - [x] 移植 `probeGatewayReachable` / `waitForGatewayReachable`（HTTP health check + 轮询）
  - [x] 移植 `resolveControlUiLinks`（5 种 bind mode URL 生成）
  - [x] 移植 `RandomToken` / `NormalizeGatewayTokenInput` / browser open
  - ⚠️ daemon OS-specific 安装（plist/systemd）→ `TODO(OB-1-DEFERRED)`
  - [x] 测试：wizard_helpers_test.go 8/8 PASS

- [x] **OB-2** (P1)：Gateway Config ✅ 2026-02-22
  - [x] 新建 `gateway/wizard_gateway_config.go` (280L)
  - [x] 移植 `configureGatewayForOnboarding`（bind/port/auth/Tailscale/deny commands）
  - [x] quickstart + guided 双路分发
  - [x] Tailscale 安全约束（funnel→password, serve/funnel→loopback）
  - [x] `DEFAULT_DANGEROUS_NODE_DENY_COMMANDS` 6 条

- [x] **OB-3** (P1)：Auth Config Core ✅ 2026-02-22
  - [x] 新建 `cmd/openacosmi/setup_auth_config.go` (390L)
  - [x] 26 个 provider config 函数（helper 模式封装）
  - [x] `ApplyAuthProfileConfig` 多 profile 凭证管理
  - [x] 测试：setup_auth_config_test.go 10/10 PASS

---

## 窗口 2：P2 频道 + 凭证（~10h）

- [x] **OB-4** (P2)：Channels Setup — 交互式频道设置向导 ✅ 2026-02-22
  - [x] 新建 `cmd/openacosmi/setup_channels.go` (429L)
  - [x] 移植 `SetupChannels`（频道状态检查 + 选择 UI + 逐频道配置）
  - [x] 移植频道状态检测 `CollectChannelStatus`（9 频道 typed ChannelsConfig 适配）
  - [x] 移植 DM 策略配置 `MaybeConfigureDmPolicies`（allowlist/open/disabled）
  - [x] `HandleChannelChoice` 连接 typed ChannelsConfig 字段
  - [x] `HandleConfiguredChannel` + `disableChannel` + `deleteChannelConfig` (D1 补全)
  - [x] `NoteChannelPrimer` DM 安全入门说明 (D2 补全)
  - [x] 测试：setup_channels_test.go 23/23 PASS

- [x] **OB-5** (P2)：Onboard Helpers — 辅助设施 ✅ 2026-02-22
  - [x] 补全 `cmd/openacosmi/setup_helpers.go` → 300L (+65L)
  - [x] 新增 `DetectBinary`（exec.LookPath + 绝对路径检查）
  - [x] 新增 `MoveToTrash`（trash CLI + os.RemoveAll fallback）
  - [x] 新增 `GuardCancel` + `DefaultWorkspace` 常量
  - [x] ResetScope 3 scope 已完整（TS 只有 3 scope，非 6）
  - [x] gateway 包已实现 probe/wait/resolve — 不需重复
  - [x] 测试：setup_helpers_test.go 20/20 PASS

- [x] **OB-6** (P2)：Non-Interactive Mode — 全自动化引导 ✅ 2026-02-22
  - [x] 新建 `cmd/openacosmi/setup_noninteractive.go` (436L)
  - [x] 移植 `RunNonInteractiveOnboarding`（local/remote 分发）
  - [x] 移植 `InferAuthChoiceFromFlags`（15 provider flag → authChoice）
  - [x] 移植 `applyNonInteractiveGatewayConfig`（port/bind/auth/Tailscale 安全约束）
  - [x] `ApplyNonInteractiveSkillsConfig` + `InstallGatewayDaemonNonInteractive` + `LogNonInteractiveOnboardingJson` (D1 补全)
  - [x] 42 CLI flags 绑定到 `newOnboardCmd` (D2 补全)
  - [x] 测试：setup_noninteractive_test.go 21/21 PASS

- [x] **OB-7** (P2)：Auth Credentials + Models — 凭证存储 + 模型选择 ✅ 2026-02-22
  - [x] 补全 `cmd/openacosmi/setup_auth_credentials.go` → 379L (+108L)
  - [x] 新增 `PickDefaultModel`（单模型自动选择 + 多模型交互）
  - [x] 新增 `BuildProviderModelCatalog`（9 provider 目录）
  - [x] 测试：+5 tests PASS

---

## 窗口 3：P2 频道交互 + P3 扩展（~8h）

- [x] **OB-8** (P2)：Channel Onboarding — Discord ✅ 2026-02-22
  - [x] 补全 `channels/onboarding_discord.go`（骨架→340L 完整）
  - [x] 移植 token 输入 + env 检测 + guild/channel access 配置
  - [x] SetDiscordDmPolicy / GroupPolicy / GuildChannelAllowlist / AllowFrom / Disable
  - [x] 测试：+12 tests PASS

- [x] **OB-9** (P2)：Channel Onboarding — Slack ✅ 2026-02-22
  - [x] 补全 `channels/onboarding_slack.go`（骨架→340L 完整）
  - [x] 移植 App Manifest 生成 + 双 token (bot+app) 输入 + env 检测
  - [x] SetSlackDmPolicy / GroupPolicy / ChannelAllowlist / AllowFrom / Disable
  - [x] 测试：+5 tests PASS

- [x] **OB-10** (P2)：Channel Onboarding — WhatsApp + Telegram ✅ 2026-02-22
  - [x] 补全 `channels/onboarding_whatsapp.go`（骨架→290L）— self-chat + group access
  - [x] 补全 `channels/onboarding_telegram.go`（骨架→310L）— BotFather 引导 + token + group access
  - [x] WhatsApp QR 扫描 loginWeb() → TODO(WhatsApp-login) 延迟 → OB-10-D1
  - [x] 测试：+10 tests PASS

- [x] **OB-11** (P3)：Channel Onboarding — Signal + iMessage ✅ 2026-02-22
  - [x] 补全 `channels/onboarding_signal.go`（骨架→310L）— signal-cli 检测 + E.164/UUID 验证
  - [x] 补全 `channels/onboarding_imessage.go`（骨架→290L）— imsg CLI 检测 + handle 验证
  - [x] 测试：+10 tests PASS

---

## 窗口 4：P3 收尾 + 全量验证（~6h）

- [x] **OB-12** (P3)：Hooks + Skills + Remote 引导 ✅ 2026-02-22
  - [x] 新建 `cmd/openacosmi/setup_hooks.go` (135L) — hooks 自动发现 + 多选启用 + config 写入
  - [x] 新建 `cmd/openacosmi/setup_skills.go` (405L) — 技能发现/依赖安装/API key/节点管理器
  - [x] 新建 `cmd/openacosmi/setup_remote.go` (251L) — Bonjour 发现 + 直连/SSH + URL/auth
  - [x] TS 对照：`onboard-hooks.ts` (85L) + `onboard-skills.ts` (205L) + `onboard-remote.ts` (155L)

- [x] **OB-13** (P3)：Channel Access 交互 ✅ 2026-02-22 (部分提前完成)
  - [x] 新建 `channels/onboarding_channel_access.go`（~150L）— 本地 Prompter 接口 + promptChannelAccessPolicy/Allowlist/Config
  - [x] 补全 `cmd/openacosmi/setup_types.go` — OnboardOptions 55 字段完整对齐 TS (245L) ✅ 2026-02-22
  - [x] TS 对照：`onboard-types.ts` (105L)

- [x] 全量验证 ✅ 2026-02-22
  - [x] `go build ./...` ✅
  - [x] `go vet ./...` ✅
  - [x] `go test -race ./internal/gateway/...` ✅
  - [x] `go test -race ./internal/channels/...` ✅
  - [x] `go test -race ./cmd/openacosmi/...` ✅
  - [x] /fuhe 复核审计 → 评级 **A**
