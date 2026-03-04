---
summary: "Stable、Beta 和 Dev 频道：语义、切换和标记"
read_when:
  - 需要在 stable/beta/dev 之间切换
  - 需要标记或发布预发版
title: "开发频道"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# 开发频道

最后更新：2026-01-21

OpenAcosmi 提供三个更新频道：

- **stable**：正式稳定版本。
- **beta**：测试中的构建版本。
- **dev**：`main` 分支的最新代码。

我们将构建发布到 **beta**，测试后将经验证的版本**提升为正式版本**。

## Switching channels

Git checkout:

```bash
openacosmi update --channel stable
openacosmi update --channel beta
openacosmi update --channel dev
```

- `stable`/`beta` check out the latest matching tag (often the same tag).
- `dev` switches to `main` and rebases on the upstream.

预编译二进制更新：

```bash
openacosmi update --channel stable
openacosmi update --channel beta
openacosmi update --channel dev
```

这将通过下载对应频道的预编译二进制文件来更新。

当您使用 `--channel` 显式切换频道时，OpenAcosmi 也会对齐安装方式：

- `dev` 确保为 git 检出（默认 `~/openacosmi`，可通过 `OPENACOSMI_GIT_DIR` 覆盖），更新并从该检出构建 CLI。
- `stable`/`beta` 下载对应版本的预编译二进制。

Tip: if you want stable + dev in parallel, keep two clones and point your gateway at the stable one.

## Plugins and channels

When you switch channels with `openacosmi update`, OpenAcosmi also syncs plugin sources:

- `dev` prefers bundled plugins from the git checkout.
- `stable` and `beta` restore npm-installed plugin packages.

## Tagging best practices

- Tag releases you want git checkouts to land on (`vYYYY.M.D` or `vYYYY.M.D-<patch>`).
- Keep tags immutable: never move or reuse a tag.
- npm dist-tags remain the source of truth for npm installs:
  - `latest` → stable
  - `beta` → candidate build
  - `dev` → main snapshot (optional)

## macOS app availability

Beta and dev builds may **not** include a macOS app release. That’s OK:

- The git tag and npm dist-tag can still be published.
- Call out “no macOS build for this beta” in release notes or changelog.
