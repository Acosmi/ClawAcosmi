---
document_type: Deferred
status: Draft
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: false
---

# Linux/Windows 跨平台 CI 编译验证

## 问题

Linux 和 Windows 后端代码使用 `#[cfg(target_os = "...")]` 条件编译门控。当前开发环境为 macOS，这些代码在 macOS 上完全跳过编译，因此无法验证：

1. **编译正确性**: `windows` crate API 签名是否匹配、`libseccomp`/`landlock`/`nix` crate API 是否正确
2. **运行时正确性**: 实际系统调用行为是否符合预期
3. **集成测试**: 沙箱隔离效果是否生效

## 影响

- Linux 后端: `landlock.rs`, `seccomp.rs`, `namespace.rs`, `cgroup.rs`, `mod.rs` — 仅逻辑验证，未编译
- Windows 后端: `job.rs`, `token.rs`, `acl.rs`, `appcontainer.rs`, `mod.rs` — 仅逻辑验证，未编译
- 集成测试: `linux_integration.rs`, `windows_integration.rs` — 0 个测试执行

## 解决方案（Phase 6 CI 管线）

### GitHub Actions 矩阵

```yaml
strategy:
  matrix:
    include:
      - os: ubuntu-24.04
        features: ""
        deps: "sudo apt-get install -y libseccomp-dev"
      - os: macos-latest
        features: ""
      - os: windows-latest
        features: ""
```

### Linux 特殊需求

- 安装 `libseccomp-dev` >= 2.5.0
- Landlock 需要 kernel 5.13+（Ubuntu 24.04 内核 6.8 满足）
- User namespace 可能被 AppArmor 限制（需测试降级路径）

### Windows 特殊需求

- `windows` crate 编译需要 Windows SDK（GitHub Actions Windows runner 已预装）
- 集成测试需管理员权限运行部分场景（Restricted Token, ACL）

## 优先级

高 — Phase 6 首要任务。任何发现的编译错误应立即修复。

## 相关代码

- `cli-rust/crates/oa-sandbox/src/linux/` — 全部文件
- `cli-rust/crates/oa-sandbox/src/windows/` — 全部文件
- `cli-rust/crates/oa-sandbox/tests/linux_integration.rs`
- `cli-rust/crates/oa-sandbox/tests/windows_integration.rs`
