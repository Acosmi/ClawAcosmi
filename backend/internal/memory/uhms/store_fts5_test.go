package uhms

import "testing"

func TestEscapeFTS5Query(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Simple ASCII
		{"hello world", `"hello" "world"`},
		// CJK text
		{"用户配置了博查搜索", `"用户配置了博查搜索"`},
		// Mixed CJK and ASCII
		{"博查搜索 Bocha", `"博查搜索" "Bocha"`},
		// Special chars: slashes, parens
		{"it's a test", `"it's" "a" "test"`},
		{"path/to/file", `"path/to/file"`},
		{"function(arg)", `"function(arg)"`},
		// Embedded double quotes
		{`say "hello"`, `"say" """hello"""`},
		// Empty / whitespace
		{"", ""},
		{"   ", ""},
		// Single token
		{"单词", `"单词"`},
		// FTS5 operators should be quoted
		{"NOT OR AND", `"NOT" "OR" "AND"`},
	}
	for _, tt := range tests {
		got := escapeFTS5Query(tt.input)
		if got != tt.want {
			t.Errorf("escapeFTS5Query(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
