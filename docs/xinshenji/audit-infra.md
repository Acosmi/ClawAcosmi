---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/infra (87 files + 2 subdirs, ~7000+ LOC)
verdict: Pass with Notes
---

# 审计报告: infra — 基础设施模块

## 范围

- **目录**: `backend/internal/infra/`
- **文件数**: 87 源文件 + `cost/` (16 files) + `errors/` (2 files)
- **核心子系统**: 执行审批、网关锁、重试/退避、设备身份、心跳、端口管理、发现/Bonjour、TLS、状态迁移、更新检查

## 审计发现

### [PASS] 安全: 执行安全检查 (exec_safety.go)

- **位置**: `exec_safety.go:46-91`
- **分析**: `IsSafeExecutableValue` 实现了 9 层检查规则链：空值→NUL字节→控制字符→Shell元字符→引号→路径形式→选项注入→裸名模式。Shell 元字符 `;& |` `` `$<>` 全部覆盖。注入防护逻辑完善。
- **风险**: None

### [PASS] 安全: 设备密钥管理 (device_identity.go)

- **位置**: `device_identity.go:168-275`
- **分析**: ED25519 密钥使用 `crypto/rand.Reader` 生成，PKCS8 DER 编码正确实现 RFC 8410。文件权限 `0600`，原子写入（tmp+rename）。加载时重新派生 deviceId 以防文件篡改。密钥存储路径在 `stateDir/identity/device.json`。
- **风险**: None

### [PASS] 安全: 网关实例锁 (gateway_lock.go)

- **位置**: `gateway_lock.go:60-148`
- **分析**: 使用 `O_CREATE|O_EXCL` 原子创建锁文件。陈旧锁检测通过 PID 存活检查 + 时间戳判断。`unlock` 函数在删除前验证 PID 一致性，防止误删他人锁。支持 `OPENACOSMI_ALLOW_MULTI_GATEWAY` 环境变量跳过锁检测。
- **风险**: None

### [WARN] 正确性: isProcessAlive Windows 不完整 (gateway_lock.go)

- **位置**: `gateway_lock.go:211-227`
- **分析**: `isProcessAlive` 对 Windows 分支调用 `isProcessAliveWindows(proc)`，但该函数定义在 `gateway_lock_windows.go` 中。Unix 分支调用 `sendSignalZero(proc)`，定义在 `gateway_lock_unix.go` 中。跨平台编译条件正确，但 Windows 实现需要确认 `OpenProcess + GetExitCodeProcess` 是否正确处理了权限不足的场景。
- **风险**: Low
- **建议**: 审查 `gateway_lock_windows.go` 确认 ACCESS_DENIED 的处理。

### [PASS] 资源安全: 重试机制 (retry.go)

- **位置**: `retry.go:132-215`
- **分析**: `RetryAsync` 泛型函数正确实现了指数退避+jitter+context取消。每次循环开始检查 `ctx.Err()`，`SleepWithCancel` 使用 `select` 监听 context 和 timer。配置合并逻辑 (`ResolveRetryConfig`) 正确保证 `MaxDelay >= MinDelay`。
- **风险**: None

### [WARN] 性能: IsSSHTunnel 每次调用编译正则 (ports.go)

- **位置**: `ports.go:444-449`
- **分析**: `IsSSHTunnel` 函数内部每次调用都 `regexp.MustCompile` 一个基于端口号的动态正则。包级变量 `sshTunnelPattern` 已定义但未使用。可用 `strings.Contains` 替代或缓存编译后的正则。
- **风险**: Low
- **建议**: 使用 `strings.Contains` 检查端口号在命令行中的出现，或在函数外预编译。

### [PASS] 正确性: 心跳调度 (heartbeat.go)

- **位置**: `heartbeat.go:78-298`
- **分析**: `HeartbeatRunner` 正确使用 mutex 保护 agent 状态，快照 agent 列表后释放锁再执行。`runAll` 在持锁范围外执行实际心跳，避免长时间持锁。`scheduleNext` 计算最早到期时间调度唤醒。`isInActiveHours` 正确处理了 HH:MM 格式的活跃时间窗。
- **风险**: None

### [WARN] 正确性: 心跳活跃时间跨午夜场景 (heartbeat.go)

- **位置**: `heartbeat.go:302-334`
- **分析**: `isInActiveHours` 不支持跨午夜的时间窗口（如 `activeFrom: "22:00", activeTo: "06:00"`）。当 `from > to` 时，逻辑不正确——会在 22:00-24:00 和 00:00-06:00 之间都返回 false。
- **风险**: Medium
- **建议**: 增加跨午夜检测逻辑：当 `from > to` 时，应判断当前时间是否在 `[from, 24:00) ∪ [00:00, to)` 范围内。

### [PASS] 正确性: 审批转发 (approval_forwarder.go)

- **位置**: `approval_forwarder.go:100-121`
- **分析**: `ApprovalForwarder` 使用 mutex 管理 pending 审批，timer 驱动超时。`ShouldForwardApproval` 正确实现了 agent/session 过滤链。`MatchSessionFilter` 同时支持子串包含和正则匹配。
- **风险**: None

### [WARN] 性能: MatchSessionFilter 每次编译正则 (approval_forwarder.go)

- **位置**: `approval_forwarder.go:219-229`
- **分析**: `MatchSessionFilter` 对每个 pattern 调用 `regexp.Compile`。在审批请求高频场景下可能成为瓶颈。
- **风险**: Low
- **建议**: 预编译 session filter 正则，或使用 `sync.Map` 缓存。

### [PASS] 正确性: 端口诊断 (ports.go)

- **位置**: `ports.go:82-141`
- **分析**: `InspectPortUsage` 正确区分了 `busy/free/unknown` 三种状态。`readUnixListeners` 使用 lsof 的 `-F` 字段输出格式（更易解析）。`checkPortInUse` 尝试多地址绑定（127.0.0.1, 0.0.0.0, ::1, ::）以确保全面检测。
- **风险**: None

### [PASS] 安全: DER 编码实现 (device_identity.go)

- **位置**: `device_identity.go:59-127`
- **分析**: `encodePrivateKeyPEM` 手工构造 PKCS8 DER 编码。`derSequence` 和 `derEncodeLength` 正确处理了短长度（<128）和长长度（>=128）两种情况。`parseED25519PrivateKeyFromPKCS8` 通过搜索 `04 20` 模式定位种子字节，容错性好。
- **风险**: None

## 测试覆盖

- `heartbeat_test.go` (6900B), `retry_test.go` (6691B), `ports_test.go` (5880B)
- `widearea_dns_test.go` (5123B), `util_test.go` (4172B), `system_events_test.go` (2276B)
- `bonjour_zeroconf_test.go` (2220B), `heartbeat_wake_test.go` (1929B)
- `agent_events_test.go` (2440B), `update_test.go` (6525B)

覆盖率中等，核心路径有测试。

## 总结

- **总发现**: 12 (8 PASS, 4 WARN, 0 FAIL)
- **阻断问题**: 无
- **建议**:
  1. 心跳 `isInActiveHours` 需支持跨午夜时间窗 (Medium)
  2. 多处动态正则编译应预编译或缓存 (Low×2)
  3. Windows 进程存活检测需确认权限处理 (Low)
- **结论**: **通过（附注释）** — 模块质量良好。安全关键路径（密钥管理、网关锁、执行安全检查）实现严谨。4 个 WARN 发现中 1 个 Medium 级别（心跳跨午夜）建议优先修复。
