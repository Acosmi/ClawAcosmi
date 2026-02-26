// Package browser — Agent proxy routes for browser automation.
// TS source: routes/agent.ts (14L) + agent.act.ts (541L) + agent.snapshot.ts (329L)
//   - agent.storage.ts (435L) + agent.debug.ts (151L)
//
// These routes expose a high-level HTTP API that AI agents use to interact
// with the browser. They map to the PlaywrightTools interface methods.
package browser

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// AgentRoutes registers all agent-facing routes on a ServeMux.
// prefix is typically "/agent".
func RegisterAgentRoutes(mux *http.ServeMux, tools PlaywrightTools, client *Client, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	r := &agentRouter{
		tools:  tools,
		client: client,
		logger: logger,
	}

	// Snapshot routes (agent.snapshot.ts)
	mux.HandleFunc("POST /agent/snapshot", r.handleSnapshot)
	mux.HandleFunc("POST /agent/screenshot", r.handleScreenshot)
	mux.HandleFunc("POST /agent/navigate", r.handleNavigate)

	// Act routes (agent.act.ts)
	mux.HandleFunc("POST /agent/act", r.handleAct)
	mux.HandleFunc("POST /agent/wait", r.handleWait)
	mux.HandleFunc("POST /agent/evaluate", r.handleEvaluate)

	// Storage routes (agent.storage.ts)
	mux.HandleFunc("GET /agent/cookies", r.handleCookiesGet)
	mux.HandleFunc("POST /agent/cookies/set", r.handleCookiesSet)
	mux.HandleFunc("POST /agent/cookies/clear", r.handleCookiesClear)
	mux.HandleFunc("GET /agent/storage", r.handleStorageGet)
	mux.HandleFunc("POST /agent/storage/set", r.handleStorageSet)
	mux.HandleFunc("POST /agent/storage/clear", r.handleStorageClear)

	// Debug routes (agent.debug.ts)
	mux.HandleFunc("GET /agent/console", r.handleConsole)
	mux.HandleFunc("GET /agent/errors", r.handleErrors)
	mux.HandleFunc("GET /agent/requests", r.handleRequests)

	// Trace routes (pw-tools-core.trace.ts)
	mux.HandleFunc("POST /agent/trace/start", r.handleTraceStart)
	mux.HandleFunc("POST /agent/trace/stop", r.handleTraceStop)

	// State emulation routes (pw-tools-core.state.ts)
	mux.HandleFunc("POST /agent/state/viewport", r.handleSetViewport)
	mux.HandleFunc("POST /agent/state/useragent", r.handleSetUserAgent)
	mux.HandleFunc("POST /agent/state/geolocation", r.handleSetGeolocation)
	mux.HandleFunc("POST /agent/state/timezone", r.handleSetTimezone)
	mux.HandleFunc("POST /agent/state/locale", r.handleSetLocale)
	mux.HandleFunc("POST /agent/state/colorscheme", r.handleSetColorScheme)
	mux.HandleFunc("POST /agent/state/headers", r.handleSetExtraHTTPHeaders)
	mux.HandleFunc("POST /agent/state/offline", r.handleSetOffline)

	// Page lifecycle routes (BR-M06)
	mux.HandleFunc("POST /agent/page/close", r.handleClosePage)
	mux.HandleFunc("POST /agent/page/pdf", r.handlePrintPDF)
	mux.HandleFunc("POST /agent/page/resize", r.handleResizeViewport)
}

type agentRouter struct {
	tools  PlaywrightTools
	client *Client
	logger *slog.Logger
}

// ── Snapshot routes ──

func (r *agentRouter) handleSnapshot(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
		Mode     string `json:"mode"` // "aria" | "ai" | "role" (default)
		Limit    int    `json:"limit"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	opts := PWSnapshotOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID},
		Limit:        body.Limit,
	}

	switch strings.ToLower(body.Mode) {
	case "aria":
		nodes, err := r.tools.SnapshotAria(req.Context(), opts)
		if err != nil {
			r.handleRouteError(w, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "nodes": nodes})

	case "ai":
		result, err := r.tools.SnapshotAI(req.Context(), opts)
		if err != nil {
			r.handleRouteError(w, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "snapshot": result})

	default: // "role" — same as AI but potentially with different options
		result, err := r.tools.SnapshotAI(req.Context(), opts)
		if err != nil {
			r.handleRouteError(w, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "snapshot": result})
	}
}

func (r *agentRouter) handleScreenshot(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	data, err := r.tools.Screenshot(req.Context(), PWTargetOpts{
		CDPURL:   body.CDPURL,
		TargetID: body.TargetID,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

func (r *agentRouter) handleNavigate(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
		Profile  string `json:"profile"`
		URL      string `json:"url"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	url := strings.TrimSpace(body.URL)
	if url == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("url is required"))
		return
	}

	// Use client navigate if profile provided, otherwise use CDP directly
	if body.Profile != "" && r.client != nil {
		if err := r.client.Navigate(req.Context(), body.Profile, url); err != nil {
			r.handleRouteError(w, err)
			return
		}
	}
	writeJSON(w, map[string]any{"ok": true, "url": url})
}

// ── Act routes ──

// actKinds maps action names to handler functions.
var actKinds = map[string]bool{
	"click": true, "fill": true, "hover": true, "highlight": true,
	"type": true, "pressKey": true, "selectOption": true,
	"scrollIntoView": true, "drag": true, "setInputFiles": true,
	"evaluate": true, "fillForm": true,
}

func (r *agentRouter) handleAct(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL    string `json:"cdpUrl"`
		TargetID  string `json:"targetId"`
		Action    string `json:"action"`
		Ref       string `json:"ref"`
		Value     string `json:"value"`
		Text      string `json:"text"`
		Key       string `json:"key"`
		Button    string `json:"button"`
		TimeoutMs int    `json:"timeoutMs"`
		// Click-specific
		DoubleClick bool     `json:"doubleClick"`
		Modifiers   []string `json:"modifiers"`
		// Drag-specific
		StartRef string `json:"startRef"`
		EndRef   string `json:"endRef"`
		// Select-specific
		Values []string `json:"values"`
		// PressKey-specific
		DelayMs int `json:"delayMs"`
		// Type-specific
		Submit bool `json:"submit"`
		Slowly bool `json:"slowly"`
		// FillForm-specific
		Fields []struct {
			Ref   string `json:"ref"`
			Type  string `json:"type"`
			Value any    `json:"value"`
		} `json:"fields"`
		// SetInputFiles-specific
		Element string   `json:"element"`
		Paths   []string `json:"paths"`
		// Evaluate-specific
		Expression string `json:"expression"`
		Fn         string `json:"fn"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	action := strings.ToLower(strings.TrimSpace(body.Action))
	if action == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("action is required"))
		return
	}
	if !actKinds[action] {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown action %q", action))
		return
	}

	targetOpts := PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID}

	var err error
	switch action {
	case "click":
		err = r.tools.Click(req.Context(), PWClickOpts{
			PWTargetOpts: targetOpts,
			Ref:          body.Ref,
			DoubleClick:  body.DoubleClick,
			Button:       body.Button,
			Modifiers:    body.Modifiers,
			TimeoutMs:    body.TimeoutMs,
		})

	case "fill":
		value := body.Value
		if value == "" {
			value = body.Text
		}
		err = r.tools.Fill(req.Context(), PWFillOpts{
			PWTargetOpts: targetOpts,
			Ref:          body.Ref,
			Value:        value,
			TimeoutMs:    body.TimeoutMs,
		})

	case "hover":
		err = r.tools.Hover(req.Context(), targetOpts, body.Ref, body.TimeoutMs)

	case "highlight":
		err = r.tools.Highlight(req.Context(), targetOpts, body.Ref)

	case "type":
		err = r.tools.Type(req.Context(), PWTypeOpts{
			PWTargetOpts: targetOpts,
			Ref:          body.Ref,
			Text:         body.Text,
			Submit:       body.Submit,
			Slowly:       body.Slowly,
			TimeoutMs:    body.TimeoutMs,
		})

	case "pressKey":
		err = r.tools.PressKey(req.Context(), PWPressKeyOpts{
			PWTargetOpts: targetOpts,
			Key:          body.Key,
			DelayMs:      body.DelayMs,
		})

	case "selectOption":
		err = r.tools.SelectOption(req.Context(), PWSelectOptionOpts{
			PWTargetOpts: targetOpts,
			Ref:          body.Ref,
			Values:       body.Values,
			TimeoutMs:    body.TimeoutMs,
		})

	case "scrollIntoView":
		err = r.tools.ScrollIntoView(req.Context(), PWScrollIntoViewOpts{
			PWTargetOpts: targetOpts,
			Ref:          body.Ref,
			TimeoutMs:    body.TimeoutMs,
		})

	case "drag":
		err = r.tools.Drag(req.Context(), PWDragOpts{
			PWTargetOpts: targetOpts,
			StartRef:     body.StartRef,
			EndRef:       body.EndRef,
			TimeoutMs:    body.TimeoutMs,
		})

	case "setInputFiles":
		err = r.tools.SetInputFiles(req.Context(), PWSetInputFilesOpts{
			PWTargetOpts: targetOpts,
			Ref:          body.Ref,
			Element:      body.Element,
			Paths:        body.Paths,
		})

	case "evaluate":
		expr := body.Expression
		if expr == "" {
			expr = body.Fn
		}
		result, evalErr := r.tools.Evaluate(req.Context(), PWEvaluateOpts{
			PWTargetOpts: targetOpts,
			Expression:   expr,
			Ref:          body.Ref,
		})
		if evalErr != nil {
			r.handleRouteError(w, evalErr)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": result})
		return

	case "fillForm":
		for _, field := range body.Fields {
			fieldRef := strings.TrimSpace(field.Ref)
			fieldType := strings.TrimSpace(field.Type)
			if fieldRef == "" || fieldType == "" {
				continue
			}
			valueStr := ""
			switch v := field.Value.(type) {
			case string:
				valueStr = v
			case float64:
				valueStr = fmt.Sprintf("%v", v)
			case bool:
				if v {
					valueStr = "true"
				} else {
					valueStr = "false"
				}
			}

			if fieldType == "checkbox" || fieldType == "radio" {
				// For checkbox/radio, click to toggle
				err = r.tools.Click(req.Context(), PWClickOpts{
					PWTargetOpts: targetOpts,
					Ref:          fieldRef,
					TimeoutMs:    body.TimeoutMs,
				})
			} else {
				err = r.tools.Fill(req.Context(), PWFillOpts{
					PWTargetOpts: targetOpts,
					Ref:          fieldRef,
					Value:        valueStr,
					TimeoutMs:    body.TimeoutMs,
				})
			}
			if err != nil {
				break
			}
		}
	}

	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleWait(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL    string `json:"cdpUrl"`
		TargetID  string `json:"targetId"`
		TimeMs    int    `json:"timeMs"`
		Text      string `json:"text"`
		TextGone  string `json:"textGone"`
		URL       string `json:"url"`
		LoadState string `json:"loadState"`
		Fn        string `json:"fn"`
		TimeoutMs int    `json:"timeoutMs"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	err := r.tools.WaitFor(req.Context(), PWWaitForOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID},
		TimeMs:       body.TimeMs,
		Text:         body.Text,
		TextGone:     body.TextGone,
		URL:          body.URL,
		LoadState:    body.LoadState,
		Fn:           body.Fn,
		TimeoutMs:    body.TimeoutMs,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleEvaluate(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL     string `json:"cdpUrl"`
		TargetID   string `json:"targetId"`
		Profile    string `json:"profile"`
		Expression string `json:"expression"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	expression := strings.TrimSpace(body.Expression)
	if expression == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("expression is required"))
		return
	}

	// Use client evaluate if profile is provided
	if body.Profile != "" && r.client != nil {
		result, err := r.client.Evaluate(req.Context(), body.Profile, expression)
		if err != nil {
			r.handleRouteError(w, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "result": json.RawMessage(result)})
		return
	}

	// Direct CDP evaluate
	cdpURL := body.CDPURL
	if cdpURL == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cdpUrl or profile is required"))
		return
	}

	var result json.RawMessage
	err := WithCdpSocket(req.Context(), cdpURL, func(send CdpSendFn) error {
		raw, err := send("Runtime.evaluate", map[string]any{
			"expression":    expression,
			"returnByValue": true,
		})
		result = raw
		return err
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "result": result})
}

// ── Storage routes ──

func (r *agentRouter) handleCookiesGet(w http.ResponseWriter, req *http.Request) {
	cdpURL := req.URL.Query().Get("cdpUrl")
	targetID := req.URL.Query().Get("targetId")

	cookies, err := r.tools.CookiesGet(req.Context(), PWTargetOpts{
		CDPURL:   cdpURL,
		TargetID: targetID,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "cookies": cookies})
}

func (r *agentRouter) handleCookiesSet(w http.ResponseWriter, req *http.Request) {
	var body PWCookieSetOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := r.tools.CookiesSet(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleCookiesClear(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := r.tools.CookiesClear(req.Context(), PWTargetOpts{
		CDPURL:   body.CDPURL,
		TargetID: body.TargetID,
	}); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleStorageGet(w http.ResponseWriter, req *http.Request) {
	cdpURL := req.URL.Query().Get("cdpUrl")
	targetID := req.URL.Query().Get("targetId")
	kind := req.URL.Query().Get("kind") // "local" | "session"
	key := req.URL.Query().Get("key")

	values, err := r.tools.StorageGet(req.Context(), PWStorageGetOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: cdpURL, TargetID: targetID},
		Kind:         kind,
		Key:          key,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "values": values})
}

func (r *agentRouter) handleStorageSet(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
		Kind     string `json:"kind"` // "local" | "session"
		Key      string `json:"key"`
		Value    string `json:"value"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := r.tools.StorageSet(req.Context(), PWStorageSetOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID},
		Kind:         body.Kind,
		Key:          body.Key,
		Value:        body.Value,
	}); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleStorageClear(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
		Kind     string `json:"kind"` // "local" | "session"
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := r.tools.StorageClear(req.Context(), PWStorageClearOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID},
		Kind:         body.Kind,
	}); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// ── Debug routes (activity diagnostics) ──

func (r *agentRouter) handleConsole(w http.ResponseWriter, req *http.Request) {
	cdpURL := req.URL.Query().Get("cdpUrl")
	targetID := req.URL.Query().Get("targetId")
	level := req.URL.Query().Get("level")

	msgs, err := r.tools.GetConsoleMessages(req.Context(), PWConsoleMessagesOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: cdpURL, TargetID: targetID},
		Level:        level,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "messages": msgs})
}

func (r *agentRouter) handleErrors(w http.ResponseWriter, req *http.Request) {
	cdpURL := req.URL.Query().Get("cdpUrl")
	targetID := req.URL.Query().Get("targetId")

	errs, err := r.tools.GetPageErrors(req.Context(), PWTargetOpts{
		CDPURL:   cdpURL,
		TargetID: targetID,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "errors": errs})
}

func (r *agentRouter) handleRequests(w http.ResponseWriter, req *http.Request) {
	cdpURL := req.URL.Query().Get("cdpUrl")
	targetID := req.URL.Query().Get("targetId")
	filter := req.URL.Query().Get("filter")

	reqs, err := r.tools.GetNetworkRequests(req.Context(), PWNetworkRequestsOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: cdpURL, TargetID: targetID},
		Filter:       filter,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "requests": reqs})
}

// ── Trace routes ──

func (r *agentRouter) handleTraceStart(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL      string `json:"cdpUrl"`
		TargetID    string `json:"targetId"`
		Screenshots bool   `json:"screenshots"`
		Snapshots   bool   `json:"snapshots"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.tools.TraceStart(req.Context(), PWTraceStartOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID},
		Screenshots:  body.Screenshots,
		Snapshots:    body.Snapshots,
	}); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleTraceStop(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
		Path     string `json:"path"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.tools.TraceStop(req.Context(), PWTraceStopOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID},
		Path:         body.Path,
	}); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// ── State emulation routes ──

func (r *agentRouter) handleSetViewport(w http.ResponseWriter, req *http.Request) {
	var body PWSetViewportOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetViewport(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleSetUserAgent(w http.ResponseWriter, req *http.Request) {
	var body PWSetUserAgentOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetUserAgent(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleSetGeolocation(w http.ResponseWriter, req *http.Request) {
	var body PWSetGeolocationOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetGeolocation(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleSetTimezone(w http.ResponseWriter, req *http.Request) {
	var body PWSetTimezoneOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetTimezone(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleSetLocale(w http.ResponseWriter, req *http.Request) {
	var body PWSetLocaleOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetLocale(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleSetColorScheme(w http.ResponseWriter, req *http.Request) {
	var body PWSetColorSchemeOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetColorScheme(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleSetExtraHTTPHeaders(w http.ResponseWriter, req *http.Request) {
	var body PWSetExtraHTTPHeadersOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetExtraHTTPHeaders(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handleSetOffline(w http.ResponseWriter, req *http.Request) {
	var body PWSetOfflineOpts
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cdpTools, ok := r.tools.(*CDPPlaywrightTools)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("state emulation requires CDP tools"))
		return
	}
	if err := cdpTools.SetOffline(req.Context(), body); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// ── Page lifecycle routes ──

func (r *agentRouter) handleClosePage(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.tools.ClosePage(req.Context(), PWTargetOpts{
		CDPURL:   body.CDPURL,
		TargetID: body.TargetID,
	}); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (r *agentRouter) handlePrintPDF(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
		Path     string `json:"path"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	data, err := r.tools.PrintPDF(req.Context(), PWPrintPDFOpts{
		PWTargetOpts: PWTargetOpts{CDPURL: body.CDPURL, TargetID: body.TargetID},
		Path:         body.Path,
	})
	if err != nil {
		r.handleRouteError(w, err)
		return
	}
	if body.Path != "" {
		writeJSON(w, map[string]any{"ok": true, "path": body.Path})
	} else {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(data)
	}
}

func (r *agentRouter) handleResizeViewport(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CDPURL   string `json:"cdpUrl"`
		TargetID string `json:"targetId"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	}
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.tools.ResizeViewport(req.Context(), PWTargetOpts{
		CDPURL:   body.CDPURL,
		TargetID: body.TargetID,
	}, body.Width, body.Height); err != nil {
		r.handleRouteError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// ── Helpers ──

func (r *agentRouter) handleRouteError(w http.ResponseWriter, err error) {
	r.logger.Error("agent route error", "err", err)
	writeError(w, http.StatusInternalServerError, err)
}

func jsonStringEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
