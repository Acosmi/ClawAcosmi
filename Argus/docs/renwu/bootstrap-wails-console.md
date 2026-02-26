# Bootstrap — Wails macOS App 打包任务

> **生成时间**: 2026-02-17 00:55  
> **任务目标**: 将 web-console (port 3090) 打包为原生 macOS App，使用 Wails v2 框架  
> **前置决策**: 经技术调研确认 Wails v2 为最优方案（vs Tauri/Electron/纯 Swift），详见下方决策记录

---

## 1. 项目概览

Argus-Compound 是一个**视觉理解执行智能体**，面向 macOS 系统。架构为 Go + Rust 混合：

| 组件 | 路径 | 说明 |
|------|------|------|
| **Go 后端** | `go-sensory/` | HTTP API 服务 (port 8090)，Go 1.25.7 |
| **Rust 核心** | `rust-core/` | 高性能计算库 `libargus_core.dylib`，通过 CGO/FFI 被 Go 调用 |
| **Web 前端** | `web-console/` | Next.js 14.2 管理界面 (port 3090) |
| **打包脚本** | `scripts/package/` | Info.plist、签名、.pkg 构建 |
| **构建产物** | `build/Argus.app` | 现有 .app（仅后台服务，无 GUI 窗口） |

## 2. 现有前端详情

### web-console/package.json

```json
{
  "name": "argus-console",
  "version": "0.1.0",
  "scripts": {
    "dev": "next dev -p 3090",
    "build": "next build",
    "start": "next start -p 3090"
  },
  "dependencies": {
    "next": "^14.2.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}
```

### Next.js 代理配置 (next.config.js)

```js
// 所有 /api/sensory/*, /v1/*, /api/config/* → localhost:8090
async rewrites() {
    return [
        { source: '/api/sensory/:path*', destination: 'http://localhost:8090/api/:path*' },
        { source: '/v1/:path*', destination: 'http://localhost:8090/v1/:path*' },
        { source: '/api/config/:path*', destination: 'http://localhost:8090/api/config/:path*' },
    ];
}
```

### 前端组件列表

```
web-console/src/
├── app/
│   ├── globals.css          — 全局样式
│   ├── layout.tsx           — 根布局 (title: "Argus — 24小时之眼")
│   └── page.tsx             — 主页面 (15KB)
├── components/
│   ├── AnomalyPage.tsx      — 异常检测页
│   ├── LangSwitch.tsx       — 语言切换
│   ├── SettingsPage.tsx     — 设置页 (15KB)
│   ├── TasksPage.tsx        — 任务页
│   ├── TimelinePage.tsx     — 时间线页
│   └── WindowManager.tsx    — 窗口管理器
├── hooks/                   — 自定义 hooks
├── i18n/                    — 国际化 (中/英)
├── styles/                  — 样式文件 (7个)
└── types/                   — TypeScript 类型
```

## 3. 现有 Go 后端详情

### 入口: `go-sensory/cmd/server/main.go` (475 行)

- `parseFlags()` — 解析 CLI 参数 (fps, port, backend, shm, openBrowser, vlmConfigPath, mcpMode)
- `initCapture()` — 屏幕抓取初始化
- `initVLM()` — VLM 路由初始化
- `initPipeline()` — 帧处理管道
- `main()` — 启动 HTTP server on port 8090
- `runMCPServer()` — MCP stdio 模式

### 依赖

```
module Argus-compound/go-sensory
go 1.25.7
require (
    golang.org/x/image v0.36.0
    golang.org/x/net v0.49.0
)
```

## 4. 现有 .app 打包结构

### Makefile `app` target

```makefile
app: build
    mkdir -p build/Argus.app/Contents/{MacOS,Frameworks,Resources}
    cp scripts/package/Info.plist build/Argus.app/Contents/
    go build -o build/Argus.app/Contents/MacOS/argus-sensory ./cmd/server
    cp rust-core/target/release/libargus_core.dylib build/Argus.app/Contents/Frameworks/
    install_name_tool -change ... @executable_path/../Frameworks/libargus_core.dylib ...
    codesign --force --deep -s "Argus Dev" build/Argus.app
```

### Info.plist 关键配置

- `CFBundleIdentifier`: `com.argus.compound`
- `CFBundleExecutable`: `argus-sensory`
- `LSUIElement`: `true` (后台 App，无 Dock 图标)
- `LSMinimumSystemVersion`: `12.3`
- 权限: Screen Capture + Accessibility

### go-sensory/Argus Sensory.app (独立版)

- 可执行文件: `sensory-server`
- 启动脚本: `launch.sh` → 运行 `sensory-server --fps 2 --open-browser=true`
- 也是 `LSUIElement: true` (后台运行)

## 5. 技术决策记录

### 为什么选 Wails v2？

| 因素 | 分析 |
|------|------|
| **Go 原生** | 项目后端就是 Go，零语言切换成本 |
| **前端零改动** | 可以直接 WebView 加载 localhost:3090 |
| **轻量** | 使用 macOS 原生 WebKit，打包 ~10MB |
| **不影响 Rust 性能** | Wails 只做窗口管理壳，不参与 FFI 调用链 |
| **内置功能** | 系统托盘、原生菜单栏、通知对话框开箱即用 |
| **成熟度** | Wails v2 稳定版，v3 正在 alpha |

### 被否的方案

- **Tauri v2**: 需要将 Next.js 改为静态导出，改动较大
- **Electron**: 打包 150MB+，内存 300MB+，过于臃肿
- **纯 Swift WKWebView**: 需新增 Swift 语言栈，维护成本高

## 6. 目标架构

```
Argus Console.app (Wails v2)
├── Contents/
│   ├── MacOS/
│   │   └── argus-console        ← Wails Go 二进制 (含 WebView 窗口管理)
│   ├── Frameworks/
│   │   └── libargus_core.dylib  ← Rust 核心库
│   ├── Resources/
│   │   ├── web/                 ← Next.js 构建产物 (next build 输出)
│   │   ├── vlm-config.json
│   │   └── AppIcon.icns
│   └── Info.plist
└── 运行逻辑:
    1. 启动 argus-console
    2. 内部启动 sensory-server (port 8090)
    3. 内部启动 Next.js 前端 (port 3090) 或由 Go serve 静态文件
    4. WebView 窗口加载 http://localhost:3090
    5. 浏览器仍可直接访问 http://localhost:3090 (备选)
    6. 系统托盘图标提供快捷操作
```

## 7. 实施任务清单

### 阶段 A — 环境与脚手架

- [x] 安装 Wails v2 CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- [x] 确认依赖: Go 1.21+, Node 15+, Xcode CLI tools
- [x] 在项目根目录创建 `wails-console/` 目录作为 Wails 子项目
- [x] 初始化 Wails 项目 (使用现有 Next.js 前端)

### 阶段 B — 核心集成

- [x] 编写 Go 后端进程管理器 (启动/停止 sensory-server)
- [x] 编写 Next.js 进程管理器 (启动/停止 `next start`)
- [x] 配置 WebView 加载 `http://localhost:3090`
- [x] 实现启动等待逻辑 (等 3090 端口 ready 后再显示窗口)
- [x] 实现优雅退出 (关窗口 → 停止所有子进程)

### 阶段 C — 原生增强

- [ ] 添加系统托盘图标和菜单
- [ ] 添加 Dock 图标 (去掉 LSUIElement: true)
- [ ] 添加应用图标 (AppIcon.icns)
- [ ] 添加原生菜单栏 (文件/编辑/窗口/帮助)

### 阶段 D — 打包与集成

- [x] 更新 Makefile 添加 `console` target
- [ ] 将 Rust dylib 集成到 Wails App bundle
- [ ] 用 install_name_tool 修正 dylib 路径
- [ ] 代码签名
- [ ] 测试最终 .app 的启动和运行

### 阶段 E — 验证

- [ ] 验证: 双击 .app 能正常打开管理界面
- [ ] 验证: 浏览器直接访问 localhost:3090 仍然正常
- [ ] 验证: 系统托盘图标和菜单功能正常
- [ ] 验证: 关闭 App 时所有子进程正确退出
- [ ] 验证: Rust FFI 调用正常工作

## 8. 注意事项

1. **Go 版本**: 项目使用 Go 1.25.7，确认 Wails v2 兼容性
2. **端口冲突**: sensory-server 用 8090，Next.js 用 3090，Wails 不需要额外端口
3. **权限**: 现有 App 需要 Screen Capture 和 Accessibility 权限，新 App 也需要
4. **dylib 路径**: 必须用 `install_name_tool` 修正 Rust 库的加载路径
5. **保留浏览器访问**: 这是硬性需求，Next.js 端口必须对浏览器开放
