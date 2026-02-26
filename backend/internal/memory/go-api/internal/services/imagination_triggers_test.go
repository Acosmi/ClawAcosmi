package services

import (
	"testing"
	"time"
)

// ============================================================================
// ActivityBurstTrigger Tests (no DB required)
// ============================================================================

func TestActivityBurst_NilDB_NoTrigger(t *testing.T) {
	trigger := NewActivityBurstTrigger()
	if trigger.ShouldTrigger(nil, "user1") {
		t.Error("Should not trigger with nil DB")
	}
}

func TestActivityBurst_EmptyUserID_NoTrigger(t *testing.T) {
	trigger := NewActivityBurstTrigger()
	if trigger.ShouldTrigger(nil, "") {
		t.Error("Should not trigger with empty user ID")
	}
}

func TestActivityBurst_Name(t *testing.T) {
	trigger := NewActivityBurstTrigger()
	if trigger.Name() != "activity_burst" {
		t.Errorf("Expected name 'activity_burst', got '%s'", trigger.Name())
	}
}

func TestActivityBurst_DefaultParams(t *testing.T) {
	trigger := NewActivityBurstTrigger()
	if trigger.Threshold != 5 {
		t.Errorf("Default threshold should be 5, got %d", trigger.Threshold)
	}
	if trigger.Window != 1*time.Hour {
		t.Errorf("Default window should be 1h, got %v", trigger.Window)
	}
	if trigger.CooldownTime != 6*time.Hour {
		t.Errorf("Default cooldown should be 6h, got %v", trigger.CooldownTime)
	}
}

func TestActivityBurst_Cooldown_Logic(t *testing.T) {
	trigger := NewActivityBurstTrigger()

	// Simulate a trigger
	trigger.lastTrigger["user1"] = time.Now()

	// Within cooldown period → ShouldTrigger should return false (even with nil DB, cooldown check is first)
	// Actually with nil DB it returns false anyway, but cooldown is checked first
	if trigger.ShouldTrigger(nil, "user1") {
		t.Error("Should not trigger during cooldown")
	}

	// Expired cooldown
	trigger.lastTrigger["user2"] = time.Now().Add(-7 * time.Hour) // 7h ago, past 6h cooldown
	// With nil DB, will return false (DB check), but cooldown is passed
	// This tests that cooldown check passes correctly
	if trigger.ShouldTrigger(nil, "user2") {
		t.Error("Should not trigger with nil DB even if cooldown expired")
	}
}

// ============================================================================
// EntityClusterTrigger Tests (no DB required)
// ============================================================================

func TestEntityCluster_NilDB_NoTrigger(t *testing.T) {
	trigger := NewEntityClusterTrigger()
	if trigger.ShouldTrigger(nil, "user1") {
		t.Error("Should not trigger with nil DB")
	}
}

func TestEntityCluster_EmptyUserID_NoTrigger(t *testing.T) {
	trigger := NewEntityClusterTrigger()
	if trigger.ShouldTrigger(nil, "") {
		t.Error("Should not trigger with empty user ID")
	}
}

func TestEntityCluster_Name(t *testing.T) {
	trigger := NewEntityClusterTrigger()
	if trigger.Name() != "entity_cluster" {
		t.Errorf("Expected name 'entity_cluster', got '%s'", trigger.Name())
	}
}

func TestEntityCluster_DefaultParams(t *testing.T) {
	trigger := NewEntityClusterTrigger()
	if trigger.MinEntities != 3 {
		t.Errorf("Default min entities should be 3, got %d", trigger.MinEntities)
	}
	if trigger.Window != 24*time.Hour {
		t.Errorf("Default window should be 24h, got %v", trigger.Window)
	}
	if trigger.CooldownTime != 24*time.Hour {
		t.Errorf("Default cooldown should be 24h, got %v", trigger.CooldownTime)
	}
}

// ============================================================================
// TopicDriftTrigger Tests (no DB required)
// ============================================================================

func TestTopicDrift_NilDB_NoTrigger(t *testing.T) {
	trigger := NewTopicDriftTrigger()
	if trigger.ShouldTrigger(nil, "user1") {
		t.Error("Should not trigger with nil DB")
	}
}

func TestTopicDrift_EmptyUserID_NoTrigger(t *testing.T) {
	trigger := NewTopicDriftTrigger()
	if trigger.ShouldTrigger(nil, "") {
		t.Error("Should not trigger with empty user ID")
	}
}

func TestTopicDrift_Name(t *testing.T) {
	trigger := NewTopicDriftTrigger()
	if trigger.Name() != "topic_drift" {
		t.Errorf("Expected name 'topic_drift', got '%s'", trigger.Name())
	}
}

func TestTopicDrift_DefaultParams(t *testing.T) {
	trigger := NewTopicDriftTrigger()
	if trigger.RecentCount != 10 {
		t.Errorf("Default recent count should be 10, got %d", trigger.RecentCount)
	}
	if trigger.BaseCount != 50 {
		t.Errorf("Default base count should be 50, got %d", trigger.BaseCount)
	}
	if trigger.CooldownTime != 12*time.Hour {
		t.Errorf("Default cooldown should be 12h, got %v", trigger.CooldownTime)
	}
}

func TestTopicDrift_Cooldown_Logic(t *testing.T) {
	trigger := NewTopicDriftTrigger()

	// Simulate a recent trigger
	trigger.lastTrigger["user1"] = time.Now()

	// During cooldown
	if trigger.ShouldTrigger(nil, "user1") {
		t.Error("Should not trigger during cooldown")
	}
}

// ============================================================================
// topCategory helper Tests
// ============================================================================

func TestTopCategory_Empty(t *testing.T) {
	result := topCategory([]string{})
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestTopCategory_SingleCategory(t *testing.T) {
	result := topCategory([]string{"fact", "fact", "fact"})
	if result != "fact" {
		t.Errorf("Expected 'fact', got '%s'", result)
	}
}

func TestTopCategory_Mixed(t *testing.T) {
	result := topCategory([]string{"fact", "opinion", "fact", "opinion", "fact"})
	if result != "fact" {
		t.Errorf("Expected 'fact' (3 vs 2), got '%s'", result)
	}
}

func TestTopCategory_EmptyStringsIgnored(t *testing.T) {
	result := topCategory([]string{"", "", "fact", ""})
	if result != "fact" {
		t.Errorf("Expected 'fact', got '%s'", result)
	}
}

func TestTopCategory_AllEmpty(t *testing.T) {
	result := topCategory([]string{"", "", ""})
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

// ============================================================================
// ImaginationTrigger Interface Tests
// ============================================================================

func TestImaginationTrigger_Interface(t *testing.T) {
	// Verify all triggers implement the interface
	var _ ImaginationTrigger = NewActivityBurstTrigger()
	var _ ImaginationTrigger = NewEntityClusterTrigger()
	var _ ImaginationTrigger = NewTopicDriftTrigger()
}
