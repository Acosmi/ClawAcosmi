// Package cloud — connection monitor with health-check based offline detection.
// Provides automatic online/offline mode switching for the local proxy.
package cloud

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionState represents the current connectivity status.
type ConnectionState int32

const (
	StateOnline  ConnectionState = 0
	StateOffline ConnectionState = 1
)

func (s ConnectionState) String() string {
	if s == StateOnline {
		return "online"
	}
	return "offline"
}

// Monitor tracks cloud connectivity and provides automatic mode switching.
type Monitor struct {
	client    *Client
	state     atomic.Int32
	listeners []func(ConnectionState)
	mu        sync.Mutex

	// Config
	checkInterval time.Duration
	failThreshold int // consecutive failures before going offline
	failCount     int
}

// NewMonitor creates a connection monitor that periodically checks cloud health.
func NewMonitor(client *Client, checkInterval time.Duration) *Monitor {
	m := &Monitor{
		client:        client,
		checkInterval: checkInterval,
		failThreshold: 3,
	}
	m.state.Store(int32(StateOnline))
	return m
}

// IsOnline returns true if the cloud is reachable.
func (m *Monitor) IsOnline() bool {
	return ConnectionState(m.state.Load()) == StateOnline
}

// State returns the current connection state.
func (m *Monitor) State() ConnectionState {
	return ConnectionState(m.state.Load())
}

// OnStateChange registers a callback for state transitions.
func (m *Monitor) OnStateChange(fn func(ConnectionState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, fn)
}

// Start begins periodic health checks. Call cancel() to stop.
func (m *Monitor) Start(ctx context.Context) {
	// Initial check
	m.check(ctx)

	go func() {
		ticker := time.NewTicker(m.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.check(ctx)
			}
		}
	}()
}

func (m *Monitor) check(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.client.Health(checkCtx)

	m.mu.Lock()
	defer m.mu.Unlock()

	oldState := ConnectionState(m.state.Load())

	if err != nil {
		m.failCount++
		if m.failCount >= m.failThreshold && oldState == StateOnline {
			m.state.Store(int32(StateOffline))
			slog.Warn("Cloud connection lost — switching to offline mode",
				"consecutive_failures", m.failCount)
			m.notifyListeners(StateOffline)
		}
	} else {
		if oldState == StateOffline {
			slog.Info("Cloud connection restored — switching to online mode")
			m.notifyListeners(StateOnline)
		}
		m.failCount = 0
		m.state.Store(int32(StateOnline))
	}
}

func (m *Monitor) notifyListeners(state ConnectionState) {
	for _, fn := range m.listeners {
		go fn(state) // async to avoid blocking
	}
}
