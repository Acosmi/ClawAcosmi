package channels

import (
	"errors"
	"testing"
)

func TestAsSendError(t *testing.T) {
	t.Parallel()

	cause := errors.New("upstream timeout")
	original := WrapSendError(ChannelWeCom, SendErrUpstream, "send.media", "media send failed", cause).
		WithRetryable(true).
		WithDetails(map[string]interface{}{"to": "u123"})

	got, ok := AsSendError(original)
	if !ok || got == nil {
		t.Fatalf("AsSendError should resolve SendError")
	}
	if got.Channel != ChannelWeCom {
		t.Fatalf("channel mismatch: got=%s", got.Channel)
	}
	if got.Code != SendErrUpstream {
		t.Fatalf("code mismatch: got=%s", got.Code)
	}
	if !got.Retryable {
		t.Fatalf("retryable should be true")
	}
	if got.Details == nil || got.Details["to"] != "u123" {
		t.Fatalf("details missing: %+v", got.Details)
	}
	if !errors.Is(original, cause) {
		t.Fatalf("wrapped cause should be discoverable")
	}
}

func TestSendErrorErrorString(t *testing.T) {
	t.Parallel()

	err := WrapSendError(ChannelDingTalk, SendErrUnavailable, "send.init", "sender unavailable", errors.New("dial failed"))
	got := err.Error()
	if got == "" {
		t.Fatalf("error string should not be empty")
	}
	if got == "sender unavailable" {
		t.Fatalf("error string should include operation/cause context, got=%q", got)
	}
}

func TestSendErrorExportedAccessors(t *testing.T) {
	t.Parallel()

	err := NewSendError(ChannelDingTalk, SendErrUnsupportedFeature, "unsupported").
		WithOperation("send.media.upload").
		WithRetryable(true)
	if err.SendCode() != string(SendErrUnsupportedFeature) {
		t.Fatalf("SendCode mismatch: %q", err.SendCode())
	}
	if err.SendChannel() != string(ChannelDingTalk) {
		t.Fatalf("SendChannel mismatch: %q", err.SendChannel())
	}
	if err.SendOperation() != "send.media.upload" {
		t.Fatalf("SendOperation mismatch: %q", err.SendOperation())
	}
	if !err.SendRetryable() {
		t.Fatalf("SendRetryable should be true")
	}
	if err.SendUserMessage() != "unsupported" {
		t.Fatalf("SendUserMessage mismatch: %q", err.SendUserMessage())
	}
}
