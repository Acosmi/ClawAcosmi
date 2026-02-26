package telegram

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
)

// ── IsRecoverableTelegramNetworkError ──

func TestNetwork_IsRecoverableTelegramNetworkError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		ctx  TelegramNetworkErrorContext
		want bool
	}{
		// ── Recoverable errors ──
		{
			name: "nil error is not recoverable",
			err:  nil,
			ctx:  NetworkCtxPolling,
			want: false,
		},
		{
			name: "net.OpError is recoverable",
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")},
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "net.DNSError is recoverable",
			err:  &net.DNSError{Err: "no such host", Name: "api.telegram.org"},
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "context.Canceled is recoverable",
			err:  context.Canceled,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "context.DeadlineExceeded is recoverable",
			err:  context.DeadlineExceeded,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "wrapped context.Canceled is recoverable",
			err:  fmt.Errorf("operation failed: %w", context.Canceled),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "wrapped context.DeadlineExceeded is recoverable",
			err:  fmt.Errorf("timed out: %w", context.DeadlineExceeded),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.ECONNRESET is recoverable",
			err:  syscall.ECONNRESET,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.ECONNRESET wrapped in fmt.Errorf is recoverable",
			err:  fmt.Errorf("read tcp: %w", syscall.ECONNRESET),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.ECONNREFUSED is recoverable",
			err:  syscall.ECONNREFUSED,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.EPIPE is recoverable",
			err:  syscall.EPIPE,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.ETIMEDOUT is recoverable",
			err:  syscall.ETIMEDOUT,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.ENETUNREACH is recoverable",
			err:  syscall.ENETUNREACH,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.EHOSTUNREACH is recoverable",
			err:  syscall.EHOSTUNREACH,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "syscall.ECONNABORTED is recoverable",
			err:  syscall.ECONNABORTED,
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "errors.Join with recoverable error nested is recoverable (BFS traversal)",
			err:  errors.Join(fmt.Errorf("wrapper"), syscall.ECONNRESET),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "errors.Join with multiple non-recoverable and one recoverable",
			err:  errors.Join(errors.New("a"), errors.New("b"), context.DeadlineExceeded),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "deeply nested recoverable via errors.Join",
			err:  errors.Join(errors.New("outer"), fmt.Errorf("mid: %w", &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET})),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "message match: 'connection reset' in polling context",
			err:  errors.New("connection reset by peer"),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "message match: 'timeout' in polling context",
			err:  errors.New("i/o timeout"),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "message match: 'no such host' in webhook context",
			err:  errors.New("dial tcp: lookup api.telegram.org: no such host"),
			ctx:  NetworkCtxWebhook,
			want: true,
		},
		{
			name: "message match: 'eof' in polling context",
			err:  errors.New("unexpected EOF"),
			ctx:  NetworkCtxPolling,
			want: true,
		},
		{
			name: "message match: 'fetch failed' in polling context",
			err:  errors.New("fetch failed"),
			ctx:  NetworkCtxPolling,
			want: true,
		},

		// ── Non-recoverable errors ──
		{
			name: "regular errors.New is not recoverable in send context",
			err:  errors.New("something went wrong"),
			ctx:  NetworkCtxSend,
			want: false,
		},
		{
			name: "regular errors.New is not recoverable even with message-like text in send context",
			err:  errors.New("connection reset by peer"),
			ctx:  NetworkCtxSend,
			want: false, // send context disables message matching
		},
		{
			name: "send context disables message matching for 'timeout'",
			err:  errors.New("request timeout occurred"),
			ctx:  NetworkCtxSend,
			want: false,
		},
		{
			name: "send context disables message matching for 'eof'",
			err:  errors.New("unexpected eof in response"),
			ctx:  NetworkCtxSend,
			want: false,
		},
		{
			name: "arbitrary error text is not recoverable",
			err:  errors.New("bad request: invalid token"),
			ctx:  NetworkCtxPolling,
			want: false,
		},
		{
			name: "errors.Join with only non-recoverable errors",
			err:  errors.Join(errors.New("one"), errors.New("two")),
			ctx:  NetworkCtxPolling,
			want: false,
		},

		// ── Send context still recovers typed errors ──
		{
			name: "send context still recovers context.Canceled",
			err:  context.Canceled,
			ctx:  NetworkCtxSend,
			want: true,
		},
		{
			name: "send context still recovers net.OpError",
			err:  &net.OpError{Op: "write", Net: "tcp", Err: syscall.EPIPE},
			ctx:  NetworkCtxSend,
			want: true,
		},
		{
			name: "send context still recovers syscall.ECONNRESET",
			err:  fmt.Errorf("write: %w", syscall.ECONNRESET),
			ctx:  NetworkCtxSend,
			want: true,
		},

		// ── Unknown context ──
		{
			name: "unknown context allows message matching",
			err:  errors.New("network error"),
			ctx:  NetworkCtxUnknown,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRecoverableTelegramNetworkError(tt.err, tt.ctx)
			if got != tt.want {
				t.Errorf("IsRecoverableTelegramNetworkError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ── collectErrorCandidates ──

func TestNetwork_collectErrorCandidates(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMin int // minimum number of candidates
	}{
		{
			name:    "single error yields one candidate",
			err:     errors.New("single"),
			wantMin: 1,
		},
		{
			name:    "wrapped error yields at least two candidates",
			err:     fmt.Errorf("outer: %w", errors.New("inner")),
			wantMin: 2,
		},
		{
			name:    "errors.Join yields all joined errors plus the join itself",
			err:     errors.Join(errors.New("a"), errors.New("b"), errors.New("c")),
			wantMin: 4, // join + a + b + c
		},
		{
			name:    "deeply wrapped chain",
			err:     fmt.Errorf("l1: %w", fmt.Errorf("l2: %w", fmt.Errorf("l3: %w", errors.New("leaf")))),
			wantMin: 4,
		},
		{
			name: "mixed wrap and join",
			err: fmt.Errorf("top: %w",
				errors.Join(
					errors.New("branch-a"),
					fmt.Errorf("branch-b: %w", errors.New("leaf-b")),
				)),
			wantMin: 5, // top, join, branch-a, branch-b, leaf-b
		},
		{
			name:    "nil error returns empty",
			err:     nil,
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var candidates []error
			if tt.err != nil {
				candidates = collectErrorCandidates(tt.err)
			}
			if len(candidates) < tt.wantMin {
				t.Errorf("got %d candidates, want at least %d", len(candidates), tt.wantMin)
				for i, c := range candidates {
					t.Logf("  candidate[%d]: %T %v", i, c, c)
				}
			}
		})
	}
}

func TestNetwork_collectErrorCandidates_NoDuplicates(t *testing.T) {
	shared := errors.New("shared")
	joined := errors.Join(shared, shared)
	candidates := collectErrorCandidates(joined)

	seen := make(map[string]int)
	for _, c := range candidates {
		key := fmt.Sprintf("%p", c)
		seen[key]++
	}
	for key, count := range seen {
		if count > 1 {
			t.Errorf("duplicate candidate found (ptr=%s) appears %d times", key, count)
		}
	}
}

// ── ResolveAutoSelectFamilyDecision ──

func TestNetwork_ResolveAutoSelectFamilyDecision(t *testing.T) {
	// Helper to set env and restore after test
	setEnv := func(t *testing.T, key, value string) {
		t.Helper()
		old, existed := os.LookupEnv(key)
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
		os.Setenv(key, value)
	}
	clearEnv := func(t *testing.T, key string) {
		t.Helper()
		old, existed := os.LookupEnv(key)
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
		os.Unsetenv(key)
	}

	tests := []struct {
		name       string
		enableEnv  string // value for ENABLE env var (empty = unset)
		disableEnv string // value for DISABLE env var (empty = unset)
		config     *TelegramNetworkConfig
		wantValue  *bool
		wantSource string
	}{
		{
			name:       "no env, no config: undecided",
			wantValue:  nil,
			wantSource: "",
		},
		{
			name:       "enable env set to '1'",
			enableEnv:  "1",
			wantValue:  boolPtr(true),
			wantSource: "env:" + TelegramEnableAutoSelectFamilyEnv,
		},
		{
			name:       "enable env set to 'true'",
			enableEnv:  "true",
			wantValue:  boolPtr(true),
			wantSource: "env:" + TelegramEnableAutoSelectFamilyEnv,
		},
		{
			name:       "enable env set to 'yes'",
			enableEnv:  "yes",
			wantValue:  boolPtr(true),
			wantSource: "env:" + TelegramEnableAutoSelectFamilyEnv,
		},
		{
			name:       "enable env set to 'on'",
			enableEnv:  "on",
			wantValue:  boolPtr(true),
			wantSource: "env:" + TelegramEnableAutoSelectFamilyEnv,
		},
		{
			name:       "disable env set to '1'",
			disableEnv: "1",
			wantValue:  boolPtr(false),
			wantSource: "env:" + TelegramDisableAutoSelectFamilyEnv,
		},
		{
			name:       "disable env set to 'true'",
			disableEnv: "true",
			wantValue:  boolPtr(false),
			wantSource: "env:" + TelegramDisableAutoSelectFamilyEnv,
		},
		{
			name:       "enable env takes priority over disable env",
			enableEnv:  "1",
			disableEnv: "1",
			wantValue:  boolPtr(true),
			wantSource: "env:" + TelegramEnableAutoSelectFamilyEnv,
		},
		{
			name:       "config value used when no env set",
			config:     &TelegramNetworkConfig{AutoSelectFamily: boolPtr(true)},
			wantValue:  boolPtr(true),
			wantSource: "config",
		},
		{
			name:       "config false value used when no env set",
			config:     &TelegramNetworkConfig{AutoSelectFamily: boolPtr(false)},
			wantValue:  boolPtr(false),
			wantSource: "config",
		},
		{
			name:       "env takes priority over config",
			enableEnv:  "1",
			config:     &TelegramNetworkConfig{AutoSelectFamily: boolPtr(false)},
			wantValue:  boolPtr(true),
			wantSource: "env:" + TelegramEnableAutoSelectFamilyEnv,
		},
		{
			name:       "config nil AutoSelectFamily falls through to undecided",
			config:     &TelegramNetworkConfig{AutoSelectFamily: nil},
			wantValue:  nil,
			wantSource: "",
		},
		{
			name:       "nil config falls through to undecided",
			config:     nil,
			wantValue:  nil,
			wantSource: "",
		},
		{
			name:       "non-truthy enable env falls through",
			enableEnv:  "0",
			config:     &TelegramNetworkConfig{AutoSelectFamily: boolPtr(true)},
			wantValue:  boolPtr(true),
			wantSource: "config",
		},
		{
			name:       "non-truthy disable env falls through",
			disableEnv: "no",
			config:     &TelegramNetworkConfig{AutoSelectFamily: boolPtr(false)},
			wantValue:  boolPtr(false),
			wantSource: "config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both env vars first
			clearEnv(t, TelegramEnableAutoSelectFamilyEnv)
			clearEnv(t, TelegramDisableAutoSelectFamilyEnv)

			if tt.enableEnv != "" {
				setEnv(t, TelegramEnableAutoSelectFamilyEnv, tt.enableEnv)
			}
			if tt.disableEnv != "" {
				setEnv(t, TelegramDisableAutoSelectFamilyEnv, tt.disableEnv)
			}

			decision := ResolveAutoSelectFamilyDecision(tt.config)

			if tt.wantValue == nil {
				if decision.Value != nil {
					t.Errorf("Value: got %v, want nil", *decision.Value)
				}
			} else {
				if decision.Value == nil {
					t.Fatalf("Value: got nil, want %v", *tt.wantValue)
				}
				if *decision.Value != *tt.wantValue {
					t.Errorf("Value: got %v, want %v", *decision.Value, *tt.wantValue)
				}
			}

			if decision.Source != tt.wantSource {
				t.Errorf("Source: got %q, want %q", decision.Source, tt.wantSource)
			}
		})
	}
}

// ── isTruthyEnvValue ──

func TestNetwork_isTruthyEnvValue(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"yes", true},
		{"on", true},
		{"TRUE", true},
		{"Yes", true},
		{"ON", true},
		{" true ", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got := isTruthyEnvValue(tt.input)
			if got != tt.want {
				t.Errorf("isTruthyEnvValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ── helpers ──

func boolPtr(b bool) *bool { return &b }
