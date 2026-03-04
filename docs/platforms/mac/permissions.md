---
summary: "macOS 权限持久化（TCC）和签名要求"
read_when:
  - 调试丢失或卡住的 macOS 权限提示
  - 打包或签名 macOS 应用
  - 更改 bundle ID 或应用安装路径
title: "macOS 权限"
---

# macOS 权限（TCC）

macOS 权限授权是脆弱的。TCC 将权限授权与应用的代码签名、bundle identifier 和磁盘路径关联。如果这些发生变化，macOS 会将应用视为新应用，可能丢弃或隐藏提示。

## 稳定权限的要求

- 相同路径：从固定位置运行应用（对 OpenAcosmi 为 `dist/OpenAcosmi.app`）。
- 相同 bundle identifier：更改 bundle ID 会创建新的权限身份。
- 签名应用：未签名或临时签名的构建不会持久化权限。
- 一致的签名：使用真实的 Apple Development 或 Developer ID 证书，使签名在重新构建时保持稳定。

临时签名每次构建都生成新身份。macOS 会忘记之前的授权，提示可能完全消失，直到清除过期条目。

## 提示消失时的恢复清单

1. 退出应用。
2. 在系统设置 → 隐私与安全性中移除应用条目。
3. 从相同路径重新启动应用并重新授予权限。
4. 如提示仍未出现，使用 `tccutil` 重置 TCC 条目后重试。
5. 某些权限仅在完全重启 macOS 后才会重新出现。

示例重置（按需替换 bundle ID）：

```bash
sudo tccutil reset Accessibility bot.molt.mac
sudo tccutil reset ScreenCapture bot.molt.mac
sudo tccutil reset AppleEvents
```

## 文件和文件夹权限（桌面/文稿/下载）

macOS 还可能对终端/后台进程限制桌面、文稿和下载文件夹的访问。如果文件读取或目录列表挂起，为执行文件操作的进程上下文授予访问权限。

解决方法：将文件移入 OpenAcosmi 工作区（`~/.openacosmi/workspace`）以避免按文件夹的授权。

测试权限时始终使用真实证书签名。临时构建仅适用于不关心权限的快速本地运行。
