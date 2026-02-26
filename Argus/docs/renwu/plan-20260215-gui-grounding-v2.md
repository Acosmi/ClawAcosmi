# GUI Grounding 增强规划

> 前置: GUI Grounding 升级已完成 | 日期: 2026-02-15

## 背景

GUI Grounding 升级已实现 AX 原生检测替代 VLM，但存在三个待解决问题：

1. **权限管理不统一** — Screen Recording 和 Accessibility 权限各自独立检测，无启动引导
2. **Electron/Web 应用检测不完整** — Electron 默认不暴露完整 AX 树，Chrome 网页依赖 ARIA 质量
3. **分发方式原始** — 命令行二进制文件，权限绑定到终端而非应用本身

## 解决方案

### 架构变更

```
升级前:  命令行启动 → 权限分散检测 → 标准 AX 枚举
升级后:  .app 独立应用 → 统一权限引导 → 智能 AX 策略 (原生/Electron/Web 分层)
```

---

## Batch A: .app Bundle + .pkg 安装器

### A1: .app Bundle 结构

创建标准 macOS 应用包结构：

```
Argus.app/
  Contents/
    Info.plist          ← 权限声明 + bundle 元信息
    MacOS/
      argus-sensory     ← Go 编译的主二进制
    Frameworks/
      libargus_core.dylib  ← Rust 动态库
    Resources/
      vlm-config.json   ← VLM 配置 (可选)
      AppIcon.icns       ← 应用图标
```

**Info.plist 关键配置：**

```xml
<key>NSAccessibilityUsageDescription</key>
<string>Argus 需要辅助功能权限来检测屏幕上的 UI 元素，实现智能操作。</string>

<key>NSScreenCaptureUsageDescription</key>  
<string>Argus 需要屏幕录制权限来捕获屏幕内容，用于 AI 视觉理解。</string>

<key>CFBundleIdentifier</key>
<string>com.argus.compound</string>
```

### A2: Makefile target

```makefile
APP_NAME := Argus
APP_DIR  := build/$(APP_NAME).app

app: build
    @echo "📦 Packaging $(APP_NAME).app..."
    mkdir -p $(APP_DIR)/Contents/{MacOS,Frameworks,Resources}
    cp scripts/package/Info.plist $(APP_DIR)/Contents/
    cp $(GO_DIR)/argus-sensory $(APP_DIR)/Contents/MacOS/
    cp $(DYLIB) $(APP_DIR)/Contents/Frameworks/
    install_name_tool -change ... # fix dylib rpath
    @echo "✅ $(APP_DIR) created"
```

### A3: .pkg 安装器

使用 `pkgbuild` + `productbuild` 创建安装包：

```bash
# 1. 构建 component pkg
pkgbuild --root build/Argus.app \
         --identifier com.argus.compound \
         --install-location /Applications/Argus.app \
         build/Argus-component.pkg

# 2. 构建 product pkg (支持安装引导界面)
productbuild --distribution scripts/package/distribution.xml \
             --resources scripts/package/resources/ \
             --package-path build/ \
             build/Argus-Installer.pkg
```

**distribution.xml** 可配置：

- 安装欢迎页 + 许可协议
- 安装位置选择
- 安装后脚本 (引导用户开启权限)

**新增文件清单：**

| 文件 | 说明 |
|------|------|
| `scripts/package/Info.plist` | 应用 bundle 配置 |
| `scripts/package/build-pkg.sh` | .pkg 构建脚本 |
| `scripts/package/distribution.xml` | 安装器界面配置 |
| `scripts/package/resources/welcome.html` | 安装欢迎页 |
| `scripts/package/entitlements.plist` | 代码签名权限声明 |

---

## Batch B: 统一权限检查

### B1: Rust 侧 — `argus_check_permissions`

在 `accessibility.rs` 中新增：

```rust
#[no_mangle]
pub extern "C" fn argus_check_permissions(
    out_json: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    // 返回 JSON:
    // {
    //   "accessibility": true/false,
    //   "screen_recording": true/false  
    // }
}
```

Screen Recording 权限通过 `CGPreflightScreenCaptureAccess()` 检测 (macOS 10.15+)。

### B2: Go 侧 — 启动引导

在 `main.go` 启动时：

```go
func checkPermissions() {
    perms := rustAccessibility.CheckPermissions()
    if !perms.Accessibility {
        fmt.Println("⚠️  请在 系统设置 → 隐私与安全 → 辅助功能 中启用 Argus")
    }
    if !perms.ScreenRecording {
        fmt.Println("⚠️  请在 系统设置 → 隐私与安全 → 屏幕录制 中启用 Argus")
    }
    if !perms.Accessibility || !perms.ScreenRecording {
        fmt.Println("部分功能将受限运行，授权后重启应用即可完全启用")
    }
}
```

---

## Batch C: 混合 AX 策略

### C1: Electron 应用 — 强制 AXManualAccessibility

在 `accessibility.rs` 的 `argus_ax_focused_app` 中增加：

```rust
// 对 Electron 应用，尝试设置 AXManualAccessibility = true
// 这会强制 Chromium 暴露完整的 Accessibility Tree
fn try_enable_manual_accessibility(app_ref: AXUIElementRef) {
    let attr = CFString::new("AXManualAccessibility");
    let value = CFBoolean::true_value();
    AXUIElementSetAttributeValue(app_ref, attr, value);
}
```

识别 Electron 应用的方式：检查进程的 bundle identifier 或可执行文件路径中是否包含 `Electron` / `Chromium` 关键字。

### C2: Chrome/Safari — Web 区域深层递归

增强 `enumerate_elements()` 的递归逻辑：

```
标准枚举: 最大深度 15
Web 区域: 检测到 AXWebArea role 后，
  → 深度计数器重置
  → 最大深度增加到 25
  → 优先枚举 AXLink, AXButton, AXTextField 等交互元素
```

### C3: 智能策略选择

在 `argus_ax_focused_app` 中根据应用类型自动选择策略：

```
1. 获取前台应用 → 读取 bundleId
2. bundleId 匹配 Electron? → 执行 AXManualAccessibility
3. bundleId 匹配浏览器? → Web 深层递归模式
4. 标准枚举
5. 结果为空? → 返回空,Go 层 fallback VLM
```

---

## 验证计划

### 自动化验证

```bash
# 1. Rust 编译
cd rust-core && cargo build --release

# 2. Rust 静态分析
cd rust-core && cargo clippy -- -D warnings

# 3. Go 编译
cd go-sensory && go build ./...

# 4. Go 静态分析
cd go-sensory && go vet ./...

# 5. 现有测试不回归
make test

# 6. .app 打包验证
make app && ls -la build/Argus.app/Contents/

# 7. .pkg 构建验证
make pkg && ls -la build/Argus-Installer.pkg
```

### 人工验证

1. 双击 `Argus.app` 启动，确认权限引导弹出
2. 在 VS Code (Electron) 中测试 AX 元素检测，确认数量增加
3. 在 Chrome 中打开网页，测试 Web 元素深层枚举
4. 安装 .pkg，确认安装流程正常

---

## 风险说明

| 风险 | 缓解措施 |
|------|----------|
| dylib 路径问题 | `install_name_tool` + `@rpath` 修正 |
| 代码签名要求 | `.pkg` 需签名才能在 Gatekeeper 下安装，开发阶段可用 `--no-sign` |
| AXManualAccessibility 副作用 | 仅对 Electron 应用设置，不影响其他应用 |
| Web 深层递归性能 | 增加 MAX_ELEMENTS 上限检查，超限截断 |
