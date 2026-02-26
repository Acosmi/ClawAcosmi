package browser

import (
	"context"
	"fmt"
	"testing"
)

// mockPlanner is a test AIPlanner that returns a fixed sequence of actions.
type mockPlanner struct {
	actions []AIBrowseAction
	callIdx int
}

func (m *mockPlanner) Plan(_ context.Context, _ string, _ AIBrowseState) (*AIBrowseAction, error) {
	if m.callIdx >= len(m.actions) {
		return &AIBrowseAction{Type: "done", Reasoning: "no more actions"}, nil
	}
	action := m.actions[m.callIdx]
	m.callIdx++
	return &action, nil
}

func TestAIBrowseLoop_CompletesOnDone(t *testing.T) {
	tools := &StubPlaywrightTools{}
	planner := &mockPlanner{
		actions: []AIBrowseAction{
			{Type: "done", Reasoning: "goal achieved"},
		},
	}

	loop := NewAIBrowseLoop(tools, planner, AIBrowseLoopConfig{
		MaxSteps:          10,
		ScreenshotEnabled: false,
	})

	result, err := loop.Run(context.Background(), "test goal", PWTargetOpts{})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.StepsTaken != 1 {
		t.Errorf("StepsTaken = %d, want 1", result.StepsTaken)
	}
}

func TestAIBrowseLoop_RespectsMaxSteps(t *testing.T) {
	tools := &StubPlaywrightTools{}
	planner := &mockPlanner{
		actions: []AIBrowseAction{
			{Type: "wait", Reasoning: "waiting"},
			{Type: "wait", Reasoning: "still waiting"},
			{Type: "wait", Reasoning: "still waiting more"},
		},
	}

	loop := NewAIBrowseLoop(tools, planner, AIBrowseLoopConfig{
		MaxSteps:          2,
		ScreenshotEnabled: false,
	})

	result, err := loop.Run(context.Background(), "test goal", PWTargetOpts{})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when max steps reached")
	}
	if result.StepsTaken != 2 {
		t.Errorf("StepsTaken = %d, want 2", result.StepsTaken)
	}
}

func TestAIBrowseLoop_RecordsActions(t *testing.T) {
	tools := &StubPlaywrightTools{}
	planner := &mockPlanner{
		actions: []AIBrowseAction{
			{Type: "wait", Reasoning: "page loading"},
			{Type: "done", Reasoning: "complete"},
		},
	}

	loop := NewAIBrowseLoop(tools, planner, AIBrowseLoopConfig{
		MaxSteps:          10,
		ScreenshotEnabled: false,
	})

	result, err := loop.Run(context.Background(), "test", PWTargetOpts{})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if len(result.Actions) != 2 {
		t.Errorf("Actions count = %d, want 2", len(result.Actions))
	}
	if result.Actions[0].Type != "wait" {
		t.Errorf("first action = %q, want wait", result.Actions[0].Type)
	}
}

func TestAIBrowseLoop_MaxStepsClamped(t *testing.T) {
	loop := NewAIBrowseLoop(&StubPlaywrightTools{}, &mockPlanner{}, AIBrowseLoopConfig{
		MaxSteps: 200, // should be clamped to 100
	})
	if loop.config.MaxSteps != 100 {
		t.Errorf("MaxSteps = %d, want 100", loop.config.MaxSteps)
	}
}

// errorPlanner always returns an error.
type errorPlanner struct{}

func (e *errorPlanner) Plan(_ context.Context, _ string, _ AIBrowseState) (*AIBrowseAction, error) {
	return nil, fmt.Errorf("LLM unavailable")
}

func TestAIBrowseLoop_PlannerError(t *testing.T) {
	tools := &StubPlaywrightTools{}
	loop := NewAIBrowseLoop(tools, &errorPlanner{}, AIBrowseLoopConfig{
		MaxSteps:          5,
		ScreenshotEnabled: false,
	})

	result, err := loop.Run(context.Background(), "test", PWTargetOpts{})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.Success {
		t.Error("expected failure on planner error")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}
