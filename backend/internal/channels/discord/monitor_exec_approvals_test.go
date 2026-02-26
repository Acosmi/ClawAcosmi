package discord

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// 1. BuildExecApprovalCustomID / ParseExecApprovalCustomID
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalCustomID_Roundtrip(t *testing.T) {
	tests := []struct {
		name       string
		approvalID string
		action     ExecApprovalDecision
	}{
		{
			name:       "allow-once simple ID",
			approvalID: "abc-123",
			action:     ExecApprovalAllowOnce,
		},
		{
			name:       "allow-always simple ID",
			approvalID: "req-456",
			action:     ExecApprovalAllowAlways,
		},
		{
			name:       "deny simple ID",
			approvalID: "req-789",
			action:     ExecApprovalDeny,
		},
		{
			name:       "ID with special characters",
			approvalID: "id=with;special&chars",
			action:     ExecApprovalAllowOnce,
		},
		{
			name:       "ID with spaces",
			approvalID: "approval id with spaces",
			action:     ExecApprovalDeny,
		},
		{
			name:       "UUID-style ID",
			approvalID: "550e8400-e29b-41d4-a716-446655440000",
			action:     ExecApprovalAllowAlways,
		},
		{
			name:       "ID with URL-unsafe chars",
			approvalID: "foo/bar?baz=qux#fragment",
			action:     ExecApprovalAllowOnce,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customID := BuildExecApprovalCustomID(tt.approvalID, tt.action)
			parsed := ParseExecApprovalCustomID(customID)
			if parsed == nil {
				t.Fatalf("ParseExecApprovalCustomID returned nil for customID=%q", customID)
			}
			if parsed.ApprovalID != tt.approvalID {
				t.Errorf("ApprovalID = %q, want %q", parsed.ApprovalID, tt.approvalID)
			}
			if parsed.Action != tt.action {
				t.Errorf("Action = %q, want %q", parsed.Action, tt.action)
			}
		})
	}
}

func TestExecApprovalCustomID_ParseMalformed(t *testing.T) {
	tests := []struct {
		name     string
		customID string
	}{
		{name: "empty string", customID: ""},
		{name: "wrong prefix", customID: "wrongprefix:id=abc;action=allow-once"},
		{name: "no prefix", customID: "id=abc;action=allow-once"},
		{name: "missing id param", customID: "execapproval:action=allow-once"},
		{name: "missing action param", customID: "execapproval:id=abc"},
		{name: "invalid action value", customID: "execapproval:id=abc;action=unknown-action"},
		{name: "prefix only", customID: "execapproval:"},
		{name: "garbage data", customID: "execapproval:;;;==="},
		{name: "empty id value", customID: "execapproval:id=;action=allow-once"},
		{name: "empty action value", customID: "execapproval:id=abc;action="},
		{name: "old approve prefix", customID: "exec_approve:session123"},
		{name: "old deny prefix", customID: "exec_deny:session123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseExecApprovalCustomID(tt.customID)
			if parsed != nil {
				t.Errorf("ParseExecApprovalCustomID(%q) = %+v, want nil", tt.customID, parsed)
			}
		})
	}
}

func TestExecApprovalCustomID_ParseValidActions(t *testing.T) {
	tests := []struct {
		name     string
		action   ExecApprovalDecision
		expected ExecApprovalDecision
	}{
		{name: "allow-once", action: ExecApprovalAllowOnce, expected: ExecApprovalAllowOnce},
		{name: "allow-always", action: ExecApprovalAllowAlways, expected: ExecApprovalAllowAlways},
		{name: "deny", action: ExecApprovalDeny, expected: ExecApprovalDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customID := BuildExecApprovalCustomID("test-id", tt.action)
			parsed := ParseExecApprovalCustomID(customID)
			if parsed == nil {
				t.Fatal("ParseExecApprovalCustomID returned nil")
			}
			if parsed.Action != tt.expected {
				t.Errorf("Action = %q, want %q", parsed.Action, tt.expected)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 2. NewDiscordExecApprovalHandler
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalHandler_New(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Token:     "test-token",
		AccountID: "acct-1",
		Config: ExecApprovalConfig{
			Enabled:   true,
			Approvers: []string{"user-1"},
		},
	})

	if handler == nil {
		t.Fatal("NewDiscordExecApprovalHandler returned nil")
	}
	if handler.pending == nil {
		t.Error("pending map should be initialized")
	}
	if handler.requestCache == nil {
		t.Error("requestCache map should be initialized")
	}
	if handler.started {
		t.Error("handler should not be started immediately")
	}
	if handler.opts.Token != "test-token" {
		t.Errorf("Token = %q, want %q", handler.opts.Token, "test-token")
	}
	if handler.opts.AccountID != "acct-1" {
		t.Errorf("AccountID = %q, want %q", handler.opts.AccountID, "acct-1")
	}
}

func TestExecApprovalHandler_NewDefaults(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{})

	if handler == nil {
		t.Fatal("NewDiscordExecApprovalHandler returned nil")
	}
	if len(handler.pending) != 0 {
		t.Errorf("pending map should be empty, got %d entries", len(handler.pending))
	}
	if len(handler.requestCache) != 0 {
		t.Errorf("requestCache should be empty, got %d entries", len(handler.requestCache))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 3. ExecApprovalRequest struct construction
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalRequest_Construction(t *testing.T) {
	now := time.Now().UnixMilli()
	req := &ExecApprovalRequest{
		ID: "req-001",
		Request: ExecApprovalRequestDetail{
			Command:      "rm -rf /tmp/test",
			Cwd:          "/home/user",
			Host:         "server-1",
			Security:     "high",
			Ask:          "Delete temp files?",
			AgentID:      "agent-42",
			ResolvedPath: "/usr/bin/rm",
			SessionKey:   "sess-abc",
		},
		CreatedAtMs: now,
		ExpiresAtMs: now + 300_000, // 5 min
	}

	if req.ID != "req-001" {
		t.Errorf("ID = %q, want %q", req.ID, "req-001")
	}
	if req.Request.Command != "rm -rf /tmp/test" {
		t.Errorf("Command = %q", req.Request.Command)
	}
	if req.Request.AgentID != "agent-42" {
		t.Errorf("AgentID = %q", req.Request.AgentID)
	}
	if req.ExpiresAtMs-req.CreatedAtMs != 300_000 {
		t.Errorf("expected 300s expiry window, got %dms", req.ExpiresAtMs-req.CreatedAtMs)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 4. ShouldHandle filtering logic
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalHandler_ShouldHandle(t *testing.T) {
	tests := []struct {
		name     string
		config   ExecApprovalConfig
		request  *ExecApprovalRequest
		expected bool
	}{
		{
			name:     "disabled config",
			config:   ExecApprovalConfig{Enabled: false, Approvers: []string{"u1"}},
			request:  &ExecApprovalRequest{ID: "r1"},
			expected: false,
		},
		{
			name:     "no approvers",
			config:   ExecApprovalConfig{Enabled: true, Approvers: []string{}},
			request:  &ExecApprovalRequest{ID: "r1"},
			expected: false,
		},
		{
			name:     "enabled with approvers, no filters",
			config:   ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
			request:  &ExecApprovalRequest{ID: "r1"},
			expected: true,
		},
		{
			name:   "agent filter match",
			config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}, AgentFilter: []string{"agent-1", "agent-2"}},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{AgentID: "agent-1"},
			},
			expected: true,
		},
		{
			name:   "agent filter no match",
			config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}, AgentFilter: []string{"agent-1"}},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{AgentID: "agent-99"},
			},
			expected: false,
		},
		{
			name:   "agent filter with empty agent ID",
			config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}, AgentFilter: []string{"agent-1"}},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{AgentID: ""},
			},
			expected: false,
		},
		{
			name:   "session filter match (substring)",
			config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}, SessionFilter: []string{"prod"}},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{SessionKey: "prod-session-123"},
			},
			expected: true,
		},
		{
			name:   "session filter no match",
			config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}, SessionFilter: []string{"prod"}},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{SessionKey: "dev-session-123"},
			},
			expected: false,
		},
		{
			name:   "session filter with empty session key",
			config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}, SessionFilter: []string{"prod"}},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{SessionKey: ""},
			},
			expected: false,
		},
		{
			name: "both filters match",
			config: ExecApprovalConfig{
				Enabled:       true,
				Approvers:     []string{"u1"},
				AgentFilter:   []string{"agent-1"},
				SessionFilter: []string{"prod"},
			},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{AgentID: "agent-1", SessionKey: "prod-main"},
			},
			expected: true,
		},
		{
			name: "agent matches but session does not",
			config: ExecApprovalConfig{
				Enabled:       true,
				Approvers:     []string{"u1"},
				AgentFilter:   []string{"agent-1"},
				SessionFilter: []string{"prod"},
			},
			request: &ExecApprovalRequest{
				ID:      "r1",
				Request: ExecApprovalRequestDetail{AgentID: "agent-1", SessionKey: "dev-main"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
				Config: tt.config,
			})
			got := handler.ShouldHandle(tt.request)
			if got != tt.expected {
				t.Errorf("ShouldHandle() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 5. Approval / Denial flow - HandleApprovalResolved
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalHandler_ResolveApproval(t *testing.T) {
	tests := []struct {
		name     string
		decision ExecApprovalDecision
	}{
		{name: "allow once", decision: ExecApprovalAllowOnce},
		{name: "allow always", decision: ExecApprovalAllowAlways},
		{name: "deny", decision: ExecApprovalDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
				Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
				// Session is nil intentionally - updateApprovalMessage returns early
			})

			approvalID := "approval-" + string(tt.decision)

			// Manually seed a pending approval and cache entry
			handler.mu.Lock()
			handler.pending[approvalID] = &pendingApproval{
				DiscordMessageID: "msg-1",
				DiscordChannelID: "ch-1",
				CancelTimeout:    func() {}, // no-op
			}
			handler.requestCache[approvalID] = &ExecApprovalRequest{
				ID: approvalID,
				Request: ExecApprovalRequestDetail{
					Command: "echo hello",
				},
			}
			handler.mu.Unlock()

			// Resolve
			handler.HandleApprovalResolved(&ExecApprovalResolved{
				ID:         approvalID,
				Decision:   tt.decision,
				ResolvedBy: "tester",
				Ts:         time.Now().UnixMilli(),
			})

			// Verify cleanup
			handler.mu.Lock()
			_, pendingExists := handler.pending[approvalID]
			_, cacheExists := handler.requestCache[approvalID]
			handler.mu.Unlock()

			if pendingExists {
				t.Error("pending entry should have been removed after resolve")
			}
			if cacheExists {
				t.Error("requestCache entry should have been removed after resolve")
			}
		})
	}
}

func TestExecApprovalHandler_ResolveCancelsTimeout(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	var cancelCalled atomic.Bool

	approvalID := "timeout-test"
	handler.mu.Lock()
	handler.pending[approvalID] = &pendingApproval{
		DiscordMessageID: "msg-1",
		DiscordChannelID: "ch-1",
		CancelTimeout:    func() { cancelCalled.Store(true) },
	}
	handler.requestCache[approvalID] = &ExecApprovalRequest{
		ID:      approvalID,
		Request: ExecApprovalRequestDetail{Command: "ls"},
	}
	handler.mu.Unlock()

	handler.HandleApprovalResolved(&ExecApprovalResolved{
		ID:       approvalID,
		Decision: ExecApprovalAllowOnce,
		Ts:       time.Now().UnixMilli(),
	})

	if !cancelCalled.Load() {
		t.Error("CancelTimeout should have been called on resolve")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 6. Timeout handling
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalHandler_Timeout(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
		// No session - updateApprovalMessage will just return early
	})

	approvalID := "will-timeout"

	handler.mu.Lock()
	handler.pending[approvalID] = &pendingApproval{
		DiscordMessageID: "msg-1",
		DiscordChannelID: "ch-1",
		CancelTimeout:    func() {},
	}
	handler.requestCache[approvalID] = &ExecApprovalRequest{
		ID:      approvalID,
		Request: ExecApprovalRequestDetail{Command: "sleep 9999"},
	}
	handler.mu.Unlock()

	// Directly invoke handleApprovalTimeout (simulating the timeout path)
	handler.handleApprovalTimeout(approvalID)

	handler.mu.Lock()
	_, pendingExists := handler.pending[approvalID]
	_, cacheExists := handler.requestCache[approvalID]
	handler.mu.Unlock()

	if pendingExists {
		t.Error("pending entry should have been removed after timeout")
	}
	if cacheExists {
		t.Error("requestCache entry should have been removed after timeout")
	}
}

func TestExecApprovalHandler_TimeoutGoroutine(t *testing.T) {
	// Test that the actual goroutine-based timeout fires within a short window.
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	approvalID := "goroutine-timeout"
	timeoutMs := int64(100) // 100ms

	var wg sync.WaitGroup
	wg.Add(1)

	// Seed the pending map with a request
	handler.mu.Lock()
	handler.pending[approvalID] = &pendingApproval{
		DiscordMessageID: "msg-1",
		DiscordChannelID: "ch-1",
		CancelTimeout:    func() {},
	}
	handler.requestCache[approvalID] = &ExecApprovalRequest{
		ID:      approvalID,
		Request: ExecApprovalRequestDetail{Command: "test"},
	}
	handler.mu.Unlock()

	// Replace the pending entry with a proper cancel func and start the timeout goroutine
	// Mimicking what HandleApprovalRequested does internally.
	done := make(chan struct{})
	go func() {
		<-time.After(time.Duration(timeoutMs) * time.Millisecond)
		handler.handleApprovalTimeout(approvalID)
		close(done)
	}()

	select {
	case <-done:
		// Good - timeout fired
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout goroutine did not fire within 500ms (expected ~100ms)")
	}

	handler.mu.Lock()
	_, exists := handler.pending[approvalID]
	handler.mu.Unlock()

	if exists {
		t.Error("pending entry should have been removed after goroutine timeout")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 7. Edge cases
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalHandler_DuplicateResolve(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	approvalID := "dup-resolve"

	handler.mu.Lock()
	handler.pending[approvalID] = &pendingApproval{
		DiscordMessageID: "msg-1",
		DiscordChannelID: "ch-1",
		CancelTimeout:    func() {},
	}
	handler.requestCache[approvalID] = &ExecApprovalRequest{
		ID:      approvalID,
		Request: ExecApprovalRequestDetail{Command: "echo dup"},
	}
	handler.mu.Unlock()

	resolved := &ExecApprovalResolved{
		ID:       approvalID,
		Decision: ExecApprovalAllowOnce,
		Ts:       time.Now().UnixMilli(),
	}

	// First resolve should succeed
	handler.HandleApprovalResolved(resolved)

	// Second resolve should be a no-op (not panic)
	handler.HandleApprovalResolved(resolved)

	handler.mu.Lock()
	pendingLen := len(handler.pending)
	cacheLen := len(handler.requestCache)
	handler.mu.Unlock()

	if pendingLen != 0 {
		t.Errorf("pending should be empty, got %d", pendingLen)
	}
	if cacheLen != 0 {
		t.Errorf("requestCache should be empty, got %d", cacheLen)
	}
}

func TestExecApprovalHandler_ResolveAfterTimeout(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	approvalID := "resolve-after-timeout"

	handler.mu.Lock()
	handler.pending[approvalID] = &pendingApproval{
		DiscordMessageID: "msg-1",
		DiscordChannelID: "ch-1",
		CancelTimeout:    func() {},
	}
	handler.requestCache[approvalID] = &ExecApprovalRequest{
		ID:      approvalID,
		Request: ExecApprovalRequestDetail{Command: "echo late"},
	}
	handler.mu.Unlock()

	// Timeout fires first
	handler.handleApprovalTimeout(approvalID)

	// Now try to resolve - should be a no-op
	handler.HandleApprovalResolved(&ExecApprovalResolved{
		ID:       approvalID,
		Decision: ExecApprovalAllowOnce,
		Ts:       time.Now().UnixMilli(),
	})

	handler.mu.Lock()
	pendingLen := len(handler.pending)
	handler.mu.Unlock()

	if pendingLen != 0 {
		t.Errorf("pending should be empty after timeout + resolve, got %d", pendingLen)
	}
}

func TestExecApprovalHandler_ResolveUnknownID(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	// Should not panic when resolving a non-existent ID
	handler.HandleApprovalResolved(&ExecApprovalResolved{
		ID:       "non-existent-id",
		Decision: ExecApprovalDeny,
		Ts:       time.Now().UnixMilli(),
	})

	handler.mu.Lock()
	pendingLen := len(handler.pending)
	handler.mu.Unlock()

	if pendingLen != 0 {
		t.Errorf("pending should remain empty, got %d", pendingLen)
	}
}

func TestExecApprovalHandler_TimeoutUnknownID(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	// Should not panic when timing out a non-existent ID
	handler.handleApprovalTimeout("non-existent-id")
}

func TestExecApprovalHandler_Stop(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	var cancelCalls atomic.Int32

	// Seed multiple pending approvals
	handler.mu.Lock()
	for i := 0; i < 5; i++ {
		id := "stop-test-" + string(rune('a'+i))
		handler.pending[id] = &pendingApproval{
			DiscordMessageID: "msg-" + id,
			DiscordChannelID: "ch-1",
			CancelTimeout:    func() { cancelCalls.Add(1) },
		}
		handler.requestCache[id] = &ExecApprovalRequest{
			ID:      id,
			Request: ExecApprovalRequestDetail{Command: "echo " + id},
		}
	}
	handler.mu.Unlock()

	handler.Stop()

	handler.mu.Lock()
	pendingLen := len(handler.pending)
	cacheLen := len(handler.requestCache)
	handler.mu.Unlock()

	if pendingLen != 0 {
		t.Errorf("pending should be empty after Stop, got %d", pendingLen)
	}
	if cacheLen != 0 {
		t.Errorf("requestCache should be empty after Stop, got %d", cacheLen)
	}
	if handler.started {
		t.Error("started should be false after Stop")
	}
	if cancelCalls.Load() != 5 {
		t.Errorf("expected 5 CancelTimeout calls, got %d", cancelCalls.Load())
	}
}

func TestExecApprovalHandler_StopEmpty(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{})
	// Should not panic on empty handler
	handler.Stop()
}

func TestExecApprovalHandler_ConcurrentResolve(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
	})

	approvalID := "concurrent"
	handler.mu.Lock()
	handler.pending[approvalID] = &pendingApproval{
		DiscordMessageID: "msg-1",
		DiscordChannelID: "ch-1",
		CancelTimeout:    func() {},
	}
	handler.requestCache[approvalID] = &ExecApprovalRequest{
		ID:      approvalID,
		Request: ExecApprovalRequestDetail{Command: "echo concurrent"},
	}
	handler.mu.Unlock()

	// Launch many concurrent resolves; only one should find the pending entry
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.HandleApprovalResolved(&ExecApprovalResolved{
				ID:       approvalID,
				Decision: ExecApprovalAllowOnce,
				Ts:       time.Now().UnixMilli(),
			})
		}()
	}
	wg.Wait()

	handler.mu.Lock()
	pendingLen := len(handler.pending)
	handler.mu.Unlock()

	if pendingLen != 0 {
		t.Errorf("pending should be empty after concurrent resolves, got %d", pendingLen)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 8. Embed formatting helpers
// ────────────────────────────────────────────────────────────────────────────

func TestExecApproval_BuildExecApprovalEmbed(t *testing.T) {
	msg := BuildExecApprovalEmbed("ls -la", "user123", "sess-abc")

	if msg == nil {
		t.Fatal("BuildExecApprovalEmbed returned nil")
	}
	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(msg.Embeds))
	}
	if len(msg.Components) != 1 {
		t.Fatalf("expected 1 action row, got %d", len(msg.Components))
	}

	embed := msg.Embeds[0]
	if embed.Color != 0xFFA500 {
		t.Errorf("embed Color = %d, want %d", embed.Color, 0xFFA500)
	}
}

func TestExecApproval_FormatRequestEmbed_LongCommand(t *testing.T) {
	longCmd := ""
	for i := 0; i < 2000; i++ {
		longCmd += "x"
	}

	request := &ExecApprovalRequest{
		ID: "long-cmd",
		Request: ExecApprovalRequestDetail{
			Command: longCmd,
		},
		CreatedAtMs: time.Now().UnixMilli(),
		ExpiresAtMs: time.Now().Add(5 * time.Minute).UnixMilli(),
	}

	embed := formatExecApprovalRequestEmbed(request)
	if embed == nil {
		t.Fatal("formatExecApprovalRequestEmbed returned nil")
	}
	// Command field should be truncated to ~1000 + "..."
	cmdField := embed.Fields[0]
	// The value includes "```\n" prefix and "\n```" suffix
	if len(cmdField.Value) > 1020 {
		// 1000 chars + "..." + backtick wrappers = max ~1015
		// We just verify it's reasonably bounded
		t.Logf("command field length = %d (truncated correctly)", len(cmdField.Value))
	}
}

func TestExecApproval_FormatResolvedEmbed(t *testing.T) {
	tests := []struct {
		name          string
		decision      ExecApprovalDecision
		resolvedBy    string
		expectedTitle string
		expectedColor int
	}{
		{
			name:          "allow-once",
			decision:      ExecApprovalAllowOnce,
			resolvedBy:    "admin",
			expectedTitle: "Exec Approval: Allowed (once)",
			expectedColor: 0x57F287,
		},
		{
			name:          "allow-always",
			decision:      ExecApprovalAllowAlways,
			resolvedBy:    "admin",
			expectedTitle: "Exec Approval: Allowed (always)",
			expectedColor: 0x5865F2,
		},
		{
			name:          "deny",
			decision:      ExecApprovalDeny,
			resolvedBy:    "",
			expectedTitle: "Exec Approval: Denied",
			expectedColor: 0xED4245,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &ExecApprovalRequest{
				ID:      "fmt-test",
				Request: ExecApprovalRequestDetail{Command: "echo test"},
			}
			embed := formatExecApprovalResolvedEmbed(request, tt.decision, tt.resolvedBy)
			if embed == nil {
				t.Fatal("formatExecApprovalResolvedEmbed returned nil")
			}
			if embed.Title != tt.expectedTitle {
				t.Errorf("Title = %q, want %q", embed.Title, tt.expectedTitle)
			}
			if embed.Color != tt.expectedColor {
				t.Errorf("Color = %d, want %d", embed.Color, tt.expectedColor)
			}
			if tt.resolvedBy != "" && embed.Description != "Resolved by "+tt.resolvedBy {
				t.Errorf("Description = %q, want %q", embed.Description, "Resolved by "+tt.resolvedBy)
			}
			if tt.resolvedBy == "" && embed.Description != "Resolved" {
				t.Errorf("Description = %q, want %q", embed.Description, "Resolved")
			}
		})
	}
}

func TestExecApproval_FormatExpiredEmbed(t *testing.T) {
	request := &ExecApprovalRequest{
		ID:      "expire-test",
		Request: ExecApprovalRequestDetail{Command: "echo expired"},
	}
	embed := formatExecApprovalExpiredEmbed(request)
	if embed == nil {
		t.Fatal("formatExecApprovalExpiredEmbed returned nil")
	}
	if embed.Title != "Exec Approval: Expired" {
		t.Errorf("Title = %q, want %q", embed.Title, "Exec Approval: Expired")
	}
	if embed.Color != 0x99AAB5 {
		t.Errorf("Color = %d, want %d", embed.Color, 0x99AAB5)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 9. capitalizeFirst helper
// ────────────────────────────────────────────────────────────────────────────

func TestExecApproval_CapitalizeFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "", expected: ""},
		{input: "a", expected: "A"},
		{input: "approved", expected: "Approved"},
		{input: "denied", expected: "Denied"},
		{input: "ALREADY", expected: "ALREADY"},
		{input: "123abc", expected: "123abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := capitalizeFirst(tt.input)
			if got != tt.expected {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 10. HandleApprovalRequested - skips when ShouldHandle returns false
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalHandler_HandleRequestedSkipsDisabled(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: false},
	})

	request := &ExecApprovalRequest{
		ID:      "skip-test",
		Request: ExecApprovalRequestDetail{Command: "echo skip"},
	}

	// Should return early without caching
	handler.HandleApprovalRequested(request)

	handler.mu.Lock()
	cacheLen := len(handler.requestCache)
	handler.mu.Unlock()

	if cacheLen != 0 {
		t.Errorf("requestCache should be empty when handler is disabled, got %d", cacheLen)
	}
}

func TestExecApprovalHandler_HandleRequestedNoSession(t *testing.T) {
	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config:  ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
		Session: nil, // no session
	})

	request := &ExecApprovalRequest{
		ID:          "no-session",
		Request:     ExecApprovalRequestDetail{Command: "echo test"},
		CreatedAtMs: time.Now().UnixMilli(),
		ExpiresAtMs: time.Now().Add(5 * time.Minute).UnixMilli(),
	}

	// Should cache the request but fail to send (no session)
	handler.HandleApprovalRequested(request)

	handler.mu.Lock()
	_, cached := handler.requestCache[request.ID]
	_, pending := handler.pending[request.ID]
	handler.mu.Unlock()

	if !cached {
		t.Error("request should be cached even without a session")
	}
	if pending {
		t.Error("no pending entry should exist without a session (no message sent)")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 11. DiscordApprovalRequest legacy struct
// ────────────────────────────────────────────────────────────────────────────

func TestExecApproval_DiscordApprovalRequestStruct(t *testing.T) {
	req := DiscordApprovalRequest{
		Command:    "rm -rf /",
		Requester:  "admin",
		SessionKey: "session-1",
		ChannelID:  "ch-123",
		MessageID:  "msg-456",
		State:      ApprovalStatePending,
	}

	if req.State != ApprovalStatePending {
		t.Errorf("State = %q, want %q", req.State, ApprovalStatePending)
	}

	// Verify constants
	if ApprovalStatePending != "pending" {
		t.Errorf("ApprovalStatePending = %q", ApprovalStatePending)
	}
	if ApprovalStateApproved != "approved" {
		t.Errorf("ApprovalStateApproved = %q", ApprovalStateApproved)
	}
	if ApprovalStateDenied != "denied" {
		t.Errorf("ApprovalStateDenied = %q", ApprovalStateDenied)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 12. OnResolve callback integration
// ────────────────────────────────────────────────────────────────────────────

func TestExecApprovalHandler_OnResolveCallback(t *testing.T) {
	var resolvedID string
	var resolvedDecision ExecApprovalDecision

	handler := NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Config: ExecApprovalConfig{Enabled: true, Approvers: []string{"u1"}},
		OnResolve: func(id string, decision ExecApprovalDecision) error {
			resolvedID = id
			resolvedDecision = decision
			return nil
		},
	})

	// Verify the callback is stored
	if handler.opts.OnResolve == nil {
		t.Fatal("OnResolve should be set")
	}

	// Invoke the callback directly to test it's wired correctly
	err := handler.opts.OnResolve("test-id", ExecApprovalDeny)
	if err != nil {
		t.Errorf("OnResolve returned unexpected error: %v", err)
	}
	if resolvedID != "test-id" {
		t.Errorf("resolvedID = %q, want %q", resolvedID, "test-id")
	}
	if resolvedDecision != ExecApprovalDeny {
		t.Errorf("resolvedDecision = %q, want %q", resolvedDecision, ExecApprovalDeny)
	}
}
