---
document_type: Audit
status: Complete
created: 2026-02-28
scope: backend/internal/security (15 files, ~3200+ LOC)
verdict: Pass
---

# 审计报告: security — 安全审计/SSRF防护模块

## 范围

- **目录**: `backend/internal/security/`
- **文件数**: 15 (含 4 个 `_test.go`)
- **核心文件**: `audit.go`(598L), `audit_extra.go`(~900L), `audit_fs.go`(~160L), `ssrf.go`(300L), `external_content.go`(~300L), `fix.go`(~500L), `skill_scanner.go`(~280L), `windows_acl.go`(~260L)

## 审计发现

### [PASS] 安全: SSRF 防护多层级检测 (ssrf.go)

- **位置**: `ssrf.go:117-185`
- **分析**: `SafeFetchURL` 实现 4 层 SSRF 防护:
  1. URL 解析 + 主机名黑名单 (localhost, metadata.google.internal, *.localhost,*.local, *.internal)
  2. 直接 IP 输入的私有 IP 检查
  3. DNS 解析后的 IP 检查（防 DNS rebinding stage 1）
  4. 重定向目标检查（防 open redirect SSRF）
  
  `CreatePinnedHTTPClient` 更进一步在 `Transport.DialContext` 层拦截，对每次实际连接进行 IP 验证。
- **风险**: None

### [PASS] 安全: 私有 IPv4/IPv6 完整覆盖 (ssrf.go)

- **位置**: `ssrf.go:244-274`
- **分析**: `isPrivateIPv4` 覆盖：`0.0.0.0/8`, `10.0.0.0/8`, `127.0.0.0/8`, `169.254.0.0/16`, `172.16.0.0/12`, `192.168.0.0/16`, `100.64.0.0/10 (CGNAT)`。IPv6 覆盖：`::`, `::1`, `fe80:`, `fec0:`, `fc`, `fd` 前缀。`::ffff:` IPv4-mapped 地址递归检查底层 v4 地址。
- **风险**: None

### [PASS] 安全: 安全审计覆盖面 (audit.go)

- **位置**: `audit.go:517-597`
- **分析**: `RunSecurityAudit` 覆盖 14 个检查维度:
  1. 攻击面汇总 2. 云同步目录检测 3. 网关配置 4. 浏览器控制
  2. 日志配置 6. 特权工具 7. 钩子加固 8. 密钥泄露
  3. 模型卫生 10. 小模型风险 11. 插件信任 12. Include 文件权限
  4. 暴露矩阵 14. 深度文件系统检查 + 插件/技能代码安全扫描
- **风险**: None

### [PASS] 安全: 网关配置安全检查 (audit.go)

- **位置**: `audit.go:321-425`
- **分析**:
  - `bind != loopback && !auth` → Critical
  - `tailscale.mode=funnel` → Critical (公网暴露)
  - `allowInsecureAuth` → Critical
  - `dangerouslyDisableDeviceAuth` → Critical
  - Token 长度 < 24 → Warn
  - 覆盖了网关暴露的主要攻击向量。
- **风险**: None

### [PASS] 安全: 文件系统权限检查 (audit.go + audit_fs.go)

- **位置**: `audit.go:235-315`
- **分析**: 检查 stateDir 和 configPath 的权限：
  - 世界可写 → Critical
  - 组可写 → Warn
  - 世界可读（config含token）→ Critical
  - 符号链接 → Warn
  权限检测实现在 `audit_fs.go` 的 `InspectPathPermissions` 中。
- **风险**: None

### [PASS] 安全: 配置依赖注入可测试性 (audit.go)

- **位置**: `audit.go:72-122`
- **分析**: `SecurityAuditOptions` 通过依赖注入传入所有外部快照（GatewayConfig, BrowserConfig, etc），使审计函数完全解耦外部模块，便于单元测试。设计模式优秀。
- **风险**: None

### [PASS] 正确性: SSRF 重定向限制差异 (ssrf.go)

- **位置**: `ssrf.go:163 vs ssrf.go:218`
- **分析**: `SafeFetchURL` 允许 10 次重定向，`CreatePinnedHTTPClient` 限制 3 次。两者适用场景不同：前者用于一次性安全获取，后者用于长期复用客户端。差异合理。
- **风险**: None

### [PASS] 正确性: DNS 解析失败处理 (ssrf.go)

- **位置**: `ssrf.go:146-157`
- **分析**: DNS 解析失败时不阻止请求（`// DNS 解析失败不阻止（可能是直接 IP）`）。这是正确的——直接 IP 输入已在步骤 2 检查过。如果主机名无法解析，后续的 HTTP 连接也会失败，构成天然防护。
- **风险**: None

## 测试覆盖

- `audit_test.go` (13705B) — 审计逻辑测试
- `ssrf_test.go` (4709B) — SSRF 防护测试
- `external_content_test.go` (5566B) — 外部内容安全测试
- `jsonc_test.go` (3531B) — JSONC 解析测试

测试覆盖充分，关键安全路径均有测试。

## 总结

- **总发现**: 7 (7 PASS, 0 WARN, 0 FAIL)
- **阻断问题**: 无
- **结论**: **通过** — 安全模块质量优秀。SSRF 防护实现多层级检测（主机名黑名单+IP检查+DNS解析后检查+重定向检查+DialContext层拦截），覆盖了常见 SSRF 攻击向量。安全审计覆盖 14 个检查维度，依赖注入设计可测试性好。
