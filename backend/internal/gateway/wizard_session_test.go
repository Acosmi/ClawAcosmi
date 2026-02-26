package gateway

import (
	"sync"
	"testing"
	"time"
)

// ---------- WizardSession 测试 ----------

func TestWizardSession_BasicFlow(t *testing.T) {
	// 简单的 intro → select → 完成 流程
	session := NewWizardSession(func(prompter WizardPrompter) error {
		if err := prompter.Intro("Test Wizard"); err != nil {
			return err
		}
		val, err := prompter.Select("Pick one", []WizardStepOption{
			{Value: "a", Label: "Option A"},
			{Value: "b", Label: "Option B"},
		}, "a")
		if err != nil {
			return err
		}
		if val != "b" {
			t.Errorf("expected 'b', got %v", val)
		}
		return nil
	})

	// Step 1: intro (note type)
	result := session.Next()
	if result.Done {
		t.Fatal("expected step, got done")
	}
	if result.Step == nil {
		t.Fatal("expected step, got nil")
	}
	if result.Step.Type != WizardStepNote {
		t.Errorf("expected note, got %s", result.Step.Type)
	}

	// Answer intro (note needs an answer to continue)
	if err := session.Answer(result.Step.ID, true); err != nil {
		t.Fatalf("answer intro: %v", err)
	}

	// Step 2: select
	result = session.Next()
	if result.Done {
		t.Fatal("expected select step, got done")
	}
	if result.Step.Type != WizardStepSelect {
		t.Errorf("expected select, got %s", result.Step.Type)
	}
	if len(result.Step.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(result.Step.Options))
	}

	// Answer select
	if err := session.Answer(result.Step.ID, "b"); err != nil {
		t.Fatalf("answer select: %v", err)
	}

	// Done
	result = session.Next()
	if !result.Done {
		t.Fatal("expected done")
	}
	if result.Status != WizardStatusDone {
		t.Errorf("expected done status, got %s", result.Status)
	}
}

func TestWizardSession_TextInput(t *testing.T) {
	session := NewWizardSession(func(prompter WizardPrompter) error {
		val, err := prompter.Text("Enter key", "sk-...", "", true)
		if err != nil {
			return err
		}
		if val != "sk-test123" {
			t.Errorf("expected 'sk-test123', got %v", val)
		}
		return nil
	})

	result := session.Next()
	if result.Done || result.Step == nil {
		t.Fatal("expected text step")
	}
	if result.Step.Type != WizardStepText {
		t.Errorf("expected text, got %s", result.Step.Type)
	}
	if !result.Step.Sensitive {
		t.Error("expected sensitive flag")
	}

	if err := session.Answer(result.Step.ID, "sk-test123"); err != nil {
		t.Fatalf("answer: %v", err)
	}

	result = session.Next()
	if !result.Done {
		t.Fatal("expected done")
	}
}

func TestWizardSession_Confirm(t *testing.T) {
	session := NewWizardSession(func(prompter WizardPrompter) error {
		ok, err := prompter.Confirm("Save?", false)
		if err != nil {
			return err
		}
		if !ok {
			t.Error("expected true")
		}
		return nil
	})

	result := session.Next()
	if result.Done || result.Step == nil {
		t.Fatal("expected confirm step")
	}
	if result.Step.Type != WizardStepConfirm {
		t.Errorf("expected confirm, got %s", result.Step.Type)
	}

	if err := session.Answer(result.Step.ID, true); err != nil {
		t.Fatalf("answer: %v", err)
	}

	result = session.Next()
	if !result.Done {
		t.Fatal("expected done")
	}
}

func TestWizardSession_Cancel(t *testing.T) {
	session := NewWizardSession(func(prompter WizardPrompter) error {
		_, err := prompter.Text("Enter something", "", "", false)
		return err
	})

	result := session.Next()
	if result.Done || result.Step == nil {
		t.Fatal("expected step")
	}

	session.Cancel()
	if session.GetStatus() != WizardStatusCancelled {
		t.Errorf("expected cancelled, got %s", session.GetStatus())
	}

	result = session.Next()
	if !result.Done {
		t.Fatal("expected done after cancel")
	}
	if result.Status != WizardStatusCancelled {
		t.Errorf("expected cancelled status, got %s", result.Status)
	}
}

func TestWizardSession_RunnerError(t *testing.T) {
	session := NewWizardSession(func(prompter WizardPrompter) error {
		return &WizardCancelledError{}
	})

	// Give the goroutine time to complete
	time.Sleep(50 * time.Millisecond)

	result := session.Next()
	if !result.Done {
		t.Fatal("expected done")
	}
	if result.Status != WizardStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.Status)
	}
}

func TestWizardSession_DoubleAnswer(t *testing.T) {
	session := NewWizardSession(func(prompter WizardPrompter) error {
		_, _ = prompter.Text("test", "", "", false)
		return nil
	})

	result := session.Next()
	if result.Step == nil {
		t.Fatal("expected step")
	}
	stepID := result.Step.ID

	if err := session.Answer(stepID, "ok"); err != nil {
		t.Fatalf("first answer: %v", err)
	}

	// Second answer should fail
	err := session.Answer(stepID, "fail")
	if err == nil {
		t.Error("expected error on double answer")
	}
}

// ---------- WizardSessionTracker 测试 ----------

func TestWizardSessionTracker_SetGetDelete(t *testing.T) {
	tracker := NewWizardSessionTracker()

	session := NewWizardSession(func(p WizardPrompter) error {
		_, _ = p.Text("test", "", "", false)
		return nil
	})

	tracker.Set("test-1", session)
	if got := tracker.Get("test-1"); got == nil {
		t.Error("expected session")
	}
	if got := tracker.Get("missing"); got != nil {
		t.Error("expected nil for missing")
	}

	tracker.Delete("test-1")
	if got := tracker.Get("test-1"); got != nil {
		t.Error("expected nil after delete")
	}
}

func TestWizardSessionTracker_FindRunning(t *testing.T) {
	tracker := NewWizardSessionTracker()

	// 创建一个运行中的会话
	session := NewWizardSession(func(p WizardPrompter) error {
		_, _ = p.Text("test", "", "", false)
		return nil
	})
	tracker.Set("running-1", session)

	id := tracker.FindRunning()
	if id != "running-1" {
		t.Errorf("expected running-1, got %s", id)
	}

	// 取消后应该找不到
	session.Cancel()
	id = tracker.FindRunning()
	if id != "" {
		t.Errorf("expected empty after cancel, got %s", id)
	}
}

func TestWizardSessionTracker_Purge(t *testing.T) {
	tracker := NewWizardSessionTracker()

	// 完成的会话可以 purge
	done := NewWizardSession(func(p WizardPrompter) error {
		return nil
	})
	time.Sleep(50 * time.Millisecond) // 等 runner 完成
	tracker.Set("done-1", done)
	tracker.Purge("done-1")
	if got := tracker.Get("done-1"); got != nil {
		t.Error("expected nil after purge")
	}

	// 运行中的会话不能 purge
	running := NewWizardSession(func(p WizardPrompter) error {
		_, _ = p.Text("test", "", "", false)
		return nil
	})
	tracker.Set("running-1", running)
	tracker.Purge("running-1")
	if got := tracker.Get("running-1"); got == nil {
		t.Error("expected session to survive purge while running")
	}
	running.Cancel()
}

func TestWizardSessionTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewWizardSessionTracker()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := NewWizardSession(func(p WizardPrompter) error { return nil })
			time.Sleep(20 * time.Millisecond)
			id := "s-" + string(rune('0'+idx))
			tracker.Set(id, s)
			_ = tracker.Get(id)
			_ = tracker.FindRunning()
			tracker.Purge(id)
		}(i)
	}
	wg.Wait()
}

// ---------- 辅助函数测试 ----------

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "(from environment)"},
		{"abc", "•••"},
		{"sk-abc123def456", "sk-a•••••••f456"},
	}
	for _, tc := range tests {
		got := maskAPIKey(tc.input)
		if got != tc.expected {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
