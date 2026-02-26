// Package pipeline — monitor.go provides periodic VLM screen monitoring.
//
// The Monitor captures the latest frame at a configurable interval,
// downscales it via imaging.Scaler, sends to VLM for analysis,
// and stores the latest results for querying via API.
package pipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"Argus-compound/go-sensory/internal/capture"
	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/vlm"
)

// DefaultMonitorInterval is the default pause between VLM analysis calls.
const DefaultMonitorInterval = 30 * time.Second

// MaxObservations is the number of recent observations kept in memory.
const MaxObservations = 50

// MonitorObservation is the result of a single VLM monitoring check.
type MonitorObservation struct {
	Timestamp   time.Time `json:"timestamp"`
	Summary     string    `json:"summary"`
	ImageSizeKB int       `json:"image_size_kb"`
	LatencyMs   int64     `json:"latency_ms"`
	Error       string    `json:"error,omitempty"`
}

// MonitorStatus holds the current state of the monitor.
type MonitorStatus struct {
	Running      bool                 `json:"running"`
	IntervalSec  int                  `json:"interval_sec"`
	TotalChecks  int64                `json:"total_checks"`
	TotalErrors  int64                `json:"total_errors"`
	Observations []MonitorObservation `json:"observations"`
}

// Monitor runs periodic VLM analysis on screen captures.
//
// Architecture:
//   - Grabs latest frame from Capturer (no extra screenshots)
//   - Downscales via Scaler.ForVLM (~87% token savings)
//   - Sends to VLM with a monitoring-specific prompt
//   - Stores last N observations for API queries
type Monitor struct {
	capturer capture.Capturer
	scaler   *imaging.Scaler
	provider vlm.Provider

	interval atomic.Int64 // nanoseconds

	mu           sync.RWMutex
	observations []MonitorObservation
	totalChecks  atomic.Int64
	totalErrors  atomic.Int64

	cancel  context.CancelFunc
	done    chan struct{}
	running atomic.Bool
}

// NewMonitor creates a VLM screen monitor with configurable interval.
// Reads VLM_MONITOR_INTERVAL_SEC env var (default: 30).
func NewMonitor(
	capturer capture.Capturer,
	scaler *imaging.Scaler,
	provider vlm.Provider,
) *Monitor {
	m := &Monitor{
		capturer:     capturer,
		scaler:       scaler,
		provider:     provider,
		observations: make([]MonitorObservation, 0, MaxObservations),
		done:         make(chan struct{}),
	}

	interval := DefaultMonitorInterval
	if v := os.Getenv("VLM_MONITOR_INTERVAL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}
	m.interval.Store(int64(interval))

	log.Printf("[Monitor] VLM screen monitor initialized (interval=%v)", interval)
	return m
}

// Start begins the monitoring loop in a background goroutine.
func (m *Monitor) Start() {
	if m.running.Swap(true) {
		return // already running
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	go m.loop(ctx)
	log.Printf("[Monitor] Started periodic VLM monitoring")
}

// Stop gracefully shuts down the monitoring loop.
func (m *Monitor) Stop() {
	if !m.running.Swap(false) {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
	<-m.done
	log.Printf("[Monitor] Stopped")
}

// SetInterval updates the monitoring interval (thread-safe, takes effect next cycle).
func (m *Monitor) SetInterval(d time.Duration) {
	if d < time.Second {
		d = time.Second
	}
	m.interval.Store(int64(d))
	log.Printf("[Monitor] Interval updated to %v", d)
}

// Status returns the current monitor status and recent observations.
func (m *Monitor) Status() MonitorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	obs := make([]MonitorObservation, len(m.observations))
	copy(obs, m.observations)

	return MonitorStatus{
		Running:      m.running.Load(),
		IntervalSec:  int(time.Duration(m.interval.Load()) / time.Second),
		TotalChecks:  m.totalChecks.Load(),
		TotalErrors:  m.totalErrors.Load(),
		Observations: obs,
	}
}

// loop is the main monitoring goroutine.
func (m *Monitor) loop(ctx context.Context) {
	defer close(m.done)

	for {
		interval := time.Duration(m.interval.Load())
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			m.checkOnce(ctx)
		}
	}
}

// checkOnce performs a single VLM monitoring check.
func (m *Monitor) checkOnce(ctx context.Context) {
	m.totalChecks.Add(1)
	start := time.Now()

	obs := MonitorObservation{
		Timestamp: start,
	}

	// Grab latest frame
	frame := m.capturer.LatestFrame()
	if frame == nil {
		obs.Error = "no frame available"
		m.totalErrors.Add(1)
		m.addObservation(obs)
		return
	}

	// Downscale for VLM
	jpegData, err := m.scaler.ForVLM(frame)
	if err != nil {
		obs.Error = fmt.Sprintf("image encode: %v", err)
		m.totalErrors.Add(1)
		m.addObservation(obs)
		return
	}
	obs.ImageSizeKB = len(jpegData) / 1024

	// Call VLM
	b64 := base64.StdEncoding.EncodeToString(jpegData)
	chatReq := vlm.ChatRequest{
		Messages: []vlm.Message{
			{
				Role: "user",
				Content: []vlm.ContentPart{
					{Type: "text", Text: monitorPrompt},
					{
						Type: "image_url",
						ImageURL: &vlm.ImageURL{
							URL: "data:image/jpeg;base64," + b64,
						},
					},
				},
			},
		},
		MaxTokens: 512,
	}

	temp := 0.1
	chatReq.Temperature = &temp

	// Use a per-check timeout (half the interval, min 10s, max 60s)
	checkTimeout := time.Duration(m.interval.Load()) / 2
	if checkTimeout < 10*time.Second {
		checkTimeout = 10 * time.Second
	}
	if checkTimeout > 60*time.Second {
		checkTimeout = 60 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	resp, err := m.provider.ChatCompletion(checkCtx, chatReq)
	obs.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		obs.Error = fmt.Sprintf("VLM call: %v", err)
		m.totalErrors.Add(1)
		m.addObservation(obs)
		return
	}

	if len(resp.Choices) > 0 {
		if text, ok := resp.Choices[0].Message.Content.(string); ok {
			obs.Summary = text
		}
	}

	m.addObservation(obs)
	log.Printf("[Monitor] Check #%d: %dKB image, %dms latency, summary=%q",
		m.totalChecks.Load(), obs.ImageSizeKB, obs.LatencyMs,
		truncate(obs.Summary, 80))
}

// addObservation appends an observation, evicting oldest if at capacity.
func (m *Monitor) addObservation(obs MonitorObservation) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.observations) >= MaxObservations {
		// Shift left: drop oldest
		copy(m.observations, m.observations[1:])
		m.observations = m.observations[:MaxObservations-1]
	}
	m.observations = append(m.observations, obs)
}

// truncate returns the first n chars of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// monitorPrompt is the system prompt for periodic monitoring checks.
const monitorPrompt = `You are a desktop monitoring assistant. Briefly describe what is currently visible on screen in 1-2 sentences. Focus on:
- Which application is active
- What the user appears to be doing
- Any notable UI state (dialogs, errors, loading screens)

Be concise. Example: "VS Code is open with a Go file. The terminal panel shows a successful build output."`
