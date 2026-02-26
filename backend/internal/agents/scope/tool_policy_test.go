package scope

import (
	"testing"
)

func TestNormalizeToolName(t *testing.T) {
	if NormalizeToolName("BASH") != "exec" {
		t.Errorf("BASH → %q, want exec", NormalizeToolName("BASH"))
	}
	if NormalizeToolName("apply-patch") != "apply_patch" {
		t.Errorf("apply-patch → %q, want apply_patch", NormalizeToolName("apply-patch"))
	}
	if NormalizeToolName("read") != "read" {
		t.Errorf("read → %q, want read", NormalizeToolName("read"))
	}
}

func TestExpandToolGroups(t *testing.T) {
	expanded := ExpandToolGroups([]string{"group:memory", "image"})
	if len(expanded) != 3 { // memory_search, memory_get, image
		t.Errorf("expanded = %v, want 3 items", expanded)
	}
}

func TestResolveToolProfilePolicy(t *testing.T) {
	// Minimal
	p := ResolveToolProfilePolicy("minimal")
	if p == nil || len(p.Allow) != 1 || p.Allow[0] != "session_status" {
		t.Errorf("minimal = %+v, want session_status", p)
	}

	// Full (returns nil — no restrictions)
	p = ResolveToolProfilePolicy("full")
	if p != nil {
		t.Errorf("full = %+v, want nil", p)
	}

	// Unknown
	p = ResolveToolProfilePolicy("unknown_profile")
	if p != nil {
		t.Errorf("unknown = %+v, want nil", p)
	}
}

func TestCreateSessionSlug(t *testing.T) {
	slug := CreateSessionSlug(nil)
	if slug == "" {
		t.Error("slug should not be empty")
	}
	// At minimum adjective-noun
	parts := 0
	for _, c := range slug {
		if c == '-' {
			parts++
		}
	}
	if parts < 1 {
		t.Errorf("slug %q should have at least one dash", slug)
	}

	// Collision avoidance
	taken := make(map[string]bool)
	for i := 0; i < 20; i++ {
		s := CreateSessionSlug(func(id string) bool { return taken[id] })
		taken[s] = true
	}
	if len(taken) != 20 {
		t.Errorf("expected 20 unique slugs, got %d", len(taken))
	}
}
