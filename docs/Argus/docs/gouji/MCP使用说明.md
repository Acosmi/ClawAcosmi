# Argus Sensory MCP — 使用说明

## 概述

Argus Sensory MCP Server 实现了 [Model Context Protocol](https://modelcontextprotocol.io) (MCP)，使 AI Agent（如 Claude Desktop、Cursor、VS Code Copilot 等）能够通过标准化协议控制你的 Mac 桌面。

**协议:** JSON-RPC 2.0 over stdio  
**安全策略:** Privacy-first — 所有写操作需人工确认

---

## 启动方式

### 方式一：手动启动

```bash
cd /Users/fushihua/Desktop/Argus-compound/go-sensory
go run ./cmd/server/main.go --mcp
```

> [!IMPORTANT]
> MCP 模式通过 stdin/stdout 通信，不使用网络端口。log 输出到 stderr。
> 通常由 MCP 客户端自动启动，不需要手动运行。

### 方式二：配置 Claude Desktop 自动启动

编辑 Claude Desktop 配置文件：

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "argus-sensory": {
      "command": "go",
      "args": ["run", "./cmd/server/main.go", "--mcp"],
      "cwd": "/Users/fushihua/Desktop/Argus-compound/go-sensory"
    }
  }
}
```

### 方式三：编译后配置

```bash
cd /Users/fushihua/Desktop/Argus-compound/go-sensory
go build -o argus-sensory ./cmd/server/main.go
```

```json
{
  "mcpServers": {
    "argus-sensory": {
      "command": "/Users/fushihua/Desktop/Argus-compound/go-sensory/argus-sensory",
      "args": ["--mcp"]
    }
  }
}
```

### 方式四：.app 包装启动（⭐ 推荐）

> [!TIP]
> 使用 `.app` 包装可以解决 macOS 屏幕录制授权问题 — 裸二进制文件（`node`、`go`）无法在系统隐私设置中被识别和勾选。

**构建 .app:**

```bash
cd /Users/fushihua/Desktop/Argus-compound/go-sensory
bash scripts/build-app.sh
```

**配置 Claude Desktop:**

```json
{
  "mcpServers": {
    "argus-sensory": {
      "command": "/Users/fushihua/Desktop/Argus-compound/go-sensory/Argus Sensory.app/Contents/MacOS/mcp-launcher.sh",
      "args": []
    }
  }
}
```

**授权步骤:**

1. 配置后重启 Claude Desktop
2. 首次调用 `capture_screen` 时会弹出授权框
3. 前往 系统设置 → 隐私与安全性 → 屏幕录制 → 找到 **Argus Sensory** → 勾选启用

---

## 可用工具 (16 个)

### 🔍 感知类 (Perception) — 只读，自动通过

| 工具 | 说明 | 风险 |
|------|------|------|
| `capture_screen` | 截取屏幕画面 | 🟢 Low |
| `describe_scene` | 用 VLM 描述屏幕内容 | 🟢 Low |
| `locate_element` | 定位 UI 元素坐标 | 🟢 Low |
| `read_text` | OCR 读取屏幕文字 | 🟢 Low |
| `detect_dialog` | 检测弹窗/对话框 | 🟢 Low |
| `watch_for_change` | 等待屏幕变化 | 🟢 Low |

### 🖱️ 操作类 (Action) — 需人工确认

| 工具 | 说明 | 风险 |
|------|------|------|
| `click` | 鼠标点击 | 🟡 Medium |
| `double_click` | 鼠标双击 | 🟡 Medium |
| `scroll` | 滚轮滚动 | 🟢 Low |
| `mouse_position` | 获取光标位置 | 🟢 Low |
| `type_text` | 模拟键盘输入 | 🟡 Medium |
| `press_key` | 单键按下 | 🟡 Medium |
| `hotkey` | 组合快捷键 | 🟡~🔴 动态 |

### 🍎 macOS (macOS 快捷键)

| 工具 | 说明 | 风险 |
|------|------|------|
| `macos_shortcut` | 执行常见 macOS 操作 (copy/paste/save/undo/...) | 🟡~🔴 按操作分级 |
| `open_url` | 打开 URL 或文件 | 🟡 Medium |

### 💻 Shell (命令执行)

| 工具 | 说明 | 风险 |
|------|------|------|
| `run_shell` | 执行 shell 命令 (4层安全防护) | 🔴 High |

---

## 安全机制

### 风险分级

| 级别 | 行为 |
|------|------|
| 🟢 **Low** | 自动通过，不弹窗确认 |
| 🟡 **Medium** | 弹窗等待人工确认 |
| 🔴 **High** | 强制人工确认 + 额外审计 |
| ⛔ **Blocked** | 直接拒绝 |

### Shell 命令 4 层防御

1. **正则黑名单** — `rm -rf /`, `mkfs`, `dd`, `shutdown` 等 13 种模式硬阻止
2. **ApprovalGateway** — 始终 High 风险，必须人工确认
3. **执行沙箱** — 30 秒超时，64KB 输出上限，敏感环境变量自动清除
4. **审计日志** — 每条命令自动记录

### 人工可修改参数

审核员在确认弹窗中可以**修改命令内容**后再放行。例如将 `sudo rm -rf /tmp` 改为 `rm -rf /tmp/cache`。

---

## 协议交互示例

### 手动测试

```bash
# 1. 发送 initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}' | go run ./cmd/server/main.go --mcp

# 2. 交互式测试 (多条消息)
go run ./cmd/server/main.go --mcp <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
EOF
```

### 协议消息格式

**请求 (Client → Server):**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "capture_screen",
    "arguments": {"quality": "vlm"}
  }
}
```

**响应 (Server → Client):**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{"type": "text", "text": "截屏数据..."}],
    "isError": false
  }
}
```

---

## macOS 快捷键操作列表

`macos_shortcut` 工具支持的 `action` 值：

| action | 操作 | 快捷键 |
|--------|------|--------|
| `copy` | 复制 | ⌘C |
| `paste` | 粘贴 | ⌘V |
| `cut` | 剪切 | ⌘X |
| `undo` | 撤销 | ⌘Z |
| `redo` | 重做 | ⌘⇧Z |
| `save` | 保存 | ⌘S |
| `select_all` | 全选 | ⌘A |
| `find` | 查找 | ⌘F |
| `new_tab` | 新标签页 | ⌘T |
| `close_tab` | 关闭标签页 | ⌘W |
| `switch_app` | 切换应用 | ⌘Tab |
| `spotlight` | 聚焦搜索 | ⌘Space |
| `screenshot` | 截屏 | ⌘⇧3 |
| `screenshot_area` | 区域截屏 | ⌘⇧4 |
| `quit_app` | 退出应用 | ⌘Q |
| `force_quit` | 强制退出 | ⌥⌘Esc |
| `minimize` | 最小化 | ⌘M |
| `hide` | 隐藏 | ⌘H |

---

## 注意事项

1. **首次使用需授权屏幕录制权限**: 系统设置 → 隐私与安全性 → 屏幕录制。建议使用 `.app` 包装启动（方式四），裸二进制可能无法在授权列表中正确显示。
2. **MCP 与 HTTP 模式互斥**: 同一进程只能选其一，但可以开两个进程分别运行
3. **VLM 可选**: 不配置 VLM 时，`describe_scene`、`locate_element`、`read_text` 等工具将返回错误
4. **默认安全**: MCP 模式下 AutoMode=false，所有非只读操作都需要人工确认
