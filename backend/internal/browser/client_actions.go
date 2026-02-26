package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ClientActions provides HTTP client wrappers for browser control server actions.
// TS source: client-actions.ts (~778L) — this is a remote HTTP client that talks
// to the BrowserServer, as opposed to the Client which uses CDP directly.
//
// Use ClientActions when the browser is controlled via a remote HTTP bridge
// (e.g. in embedded/gateway builds).
type ClientActions struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewClientActions creates a new browser actions HTTP client.
func NewClientActions(baseURL, authToken string) *ClientActions {
	return &ClientActions{
		baseURL: baseURL,
		token:   authToken,
		client:  http.DefaultClient,
	}
}

// ── Core Navigation ──

// Navigate navigates a browser profile to a URL.
func (a *ClientActions) Navigate(ctx context.Context, profile, url string) error {
	_, err := a.post(ctx, "/navigate", map[string]string{"profile": profile, "url": url})
	return err
}

// Screenshot captures a screenshot from a browser profile.
func (a *ClientActions) Screenshot(ctx context.Context, profile string) ([]byte, error) {
	return a.postRaw(ctx, "/screenshot", map[string]string{"profile": profile})
}

// Evaluate runs JavaScript in a browser profile.
func (a *ClientActions) Evaluate(ctx context.Context, profile, expression string) (json.RawMessage, error) {
	resp, err := a.post(ctx, "/evaluate", map[string]any{
		"profile":    profile,
		"expression": expression,
	})
	if err != nil {
		return nil, err
	}
	result, ok := resp["result"]
	if !ok {
		return nil, nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// ── Lifecycle ──

// Launch starts a browser profile.
func (a *ClientActions) Launch(ctx context.Context, profile string) error {
	_, err := a.post(ctx, "/launch", map[string]string{"profile": profile})
	return err
}

// Close closes a browser profile session.
func (a *ClientActions) Close(ctx context.Context, profile string) error {
	_, err := a.post(ctx, "/close", map[string]string{"profile": profile})
	return err
}

// ── Observation ──

// Status returns the server status.
func (a *ClientActions) Status(ctx context.Context) (map[string]any, error) {
	return a.get(ctx, "/status")
}

// ── Agent Actions ──

// AgentSnapshot captures a snapshot from the agent API.
func (a *ClientActions) AgentSnapshot(ctx context.Context, cdpURL, targetID, mode string, limit int) (map[string]any, error) {
	return a.post(ctx, "/agent/snapshot", map[string]any{
		"cdpUrl": cdpURL, "targetId": targetID, "mode": mode, "limit": limit,
	})
}

// AgentAct performs a browser action via the agent API.
func (a *ClientActions) AgentAct(ctx context.Context, params map[string]any) (map[string]any, error) {
	return a.post(ctx, "/agent/act", params)
}

// AgentWait waits for a condition via the agent API.
func (a *ClientActions) AgentWait(ctx context.Context, params map[string]any) (map[string]any, error) {
	return a.post(ctx, "/agent/wait", params)
}

// ── Storage Actions ──

// CookiesGet returns all cookies.
func (a *ClientActions) CookiesGet(ctx context.Context, cdpURL, targetID string) (map[string]any, error) {
	return a.get(ctx, fmt.Sprintf("/agent/cookies?cdpUrl=%s&targetId=%s", cdpURL, targetID))
}

// CookiesSet adds or overwrites a cookie.
func (a *ClientActions) CookiesSet(ctx context.Context, params map[string]any) error {
	_, err := a.post(ctx, "/agent/cookies/set", params)
	return err
}

// CookiesClear removes all cookies.
func (a *ClientActions) CookiesClear(ctx context.Context, cdpURL, targetID string) error {
	_, err := a.post(ctx, "/agent/cookies/clear", map[string]string{"cdpUrl": cdpURL, "targetId": targetID})
	return err
}

// StorageGet returns storage entries (local or session).
func (a *ClientActions) StorageGet(ctx context.Context, cdpURL, targetID, kind, key string) (map[string]any, error) {
	return a.get(ctx, fmt.Sprintf("/agent/storage?cdpUrl=%s&targetId=%s&kind=%s&key=%s", cdpURL, targetID, kind, key))
}

// StorageSet sets a storage key-value pair.
func (a *ClientActions) StorageSet(ctx context.Context, cdpURL, targetID, kind, key, value string) error {
	_, err := a.post(ctx, "/agent/storage/set", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "kind": kind, "key": key, "value": value,
	})
	return err
}

// StorageClear clears storage.
func (a *ClientActions) StorageClear(ctx context.Context, cdpURL, targetID, kind string) error {
	_, err := a.post(ctx, "/agent/storage/clear", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "kind": kind,
	})
	return err
}

// ── Debug/Activity Actions ──

// GetConsoleMessages returns captured console messages.
func (a *ClientActions) GetConsoleMessages(ctx context.Context, cdpURL, targetID, level string) (map[string]any, error) {
	return a.get(ctx, fmt.Sprintf("/agent/console?cdpUrl=%s&targetId=%s&level=%s", cdpURL, targetID, level))
}

// GetPageErrors returns captured JS errors.
func (a *ClientActions) GetPageErrors(ctx context.Context, cdpURL, targetID string) (map[string]any, error) {
	return a.get(ctx, fmt.Sprintf("/agent/errors?cdpUrl=%s&targetId=%s", cdpURL, targetID))
}

// GetNetworkRequests returns captured network requests.
func (a *ClientActions) GetNetworkRequests(ctx context.Context, cdpURL, targetID, filter string) (map[string]any, error) {
	return a.get(ctx, fmt.Sprintf("/agent/requests?cdpUrl=%s&targetId=%s&filter=%s", cdpURL, targetID, filter))
}

// ── Trace Actions ──

// TraceStart begins CDP tracing.
func (a *ClientActions) TraceStart(ctx context.Context, cdpURL, targetID string, screenshots, snapshots bool) error {
	_, err := a.post(ctx, "/agent/trace/start", map[string]any{
		"cdpUrl": cdpURL, "targetId": targetID, "screenshots": screenshots, "snapshots": snapshots,
	})
	return err
}

// TraceStop stops CDP tracing and saves to path.
func (a *ClientActions) TraceStop(ctx context.Context, cdpURL, targetID, path string) error {
	_, err := a.post(ctx, "/agent/trace/stop", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "path": path,
	})
	return err
}

// ── State Emulation Actions ──

// SetViewport sets the viewport size.
func (a *ClientActions) SetViewport(ctx context.Context, cdpURL, targetID string, width, height int) error {
	_, err := a.post(ctx, "/agent/state/viewport", map[string]any{
		"cdpUrl": cdpURL, "targetId": targetID, "width": width, "height": height,
	})
	return err
}

// SetUserAgent overrides the user agent string.
func (a *ClientActions) SetUserAgent(ctx context.Context, cdpURL, targetID, userAgent string) error {
	_, err := a.post(ctx, "/agent/state/useragent", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "userAgent": userAgent,
	})
	return err
}

// SetGeolocation overrides the geolocation.
func (a *ClientActions) SetGeolocation(ctx context.Context, cdpURL, targetID string, lat, lng float64) error {
	_, err := a.post(ctx, "/agent/state/geolocation", map[string]any{
		"cdpUrl": cdpURL, "targetId": targetID, "latitude": lat, "longitude": lng,
	})
	return err
}

// SetTimezone overrides the timezone.
func (a *ClientActions) SetTimezone(ctx context.Context, cdpURL, targetID, timezoneID string) error {
	_, err := a.post(ctx, "/agent/state/timezone", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "timezoneId": timezoneID,
	})
	return err
}

// SetLocale overrides the locale.
func (a *ClientActions) SetLocale(ctx context.Context, cdpURL, targetID, locale string) error {
	_, err := a.post(ctx, "/agent/state/locale", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "locale": locale,
	})
	return err
}

// SetColorScheme emulates a color scheme.
func (a *ClientActions) SetColorScheme(ctx context.Context, cdpURL, targetID, scheme string) error {
	_, err := a.post(ctx, "/agent/state/colorscheme", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "colorScheme": scheme,
	})
	return err
}

// SetOffline enables or disables offline emulation.
func (a *ClientActions) SetOffline(ctx context.Context, cdpURL, targetID string, offline bool) error {
	_, err := a.post(ctx, "/agent/state/offline", map[string]any{
		"cdpUrl": cdpURL, "targetId": targetID, "offline": offline,
	})
	return err
}

// ── Page Lifecycle Actions ──

// ClosePage closes a target page.
func (a *ClientActions) ClosePage(ctx context.Context, cdpURL, targetID string) error {
	_, err := a.post(ctx, "/agent/page/close", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID,
	})
	return err
}

// PrintPDF generates a PDF of the page.
func (a *ClientActions) PrintPDF(ctx context.Context, cdpURL, targetID, path string) ([]byte, error) {
	return a.postRaw(ctx, "/agent/page/pdf", map[string]string{
		"cdpUrl": cdpURL, "targetId": targetID, "path": path,
	})
}

// ResizeViewport resizes the page viewport.
func (a *ClientActions) ResizeViewport(ctx context.Context, cdpURL, targetID string, width, height int) error {
	_, err := a.post(ctx, "/agent/page/resize", map[string]any{
		"cdpUrl": cdpURL, "targetId": targetID, "width": width, "height": height,
	})
	return err
}

// ── HTTP helpers ──

func (a *ClientActions) post(ctx context.Context, path string, body any) (map[string]any, error) {
	raw, err := a.postRaw(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("browser action %s: decode: %w", path, err)
	}
	if errMsg, ok := result["error"]; ok {
		return nil, fmt.Errorf("browser action %s: %v", path, errMsg)
	}
	return result, nil
}

func (a *ClientActions) postRaw(ctx context.Context, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browser action %s: %w", path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("browser action %s: HTTP %d: %s", path, resp.StatusCode, string(data))
	}
	return data, nil
}

func (a *ClientActions) get(ctx context.Context, path string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browser action %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("browser action %s: HTTP %d: %s", path, resp.StatusCode, string(text))
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
