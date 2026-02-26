package datetime

import (
	"testing"
	"time"
)

func TestResolveUserTimezone(t *testing.T) {
	// 有效值
	tz := ResolveUserTimezone("Asia/Shanghai")
	if tz != "Asia/Shanghai" {
		t.Errorf("valid = %q, want Asia/Shanghai", tz)
	}

	// 无效值 → 回退到系统
	tz = ResolveUserTimezone("Invalid/Zone")
	if tz == "Invalid/Zone" {
		t.Error("should fallback from invalid timezone")
	}
	if tz == "" {
		t.Error("timezone should not be empty")
	}
}

func TestNormalizeTimestamp_Number(t *testing.T) {
	// 秒级 → 毫秒
	result := NormalizeTimestamp(float64(1700000000))
	if result == nil {
		t.Fatal("nil for seconds timestamp")
	}
	if result.TimestampMs != 1700000000000 {
		t.Errorf("seconds: ms = %d, want 1700000000000", result.TimestampMs)
	}

	// 毫秒级
	result = NormalizeTimestamp(float64(1700000000000))
	if result == nil {
		t.Fatal("nil for ms timestamp")
	}
	if result.TimestampMs != 1700000000000 {
		t.Errorf("ms: ms = %d, want 1700000000000", result.TimestampMs)
	}
}

func TestNormalizeTimestamp_String(t *testing.T) {
	result := NormalizeTimestamp("2024-01-01T00:00:00Z")
	if result == nil {
		t.Fatal("nil for ISO string")
	}
	if result.TimestampMs <= 0 {
		t.Error("timestamp should be positive")
	}

	// 空字符串
	result = NormalizeTimestamp("")
	if result != nil {
		t.Error("empty string should return nil")
	}
}

func TestNormalizeTimestamp_Time(t *testing.T) {
	now := time.Now()
	result := NormalizeTimestamp(now)
	if result == nil {
		t.Fatal("nil for time.Time")
	}
	if result.TimestampMs != now.UnixMilli() {
		t.Errorf("time: ms = %d, want %d", result.TimestampMs, now.UnixMilli())
	}
}

func TestNormalizeTimestamp_Nil(t *testing.T) {
	result := NormalizeTimestamp(nil)
	if result != nil {
		t.Error("nil input should return nil")
	}
}

func TestOrdinalSuffix(t *testing.T) {
	cases := map[int]string{
		1: "st", 2: "nd", 3: "rd", 4: "th",
		11: "th", 12: "th", 13: "th",
		21: "st", 22: "nd", 23: "rd", 24: "th",
	}
	for day, want := range cases {
		got := OrdinalSuffix(day)
		if got != want {
			t.Errorf("OrdinalSuffix(%d) = %q, want %q", day, got, want)
		}
	}
}

func TestFormatUserTime(t *testing.T) {
	ts := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)

	result := FormatUserTime(ts, "UTC", Resolved24)
	if result == "" {
		t.Error("FormatUserTime returned empty")
	}
	// 应包含 Friday, March 15th, 2024
	if result != "Friday, March 15th, 2024 — 14:30" {
		t.Errorf("24h format = %q, want Friday, March 15th, 2024 — 14:30", result)
	}
}

func TestResolveUserTimeFormat(t *testing.T) {
	// 明确指定
	if ResolveUserTimeFormat(TimeFormat12) != Resolved12 {
		t.Error("explicit 12 should return 12")
	}
	if ResolveUserTimeFormat(TimeFormat24) != Resolved24 {
		t.Error("explicit 24 should return 24")
	}
	// auto 应返回有效值
	format := ResolveUserTimeFormat(TimeFormatAuto)
	if format != Resolved12 && format != Resolved24 {
		t.Errorf("auto = %q, want 12 or 24", format)
	}
}
