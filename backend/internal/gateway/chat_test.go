package gateway

import (
	"testing"
	"time"
)

func TestChatRunRegistry_AddPeekShift(t *testing.T) {
	r := NewChatRunRegistry()
	r.Add("s1", ChatRunEntry{SessionKey: "main", ClientRunID: "r1"})
	r.Add("s1", ChatRunEntry{SessionKey: "main", ClientRunID: "r2"})

	e := r.Peek("s1")
	if e == nil || e.ClientRunID != "r1" {
		t.Errorf("Peek should return r1, got %v", e)
	}
	e = r.Shift("s1")
	if e == nil || e.ClientRunID != "r1" {
		t.Errorf("Shift should return r1")
	}
	e = r.Peek("s1")
	if e == nil || e.ClientRunID != "r2" {
		t.Errorf("after Shift, Peek should return r2")
	}
	e = r.Shift("s1")
	e = r.Shift("s1")
	if e != nil {
		t.Error("empty queue should return nil")
	}
}

func TestChatRunRegistry_Remove(t *testing.T) {
	r := NewChatRunRegistry()
	r.Add("s1", ChatRunEntry{SessionKey: "main", ClientRunID: "r1"})
	r.Add("s1", ChatRunEntry{SessionKey: "main", ClientRunID: "r2"})

	e := r.Remove("s1", "r2", "")
	if e == nil || e.ClientRunID != "r2" {
		t.Error("should remove r2")
	}
	e = r.Peek("s1")
	if e == nil || e.ClientRunID != "r1" {
		t.Error("r1 should remain")
	}
}

func TestChatRunRegistry_Clear(t *testing.T) {
	r := NewChatRunRegistry()
	r.Add("s1", ChatRunEntry{SessionKey: "k", ClientRunID: "r"})
	r.Clear()
	if r.Peek("s1") != nil {
		t.Error("Clear should empty registry")
	}
}

func TestChatRunState_Clear(t *testing.T) {
	s := NewChatRunState()
	s.Registry.Add("s1", ChatRunEntry{SessionKey: "k", ClientRunID: "r"})
	s.Buffers.Store("s1", "content")
	s.AbortedRuns.Store("r1", time.Now().UnixMilli())
	s.Clear()
	if s.Registry.Peek("s1") != nil {
		t.Error("Clear should reset registry")
	}
	if _, ok := s.Buffers.Load("s1"); ok {
		t.Error("Clear should reset buffers")
	}
}

func TestToolEventRecipientRegistry_AddGet(t *testing.T) {
	r := NewToolEventRecipientRegistry()
	r.Add("run1", "conn1")
	r.Add("run1", "conn2")

	ids := r.Get("run1")
	if len(ids) != 2 {
		t.Errorf("expected 2 connIDs, got %d", len(ids))
	}
	if _, ok := ids["conn1"]; !ok {
		t.Error("should have conn1")
	}
	if r.Get("nonexistent") != nil {
		t.Error("nonexistent should return nil")
	}
}

func TestToolEventRecipientRegistry_MarkFinal(t *testing.T) {
	r := NewToolEventRecipientRegistry()
	r.Add("run1", "conn1")
	r.MarkFinal("run1")

	ids := r.Get("run1")
	if ids == nil {
		t.Error("should still exist within grace period")
	}
}

func TestToolEventRecipientRegistry_EmptyInputs(t *testing.T) {
	r := NewToolEventRecipientRegistry()
	r.Add("", "conn1") // should be no-op
	r.Add("run1", "")  // should be no-op
	if r.Get("run1") != nil {
		t.Error("empty runID add should be no-op")
	}
}
