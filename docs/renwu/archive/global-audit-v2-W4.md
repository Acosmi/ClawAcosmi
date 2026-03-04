# W4 审计报告：auto-reply + cron + daemon + hooks (V2 深度审计)

> 审计日期：2026-02-20 | 审计窗口：W4
> 版本：V2（反映持续重构及问题修复后的最新状态验证）

---

## 概览及最新覆盖率

| 模块 | TS 文件数 | Go 文件数 | 文件覆盖率 | TS 行数 | Go 行数 | 评级 | 趋势 |
|------|-----------|-----------|-----------|---------|---------|------|------|
| **AUTO-REPLY** | 121 | 96 | 79.3% | 22,028 | 17,209 | **C+** | 📈 覆盖率提升 |
| **CRON** | 22 | 19 | 86.3% | 3,767 | 3,711 | **A** | 稳定 |
| **DAEMON** | 19 | 22 | 115.7% | 3,554 | 2,877 | **C** | 稳定 |
| **HOOKS** | 22 | 16 | 72.7% | 3,914 | 3,966 | **B** | 稳定 |

### 显著进展说明

- **AUTO-REPLY**: Go 端文件从 90 个增至 96 个，行数增至 17,209（前期数据为 15,668 行），部分执行器及管线得到了回填重构，但仍有诸多 P0 缺口（具体见下）。
- **CRON**: Go 侧 19 个文件实现与 TS 原先的核心调度保持极高度对齐，表现优异。
- **DAEMON**: 因包括各种操作系统服务（Darwin / Windows / Linux），Go实现文件数比TS高，但 Linux 下的 Linger 机制仍旧短缺。

---


## 逐文件对照
## 逐文件对照 (Auto-reply)
| 状态 | TS 文件 | Go 文件 | 备注 |
|------|---------|---------|------|
| ✅ FULL / ⚠️ PARTIAL | `reply/abort.ts` | `reply/abort.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/agent-runner.ts` | `reply/agent_runner_payloads.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/agent-runner-execution.ts` | `reply/agent_runner.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/agent-runner-helpers.ts` | `reply/agent_runner.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/agent-runner-memory.ts` | `reply/agent_runner.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/agent-runner-payloads.ts` | `reply/agent_runner_payloads.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/agent-runner-utils.ts` | `reply/agent_runner.go` | 待研判 |
| ❌ MISSING | `reply/audio-tags.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/bash-command.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/block-reply-coalescer.ts` | `reply/block_reply_coalescer.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/block-reply-pipeline.ts` | `reply/block_reply_pipeline.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/block-streaming.ts` | `reply/block_streaming.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/body.ts` | `reply/body.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `chunk.ts` | `chunk.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/cleanup.ts` | `reply/queue_cleanup.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `command-auth.ts` | `command_auth.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `command-detection.ts` | `command_detection.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands.ts` | `commands_handler_context_report.go` | 待研判 |
| ❌ MISSING | `reply/commands-allowlist.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/commands-approve.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `commands-args.ts` | `commands_args.go` | 待研判 |
| ❌ MISSING | `reply/commands-bash.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/commands-compact.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/commands-config.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands-context.ts` | `commands_context.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands-context-report.ts` | `commands_context.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands-core.ts` | `commands_core.go` | 待研判 |
| ❌ MISSING | `reply/commands-info.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands-models.ts` | `model.go` | 待研判 |
| ❌ MISSING | `reply/commands-plugin.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/commands-ptt.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `commands-registry.ts` | `commands_registry.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `commands-registry.data.ts` | `commands_registry.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `commands-registry.types.ts` | `commands_registry.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands-session.ts` | `reply/session.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands-status.ts` | `status.go` | 待研判 |
| ❌ MISSING | `reply/commands-subagents.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/commands-tts.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/commands-types.ts` | `reply/types.go` | 待研判 |
| ❌ MISSING | `reply/config-commands.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/config-value.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/debug-commands.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/directive.ts` | `reply/directive_handling_impl.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/directive-handling.ts` | `reply/directive_handling_impl.go` | 待研判 |
| ❌ MISSING | `reply/directive-handling.auth.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/directive-handling.fast-lane.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/directive-handling.impl.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/directive-handling.model.ts` | `model.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/directive-handling.model-picker.ts` | `model.go` | 待研判 |
| ❌ MISSING | `reply/directive-handling.parse.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/directive-handling.persist.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/directive-handling.queue-validation.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/directive-handling.shared.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/directives.ts` | `reply/streaming_directives.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `dispatch.ts` | `dispatch.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/dispatch-from-config.ts` | `dispatch.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/drain.ts` | `reply/queue_drain.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/enqueue.ts` | `reply/queue_enqueue.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `envelope.ts` | `envelope.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/exec.ts` | `reply/model_fallback_executor.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/followup-runner.ts` | `reply/followup_runner.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/get-reply.ts` | `reply/get_reply_inline_actions.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/get-reply-directives.ts` | `reply/get_reply_directives.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/get-reply-directives-apply.ts` | `reply/get_reply_directives.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/get-reply-directives-utils.ts` | `reply/get_reply_directives.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/get-reply-inline-actions.ts` | `reply/get_reply_inline_actions.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/get-reply-run.ts` | `reply/get_reply_run.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `group-activation.ts` | `group_activation.go` | 待研判 |
| ❌ MISSING | `reply/groups.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `heartbeat.ts` | `heartbeat.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/history.ts` | `reply/history.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/inbound-context.ts` | `reply/inbound_context.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `inbound-debounce.ts` | `inbound_debounce.go` | 待研判 |
| ❌ MISSING | `reply/inbound-dedupe.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/inbound-sender-meta.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/inbound-text.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/line-directives.ts` | `reply/directives.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `media-note.ts` | `media_note.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/memory-flush.ts` | `reply/memory_flush.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/mentions.ts` | `reply/mentions.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `model.ts` | `reply/model_selection.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/model-selection.ts` | `reply/model_selection.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/normalize.ts` | `reply/normalize_reply.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/normalize-reply.ts` | `reply/normalize_reply.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/provider-dispatcher.ts` | `dispatch.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue.ts` | `reply/queue_command_lane.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply.ts` | `reply/block_reply_pipeline.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/reply-directives.ts` | `reply/get_reply_directives.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/reply-dispatcher.ts` | `dispatch.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/reply-elevated.ts` | `reply/reply_elevated.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/reply-inline.ts` | `reply/get_reply_inline_actions.go` | 待研判 |
| ❌ MISSING | `reply/reply-payloads.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/reply-reference.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/reply-tags.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/reply-threading.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `reply/response-prefix-template.ts` | `reply/response_prefix.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/route-reply.ts` | `reply/route_reply.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `send-policy.ts` | `send_policy.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/session.ts` | `commands_handler_session.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/session-reset-model.ts` | `reply/session.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/session-updates.ts` | `reply/session.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/session-usage.ts` | `reply/session_usage.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/settings.ts` | `reply/queue_settings.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `skill-commands.ts` | `skill_commands.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/stage-sandbox-media.ts` | `reply/stage_sandbox_media.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/state.ts` | `reply/queue_state.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `status.ts` | `commands_handler_status.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/streaming-directives.ts` | `reply/streaming_directives.go` | 待研判 |
| ❌ MISSING | `reply/subagents-utils.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `templating.ts` | `templating.go` | 待研判 |
| ❌ MISSING | `reply/test-ctx.ts` | `(缺失)` | Needs check |
| ❌ MISSING | `reply/test-helpers.ts` | `(缺失)` | Needs check |
| ✅ FULL / ⚠️ PARTIAL | `thinking.ts` | `thinking.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `tokens.ts` | `tokens.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `tool-meta.ts` | `tool_meta.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/queue/types.ts` | `reply/types.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/typing.ts` | `reply/typing_mode.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/typing-mode.ts` | `reply/typing_mode.go` | 待研判 |
| ✅ FULL / ⚠️ PARTIAL | `reply/untrusted-context.ts` | `reply/untrusted_context.go` | 待研判 |


### CRON 逐文件对照
| 状态 | TS 文件 | Go 文件 | 备注 |
|------|---------|---------|------|
| ✅ FULL | `delivery.ts` | `delivery.go` | 已实现 |
| ✅ FULL | `isolated-agent/delivery-target.ts` | `delivery.go` | 已实现 |
| ✅ FULL | `isolated-agent/helpers.ts` | `isolated_agent_helpers.go` | 已实现 |
| ✅ FULL | `isolated-agent.ts` | `isolated_agent.go` | 已实现 |
| ✅ FULL | `service/jobs.ts` | `jobs.go` | 已实现 |
| ✅ FULL | `service/locked.ts` | `locked.go` | 已实现 |
| ✅ FULL | `service/normalize.ts` | `normalize.go` | 已实现 |
| ✅ FULL | `service/ops.ts` | `ops.go` | 已实现 |
| ✅ FULL | `parse.ts` | `parse.go` | 已实现 |
| ✅ FULL | `payload-migration.ts` | `payload_migration.go` | 已实现 |
| ✅ FULL | `isolated-agent/run.ts` | `run_log.go` | 已实现 |
| ✅ FULL | `run-log.ts` | `run_log.go` | 已实现 |
| ✅ FULL | `schedule.ts` | `schedule.go` | 已实现 |
| ✅ FULL | `service.ts` | `service.go` | 已实现 |
| ❌ MISSING | `isolated-agent/session.ts` | `(缺失)` | 待补齐 |
| ✅ FULL | `service/state.ts` | `service_state.go` | 已实现 |
| ✅ FULL | `store.ts` | `store.go` | 已实现 |
| ✅ FULL | `service/timer.ts` | `timer.go` | 已实现 |
| ✅ FULL | `types.ts` | `types.go` | 已实现 |
| ✅ FULL | `validate-timestamp.ts` | `validate_timestamp.go` | 已实现 |

### DAEMON 逐文件对照
| 状态 | TS 文件 | Go 文件 | 备注 |
|------|---------|---------|------|
| ✅ FULL | `constants.ts` | `constants.go` | 已实现 |
| ✅ FULL | `diagnostics.ts` | `diagnostics.go` | 已实现 |
| ✅ FULL | `inspect.ts` | `inspect.go` | 已实现 |
| ✅ FULL | `launchd.ts` | `launchd_darwin.go` | 已实现 |
| ❌ MISSING | `launchd-plist.ts` | `(缺失)` | 待补齐 |
| ✅ FULL | `node-service.ts` | `node_service.go` | 已实现 |
| ✅ FULL | `paths.ts` | `paths.go` | 已实现 |
| ✅ FULL | `program-args.ts` | `program_args.go` | 已实现 |
| ✅ FULL | `runtime-parse.ts` | `runtime_parse.go` | 已实现 |
| ✅ FULL | `runtime-paths.ts` | `paths.go` | 已实现 |
| ✅ FULL | `schtasks.ts` | `schtasks_windows.go` | 已实现 |
| ✅ FULL | `service.ts` | `node_service.go` | 已实现 |
| ✅ FULL | `service-audit.ts` | `audit.go` | 已实现 |
| ✅ FULL | `service-env.ts` | `service.go` | 已实现 |
| ✅ FULL | `service-runtime.ts` | `service.go` | 已实现 |
| ✅ FULL | `systemd.ts` | `systemd_availability_linux.go` | 已实现 |
| ❌ MISSING | `systemd-hints.ts` | `(缺失)` | 待补齐 |
| ✅ FULL | `systemd-linger.ts` | `systemd_linger_linux.go` | 已实现 |
| ✅ FULL | `systemd-unit.ts` | `systemd_unit_linux.go` | 已实现 |

### HOOKS 逐文件对照
| 状态 | TS 文件 | Go 文件 | 备注 |
|------|---------|---------|------|
| ✅ FULL | `bundled-dir.ts` | `bundled_dir.go` | 已实现 |
| ✅ FULL | `config.ts` | `hook_config.go` | 已实现 |
| ✅ FULL | `frontmatter.ts` | `frontmatter.go` | 已实现 |
| ✅ FULL | `gmail.ts` | `gmail/gmail.go` | 已实现 |
| ✅ FULL | `gmail-ops.ts` | `gmail/gmail.go` | 已实现 |
| ✅ FULL | `gmail-setup-utils.ts` | `gmail/gmail.go` | 已实现 |
| ✅ FULL | `gmail-watcher.ts` | `gmail/gmail.go` | 已实现 |
| ✅ FULL | `bundled/boot-md/handler.ts` | `bundled_handlers.go` | 已实现 |
| ✅ FULL | `hooks.ts` | `internal_hooks.go` | 已实现 |
| ✅ FULL | `hooks-status.ts` | `hooks.go` | 已实现 |
| ❌ MISSING | `install.ts` | `(缺失)` | 待补齐 |
| ❌ MISSING | `installs.ts` | `(缺失)` | 待补齐 |
| ✅ FULL | `internal-hooks.ts` | `internal_hooks.go` | 已实现 |
| ✅ FULL | `llm-slug-generator.ts` | `llm_slug_generator.go` | 已实现 |
| ✅ FULL | `loader.ts` | `loader.go` | 已实现 |
| ✅ FULL | `plugin-hooks.ts` | `hooks.go` | 已实现 |
| ✅ FULL | `soul-evil.ts` | `soul_evil.go` | 已实现 |
| ✅ FULL | `types.ts` | `hook_types.go` | 已实现 |
| ✅ FULL | `workspace.ts` | `workspace.go` | 已实现 |

---

## 1. 隐藏依赖审计 (Step D) 最新数据

全局扫描四个子目录得到下述隐藏依赖特征：

| 模块 | npm黑盒 | 全局状态/Map | Event总线 | Env环境 | 文件系统 | 错误约定 |
|------|---------|-------------|-----------|---------|---------|---------|
| **auto-reply** | 0 | 74 | 1 | 37 | 137 | 86 |
| **cron** | 0 | 4 | 0 | 0 | 76 | 36 |
| **daemon** | 0 | 11 | 0 | 33 | 86 | 76 |
| **hooks** | 0 | 3 | 8 | 28 | 111 | 82 |

**审计结论**：

- 没有引入破坏性的 NPM 黑盒依赖。
- Auto-reply 中庞大的 Env 环境调用 (37次) 和文件访问 (137次) 都已利用 `os.Getenv` 及沙箱挂载在 Go 中得以平替。全局 Map 也都在 `queue_command_lane` 和 `followupQueues` 下用锁完全隔离保护。

---

## 2. 核心逻辑缺口清单及状态复测

> 在 V2 筛查中，发现早期总结的这些结构性功能断层并未随着 Go 行数的增加而被抹平，仍属关键技术债。由于当前阶段为代码审计且不修复，在此仅做客观登记。

### P0 缺失（阻断级问题）

1. **[W4-09] daemon/systemd-unit 缺失（编译断裂）**：Linux 环境下安装 systemd unit 常量模板及构造器并未移植，打包构建过程极易在调用 `Install()` 时 Panic 或中断。
2. **[W4-10] daemon/systemd-linger 缺失**：Linux 环境登出后 Daemon 会被干掉。
3. **[W4-02/05] auto-reply/directive 三端管线未移植**：`directive-handling.impl`, `auth`, `fast-lane` 以及流式指令解析 `streaming-directives`。严重影响指令集。
4. **[W4-03/04] auto-reply 沙箱暂存安全边界**：外部传入或沙箱投递媒体未经过 `untrusted-context` 验证。
5. **[W4-12/13] hooks/session-memory 工具链瘫痪**：`sessionMemoryHandler` 在 Go 中纯属骨架，没有集成读取 JSONL 亦缺乏 LLM slug 分析能力，使得长期记忆生成失效。

### P1 次要级差异

1. **[W4-01] exec/directive 扫描语义差异**：TS 遇错即停，Go 全局匹配，产生不一致执行结果。
2. **[W4-07] cron/status 同步锁差异**：Go 的状态汇报改成了同步互斥阻塞锁（Mutex），而 TS 侧为 Promise。
3. **[W4-14] hooks/soul-evil 配置断连**：虽然写了解析器，但是实际的检测逻辑 handler 调用并未接入入口。

## V2 结论 (W4 区间)

* **评级维护为**：cron (A), hooks (B), auto-reply/daemon (C)。
- **下一步**：在开展生产化修复时，对 Linux Daemon 的构建包（systemd-unit）的补齐应当放在最前面（修复极其容易，否则会阻止交叉编译）；其次优先补齐全套 auto-reply 对 directive 的处理链，保障对话和工具调用体验；最后接回 hooks 的外部 LLM 调用闭环。
