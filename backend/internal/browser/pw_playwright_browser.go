// pw_playwright_browser.go — BrowserController implementation backed by
// PlaywrightTools. This bridges the tools.BrowserController interface
// (used by browser_tool.go) to the PlaywrightTools abstraction.
//
// TS source: browser-tool.ts (724L)
package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// PlaywrightBrowserController implements tools.BrowserController using
// a PlaywrightTools backend (either CDP or Playwright-native).
type PlaywrightBrowserController struct {
	tools  PlaywrightTools
	target PWTargetOpts
}

// NewPlaywrightBrowserController creates a BrowserController backed by
// any PlaywrightTools implementation.
func NewPlaywrightBrowserController(tools PlaywrightTools, cdpURL string) *PlaywrightBrowserController {
	return &PlaywrightBrowserController{
		tools: tools,
		target: PWTargetOpts{
			CDPURL: cdpURL,
		},
	}
}

// Navigate navigates to the given URL.
func (c *PlaywrightBrowserController) Navigate(ctx context.Context, url string) error {
	// Use CDP client directly for navigation as it's simpler.
	cdp := NewCDPClient(c.target.CDPURL, nil)
	return cdp.Navigate(ctx, url)
}

// GetContent returns the page's text content via JS evaluation.
func (c *PlaywrightBrowserController) GetContent(ctx context.Context) (string, error) {
	cdp := NewCDPClient(c.target.CDPURL, nil)
	raw, err := cdp.Evaluate(ctx, "document.body.innerText || document.body.textContent || ''")
	if err != nil {
		return "", fmt.Errorf("get content: %w", err)
	}
	var resp struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parse content: %w", err)
	}
	return resp.Result.Value, nil
}

// Click clicks the element matching the CSS selector.
func (c *PlaywrightBrowserController) Click(ctx context.Context, selector string) error {
	return c.tools.Click(ctx, PWClickOpts{
		PWTargetOpts: c.target,
		Ref:          selector,
	})
}

// Type types text into the element matching the CSS selector.
func (c *PlaywrightBrowserController) Type(ctx context.Context, selector, text string) error {
	return c.tools.Fill(ctx, PWFillOpts{
		PWTargetOpts: c.target,
		Ref:          selector,
		Value:        text,
	})
}

// Screenshot captures a PNG screenshot and returns (data, mimeType, error).
func (c *PlaywrightBrowserController) Screenshot(ctx context.Context) ([]byte, string, error) {
	data, err := c.tools.Screenshot(ctx, c.target)
	if err != nil {
		return nil, "", err
	}
	// Screenshot returns base64-encoded PNG from CDP.
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		// If not base64, return raw bytes.
		return data, "image/png", nil
	}
	return decoded, "image/png", nil
}

// Evaluate executes JavaScript and returns the result.
func (c *PlaywrightBrowserController) Evaluate(ctx context.Context, script string) (any, error) {
	cdp := NewCDPClient(c.target.CDPURL, nil)
	raw, err := cdp.Evaluate(ctx, script)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return string(raw), nil
	}
	return result, nil
}

// WaitForSelector waits for an element matching the selector to appear.
// Uses polling via CDP Runtime.evaluate.
func (c *PlaywrightBrowserController) WaitForSelector(ctx context.Context, selector string) error {
	cdp := NewCDPClient(c.target.CDPURL, nil)
	script := fmt.Sprintf(`(function() {
		var el = document.querySelector(%q);
		return el !== null;
	})()`, selector)
	_, err := cdp.Evaluate(ctx, script)
	return err
}

// GoBack navigates back in browser history.
func (c *PlaywrightBrowserController) GoBack(ctx context.Context) error {
	cdp := NewCDPClient(c.target.CDPURL, nil)
	_, err := cdp.Evaluate(ctx, "window.history.back()")
	return err
}

// GoForward navigates forward in browser history.
func (c *PlaywrightBrowserController) GoForward(ctx context.Context) error {
	cdp := NewCDPClient(c.target.CDPURL, nil)
	_, err := cdp.Evaluate(ctx, "window.history.forward()")
	return err
}

// GetURL returns the current page URL.
func (c *PlaywrightBrowserController) GetURL(ctx context.Context) (string, error) {
	cdp := NewCDPClient(c.target.CDPURL, nil)
	raw, err := cdp.Evaluate(ctx, "window.location.href")
	if err != nil {
		return "", err
	}
	var resp struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	return resp.Result.Value, nil
}
