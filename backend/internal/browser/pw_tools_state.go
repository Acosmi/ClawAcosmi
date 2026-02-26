// pw_tools_state.go — Page emulation/state functions via CDP.
//
// Implements page-level emulation settings: viewport, user agent,
// geolocation, timezone, locale, color scheme, extra HTTP headers.
//
// TS source: pw-tools-core.state.ts (209L)
// All functions use CDP Emulation domain commands directly.
package browser

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// --- Parameter types ---

// PWSetViewportOpts parameters for SetViewport.
type PWSetViewportOpts struct {
	PWTargetOpts
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor,omitempty"`
	Mobile            bool    `json:"mobile,omitempty"`
}

// PWSetUserAgentOpts parameters for SetUserAgent.
type PWSetUserAgentOpts struct {
	PWTargetOpts
	UserAgent      string `json:"userAgent"`
	AcceptLanguage string `json:"acceptLanguage,omitempty"`
	Platform       string `json:"platform,omitempty"`
}

// PWSetGeolocationOpts parameters for SetGeolocation.
type PWSetGeolocationOpts struct {
	PWTargetOpts
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Accuracy  float64  `json:"accuracy,omitempty"`
	Clear     bool     `json:"clear,omitempty"`
}

// PWSetTimezoneOpts parameters for SetTimezone.
type PWSetTimezoneOpts struct {
	PWTargetOpts
	TimezoneID string `json:"timezoneId"`
}

// PWSetLocaleOpts parameters for SetLocale.
type PWSetLocaleOpts struct {
	PWTargetOpts
	Locale string `json:"locale"`
}

// PWSetColorSchemeOpts parameters for SetColorScheme.
type PWSetColorSchemeOpts struct {
	PWTargetOpts
	ColorScheme string `json:"colorScheme"` // "dark" | "light" | "no-preference"
}

// PWSetExtraHTTPHeadersOpts parameters for SetExtraHTTPHeaders.
type PWSetExtraHTTPHeadersOpts struct {
	PWTargetOpts
	Headers map[string]string `json:"headers"`
}

// PWSetOfflineOpts parameters for SetOffline.
type PWSetOfflineOpts struct {
	PWTargetOpts
	Offline bool `json:"offline"`
}

// --- CDPPlaywrightTools state methods ---

// SetViewport sets the viewport size and device metrics via CDP.
// CDP command: Emulation.setDeviceMetricsOverride
func (t *CDPPlaywrightTools) SetViewport(ctx context.Context, opts PWSetViewportOpts) error {
	if opts.Width <= 0 || opts.Height <= 0 {
		return fmt.Errorf("viewport width and height must be positive")
	}
	dsf := opts.DeviceScaleFactor
	if dsf <= 0 {
		dsf = 1.0
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		_, err := send("Emulation.setDeviceMetricsOverride", map[string]any{
			"width":             opts.Width,
			"height":            opts.Height,
			"deviceScaleFactor": dsf,
			"mobile":            opts.Mobile,
			"screenWidth":       opts.Width,
			"screenHeight":      opts.Height,
		})
		return err
	})
}

// SetUserAgent overrides the browser user agent string via CDP.
// CDP command: Emulation.setUserAgentOverride
func (t *CDPPlaywrightTools) SetUserAgent(ctx context.Context, opts PWSetUserAgentOpts) error {
	ua := strings.TrimSpace(opts.UserAgent)
	if ua == "" {
		return fmt.Errorf("userAgent is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		params := map[string]any{"userAgent": ua}
		if opts.AcceptLanguage != "" {
			params["acceptLanguage"] = opts.AcceptLanguage
		}
		if opts.Platform != "" {
			params["platform"] = opts.Platform
		}
		_, err := send("Emulation.setUserAgentOverride", params)
		return err
	})
}

// SetGeolocation overrides the geolocation via CDP.
// CDP command: Emulation.setGeolocationOverride
func (t *CDPPlaywrightTools) SetGeolocation(ctx context.Context, opts PWSetGeolocationOpts) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		if opts.Clear {
			_, err := send("Emulation.clearGeolocationOverride", nil)
			return err
		}
		if opts.Latitude == nil || opts.Longitude == nil {
			return fmt.Errorf("latitude and longitude are required")
		}
		params := map[string]any{
			"latitude":  *opts.Latitude,
			"longitude": *opts.Longitude,
		}
		if opts.Accuracy > 0 {
			params["accuracy"] = opts.Accuracy
		}
		_, err := send("Emulation.setGeolocationOverride", params)
		return err
	})
}

// SetTimezone overrides the timezone via CDP.
// CDP command: Emulation.setTimezoneOverride
func (t *CDPPlaywrightTools) SetTimezone(ctx context.Context, opts PWSetTimezoneOpts) error {
	tz := strings.TrimSpace(opts.TimezoneID)
	if tz == "" {
		return fmt.Errorf("timezoneId is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		_, err := send("Emulation.setTimezoneOverride", map[string]any{
			"timezoneId": tz,
		})
		if err != nil && strings.Contains(err.Error(), "already in effect") {
			return nil // idempotent
		}
		return err
	})
}

// SetLocale overrides the navigator.language and Accept-Language via CDP.
// CDP command: Emulation.setLocaleOverride
func (t *CDPPlaywrightTools) SetLocale(ctx context.Context, opts PWSetLocaleOpts) error {
	locale := strings.TrimSpace(opts.Locale)
	if locale == "" {
		return fmt.Errorf("locale is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		_, err := send("Emulation.setLocaleOverride", map[string]any{
			"locale": locale,
		})
		if err != nil && strings.Contains(err.Error(), "already in effect") {
			return nil // idempotent
		}
		return err
	})
}

// SetColorScheme emulates a preferred color scheme via CDP.
// CDP command: Emulation.setEmulatedMedia
func (t *CDPPlaywrightTools) SetColorScheme(ctx context.Context, opts PWSetColorSchemeOpts) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		features := []map[string]string{}
		if opts.ColorScheme != "" {
			features = append(features, map[string]string{
				"name":  "prefers-color-scheme",
				"value": opts.ColorScheme,
			})
		}
		_, err := send("Emulation.setEmulatedMedia", map[string]any{
			"features": features,
		})
		return err
	})
}

// SetExtraHTTPHeaders sets extra HTTP headers via CDP.
// CDP command: Network.setExtraHTTPHeaders
func (t *CDPPlaywrightTools) SetExtraHTTPHeaders(ctx context.Context, opts PWSetExtraHTTPHeadersOpts) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		if _, err := send("Network.enable", nil); err != nil {
			return err
		}
		_, err := send("Network.setExtraHTTPHeaders", map[string]any{
			"headers": opts.Headers,
		})
		return err
	})
}

// SetOffline enables or disables network offline emulation via CDP.
// CDP command: Network.emulateNetworkConditions
func (t *CDPPlaywrightTools) SetOffline(ctx context.Context, opts PWSetOfflineOpts) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(opts.PWTargetOpts), func(send CdpSendFn) error {
		if _, err := send("Network.enable", nil); err != nil {
			return err
		}
		_, err := send("Network.emulateNetworkConditions", map[string]any{
			"offline":            opts.Offline,
			"latency":            0,
			"downloadThroughput": -1,
			"uploadThroughput":   -1,
		})
		return err
	})
}

// SetTouchEmulation enables or disables touch emulation via CDP.
// CDP command: Emulation.setTouchEmulationEnabled
func (t *CDPPlaywrightTools) SetTouchEmulation(ctx context.Context, target PWTargetOpts, enabled bool) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	return WithCdpSocket(ctx, t.resolveTargetWsURL(target), func(send CdpSendFn) error {
		_, err := send("Emulation.setTouchEmulationEnabled", map[string]any{
			"enabled": enabled,
		})
		return err
	})
}
