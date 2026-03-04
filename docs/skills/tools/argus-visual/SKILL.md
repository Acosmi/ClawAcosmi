---
name: argus-visual
description: "Argus visual sub-agent: screen perception + UI automation via argus_* tools"
tools: spawn_argus_agent
---

## When to Use Argus (vs browser)

| 场景 | 工具 | 原因 |
|------|------|------|
| 桌面应用（Finder、Terminal、Xcode） | spawn_argus_agent | 原生 UI 无 CSS 选择器 |
| 网页自动化 | browser | CSS 选择器比坐标更精准 |
| OCR / 屏幕文字识别 | argus_read_text | 仅 Argus 有 OCR |
| 快速截图验证 | argus_capture_screen（直接） | 无需启动子智能体 |
| 复杂多步桌面工作流 | spawn_argus_agent | 独立会话 + 视觉感知 |

**规则**: 有 URL 的用 browser，原生桌面窗口用 Argus。

---

# Argus Visual Sub-Agent

Argus is an integrated **visual perception and UI automation** sub-agent.
When connected, you have access to `argus_*` prefixed tools that let you
**see the screen, find UI elements, click, type, scroll, and open URLs**.

Argus uses macOS Accessibility API (CGEvent) for input simulation and
Core Graphics for screen capture. It requires **Screen Recording** and
**Accessibility** permissions in System Settings > Privacy & Security.

> Reference: [Apple Accessibility API](https://developer.apple.com/documentation/accessibility/accessibility-api),
> [CGEvent](https://developer.apple.com/documentation/coregraphics/cgevent)

## Available Tools

### Perception (read-only, safe)

| Tool | Description |
|------|-------------|
| `argus_capture_screen` | Take a screenshot (returns base64 image) |
| `argus_describe_scene` | AI-powered description of what's on screen |
| `argus_locate_element` | Find a UI element by natural language description |
| `argus_read_text` | OCR: extract text from screen region |
| `argus_detect_dialog` | Detect dialog/modal boxes on screen |
| `argus_watch_for_change` | Monitor screen for changes |

### Actions (interactive)

| Tool | Input | Description |
|------|-------|-------------|
| `argus_click` | `{"x": int, "y": int, "button": 0}` | Click at coordinates (0=left, 1=right, 2=middle) |
| `argus_double_click` | `{"x": int, "y": int}` | Double-click |
| `argus_type_text` | `{"text": "string"}` | Type text into the focused field |
| `argus_press_key` | `{"key": "string"}` | Press a keyboard key (Enter, Tab, Escape, etc.) |
| `argus_hotkey` | `{"keys": ["cmd", "c"]}` | Trigger arbitrary keyboard shortcuts |
| `argus_scroll` | `{"direction": "up/down", "amount": int}` | Scroll viewport |
| `argus_mouse_position` | `{}` | Get current mouse cursor position |

### macOS Specific

| Tool | Input | Description |
|------|-------|-------------|
| `argus_open_url` | `{"target": "https://..."}` | Open URL via Spotlight (⌘Space → type → Enter) |
| `argus_macos_shortcut` | `{"action": "name"}` | Execute a named macOS keyboard shortcut |

### Shell

| Tool | Description |
|------|-------------|
| `argus_run_shell` | Execute a shell command (high risk) |

## macOS Keyboard Shortcuts Reference

> Based on [Apple Mac keyboard shortcuts](https://support.apple.com/en-us/102650)

Use `argus_macos_shortcut` with the `action` parameter.

### Clipboard & Edit

| Action | Shortcut | Description |
|--------|----------|-------------|
| `copy` | ⌘C | Copy the selected item to the Clipboard |
| `paste` | ⌘V | Paste the contents of the Clipboard |
| `cut` | ⌘X | Cut the selected item |
| `select_all` | ⌘A | Select All items |
| `undo` | ⌘Z | Undo the previous command |
| `redo` | ⇧⌘Z | Reverse an undo action |
| `save` | ⌘S | Save the current document |
| `find` | ⌘F | Find items in a document or open a Find window |

### Window Management

| Action | Shortcut | Description |
|--------|----------|-------------|
| `close_window` | ⌘W | Close the front window |
| `minimize` | ⌘M | Minimize the front window to the Dock |
| `hide` | ⌘H | Hide the windows of the front app |
| `fullscreen` | ⌃⌘F | Toggle full screen mode |

### App & System

| Action | Shortcut | Description |
|--------|----------|-------------|
| `switch_app` | ⌘Tab | Switch to the next most recently used app |
| `spotlight` | ⌘Space | Show or hide the Spotlight search field |

### Screenshot

| Action | Shortcut | Description |
|--------|----------|-------------|
| `screenshot` | ⇧⌘3 | Capture entire screen to file |
| `screenshot_region` | ⇧⌘4 | Capture selected region to file |

### Tab & Navigation

| Action | Shortcut | Description |
|--------|----------|-------------|
| `new_tab` | ⌘T | Open a new tab |
| `close_tab` | ⌘W | Close current tab |
| `back` | ⌘← | Go to the previous page |
| `forward` | ⌘→ | Go to the next page |

### Additional macOS Shortcuts (via `argus_hotkey`)

For shortcuts not in `argus_macos_shortcut`, use `argus_hotkey` directly:

| Keys | Shortcut | Description |
|------|----------|-------------|
| `["cmd", "q"]` | ⌘Q | Quit the current app |
| `["cmd", "o"]` | ⌘O | Open file dialog |
| `["cmd", "p"]` | ⌘P | Print |
| `["cmd", ","]` | ⌘, | Open app preferences/settings |
| `["option", "cmd", "escape"]` | ⌥⌘Esc | Force quit dialog |
| `["cmd", "n"]` | ⌘N | New window/document |
| `["shift", "cmd", "n"]` | ⇧⌘N | New folder (Finder) |
| `["cmd", "delete"]` | ⌘⌫ | Move to Trash (Finder) |
| `["ctrl", "up"]` | ⌃↑ | Mission Control |
| `["ctrl", "down"]` | ⌃↓ | App Expose (show app windows) |
| `["cmd", "b"]` | ⌘B | Bold text |
| `["cmd", "i"]` | ⌘I | Italic text |
| `["cmd", "u"]` | ⌘U | Underline text |

## Workflow Pattern

Always follow this **perceive → understand → act → verify** loop:

```
1. argus_capture_screen  →  See what's on screen
2. argus_describe_scene  →  Understand the current state
3. argus_locate_element  →  Find the target element
4. argus_click / argus_type_text / argus_press_key  →  Take action
5. argus_capture_screen  →  Verify the result
```

### Example: Open a website

```
Step 1: argus_open_url {"target": "https://example.com"}
Step 2: argus_capture_screen {}                     → wait for page to load
Step 3: argus_describe_scene {}                     → verify page loaded correctly
```

### Example: Click a button

```
Step 1: argus_capture_screen {}                     → see current screen
Step 2: argus_locate_element {"description": "Submit button"}  → get coordinates
Step 3: argus_click {"x": <x>, "y": <y>}           → click it
Step 4: argus_capture_screen {}                     → verify result
```

### Example: Fill a form field

```
Step 1: argus_locate_element {"description": "Email input field"}
Step 2: argus_click {"x": <x>, "y": <y>}           → focus the field
Step 3: argus_type_text {"text": "user@example.com"}
Step 4: argus_press_key {"key": "Tab"}              → move to next field
```

### Example: Copy text from screen

```
Step 1: argus_capture_screen {}                     → see what's on screen
Step 2: argus_locate_element {"description": "paragraph with the address"}
Step 3: argus_click {"x": <x>, "y": <y>}           → click to position cursor
Step 4: argus_macos_shortcut {"action": "select_all"}
Step 5: argus_macos_shortcut {"action": "copy"}     → text now in clipboard
```

### Example: Switch between apps

```
Step 1: argus_macos_shortcut {"action": "switch_app"}  → ⌘Tab
Step 2: argus_capture_screen {}                        → verify switched to target app
```

### Example: Search with Spotlight

```
Step 1: argus_macos_shortcut {"action": "spotlight"}   → ⌘Space opens Spotlight
Step 2: argus_type_text {"text": "Terminal"}           → type app name
Step 3: argus_press_key {"key": "Return"}              → open the app
Step 4: argus_capture_screen {}                        → verify app opened
```

## macOS TCC Permission Requirements

Argus requires the following macOS permissions (System Settings > Privacy & Security):

| Permission | Purpose | Required For |
|------------|---------|--------------|
| **Screen Recording** | Screen capture via CGWindowListCreateImage | `capture_screen`, `describe_scene`, `read_text` |
| **Accessibility** | Input simulation via CGEvent API | `click`, `type_text`, `press_key`, `hotkey`, all shortcuts |

If permissions are not granted, Argus tools will return errors.
The Argus binary must be **code-signed** for TCC authorization to persist across rebuilds.

## Important Notes

- Always **capture screen first** before taking actions
- After every action, **capture screen again** to verify the result
- Use `argus_locate_element` with natural language descriptions to find UI elements
- Tool names must include the `argus_` prefix (e.g., `argus_click`, not `click`)
- If an action fails, capture screen to understand what went wrong
- Argus tools operate on the **physical screen**, not a virtual browser
- Actions are visible to the user — Argus moves the real mouse and types real keys
- Use `argus_macos_shortcut` for common shortcuts; use `argus_hotkey` for anything else
