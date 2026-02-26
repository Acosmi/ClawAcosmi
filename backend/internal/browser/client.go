package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Client provides the public API for browser automation.
// It manages sessions and exposes high-level actions.
type Client struct {
	mu       sync.RWMutex
	config   *ResolvedBrowserConfig
	sessions map[string]*Session
	logger   *slog.Logger
}

// NewClient creates a browser automation client.
func NewClient(config *ResolvedBrowserConfig, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		config:   config,
		sessions: make(map[string]*Session),
		logger:   logger,
	}
}

// Launch starts a browser session for the given profile name.
func (c *Client) Launch(ctx context.Context, profileName string) (*Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.sessions[profileName]; ok {
		return existing, nil
	}

	profile, ok := c.config.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("unknown profile: %s", profileName)
	}

	session, err := NewSession(ctx, SessionConfig{
		Profile: profile,
		Config:  c.config,
		Logger:  c.logger,
	})
	if err != nil {
		return nil, err
	}

	c.sessions[profileName] = session
	return session, nil
}

// GetSession returns an existing session by profile name.
func (c *Client) GetSession(profileName string) (*Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.sessions[profileName]
	return s, ok
}

// Navigate navigates a profile's session to a URL.
func (c *Client) Navigate(ctx context.Context, profileName, url string) error {
	session, ok := c.GetSession(profileName)
	if !ok {
		return fmt.Errorf("no session for profile %q", profileName)
	}
	return session.Navigate(ctx, url)
}

// Evaluate runs JavaScript in a profile's session.
func (c *Client) Evaluate(ctx context.Context, profileName, expression string) (json.RawMessage, error) {
	session, ok := c.GetSession(profileName)
	if !ok {
		return nil, fmt.Errorf("no session for profile %q", profileName)
	}
	cdp := session.CDP()
	if cdp == nil {
		return nil, fmt.Errorf("session not connected")
	}
	return cdp.Evaluate(ctx, expression)
}

// Screenshot captures a screenshot from a profile's session.
func (c *Client) Screenshot(ctx context.Context, profileName string) ([]byte, error) {
	session, ok := c.GetSession(profileName)
	if !ok {
		return nil, fmt.Errorf("no session for profile %q", profileName)
	}
	cdp := session.CDP()
	if cdp == nil {
		return nil, fmt.Errorf("session not connected")
	}
	return cdp.CaptureScreenshot(ctx)
}

// CloseSession closes a specific session.
func (c *Client) CloseSession(profileName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	session, ok := c.sessions[profileName]
	if !ok {
		return nil
	}
	delete(c.sessions, profileName)
	return session.Close()
}

// Close closes all sessions and the client.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var firstErr error
	for name, session := range c.sessions {
		if err := session.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(c.sessions, name)
	}
	return firstErr
}
