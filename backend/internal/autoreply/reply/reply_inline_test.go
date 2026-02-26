package reply

import "testing"

func TestExtractInlineSimpleCommand_Empty(t *testing.T) {
	if ExtractInlineSimpleCommand("") != nil {
		t.Error("expected nil for empty body")
	}
}

func TestExtractInlineSimpleCommand_NoCommand(t *testing.T) {
	if ExtractInlineSimpleCommand("hello world") != nil {
		t.Error("expected nil for body without command")
	}
}

func TestExtractInlineSimpleCommand_Help(t *testing.T) {
	r := ExtractInlineSimpleCommand("hello /help world")
	if r == nil {
		t.Fatal("expected result")
	}
	if r.Command != "/help" {
		t.Errorf("Command = %q, want /help", r.Command)
	}
	if r.Cleaned != "hello world" {
		t.Errorf("Cleaned = %q, want 'hello world'", r.Cleaned)
	}
}

func TestExtractInlineSimpleCommand_Id(t *testing.T) {
	r := ExtractInlineSimpleCommand("/id")
	if r == nil {
		t.Fatal("expected result")
	}
	if r.Command != "/whoami" {
		t.Errorf("Command = %q, want /whoami (alias)", r.Command)
	}
}

func TestExtractInlineSimpleCommand_Commands(t *testing.T) {
	r := ExtractInlineSimpleCommand("test /commands end")
	if r == nil {
		t.Fatal("expected result")
	}
	if r.Command != "/commands" {
		t.Errorf("Command = %q, want /commands", r.Command)
	}
}

func TestExtractInlineSimpleCommand_CaseInsensitive(t *testing.T) {
	r := ExtractInlineSimpleCommand("/HELP me")
	if r == nil {
		t.Fatal("expected result for case insensitive")
	}
	if r.Command != "/help" {
		t.Errorf("Command = %q, want /help", r.Command)
	}
}

func TestStripInlineStatus_NoStatus(t *testing.T) {
	r := StripInlineStatus("hello world")
	if r.DidStrip {
		t.Error("should not strip without /status")
	}
	if r.Cleaned != "hello world" {
		t.Errorf("Cleaned = %q, want 'hello world'", r.Cleaned)
	}
}

func TestStripInlineStatus_WithStatus(t *testing.T) {
	r := StripInlineStatus("hello /status world")
	if !r.DidStrip {
		t.Error("expected strip")
	}
	if r.Cleaned != "hello world" {
		t.Errorf("Cleaned = %q, want 'hello world'", r.Cleaned)
	}
}

func TestStripInlineStatus_Empty(t *testing.T) {
	r := StripInlineStatus("")
	if r.DidStrip {
		t.Error("should not strip empty")
	}
}
