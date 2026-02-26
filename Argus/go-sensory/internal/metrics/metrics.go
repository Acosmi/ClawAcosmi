// Package metrics implements Prometheus-compatible metrics for Argus.
//
// Uses Go's sync/atomic for lock-free counters and manual Prometheus
// text format generation — zero external dependencies.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Metric primitives
// ──────────────────────────────────────────────────────────────

// Counter is a monotonically increasing counter.
type Counter struct {
	name  string
	help  string
	value atomic.Int64
}

func (c *Counter) Inc()          { c.value.Add(1) }
func (c *Counter) Add(delta int) { c.value.Add(int64(delta)) }
func (c *Counter) Get() int64    { return c.value.Load() }

// LabeledCounter supports counters with label dimensions.
type LabeledCounter struct {
	name   string
	help   string
	labels []string
	mu     sync.RWMutex
	values map[string]*atomic.Int64
}

func (c *LabeledCounter) With(labels map[string]string) *atomic.Int64 {
	key := labelsToKey(c.labels, labels)
	c.mu.RLock()
	v, ok := c.values[key]
	c.mu.RUnlock()
	if ok {
		return v
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok = c.values[key]; ok {
		return v
	}
	v = &atomic.Int64{}
	c.values[key] = v
	return v
}

// Gauge is a numeric value that can go up and down.
type Gauge struct {
	name  string
	help  string
	value atomic.Int64
}

func (g *Gauge) Set(v int64) { g.value.Store(v) }
func (g *Gauge) Inc()        { g.value.Add(1) }
func (g *Gauge) Dec()        { g.value.Add(-1) }
func (g *Gauge) Get() int64  { return g.value.Load() }

// Histogram tracks distributions of observations.
type Histogram struct {
	name    string
	help    string
	buckets []float64
	mu      sync.Mutex
	counts  []atomic.Int64
	sum     atomic.Int64 // sum * 1e6 for microsecond precision
	count   atomic.Int64
}

func (h *Histogram) Observe(v float64) {
	h.count.Add(1)
	h.sum.Add(int64(v * 1e6))
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i].Add(1)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// ArgusMetrics — the global metrics singleton
// ──────────────────────────────────────────────────────────────

// ArgusMetrics holds all Argus system metrics.
type ArgusMetrics struct {
	// Frame Processing
	FramesTotal      Counter
	FrameReadLatency Histogram // microseconds
	FrameSizeBytes   Histogram

	// Keyframes
	KeyframesTotal     LabeledCounter // labels: trigger
	KeyframeBufferSize Gauge

	// VLM
	VLMCalls   LabeledCounter // labels: method, backend
	VLMLatency LabeledCounter // approximate: total ms by method+backend
	VLMErrors  LabeledCounter // labels: method, backend, error_type

	// ReAct Agent
	ReactTasksTotal LabeledCounter // labels: status
	ReactSteps      Histogram      // steps per task

	// API
	APIRequests LabeledCounter // labels: endpoint, method, status
	APILatency  Histogram      // seconds

	// Pipeline
	PipelineQueueSize Gauge
	PipelineDropped   Counter

	// Vector Store
	VectorStoreCount   Gauge
	VectorQueryLatency Histogram
}

// NewArgusMetrics creates a fresh metrics instance.
func NewArgusMetrics() *ArgusMetrics {
	return &ArgusMetrics{
		FramesTotal: Counter{name: "argus_frames_total", help: "Total frames read"},
		FrameReadLatency: Histogram{
			name:    "argus_frame_read_latency_us",
			help:    "SHM frame read latency in microseconds",
			buckets: []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000},
			counts:  make([]atomic.Int64, 8),
		},
		FrameSizeBytes: Histogram{
			name:    "argus_frame_size_bytes",
			help:    "Raw frame data size in bytes",
			buckets: []float64{1e5, 5e5, 1e6, 5e6, 1e7, 2e7, 5e7},
			counts:  make([]atomic.Int64, 7),
		},
		KeyframesTotal: LabeledCounter{
			name: "argus_keyframes_total", help: "Total keyframes extracted",
			labels: []string{"trigger"}, values: make(map[string]*atomic.Int64),
		},
		KeyframeBufferSize: Gauge{name: "argus_keyframe_buffer_size", help: "Current keyframe buffer size"},
		VLMCalls: LabeledCounter{
			name: "argus_vlm_calls_total", help: "Total VLM API calls",
			labels: []string{"method", "backend"}, values: make(map[string]*atomic.Int64),
		},
		VLMLatency: LabeledCounter{
			name: "argus_vlm_latency_ms_total", help: "VLM total latency in ms",
			labels: []string{"method", "backend"}, values: make(map[string]*atomic.Int64),
		},
		VLMErrors: LabeledCounter{
			name: "argus_vlm_errors_total", help: "VLM API errors",
			labels: []string{"method", "backend", "error_type"}, values: make(map[string]*atomic.Int64),
		},
		ReactTasksTotal: LabeledCounter{
			name: "argus_react_tasks_total", help: "Total ReAct tasks",
			labels: []string{"status"}, values: make(map[string]*atomic.Int64),
		},
		ReactSteps: Histogram{
			name: "argus_react_steps_per_task", help: "Steps per ReAct task",
			buckets: []float64{1, 2, 5, 10, 15, 20}, counts: make([]atomic.Int64, 6),
		},
		APIRequests: LabeledCounter{
			name: "argus_api_requests_total", help: "API requests",
			labels: []string{"endpoint", "method", "status"}, values: make(map[string]*atomic.Int64),
		},
		APILatency: Histogram{
			name: "argus_api_latency_seconds", help: "API request latency",
			buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0}, counts: make([]atomic.Int64, 8),
		},
		PipelineQueueSize: Gauge{name: "argus_pipeline_queue_size", help: "Async pipeline queue size"},
		PipelineDropped:   Counter{name: "argus_pipeline_dropped_total", help: "Frames dropped"},
		VectorStoreCount:  Gauge{name: "argus_vector_store_count", help: "Embeddings in ChromaDB"},
		VectorQueryLatency: Histogram{
			name: "argus_vector_query_latency_seconds", help: "ChromaDB query latency",
			buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0}, counts: make([]atomic.Int64, 6),
		},
	}
}

// MeasureVLM records a VLM call duration.
func (m *ArgusMetrics) MeasureVLM(method, backend string, duration time.Duration, err error) {
	m.VLMCalls.With(map[string]string{"method": method, "backend": backend}).Add(1)
	m.VLMLatency.With(map[string]string{"method": method, "backend": backend}).Add(duration.Milliseconds())
	if err != nil {
		m.VLMErrors.With(map[string]string{
			"method": method, "backend": backend, "error_type": fmt.Sprintf("%T", err),
		}).Add(1)
	}
}

// MeasureAPI records an API request.
func (m *ArgusMetrics) MeasureAPI(endpoint, method string, status int, duration time.Duration) {
	m.APIRequests.With(map[string]string{
		"endpoint": endpoint, "method": method, "status": fmt.Sprintf("%d", status),
	}).Add(1)
	m.APILatency.Observe(duration.Seconds())
}

// ──────────────────────────────────────────────────────────────
// Prometheus text format output
// ──────────────────────────────────────────────────────────────

// ServeHTTP handles GET /metrics in Prometheus exposition format.
func (m *ArgusMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprint(w, m.RenderText())
}

// RenderText generates Prometheus-format metrics output.
func (m *ArgusMetrics) RenderText() string {
	var b strings.Builder

	// Info
	b.WriteString("# HELP argus_system_info Argus system information\n")
	b.WriteString("# TYPE argus_system_info gauge\n")
	b.WriteString("argus_system_info{version=\"go-sensory/p5\",module=\"go-sensory\"} 1\n\n")

	// Simple counters
	writeCounter(&b, &m.FramesTotal)
	writeCounter(&b, &m.PipelineDropped)

	// Simple gauges
	writeGauge(&b, &m.KeyframeBufferSize)
	writeGauge(&b, &m.PipelineQueueSize)
	writeGauge(&b, &m.VectorStoreCount)

	// Histograms
	writeHistogram(&b, &m.FrameReadLatency)
	writeHistogram(&b, &m.FrameSizeBytes)
	writeHistogram(&b, &m.ReactSteps)
	writeHistogram(&b, &m.APILatency)
	writeHistogram(&b, &m.VectorQueryLatency)

	// Labeled counters
	writeLabeledCounter(&b, &m.KeyframesTotal)
	writeLabeledCounter(&b, &m.VLMCalls)
	writeLabeledCounter(&b, &m.VLMLatency)
	writeLabeledCounter(&b, &m.VLMErrors)
	writeLabeledCounter(&b, &m.ReactTasksTotal)
	writeLabeledCounter(&b, &m.APIRequests)

	return b.String()
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

func writeCounter(b *strings.Builder, c *Counter) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n\n",
		c.name, c.help, c.name, c.name, c.Get())
}

func writeGauge(b *strings.Builder, g *Gauge) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n\n",
		g.name, g.help, g.name, g.name, g.Get())
}

func writeHistogram(b *strings.Builder, h *Histogram) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
	cumCount := int64(0)
	for i, bucket := range h.buckets {
		cumCount += h.counts[i].Load()
		fmt.Fprintf(b, "%s_bucket{le=\"%g\"} %d\n", h.name, bucket, cumCount)
	}
	fmt.Fprintf(b, "%s_bucket{le=\"+Inf\"} %d\n", h.name, h.count.Load())
	fmt.Fprintf(b, "%s_sum %g\n", h.name, float64(h.sum.Load())/1e6)
	fmt.Fprintf(b, "%s_count %d\n\n", h.name, h.count.Load())
}

func writeLabeledCounter(b *strings.Builder, c *LabeledCounter) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n", c.name, c.help, c.name)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(c.values))
	for k := range c.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := c.values[key].Load()
		labels := keyToLabels(c.labels, key)
		fmt.Fprintf(b, "%s{%s} %d\n", c.name, labels, val)
	}
	b.WriteByte('\n')
}

func labelsToKey(names []string, labels map[string]string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = labels[n]
	}
	return strings.Join(parts, "\x00")
}

func keyToLabels(names []string, key string) string {
	parts := strings.Split(key, "\x00")
	pairs := make([]string, len(names))
	for i, n := range names {
		val := ""
		if i < len(parts) {
			val = parts[i]
		}
		pairs[i] = fmt.Sprintf("%s=%q", n, val)
	}
	return strings.Join(pairs, ",")
}

// ──────────────────────────────────────────────────────────────
// Singleton
// ──────────────────────────────────────────────────────────────

// Metrics is the global metrics singleton, initialized on first access.
var Metrics = NewArgusMetrics()
