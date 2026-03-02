package runner

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/openacosmi/claw-acismi/internal/agents/llmclient"
)

// ---------- H3-2: CompactionFailureEmitter ----------

func TestCompactionFailureEmitter_EmitNotifiesListeners(t *testing.T) {
	emitter := &CompactionFailureEmitter{}
	var called int32 // atomic counter

	emitter.OnCompactionFailure(func(reason string) {
		atomic.AddInt32(&called, 1)
		if reason != "test-reason" {
			t.Errorf("unexpected reason: %s", reason)
		}
	})
	emitter.OnCompactionFailure(func(reason string) {
		atomic.AddInt32(&called, 1)
	})

	emitter.EmitCompactionFailure("test-reason")

	if atomic.LoadInt32(&called) != 2 {
		t.Errorf("expected 2 listeners called, got %d", called)
	}
}

func TestCompactionFailureEmitter_UnregisterWorks(t *testing.T) {
	emitter := &CompactionFailureEmitter{}
	var called int32

	unregister := emitter.OnCompactionFailure(func(reason string) {
		atomic.AddInt32(&called, 1)
	})

	// 取消注册
	unregister()

	emitter.EmitCompactionFailure("should-not-reach")

	if atomic.LoadInt32(&called) != 0 {
		t.Errorf("unregistered listener should not be called, got %d", called)
	}
}

func TestCompactionFailureEmitter_PanicRecovery(t *testing.T) {
	emitter := &CompactionFailureEmitter{}
	var secondCalled int32

	// 第一个回调 panic
	emitter.OnCompactionFailure(func(reason string) {
		panic("intentional panic")
	})
	// 第二个回调应该仍然被调用
	emitter.OnCompactionFailure(func(reason string) {
		atomic.AddInt32(&secondCalled, 1)
	})

	// 不应 panic
	emitter.EmitCompactionFailure("test")

	if atomic.LoadInt32(&secondCalled) != 1 {
		t.Error("second listener should still be called after first panics")
	}
}

func TestCompactionFailureEmitter_ConcurrentSafe(t *testing.T) {
	emitter := &CompactionFailureEmitter{}
	var wg sync.WaitGroup
	var count int32

	// 并发注册 + 触发
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			emitter.OnCompactionFailure(func(reason string) {
				atomic.AddInt32(&count, 1)
			})
		}()
	}
	wg.Wait()

	emitter.EmitCompactionFailure("concurrent-test")
	if atomic.LoadInt32(&count) != 10 {
		t.Errorf("expected 10 calls, got %d", count)
	}
}

// ---------- H3-3: rawStream ----------

func TestAppendRawStream_DisabledByDefault(t *testing.T) {
	ResetRawStreamForTest()
	// 默认环境变量未设置，应不做任何操作
	AppendRawStream(map[string]interface{}{"type": "test"})
	// 无文件写入 — 仅验证不 panic
}

// ---------- H3-4: promoteThinkingTagsToBlocks ----------

func TestSplitThinkingTaggedText_BasicSplit(t *testing.T) {
	text := `<thinking>I need to analyze this.</thinking>Here is my response.`
	blocks := SplitThinkingTaggedText(text)
	if blocks == nil {
		t.Fatal("expected non-nil blocks")
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "thinking" || blocks[0].Thinking != "I need to analyze this." {
		t.Errorf("block[0] = %+v", blocks[0])
	}
	if blocks[1].Type != "text" || blocks[1].Text != "Here is my response." {
		t.Errorf("block[1] = %+v", blocks[1])
	}
}

func TestSplitThinkingTaggedText_MultipleTags(t *testing.T) {
	text := `<think>First thought</think>Response 1<thinking>Second thought</thinking>Response 2`
	blocks := SplitThinkingTaggedText(text)
	if blocks == nil {
		t.Fatal("expected non-nil blocks")
	}
	if len(blocks) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "thinking" {
		t.Errorf("block[0] type = %s", blocks[0].Type)
	}
	if blocks[1].Type != "text" || blocks[1].Text != "Response 1" {
		t.Errorf("block[1] = %+v", blocks[1])
	}
}

func TestSplitThinkingTaggedText_NoTags(t *testing.T) {
	blocks := SplitThinkingTaggedText("Just plain text.")
	if blocks != nil {
		t.Errorf("expected nil for text without tags, got %d blocks", len(blocks))
	}
}

func TestSplitThinkingTaggedText_UnclosedTag(t *testing.T) {
	blocks := SplitThinkingTaggedText("<thinking>Unclosed tag content")
	if blocks != nil {
		t.Error("expected nil for unclosed thinking tag")
	}
}

func TestSplitThinkingTaggedText_VariantTags(t *testing.T) {
	text := `<thought>Analyzing...</thought>Result<antthinking>Meta-thought</antthinking>Final`
	blocks := SplitThinkingTaggedText(text)
	if blocks == nil {
		t.Fatal("expected non-nil blocks")
	}
	thinking := 0
	for _, b := range blocks {
		if b.Type == "thinking" {
			thinking++
		}
	}
	if thinking != 2 {
		t.Errorf("expected 2 thinking blocks, got %d", thinking)
	}
}

func TestPromoteThinkingTagsToBlocks_NoChange(t *testing.T) {
	content := []llmclient.ContentBlock{
		{Type: "text", Text: "Just text"},
	}
	result, changed := PromoteThinkingTagsToBlocks(content)
	if changed {
		t.Error("should not change plain text blocks")
	}
	if len(result) != 1 {
		t.Errorf("expected 1 block, got %d", len(result))
	}
}

func TestPromoteThinkingTagsToBlocks_ConvertsTag(t *testing.T) {
	content := []llmclient.ContentBlock{
		{Type: "text", Text: "<thinking>My thoughts</thinking>My response"},
	}
	result, changed := PromoteThinkingTagsToBlocks(content)
	if !changed {
		t.Error("should have changed")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	if result[0].Type != "thinking" || result[0].Thinking != "My thoughts" {
		t.Errorf("block[0] = %+v", result[0])
	}
	if result[1].Type != "text" || result[1].Text != "My response" {
		t.Errorf("block[1] = %+v", result[1])
	}
}

// ---------- H3-5: normalizeToolParameters ----------

func TestNormalizeToolParameters_WithTypeAndProperties(t *testing.T) {
	tool := llmclient.ToolDef{
		Name:        "test",
		Description: "desc",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","minLength":1}}}`),
	}
	result := NormalizeToolParameters(tool)
	var schema map[string]interface{}
	json.Unmarshal(result.InputSchema, &schema)
	// minLength should be cleaned by Gemini cleaning
	props := schema["properties"].(map[string]interface{})
	nameProp := props["name"].(map[string]interface{})
	if _, ok := nameProp["minLength"]; ok {
		t.Error("minLength should be cleaned")
	}
}

func TestNormalizeToolParameters_MissingType(t *testing.T) {
	tool := llmclient.ToolDef{
		Name:        "test",
		Description: "desc",
		InputSchema: json.RawMessage(`{"properties":{"arg":{"type":"string"}},"required":["arg"]}`),
	}
	result := NormalizeToolParameters(tool)
	var schema map[string]interface{}
	json.Unmarshal(result.InputSchema, &schema)
	if schema["type"] != "object" {
		t.Errorf("expected type:object, got %v", schema["type"])
	}
}

func TestNormalizeToolParameters_AnyOfUnion(t *testing.T) {
	tool := llmclient.ToolDef{
		Name:        "test",
		Description: "desc",
		InputSchema: json.RawMessage(`{
			"anyOf": [
				{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]},
				{"type":"object","properties":{"id":{"type":"number"}},"required":["id"]}
			]
		}`),
	}
	result := NormalizeToolParameters(tool)
	var schema map[string]interface{}
	json.Unmarshal(result.InputSchema, &schema)
	if schema["type"] != "object" {
		t.Errorf("expected type:object, got %v", schema["type"])
	}
	props := schema["properties"].(map[string]interface{})
	if _, ok := props["name"]; !ok {
		t.Error("expected 'name' property from first variant")
	}
	if _, ok := props["id"]; !ok {
		t.Error("expected 'id' property from second variant")
	}
}

func TestNormalizeToolParameters_EmptySchema(t *testing.T) {
	tool := llmclient.ToolDef{
		Name:        "test",
		Description: "desc",
		InputSchema: nil,
	}
	result := NormalizeToolParameters(tool)
	if result.Name != "test" {
		t.Error("should return tool unchanged")
	}
}

func TestMergePropertySchemas_EnumMerge(t *testing.T) {
	a := map[string]interface{}{"enum": []interface{}{"a", "b"}, "description": "first"}
	b := map[string]interface{}{"enum": []interface{}{"b", "c"}}
	result := mergePropertySchemas(a, b)
	obj := result.(map[string]interface{})
	enum := obj["enum"].([]interface{})
	if len(enum) != 3 {
		t.Errorf("expected 3 enum values (deduplicated), got %d: %v", len(enum), enum)
	}
	if obj["description"] != "first" {
		t.Error("should preserve description from first source")
	}
}
