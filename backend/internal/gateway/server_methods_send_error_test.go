package gateway

import (
	"errors"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
)

func TestMapSendErrorToShape(t *testing.T) {
	t.Parallel()

	sendErr := channels.WrapSendError(
		channels.ChannelDingTalk,
		channels.SendErrUnsupportedFeature,
		"send.media",
		"binary media not supported",
		errors.New("unsupported"),
	).WithDetails(map[string]interface{}{
		"to": "cid-123",
	})

	shape := mapSendErrorToShape(sendErr)
	if shape == nil {
		t.Fatalf("shape should not be nil")
	}
	if shape.Code != ErrCodeUnsupportedFeature {
		t.Fatalf("shape.Code=%s want=%s", shape.Code, ErrCodeUnsupportedFeature)
	}
	if shape.Message == "" {
		t.Fatalf("shape.Message should not be empty")
	}
	details, ok := shape.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("shape.Details type mismatch: %T", shape.Details)
	}
	if details["sendCode"] != string(channels.SendErrUnsupportedFeature) {
		t.Fatalf("sendCode mismatch: %+v", details)
	}
	if details["channel"] != string(channels.ChannelDingTalk) {
		t.Fatalf("channel mismatch: %+v", details)
	}
	if details["operation"] != "send.media" {
		t.Fatalf("operation mismatch: %+v", details)
	}
}

func TestMapSendErrorToShapeRetryable(t *testing.T) {
	t.Parallel()

	sendErr := channels.NewSendError(channels.ChannelWeCom, channels.SendErrUnavailable, "downstream unavailable").
		WithRetryable(true)

	shape := mapSendErrorToShape(sendErr)
	if shape == nil {
		t.Fatalf("shape should not be nil")
	}
	if shape.Code != ErrCodeServiceUnavailable {
		t.Fatalf("shape.Code=%s want=%s", shape.Code, ErrCodeServiceUnavailable)
	}
	if shape.Retryable == nil || !*shape.Retryable {
		t.Fatalf("retryable flag expected true")
	}
	if shape.RetryAfterMs == nil || *shape.RetryAfterMs <= 0 {
		t.Fatalf("retryAfterMs should be set")
	}
}

func TestMapSendErrorToShapeFallback(t *testing.T) {
	t.Parallel()

	shape := mapSendErrorToShape(errors.New("plain error"))
	if shape == nil {
		t.Fatalf("shape should not be nil")
	}
	if shape.Code != ErrCodeInternalError {
		t.Fatalf("shape.Code=%s want=%s", shape.Code, ErrCodeInternalError)
	}
}
