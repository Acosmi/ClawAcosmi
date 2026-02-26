package plugins

import "testing"

func TestValidateCommandName_Empty(t *testing.T) {
	if msg := ValidateCommandName(""); msg == "" {
		t.Error("expected error for empty name")
	}
}

func TestValidateCommandName_Reserved(t *testing.T) {
	if msg := ValidateCommandName("help"); msg == "" {
		t.Error("expected error for reserved name 'help'")
	}
	if msg := ValidateCommandName("STOP"); msg == "" {
		t.Error("expected error for reserved name 'STOP' (case insensitive)")
	}
}

func TestValidateCommandName_InvalidChars(t *testing.T) {
	if msg := ValidateCommandName("foo bar"); msg == "" {
		t.Error("expected error for name with space")
	}
	if msg := ValidateCommandName("123abc"); msg == "" {
		t.Error("expected error for name starting with digit")
	}
}

func TestValidateCommandName_Valid(t *testing.T) {
	if msg := ValidateCommandName("tts"); msg != "" {
		t.Errorf("expected valid, got: %s", msg)
	}
	if msg := ValidateCommandName("my-cool-cmd"); msg != "" {
		t.Errorf("expected valid, got: %s", msg)
	}
}

func TestRegisterPluginCommand_Success(t *testing.T) {
	ClearPluginCommands()
	defer ClearPluginCommands()

	handler := func(ctx PluginCommandContext) (PluginCommandResult, error) {
		return PluginCommandResult{Text: "ok"}, nil
	}
	result := RegisterPluginCommand("test-plugin", PluginCommandDefinition{
		Name:        "tts",
		Description: "Text to speech",
		Handler:     handler,
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %s", result.Error)
	}
}

func TestRegisterPluginCommand_DuplicateReject(t *testing.T) {
	ClearPluginCommands()
	defer ClearPluginCommands()

	handler := func(ctx PluginCommandContext) (PluginCommandResult, error) {
		return PluginCommandResult{}, nil
	}
	cmd := PluginCommandDefinition{Name: "dup", Description: "test", Handler: handler}
	RegisterPluginCommand("p1", cmd)
	result := RegisterPluginCommand("p2", cmd)
	if result.OK {
		t.Error("expected duplicate rejection")
	}
}

func TestMatchPluginCommand_Found(t *testing.T) {
	ClearPluginCommands()
	defer ClearPluginCommands()

	handler := func(ctx PluginCommandContext) (PluginCommandResult, error) {
		return PluginCommandResult{Text: "ok"}, nil
	}
	RegisterPluginCommand("p1", PluginCommandDefinition{
		Name: "tts", Description: "test", AcceptsArgs: true, Handler: handler,
	})

	match := MatchPluginCommand("/tts hello world")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.Command.Name != "tts" {
		t.Errorf("expected 'tts', got %q", match.Command.Name)
	}
	if match.Args != "hello world" {
		t.Errorf("expected 'hello world', got %q", match.Args)
	}
}

func TestMatchPluginCommand_NoArgsReject(t *testing.T) {
	ClearPluginCommands()
	defer ClearPluginCommands()

	handler := func(ctx PluginCommandContext) (PluginCommandResult, error) {
		return PluginCommandResult{}, nil
	}
	RegisterPluginCommand("p1", PluginCommandDefinition{
		Name: "toggle", Description: "test", AcceptsArgs: false, Handler: handler,
	})

	match := MatchPluginCommand("/toggle some args")
	if match != nil {
		t.Error("expected no match when args provided but not accepted")
	}

	match2 := MatchPluginCommand("/toggle")
	if match2 == nil {
		t.Error("expected match for command without args")
	}
}

func TestMatchPluginCommand_NotFound(t *testing.T) {
	ClearPluginCommands()
	if match := MatchPluginCommand("/unknown"); match != nil {
		t.Error("expected nil match for unknown command")
	}
	if match := MatchPluginCommand("not a command"); match != nil {
		t.Error("expected nil match for non-slash prefixed")
	}
}

func TestSanitizeArgs(t *testing.T) {
	// Preserves tabs and newlines
	result := SanitizeArgs("hello\tworld\nfoo")
	if result != "hello\tworld\nfoo" {
		t.Errorf("expected preserved tabs+newlines, got %q", result)
	}

	// Removes control chars
	result2 := SanitizeArgs("hello\x00world\x7f")
	if result2 != "helloworld" {
		t.Errorf("expected control chars removed, got %q", result2)
	}
}

func TestListPluginCommands_Empty(t *testing.T) {
	ClearPluginCommands()
	list := ListPluginCommands()
	if len(list) != 0 {
		t.Errorf("expected 0 commands, got %d", len(list))
	}
}

func TestNormalizePluginHttpPath(t *testing.T) {
	tests := []struct {
		path     string
		fallback string
		want     string
	}{
		{"", "", ""},
		{"", "api", "/api"},
		{"", "/api", "/api"},
		{"webhook", "", "/webhook"},
		{"/webhook", "", "/webhook"},
		{"  /spaced  ", "", "/spaced"},
	}
	for _, tt := range tests {
		got := NormalizePluginHttpPath(tt.path, tt.fallback)
		if got != tt.want {
			t.Errorf("NormalizePluginHttpPath(%q, %q) = %q, want %q", tt.path, tt.fallback, got, tt.want)
		}
	}
}
