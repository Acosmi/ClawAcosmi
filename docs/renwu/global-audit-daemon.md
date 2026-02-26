# daemon 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W8，部分 2 (中型模块)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 19 | 23 | 121.0% |
| 总行数 | 3554 | 2924 | 82.3% |

*(注：Go端针对不同操作系统采用了 `_linux.go`, `_darwin.go`, `_windows.go` 条件编译，文件数增多，但消除了大部分运行时的系统检测样板代码，因此总行数也更精致。)*

## 逐文件对照

| 状态 | TS 文件 | Go 文件 |
|------|---------|---------|
| ✅ FULL | `systemd-hints.ts` | `systemd_hints_linux.go`, `systemd_availability_linux.go`, `systemd_linger_linux.go` |
| ✅ FULL | `service-runtime.ts`, `node-service.ts` | `node_service.go`, `service.go` |
| ✅ FULL | `runtime-parse.ts` | `runtime_parse.go` |
| ✅ FULL | `paths.ts`, `runtime-paths.ts` | `paths.go`, `runtime_paths.go` |
| ✅ FULL | `diagnostics.ts` | `diagnostics.go` |
| ✅ FULL | `constants.ts` | `constants.go` |
| ✅ FULL | `systemd-unit.ts`, `systemd.ts` | `systemd_unit_linux.go`, `systemd_linux.go` |
| ✅ FULL | `service-env.ts` | `service_env.go` |
| ✅ FULL | `program-args.ts` | `program_args.go` |
| ✅ FULL | `service-audit.ts`, `inspect.ts` | `audit.go`, `inspect.go` |
| ✅ FULL | `schtasks.ts` | `schtasks_windows.go` |
| ✅ FULL | `launchd.ts`, `launchd-plist.ts` | `launchd_darwin.go`, `plist_darwin.go` |
| ✅ FULL | `(无)` | `types.go`, `platform_*.go` |

> 评价：Go版重构非常符合云原生或系统级编程的最佳实践，充分运用了 `//go:build` 条件编译分离 Systemd、Launchd 和 Schtasks 逻辑，这不仅杜绝了跨平台问题，也降低了代码运行时的判断开销。

## 隐藏依赖审计

| # | 类别 | 检查结果 | Go端实现方案 |
|---|------|----------|-------------|
| 1 | npm 包黑盒行为 | ✅ 无 | 使用了本地操作系统的内建命令 `systemctl`, `launchctl`, `schtasks` |
| 2 | 全局状态/单例 | ✅ 无 | 完全无副作用的结构体设计 |
| 3 | 事件总线/回调链 | ✅ 无 | 命令行调用均为同步或明确等待 |
| 4 | 环境变量依赖 | ⚠️ `service-env.ts` 高度依赖环境变量组合用于守护进程 | Go 端 `service_env.go` 安全继承了原所有的特殊跨进程变量 |
| 5 | 文件系统约定 | ⚠️ `paths.ts` 处理 `.plist`, `.service` 文件的硬编码 | Go 的 `paths.go` 和各 OS 实现中利用了标准 `filepath` 做规范化 |
| 6 | 协议/消息格式 | ✅ 无 | |
| 7 | 错误处理约定 | ⚠️ Node 环境抛出特有的 Error Code (如 EACCES) | Go 端利用 `os.IsPermission(err)` 等标准方式重写了错误判断 |

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| DAE-1 | 架构重构 | 系统检测逻辑 | `platform_*.go` | 从 Node 的基于 `process.platform` 运行时全量检测变为了针对特定架构编译阶段隔离，显著提升了可读性和安全性。 | P3 | 极佳重构方案，无需修复 |
| DAE-2 | 类型安全 | 分散的方法签名 | `types.go` | Go 抽取出了所有平台统一个接口定义，以接口动态绑定的方式执行服务检测。 | P3 | 接口导向，无需修复 |

## 总结

- P0 差异: 0 项
- P1 差异: 0 项
- P2 差异: 0 项
- **模块审计评级: A** (Go 原本就最擅长系统守护进程交互管理，完美消除了 TS 在跨平台服务操作时的“非原生感”)
