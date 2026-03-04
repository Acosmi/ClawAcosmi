# Argus 签名发布双轨指南（ARGUS-008）

> 适用于 argus-sensory 二进制的签名与分发。

---

## 模式 A：内部自用（开发/测试环境）

### 前提

- 本机编译的 argus-sensory 裸二进制
- 无需 Apple Developer ID

### 流程

1. **创建自签名证书**（仅首次）：

```bash
cd Argus/scripts/package
./create-dev-cert.sh
# 在 Keychain 中创建 "Argus Dev" 代码签名证书
```

1. **自动签名**：
   - 网关启动时 `EnsureCodeSigned()` 自动检测并签名
   - 签名使用 `Argus Dev` 证书 + `com.argus.sensory.mcp` identifier
   - TCC 授权按 identifier 追踪，重新编译后授权不丢失

2. **验证**：

```bash
codesign --verify --verbose=2 /path/to/argus-sensory
# 应显示: valid on disk
```

### 注意事项

- `Argus Dev` 证书仅本机有效，不可分发
- 如果证书不存在，系统回退到 ad-hoc 签名（每次编译需重新授权）

---

## 模式 B：分发模式（Developer ID + Notarization）

### 前提

- Apple Developer Program 会员
- `Developer ID Application` 证书

### 流程

1. **打包 .app bundle**：

```bash
cd Argus
make app
# 产出: build/Argus.app/Contents/MacOS/argus-sensory
```

1. **签名**：

```bash
codesign --force --options runtime \
  -s "Developer ID Application: Your Name (TEAMID)" \
  --entitlements scripts/package/entitlements.plist \
  build/Argus.app
```

1. **公证**：

```bash
# 创建 ZIP
ditto -c -k --keepParent build/Argus.app Argus.zip

# 提交公证
xcrun notarytool submit Argus.zip \
  --apple-id "your@email.com" \
  --team-id "TEAMID" \
  --password "@keychain:AC_PASSWORD" \
  --wait

# 装订
xcrun stapler staple build/Argus.app
```

1. **分发**：
   - 安装到 `/Applications/Argus.app` 或 `~/.openacosmi/Argus.app`
   - Resolver 自动发现 .app bundle 内的二进制

### 标准安装位

| 路径 | 说明 |
|------|------|
| `/Applications/Argus.app` | 系统级安装（需 admin） |
| `~/Applications/Argus.app` | 用户级安装 |
| `~/.openacosmi/Argus.app` | OpenAcosmi 专用安装 |
| `~/.openacosmi/bin/argus-sensory` | 裸二进制/符号链接 |
