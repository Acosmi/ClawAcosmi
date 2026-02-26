// Package services — Core Memory Prometheus 指标。
// 提供核心记忆自动编辑的可观测性。
package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// coreMemoryAutoEditsTotal 统计核心记忆自动编辑次数。
	// labels: section (persona/preferences/instructions), mode (replace/append), source (reflection/imagination)
	coreMemoryAutoEditsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "uhms",
			Name:      "core_memory_auto_edits_total",
			Help:      "Total number of automatic core memory edits by LLM (reflection/imagination).",
		},
		[]string{"section", "mode", "source"},
	)
)
