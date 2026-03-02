package reply

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

func TestExtractThinkDirective(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantHas   bool
		wantLvl   autoreply.ThinkLevel
		wantClean string
	}{
		{"empty", "", false, "", ""},
		{"no directive", "hello world", false, "", "hello world"},
		{"/think high", "hello /think high world", true, autoreply.ThinkHigh, "hello world"},
		{"/t medium", "hello /t medium", true, autoreply.ThinkMedium, "hello"},
		{"/thinking off", "/thinking off", true, autoreply.ThinkOff, ""},
		{"/think no level", "/think", true, "", ""},
		{"/think xhigh", "/think xhigh test", true, autoreply.ThinkXHigh, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ExtractThinkDirective(tt.body)
			if r.HasDirective != tt.wantHas {
				t.Errorf("HasDirective = %v, want %v", r.HasDirective, tt.wantHas)
			}
			if r.ThinkLevel != tt.wantLvl {
				t.Errorf("ThinkLevel = %q, want %q", r.ThinkLevel, tt.wantLvl)
			}
			if tt.wantClean != "" && r.Cleaned != tt.wantClean {
				t.Errorf("Cleaned = %q, want %q", r.Cleaned, tt.wantClean)
			}
		})
	}
}

func TestExtractVerboseDirective(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantHas bool
		wantLvl autoreply.VerboseLevel
	}{
		{"empty", "", false, ""},
		{"/verbose on", "/verbose on", true, autoreply.VerboseOn},
		{"/v off", "test /v off", true, autoreply.VerboseOff},
		{"/verbose full", "/verbose full", true, autoreply.VerboseFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ExtractVerboseDirective(tt.body)
			if r.HasDirective != tt.wantHas {
				t.Errorf("HasDirective = %v, want %v", r.HasDirective, tt.wantHas)
			}
			if r.VerboseLevel != tt.wantLvl {
				t.Errorf("VerboseLevel = %q, want %q", r.VerboseLevel, tt.wantLvl)
			}
		})
	}
}

func TestExtractElevatedDirective(t *testing.T) {
	r := ExtractElevatedDirective("/elevated on do stuff")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.ElevatedLevel != autoreply.ElevatedOn {
		t.Errorf("ElevatedLevel = %q, want %q", r.ElevatedLevel, autoreply.ElevatedOn)
	}
}

func TestExtractReasoningDirective(t *testing.T) {
	r := ExtractReasoningDirective("/reasoning stream test")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.ReasoningLevel != autoreply.ReasoningStream {
		t.Errorf("ReasoningLevel = %q, want %q", r.ReasoningLevel, autoreply.ReasoningStream)
	}
}

func TestExtractStatusDirective(t *testing.T) {
	r := ExtractStatusDirective("hello /status world")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.Cleaned != "hello world" {
		t.Errorf("Cleaned = %q, want %q", r.Cleaned, "hello world")
	}
}

func TestParseInlineDirectives(t *testing.T) {
	body := "/think high /verbose on hello world"
	d := ParseInlineDirectives(body, nil)
	if !d.HasThinkDirective {
		t.Error("expected think directive")
	}
	if !d.HasVerboseDirective {
		t.Error("expected verbose directive")
	}
	if d.ThinkLevel != autoreply.ThinkHigh {
		t.Errorf("ThinkLevel = %q, want high", d.ThinkLevel)
	}
	if d.VerboseLevel != autoreply.VerboseOn {
		t.Errorf("VerboseLevel = %q, want on", d.VerboseLevel)
	}
}

func TestParseInlineDirectivesDisableElevated(t *testing.T) {
	body := "/elevated on test"
	d := ParseInlineDirectives(body, &ParseInlineDirectivesOptions{DisableElevated: true})
	if d.HasElevatedDirective {
		t.Error("elevated should be disabled")
	}
}

func TestIsDirectiveOnly(t *testing.T) {
	d := ParseInlineDirectives("/think high", nil)
	if !IsDirectiveOnly(d, d.Cleaned, false, nil) {
		t.Error("expected directive only")
	}

	d2 := ParseInlineDirectives("/think high actual text", nil)
	if IsDirectiveOnly(d2, d2.Cleaned, false, nil) {
		t.Error("should not be directive only with text")
	}
}

func TestExtractExecDirective(t *testing.T) {
	r := ExtractExecDirective("/exec host=sandbox security=full")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.ExecHost != ExecHostSandbox {
		t.Errorf("ExecHost = %q, want sandbox", r.ExecHost)
	}
	if r.ExecSecurity != ExecSecurityFull {
		t.Errorf("ExecSecurity = %q, want full", r.ExecSecurity)
	}
}

func TestExtractQueueDirective(t *testing.T) {
	r := ExtractQueueDirective("/queue followup debounce:500ms cap:3")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.QueueMode != QueueModeFollowup {
		t.Errorf("QueueMode = %q, want followup", r.QueueMode)
	}
	if r.DebounceMs == nil || *r.DebounceMs != 500 {
		t.Errorf("DebounceMs = %v, want 500", r.DebounceMs)
	}
	if r.Cap == nil || *r.Cap != 3 {
		t.Errorf("Cap = %v, want 3", r.Cap)
	}
}

func TestExtractQueueDirectiveReset(t *testing.T) {
	r := ExtractQueueDirective("/queue reset")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if !r.QueueReset {
		t.Error("expected QueueReset")
	}
}
