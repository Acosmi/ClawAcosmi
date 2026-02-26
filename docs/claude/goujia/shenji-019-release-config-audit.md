> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 审计报告 019: 发布包配置调整方案

- **日期**: 2026-02-25
- **前置**: `shenji-018-cli-gap-analysis.md`
- **范围**: npm/Docker/安装脚本/GitHub Release 发布配置

---

## 一、现状

### 1.1 当前分发架构

| 组件 | 分发渠道 | 格式 |
|------|----------|------|
| TS CLI (`openacosmi`) | npm registry | Node.js 包 (`npm install -g openacosmi`) |
| Go Gateway (`acosmi`) | GitHub Release | 多平台二进制 |
| Rust CLI (`openacosmi`) | GitHub Release | 多平台二进制（已配置 release.yml） |

### 1.2 安装脚本（外部托管 openacosmi.ai）

| 脚本 | 平台 | 当前行为 |
|------|------|----------|
| `install.sh` | macOS/Linux/WSL | 安装 Node.js + `npm install -g openacosmi` |
| `install-cli.sh` | macOS/Linux/WSL | 本地安装到 `~/.openacosmi` |
| `install.ps1` | Windows | 安装 Node.js + `npm install -g openacosmi` |

### 1.3 Docker

| 镜像 | Dockerfile | 入口 |
|------|-----------|------|
| `openacosmi-gateway` | `Dockerfile.gateway` (Go) | `acosmi` |
| `openacosmi` (legacy) | `Dockerfile` (Node.js) | TS CLI |

### 1.4 npm 包结构

```
package.json:
  name: "openacosmi"
  bin: { "openacosmi": "openacosmi.mjs" }
  files: [ "dist/", "extensions/", "assets/", "docs/", "skills/" ]
  prepack: "pnpm build && pnpm ui:build"
```

兼容 shim 包: `packages/clawdbot`, `packages/moltbot` → 依赖 `openacosmi` workspace。

---

## 二、目标状态

```
用户安装: curl https://openacosmi.ai/install.sh | bash
    ↓
安装脚本检测 OS/Arch → 从 GitHub Release 下载 Rust 二进制 (openacosmi)
    ↓
Rust CLI (openacosmi) ──── WebSocket RPC ────→ Go Gateway (acosmi)
```

---

## 三、需要变更的文件

### 3.1 仓库内文件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `package.json` | 修改 | 添加 `postinstall` 脚本下载 Rust 二进制；保留 npm 分发通道作为安装器 |
| `scripts/install-rust-binary.mjs` | 新增 | npm postinstall 钩子：检测平台 → 从 GitHub Release 下载对应 Rust 二进制 |
| `scripts/release-check.ts` | 修改 | 添加 Rust 二进制产物校验 |
| `docs/install/installer.md` | 修改 | 更新安装说明 |
| `.github/workflows/release.yml` | 修改 | 添加 SHA256 校验和文件生成 |

### 3.2 仓库外文件（需手动更新）

| 文件 | 说明 |
|------|------|
| `openacosmi.ai/install.sh` | 改为直接下载 Rust 二进制 |
| `openacosmi.ai/install-cli.sh` | 改为直接下载 Rust 二进制 |
| `openacosmi.ai/install.ps1` | 改为直接下载 Rust 二进制 |

---

## 四、建议方案

### 方案 A: npm 包作为安装器（推荐）

保留 `npm install -g openacosmi`，但 `postinstall` 钩子自动下载 Rust 二进制。

**优势**:
- 用户安装体验不变
- npm 生态兼容性保留
- 渐进式迁移，无破坏性变更

**实现**:
1. `package.json` 添加 `"postinstall": "node scripts/install-rust-binary.mjs"`
2. `scripts/install-rust-binary.mjs` 检测 `process.platform` + `process.arch` → 下载对应二进制到 `node_modules/.bin/`
3. `openacosmi.mjs` 作为 fallback wrapper：优先执行 Rust 二进制，Node.js 不存在时 fallback 到 TS 实现

### 方案 B: 纯二进制分发

安装脚本直接下载 Rust 二进制，不经过 npm。

**优势**: 不依赖 Node.js
**劣势**: 破坏现有 `npm install -g` 安装路径

---

## 五、代码草稿

### 5.1 `scripts/install-rust-binary.mjs`

```javascript
#!/usr/bin/env node
import { execSync } from 'child_process';
import { createWriteStream, chmodSync, existsSync, mkdirSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import https from 'https';

const VERSION = process.env.OPENACOSMI_VERSION || 'latest';
const REPO = 'anthropic/open-acosmi';
const BINARY_NAME = 'openacosmi';

const PLATFORM_MAP = {
  'darwin-arm64':  'aarch64-apple-darwin',
  'darwin-x64':    'x86_64-apple-darwin',
  'linux-arm64':   'aarch64-unknown-linux-gnu',
  'linux-x64':     'x86_64-unknown-linux-gnu',
};

async function main() {
  const key = `${process.platform}-${process.arch}`;
  const target = PLATFORM_MAP[key];
  if (!target) {
    console.warn(`[openacosmi] No prebuilt binary for ${key}, falling back to TypeScript CLI`);
    process.exit(0);
  }

  const binDir = join(dirname(fileURLToPath(import.meta.url)), '..', 'bin');
  if (!existsSync(binDir)) mkdirSync(binDir, { recursive: true });

  const dest = join(binDir, BINARY_NAME);
  const url = `https://github.com/${REPO}/releases/${VERSION === 'latest' ? 'latest/download' : `download/v${VERSION}`}/${BINARY_NAME}-${target}`;

  console.log(`[openacosmi] Downloading Rust CLI for ${target}...`);
  // Download logic with retry and checksum verification...
}

main().catch(err => {
  console.warn(`[openacosmi] Failed to download binary: ${err.message}`);
  console.warn('[openacosmi] Falling back to TypeScript CLI');
  process.exit(0); // Don't fail npm install
});
```

### 5.2 `release.yml` 添加 checksum

```yaml
      - name: Generate checksums
        run: |
          cd release-artifacts
          sha256sum * > SHA256SUMS.txt

      - name: Upload checksums
        uses: softprops/action-gh-release@v2
        with:
          files: release-artifacts/SHA256SUMS.txt
```

---

## 六、阻断等待

以上为发布包配置调整方案。等待用户指令：
- **同意补全**: 执行方案 A（npm 作为安装器 + postinstall 下载 Rust 二进制）
- **同意方案 B**: 执行纯二进制分发
- **调整**: 指定其他方案
