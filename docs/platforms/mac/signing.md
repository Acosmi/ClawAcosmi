---
summary: "macOS 应用代码签名要求和流程"
read_when:
  - 签名 macOS 应用
  - 调试签名问题
title: "签名"
---

# macOS 应用签名

## 为何需要签名

- TCC 权限持久化需要稳定的代码签名。
- 未签名/临时签名的构建每次重建都会重置权限。
- 分发需要 Developer ID 签名 + Apple 公证。

## 签名类型

| 类型 | 用途 | 权限持久化 |
| ---- | ---- | --------- |
| Apple Development | 本地开发/测试 | ✅ |
| Developer ID | 分发 | ✅ |
| 临时签名（ad-hoc） | 快速本地运行 | ❌ |

## 开发签名

```bash
SIGN_IDENTITY="Apple Development: <Your Name> (<TEAMID>)" scripts/restart-mac.sh
```

## 无签名构建

```bash
scripts/restart-mac.sh --no-sign
```

说明：

- 无签名构建写入 `~/.openacosmi/disable-launchagent` 防止 launchd 指向未签名二进制。
- 签名构建会自动清除此标记。
- 手动重置：`rm ~/.openacosmi/disable-launchagent`

## 公证（分发必需）

```bash
xcrun notarytool submit OpenAcosmi.dmg --apple-id <email> --team-id <TEAMID> --password <app-specific-password>
```
