# V2-W5 实施跟踪清单 (Infra / CLI / Media-Understanding / Memory)

> 关联审计报告: `global-audit-v2-W5.md`
> 模块评级: **MEDIA (A) / MEMORY (B+) / INFRA (C-→B) / CLI (D→C+)**
> 最后更新: 2026-02-20

## 任务目标

基于 V2 深度审计结果，跟踪 W5 大板块的关键断点。经逐文件 TS↔Go 深度对照后发现多数项已在后续重构中完成实现。

## 实施清单 (已修复验证)

### [P0] 阻断级缺陷

- [x] **[W5-08] cli 交互指导屏**: `cmd_setup.go` (276L) 已完整实现 setup + onboard 向导，含配置 IO + workspace 创建 + cliPrompter。**审计确认完备**。
- [x] **[W5-09] cli 认证流架构**: `setup_auth_apply.go` (344L) + `setup_auth_options.go` (188L) 覆盖 20+ 提供商 + 分组选择 + OAuth + API key + Device Flow。**审计确认完备**。
- [x] **[W5-06] infra 网络攻防边界**: `internal/security/ssrf.go` (300L) + `ssrf_test.go` (212L) 完整映射 TS `net/ssrf.ts` (309L)，含 IPv4/IPv6/mapped 私有 IP 检测、DNS rebinding 防护。**审计确认完备**。
- [x] **[W5-01] infra 防多开锁**: `gateway_lock.go` (223L) 完整实现。**Windows `isProcessAliveWindows` 从 naive stub 修复为 tasklist PID 检测**。
- [x] **[W5-02] infra 硬件身份存管**: `device_identity.go` (405L) 1:1 映射 TS 180L，含 ED25519 PEM 编解码、PKCS8 序列化、SHA256 设备 ID、0600 权限加密落盘。**审计确认完备**。
- [x] **[W5-05] infra 系统投递管线**: `internal/outbound/` (9 Go 文件, ~76KB) + `outbound_test.go` (214L) 全量覆盖 TS outbound-policy/session/send-service/deliver。**审计确认完备**。

### [P1] 次要级缺陷

- [x] **[W5-13] memory 异步记录队列**: 新建 `sync_sessions.go` (220L)，`manager.go` 扩展 `SessionsDir` + 监听路径。Session 文件变更后可触发 indexer 流水线。**已修复**。

## 隐藏依赖审计与功能对齐验证补充

- [x] 针对 CLI / Infra 中大量的环境变量和文件 IO 操作，确保重写时全部引入安全的跨平台封装。
- [x] 维持并强化 `media` 模块目前稳定表现中的限流及模型路由等隐式并发隔离逻辑。
- [ ] 对缺失的部分 CLI 工具树和向导命令行交互 (`memory-cli`, `logs-cli`) 进行中长期规划与补齐。→ **已登记** `deferred-items.md` W5-D2/D3 (P3)

## 后续动作

W5 核心阻断项已全部修复/确认完备。延迟项详见 [`deferred-items.md`](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)：

- **W5-D1** (P3): Windows `isProcessAliveWindows` 进一步优化为 Windows API 调用
- **W5-D2** (P3): `memory-cli` 子命令补全
- **W5-D3** (P3): `logs-cli` 子命令补全
