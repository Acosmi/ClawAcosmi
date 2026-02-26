package pipeline

import (
	"testing"
)

// TestRustPIIFilter_ChineseIDCard verifies Chinese ID card detection.
func TestRustPIIFilter_ChineseIDCard(t *testing.T) {
	result, err := RustPIIFilter("身份证号码: 110101199001011234")
	if err != nil {
		t.Fatalf("RustPIIFilter failed: %v", err)
	}
	if !result.PIIDetected {
		t.Error("expected PII detected for Chinese ID card")
	}
	found := false
	for _, m := range result.Matches {
		if m.EntityType == "cn_id_card" {
			found = true
			t.Logf("Detected: %s → %s", m.Original, m.Masked)
		}
	}
	if !found {
		t.Error("expected cn_id_card match")
	}
}

// TestRustPIIFilter_ChinesePhone verifies Chinese phone number detection.
func TestRustPIIFilter_ChinesePhone(t *testing.T) {
	result, err := RustPIIFilter("联系电话 13812345678")
	if err != nil {
		t.Fatalf("RustPIIFilter failed: %v", err)
	}
	if !result.PIIDetected {
		t.Error("expected PII detected for Chinese phone")
	}
	found := false
	for _, m := range result.Matches {
		if m.EntityType == "cn_phone" {
			found = true
			t.Logf("Detected: %s → %s", m.Original, m.Masked)
		}
	}
	if !found {
		t.Error("expected cn_phone match")
	}
}

// TestRustPIIFilter_Email verifies email detection.
func TestRustPIIFilter_Email(t *testing.T) {
	result, err := RustPIIFilter("请联系 user@example.com 获取更多信息")
	if err != nil {
		t.Fatalf("RustPIIFilter failed: %v", err)
	}
	if !result.PIIDetected {
		t.Error("expected PII detected for email")
	}
	found := false
	for _, m := range result.Matches {
		if m.EntityType == "email" {
			found = true
			t.Logf("Detected: %s → %s", m.Original, m.Masked)
		}
	}
	if !found {
		t.Error("expected email match")
	}
}

// TestRustPIIFilter_IPAddress verifies IP address detection.
func TestRustPIIFilter_IPAddress(t *testing.T) {
	result, err := RustPIIFilter("服务器地址 192.168.1.100 端口8080")
	if err != nil {
		t.Fatalf("RustPIIFilter failed: %v", err)
	}
	if !result.PIIDetected {
		t.Error("expected PII detected for IP address")
	}
	found := false
	for _, m := range result.Matches {
		if m.EntityType == "ip_address" {
			found = true
			t.Logf("Detected: %s → %s", m.Original, m.Masked)
		}
	}
	if !found {
		t.Error("expected ip_address match")
	}
}

// TestRustPIIFilter_SafeText verifies no false positives on safe text.
func TestRustPIIFilter_SafeText(t *testing.T) {
	result, err := RustPIIFilter("这是一段安全的文本，没有任何敏感信息。")
	if err != nil {
		t.Fatalf("RustPIIFilter failed: %v", err)
	}
	if result.PIIDetected {
		t.Errorf("expected no PII in safe text, but found %d matches", len(result.Matches))
	}
}

// TestRustPIIIsSafe verifies the quick-check function.
func TestRustPIIIsSafe(t *testing.T) {
	if !RustPIIIsSafe("Hello world, nothing sensitive here.") {
		t.Error("expected safe text to be safe")
	}
	if RustPIIIsSafe("我的电话是 13912345678") {
		t.Error("expected text with phone number to be unsafe")
	}
}

// TestRustPIIFilter_MultiplePII verifies detection of multiple PII types.
func TestRustPIIFilter_MultiplePII(t *testing.T) {
	text := "姓名张三，电话13812345678，邮箱zhang@example.com，IP:10.0.0.1"
	result, err := RustPIIFilter(text)
	if err != nil {
		t.Fatalf("RustPIIFilter failed: %v", err)
	}
	if len(result.Matches) < 3 {
		t.Errorf("expected at least 3 PII matches, got %d", len(result.Matches))
	}
	t.Logf("Original: %s", result.OriginalText)
	t.Logf("Filtered: %s", result.FilteredText)
	for _, m := range result.Matches {
		t.Logf("  [%s] %s → %s", m.EntityType, m.Original, m.Masked)
	}
}

// ===== Benchmarks =====

// BenchmarkRustPIIFilter benchmarks Rust PII filter.
func BenchmarkRustPIIFilter(b *testing.B) {
	text := "姓名张三，电话13812345678，邮箱zhang@example.com，IP:10.0.0.1，身份证110101199001011234"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RustPIIFilter(text)
	}
}

// BenchmarkGoPIIFilter benchmarks Go PII filter.
func BenchmarkGoPIIFilter(b *testing.B) {
	f := NewPIIFilter()
	text := "姓名张三，电话13812345678，邮箱zhang@example.com，IP:10.0.0.1，身份证110101199001011234"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.Filter(text)
	}
}
