// Package browser — driver management for browser automation backends.
// Provides unified factory to select between CDP (raw protocol) and
// Playwright (playwright-go) drivers.
//
// Design reference: Google DeepMind Project Mariner dual-mode architecture.
package browser

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/openacosmi/claw-acismi/pkg/i18n"
)

// DriverKind identifies the browser automation backend.
type DriverKind string

const (
	// DriverCDP uses raw Chrome DevTools Protocol via WebSocket.
	// This is the default and requires no external dependencies.
	DriverCDP DriverKind = "cdp"

	// DriverPlaywright uses playwright-go bindings.
	// Requires Node.js runtime and Playwright browser binaries.
	DriverPlaywright DriverKind = "playwright"
)

// DriverManager manages the lifecycle of browser automation drivers.
// It provides a unified entry point for creating PlaywrightTools instances
// regardless of the underlying driver.
type DriverManager struct {
	mu     sync.RWMutex
	kind   DriverKind
	cdpURL string
	logger *slog.Logger

	// Cached driver instances (lazily initialized).
	cdpTools        *CDPPlaywrightTools
	playwrightTools *PlaywrightNativeTools
}

// DriverManagerConfig configures the DriverManager.
type DriverManagerConfig struct {
	Kind   DriverKind // default: DriverCDP
	CDPURL string     // WebSocket URL for CDP mode
	Logger *slog.Logger
}

// NewDriverManager creates a new DriverManager with the given configuration.
func NewDriverManager(cfg DriverManagerConfig) *DriverManager {
	kind := cfg.Kind
	if kind == "" {
		kind = DriverCDP
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info(i18n.T("browser.driver.init", map[string]string{
		"driver": string(kind),
	}))

	return &DriverManager{
		kind:   kind,
		cdpURL: cfg.CDPURL,
		logger: logger,
	}
}

// Tools returns a PlaywrightTools implementation for the configured driver.
// The returned instance is cached for reuse.
func (dm *DriverManager) Tools() (PlaywrightTools, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	switch dm.kind {
	case DriverCDP:
		return dm.cdpToolsLocked(), nil
	case DriverPlaywright:
		return dm.playwrightToolsLocked()
	default:
		return nil, fmt.Errorf("unknown driver kind: %s", dm.kind)
	}
}

// cdpToolsLocked returns the cached CDP tools instance (must hold mu).
func (dm *DriverManager) cdpToolsLocked() *CDPPlaywrightTools {
	if dm.cdpTools == nil {
		dm.cdpTools = NewCDPPlaywrightTools(dm.cdpURL, dm.logger)
		dm.logger.Info(i18n.Tp("browser.driver.cdp.ready"))
	}
	return dm.cdpTools
}

// playwrightToolsLocked returns the cached Playwright tools instance (must hold mu).
func (dm *DriverManager) playwrightToolsLocked() (*PlaywrightNativeTools, error) {
	if dm.playwrightTools == nil {
		tools, err := NewPlaywrightNativeTools(PlaywrightNativeConfig{
			CDPURL: dm.cdpURL,
			Logger: dm.logger,
		})
		if err != nil {
			dm.logger.Error(i18n.T("browser.driver.playwright.failed", map[string]string{
				"error": err.Error(),
			}))
			return nil, fmt.Errorf("init playwright driver: %w", err)
		}
		dm.playwrightTools = tools
		dm.logger.Info(i18n.Tp("browser.driver.playwright.ready"))
	}
	return dm.playwrightTools, nil
}

// Kind returns the current driver kind.
func (dm *DriverManager) Kind() DriverKind {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.kind
}

// Close releases resources held by the driver.
func (dm *DriverManager) Close() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.playwrightTools != nil {
		if err := dm.playwrightTools.Close(); err != nil {
			dm.logger.Warn("close playwright driver", "err", err)
		}
		dm.playwrightTools = nil
	}
	// CDP tools are stateless per-connection, no cleanup needed.
	dm.cdpTools = nil
	return nil
}

// IsPlaywrightAvailable checks if playwright-go runtime is installed.
func IsPlaywrightAvailable() bool {
	// Check if the playwright CLI is accessible.
	// This is a lightweight check that doesn't launch a browser.
	_, err := findPlaywrightCLI()
	return err == nil
}
