// Package services — unit tests for the observability metrics system.
package services

import (
	"testing"
	"time"
)

// TestNewMemoryMetrics verifies metrics initialization.
func TestNewMemoryMetrics(t *testing.T) {
	m := NewMemoryMetrics()
	if m == nil {
		t.Fatal("NewMemoryMetrics should return non-nil")
	}

	summary := m.GetSummary()
	if summary.UptimeSeconds < 0 {
		t.Error("Uptime should be non-negative")
	}
	if len(summary.Counters) != 0 {
		t.Error("Initial counters should be empty")
	}
	if len(summary.Histograms) != 0 {
		t.Error("Initial histograms should be empty")
	}
}

// TestMetricsIncrement verifies counter increments.
func TestMetricsIncrement(t *testing.T) {
	m := NewMemoryMetrics()
	m.Increment("requests", 1, nil)
	m.Increment("requests", 2, nil)
	m.Increment("errors", 1, nil)

	summary := m.GetSummary()
	if summary.Counters["requests"] != 3 {
		t.Errorf("Expected requests=3, got %d", summary.Counters["requests"])
	}
	if summary.Counters["errors"] != 1 {
		t.Errorf("Expected errors=1, got %d", summary.Counters["errors"])
	}
}

// TestMetricsIncrementWithLabels verifies labeled counter separation.
func TestMetricsIncrementWithLabels(t *testing.T) {
	m := NewMemoryMetrics()
	m.Increment("llm_calls", 1, map[string]string{"model": "gpt-4"})
	m.Increment("llm_calls", 1, map[string]string{"model": "gpt-3.5"})
	m.Increment("llm_calls", 2, map[string]string{"model": "gpt-4"})

	summary := m.GetSummary()
	if summary.Counters["llm_calls{model=gpt-4}"] != 3 {
		t.Errorf("Expected gpt-4 calls=3, got %d", summary.Counters["llm_calls{model=gpt-4}"])
	}
	if summary.Counters["llm_calls{model=gpt-3.5}"] != 1 {
		t.Errorf("Expected gpt-3.5 calls=1, got %d", summary.Counters["llm_calls{model=gpt-3.5}"])
	}
}

// TestMetricsObserve verifies histogram observation.
func TestMetricsObserve(t *testing.T) {
	m := NewMemoryMetrics()
	m.Observe("latency", 100, nil)
	m.Observe("latency", 200, nil)
	m.Observe("latency", 300, nil)

	summary := m.GetSummary()
	hist := summary.Histograms["latency"]
	if hist.Count != 3 {
		t.Errorf("Expected count=3, got %d", hist.Count)
	}
	if hist.Avg != 200 {
		t.Errorf("Expected avg=200, got %f", hist.Avg)
	}
	if hist.Max != 300 {
		t.Errorf("Expected max=300, got %f", hist.Max)
	}
}

// TestTrackLatency verifies the latency tracking helper.
func TestTrackLatency(t *testing.T) {
	m := NewMemoryMetrics()
	finish := m.TrackLatency("search")
	time.Sleep(10 * time.Millisecond)
	finish()

	summary := m.GetSummary()
	hist, ok := summary.Histograms["search_latency_ms"]
	if !ok {
		t.Fatal("Expected search_latency_ms histogram")
	}
	if hist.Count != 1 {
		t.Errorf("Expected count=1, got %d", hist.Count)
	}
	if hist.Avg < 5 {
		t.Errorf("Latency should be >= 5ms, got %f", hist.Avg)
	}
}

// TestTrackSuccessError verifies success/error tracking.
func TestTrackSuccessError(t *testing.T) {
	m := NewMemoryMetrics()
	m.TrackSuccess("embedding_call")
	m.TrackSuccess("embedding_call")
	m.TrackError("embedding_call")

	summary := m.GetSummary()
	if summary.Counters["embedding_call_success"] != 2 {
		t.Errorf("Expected 2 successes, got %d", summary.Counters["embedding_call_success"])
	}
	if summary.Counters["embedding_call_error"] != 1 {
		t.Errorf("Expected 1 error, got %d", summary.Counters["embedding_call_error"])
	}
}

// TestTrackLLMCall verifies LLM call tracking.
func TestTrackLLMCall(t *testing.T) {
	m := NewMemoryMetrics()
	finish := m.TrackLLMCall("gpt-4o")
	time.Sleep(5 * time.Millisecond)
	finish(100, 50, nil) // 100 prompt tokens, 50 completion tokens, no error

	summary := m.GetSummary()
	labels := "llm_calls_total{model=gpt-4o}"
	if summary.Counters[labels] != 1 {
		t.Errorf("Expected 1 LLM call, got %d", summary.Counters[labels])
	}
	promptKey := "llm_prompt_tokens{model=gpt-4o}"
	if summary.Counters[promptKey] != 100 {
		t.Errorf("Expected 100 prompt tokens, got %d", summary.Counters[promptKey])
	}
	totalKey := "llm_total_tokens{model=gpt-4o}"
	if summary.Counters[totalKey] != 150 {
		t.Errorf("Expected 150 total tokens, got %d", summary.Counters[totalKey])
	}
}

// TestTrackVectorStoreOp verifies vector store operation tracking.
func TestTrackVectorStoreOp(t *testing.T) {
	m := NewMemoryMetrics()
	finish := m.TrackVectorStoreOp("upsert")
	finish(nil)

	summary := m.GetSummary()
	if summary.Counters["vectorstore_ops_total{op=upsert}"] != 1 {
		t.Errorf("Expected 1 upsert op")
	}
	if summary.Counters["vectorstore_upsert_success"] != 1 {
		t.Errorf("Expected 1 success")
	}
}

// TestGetMetricsSingleton verifies singleton behavior.
func TestGetMetricsSingleton(t *testing.T) {
	m1 := GetMetrics()
	m2 := GetMetrics()
	if m1 != m2 {
		t.Fatal("GetMetrics should return the same instance")
	}
}
