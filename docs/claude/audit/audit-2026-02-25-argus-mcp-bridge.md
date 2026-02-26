---
document_type: Audit
status: Archived
created: 2026-02-25
last_updated: 2026-02-25
---

# 代码审计报告：Argus MCP 桥接接入

**审计范围**: 5 个新建文件 + 4 个修改文件（Argus MCP 桥接到网关）

## 审计清单

### 1. `mcpclient/types.go` — MCP 协议类型

| 检查项 | 结果 | 备注 |
|---|---|---|
| 类型安全 | PASS | 所有 JSON 字段使用 `json.RawMessage` 延迟解析，无 `interface{}` 泄露 |
| 协议兼容 | PASS | 版本 `2024-11-05` 匹配 Argus server.go |
| 零值安全 | PASS | `JSONRPCError` 为指针，nil 表示无错误 |

**发现**: 无问题。

---

### 2. `mcpclient/client.go` — MCP stdio 客户端

| 检查项 | 结果 | 备注 |
|---|---|---|
| 并发安全 | PASS | `sync.Map` 做 pending 管理，`sync.Mutex` 保护写入 |
| 资源泄漏 | PASS | `readLoop` 退出时 close(done)，pending 通过 defer Delete 清理 |
| Scanner buffer | PASS | 10MB 匹配 Argus 服务端上限 |
| Context 传播 | PASS | send() 监听 ctx.Done()/c.done/ch 三路 |
| Close 安全 | PASS | 幂等，double-close 不 panic |
| 请求 ID 唯一 | PASS | atomic.Int64 单调递增 |

**发现 F-1（低风险）**: `readLoop` 中 `json.Unmarshal` 失败时静默跳过。对于调试困难场景，可考虑添加 debug 级日志。
- **位置**: `client.go:64`
- **风险**: 低 — 非法行跳过是合理行为，仅影响调试
- **建议**: 可在后续迭代中增加 `slog.Debug` 日志

---

### 3. `argus/bridge.go` — 进程生命周期管理

| 检查项 | 结果 | 备注 |
|---|---|---|
| 状态机完整性 | PASS | init→starting→ready→degraded→stopped，所有转换有锁保护 |
| 进程清理 | PASS | Stop() 先关 stdin → 等 3s → force kill，processMonitor 通过 ctx 取消退出 |
| 僵尸进程 | PASS | spawnAndHandshake 失败时 kill+wait，processMonitor 循环中 wait |
| 重启退避 | PASS | 1s→2s→4s...→60s cap，成功后重置 |
| 并发安全 | PASS | 所有状态读写有 RWMutex 保护 |
| 资源泄漏 | PASS | client.Close() 在所有错误路径都被调用 |

**发现 F-2（低风险）**: `processMonitor` 中 `cmd.Wait()` 可能在 `Stop()` 已经调用 `cmd.Wait()` 后重复调用。
- **位置**: `bridge.go:288` 和 `bridge.go:375`
- **风险**: 低 — Go 的 `cmd.Wait()` 重复调用仅返回错误，不 panic
- **建议**: 可接受，无需修改

**发现 F-3（信息级）**: `spawnAndHandshake` 中 MCP 握手超时硬编码 5s。
- **位置**: `bridge.go:182`
- **风险**: 无 — 对于 stdio 本地进程，5s 足够
- **建议**: 如需远程 MCP Server 支持，可配置化

---

### 4. `argus/skills.go` — 技能条目转换

| 检查项 | 结果 | 备注 |
|---|---|---|
| 输入验证 | PASS | nil tools 返回空切片 |
| 未知工具处理 | PASS | 默认 category="unknown", risk="medium" |
| JSON 兼容性 | PASS | 字段与 gateway skillStatusEntry 完全对齐（单元测试验证） |

**发现**: 无问题。

---

### 5. `server_methods_argus.go` — argus.* RPC 方法

| 检查项 | 结果 | 备注 |
|---|---|---|
| nil bridge 处理 | PASS | handleArgusStatus 返回 available=false |
| 闭包捕获 | PASS | RegisterArgusDynamicMethods 正确捕获 toolName 到闭包局部变量 |
| 超时处理 | PASS | 支持 `_timeout` 参数，默认 30s |
| 错误传播 | PASS | 区分 transport error（500）和 tool error（解析 content） |

**发现 F-4（低风险）**: `_timeout` 参数名用下划线前缀可能不直观。
- **位置**: `server_methods_argus.go:84`
- **风险**: 低 — API 契约问题，不影响安全
- **建议**: 可在文档中说明

---

### 6. `boot.go` 修改

| 检查项 | 结果 | 备注 |
|---|---|---|
| 条件初始化 | PASS | 复用沙箱模式，二进制不存在时跳过 |
| 错误处理 | PASS | Start 失败仅 warn，不阻断网关启动 |
| 路径解析 | PASS | 3 级 fallback：env → ~/.openacosmi/bin → PATH |

**发现**: 无问题。

---

### 7. `server_methods.go` 修改

| 检查项 | 结果 | 备注 |
|---|---|---|
| 权限规则 | PASS | argus.status = read，其他 = write |
| 默认拒绝 | PASS | argus 规则在默认拒绝之前插入 |

**发现**: 无问题。

---

### 8. `server.go` + `ws_server.go` 修改

| 检查项 | 结果 | 备注 |
|---|---|---|
| 注册顺序 | PASS | 静态方法先注册，动态方法后注册（bridge != nil 时） |
| 关闭顺序 | PASS | StopArgus 在 StopSandbox 之后调用 |
| methodCtx 接入 | PASS | ArgusBridge 从 cfg.State.ArgusBridge() 获取 |

**发现**: 无问题。

---

### 9. `server_methods_skills.go` 修改

| 检查项 | 结果 | 备注 |
|---|---|---|
| nil 安全 | PASS | `if bridge := ctx.Context.ArgusBridge; bridge != nil` 保护 |
| 字段映射 | PASS | 逐字段复制，无遗漏 |

**发现**: 无问题。

---

## 安全审计

| 安全检查 | 结果 |
|---|---|
| 路径遍历 | N/A — 无文件路径操作 |
| 命令注入 | PASS — BinaryPath 来自 env/配置，不从用户输入构造 |
| 权限边界 | PASS — argus.* 方法需要 write scope，status 需要 read |
| 输入验证 | PASS — MCP 参数通过 json.Marshal 序列化，无直接字符串拼接 |
| 资源安全 | PASS — 进程清理、fd 清理、stdin pipe 关闭都有保障 |

## 测试覆盖

| 包 | 测试数 | 覆盖范围 |
|---|---|---|
| mcpclient | 9 | Initialize, ListTools, CallTool (成功/MCP错误/RPC错误), Ping, Context取消, Close, 并发 |
| argus | 14 + 2 集成 | IsAvailable, 状态机, CallTool不可用, Skills转换(已知/未知/空/JSON), emoji, slogWriter |

## 审计结论

**裁决: PASS**

4 个发现均为低风险/信息级，不阻塞归档。代码符合 Skill 3 编码规范：
- 无 unwrap/panic
- 资源通过 RAII/defer 清理
- 并发安全（RWMutex + atomic）
- 错误上下文完整

### 发现汇总

| ID | 严重性 | 位置 | 描述 | 处置 |
|---|---|---|---|---|
| F-1 | 低 | client.go:64 | JSON 解析失败静默跳过 | 可接受，后续可加 debug 日志 |
| F-2 | 低 | bridge.go:288/375 | cmd.Wait() 可能重复调用 | 可接受，Go 安全处理 |
| F-3 | 信息 | bridge.go:182 | 握手超时硬编码 5s | 可接受，stdio 本地足够 |
| F-4 | 低 | server_methods_argus.go:84 | _timeout 参数名不直观 | 文档说明即可 |
