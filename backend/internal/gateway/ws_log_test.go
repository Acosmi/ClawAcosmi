package gateway

import (
	"testing"
)

// ---------- WsLogStyle ----------

func TestGetSetWsLogStyle(t *testing.T) {
	// 保存原始值并恢复
	original := GetWsLogStyle()
	defer SetWsLogStyle(original)

	SetWsLogStyle(WsLogStyleFull)
	if got := GetWsLogStyle(); got != WsLogStyleFull {
		t.Errorf("GetWsLogStyle() = %q, want %q", got, WsLogStyleFull)
	}

	SetWsLogStyle(WsLogStyleCompact)
	if got := GetWsLogStyle(); got != WsLogStyleCompact {
		t.Errorf("GetWsLogStyle() = %q, want %q", got, WsLogStyleCompact)
	}

	SetWsLogStyle(WsLogStyleAuto)
	if got := GetWsLogStyle(); got != WsLogStyleAuto {
		t.Errorf("GetWsLogStyle() = %q, want %q", got, WsLogStyleAuto)
	}
}

// ---------- ShortID ----------

func TestShortID_UUID(t *testing.T) {
	got := ShortID("550e8400-e29b-41d4-a716-446655440000")
	want := "550e8400…0000"
	if got != want {
		t.Errorf("ShortID(UUID) = %q, want %q", got, want)
	}
}

func TestShortID_Short(t *testing.T) {
	got := ShortID("short-id")
	if got != "short-id" {
		t.Errorf("ShortID(short) = %q, want %q", got, "short-id")
	}
}

func TestShortID_Long(t *testing.T) {
	got := ShortID("this-is-a-very-long-identifier-that-exceeds-limit")
	if len(got) > 20 {
		// 12 + "…" + 4 = 17 characters
	}
	if got != "this-is-a-ve…imit" {
		t.Errorf("ShortID(long) = %q, want %q", got, "this-is-a-ve…imit")
	}
}

func TestShortID_Empty(t *testing.T) {
	got := ShortID("")
	if got != "?" {
		t.Errorf("ShortID('') = %q, want %q", got, "?")
	}
}

func TestShortID_Whitespace(t *testing.T) {
	got := ShortID("  ")
	if got != "?" {
		t.Errorf("ShortID(whitespace) = %q, want %q", got, "?")
	}
}

// ---------- FormatForLog ----------

func TestFormatForLog_String(t *testing.T) {
	got := FormatForLog("hello world")
	if got != "hello world" {
		t.Errorf("FormatForLog(string) = %q", got)
	}
}

func TestFormatForLog_Nil(t *testing.T) {
	got := FormatForLog(nil)
	if got != "" {
		t.Errorf("FormatForLog(nil) = %q, want empty", got)
	}
}

func TestFormatForLog_Long(t *testing.T) {
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'a'
	}
	got := FormatForLog(string(long))
	if len(got) > LogValueLimit+3 { // +3 for "..."
		t.Errorf("FormatForLog(long) length = %d, want <= %d", len(got), LogValueLimit+3)
	}
}

func TestFormatForLog_Map(t *testing.T) {
	got := FormatForLog(map[string]string{"key": "value"})
	if got == "" {
		t.Error("FormatForLog(map) should not be empty")
	}
}

// ---------- LogWs 各模式 ----------

func TestLogWs_AutoMode_DoesNotPanic(t *testing.T) {
	original := GetWsLogStyle()
	defer SetWsLogStyle(original)
	SetWsLogStyle(WsLogStyleAuto)

	// 不应 panic
	LogWs("in", "req", map[string]interface{}{
		"connId": "conn-1",
		"id":     "req-1",
		"method": "chat.send",
	})
	LogWs("out", "res", map[string]interface{}{
		"connId": "conn-1",
		"id":     "req-1",
		"method": "chat.send",
		"ok":     true,
	})
	LogWs("out", "res", map[string]interface{}{
		"connId": "conn-1",
		"id":     "req-2",
		"method": "bad.method",
		"ok":     false,
	})
}

func TestLogWs_CompactMode_DoesNotPanic(t *testing.T) {
	original := GetWsLogStyle()
	defer SetWsLogStyle(original)
	SetWsLogStyle(WsLogStyleCompact)

	LogWs("in", "req", map[string]interface{}{
		"connId": "conn-1",
		"id":     "req-1",
		"method": "sessions.list",
	})
	LogWs("out", "res", map[string]interface{}{
		"connId": "conn-1",
		"id":     "req-1",
		"method": "sessions.list",
		"ok":     true,
	})
	LogWs("out", "event", map[string]interface{}{
		"connId": "conn-1",
		"event":  "tick",
	})
}

func TestLogWs_FullMode_DoesNotPanic(t *testing.T) {
	original := GetWsLogStyle()
	defer SetWsLogStyle(original)
	SetWsLogStyle(WsLogStyleFull)

	LogWs("in", "open", map[string]interface{}{
		"connId":     "conn-1",
		"remoteAddr": "127.0.0.1",
	})
	LogWs("in", "req", map[string]interface{}{
		"connId": "conn-1",
		"id":     "req-1",
		"method": "config.get",
	})
	LogWs("out", "res", map[string]interface{}{
		"connId": "conn-1",
		"id":     "req-1",
		"method": "config.get",
		"ok":     true,
	})
	LogWs("in", "close", map[string]interface{}{
		"connId":     "conn-1",
		"code":       1000,
		"durationMs": 5000,
	})
}

func TestLogWs_ParseError_DoesNotPanic(t *testing.T) {
	original := GetWsLogStyle()
	defer SetWsLogStyle(original)
	SetWsLogStyle(WsLogStyleAuto)

	LogWs("in", "parse-error", map[string]interface{}{
		"connId": "conn-bad",
		"error":  "invalid JSON",
	})
}

func TestLogWs_NilMeta(t *testing.T) {
	original := GetWsLogStyle()
	defer SetWsLogStyle(original)

	// 所有模式下 nil meta 不应 panic
	for _, style := range []WsLogStyle{WsLogStyleAuto, WsLogStyleCompact, WsLogStyleFull} {
		SetWsLogStyle(style)
		LogWs("in", "req", nil)
		LogWs("out", "res", nil)
		LogWs("out", "event", nil)
	}
}

// ---------- 内部辅助函数 ----------

func TestInflightKeyFrom(t *testing.T) {
	if got := inflightKeyFrom("conn-1", "req-1"); got != "conn-1:req-1" {
		t.Errorf("inflightKeyFrom = %q", got)
	}
	if got := inflightKeyFrom("", "req-1"); got != "" {
		t.Errorf("inflightKeyFrom(empty conn) = %q, want empty", got)
	}
	if got := inflightKeyFrom("conn-1", ""); got != "" {
		t.Errorf("inflightKeyFrom(empty id) = %q, want empty", got)
	}
}

func TestDirectionArrow(t *testing.T) {
	if got := directionArrow("in", "req"); got != "⇄" {
		t.Errorf("req arrow = %q", got)
	}
	if got := directionArrow("out", "res"); got != "⇄" {
		t.Errorf("res arrow = %q", got)
	}
	if got := directionArrow("in", "event"); got != "←" {
		t.Errorf("in event arrow = %q", got)
	}
	if got := directionArrow("out", "event"); got != "→" {
		t.Errorf("out event arrow = %q", got)
	}
}

func TestDefaultWsSlowMs_Constant(t *testing.T) {
	if DefaultWsSlowMs != 50 {
		t.Errorf("DefaultWsSlowMs = %d, want 50", DefaultWsSlowMs)
	}
}

// ---------- CompactPreview ----------

func TestCompactPreview_Short(t *testing.T) {
	got := CompactPreview("hello world", 160)
	if got != "hello world" {
		t.Errorf("CompactPreview(short) = %q", got)
	}
}

func TestCompactPreview_Long(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	got := CompactPreview(long, 20)
	if len(got) > 25 { // 19 chars + "…" (3 bytes)
		t.Errorf("CompactPreview(long) length = %d", len(got))
	}
}

func TestCompactPreview_Multiline(t *testing.T) {
	got := CompactPreview("hello\n  world\n  foo", 160)
	if got != "hello world foo" {
		t.Errorf("CompactPreview(multiline) = %q", got)
	}
}

func TestCompactPreview_Default(t *testing.T) {
	got := CompactPreview("short", 0) // maxLen=0 → default 160
	if got != "short" {
		t.Errorf("CompactPreview(default) = %q", got)
	}
}

// ---------- SummarizeAgentEventForWsLog ----------

func TestSummarizeAgentEvent_Nil(t *testing.T) {
	got := SummarizeAgentEventForWsLog(nil)
	if len(got) != 0 {
		t.Errorf("Summarize(nil) should be empty, got %v", got)
	}
}

func TestSummarizeAgentEvent_Empty(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{})
	if len(got) != 0 {
		t.Errorf("Summarize(empty) should be empty, got %v", got)
	}
}

func TestSummarizeAgentEvent_BasicFields(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{
		"runId":  "550e8400-e29b-41d4-a716-446655440000",
		"stream": "assistant",
		"seq":    float64(5),
	})
	if got["run"] != "550e8400…0000" {
		t.Errorf("run = %q", got["run"])
	}
	if got["stream"] != "assistant" {
		t.Errorf("stream = %q", got["stream"])
	}
	if got["aseq"] != 5 {
		t.Errorf("aseq = %v", got["aseq"])
	}
}

func TestSummarizeAgentEvent_Assistant(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{
		"stream": "assistant",
		"data": map[string]interface{}{
			"text":      "Hello, world!",
			"mediaUrls": []interface{}{"https://example.com/img.png"},
		},
	})
	if got["text"] != "Hello, world!" {
		t.Errorf("text = %q", got["text"])
	}
	if got["media"] != 1 {
		t.Errorf("media = %v", got["media"])
	}
}

func TestSummarizeAgentEvent_Tool(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{
		"stream": "tool",
		"data": map[string]interface{}{
			"phase":      "start",
			"name":       "web_search",
			"toolCallId": "550e8400-e29b-41d4-a716-446655440000",
			"isError":    false,
		},
	})
	if got["tool"] != "start:web_search" {
		t.Errorf("tool = %q", got["tool"])
	}
	if got["call"] != "550e8400…0000" {
		t.Errorf("call = %q", got["call"])
	}
	if got["err"] != false {
		t.Errorf("err = %v", got["err"])
	}
}

func TestSummarizeAgentEvent_Lifecycle(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{
		"stream": "lifecycle",
		"data": map[string]interface{}{
			"phase":   "complete",
			"aborted": true,
			"error":   "timeout exceeded",
		},
	})
	if got["phase"] != "complete" {
		t.Errorf("phase = %q", got["phase"])
	}
	if got["aborted"] != true {
		t.Errorf("aborted = %v", got["aborted"])
	}
	if got["error"] != "timeout exceeded" {
		t.Errorf("error = %q", got["error"])
	}
}

func TestSummarizeAgentEvent_Other(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{
		"stream": "custom",
		"data": map[string]interface{}{
			"reason": "user requested",
		},
	})
	if got["reason"] != "user requested" {
		t.Errorf("reason = %q", got["reason"])
	}
}

func TestSummarizeAgentEvent_SessionKey(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{
		"sessionKey": "agent:mybot:main",
	})
	if got["agent"] != "mybot" {
		t.Errorf("agent = %q", got["agent"])
	}
	if got["session"] != "main" {
		t.Errorf("session = %q", got["session"])
	}
}

func TestSummarizeAgentEvent_UnparsableSessionKey(t *testing.T) {
	got := SummarizeAgentEventForWsLog(map[string]interface{}{
		"sessionKey": "plain-session-key",
	})
	if got["session"] != "plain-session-key" {
		t.Errorf("session = %q", got["session"])
	}
}

// ---------- RedactSensitiveText ----------

func TestRedact_Empty(t *testing.T) {
	got := RedactSensitiveText("")
	if got != "" {
		t.Errorf("RedactSensitiveText('') = %q", got)
	}
}

func TestRedact_NoSensitive(t *testing.T) {
	got := RedactSensitiveText("hello world, no secrets here")
	if got != "hello world, no secrets here" {
		t.Errorf("RedactSensitiveText(clean) = %q", got)
	}
}

func TestRedact_APIKey_SK(t *testing.T) {
	got := RedactSensitiveText("my key is sk-1234567890abcdefghijklmnop")
	if got == "my key is sk-1234567890abcdefghijklmnop" {
		t.Error("sk- prefix should be redacted")
	}
	if len(got) == 0 {
		t.Error("result should not be empty")
	}
	// Should contain masked version
	if !containsStr(got, "…") {
		t.Errorf("should contain ellipsis in masked token, got: %q", got)
	}
}

func TestRedact_APIKey_GHP(t *testing.T) {
	got := RedactSensitiveText("export token=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if got == "export token=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		t.Error("ghp_ prefix should be redacted")
	}
}

func TestRedact_Bearer(t *testing.T) {
	got := RedactSensitiveText("Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test")
	if got == "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test" {
		t.Error("Bearer token should be redacted")
	}
}

func TestRedact_PEM(t *testing.T) {
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBAPR...\nmore lines\n-----END RSA PRIVATE KEY-----"
	got := RedactSensitiveText(pem)
	if got == pem {
		t.Error("PEM block should be redacted")
	}
	if !containsStr(got, "…redacted…") {
		t.Errorf("PEM redaction should contain '…redacted…', got: %q", got)
	}
}

func TestRedact_EnvAssignment(t *testing.T) {
	got := RedactSensitiveText("API_KEY=super-secret-long-value-12345")
	if got == "API_KEY=super-secret-long-value-12345" {
		t.Error("env assignment should be redacted")
	}
}

func TestRedact_JSONField(t *testing.T) {
	got := RedactSensitiveText(`{"apiKey": "sk-1234567890abcdefghijklmno"}`)
	if got == `{"apiKey": "sk-1234567890abcdefghijklmno"}` {
		t.Error("JSON apiKey field should be redacted")
	}
}

func TestMaskToken_Short(t *testing.T) {
	got := maskToken("short")
	if got != "***" {
		t.Errorf("maskToken(short) = %q, want ***", got)
	}
}

func TestMaskToken_Long(t *testing.T) {
	got := maskToken("abcdefghijklmnopqrstuvwxyz")
	// start=6 + "…" + end=4
	if got != "abcdef…wxyz" {
		t.Errorf("maskToken(long) = %q, want abcdef…wxyz", got)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
