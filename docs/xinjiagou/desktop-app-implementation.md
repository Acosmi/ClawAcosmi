# 创宇太虚 桌面 App 跨平台落地方案（联网验证版）

> 文档创建：2026-03-04 | 基于 Wails v3 官方文档 + GitHub Issues 实测验证

---

## 一、技术选型：Wails v3

| 维度 | 详情 | 验证来源 |
|------|------|---------|
| 框架 | Wails v3（Go + 原生 WebView） | wails.io 官方 |
| 当前状态 | Alpha，API 基本稳定，已有生产应用 | GitHub releases |
| Go 要求 | Go 1.24+（Win 推荐 1.25+） | ✅ 项目 Go 1.25.7 |
| CLI 安装 | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` | wails.io/docs |
| 环境诊断 | `wails3 doctor` 检查所有依赖 | wails.io/docs |
| 构建系统 | Taskfile.yml（非 Makefile），`wails3 build` / `wails3 package` | wails.io/docs |
| 产物大小 | ~15-25MB（原生 WebView，非 Chromium 内嵌） | 社区实测 |

### 各平台 WebView 引擎

| 平台 | 引擎 | 系统依赖 |
|------|------|---------|
| **macOS** | WebKit (WKWebView) | Xcode CLT（`xcode-select --install`） |
| **Windows** | WebView2 (Chromium) | Win10/11 自带；旧版用 Evergreen Bootstrapper |
| **Linux** | WebKitGTK | `libgtk-3-dev` + `libwebkit2gtk-4.1-dev` |

---

## 二、关键技术发现与验证

### ⚠️ WebSocket 限制（已验证，关键）

**问题**：Wails v3 生产构建的内嵌服务器**不支持 WebSocket upgrade**。我们的 Gateway 使用 `gorilla/websocket` 进行全部 RPC 通信，直接嵌入会导致 WebSocket 连接失败。

**官方确认来源**：GitHub Issues + wails.io 文档

**解决方案**：**WebView 加载 `localhost` URL，而非嵌入资源**

```
┌─────────────────────────────────────────┐
│ Wails 壳（窗口管理 + 系统托盘）          │
│                                         │
│  WebView 加载 → http://localhost:19001  │
│         ↕ WebSocket                     │
│  Gateway HTTP Server (同进程, :19001)   │
│         ↕ gorilla/websocket             │
│  前端静态文件由 Gateway 托管 (/ui/)     │
└─────────────────────────────────────────┘
```

**关键点**：

1. Gateway 以 goroutine 在同进程内启动，监听 `:19001`
2. 前端静态文件用 `go:embed` 打包，由 Gateway 的 `/ui/` 路由托管
3. WebView 的 URL 设为 `http://localhost:19001/ui/`
4. WebSocket 连接 `ws://localhost:19001/ws` 正常工作
5. `websocket.Upgrader.CheckOrigin` 需允许 `wails://` 来源

**这与当前架构完美契合**——Gateway 的 `server_http.go` 已有 `ControlUIDir` 静态文件托管逻辑（L78-81），只需改为用 `embed.FS` 替代磁盘目录。

### ✅ 前端嵌入（已验证）

```go
//go:embed all:frontend/dist
var assets embed.FS
```

- Wails v3 通过 `application.AssetOptions{Handler: application.AssetFileServerFS(assets)}` 提供嵌入资源
- 但因 WebSocket 限制，我们**不用 Wails 的资产服务**，改由 Gateway 自身托管
- `go:embed` 仍用于将前端打进二进制，只是由 Gateway 的 HTTP handler 读取

### ✅ 系统托盘（已验证）

```go
tray := app.NewSystemTray()
tray.SetIcon(iconBytes)
menu := app.NewMenu()
menu.Add("显示主界面").OnClick(showWindow)
tray.SetMenu(menu)
```

- macOS / Windows / Linux 三平台统一 API
- 支持自定义图标、菜单项、点击事件

### ✅ 多窗口支持（已验证）

- Wails v3 原生支持多窗口（v2 仅单窗口）
- 每个窗口独立配置大小、位置、URL
- 可通过 `window.SetURL()` 加载外部 `localhost` URL

### ✅ 生命周期回调（已验证）

- `OnShutdown` 回调在应用退出前执行（前端销毁后）
- 可在此优雅关闭 Gateway、清理 WebSocket 连接
- Window 事件：`EventWindowClose` 可拦截关窗口改为最小化到托盘

### ✅ 事件系统（已验证）

- `app.Event.Emit()` — Go → 前端
- 统一 pub/sub 机制，Go ↔ JS 双向通信
- 支持 TypedEvent（类型安全，自动补全）

---

## 三、整体架构

```
┌──────────────────────────────────────────────────────┐
│               创宇太虚.app / .exe                     │
│                                                      │
│  ┌────────────┐    ┌──────────────────────────────┐  │
│  │ Wails v3   │    │ Gateway Core                 │  │
│  │ 桌面壳     │    │ (现有 RunGatewayBlocking)    │  │
│  │            │    │                              │  │
│  │ - 窗口管理 │    │ HTTP :19001                  │  │
│  │ - 系统托盘 │    │  ├─ /ui/  静态文件(embed.FS) │  │
│  │ - 原生菜单 │    │  ├─ /ws   WebSocket RPC      │  │
│  │ - 生命周期 │    │  ├─ /v1/  OpenAI API         │  │
│  │            │    │  └─ /hooks/ Webhook          │  │
│  │ WebView ──────→ │ http://localhost:19001/ui/   │  │
│  └────────────┘    └──────────────────────────────┘  │
│                                                      │
│  ┌────────────┐    ┌──────────────────────────────┐  │
│  │ go:embed   │    │ oa-sandbox (Rust FFI)        │  │
│  │ 前端资源   │    │ 沙箱引擎                     │  │
│  └────────────┘    └──────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
        │ MCP stdio                    │ HTTP
        ↓                              ↓
  Argus.app (外部子进程)         Rust CLI (openacosmi)
```

### 核心设计决策

| # | 决策 | 理由 |
|---|------|------|
| 1 | Gateway 以 goroutine 在 Wails 进程内启动 | 单进程，关窗口即关服务 |
| 2 | WebView 加载 `localhost:19001/ui/` | 解决 WebSocket 兼容问题 |
| 3 | 前端用 `go:embed` 打包，Gateway HTTP 托管 | 单文件分发，无外部依赖 |
| 4 | Argus 仍为外部子进程 | 独立签名 + TCC 授权管理 |
| 5 | 关窗口 = 最小化到托盘，右键退出 = 优雅关闭 | 用户体验 |

---

## 四、项目目录结构

```
OpenAcosmi-rust+go/
├── desktop/                        # [新增] 桌面 App 主包
│   ├── main.go                     # Wails 入口 + Gateway 启动
│   ├── app.go                      # 生命周期 + 首次检测
│   ├── tray.go                     # 系统托盘
│   ├── menu.go                     # 原生菜单
│   ├── embed.go                    # go:embed 前端资源
│   ├── Taskfile.yml                # Wails 构建主文件
│   ├── build/
│   │   ├── appicon.png             # 应用图标 1024x1024
│   │   ├── darwin/
│   │   │   ├── Taskfile.yml        # macOS 构建任务
│   │   │   ├── Info.plist          # CFBundleIdentifier
│   │   │   └── entitlements.plist  # 权限声明
│   │   ├── windows/
│   │   │   ├── Taskfile.yml        # Windows 构建任务
│   │   │   ├── icon.ico
│   │   │   ├── info.json           # 版本信息
│   │   │   └── wails.exe.manifest  # DPI/UAC
│   │   └── linux/
│   │       ├── Taskfile.yml        # Linux 构建任务
│   │       └── icon.png
│   └── frontend/                   # 构建时从 ui/dist/control-ui 复制
├── backend/                        # 现有 Gateway（不改动核心）
├── ui/                             # 现有前端
└── ...
```

> **注意**：Wails v3 使用 **Taskfile.yml**（非 Makefile）作为构建系统。`wails3 build` 和 `wails3 package` 实际执行 Taskfile 中定义的任务。

---

## 五、核心代码框架（已验证 API）

### 5.1 embed.go — 嵌入前端

```go
package main

import "embed"

// 前端构建产物（npm run build → dist/control-ui/）
//go:embed all:frontend/dist
var frontendAssets embed.FS
```

### 5.2 main.go — 入口

```go
package main

import (
    "fmt"
    "net"
    "os"
    "path/filepath"

    "github.com/wailsapp/wails/v3/pkg/application"
    "github.com/Acosmi/ClawAcosmi/internal/config"
    "github.com/Acosmi/ClawAcosmi/internal/gateway"
)

var version = "dev"

func main() {
    // 1. 找到空闲端口（或使用默认 19001）
    port := resolvePort()

    // 2. 后台启动 Gateway（非阻塞）
    go func() {
        opts := gateway.GatewayServerOptions{
            EmbeddedAssets: frontendAssets, // 传入嵌入资源
        }
        if err := gateway.RunGatewayBlocking(port, opts); err != nil {
            fmt.Fprintf(os.Stderr, "Gateway 启动失败: %v\n", err)
            os.Exit(1)
        }
    }()

    // 3. 等待 Gateway 就绪
    waitForReady(port)

    // 4. 创建 Wails App
    app := application.New(application.Options{
        Name: "创宇太虚",
    })

    // 5. 构建 URL（首次启动带向导参数）
    url := fmt.Sprintf("http://localhost:%d/ui/", port)
    if needsOnboarding() {
        url += "?onboarding=true"
    }

    // 6. 创建主窗口 — 加载 Gateway 托管的 UI
    app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
        Title:  "创宇太虚 — Claw Acosmi",
        Width:  1280,
        Height: 800,
        URL:    url,
    })

    // 7. 设置系统托盘
    setupTray(app)

    // 8. 运行（阻塞直到退出）
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "应用运行失败: %v\n", err)
        os.Exit(1)
    }
}
```

### 5.3 app.go — 生命周期

```go
package main

import (
    "fmt"
    "net"
    "net/http"
    "os"
    "path/filepath"
    "time"
)

func needsOnboarding() bool {
    home, _ := os.UserHomeDir()
    configPath := filepath.Join(home, ".openacosmi", "config.json")
    _, err := os.Stat(configPath)
    return os.IsNotExist(err)
}

func resolvePort() int {
    // 尝试默认端口
    ln, err := net.Listen("tcp", ":19001")
    if err == nil {
        ln.Close()
        return 19001
    }
    // 端口被占用，找空闲端口
    ln, _ = net.Listen("tcp", ":0")
    port := ln.Addr().(*net.TCPAddr).Port
    ln.Close()
    return port
}

func waitForReady(port int) {
    url := fmt.Sprintf("http://localhost:%d/health", port)
    for i := 0; i < 30; i++ {
        resp, err := http.Get(url)
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            return
        }
        time.Sleep(200 * time.Millisecond)
    }
}
```

### 5.4 tray.go — 系统托盘

```go
package main

import (
    _ "embed"
    "github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/appicon.png
var trayIcon []byte

func setupTray(app *application.App) {
    tray := app.NewSystemTray()
    tray.SetIcon(trayIcon)

    menu := app.NewMenu()
    menu.Add("显示主界面").OnClick(func(_ *application.Context) {
        // 显示主窗口
    })
    menu.Add("重新配置向导").OnClick(func(_ *application.Context) {
        // 打开向导
    })
    menu.AddSeparator()
    menu.Add("退出创宇太虚").OnClick(func(_ *application.Context) {
        app.Quit()
    })

    tray.SetMenu(menu)
}

---

## 六、跨平台打包（已验证命令）

### 6.1 构建与打包命令

```bash
# 安装 Wails v3 CLI
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

# 诊断环境
wails3 doctor

# 开发模式（热重载前端）
wails3 dev

# 生产构建（输出到 bin/ 目录）
wails3 build

# 打包分发格式
wails3 package
```

### 6.2 各平台打包产物

| 平台 | 命令 | 产物 | 签名工具 |
|------|------|------|---------|
| **macOS** | `wails3 package` | `.app` bundle → `.dmg` | `codesign` + `notarytool` |
| **Windows** | `wails3 package` | NSIS `.exe` 安装包 | EV 代码签名证书 |
| **Linux** | `wails3 package` | `.AppImage` + `.deb` + `.rpm` | GPG 签名 |

### 6.3 macOS 签名与公证（已验证流程）

1. **证书**：需要 Apple Developer ID Application 证书（$99/年）
2. **签名**：`codesign --force --deep --sign "Developer ID" --entitlements entitlements.plist --timestamp 创宇太虚.app`
3. **公证**：使用 `notarytool`（`altool` 已弃用）
4. **entitlements.plist 关键项**：
   - `com.apple.security.network.client` — 网络访问
   - `com.apple.security.files.user-selected.read-write` — 文件访问
   - 发布构建**必须移除** `com.apple.security.get-task-allow`（否则公证失败）
5. **Info.plist**：放在 `build/darwin/Info.plist`，定义 `CFBundleIdentifier`

### 6.4 Windows NSIS 安装包（已验证）

- NSIS 配置位于 `build/windows/nsis/`
- 可自定义 `project.nsi` 脚本
- 自动打包 `MicrosoftEdgeWebview2Setup.exe`（旧系统需要）
- `wails.exe.manifest` 声明 DPI 感知和 UAC 级别

### 6.5 Linux AppImage（已验证）

- 配置位于 `build/linux/appimage/`
- 图标取自 `build/appicon.png`
- 也可用 `wails3 task linux:create:appimage` 单独创建
- 同时输出 `.deb` 和 `.rpm` 格式

---

## 七、CI 构建流水线

**跨编译限制**（已验证）：Wails/CGO 不支持从 Linux 交叉编译 macOS 目标。需要各平台原生 CI runner。

### GitHub Actions 矩阵

```yaml
name: Desktop Release
on:
  push:
    tags: ['v*']
jobs:
  build:
    strategy:
      matrix:
        include:
          - os: macos-14
            artifact: 创宇太虚-macOS-ARM64.dmg
          - os: macos-13
            artifact: 创宇太虚-macOS-Intel.dmg
          - os: windows-latest
            artifact: 创宇太虚-Windows-Setup.exe
          - os: ubuntu-latest
            artifact: 创宇太虚-Linux.AppImage
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - uses: actions/setup-node@v4
        with: { node-version: '22' }
      - name: Install Wails CLI
        run: go install github.com/wailsapp/wails/v3/cmd/wails3@latest
      - name: Linux deps
        if: runner.os == 'Linux'
        run: sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
      - name: Build frontend
        run: cd ui && npm ci && npm run build
      - name: Copy frontend
        run: cp -r dist/control-ui desktop/frontend/dist
      - name: Package
        run: cd desktop && wails3 package
      - uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.artifact }}
          path: desktop/bin/*
```

---

## 八、对现有代码的影响

### 不改动

- `backend/` 全部业务逻辑 — Gateway 核心完全复用
- `ui/` 全部组件/样式 — 前端代码零改动
- Wizard V2 后端 RPC — `wizard.v2.apply` 等不变

### 小幅改动（~20 行）

- `backend/internal/gateway/server.go` — `GatewayServerOptions` 增加 `EmbeddedAssets embed.FS`
- `backend/internal/gateway/server_http.go` — 增加 embed.FS 回退分支
- `ui/vite.config.ts` — 已是 `base: "./"` ✅ 无需改动

### 新增文件（~400 行 Go + 资源文件）

| 文件 | 行数 |
|------|------|
| `desktop/main.go` | ~70 |
| `desktop/app.go` | ~50 |
| `desktop/tray.go` | ~40 |
| `desktop/menu.go` | ~40 |
| `desktop/embed.go` | ~8 |
| `desktop/Taskfile.yml` | ~60 |
| `desktop/build/` 资源 | ~15 文件 |
| `.github/workflows/desktop-release.yml` | ~80 |

---

## 九、风险与注意事项

| 风险 | 缓解措施 |
|------|---------|
| Wails v3 Alpha API 变动 | 锁定版本号，关注 changelog |
| macOS 签名费用 $99/年 | 个人/组织申请 Apple Developer |
| Windows EV 证书 ~$200-400/年 | 初期用普通签名（SmartScreen 仅首次警告） |
| Linux WebKitGTK 版本差异 | AppImage 内嵌依赖 |
| entitlements 配置错误导致白屏 | 开发/生产用不同 plist |
| WebSocket CheckOrigin 跨域 | 允许 `localhost` + `wails://` 来源 |

---

## 十、实施路线

| 阶段 | 内容 | 预计工作量 |
|------|------|-----------|
| Phase 1 | 核心壳 + Gateway 集成 + 向导检测 | 2-3 天 |
| Phase 2 | 系统托盘 + 原生菜单 + 最小化到托盘 | 1 天 |
| Phase 3 | CI 构建 + 三平台打包 + 签名 | 2-3 天 |
| Phase 4 | 自动更新 | 1-2 天 |

**总计**：~6-9 天，新增 ~400 行 Go 代码，现有代码改动 ~20 行。

---

## 附录：签名证书详细分析（不上架场景）

> 以下分析针对**不上架 App Store / Microsoft Store**、仅通过官网或 GitHub Releases 分发的场景。

### macOS 签名方案对比

| 方案 | 年费 | 用户体验 | 适用阶段 |
|------|------|---------|---------|
| **不签名** | 免费 | Gatekeeper 拦截，用户需右键→打开→确认 | 内测/开发者群体 |
| **自签名（ad-hoc）** | 免费 | 同上，Gatekeeper 仍拦截 | 同上 |
| **Developer ID + 公证** | $99/年 | 双击直接打开，无任何警告 | 正式发布 |

**注意**：macOS 10.15+ 起所有分发软件必须经过 Notarization（公证），否则 Gatekeeper 默认拦截。用户可手动绕过：

```bash
# 方法 1：右键 → 打开 → 确认
# 方法 2：终端移除隔离标记
xattr -d com.apple.quarantine /Applications/创宇太虚.app
```

**结论**：初期面向开发者可以不买证书；面向普通用户发布时需要 $99/年。

### Windows 签名方案对比

| 方案 | 年费 | 用户体验 | 适用阶段 |
|------|------|---------|---------|
| **不签名** | 免费 | SmartScreen 蓝色警告"Windows 已保护你的电脑"，点"仍要运行" | 内测 |
| **普通代码签名** | $70-200/年 | SmartScreen 首次可能提示，积累安装量后消失 | 小规模发布 |
| **EV 代码签名** | $200-400/年 | 即时信任，SmartScreen 不拦截 | 正式大规模发布 |

**结论**：不签名也能安装运行，只是多一步确认。初期完全可接受。

### Linux — 无需购买

GPG 签名免费。Linux 用户不依赖系统级签名验证。

### 推荐策略

| 项目阶段 | macOS | Windows | 年费总计 |
|---------|-------|---------|---------|
| **内测期** | 不签名 | 不签名 | $0 |
| **早期发布** | Developer ID | 普通签名 | ~$170-300/年 |
| **成熟期** | Developer ID | EV 签名 | ~$300-500/年 |
