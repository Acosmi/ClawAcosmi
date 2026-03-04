package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
)

type fakeRemoteApprovalProvider struct {
	name        string
	validateErr error
	sendErr     error
}

func (p *fakeRemoteApprovalProvider) Name() string { return p.name }

func (p *fakeRemoteApprovalProvider) ValidateConfig() error { return p.validateErr }

func (p *fakeRemoteApprovalProvider) SendApprovalRequest(ctx context.Context, req ApprovalCardRequest) error {
	return p.sendErr
}

func TestHandleRemoteApprovalTest_MapsValidationErrorAsSendError(t *testing.T) {
	t.Parallel()

	notifier := &RemoteApprovalNotifier{
		config: RemoteApprovalConfig{
			Enabled:     true,
			CallbackURL: "https://example.com/callback",
		},
		providers: []RemoteApprovalProvider{
			&fakeRemoteApprovalProvider{
				name:        "feishu",
				validateErr: errors.New("missing app secret"),
			},
		},
	}

	registry := NewMethodRegistry()
	registry.RegisterAll(RemoteApprovalHandlers())

	req := &RequestFrame{
		Method: "security.remoteApproval.test",
		Params: map[string]interface{}{
			"provider": "feishu",
		},
	}

	var gotOK bool
	var gotErr *ErrorShape
	HandleGatewayRequest(registry, req, nil, &GatewayMethodContext{
		RemoteApprovalNotifier: notifier,
	}, func(ok bool, payload interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	})

	if gotOK {
		t.Fatalf("expected test call to fail on invalid config")
	}
	if gotErr == nil {
		t.Fatalf("expected error shape")
	}
	if gotErr.Code != ErrCodeBadRequest {
		t.Fatalf("unexpected error code: %s", gotErr.Code)
	}
	details, ok := gotErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("expected details map, got %T", gotErr.Details)
	}
	if details["sendCode"] != string(channels.SendErrInvalidRequest) {
		t.Fatalf("unexpected sendCode: %+v", details)
	}
}

func TestHandleRemoteApprovalTest_MapsUpstreamSendError(t *testing.T) {
	t.Parallel()

	notifier := &RemoteApprovalNotifier{
		config: RemoteApprovalConfig{
			Enabled:     true,
			CallbackURL: "https://example.com/callback",
		},
		providers: []RemoteApprovalProvider{
			&fakeRemoteApprovalProvider{
				name:    "dingtalk",
				sendErr: errors.New("webhook timeout"),
			},
		},
	}

	registry := NewMethodRegistry()
	registry.RegisterAll(RemoteApprovalHandlers())

	req := &RequestFrame{
		Method: "security.remoteApproval.test",
		Params: map[string]interface{}{
			"provider": "dingtalk",
		},
	}

	var gotOK bool
	var gotErr *ErrorShape
	HandleGatewayRequest(registry, req, nil, &GatewayMethodContext{
		RemoteApprovalNotifier: notifier,
	}, func(ok bool, payload interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	})

	if gotOK {
		t.Fatalf("expected test call to fail on upstream send error")
	}
	if gotErr == nil {
		t.Fatalf("expected error shape")
	}
	if gotErr.Code != ErrCodeServiceUnavailable {
		t.Fatalf("unexpected error code: %s", gotErr.Code)
	}
	if gotErr.Retryable == nil || !*gotErr.Retryable {
		t.Fatalf("expected retryable send error")
	}
	details, ok := gotErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("expected details map, got %T", gotErr.Details)
	}
	if details["sendCode"] != string(channels.SendErrUpstream) {
		t.Fatalf("unexpected sendCode: %+v", details)
	}
	if details["channel"] != string(channels.ChannelDingTalk) {
		t.Fatalf("unexpected channel detail: %+v", details)
	}
}

func TestRemoteApprovalNotifier_TestProvider_MissingProviderReturnsInvalidTarget(t *testing.T) {
	t.Parallel()

	notifier := &RemoteApprovalNotifier{
		config: RemoteApprovalConfig{
			Enabled:     true,
			CallbackURL: "https://example.com/callback",
		},
		providers: []RemoteApprovalProvider{
			&fakeRemoteApprovalProvider{name: "wecom"},
		},
	}

	err := notifier.TestProvider("feishu")
	if err == nil {
		t.Fatalf("expected missing provider error")
	}
	sendErr, ok := channels.AsSendError(err)
	if !ok || sendErr == nil {
		t.Fatalf("expected SendError, got %v", err)
	}
	if sendErr.Code != channels.SendErrInvalidTarget {
		t.Fatalf("unexpected send error code: %s", sendErr.Code)
	}
	if sendErr.Operation != "remote_approval.resolve_provider" {
		t.Fatalf("unexpected operation: %s", sendErr.Operation)
	}
}

func TestRemoteApprovalNotifier_TestProvider_PassesThroughStructuredError(t *testing.T) {
	t.Parallel()

	structuredErr := channels.NewSendError(channels.ChannelWeCom, channels.SendErrUnauthorized,
		"token invalid").WithOperation("wecom.auth")
	notifier := &RemoteApprovalNotifier{
		config: RemoteApprovalConfig{
			Enabled:     true,
			CallbackURL: "https://example.com/callback",
		},
		providers: []RemoteApprovalProvider{
			&fakeRemoteApprovalProvider{
				name:    "wecom",
				sendErr: structuredErr,
			},
		},
	}

	err := notifier.TestProvider("wecom")
	if err == nil {
		t.Fatalf("expected error")
	}
	sendErr, ok := channels.AsSendError(err)
	if !ok || sendErr == nil {
		t.Fatalf("expected SendError, got %v", err)
	}
	if sendErr.Code != channels.SendErrUnauthorized {
		t.Fatalf("unexpected send error code: %s", sendErr.Code)
	}
}

func TestRemoteApprovalNotifier_TestProvider_Success(t *testing.T) {
	t.Parallel()

	notifier := &RemoteApprovalNotifier{
		config: RemoteApprovalConfig{
			Enabled:     true,
			CallbackURL: "https://example.com/callback",
		},
		providers: []RemoteApprovalProvider{
			&fakeRemoteApprovalProvider{
				name: "feishu",
			},
		},
	}

	start := time.Now()
	if err := notifier.TestProvider("feishu"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("test provider should not block in fake provider")
	}
}
