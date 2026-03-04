---
summary: "OpenAcosmi 如何在 macOS 应用中为设备型号标识符提供友好名称。"
read_when:
  - 更新设备型号标识符映射或 NOTICE/许可证文件
  - 修改 Instances UI 中的设备名称显示
title: "设备型号数据库"
status: active
arch: rust-cli+go-gateway
---

# 设备型号数据库（友好名称）

macOS 伴侣应用在 **Instances** UI 中通过映射 Apple 型号标识符（如 `iPad16,6`、`Mac16,6`）为人类可读名称来显示友好的 Apple 设备型号名称。

映射以 JSON 文件形式存放在：

- `apps/macos/Sources/OpenAcosmi/Resources/DeviceModels/`

## 数据来源

我们目前使用 MIT 许可的仓库：

- `kyle-seongwoo-jun/apple-device-identifiers`

为保持构建确定性，JSON 文件固定到特定的上游 commit（记录在 `apps/macos/Sources/OpenAcosmi/Resources/DeviceModels/NOTICE.md`）。

## 更新数据库

1. 选择要固定的上游 commit（iOS 和 macOS 各一个）。
2. 更新 `apps/macos/Sources/OpenAcosmi/Resources/DeviceModels/NOTICE.md` 中的 commit hash。
3. 重新下载 JSON 文件，固定到这些 commit：

```bash
IOS_COMMIT="<commit sha for ios-device-identifiers.json>"
MAC_COMMIT="<commit sha for mac-device-identifiers.json>"

curl -fsSL "https://raw.githubusercontent.com/kyle-seongwoo-jun/apple-device-identifiers/${IOS_COMMIT}/ios-device-identifiers.json" \
  -o apps/macos/Sources/OpenAcosmi/Resources/DeviceModels/ios-device-identifiers.json

curl -fsSL "https://raw.githubusercontent.com/kyle-seongwoo-jun/apple-device-identifiers/${MAC_COMMIT}/mac-device-identifiers.json" \
  -o apps/macos/Sources/OpenAcosmi/Resources/DeviceModels/mac-device-identifiers.json
```

1. 确保 `apps/macos/Sources/OpenAcosmi/Resources/DeviceModels/LICENSE.apple-device-identifiers.txt` 仍与上游匹配。
2. 验证 macOS 应用构建正常：

```bash
swift build --package-path apps/macos
```
