---
summary: "macOS 应用发布流程和版本管理"
read_when:
  - 发布新版本 macOS 应用
  - 配置 CI/CD 打包流程
title: "发布"
---

# macOS 应用发布

## 发布清单

1. 更新版本号（`Info.plist` 和 `Package.swift`）
2. 在目标架构上构建并测试
3. 代码签名和公证
4. 创建 DMG 或 ZIP 分发包
5. 上传到发布渠道

## 构建发布版本

```bash
cd apps/macos
swift build -c release
```

## 打包

```bash
scripts/package-mac-app.sh
```

此脚本执行：

- Release 构建
- 使用 Developer ID 证书签名
- 创建 DMG 磁盘映像
- 可选：Apple 公证

## 版本兼容性

macOS 应用检查连接的 Go Gateway 版本。确保发布时 CLI/Gateway 和应用版本匹配。

## 架构支持

- Apple Silicon（arm64）：原生支持
- Intel（x86_64）：通过 Universal Binary 支持

## CI/CD

- GitHub Actions 用于自动构建和测试
- 需要 Apple Developer 证书和公证凭证作为 secrets
