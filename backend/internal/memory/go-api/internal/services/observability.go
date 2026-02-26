// Package services — Observability: in-process metrics + latency tracking.
// Mirrors Python services/observability.py — counters, histograms, percentile summaries.
package services

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// MemoryMetrics is a simple in-process metrics collector.
// Can later be replaced with Prometheus or OpenTelemetry exporters.
type MemoryMetrics struct {
	mu         sync.RWMutex
	counters   map[string]int64
	histograms map[string][]float64
	startTime  time.Time
}

// NewMemoryMetrics creates a new metrics collector.
func NewMemoryMetrics() *MemoryMetrics {
	return &MemoryMetrics{
		counters:   make(map[string]int64),
		histograms: make(map[string][]float64),
		startTime:  time.Now(),
	}
}

// Increment adds to a counter.
func (m *MemoryMetrics) Increment(name string, value int64, labels map[string]string) {
	key := makeKey(name, labels)
	m.mu.Lock()
	m.counters[key] += value
	m.mu.Unlock()
}

// Observe records a histogram observation.
func (m *MemoryMetrics) Observe(name string, value float64, labels map[string]string) {
	key := makeKey(name, labels)
	m.mu.Lock()
	m.histograms[key] = append(m.histograms[key], value)
	// Keep only last 1000 observations to avoid memory issues
	if len(m.histograms[key]) > 1000 {
		m.histograms[key] = m.histograms[key][len(m.histograms[key])-500:]
	}
	m.mu.Unlock()
}

// HistogramSummary holds percentile stats for a histogram.
type HistogramSummary struct {
	Count int     `json:"count"`
	Avg   float64 `json:"avg"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Max   float64 `json:"max"`
}

// MetricsSummary holds the full metrics snapshot.
type MetricsSummary struct {
	UptimeSeconds float64                     `json:"uptime_seconds"`
	Counters      map[string]int64            `json:"counters"`
	Histograms    map[string]HistogramSummary `json:"histograms"`
}

// GetSummary returns a snapshot of all metrics.
func (m *MemoryMetrics) GetSummary() MetricsSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := MetricsSummary{
		UptimeSeconds: time.Since(m.startTime).Seconds(),
		Counters:      make(map[string]int64, len(m.counters)),
		Histograms:    make(map[string]HistogramSummary, len(m.histograms)),
	}

	for k, v := range m.counters {
		summary.Counters[k] = v
	}

	for k, values := range m.histograms {
		if len(values) == 0 {
			continue
		}
		sorted := make([]float64, len(values))
		copy(sorted, values)
		sort.Float64s(sorted)

		n := len(sorted)
		total := 0.0
		for _, v := range sorted {
			total += v
		}

		summary.Histograms[k] = HistogramSummary{
			Count: n,
			Avg:   total / float64(n),
			P50:   sorted[n/2],
			P95:   sorted[int(float64(n)*0.95)],
			P99:   sorted[int(float64(n)*0.99)],
			Max:   sorted[n-1],
		}
	}

	return summary
}

func makeKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	// Sort for deterministic key
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	labelStr := ""
	for i, k := range keys {
		if i > 0 {
			labelStr += ","
		}
		labelStr += fmt.Sprintf("%s=%s", k, labels[k])
	}
	return fmt.Sprintf("%s{%s}", name, labelStr)
}

// --- Latency Tracker ---

// TrackLatency records operation start time, returns a finish func.
// Usage:
//
//	finish := metrics.TrackLatency("search")
//	defer finish()
func (m *MemoryMetrics) TrackLatency(operation string) func() {
	start := time.Now()
	return func() {
		elapsed := float64(time.Since(start).Milliseconds())
		m.Observe(operation+"_latency_ms", elapsed, nil)
		if elapsed > 500 {
			slog.Warn("Slow operation", "operation", operation, "latency_ms", elapsed)
		}
	}
}

// TrackSuccess increments the success counter for an operation.
func (m *MemoryMetrics) TrackSuccess(operation string) {
	m.Increment(operation+"_success", 1, nil)
}

// TrackError increments the error counter for an operation.
func (m *MemoryMetrics) TrackError(operation string) {
	m.Increment(operation+"_error", 1, nil)
}

// --- Global Singleton ---

var (
	metricsOnce   sync.Once
	globalMetrics *MemoryMetrics
)

// GetMetrics returns the global MemoryMetrics singleton.
func GetMetrics() *MemoryMetrics {
	metricsOnce.Do(func() {
		globalMetrics = NewMemoryMetrics()
	})
	return globalMetrics
}

// --- LLM Call Tracking ---

// TrackLLMCall records an LLM call's latency and token usage.
// Usage:
//
//	finish := metrics.TrackLLMCall("gpt-4o-mini")
//	defer finish(promptTokens, completionTokens, nil)
func (m *MemoryMetrics) TrackLLMCall(model string) func(promptTokens, completionTokens int, err error) {
	start := time.Now()
	return func(promptTokens, completionTokens int, err error) {
		elapsedMs := float64(time.Since(start).Milliseconds())
		labels := map[string]string{"model": model}

		m.Observe("llm_call_latency_ms", elapsedMs, labels)
		m.Increment("llm_prompt_tokens", int64(promptTokens), labels)
		m.Increment("llm_completion_tokens", int64(completionTokens), labels)
		m.Increment("llm_total_tokens", int64(promptTokens+completionTokens), labels)
		m.Increment("llm_calls_total", 1, labels)

		if err != nil {
			m.TrackError("llm_call")
			slog.Warn("LLM call failed", "model", model, "latency_ms", elapsedMs, "error", err)
		} else {
			m.TrackSuccess("llm_call")
			if elapsedMs > 3000 {
				slog.Warn("Slow LLM call", "model", model, "latency_ms", elapsedMs,
					"prompt_tokens", promptTokens, "completion_tokens", completionTokens)
			}
		}
	}
}

// TrackEmbeddingCall records an embedding call's latency and batch size.
func (m *MemoryMetrics) TrackEmbeddingCall(model string) func(batchSize int, err error) {
	start := time.Now()
	return func(batchSize int, err error) {
		elapsedMs := float64(time.Since(start).Milliseconds())
		labels := map[string]string{"model": model}

		m.Observe("embedding_call_latency_ms", elapsedMs, labels)
		m.Increment("embedding_calls_total", 1, labels)
		m.Increment("embedding_vectors_total", int64(batchSize), labels)

		if err != nil {
			m.TrackError("embedding_call")
		} else {
			m.TrackSuccess("embedding_call")
		}
	}
}

// TrackVectorStoreOp records a vector store operation's latency.
func (m *MemoryMetrics) TrackVectorStoreOp(operation string) func(err error) {
	start := time.Now()
	return func(err error) {
		elapsedMs := float64(time.Since(start).Milliseconds())
		labels := map[string]string{"op": operation}

		m.Observe("vectorstore_op_latency_ms", elapsedMs, labels)
		m.Increment("vectorstore_ops_total", 1, labels)

		if err != nil {
			m.TrackError("vectorstore_" + operation)
		} else {
			m.TrackSuccess("vectorstore_" + operation)
		}
	}
}
