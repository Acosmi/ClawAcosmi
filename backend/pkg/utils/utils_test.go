package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id := GenerateID(16)
	if len(id) != 16 {
		t.Errorf("expected length 16, got %d", len(id))
	}
	// 应该是十六进制
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("unexpected character %c in hex ID", c)
		}
	}
	// 两次生成应不同
	id2 := GenerateID(16)
	if id == id2 {
		t.Error("two generated IDs should differ")
	}
}

func TestClampNumber(t *testing.T) {
	tests := []struct {
		value, min, max, want float64
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 0, 0},
	}
	for _, tt := range tests {
		got := ClampNumber(tt.value, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("ClampNumber(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestClampInt(t *testing.T) {
	if ClampInt(5, 0, 10) != 5 {
		t.Error("5 in [0,10] should be 5")
	}
	if ClampInt(-1, 0, 10) != 0 {
		t.Error("-1 in [0,10] should be 0")
	}
	if ClampInt(15, 0, 10) != 10 {
		t.Error("15 in [0,10] should be 10")
	}
}

func TestTruncate(t *testing.T) {
	if Truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if Truncate("hello world", 8) != "hello..." {
		t.Errorf("got %q", Truncate("hello world", 8))
	}
	if Truncate("hello", 3) != "hel" {
		t.Errorf("got %q", Truncate("hello", 3))
	}
}

func TestNormalizeE164(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"+1234567890", "+1234567890"},
		{"1234567890", "+1234567890"},
		{"+1 (234) 567-890", "+1234567890"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeE164(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeE164(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsPortAvailable(t *testing.T) {
	// 高端口号通常可用
	// 注意：此测试可能在某些环境下失败
	if !IsPortAvailable(0) {
		t.Error("port 0 should be available (OS assigns random)")
	}
}

func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestNormalizePath(t *testing.T) {
	// 相对路径应被解析为绝对路径
	p := NormalizePath("./foo")
	if !filepath.IsAbs(p) {
		t.Errorf("expected absolute path, got %q", p)
	}

	// ~ 展开
	home, _ := os.UserHomeDir()
	if home != "" {
		p = NormalizePath("~/test")
		if !strings.HasPrefix(p, home) {
			t.Errorf("~ not expanded: %q", p)
		}
	}
}

func TestShortenHomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	if ShortenHomePath(home) != "~" {
		t.Errorf("home dir should become ~")
	}
	if ShortenHomePath(home+"/foo") != "~/foo" {
		t.Errorf("home/foo should become ~/foo")
	}
	if ShortenHomePath("/other/path") != "/other/path" {
		t.Error("non-home path should not change")
	}
}

func TestParseBooleanValue(t *testing.T) {
	tests := []struct {
		input      string
		wantResult bool
		wantOk     bool
	}{
		{"true", true, true},
		{"TRUE", true, true},
		{"1", true, true},
		{"yes", true, true},
		{"on", true, true},
		{"false", false, true},
		{"0", false, true},
		{"no", false, true},
		{"off", false, true},
		{"maybe", false, false},
		{"", false, false},
		{"  true  ", true, true},
	}
	for _, tt := range tests {
		result, ok := ParseBooleanValue(tt.input)
		if result != tt.wantResult || ok != tt.wantOk {
			t.Errorf("ParseBooleanValue(%q) = (%v, %v), want (%v, %v)",
				tt.input, result, ok, tt.wantResult, tt.wantOk)
		}
	}
}

func TestIsTruthy(t *testing.T) {
	if !IsTruthy("true") {
		t.Error("'true' should be truthy")
	}
	if IsTruthy("false") {
		t.Error("'false' should not be truthy")
	}
	if IsTruthy("") {
		t.Error("empty should not be truthy")
	}
}

func TestSplitShellArgs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{`hello "world foo"`, []string{"hello", "world foo"}},
		{`hello 'world foo'`, []string{"hello", "world foo"}},
		{`hello\ world`, []string{"hello world"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := SplitShellArgs(tt.input)
		if tt.want == nil && got == nil {
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("SplitShellArgs(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("SplitShellArgs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}

	// 未闭合引号应返回 nil
	if SplitShellArgs(`hello "world`) != nil {
		t.Error("unclosed quote should return nil")
	}
}

func TestRandomInt(t *testing.T) {
	for i := 0; i < 100; i++ {
		n := RandomInt(100)
		if n < 0 || n >= 100 {
			t.Errorf("RandomInt(100) = %d, out of range", n)
		}
	}
}
