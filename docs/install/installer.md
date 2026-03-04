---
summary: "安装脚本工作原理（install.sh、install-cli.sh、install.ps1）、标志和自动化"
read_when:
  - 需要了解 `openacosmi.ai/install.sh`
  - 需要自动化安装（CI / 无头环境）
  - 需要从 GitHub 检出安装
title: "安装器内部机制"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。推荐使用 `install-binary.sh` 直接下载预编译二进制文件。

# 安装器内部机制

OpenAcosmi ships four installer scripts, served from `openacosmi.ai`.

| Script                                           | Platform             | What it does                                                                                 |
| ------------------------------------------------ | -------------------- | -------------------------------------------------------------------------------------------- |
| [`install-binary.sh`](#install-binarysh)         | macOS / Linux        | Downloads the prebuilt Rust CLI binary. No Node.js required. **(Recommended)**               |
| [`install.sh`](#installsh)                       | macOS / Linux / WSL  | Installs Node if needed, installs OpenAcosmi via npm (default) or git, and can run onboarding. |
| [`install-cli.sh`](#install-clish)               | macOS / Linux / WSL  | Installs Node + OpenAcosmi into a local prefix (`~/.openacosmi`). No root required.              |
| [`install.ps1`](#installps1)                     | Windows (PowerShell) | Installs Node if needed, installs OpenAcosmi via npm (default) or git, and can run onboarding. |

## Quick commands

<Tabs>
  <Tab title="Binary (recommended)">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-binary.sh | bash
    ```

    Downloads the prebuilt Rust CLI binary directly. No Node.js required.

  </Tab>
  <Tab title="install.sh (npm)">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash
    ```

    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash -s -- --help
    ```

    Installs via npm. The `postinstall` hook automatically downloads the Rust binary.

  </Tab>
  <Tab title="install-cli.sh">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-cli.sh | bash
    ```

    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-cli.sh | bash -s -- --help
    ```

  </Tab>
  <Tab title="install.ps1">
    ```powershell
    iwr -useb https://openacosmi.ai/install.ps1 | iex
    ```

    ```powershell
    & ([scriptblock]::Create((iwr -useb https://openacosmi.ai/install.ps1))) -Tag beta -NoOnboard -DryRun
    ```

  </Tab>
</Tabs>

<Note>
If install succeeds but `openacosmi` is not found in a new terminal, see [Node.js troubleshooting](/install/node#troubleshooting).
</Note>

---

## install-binary.sh

<Tip>
Recommended for most installs. Downloads a native binary — no Node.js required.
</Tip>

### Flow (install-binary.sh)

<Steps>
  <Step title="Detect OS and architecture">
    Supports macOS (x86_64, arm64) and Linux (x86_64, aarch64).
  </Step>
  <Step title="Resolve version">
    Defaults to `latest` GitHub release. Use `--version` to pin.
  </Step>
  <Step title="Download binary">
    Downloads the prebuilt Rust CLI from GitHub Releases with SHA-256 checksum verification.
  </Step>
  <Step title="Install">
    Places the binary in `~/.local/bin` (or `--dir` override) and checks PATH.
  </Step>
</Steps>

### Examples (install-binary.sh)

<Tabs>
  <Tab title="Default">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-binary.sh | bash
    ```
  </Tab>
  <Tab title="Specific version">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-binary.sh | bash -s -- --version 2026.2.6
    ```
  </Tab>
  <Tab title="Custom directory">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-binary.sh | bash -s -- --dir /usr/local/bin
    ```
  </Tab>
  <Tab title="Dry run">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-binary.sh | bash -s -- --dry-run
    ```
  </Tab>
</Tabs>

<AccordionGroup>
  <Accordion title="Flags reference">

| Flag                | Description                                       |
| ------------------- | ------------------------------------------------- |
| `--version <ver>`   | Version to install (default: `latest`)            |
| `--dir <path>`      | Install directory (default: `~/.local/bin`)        |
| `--no-onboard`      | Skip post-install onboarding                      |
| `--dry-run`         | Print actions without executing                   |
| `--help`            | Show usage                                        |

  </Accordion>

  <Accordion title="Environment variables reference">

| Variable                        | Description                             |
| ------------------------------- | --------------------------------------- |
| `OPENACOSMI_VERSION=<ver>`        | Version to install                      |
| `OPENACOSMI_INSTALL_DIR=<path>`   | Install directory                       |
| `OPENACOSMI_BINARY_MIRROR=<url>`  | Custom mirror URL                       |
| `OPENACOSMI_NO_ONBOARD=1`         | Skip onboarding                         |
| `OPENACOSMI_DRY_RUN=1`            | Dry run mode                            |

  </Accordion>
</AccordionGroup>

---

## install.sh

<Tip>
Recommended for most interactive installs on macOS/Linux/WSL.
</Tip>

### Flow (install.sh)

<Steps>
  <Step title="Detect OS">
    Supports macOS and Linux (including WSL). If macOS is detected, installs Homebrew if missing.
  </Step>
  <Step title="Ensure Node.js 22+">
    Checks Node version and installs Node 22 if needed (Homebrew on macOS, NodeSource setup scripts on Linux apt/dnf/yum).
  </Step>
  <Step title="Ensure Git">
    Installs Git if missing.
  </Step>
  <Step title="Install OpenAcosmi">
    - `npm` method (default): global npm install
    - `git` method: clone/update repo, install deps with pnpm, build, then install wrapper at `~/.local/bin/openacosmi`
  </Step>
  <Step title="Post-install tasks">
    - Runs `openacosmi doctor --non-interactive` on upgrades and git installs (best effort)
    - Attempts onboarding when appropriate (TTY available, onboarding not disabled, and bootstrap/config checks pass)
    - Defaults `SHARP_IGNORE_GLOBAL_LIBVIPS=1`
  </Step>
</Steps>

### Source checkout detection

If run inside an OpenAcosmi checkout (`Cargo.toml` + `backend/`), the script offers:

- use checkout (`git`), or
- use binary install

If no TTY is available and no install method is set, it defaults to `npm` and warns.

The script exits with code `2` for invalid method selection or invalid `--install-method` values.

### Examples (install.sh)

<Tabs>
  <Tab title="Default">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash
    ```
  </Tab>
  <Tab title="Skip onboarding">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash -s -- --no-onboard
    ```
  </Tab>
  <Tab title="Git install">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash -s -- --install-method git
    ```
  </Tab>
  <Tab title="Dry run">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash -s -- --dry-run
    ```
  </Tab>
</Tabs>

<AccordionGroup>
  <Accordion title="Flags reference">

| Flag                            | Description                                                |
| ------------------------------- | ---------------------------------------------------------- |
| `--install-method npm\|git`     | Choose install method (default: `npm`). Alias: `--method`  |
| `--npm`                         | Shortcut for npm method                                    |
| `--git`                         | Shortcut for git method. Alias: `--github`                 |
| `--version <version\|dist-tag>` | npm version or dist-tag (default: `latest`)                |
| `--beta`                        | Use beta dist-tag if available, else fallback to `latest`  |
| `--git-dir <path>`              | Checkout directory (default: `~/openacosmi`). Alias: `--dir` |
| `--no-git-update`               | Skip `git pull` for existing checkout                      |
| `--no-prompt`                   | Disable prompts                                            |
| `--no-onboard`                  | Skip onboarding                                            |
| `--onboard`                     | Enable onboarding                                          |
| `--dry-run`                     | Print actions without applying changes                     |
| `--verbose`                     | Enable debug output (`set -x`, npm notice-level logs)      |
| `--help`                        | Show usage (`-h`)                                          |

  </Accordion>

  <Accordion title="Environment variables reference">

| Variable                                    | Description                                   |
| ------------------------------------------- | --------------------------------------------- |
| `OPENACOSMI_INSTALL_METHOD=git\|npm`          | Install method                                |
| `OPENACOSMI_VERSION=latest\|next\|<semver>`   | npm version or dist-tag                       |
| `OPENACOSMI_BETA=0\|1`                        | Use beta if available                         |
| `OPENACOSMI_GIT_DIR=<path>`                   | Checkout directory                            |
| `OPENACOSMI_GIT_UPDATE=0\|1`                  | Toggle git updates                            |
| `OPENACOSMI_NO_PROMPT=1`                      | Disable prompts                               |
| `OPENACOSMI_NO_ONBOARD=1`                     | Skip onboarding                               |
| `OPENACOSMI_DRY_RUN=1`                        | Dry run mode                                  |
| `OPENACOSMI_VERBOSE=1`                        | Debug mode                                    |
| `OPENACOSMI_NPM_LOGLEVEL=error\|warn\|notice` | npm log level                                 |
| `SHARP_IGNORE_GLOBAL_LIBVIPS=0\|1`          | Control sharp/libvips behavior (default: `1`) |

  </Accordion>
</AccordionGroup>

---

## install-cli.sh

<Info>
Designed for environments where you want everything under a local prefix (default `~/.openacosmi`) and no system Node dependency.
</Info>

### Flow (install-cli.sh)

<Steps>
  <Step title="Install local Node runtime">
    Downloads Node tarball (default `22.22.0`) to `<prefix>/tools/node-v<version>` and verifies SHA-256.
  </Step>
  <Step title="Ensure Git">
    If Git is missing, attempts install via apt/dnf/yum on Linux or Homebrew on macOS.
  </Step>
  <Step title="Install OpenAcosmi under prefix">
    Installs with npm using `--prefix <prefix>`, then writes wrapper to `<prefix>/bin/openacosmi`.
  </Step>
</Steps>

### Examples (install-cli.sh)

<Tabs>
  <Tab title="Default">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-cli.sh | bash
    ```
  </Tab>
  <Tab title="Custom prefix + version">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-cli.sh | bash -s -- --prefix /opt/openacosmi --version latest
    ```
  </Tab>
  <Tab title="Automation JSON output">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-cli.sh | bash -s -- --json --prefix /opt/openacosmi
    ```
  </Tab>
  <Tab title="Run onboarding">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-cli.sh | bash -s -- --onboard
    ```
  </Tab>
</Tabs>

<AccordionGroup>
  <Accordion title="Flags reference">

| Flag                   | Description                                                                     |
| ---------------------- | ------------------------------------------------------------------------------- |
| `--prefix <path>`      | Install prefix (default: `~/.openacosmi`)                                         |
| `--version <ver>`      | OpenAcosmi version or dist-tag (default: `latest`)                                |
| `--node-version <ver>` | Node version (default: `22.22.0`)                                               |
| `--json`               | Emit NDJSON events                                                              |
| `--onboard`            | Run `openacosmi onboard` after install                                            |
| `--no-onboard`         | Skip onboarding (default)                                                       |
| `--set-npm-prefix`     | On Linux, force npm prefix to `~/.npm-global` if current prefix is not writable |
| `--help`               | Show usage (`-h`)                                                               |

  </Accordion>

  <Accordion title="Environment variables reference">

| Variable                                    | Description                                                                       |
| ------------------------------------------- | --------------------------------------------------------------------------------- |
| `OPENACOSMI_PREFIX=<path>`                    | Install prefix                                                                    |
| `OPENACOSMI_VERSION=<ver>`                    | OpenAcosmi version or dist-tag                                                      |
| `OPENACOSMI_NODE_VERSION=<ver>`               | Node version                                                                      |
| `OPENACOSMI_NO_ONBOARD=1`                     | Skip onboarding                                                                   |
| `OPENACOSMI_NPM_LOGLEVEL=error\|warn\|notice` | npm log level                                                                     |
| `OPENACOSMI_GIT_DIR=<path>`                   | Legacy cleanup lookup path (used when removing old `Peekaboo` submodule checkout) |
| `SHARP_IGNORE_GLOBAL_LIBVIPS=0\|1`          | Control sharp/libvips behavior (default: `1`)                                     |

  </Accordion>
</AccordionGroup>

---

## install.ps1

### Flow (install.ps1)

<Steps>
  <Step title="Ensure PowerShell + Windows environment">
    Requires PowerShell 5+.
  </Step>
  <Step title="Ensure Node.js 22+">
    If missing, attempts install via winget, then Chocolatey, then Scoop.
  </Step>
  <Step title="Install OpenAcosmi">
    - `npm` method (default): global npm install using selected `-Tag`
    - `git` method: clone/update repo, install/build with pnpm, and install wrapper at `%USERPROFILE%\.local\bin\openacosmi.cmd`
  </Step>
  <Step title="Post-install tasks">
    Adds needed bin directory to user PATH when possible, then runs `openacosmi doctor --non-interactive` on upgrades and git installs (best effort).
  </Step>
</Steps>

### Examples (install.ps1)

<Tabs>
  <Tab title="Default">
    ```powershell
    iwr -useb https://openacosmi.ai/install.ps1 | iex
    ```
  </Tab>
  <Tab title="Git install">
    ```powershell
    & ([scriptblock]::Create((iwr -useb https://openacosmi.ai/install.ps1))) -InstallMethod git
    ```
  </Tab>
  <Tab title="Custom git directory">
    ```powershell
    & ([scriptblock]::Create((iwr -useb https://openacosmi.ai/install.ps1))) -InstallMethod git -GitDir "C:\openacosmi"
    ```
  </Tab>
  <Tab title="Dry run">
    ```powershell
    & ([scriptblock]::Create((iwr -useb https://openacosmi.ai/install.ps1))) -DryRun
    ```
  </Tab>
</Tabs>

<AccordionGroup>
  <Accordion title="Flags reference">

| Flag                      | Description                                            |
| ------------------------- | ------------------------------------------------------ |
| `-InstallMethod npm\|git` | Install method (default: `npm`)                        |
| `-Tag <tag>`              | npm dist-tag (default: `latest`)                       |
| `-GitDir <path>`          | Checkout directory (default: `%USERPROFILE%\openacosmi`) |
| `-NoOnboard`              | Skip onboarding                                        |
| `-NoGitUpdate`            | Skip `git pull`                                        |
| `-DryRun`                 | Print actions only                                     |

  </Accordion>

  <Accordion title="Environment variables reference">

| Variable                           | Description        |
| ---------------------------------- | ------------------ |
| `OPENACOSMI_INSTALL_METHOD=git\|npm` | Install method     |
| `OPENACOSMI_GIT_DIR=<path>`          | Checkout directory |
| `OPENACOSMI_NO_ONBOARD=1`            | Skip onboarding    |
| `OPENACOSMI_GIT_UPDATE=0`            | Disable git pull   |
| `OPENACOSMI_DRY_RUN=1`               | Dry run mode       |

  </Accordion>
</AccordionGroup>

<Note>
If `-InstallMethod git` is used and Git is missing, the script exits and prints the Git for Windows link.
</Note>

---

## CI and automation

Use non-interactive flags/env vars for predictable runs.

<Tabs>
  <Tab title="install.sh (non-interactive npm)">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash -s -- --no-prompt --no-onboard
    ```
  </Tab>
  <Tab title="install.sh (non-interactive git)">
    ```bash
    OPENACOSMI_INSTALL_METHOD=git OPENACOSMI_NO_PROMPT=1 \
      curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash
    ```
  </Tab>
  <Tab title="install-cli.sh (JSON)">
    ```bash
    curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install-cli.sh | bash -s -- --json --prefix /opt/openacosmi
    ```
  </Tab>
  <Tab title="install.ps1 (skip onboarding)">
    ```powershell
    & ([scriptblock]::Create((iwr -useb https://openacosmi.ai/install.ps1))) -NoOnboard
    ```
  </Tab>
</Tabs>

---

## Troubleshooting

<AccordionGroup>
  <Accordion title="Why is Git required?">
    Git is required for `git` install method. For `npm` installs, Git is still checked/installed to avoid `spawn git ENOENT` failures when dependencies use git URLs.
  </Accordion>

  <Accordion title="Why does npm hit EACCES on Linux?">
    Some Linux setups point npm global prefix to root-owned paths. `install.sh` can switch prefix to `~/.npm-global` and append PATH exports to shell rc files (when those files exist).
  </Accordion>

  <Accordion title="sharp/libvips issues">
    The scripts default `SHARP_IGNORE_GLOBAL_LIBVIPS=1` to avoid sharp building against system libvips. To override:

    ```bash
    SHARP_IGNORE_GLOBAL_LIBVIPS=0 curl -fsSL --proto '=https' --tlsv1.2 https://openacosmi.ai/install.sh | bash
    ```

  </Accordion>

  <Accordion title='Windows: "npm error spawn git / ENOENT"'>
    Install Git for Windows, reopen PowerShell, rerun installer.
  </Accordion>

  <Accordion title='Windows: "openacosmi is not recognized"'>
    Run `npm config get prefix`, append `\bin`, add that directory to user PATH, then reopen PowerShell.
  </Accordion>

  <Accordion title="openacosmi not found after install">
    Usually a PATH issue. See [Node.js troubleshooting](/install/node#troubleshooting).
  </Accordion>
</AccordionGroup>
