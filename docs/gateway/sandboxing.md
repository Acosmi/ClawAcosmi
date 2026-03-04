---
summary: "沙箱工作原理：Rust 原生 OS 级隔离 + Docker 回退"
title: "沙箱（Sandboxing）"
read_when: "了解沙箱机制或调整沙箱配置。"
status: active
---

# 沙箱

> [!IMPORTANT]
> **架构状态**：沙箱由 **Rust** 原生实现（`cli-rust/crates/oa-sandbox/`），
> 提供 OS 级进程隔离，Docker 仅作为回退。Go Gateway 通过 IPC 调用 Rust 沙箱。

OpenAcosmi 使用 **Rust 原生 OS 级沙箱**隔离工具执行，实现最小影响范围。

## 平台后端

| 平台 | 主后端 | 回退 |
|------|--------|------|
| Linux | Landlock + Seccomp（+ Namespaces） | Docker |
| macOS | Seatbelt FFI（`sandbox_init_with_parameters`） | Docker |
| Windows | Restricted Token + Job Object | Docker |

后端选择自动执行（`BackendPreference::Auto`）：优先尝试原生，不可用时降级到 Docker。

可通过 `--backend native` 或 `--backend docker` 强制指定后端。

## 安全等级

三个安全等级（`SecurityLevel`）：

- **L0 Deny**（`deny`）：最大隔离。网络全部拒绝，最小文件系统访问。适合不信任代码执行。
- **L1 Allowlist**（`allowlist`）：白名单受限沙箱。受限网络（仅公网 TCP + DNS），工作区读写。
- **L2 Sandboxed**（`sandboxed`）：完整沙箱权限 + 临时挂载授权。完整网络访问。

## 网络策略

由安全等级确定默认值：

| 安全等级 | 默认网络策略 |
|----------|-------------|
| L0 Deny | `None`（阻止所有 socket 系统调用） |
| L1 Allowlist | `Restricted`（仅出站公网 TCP + DNS，阻止 localhost/LAN） |
| L2 Sandboxed | `Host`（完整宿主机网络访问） |

网络策略不允许**弱化**安全等级的默认值。

## 沙箱范围

- 工具执行（`exec`、`read`、`write`、`edit`、`apply_patch`、`process` 等）。

**不在沙箱内**：Gateway 进程本身、`tools.elevated` 明确允许在宿主机运行的工具。

## 配置

```json5
{
  agents: {
    defaults: {
      sandbox: {
        mode: "non-main",     // off | non-main | all
        scope: "session",      // session | agent | shared
        workspaceAccess: "none", // none | ro | rw
      },
    },
  },
}
```

## 挂载规格

通过 `MountSpec` 将宿主机路径绑定到沙箱内：

```json5
{
  mounts: [
    { host_path: "/home/user/source", sandbox_path: "/source", mode: "readonly" }
  ],
}
```

## 资源限制

```json5
{
  resource_limits: {
    memory_bytes: 1073741824,  // 1GB
    cpu_millicores: 2000,       // 2 核
    max_pids: 100,
    timeout_secs: 300,
  },
}
```

## 安全验证

Rust 沙箱在执行前进行严格验证：

- 命令非空且无 null 字节
- 工作区路径为绝对路径
- 无路径遍历攻击（`..`）
- 网络策略不弱化安全等级
- 阻止危险环境变量（`LD_PRELOAD`、`DYLD_INSERT_LIBRARIES` 等）

## CLI 命令

```bash
openacosmi sandbox explain               # 查看当前生效的沙箱配置
openacosmi sandbox explain --session agent:main:main
openacosmi sandbox explain --json
```

## 相关文档

- [沙箱 vs 工具策略 vs 提权](/gateway/sandbox-vs-tool-policy-vs-elevated)
- [安全](/gateway/security)
