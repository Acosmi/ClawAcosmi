package browser

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Session manages a browser session tying together a Chrome instance
// and a CDP client. Ported from pw-session.ts.
type Session struct {
	mu      sync.RWMutex
	chrome  *ChromeInstance
	cdp     *CDPClient
	profile *ResolvedBrowserProfile
	config  *ResolvedBrowserConfig
	logger  *slog.Logger
	closed  bool
}

// SessionConfig holds parameters for creating a new Session.
type SessionConfig struct {
	Profile *ResolvedBrowserProfile
	Config  *ResolvedBrowserConfig
	Logger  *slog.Logger
}

// NewSession creates and starts a new browser session.
func NewSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	exe := ResolveBrowserExecutable(cfg.Config)
	if exe == nil {
		return nil, fmt.Errorf("no Chrome executable found")
	}

	chrome, err := StartChrome(ctx, ChromeStartConfig{
		Profile:    cfg.Profile,
		Executable: exe,
		Logger:     cfg.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("session: start chrome: %w", err)
	}

	// Wait for CDP to be ready.
	wsURL, err := chrome.WaitForCDP(30_000_000_000) // 30 seconds
	if err != nil {
		chrome.Stop()
		return nil, fmt.Errorf("session: CDP not ready: %w", err)
	}

	cdp := NewCDPClient(wsURL, cfg.Logger)

	return &Session{
		chrome:  chrome,
		cdp:     cdp,
		profile: cfg.Profile,
		config:  cfg.Config,
		logger:  cfg.Logger,
	}, nil
}

// CDP returns the CDP client for this session.
func (s *Session) CDP() *CDPClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cdp
}

// Profile returns the profile for this session.
func (s *Session) Profile() *ResolvedBrowserProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profile
}

// Navigate navigates to a URL.
func (s *Session) Navigate(ctx context.Context, url string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	return s.cdp.Navigate(ctx, url)
}

// Close closes the session and stops Chrome.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.chrome != nil {
		return s.chrome.Stop()
	}
	return nil
}
