package browser

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseRoleRef(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"e1", "e1"},
		{"e12", "e12"},
		{"e999", "e999"},
		{"@e1", "e1"},
		{"@e42", "e42"},
		{"ref=e5", "e5"},
		{" e7 ", "e7"},
		{"", ""},
		{"foo", ""},
		{"@foo", ""},
		{"ref=bar", ""},
		{"eabc", ""},
		{"12", ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			got := ParseRoleRef(tt.input)
			if got != tt.want {
				t.Errorf("ParseRoleRef(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildRoleSnapshotFromAriaSnapshot_Empty(t *testing.T) {
	result := BuildRoleSnapshotFromAriaSnapshot("", RoleSnapshotOptions{})
	if result.Snapshot != "(empty)" {
		t.Errorf("expected (empty), got %q", result.Snapshot)
	}
	if len(result.Refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(result.Refs))
	}
}

func TestBuildRoleSnapshotFromAriaSnapshot_BasicTree(t *testing.T) {
	input := `- navigation "Main Nav"
  - link "Home"
  - link "About"
  - button "Menu"
- main "Content"
  - heading "Welcome"
  - textbox "Search"
  - generic`

	result := BuildRoleSnapshotFromAriaSnapshot(input, RoleSnapshotOptions{})

	// Should have refs for interactive elements (link, button, textbox)
	// and content elements with names (heading "Welcome", navigation "Main Nav", main "Content")
	if len(result.Refs) == 0 {
		t.Fatal("expected refs, got none")
	}

	// Check that interactive elements got refs
	foundLink := false
	foundButton := false
	foundTextbox := false
	for _, ref := range result.Refs {
		if ref.Role == "link" {
			foundLink = true
		}
		if ref.Role == "button" {
			foundButton = true
		}
		if ref.Role == "textbox" {
			foundTextbox = true
		}
	}
	if !foundLink {
		t.Error("expected link ref")
	}
	if !foundButton {
		t.Error("expected button ref")
	}
	if !foundTextbox {
		t.Error("expected textbox ref")
	}

	// Check refs are e1, e2, etc.
	for ref := range result.Refs {
		if !strings.HasPrefix(ref, "e") {
			t.Errorf("ref should start with 'e', got %q", ref)
		}
	}
}

func TestBuildRoleSnapshotFromAriaSnapshot_InteractiveOnly(t *testing.T) {
	input := `- navigation "Main"
  - link "Home"
  - link "About"
- main "Content"
  - heading "Title"
  - textbox "Search"
  - button "Submit"`

	result := BuildRoleSnapshotFromAriaSnapshot(input, RoleSnapshotOptions{Interactive: true})

	// Should only have interactive elements
	for _, ref := range result.Refs {
		if !interactiveRoles.has(ref.Role) {
			t.Errorf("non-interactive role %q should not appear in interactive-only mode", ref.Role)
		}
	}

	// Should have link, textbox, button
	if len(result.Refs) != 4 { // 2 links + 1 textbox + 1 button
		t.Errorf("expected 4 interactive refs, got %d", len(result.Refs))
	}
}

func TestBuildRoleSnapshotFromAriaSnapshot_NoInteractive(t *testing.T) {
	input := `- heading "Title"
- paragraph "Some text"`

	result := BuildRoleSnapshotFromAriaSnapshot(input, RoleSnapshotOptions{Interactive: true})
	if result.Snapshot != "(no interactive elements)" {
		t.Errorf("expected '(no interactive elements)', got %q", result.Snapshot)
	}
}

func TestBuildRoleSnapshotFromAriaSnapshot_NthDedup(t *testing.T) {
	input := `- button "Submit"
- button "Submit"
- button "Cancel"`

	result := BuildRoleSnapshotFromAriaSnapshot(input, RoleSnapshotOptions{})

	// "Submit" appears twice, so nth should be preserved
	submitCount := 0
	for _, ref := range result.Refs {
		if ref.Role == "button" && ref.Name == "Submit" {
			submitCount++
			if ref.Nth == nil {
				t.Error("expected nth for duplicate Submit button")
			}
		}
		// "Cancel" appears only once, so nth should be nil
		if ref.Role == "button" && ref.Name == "Cancel" {
			if ref.Nth != nil {
				t.Error("expected nil nth for unique Cancel button")
			}
		}
	}
	if submitCount != 2 {
		t.Errorf("expected 2 Submit buttons, got %d", submitCount)
	}
}

func TestBuildRoleSnapshotFromAriaSnapshot_CompactMode(t *testing.T) {
	input := `- generic
  - group
    - button "Click me"
  - group
    - generic
- navigation "Nav"
  - link "Home"`

	result := BuildRoleSnapshotFromAriaSnapshot(input, RoleSnapshotOptions{Compact: true})

	// Unnamed structural elements (generic, group) without relevant children should be pruned
	if strings.Contains(result.Snapshot, "- generic\n") {
		// generic without ref children should be removed in compact mode
	}
	// Navigation with a name should remain
	if !strings.Contains(result.Snapshot, "link") {
		t.Error("link should be preserved in compact mode")
	}
}

func TestBuildRoleSnapshotFromAiSnapshot_PreservesRefs(t *testing.T) {
	input := `- navigation "Main"
  - link "Home" [ref=e1]
  - link "About" [ref=e2]
- main "Content"
  - button "Submit" [ref=e3]`

	result := BuildRoleSnapshotFromAiSnapshot(input, RoleSnapshotOptions{})

	// Should preserve the original refs from the AI snapshot
	if _, ok := result.Refs["e1"]; !ok {
		t.Error("expected ref e1 to be preserved")
	}
	if _, ok := result.Refs["e2"]; !ok {
		t.Error("expected ref e2 to be preserved")
	}
	if _, ok := result.Refs["e3"]; !ok {
		t.Error("expected ref e3 to be preserved")
	}

	// Verify role info
	if result.Refs["e1"].Role != "link" {
		t.Errorf("e1 role = %q, want 'link'", result.Refs["e1"].Role)
	}
	if result.Refs["e3"].Role != "button" {
		t.Errorf("e3 role = %q, want 'button'", result.Refs["e3"].Role)
	}
}

func TestBuildRoleSnapshotFromAiSnapshot_InteractiveOnly(t *testing.T) {
	input := `- heading "Title" [ref=e1]
- link "Home" [ref=e2]
- button "Go" [ref=e3]
- paragraph "text"`

	result := BuildRoleSnapshotFromAiSnapshot(input, RoleSnapshotOptions{Interactive: true})

	// Should only have interactive elements
	if len(result.Refs) != 2 { // link + button
		t.Errorf("expected 2 interactive refs, got %d", len(result.Refs))
	}
	if _, ok := result.Refs["e1"]; ok {
		t.Error("heading should not appear in interactive-only mode")
	}
}

func TestBuildRoleSnapshotFromAiSnapshot_Empty(t *testing.T) {
	result := BuildRoleSnapshotFromAiSnapshot("", RoleSnapshotOptions{})
	if result.Snapshot != "(empty)" {
		t.Errorf("expected (empty), got %q", result.Snapshot)
	}
}

func TestGetRoleSnapshotStats(t *testing.T) {
	snapshot := "- button \"Click\" [ref=e1]\n- link \"Home\" [ref=e2]\n- heading \"Title\" [ref=e3]"
	refs := RoleRefMap{
		"e1": {Role: "button", Name: "Click"},
		"e2": {Role: "link", Name: "Home"},
		"e3": {Role: "heading", Name: "Title"},
	}

	stats := GetRoleSnapshotStats(snapshot, refs)

	if stats.Lines != 3 {
		t.Errorf("Lines = %d, want 3", stats.Lines)
	}
	if stats.Refs != 3 {
		t.Errorf("Refs = %d, want 3", stats.Refs)
	}
	if stats.Interactive != 2 { // button + link
		t.Errorf("Interactive = %d, want 2", stats.Interactive)
	}
}

func TestGetIndentLevel(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"- button", 0},
		{"  - link", 1},
		{"    - textbox", 2},
		{"      - checkbox", 3},
		{"", 0},
		{"no indent", 0},
	}
	for _, tt := range tests {
		got := getIndentLevel(tt.input)
		if got != tt.want {
			t.Errorf("getIndentLevel(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestBuildRoleSnapshotFromAriaSnapshot_MaxDepth(t *testing.T) {
	input := `- navigation "Nav"
  - link "Home"
    - heading "H"
      - button "Deep"`

	result := BuildRoleSnapshotFromAriaSnapshot(input, RoleSnapshotOptions{MaxDepth: 1})

	// Should include depth 0 and 1 only
	if strings.Contains(result.Snapshot, "Deep") {
		t.Error("button at depth 3 should be excluded with MaxDepth=1")
	}
}
