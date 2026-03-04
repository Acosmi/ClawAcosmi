# 后端 i18n 国际化实施跟踪

> 创建日期：2026-02-22 | 状态：**全部完成 ✅**

## 概览

| 维度 | 值 |
|------|-----|
| 总 key 数 | ~148（zh-CN=148, en-US=148） |
| 影响文件 | 16 个 |
| 语言 | 中文（默认）+ 英文 |
| 测试数 | 17 个（含 key 完整性、占位符一致性） |
| 前端状态 | ✅ 已完成双语言 (`en.ts` + `zh.ts`) |

## 窗口进度

### W1: i18n 基础设施增强 ✅

- [x] `pkg/i18n/i18n.go` — `InitFromEnv()` + `Tp()` + `Tf()`
- [x] `pkg/i18n/i18n_onboarding_zh.go` — 中文语言包 (~110 keys)
- [x] `pkg/i18n/i18n_onboarding_en.go` — 英文语言包 (~110 keys)
- [x] `pkg/i18n/i18n_test.go` — 17 tests（key 完整性 + Tp/Tf + InitFromEnv + 占位符一致性）

### W2: 初始化入口集成 ✅

- [x] `cmd/openacosmi/main.go` — `i18n.InitFromEnv()` + `--lang` global flag

### W3: gateway/wizard 模块替换 ✅

- [x] `wizard_finalize.go` — ~25 处替换
- [x] `wizard_gateway_config.go` — ~10 处替换
- [x] `wizard_onboarding.go` — ~8 处替换

### W4: cmd/setup 模块替换 ✅

- [x] `setup_channels.go` — ~10 处替换
- [x] `setup_skills.go` — ~9 处替换
- [x] `setup_remote.go` — ~9 处替换
- [x] `setup_hooks.go` — ~4 处替换
- [x] `setup_auth_credentials.go` — ~1 处替换
- [x] `setup_auth_options.go` — ~3 处替换

### W5: channels/onboarding 模块替换 ✅

- [x] `onboarding_discord.go` — ~7 处替换
- [x] `onboarding_slack.go` — ~8 处替换
- [x] `onboarding_telegram.go` — ~5 处替换
- [x] `onboarding_whatsapp.go` — ~3 处替换
- [x] `onboarding_signal.go` — ~3 处替换
- [x] `onboarding_imessage.go` — ~3 处替换
- [x] `onboarding_channel_access.go` — ~2 处替换

### 全量验证 ✅

- [x] `go build ./...`
- [x] `go vet ./...`
- [x] `go test -race ./pkg/i18n/...` (17 tests)
- [x] `go test -race ./internal/gateway/...`
- [x] `go test -race ./cmd/openacosmi/...`
- [x] `go test -race ./internal/channels/...`
- [ ] 手动验证：`LANG=zh_CN.UTF-8 openacosmi setup`
- [ ] 手动验证：`LANG=en_US.UTF-8 openacosmi setup`

## Key 命名规范

```
onboard.{module}.{action}

示例:
onboard.daemon.confirm          → "安装 Gateway 服务（推荐）"
onboard.ch.discord.title        → "Discord"
onboard.skill.configure         → "现在配置技能？（推荐）"
onboard.completion.prompt       → "启用 %s shell 补全？"
```
