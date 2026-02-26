package markdown

import (
	"strings"
	"testing"
)

func TestConvertMarkdownTables_Off(t *testing.T) {
	input := "| A | B |\n| --- | --- |\n| 1 | 2 |"
	got := ConvertMarkdownTables(input, TableModeOff)
	if got != input {
		t.Error("off mode should return input unchanged")
	}
}

func TestConvertMarkdownTables_Empty(t *testing.T) {
	got := ConvertMarkdownTables("", TableModeBullets)
	if got != "" {
		t.Error("empty input should return empty")
	}
}

func TestConvertMarkdownTables_NoTable(t *testing.T) {
	input := "# Hello\n\nSome paragraph text.\n\n- bullet one\n- bullet two"
	got := ConvertMarkdownTables(input, TableModeBullets)
	if got != input {
		t.Errorf("no-table input should be unchanged\ngot: %q", got)
	}
}

func TestConvertMarkdownTables_Bullets(t *testing.T) {
	input := "| Name | Age | City |\n| --- | --- | --- |\n| Alice | 30 | NYC |\n| Bob | 25 | LA |"
	got := ConvertMarkdownTables(input, TableModeBullets)

	// Multi-column: first column as label
	if !strings.Contains(got, "**Alice**") {
		t.Error("expected first column as bold label")
	}
	if !strings.Contains(got, "• Age: 30") {
		t.Error("expected bullet with header:value")
	}
	if !strings.Contains(got, "• City: NYC") {
		t.Error("expected city bullet")
	}
	if !strings.Contains(got, "**Bob**") {
		t.Error("expected Bob label")
	}
}

func TestConvertMarkdownTables_Code(t *testing.T) {
	input := "| A | B |\n| --- | --- |\n| short | longer value |"
	got := ConvertMarkdownTables(input, TableModeCode)

	// Should produce aligned table
	lines := strings.Split(got, "\n")
	// At least header + sep + 1 data row + trailing empty
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines, got %d", len(lines))
	}
	// Header and data rows should start with |
	if !strings.HasPrefix(lines[0], "|") {
		t.Error("header should start with |")
	}
	// Separator should have dashes
	if !strings.Contains(lines[1], "---") {
		t.Error("separator should contain ---")
	}
}

func TestConvertMarkdownTables_PreserveSurrounding(t *testing.T) {
	input := "Before table.\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\nAfter table."
	got := ConvertMarkdownTables(input, TableModeBullets)

	if !strings.Contains(got, "Before table.") {
		t.Error("should preserve text before table")
	}
	if !strings.Contains(got, "After table.") {
		t.Error("should preserve text after table")
	}
}

func TestParseTableRow_Valid(t *testing.T) {
	cells := parseTableRow("| foo | bar | baz |")
	if cells == nil {
		t.Fatal("expected non-nil cells")
	}
	if len(cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(cells))
	}
	if cells[0] != "foo" || cells[1] != "bar" || cells[2] != "baz" {
		t.Errorf("unexpected cells: %v", cells)
	}
}

func TestParseTableRow_Invalid(t *testing.T) {
	cells := parseTableRow("not a table row")
	if cells != nil {
		t.Error("expected nil for non-table row")
	}
}

func TestTableSepPattern(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		{"| --- | --- |", true},
		{"| :---: | ---: |", true},
		{"| - | - |", true},
		{"not a sep", false},
		{"| foo | bar |", false},
	}
	for _, tt := range tests {
		got := tableSepPattern.MatchString(tt.input)
		if got != tt.match {
			t.Errorf("tableSepPattern(%q) = %v, want %v", tt.input, got, tt.match)
		}
	}
}
