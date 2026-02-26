---
document_type: Deferred
status: Draft
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: true
---

# SBPL LAN 地址阻断限制

## 问题

macOS Seatbelt 的 SBPL (Sandbox Profile Language) 网络过滤仅支持 `*` 或 `localhost` 作为主机名，**不支持 CIDR 表示法**（如 `127.0.0.0/8`、`10.0.0.0/8`、`192.168.0.0/16`）。

尝试使用 CIDR 会导致 `sandbox_init_with_parameters` 返回 EINVAL:
```
host must be * or localhost in network address
```

## 影响

`NetworkPolicy::Restricted` 模式下：
- `localhost:*` 已正确阻断
- LAN 地址 (10.x, 172.16.x, 192.168.x) **无法通过 SBPL 阻断**
- Unix domain socket 已阻断

## 解决方案（Phase 6 增强）

1. **Network Extension** (macOS 10.15+): 系统级网络过滤，支持 IP 范围
2. **代理方案**: 强制流量通过本地代理，在代理层过滤
3. **`/etc/hosts` 注入**: 临时修改 hosts 文件（需权限）

## 相关代码

- `cli-rust/crates/oa-sandbox/src/macos/seatbelt.rs` `emit_network_rules()` 函数
- 注释标记: `// NOTE: LAN addresses ... cannot be blocked via SBPL alone`
