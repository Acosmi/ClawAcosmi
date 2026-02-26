# Browser Automation International Benchmark Research

**Date**: 2026-02-25
**Purpose**: Research international top-tier open-source projects and Chinese big tech solutions to elevate our browser automation to world-class standards.

---

## Key Projects Analyzed

| Project | Origin | Category | Key Innovation |
|---------|--------|----------|----------------|
| Playwright | Microsoft | Full framework | Auto-waiting, 5-point actionability checks |
| Puppeteer | Google | CDP wrapper | Page.lifecycleEvent for navigation tracking |
| Rod | Go community | Go CDP library | Persistent WebSocket, launcher state persistence |
| Chromedp | Go community | Go CDP library | Context-based API, composable Action interface |
| browser-use | Open source | AI automation | CDP event bus + watchdog architecture |
| Stagehand v3 | Browserbase | AI automation | Action caching (44% speed improvement), driver-agnostic |
| Midscene.js | ByteDance | Vision automation | Pure-vision element localization |
| UI-TARS | ByteDance | Desktop agent | Operator abstraction pattern |
| OmniParser V2 | Microsoft | Visual grounding | YOLO+Florence-2 (0.6-0.8s latency) |

---

## Priority Architecture Improvements

### P0: Persistent CDP Session (Highest Impact)

**Problem**: Current `WithCdpSocket()` opens/closes WebSocket per operation.
**Solution**: `CDPSession` with persistent connection + event bus.
**Impact**: 10-50x faster for multi-step operations.
**Reference**: browser-use, Rod

### P1: CDP Event Handling

**Problem**: `cdpSender.readLoop()` discards all CDP events (ID=0 messages).
**Solution**: Route events to `EventBus` subscribers.
**Enables**: Download watchdog, dialog handling, navigation tracking, crash detection.
**Reference**: browser-use bubus architecture

### P2: Actionability Checks (Playwright Pattern)

**Problem**: Click/Fill/Hover dispatch immediately without verifying element state.
**Solution**: Pre-action 5-point check: attached, visible, stable, receives events, enabled.
**Impact**: Dramatically improves interaction reliability.
**Reference**: Playwright auto-waiting

### P3: Watchdog Services

- `NavigationWatchdog`: Page.lifecycleEvent subscriptions
- `DownloadWatchdog`: Page.downloadWillBegin/Progress
- `DialogWatchdog`: Page.javascriptDialogOpening
- `CrashWatchdog`: Inspector.targetCrashed
**Reference**: browser-use event-driven architecture

### P4: AI Loop Enhancements

- Action caching by page state hash (Stagehand v3: 44% speed gain)
- State history for loop detection (Cypress time-travel pattern)
- Operator abstraction for pluggable execution (UI-TARS pattern)
- Hybrid vision fallback (ARIA first, screenshot if insufficient)

### P5: Connection Resilience

- Exponential backoff reconnection
- Session state persistence to JSON (Rod pattern)
- Chrome process crash recovery
- Rate limiting on CDP commands

---

## Implementation Plan

| Phase | Items | Effort | Files |
|-------|-------|--------|-------|
| Phase 1 | CDPSession + EventBus | 2-3 days | cdp_session.go (new), cdp_helpers.go |
| Phase 2 | Actionability checks | 1-2 days | pw_tools_cdp.go |
| Phase 3 | Watchdog services | 2-3 days | watchdog_*.go (new) |
| Phase 4 | AI loop improvements | 2-3 days | pw_ai_loop.go |
| Phase 5 | Resilience hardening | 1-2 days | cdp_session.go, chrome.go |

---

## Key Code Patterns to Adopt

### Composable Action Interface (Chromedp)
```go
type BrowserAction interface {
    Do(ctx context.Context, session *CDPSession) error
}
```

### Event Bus (browser-use)
```go
type EventBus struct {
    handlers map[string][]func(json.RawMessage)
}
func (eb *EventBus) Subscribe(method string, fn func(json.RawMessage))
func (eb *EventBus) Emit(method string, params json.RawMessage)
```

### Operator Abstraction (UI-TARS)
```go
type Operator interface {
    Execute(ctx context.Context, action AIBrowseAction) error
    Observe(ctx context.Context) (*AIBrowseState, error)
}
```
