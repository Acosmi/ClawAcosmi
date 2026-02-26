package gateway

import (
	"testing"

	"github.com/anthropic/open-acosmi/internal/session"
)

// TS 对照: delivery-context.ts 单元测试

func TestNormalizeDeliveryContext_Nil(t *testing.T) {
	if got := NormalizeDeliveryContext(nil); got != nil {
		t.Errorf("NormalizeDeliveryContext(nil) = %v; want nil", got)
	}
}

func TestNormalizeDeliveryContext_Empty(t *testing.T) {
	dc := &session.DeliveryContext{}
	if got := NormalizeDeliveryContext(dc); got != nil {
		t.Errorf("NormalizeDeliveryContext(empty) = %v; want nil", got)
	}
}

func TestNormalizeDeliveryContext_WithChannel(t *testing.T) {
	dc := &session.DeliveryContext{Channel: "telegram", ThreadId: "42"}
	got := NormalizeDeliveryContext(dc)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Channel != "telegram" {
		t.Errorf("Channel = %q", got.Channel)
	}
	if got.ThreadId != "42" {
		t.Errorf("ThreadId = %v", got.ThreadId)
	}
}

func TestMergeDeliveryContext_PrimaryWins(t *testing.T) {
	primary := &session.DeliveryContext{Channel: "telegram", To: "user-1"}
	secondary := &session.DeliveryContext{Channel: "slack", To: "user-2", AccountId: "acc-2"}
	got := MergeDeliveryContext(primary, secondary)
	if got.Channel != "telegram" {
		t.Errorf("Channel = %q; want telegram", got.Channel)
	}
	if got.To != "user-1" {
		t.Errorf("To = %q; want user-1", got.To)
	}
	if got.AccountId != "acc-2" {
		t.Errorf("AccountId = %q; want acc-2 (from secondary)", got.AccountId)
	}
}

func TestMergeDeliveryContext_NilPrimary(t *testing.T) {
	secondary := &session.DeliveryContext{Channel: "slack"}
	got := MergeDeliveryContext(nil, secondary)
	if got.Channel != "slack" {
		t.Errorf("Channel = %q; want slack", got.Channel)
	}
}

func TestMergeDeliveryContext_BothNil(t *testing.T) {
	if got := MergeDeliveryContext(nil, nil); got != nil {
		t.Errorf("expected nil")
	}
}

func TestRemoveThreadFromDeliveryContext(t *testing.T) {
	dc := &session.DeliveryContext{Channel: "telegram", ThreadId: "42", To: "user"}
	got := RemoveThreadFromDeliveryContext(dc)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.ThreadId != nil {
		t.Errorf("ThreadId should be nil, got %v", got.ThreadId)
	}
	if got.Channel != "telegram" {
		t.Errorf("Channel = %q", got.Channel)
	}
	if got.To != "user" {
		t.Errorf("To = %q", got.To)
	}
}

func TestRemoveThreadFromDeliveryContext_Nil(t *testing.T) {
	if got := RemoveThreadFromDeliveryContext(nil); got != nil {
		t.Errorf("expected nil")
	}
}

func TestDeliveryContextFromSession_WithDC(t *testing.T) {
	entry := &SessionEntry{
		SessionKey: "test",
		DeliveryContext: &session.DeliveryContext{
			Channel:   "telegram",
			To:        "user-1",
			AccountId: "acc-1",
			ThreadId:  "42",
		},
	}
	got := DeliveryContextFromSession(entry)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Channel != "telegram" {
		t.Errorf("Channel = %q", got.Channel)
	}
}

func TestDeliveryContextFromSession_FallbackToLegacy(t *testing.T) {
	entry := &SessionEntry{
		SessionKey: "test",
		LastChannel: &session.SessionLastChannel{
			Channel: "slack",
		},
		LastTo:        "user-2",
		LastAccountId: "acc-2",
		LastThreadId:  "99",
	}
	got := DeliveryContextFromSession(entry)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Channel != "slack" {
		t.Errorf("Channel = %q; want slack", got.Channel)
	}
	if got.To != "user-2" {
		t.Errorf("To = %q; want user-2", got.To)
	}
}

func TestDeliveryContextFromSession_Nil(t *testing.T) {
	if got := DeliveryContextFromSession(nil); got != nil {
		t.Errorf("expected nil")
	}
}

func TestComputeDeliveryFields_WithChannel(t *testing.T) {
	dc := &session.DeliveryContext{
		Channel:   "telegram",
		To:        "user-1",
		AccountId: "acc-1",
		ThreadId:  "42",
	}
	result := ComputeDeliveryFields(dc)
	if result.DeliveryContext == nil {
		t.Fatal("expected non-nil DC")
	}
	if result.LastChannel == nil || result.LastChannel.Channel != "telegram" {
		t.Error("LastChannel should be set")
	}
	if result.LastTo != "user-1" {
		t.Errorf("LastTo = %q", result.LastTo)
	}
	if result.LastThreadId != "42" {
		t.Errorf("LastThreadId = %v", result.LastThreadId)
	}
}

func TestComputeDeliveryFields_Nil(t *testing.T) {
	result := ComputeDeliveryFields(nil)
	if result.DeliveryContext != nil {
		t.Error("expected nil DC")
	}
	if result.LastChannel != nil {
		t.Error("expected nil LastChannel")
	}
}
