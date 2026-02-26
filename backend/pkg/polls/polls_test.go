package polls

import (
	"reflect"
	"testing"
)

func TestNormalizePollInput_Valid(t *testing.T) {
	result, err := NormalizePollInput(PollInput{
		Question: "  Favorite color?  ",
		Options:  []string{"  Red ", "Blue", " Green "},
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Question != "Favorite color?" {
		t.Errorf("question: got %q, want %q", result.Question, "Favorite color?")
	}
	want := []string{"Red", "Blue", "Green"}
	if !reflect.DeepEqual(result.Options, want) {
		t.Errorf("options: got %v, want %v", result.Options, want)
	}
	if result.MaxSelections != 1 {
		t.Errorf("maxSelections: got %d, want 1", result.MaxSelections)
	}
}

func TestNormalizePollInput_EmptyQuestion(t *testing.T) {
	_, err := NormalizePollInput(PollInput{
		Question: "   ",
		Options:  []string{"A", "B"},
	}, 0)
	if err == nil {
		t.Fatal("expected error for empty question")
	}
}

func TestNormalizePollInput_TooFewOptions(t *testing.T) {
	_, err := NormalizePollInput(PollInput{
		Question: "Q?",
		Options:  []string{"Only one"},
	}, 0)
	if err == nil {
		t.Fatal("expected error for < 2 options")
	}
}

func TestNormalizePollInput_MaxOptionsExceeded(t *testing.T) {
	_, err := NormalizePollInput(PollInput{
		Question: "Q?",
		Options:  []string{"A", "B", "C", "D"},
	}, 3)
	if err == nil {
		t.Fatal("expected error when options exceed maxOptions")
	}
}

func TestNormalizePollInput_TrimEmpty(t *testing.T) {
	result, err := NormalizePollInput(PollInput{
		Question: "Q?",
		Options:  []string{"  ", "", "valid", "  spaces  "},
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"valid", "spaces"}
	if !reflect.DeepEqual(result.Options, want) {
		t.Errorf("options: got %v, want %v", result.Options, want)
	}
}

func TestNormalizePollInput_MaxSelectionsExceedsOptions(t *testing.T) {
	_, err := NormalizePollInput(PollInput{
		Question:      "Q?",
		Options:       []string{"A", "B"},
		MaxSelections: 5,
	}, 0)
	if err == nil {
		t.Fatal("expected error when maxSelections > option count")
	}
}

func TestNormalizePollInput_DurationValidation(t *testing.T) {
	result, err := NormalizePollInput(PollInput{
		Question:      "Q?",
		Options:       []string{"A", "B"},
		DurationHours: 24.7,
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DurationHours != 24 {
		t.Errorf("durationHours: got %v, want 24 (floor)", result.DurationHours)
	}
}

func TestNormalizePollDurationHours_Clamp(t *testing.T) {
	cases := []struct {
		value, def, max, want float64
	}{
		{0, 24, 168, 24},    // value=0 → 使用 default
		{0.5, 24, 168, 1},   // value=0.5 → floor=0 → clamp 到 1
		{200, 24, 168, 168}, // 超过 max → clamp
		{48, 24, 168, 48},   // 正常值
		{1, 24, 168, 1},     // 最小值
		{168, 24, 168, 168}, // 最大值
	}
	for _, tc := range cases {
		got := NormalizePollDurationHours(tc.value, tc.def, tc.max)
		if got != tc.want {
			t.Errorf("NormalizePollDurationHours(%v, %v, %v) = %v, want %v",
				tc.value, tc.def, tc.max, got, tc.want)
		}
	}
}
