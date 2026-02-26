# V2-W3 实施跟踪清单 (Channels)

> 关联审计报告: `global-audit-v2-W3.md`
> 评级: **A** (历史阻尼彻底扫清)

## 任务目标

基于 V2 深度审计结果，跟踪 W3 大板块 (Channels 消息通道) 的实现情况。W3 板块的全部 P0/P1 问题（特别是 LINE 和 iMessage）已被全数修复。

## 实施清单 (已修复验证清单)

### [P0] 阻断级缺陷 (已清零)

- [x] **LINE bot-handlers 事件策略门控**: `ShouldProcessLineEvent` 中的 DM、群组路由鉴权、Pairing 门控验证通过。
- [x] **LINE monitor 初始化及入口通道**: Bot 初始化注册及消息分布闭环在 `monitor.go` 完成。
- [x] **LINE 自动回复投递管线**: 大段分块发送以及快速回复适配在 `auto_reply_delivery.go` 成功落地。

### [P1] 次要级缺陷 (已清零)

- [x] **iMessage 规范化**: `targets.go` 抹平了大小写差异配置。
- [x] **Discord Mention 解析**: `discord/targets.go` 以及 `resolve_users.go` 正确处理 `<@ID>` 转义归一化。
- [x] **WhatsApp 激活策略**: `whatsapp/monitor_inbound.go` 已实现对 `always` 和 `silent_token` 模式的群组策略验证。
- [x] **LINE Rich Menu & Media**: 富文本菜单与多媒体下载管线 (`download.go`) 迁移成功。

## 隐藏依赖审计与验证补充

- [x] 全局态保护：确保剩余的 `sync.Map` 锁机制覆盖了并发调用全生命周期
- [x] 机制迁移：已验证 EventEmitter/chokidar 被 Go Native Channel 完全取代
- [x] Error 传播：确认 `fmt.Errorf` / `errors.Is` 全面对齐
- [x] 外部配置读取：确认为 Config 包注入而非硬编码环境变量读取

## 后续动作

W3 模块目前代码健康度高，具备高度一致的处理范式。无需产生额外的修复任务，进入后续全局端到端联调测试阶段。
