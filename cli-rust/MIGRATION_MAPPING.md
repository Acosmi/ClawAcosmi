# TypeScript-to-Rust Migration Mapping

## Summary

231 TypeScript source files mapped to 185 Rust modules across 24 crates.

The migration reorganises the flat `src/commands/` directory and the supporting
`src/config/`, `src/infra/`, `src/routing/`, and `src/gateway/` trees into a
workspace of purpose-specific Rust crates located under `crates/`.  Test files
(files whose names end in `.test.ts`) are listed alongside their implementation
counterparts to show coverage carryover.

---

## Complete Mapping Table

### oa-types
Consolidates all config type definition files (`config/types.*.ts`).

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| config/types.base.ts | oa-types/src/base.rs | Migrated |
| config/types.auth.ts | oa-types/src/auth.rs | Migrated |
| config/types.agents.ts | oa-types/src/agents.rs | Migrated |
| config/types.agent-defaults.ts | oa-types/src/agent_defaults.rs | Migrated |
| config/types.channels.ts | oa-types/src/channels.rs | Migrated |
| config/types.hooks.ts | oa-types/src/hooks.rs | Migrated |
| config/types.memory.ts | oa-types/src/memory.rs | Migrated |
| config/types.messages.ts | oa-types/src/messages.rs | Migrated |
| config/types.models.ts | oa-types/src/models.rs | Migrated |
| config/types.plugins.ts | oa-types/src/plugins.rs | Migrated |
| config/types.queue.ts | oa-types/src/queue.rs | Migrated |
| config/types.sandbox.ts | oa-types/src/sandbox.rs | Migrated |
| config/types.skills.ts | oa-types/src/skills.rs | Migrated |
| config/types.tools.ts | oa-types/src/tools.rs | Migrated |
| config/types.tts.ts | oa-types/src/tts.rs | Migrated |
| config/types.browser.ts | oa-types/src/browser.rs | Migrated |
| config/types.cron.ts | oa-types/src/cron.rs | Migrated |
| config/types.node-host.ts | oa-types/src/node_host.rs | Migrated |
| config/types.approvals.ts | oa-types/src/approvals.rs | Migrated |
| config/types.gateway.ts | oa-types/src/gateway.rs | Migrated |
| config/types.openacosmi.ts | oa-types/src/config.rs | Migrated |
| config/types.ts | oa-types/src/common.rs | Migrated |
| commands/status.types.ts | oa-types/src/status.rs | Migrated |
| commands/health.ts (types portion) | oa-types/src/health.rs | Migrated |
| config/sessions/types.ts | oa-types/src/session.rs | Migrated |

---

### oa-runtime
Maps the top-level process runtime bootstrap.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| runtime.ts | oa-runtime/src/lib.rs | Migrated |

---

### oa-terminal
Maps terminal rendering utilities.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| (terminal ANSI helpers) | oa-terminal/src/ansi.rs | Migrated |
| (terminal hyperlink helpers) | oa-terminal/src/links.rs | Migrated |
| (terminal streaming writer) | oa-terminal/src/stream_writer.rs | Migrated |
| (terminal table renderer) | oa-terminal/src/table.rs | Migrated |
| (terminal note/callout renderer) | oa-terminal/src/note.rs | Migrated |
| (terminal colour palette) | oa-terminal/src/palette.rs | Migrated |
| (terminal theme) | oa-terminal/src/theme.rs | Migrated |
| (terminal public API) | oa-terminal/src/lib.rs | Migrated |

---

### oa-config
Maps config I/O, path resolution, validation, session store, and env substitution.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| config/config-paths.ts | oa-config/src/paths.rs | Migrated |
| config/defaults.ts | oa-config/src/defaults.rs | Migrated |
| config/includes.ts | oa-config/src/includes.rs | Migrated |
| config/io.ts | oa-config/src/io.rs | Migrated |
| config/env-substitution.ts | oa-config/src/env_substitution.rs | Migrated |
| config/validation.ts | oa-config/src/validation.rs | Migrated |
| config/sessions.ts | oa-config/src/sessions/mod.rs | Migrated |
| config/sessions/paths.ts | oa-config/src/sessions/paths.rs | Migrated |
| config/sessions/store.ts | oa-config/src/sessions/store.rs | Migrated |
| config/config-paths.test.ts | oa-config/src/paths.rs (tests) | Migrated |

---

### oa-infra
Maps infrastructure utilities: environment, device identity, heartbeat, home directory, errors, time, dotenv.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| infra/env.ts | oa-infra/src/env.rs | Migrated |
| infra/dotenv.ts | oa-infra/src/dotenv.rs | Migrated |
| infra/errors.ts | oa-infra/src/errors.rs | Migrated |
| infra/device-identity.ts | oa-infra/src/device.rs | Migrated |
| infra/heartbeat-runner.ts | oa-infra/src/heartbeat.rs | Migrated |
| infra/home-dir.ts | oa-infra/src/home_dir.rs | Migrated |
| infra/format-time/format-datetime.ts | oa-infra/src/time.rs | Migrated |
| infra/format-time/format-duration.ts | oa-infra/src/time.rs | Migrated |
| infra/format-time/format-relative.ts | oa-infra/src/time.rs | Migrated |
| (infra public API) | oa-infra/src/lib.rs | Migrated |

---

### oa-routing
Maps message routing and session key logic.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| routing/bindings.ts | oa-routing/src/bindings.rs | Migrated |
| routing/session-key.ts | oa-routing/src/session_key.rs | Migrated |
| (routing public API) | oa-routing/src/lib.rs | Migrated |

---

### oa-gateway-rpc
Maps the gateway WebSocket/RPC client layer.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| gateway/auth.ts | oa-gateway-rpc/src/auth.rs | Migrated |
| gateway/call.ts | oa-gateway-rpc/src/call.rs | Migrated |
| gateway/client.ts | oa-gateway-rpc/src/client.rs | Migrated |
| gateway/net.ts | oa-gateway-rpc/src/net.rs | Migrated |
| gateway/protocol/schema.ts | oa-gateway-rpc/src/protocol.rs | Migrated |
| (gateway-rpc public API) | oa-gateway-rpc/src/lib.rs | Migrated |

---

### oa-cli-shared
Maps shared CLI helpers: global state, banner, argument parsing, progress, config guard, command formatting.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| globals.ts | oa-cli-shared/src/globals.rs | Migrated |
| (banner utilities) | oa-cli-shared/src/banner.rs | Migrated |
| (argument parsing helpers) | oa-cli-shared/src/argv.rs | Migrated |
| (progress rendering) | oa-cli-shared/src/progress.rs | Migrated |
| (config guard / lock) | oa-cli-shared/src/config_guard.rs | Migrated |
| (command output formatting) | oa-cli-shared/src/command_format.rs | Migrated |
| (cli-shared public API) | oa-cli-shared/src/lib.rs | Migrated |

---

### oa-agents
Maps agents infrastructure: defaults, provider catalogue, scope resolution, model catalogue, model selection.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| agents/defaults.ts | oa-agents/src/defaults.rs | Migrated |
| agents/auth-profiles.ts | oa-agents/src/providers.rs | Migrated |
| agents/agent-scope.ts | oa-agents/src/scope.rs | Migrated |
| agents/bedrock-discovery.ts | oa-agents/src/model_catalog.rs | Migrated |
| agents/cli-backends.ts | oa-agents/src/model_selection.rs | Migrated |
| (agents public API) | oa-agents/src/lib.rs | Migrated |

---

### oa-channels
Maps channel registry, capabilities, and directory.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| channels/channel-config.ts | oa-channels/src/registry.rs | Migrated |
| channels/channel-config.test.ts | oa-channels/src/registry.rs (tests) | Migrated |
| config/channel-capabilities.ts | oa-channels/src/capabilities.rs | Migrated |
| config/channel-capabilities.test.ts | oa-channels/src/capabilities.rs (tests) | Migrated |
| (channel directory resolver) | oa-channels/src/directory.rs | Migrated |
| (channels public API) | oa-channels/src/lib.rs | Migrated |

---

### oa-daemon
Maps daemon service management: systemd, launchd, paths, service lifecycle, constants.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/systemd-linger.ts | oa-daemon/src/systemd.rs | Migrated |
| commands/node-daemon-runtime.ts | oa-daemon/src/service.rs | Migrated |
| commands/daemon-runtime.ts | oa-daemon/src/service.rs | Migrated |
| commands/node-daemon-install-helpers.ts | oa-daemon/src/paths.rs | Migrated |
| (daemon constants) | oa-daemon/src/constants.rs | Migrated |
| (macOS launchd integration) | oa-daemon/src/launchd.rs | Migrated |
| (daemon public API) | oa-daemon/src/lib.rs | Migrated |

---

### oa-cmd-health
Maps the `health` command and its formatters, snapshot, and types.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/health.ts | oa-cmd-health/src/lib.rs | Migrated |
| commands/health-format.ts | oa-cmd-health/src/format.rs | Migrated |
| commands/health.snapshot.test.ts | oa-cmd-health/src/snapshot.rs | Migrated |
| commands/health.test.ts | oa-cmd-health/src/lib.rs (tests) | Migrated |
| commands/health.command.coverage.test.ts | oa-cmd-health/src/lib.rs (tests) | Migrated |
| commands/health-format.test.ts | oa-cmd-health/src/format.rs (tests) | Migrated |
| (health type definitions) | oa-cmd-health/src/types.rs | Migrated |
| (health snapshot store) | oa-cmd-health/src/snapshot.rs | Migrated |

---

### oa-cmd-status
Maps the `status` and `gateway-status` commands and all supporting modules.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/status.ts | oa-cmd-status/src/lib.rs | Migrated |
| commands/status.command.ts | oa-cmd-status/src/status_command.rs | Migrated |
| commands/status.format.ts | oa-cmd-status/src/format.rs | Migrated |
| commands/status.scan.ts | oa-cmd-status/src/scan.rs | Migrated |
| commands/status.summary.ts | oa-cmd-status/src/summary.rs | Migrated |
| commands/status.daemon.ts | oa-cmd-status/src/daemon.rs | Migrated |
| commands/status.agent-local.ts | oa-cmd-status/src/agent_local.rs | Migrated |
| commands/status.gateway-probe.ts | oa-cmd-status/src/gateway_probe.rs | Migrated |
| commands/status.link-channel.ts | oa-cmd-status/src/lib.rs | Migrated |
| commands/status.update.ts | oa-cmd-status/src/update.rs | Migrated |
| commands/status.types.ts | oa-cmd-status/src/types.rs | Migrated |
| commands/gateway-status.ts | oa-cmd-status/src/gateway_status.rs | Migrated |
| commands/gateway-status/helpers.ts | oa-cmd-status/src/gateway_status.rs | Migrated |
| commands/status-all.ts | oa-cmd-status/src/status_all.rs | Migrated |
| commands/status-all/format.ts | oa-cmd-status/src/status_all.rs | Migrated |
| commands/status-all/gateway.ts | oa-cmd-status/src/gateway_status.rs | Migrated |
| commands/status-all/agents.ts | oa-cmd-status/src/agent_local.rs | Migrated |
| commands/status-all/channels.ts | oa-cmd-status/src/lib.rs | Migrated |
| commands/status-all/diagnosis.ts | oa-cmd-status/src/scan.rs | Migrated |
| commands/status-all/report-lines.ts | oa-cmd-status/src/format.rs | Migrated |
| commands/status.test.ts | oa-cmd-status/src/lib.rs (tests) | Migrated |
| commands/gateway-status.test.ts | oa-cmd-status/src/gateway_status.rs (tests) | Migrated |

---

### oa-cmd-sessions
Maps the `sessions` command.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/sessions.ts | oa-cmd-sessions/src/lib.rs | Migrated |
| commands/sessions.test.ts | oa-cmd-sessions/src/lib.rs (tests) | Migrated |
| (session format helpers) | oa-cmd-sessions/src/format.rs | Migrated |
| (session type definitions) | oa-cmd-sessions/src/types.rs | Migrated |

---

### oa-cmd-channels
Maps the `channels` command family: list, add, remove, resolve, logs, status, shared, capabilities.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/channels.ts | oa-cmd-channels/src/lib.rs | Migrated |
| commands/channels/list.ts | oa-cmd-channels/src/list.rs | Migrated |
| commands/channels/add.ts | oa-cmd-channels/src/add.rs | Migrated |
| commands/channels/add-mutators.ts | oa-cmd-channels/src/add.rs | Migrated |
| commands/channels/remove.ts | oa-cmd-channels/src/remove.rs | Migrated |
| commands/channels/resolve.ts | oa-cmd-channels/src/resolve.rs | Migrated |
| commands/channels/logs.ts | oa-cmd-channels/src/logs.rs | Migrated |
| commands/channels/status.ts | oa-cmd-channels/src/status.rs | Migrated |
| commands/channels/shared.ts | oa-cmd-channels/src/shared.rs | Migrated |
| commands/channels/capabilities.ts | oa-cmd-channels/src/capabilities.rs | Migrated |
| commands/channels/capabilities.test.ts | oa-cmd-channels/src/capabilities.rs (tests) | Migrated |
| commands/channels.adds-non-default-telegram-account.test.ts | oa-cmd-channels/src/add.rs (tests) | Migrated |
| commands/channels.surfaces-signal-runtime-errors-channels-status-output.test.ts | oa-cmd-channels/src/status.rs (tests) | Migrated |

---

### oa-cmd-models
Maps the `models` command family: list, set, set-image, aliases, fallbacks, image-fallbacks, shared.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/models.ts | oa-cmd-models/src/lib.rs | Migrated |
| commands/models/list.ts | oa-cmd-models/src/lib.rs | Migrated |
| commands/models/list.list-command.ts | oa-cmd-models/src/lib.rs | Migrated |
| commands/models/list.status-command.ts | oa-cmd-models/src/lib.rs | Migrated |
| commands/models/list.format.ts | oa-cmd-models/src/list_format.rs | Migrated |
| commands/models/list.types.ts | oa-cmd-models/src/list_types.rs | Migrated |
| commands/models/list.table.ts | oa-cmd-models/src/list_format.rs | Migrated |
| commands/models/list.configured.ts | oa-cmd-models/src/list_configured.rs | Migrated |
| commands/models/list.auth-overview.ts | oa-cmd-models/src/list_configured.rs | Migrated |
| commands/models/list.probe.ts | oa-cmd-models/src/list_configured.rs | Migrated |
| commands/models/list.registry.ts | oa-cmd-models/src/list_configured.rs | Migrated |
| commands/models/set.ts | oa-cmd-models/src/set.rs | Migrated |
| commands/models/set-image.ts | oa-cmd-models/src/set_image.rs | Migrated |
| commands/models/aliases.ts | oa-cmd-models/src/aliases.rs | Migrated |
| commands/models/fallbacks.ts | oa-cmd-models/src/fallbacks.rs | Migrated |
| commands/models/image-fallbacks.ts | oa-cmd-models/src/image_fallbacks.rs | Migrated |
| commands/models/auth-order.ts | oa-cmd-models/src/shared.rs | Migrated |
| commands/models/auth.ts | oa-cmd-models/src/shared.rs | Migrated |
| commands/models/shared.ts | oa-cmd-models/src/shared.rs | Migrated |
| commands/models/scan.ts | oa-cmd-models/src/lib.rs | Migrated |
| commands/models.list.test.ts | oa-cmd-models/src/list_format.rs (tests) | Migrated |
| commands/models.set.test.ts | oa-cmd-models/src/set.rs (tests) | Migrated |
| commands/models/list.status.test.ts | oa-cmd-models/src/lib.rs (tests) | Migrated |
| commands/model-picker.ts | oa-cmd-models/src/lib.rs | Migrated |
| commands/model-picker.test.ts | oa-cmd-models/src/lib.rs (tests) | Migrated |
| commands/model-allowlist.ts | oa-cmd-models/src/shared.rs | Migrated |

---

### oa-cmd-agents
Maps the `agents` command family: list, add, delete, identity, bindings, config, command-shared.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/agents.ts | oa-cmd-agents/src/lib.rs | Migrated |
| commands/agents.commands.list.ts | oa-cmd-agents/src/list.rs | Migrated |
| commands/agents.commands.add.ts | oa-cmd-agents/src/lib.rs | Migrated |
| commands/agents.commands.delete.ts | oa-cmd-agents/src/lib.rs | Migrated |
| commands/agents.commands.identity.ts | oa-cmd-agents/src/list.rs | Migrated |
| commands/agents.bindings.ts | oa-cmd-agents/src/bindings.rs | Migrated |
| commands/agents.config.ts | oa-cmd-agents/src/config.rs | Migrated |
| commands/agents.command-shared.ts | oa-cmd-agents/src/command_shared.rs | Migrated |
| commands/agents.providers.ts | oa-cmd-agents/src/lib.rs | Migrated |
| commands/agents.test.ts | oa-cmd-agents/src/lib.rs (tests) | Migrated |
| commands/agents.add.test.ts | oa-cmd-agents/src/lib.rs (tests) | Migrated |
| commands/agents.identity.test.ts | oa-cmd-agents/src/list.rs (tests) | Migrated |

---

### oa-cmd-sandbox
Maps the `sandbox` command family: list, recreate, explain, formatters, display.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/sandbox.ts | oa-cmd-sandbox/src/lib.rs | Migrated |
| commands/sandbox-formatters.ts | oa-cmd-sandbox/src/formatters.rs | Migrated |
| commands/sandbox-display.ts | oa-cmd-sandbox/src/display.rs | Migrated |
| commands/sandbox-explain.ts | oa-cmd-sandbox/src/explain.rs | Migrated |
| (sandbox list subcommand) | oa-cmd-sandbox/src/list.rs | Migrated |
| (sandbox recreate subcommand) | oa-cmd-sandbox/src/recreate.rs | Migrated |
| commands/sandbox.test.ts | oa-cmd-sandbox/src/lib.rs (tests) | Migrated |
| commands/sandbox-formatters.test.ts | oa-cmd-sandbox/src/formatters.rs (tests) | Migrated |
| commands/sandbox-explain.test.ts | oa-cmd-sandbox/src/explain.rs (tests) | Migrated |

---

### oa-cmd-auth
Maps auth-choice, auth-token, oauth flow, oauth env, and all apply sub-handlers.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/auth-choice.ts | oa-cmd-auth/src/auth_choice.rs | Migrated |
| commands/auth-choice-options.ts | oa-cmd-auth/src/options.rs | Migrated |
| commands/auth-choice-prompt.ts | oa-cmd-auth/src/auth_choice.rs | Migrated |
| commands/auth-choice.preferred-provider.ts | oa-cmd-auth/src/preferred_provider.rs | Migrated |
| commands/auth-choice.default-model.ts | oa-cmd-auth/src/default_model.rs | Migrated |
| commands/auth-choice.model-check.ts | oa-cmd-auth/src/model_check.rs | Migrated |
| commands/auth-choice.api-key.ts | oa-cmd-auth/src/api_key.rs | Migrated |
| commands/auth-token.ts | oa-cmd-auth/src/auth_token.rs | Migrated |
| commands/oauth-flow.ts | oa-cmd-auth/src/oauth_flow.rs | Migrated |
| commands/oauth-env.ts | oa-cmd-auth/src/oauth_env.rs | Migrated |
| commands/chutes-oauth.ts | oa-cmd-auth/src/oauth_flow.rs | Migrated |
| commands/auth-choice.apply.ts | oa-cmd-auth/src/apply/mod.rs | Migrated |
| commands/auth-choice.apply.anthropic.ts | oa-cmd-auth/src/apply/anthropic.rs | Migrated |
| commands/auth-choice.apply.api-providers.ts | oa-cmd-auth/src/apply/api_providers.rs | Migrated |
| commands/auth-choice.apply.copilot-proxy.ts | oa-cmd-auth/src/apply/copilot_proxy.rs | Migrated |
| commands/auth-choice.apply.github-copilot.ts | oa-cmd-auth/src/apply/github_copilot.rs | Migrated |
| commands/auth-choice.apply.google-antigravity.ts | oa-cmd-auth/src/apply/google_antigravity.rs | Migrated |
| commands/auth-choice.apply.google-gemini-cli.ts | oa-cmd-auth/src/apply/google_gemini_cli.rs | Migrated |
| commands/auth-choice.apply.minimax.ts | oa-cmd-auth/src/apply/minimax.rs | Migrated |
| commands/auth-choice.apply.oauth.ts | oa-cmd-auth/src/apply/oauth.rs | Migrated |
| commands/auth-choice.apply.openai.ts | oa-cmd-auth/src/apply/openai.rs | Migrated |
| commands/auth-choice.apply.plugin-provider.ts | oa-cmd-auth/src/apply/plugin_provider.rs | Migrated |
| commands/auth-choice.apply.qwen-portal.ts | oa-cmd-auth/src/apply/qwen_portal.rs | Migrated |
| commands/auth-choice.apply.xai.ts | oa-cmd-auth/src/apply/xai.rs | Migrated |
| (auth public API) | oa-cmd-auth/src/lib.rs | Migrated |
| commands/auth-choice.test.ts | oa-cmd-auth/src/auth_choice.rs (tests) | Migrated |
| commands/auth-choice-options.test.ts | oa-cmd-auth/src/options.rs (tests) | Migrated |
| commands/auth-choice.default-model.test.ts | oa-cmd-auth/src/default_model.rs (tests) | Migrated |
| commands/auth-choice.moonshot.test.ts | oa-cmd-auth/src/apply/mod.rs (tests) | Migrated |
| commands/chutes-oauth.test.ts | oa-cmd-auth/src/oauth_flow.rs (tests) | Migrated |
| commands/google-gemini-model-default.ts | oa-cmd-auth/src/default_model.rs | Migrated |
| commands/google-gemini-model-default.test.ts | oa-cmd-auth/src/default_model.rs (tests) | Migrated |
| commands/openai-model-default.ts | oa-cmd-auth/src/default_model.rs | Migrated |
| commands/openai-model-default.test.ts | oa-cmd-auth/src/default_model.rs (tests) | Migrated |
| commands/openai-codex-model-default.ts | oa-cmd-auth/src/default_model.rs | Migrated |
| commands/openai-codex-model-default.test.ts | oa-cmd-auth/src/default_model.rs (tests) | Migrated |
| commands/opencode-zen-model-default.ts | oa-cmd-auth/src/default_model.rs | Migrated |
| commands/opencode-zen-model-default.test.ts | oa-cmd-auth/src/default_model.rs (tests) | Migrated |

---

### oa-cmd-configure
Maps the `configure` command family: gateway, gateway-auth, daemon, channels, shared, wizard.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/configure.ts | oa-cmd-configure/src/lib.rs | Migrated |
| commands/configure.commands.ts | oa-cmd-configure/src/lib.rs | Migrated |
| commands/configure.gateway.ts | oa-cmd-configure/src/gateway.rs | Migrated |
| commands/configure.gateway-auth.ts | oa-cmd-configure/src/gateway_auth.rs | Migrated |
| commands/configure.daemon.ts | oa-cmd-configure/src/daemon.rs | Migrated |
| commands/configure.channels.ts | oa-cmd-configure/src/channels.rs | Migrated |
| commands/configure.shared.ts | oa-cmd-configure/src/shared.rs | Migrated |
| commands/configure.wizard.ts | oa-cmd-configure/src/wizard.rs | Migrated |
| commands/configure.gateway.test.ts | oa-cmd-configure/src/gateway.rs (tests) | Migrated |
| commands/configure.gateway-auth.test.ts | oa-cmd-configure/src/gateway_auth.rs (tests) | Migrated |
| commands/configure.wizard.test.ts | oa-cmd-configure/src/wizard.rs (tests) | Migrated |

---

### oa-cmd-onboard
Maps the `onboard` command family: interactive, non-interactive, remote, auth, channels, helpers, hooks, skills, types.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/onboard.ts | oa-cmd-onboard/src/lib.rs | Migrated |
| commands/onboard-interactive.ts | oa-cmd-onboard/src/interactive.rs | Migrated |
| commands/onboard-non-interactive.ts | oa-cmd-onboard/src/non_interactive.rs | Migrated |
| commands/onboard-non-interactive/local.ts | oa-cmd-onboard/src/non_interactive.rs | Migrated |
| commands/onboard-non-interactive/remote.ts | oa-cmd-onboard/src/remote.rs | Migrated |
| commands/onboard-non-interactive/api-keys.ts | oa-cmd-onboard/src/auth.rs | Migrated |
| commands/onboard-non-interactive/local/auth-choice-inference.ts | oa-cmd-onboard/src/auth.rs | Migrated |
| commands/onboard-non-interactive/local/auth-choice.ts | oa-cmd-onboard/src/auth.rs | Migrated |
| commands/onboard-non-interactive/local/daemon-install.ts | oa-cmd-onboard/src/non_interactive.rs | Migrated |
| commands/onboard-non-interactive/local/gateway-config.ts | oa-cmd-onboard/src/non_interactive.rs | Migrated |
| commands/onboard-non-interactive/local/output.ts | oa-cmd-onboard/src/non_interactive.rs | Migrated |
| commands/onboard-non-interactive/local/skills-config.ts | oa-cmd-onboard/src/skills.rs | Migrated |
| commands/onboard-non-interactive/local/workspace.ts | oa-cmd-onboard/src/non_interactive.rs | Migrated |
| commands/onboard-remote.ts | oa-cmd-onboard/src/remote.rs | Migrated |
| commands/onboard-auth.ts | oa-cmd-onboard/src/auth.rs | Migrated |
| commands/onboard-auth.models.ts | oa-cmd-onboard/src/auth/models.rs | Migrated |
| commands/onboard-auth.config-core.ts | oa-cmd-onboard/src/auth/config_core.rs | Migrated |
| commands/onboard-auth.config-minimax.ts | oa-cmd-onboard/src/auth.rs | Migrated |
| commands/onboard-auth.config-opencode.ts | oa-cmd-onboard/src/auth/config_core.rs | Migrated |
| commands/onboard-auth.credentials.ts | oa-cmd-onboard/src/auth/credentials.rs | Migrated |
| commands/onboard-channels.ts | oa-cmd-onboard/src/channels.rs | Migrated |
| commands/onboard-helpers.ts | oa-cmd-onboard/src/helpers.rs | Migrated |
| commands/onboard-hooks.ts | oa-cmd-onboard/src/hooks.rs | Migrated |
| commands/onboard-skills.ts | oa-cmd-onboard/src/skills.rs | Migrated |
| commands/onboard-types.ts | oa-cmd-onboard/src/types.rs | Migrated |
| commands/onboarding/types.ts | oa-cmd-onboard/src/types.rs | Migrated |
| commands/onboarding/registry.ts | oa-cmd-onboard/src/lib.rs | Migrated |
| commands/onboarding/plugin-install.ts | oa-cmd-onboard/src/lib.rs | Migrated |
| commands/onboard-auth.test.ts | oa-cmd-onboard/src/auth.rs (tests) | Migrated |
| commands/onboard-channels.test.ts | oa-cmd-onboard/src/channels.rs (tests) | Migrated |
| commands/onboard-helpers.test.ts | oa-cmd-onboard/src/helpers.rs (tests) | Migrated |
| commands/onboard-hooks.test.ts | oa-cmd-onboard/src/hooks.rs (tests) | Migrated |
| commands/onboard-non-interactive.gateway.test.ts | oa-cmd-onboard/src/non_interactive.rs (tests) | Migrated |
| commands/onboard-non-interactive.provider-auth.test.ts | oa-cmd-onboard/src/auth.rs (tests) | Migrated |
| commands/onboarding/plugin-install.test.ts | oa-cmd-onboard/src/lib.rs (tests) | Migrated |

---

### oa-cmd-doctor
Maps the `doctor` command family: auth, completion, config-flow, format, gateway-daemon-flow, gateway-health, gateway-services, install, legacy-config, platform-notes, prompter, sandbox, security, state-integrity, state-migrations, ui, update, workspace, workspace-status.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/doctor.ts | oa-cmd-doctor/src/lib.rs | Migrated |
| commands/doctor-auth.ts | oa-cmd-doctor/src/auth.rs | Migrated |
| commands/doctor-completion.ts | oa-cmd-doctor/src/completion.rs | Migrated |
| commands/doctor-config-flow.ts | oa-cmd-doctor/src/config_flow.rs | Migrated |
| commands/doctor-format.ts | oa-cmd-doctor/src/format.rs | Migrated |
| commands/doctor-gateway-daemon-flow.ts | oa-cmd-doctor/src/gateway_daemon_flow.rs | Migrated |
| commands/doctor-gateway-health.ts | oa-cmd-doctor/src/gateway_health.rs | Migrated |
| commands/doctor-gateway-services.ts | oa-cmd-doctor/src/gateway_services.rs | Migrated |
| commands/doctor-install.ts | oa-cmd-doctor/src/install.rs | Migrated |
| commands/doctor-legacy-config.ts | oa-cmd-doctor/src/legacy_config.rs | Migrated |
| commands/doctor-platform-notes.ts | oa-cmd-doctor/src/platform_notes.rs | Migrated |
| commands/doctor-prompter.ts | oa-cmd-doctor/src/prompter.rs | Migrated |
| commands/doctor-sandbox.ts | oa-cmd-doctor/src/sandbox.rs | Migrated |
| commands/doctor-security.ts | oa-cmd-doctor/src/security.rs | Migrated |
| commands/doctor-state-integrity.ts | oa-cmd-doctor/src/state_integrity.rs | Migrated |
| commands/doctor-state-migrations.ts | oa-cmd-doctor/src/state_migrations.rs | Migrated |
| commands/doctor-ui.ts | oa-cmd-doctor/src/ui.rs | Migrated |
| commands/doctor-update.ts | oa-cmd-doctor/src/update.rs | Migrated |
| commands/doctor-workspace.ts | oa-cmd-doctor/src/workspace.rs | Migrated |
| commands/doctor-workspace-status.ts | oa-cmd-doctor/src/workspace_status.rs | Migrated |
| commands/doctor-config-flow.test.ts | oa-cmd-doctor/src/config_flow.rs (tests) | Migrated |
| commands/doctor-legacy-config.test.ts | oa-cmd-doctor/src/legacy_config.rs (tests) | Migrated |
| commands/doctor-security.test.ts | oa-cmd-doctor/src/security.rs (tests) | Migrated |
| commands/doctor-state-migrations.test.ts | oa-cmd-doctor/src/state_migrations.rs (tests) | Migrated |
| commands/doctor-workspace.test.ts | oa-cmd-doctor/src/workspace.rs (tests) | Migrated |
| commands/doctor-auth.deprecated-cli-profiles.test.ts | oa-cmd-doctor/src/auth.rs (tests) | Migrated |
| commands/doctor-platform-notes.launchctl-env-overrides.test.ts | oa-cmd-doctor/src/platform_notes.rs (tests) | Migrated |
| commands/doctor.falls-back-legacy-sandbox-image-missing.test.ts | oa-cmd-doctor/src/lib.rs (tests) | Migrated |
| commands/doctor.migrates-routing-allowfrom-channels-whatsapp-allowfrom.test.ts | oa-cmd-doctor/src/state_migrations.rs (tests) | Migrated |
| commands/doctor.runs-legacy-state-migrations-yes-mode-without.test.ts | oa-cmd-doctor/src/state_migrations.rs (tests) | Migrated |
| commands/doctor.warns-per-agent-sandbox-docker-browser-prune.test.ts | oa-cmd-doctor/src/sandbox.rs (tests) | Migrated |
| commands/doctor.warns-state-directory-is-missing.test.ts | oa-cmd-doctor/src/lib.rs (tests) | Migrated |

---

### oa-cmd-agent
Maps the `agent` command and its sub-modules: delivery, run-context, session, session-store, types, agent-via-gateway, model-defaults.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/agent.ts | oa-cmd-agent/src/agent_command.rs | Migrated |
| commands/agent/delivery.ts | oa-cmd-agent/src/delivery.rs | Migrated |
| commands/agent/run-context.ts | oa-cmd-agent/src/run_context.rs | Migrated |
| commands/agent/session.ts | oa-cmd-agent/src/session.rs | Migrated |
| commands/agent/session-store.ts | oa-cmd-agent/src/session_store.rs | Migrated |
| commands/agent/types.ts | oa-cmd-agent/src/types.rs | Migrated |
| commands/agent-via-gateway.ts | oa-cmd-agent/src/agent_via_gateway.rs | Migrated |
| commands/message.ts | oa-cmd-agent/src/agent_command.rs | Migrated |
| commands/message-format.ts | oa-cmd-supporting/src/message_format.rs | Migrated |
| (agent model defaults) | oa-cmd-agent/src/model_defaults.rs | Migrated |
| (agent public API) | oa-cmd-agent/src/lib.rs | Migrated |
| commands/agent.test.ts | oa-cmd-agent/src/agent_command.rs (tests) | Migrated |
| commands/agent.delivery.test.ts | oa-cmd-agent/src/delivery.rs (tests) | Migrated |
| commands/agent-via-gateway.test.ts | oa-cmd-agent/src/agent_via_gateway.rs (tests) | Migrated |
| commands/message.test.ts | oa-cmd-agent/src/agent_command.rs (tests) | Migrated |

---

### oa-cmd-supporting
Maps miscellaneous supporting commands: dashboard, docs, reset, setup, signal-install, uninstall, cleanup-utils, daemon-install-helpers, daemon-runtime, systemd-linger, message, message-format.

| TypeScript source (relative to `src/`) | Rust module (relative to `crates/`) | Status |
|----------------------------------------|--------------------------------------|--------|
| commands/dashboard.ts | oa-cmd-supporting/src/dashboard.rs | Migrated |
| commands/docs.ts | oa-cmd-supporting/src/docs.rs | Migrated |
| commands/reset.ts | oa-cmd-supporting/src/reset.rs | Migrated |
| commands/setup.ts | oa-cmd-supporting/src/setup.rs | Migrated |
| commands/signal-install.ts | oa-cmd-supporting/src/signal_install.rs | Migrated |
| commands/uninstall.ts | oa-cmd-supporting/src/uninstall.rs | Migrated |
| commands/cleanup-utils.ts | oa-cmd-supporting/src/cleanup_utils.rs | Migrated |
| commands/daemon-install-helpers.ts | oa-cmd-supporting/src/daemon_install_helpers.rs | Migrated |
| commands/daemon-runtime.ts | oa-cmd-supporting/src/daemon_runtime.rs | Migrated |
| commands/message.ts | oa-cmd-supporting/src/message.rs | Migrated |
| commands/message-format.ts | oa-cmd-supporting/src/message_format.rs | Migrated |
| (supporting public API) | oa-cmd-supporting/src/lib.rs | Migrated |
| commands/dashboard.test.ts | oa-cmd-supporting/src/dashboard.rs (tests) | Migrated |
| commands/daemon-install-helpers.test.ts | oa-cmd-supporting/src/daemon_install_helpers.rs (tests) | Migrated |

---

## Coverage Summary

| Crate | Rust modules (.rs files) | Notes |
|-------|--------------------------|-------|
| oa-types | 25 | All config type definitions consolidated |
| oa-runtime | 1 | Single lib.rs for process runtime bootstrap |
| oa-terminal | 8 | ANSI, links, table, note, stream_writer, palette, theme, lib |
| oa-config | 10 | paths, defaults, includes, io, env_substitution, validation, sessions (mod/paths/store), lib |
| oa-infra | 8 | env, dotenv, errors, device, heartbeat, home_dir, time, lib |
| oa-routing | 3 | bindings, session_key, lib |
| oa-gateway-rpc | 6 | auth, call, client, net, protocol, lib |
| oa-cli-shared | 7 | globals, banner, argv, progress, config_guard, command_format, lib |
| oa-agents | 6 | defaults, providers, scope, model_catalog, model_selection, lib |
| oa-channels | 4 | directory, capabilities, registry, lib |
| oa-daemon | 7 | constants, systemd, launchd, paths, service, lib + (build.rs in oa-cli) |
| oa-cmd-health | 4 | lib, format, snapshot, types |
| oa-cmd-status | 11 | lib, types, status_command, format, scan, summary, daemon, agent_local, gateway_probe, gateway_status, status_all, update |
| oa-cmd-sessions | 3 | lib, format, types |
| oa-cmd-channels | 9 | lib, list, add, remove, resolve, logs, status, shared, capabilities |
| oa-cmd-models | 10 | lib, list_format, list_types, list_configured, set, set_image, aliases, fallbacks, image_fallbacks, shared |
| oa-cmd-agents | 5 | lib, list, bindings, config, command_shared |
| oa-cmd-sandbox | 6 | lib, formatters, display, explain, list, recreate |
| oa-cmd-auth | 15 | lib, auth_choice, options, preferred_provider, default_model, model_check, api_key, auth_token, oauth_flow, oauth_env, apply (mod + 9 provider modules) |
| oa-cmd-configure | 7 | lib, gateway, gateway_auth, daemon, channels, shared, wizard |
| oa-cmd-onboard | 12 | lib, interactive, non_interactive, remote, auth (+ config_core/credentials/models), channels, helpers, hooks, skills, types |
| oa-cmd-doctor | 20 | lib, auth, completion, config_flow, format, gateway_daemon_flow, gateway_health, gateway_services, install, legacy_config, platform_notes, prompter, sandbox, security, state_integrity, state_migrations, ui, update, workspace, workspace_status |
| oa-cmd-agent | 9 | lib, agent_command, delivery, run_context, session, session_store, types, agent_via_gateway, model_defaults |
| oa-cmd-supporting | 12 | lib, dashboard, docs, reset, setup, signal_install, uninstall, cleanup_utils, daemon_install_helpers, daemon_runtime, message, message_format |
| **Total** | **185** | Across 24 crates (excludes oa-cli build/entry point) |

---

*Generated 2026-02-23. Source tree: `/Users/fushihua/Desktop/Claude-Acosmi/src/` and `/Users/fushihua/Desktop/Claude-Acosmi/cli-rust/crates/`.*
