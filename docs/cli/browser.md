---
summary: "浏览器控制（profile、标签页、截图、快照、操作、状态管理、Chrome 扩展中继）"
read_when:
  - You want to manage browser profiles and tabs
  - You need Chrome extension relay or remote browser control
  - You're acting through a browser on a node host
  - You want to take snapshots, screenshots, or automate browser actions
title: "browser"
status: active
arch: rust-cli
source: cli-rust/crates/oa-cmd-browser/src/lib.rs
---

> [!NOTE]
> **架构状态：Rust CLI** — 对应 `cli-rust/crates/oa-cmd-browser/src/lib.rs`。
> 所有命令通过 `browser.request` Gateway RPC 路由到本地浏览器控制服务或远程 Node 代理。

# `openacosmi browser`

Manage OpenAcosmi's browser control server and run browser actions (tabs, snapshots, screenshots, navigation, clicks, typing, state management).

Related:

- Browser tool + API: [Browser tool](/tools/browser)
- Chrome extension relay: [Chrome extension](/tools/chrome-extension)

## Common flags

- `--browser-profile <name>`: choose a browser profile (default from config).
- `--json`: machine-readable output (where supported).
- `--target-id <id>`: target a specific tab (where supported).

## Quick start (local)

```bash
openacosmi browser --browser-profile chrome tabs
openacosmi browser --browser-profile openacosmi start
openacosmi browser --browser-profile openacosmi open https://example.com
openacosmi browser --browser-profile openacosmi snapshot
```

## Profiles

Profiles are named browser routing configs. In practice:

- `openacosmi`: launches/attaches to a dedicated OpenAcosmi-managed Chrome instance (isolated user data dir).
- `chrome`: controls your existing Chrome tab(s) via the Chrome extension relay.

```bash
openacosmi browser profiles
openacosmi browser create-profile work --color "#FF5A36"
openacosmi browser delete-profile --name work
openacosmi browser reset-profile --browser-profile openacosmi
```

## Tabs

```bash
openacosmi browser tabs
openacosmi browser open https://docs.openacosmi.ai
openacosmi browser focus <targetId>
openacosmi browser close <targetId>
```

## Snapshot (accessibility tree)

```bash
openacosmi browser snapshot
openacosmi browser snapshot --format aria --limit 200
openacosmi browser snapshot --interactive --compact --depth 6
openacosmi browser snapshot --efficient
openacosmi browser snapshot --labels
openacosmi browser snapshot --selector "#main" --interactive
openacosmi browser snapshot --frame "iframe#main" --interactive
```

## Screenshot

```bash
openacosmi browser screenshot
openacosmi browser screenshot --full-page
openacosmi browser screenshot --ref 12
openacosmi browser screenshot --ref e12
openacosmi browser screenshot output.png --type jpeg
```

## Actions (ref-based)

Actions use element refs from `snapshot`. Either numeric refs (from AI snapshot) or role refs like `e12` (from role snapshot).

```bash
openacosmi browser navigate https://example.com
openacosmi browser resize 1280 720
openacosmi browser click 12 --double
openacosmi browser click e12
openacosmi browser type 23 "hello" --submit
openacosmi browser press Enter
openacosmi browser hover 44
openacosmi browser scrollintoview e12
openacosmi browser drag 10 11
openacosmi browser select 9 OptionA OptionB
openacosmi browser download e12 /tmp/report.pdf
openacosmi browser waitfordownload /tmp/report.pdf
openacosmi browser upload /tmp/file.pdf --ref 5
openacosmi browser fill --fields '[{"ref":"1","type":"text","value":"Ada"}]'
openacosmi browser dialog --accept
openacosmi browser wait --text "Done"
openacosmi browser wait "#main" --url "**/dash" --load networkidle --fn "window.ready===true"
openacosmi browser evaluate --fn '(el) => el.textContent' --ref 7
openacosmi browser highlight e12
```

## Debug

```bash
openacosmi browser console --level error
openacosmi browser errors --clear
openacosmi browser requests --filter api --clear
openacosmi browser responsebody "**/api" --max-chars 5000
openacosmi browser pdf
openacosmi browser trace start
openacosmi browser trace stop
```

## State management

```bash
openacosmi browser cookies
openacosmi browser cookies set session abc123 --url "https://example.com"
openacosmi browser cookies clear
openacosmi browser storage get local
openacosmi browser storage set local theme dark
openacosmi browser storage clear session
openacosmi browser set offline on
openacosmi browser set headers --json '{"X-Debug":"1"}'
openacosmi browser set credentials user pass
openacosmi browser set credentials --clear
openacosmi browser set geo 37.7749 -122.4194 --origin "https://example.com"
openacosmi browser set geo --clear
openacosmi browser set media dark
openacosmi browser set timezone America/New_York
openacosmi browser set locale en-US
openacosmi browser set device "iPhone 14"
```

## Chrome extension relay (attach via toolbar button)

This mode lets the agent control an existing Chrome tab that you attach manually (it does not auto-attach).

Full guide: [Chrome extension](/tools/chrome-extension)

## Remote browser control (node host proxy)

If the Gateway runs on a different machine than the browser, run a **node host** on the machine that has Chrome/Brave/Edge/Chromium. The Gateway will proxy browser actions to that node (no separate browser control server required).

Use `gateway.nodes.browser.mode` to control auto-routing and `gateway.nodes.browser.node` to pin a specific node if multiple are connected.

Security + remote setup: [Browser tool](/tools/browser), [Remote access](/gateway/remote), [Tailscale](/gateway/tailscale), [Security](/gateway/security)

## RPC protocol

All CLI commands use the single Gateway RPC method `browser.request`:

```json
{
  "method": "browser.request",
  "params": {
    "method": "GET|POST|DELETE",
    "path": "/...",
    "query": {},
    "body": {}
  }
}
```

The Gateway routes this to the local browser control HTTP service or proxies to a connected node.
