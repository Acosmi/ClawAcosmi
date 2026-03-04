# V2-W1 实施跟踪清单 (Gateway, Security, Config)

> 关联审计报告: `global-audit-v2-W1.md`
> 评级: **A** (符合提测与预发布标准)

## 任务目标

基于 V2 深度审计结果，跟踪 W1 大板块 (Config, Gateway, Security) 的后续跟进项。当前审计结果显示该板块**无功能缺陷 (0项 P0/P1/P2)**。

## 实施清单

### Config 模块

- [x] 配置路径解析逻辑功能对齐 (`paths.go`, `normpaths.go`, `configpath.go`)
- [x] 遗留配置迁移逻辑兼容性对齐 (`legacy*.go`)
- [x] Config 结构体映射与校验完全覆盖 (`schema.go`, `validator.go`)
- [x] 环境变量/配置读取机制对齐 (`loader.go`, `envsubst.go`)

### Gateway 模块

- [x] JSON-RPC/WS 方法注册 1:1 实现
- [x] WebSocket 连接管理与心跳、协议协商对齐
- [x] Preshared-Key / Device Auth 逻辑对接完成
- [x] Hooks 生命周期扩展对齐
- [x] 隐藏依赖审计完成 (WebSocket 广播中心改造等)

### Security 模块

- [x] 全局安全审计项规则扫描对齐 (`audit.go`, `audit_extra.go`)
- [x] 自动修复逻辑实现全面对齐 (`fix.go`)
- [x] 插件恶意代码扫描模式对齐 (`skill_scanner.go`)
- [x] 外部 SSRF 防御机制对齐

## 后续动作

由于 W1 板块已达到 A 级健康标准，本板块不再安排主动重构任务，直接进入下一阶段的集成测试观察期。
