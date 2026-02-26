// Package services — unit tests for bi-temporal event time extraction.
package services

import (
	"context"
	"testing"
	"time"
)

// --- parseFlexibleTime tests ---

func TestParseFlexibleTime_RFC3339(t *testing.T) {
	result, err := parseFlexibleTime("2025-06-15T10:30:00Z")
	if err != nil {
		t.Fatalf("RFC3339 解析失败: %v", err)
	}
	if result.Year() != 2025 || result.Month() != 6 || result.Day() != 15 {
		t.Errorf("日期不匹配: %v", result)
	}
}

func TestParseFlexibleTime_RFC3339WithOffset(t *testing.T) {
	result, err := parseFlexibleTime("2025-06-15T10:30:00+08:00")
	if err != nil {
		t.Fatalf("RFC3339+offset 解析失败: %v", err)
	}
	if result.Year() != 2025 || result.Month() != 6 {
		t.Errorf("日期不匹配: %v", result)
	}
}

func TestParseFlexibleTime_DateOnly(t *testing.T) {
	result, err := parseFlexibleTime("2025-06-15")
	if err != nil {
		t.Fatalf("纯日期解析失败: %v", err)
	}
	if result.Year() != 2025 || result.Month() != 6 || result.Day() != 15 {
		t.Errorf("日期不匹配: %v", result)
	}
}

func TestParseFlexibleTime_NoTimezone(t *testing.T) {
	result, err := parseFlexibleTime("2025-06-15T10:30:00")
	if err != nil {
		t.Fatalf("无时区格式解析失败: %v", err)
	}
	if result.Year() != 2025 {
		t.Errorf("年份不匹配: %v", result)
	}
}

func TestParseFlexibleTime_Invalid(t *testing.T) {
	_, err := parseFlexibleTime("not a date")
	if err == nil {
		t.Fatal("无效字符串应返回错误")
	}
}

func TestParseFlexibleTime_EmptyString(t *testing.T) {
	_, err := parseFlexibleTime("")
	if err == nil {
		t.Fatal("空字符串应返回错误")
	}
}

func TestParseFlexibleTime_WithWhitespace(t *testing.T) {
	result, err := parseFlexibleTime("  2025-06-15  ")
	if err != nil {
		t.Fatalf("带空白字符解析失败: %v", err)
	}
	if result.Day() != 15 {
		t.Errorf("日期不匹配: %v", result)
	}
}

// --- ExtractEventTime tests (nil LLM fallback) ---

func TestExtractEventTime_NilLLM(t *testing.T) {
	result, err := ExtractEventTime(context.Background(), nil, "去年6月结婚了")
	if err != nil {
		t.Fatalf("nil LLM 不应返回错误: %v", err)
	}
	if result != nil {
		t.Fatal("nil LLM 应返回 nil event_time")
	}
}

// --- VectorSearchResult.EventTime field test ---

func TestVectorSearchResult_EventTimeField(t *testing.T) {
	now := time.Now()
	r := VectorSearchResult{
		EventTime: &now,
	}
	if r.EventTime == nil {
		t.Fatal("EventTime 应非 nil")
	}
	if !r.EventTime.Equal(now) {
		t.Errorf("EventTime 不匹配: %v != %v", r.EventTime, now)
	}
}
