package gateway

import (
	"sync"
	"testing"
)

func TestAgentRunContextStore_RegisterGet(t *testing.T) {
	store := NewAgentRunContextStore()
	ctx := &AgentRunContext{RunID: "r1", SessionKey: "s1", VerboseLevel: "compact"}
	store.Register(ctx)
	got := store.Get("r1")
	if got == nil || got.SessionKey != "s1" {
		t.Fatal("expected registered context")
	}
	if store.Get("unknown") != nil {
		t.Fatal("expected nil for unknown runID")
	}
}

func TestAgentRunContextStore_NextSeq(t *testing.T) {
	store := NewAgentRunContextStore()
	store.Register(&AgentRunContext{RunID: "r1"})
	if seq := store.NextSeq("r1"); seq != 1 {
		t.Fatalf("expected seq 1, got %d", seq)
	}
	if seq := store.NextSeq("r1"); seq != 2 {
		t.Fatalf("expected seq 2, got %d", seq)
	}
	// 未知 runID 返回 0
	if seq := store.NextSeq("unknown"); seq != 0 {
		t.Fatalf("expected seq 0, got %d", seq)
	}
}

func TestAgentRunContextStore_Clear(t *testing.T) {
	store := NewAgentRunContextStore()
	store.Register(&AgentRunContext{RunID: "r1"})
	store.Clear("r1")
	if store.Get("r1") != nil {
		t.Fatal("expected nil after clear")
	}
}

func TestAgentRunContextStore_Emit(t *testing.T) {
	store := NewAgentRunContextStore()
	var received []AgentEvent
	store.AddListener(func(evt AgentEvent) {
		received = append(received, evt)
	})
	store.Emit(AgentEvent{RunID: "r1", Seq: 1, Type: "test"})
	if len(received) != 1 || received[0].Type != "test" {
		t.Fatal("expected listener to receive event")
	}
}

func TestAgentRunContextStore_ConcurrentAccess(t *testing.T) {
	store := NewAgentRunContextStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctx := &AgentRunContext{RunID: "r1"}
			store.Register(ctx)
			store.Get("r1")
			store.NextSeq("r1")
		}(i)
	}
	wg.Wait()
}

func TestAgentRunContextStore_Reset(t *testing.T) {
	store := NewAgentRunContextStore()
	store.Register(&AgentRunContext{RunID: "r1"})
	store.AddListener(func(evt AgentEvent) {})
	store.Reset()
	if store.Get("r1") != nil {
		t.Fatal("expected nil after reset")
	}
}
