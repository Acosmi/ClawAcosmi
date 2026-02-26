// Package services — memory_compressor 单元测试。
package services

import (
	"context"
	"testing"
)

func TestCompressionStats_Defaults(t *testing.T) {
	stats := &CompressionStats{}
	if stats.Archived != 0 || stats.Merged != 0 || stats.TokensSaved != 0 {
		t.Fatal("默认统计值应为 0")
	}
}

func TestSummarizeMemories_NoLLM(t *testing.T) {
	mc := &MemoryCompressor{llm: nil}
	result, err := mc.summarizeMemories(context.Background(),
		[]string{"记忆A", "记忆B"}, "fact")
	if err != nil {
		t.Fatalf("无 LLM 不应报错: %v", err)
	}
	if result != "记忆A; 记忆B" {
		t.Errorf("无 LLM 应拼接内容，得到: %s", result)
	}
}

func TestSummarizeMemories_WithLLM(t *testing.T) {
	mock := &mockLLMForFacts{response: "用户有两个相关记忆"}
	mc := &MemoryCompressor{llm: mock}
	result, err := mc.summarizeMemories(context.Background(),
		[]string{"记忆A", "记忆B"}, "fact")
	if err != nil {
		t.Fatalf("LLM 摘要不应报错: %v", err)
	}
	if result != "用户有两个相关记忆" {
		t.Errorf("摘要结果不匹配: %s", result)
	}
}

func TestArchiveThresholdConsistency(t *testing.T) {
	// ArchiveThreshold 应小于 ConsolidationThreshold
	if ArchiveThreshold >= ConsolidationThreshold {
		t.Errorf("ArchiveThreshold (%.2f) 应小于 ConsolidationThreshold (%.2f)",
			ArchiveThreshold, ConsolidationThreshold)
	}
}

func TestNewMemoryCompressor(t *testing.T) {
	mc := NewMemoryCompressor(nil, nil, nil)
	if mc == nil {
		t.Fatal("NewMemoryCompressor 不应返回 nil")
	}
}
