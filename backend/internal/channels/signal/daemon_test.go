package signal

// daemon 测试 — 对齐 src/signal/daemon.test.ts

import (
	"testing"
)

func TestClassifySignalCliLogLine_InfoDebug(t *testing.T) {
	// 对齐 TS: "treats INFO/DEBUG as log (even if emitted on stderr)"
	tests := []struct {
		line string
		want SignalLogLevel
	}{
		{"INFO  DaemonCommand - Started", LogLevelInfo},
		{"DEBUG Something", LogLevelInfo},
		{"info line here", LogLevelInfo},
	}
	for _, tt := range tests {
		got := ClassifySignalCliLogLine(tt.line)
		if got == nil {
			t.Errorf("ClassifySignalCliLogLine(%q) = nil, want %q", tt.line, tt.want)
			continue
		}
		if *got != tt.want {
			t.Errorf("ClassifySignalCliLogLine(%q) = %q, want %q", tt.line, *got, tt.want)
		}
	}
}

func TestClassifySignalCliLogLine_WarnError(t *testing.T) {
	// 对齐 TS: "treats WARN/ERROR as error"
	// TS 将 WARN/WARNING 都归类为 "error"
	tests := []struct {
		line string
		want SignalLogLevel
	}{
		{"WARN  Something", LogLevelError},
		{"WARNING Something", LogLevelError},
		{"ERROR Something", LogLevelError},
		{"error: connection refused", LogLevelError},
	}
	for _, tt := range tests {
		got := ClassifySignalCliLogLine(tt.line)
		if got == nil {
			t.Errorf("ClassifySignalCliLogLine(%q) = nil, want %q", tt.line, tt.want)
			continue
		}
		if *got != tt.want {
			t.Errorf("ClassifySignalCliLogLine(%q) = %q, want %q", tt.line, *got, tt.want)
		}
	}
}

func TestClassifySignalCliLogLine_HeuristicErrors(t *testing.T) {
	// 对齐 TS: "treats failures without explicit severity as error"
	tests := []struct {
		line string
		want SignalLogLevel
	}{
		{`Exception in thread "main"`, LogLevelError},
		{"SEVERE: out of memory", LogLevelError},
		{"FAILED to initialize", LogLevelError},
	}
	for _, tt := range tests {
		got := ClassifySignalCliLogLine(tt.line)
		if got == nil {
			t.Errorf("ClassifySignalCliLogLine(%q) = nil, want %q", tt.line, tt.want)
			continue
		}
		if *got != tt.want {
			t.Errorf("ClassifySignalCliLogLine(%q) = %q, want %q", tt.line, *got, tt.want)
		}
	}
}

func TestClassifySignalCliLogLine_EmptyLine(t *testing.T) {
	// 对齐 TS: "returns null for empty lines"
	got := ClassifySignalCliLogLine("")
	if got != nil {
		t.Errorf("ClassifySignalCliLogLine(\"\") = %q, want nil", *got)
	}
	got2 := ClassifySignalCliLogLine("   ")
	if got2 != nil {
		t.Errorf("ClassifySignalCliLogLine(\"   \") = %q, want nil", *got2)
	}
}

func TestClassifySignalCliLogLine_MixedCase(t *testing.T) {
	// 大小写不敏感
	tests := []struct {
		line string
		want SignalLogLevel
	}{
		{"Error: something broke", LogLevelError},
		{"WARNING: disk full", LogLevelError},
		{"Some exception happened", LogLevelError},
	}
	for _, tt := range tests {
		got := ClassifySignalCliLogLine(tt.line)
		if got == nil {
			t.Errorf("ClassifySignalCliLogLine(%q) = nil, want %q", tt.line, tt.want)
			continue
		}
		if *got != tt.want {
			t.Errorf("ClassifySignalCliLogLine(%q) = %q, want %q", tt.line, *got, tt.want)
		}
	}
}
