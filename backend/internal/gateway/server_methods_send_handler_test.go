package gateway

import (
	"encoding/base64"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
)

type fakeChannelSendPlugin struct {
	id         channels.ChannelID
	sendErr    error
	sendResult *channels.OutboundSendResult
}

func (p *fakeChannelSendPlugin) ID() channels.ChannelID { return p.id }

func (p *fakeChannelSendPlugin) Start(accountID string) error { return nil }

func (p *fakeChannelSendPlugin) Stop(accountID string) error { return nil }

func (p *fakeChannelSendPlugin) SendMessage(params channels.OutboundSendParams) (*channels.OutboundSendResult, error) {
	if p.sendErr != nil {
		return nil, p.sendErr
	}
	if p.sendResult != nil {
		return p.sendResult, nil
	}
	return &channels.OutboundSendResult{
		Channel: string(p.id),
		ChatID:  params.To,
	}, nil
}

type fakeOutboundPipeWithErr struct {
	pollErr error
}

func (f *fakeOutboundPipeWithErr) ResolveTarget(channelID string, to string, accountID string) (*OutboundResolvedTarget, error) {
	return &OutboundResolvedTarget{ChannelID: channelID, To: to, AccountID: accountID}, nil
}

func (f *fakeOutboundPipeWithErr) EnsureSessionRoute(target *OutboundResolvedTarget, sessionKey string) error {
	return nil
}

func (f *fakeOutboundPipeWithErr) Deliver(target *OutboundResolvedTarget, message string, mediaURLs []string, opts *OutboundDeliverOpts) (*OutboundDeliverResult, error) {
	return &OutboundDeliverResult{MessageID: "msg-deliver", ChatID: target.To, OK: true}, nil
}

func (f *fakeOutboundPipeWithErr) SendPoll(channelID string, question string, options []string, to string) (*OutboundPollResult, error) {
	if f.pollErr != nil {
		return nil, f.pollErr
	}
	return &OutboundPollResult{PollID: "poll-1", OK: true}, nil
}

func TestHandleSend_ChannelMgrFallbackMapsSendError(t *testing.T) {
	t.Parallel()

	mgr := channels.NewManager()
	mgr.RegisterPlugin(&fakeChannelSendPlugin{
		id: channels.ChannelDingTalk,
		sendErr: channels.NewSendError(channels.ChannelDingTalk, channels.SendErrUnsupportedFeature,
			"binary media not supported").WithOperation("send.media"),
	})

	registry := NewMethodRegistry()
	registry.RegisterAll(SendHandlers())

	req := &RequestFrame{
		Method: "send",
		Params: map[string]interface{}{
			"message": "hello",
			"to":      "cid_group_001",
			"channel": "dingtalk",
		},
	}

	var gotOK bool
	var gotErr *ErrorShape
	HandleGatewayRequest(registry, req, nil, &GatewayMethodContext{
		ChannelMgr: mgr,
	}, func(ok bool, payload interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	})

	if gotOK {
		t.Fatalf("send should fail when plugin returns structured error")
	}
	if gotErr == nil {
		t.Fatalf("expected error shape")
	}
	if gotErr.Code != ErrCodeUnsupportedFeature {
		t.Fatalf("unexpected error code: %s", gotErr.Code)
	}
	details, ok := gotErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details type mismatch: %T", gotErr.Details)
	}
	if details["sendCode"] != string(channels.SendErrUnsupportedFeature) {
		t.Fatalf("sendCode mismatch: %+v", details)
	}
}

func TestHandleSend_Base64PathMapsSendError(t *testing.T) {
	t.Parallel()

	mgr := channels.NewManager()
	mgr.RegisterPlugin(&fakeChannelSendPlugin{
		id: channels.ChannelDingTalk,
		sendErr: channels.NewSendError(channels.ChannelDingTalk, channels.SendErrUnauthorized,
			"token invalid").WithOperation("send.auth"),
	})

	registry := NewMethodRegistry()
	registry.RegisterAll(SendHandlers())

	req := &RequestFrame{
		Method: "send",
		Params: map[string]interface{}{
			"message":       "hello",
			"to":            "cid_group_001",
			"channel":       "dingtalk",
			"mediaBase64":   base64.StdEncoding.EncodeToString([]byte("png")),
			"mediaMimeType": "image/png",
		},
	}

	var gotOK bool
	var gotErr *ErrorShape
	HandleGatewayRequest(registry, req, nil, &GatewayMethodContext{
		ChannelMgr: mgr,
	}, func(ok bool, payload interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	})

	if gotOK {
		t.Fatalf("send should fail when base64 path plugin returns structured error")
	}
	if gotErr == nil {
		t.Fatalf("expected error shape")
	}
	if gotErr.Code != ErrCodeUnauthorized {
		t.Fatalf("unexpected error code: %s", gotErr.Code)
	}
}

func TestHandlePoll_MapsSendError(t *testing.T) {
	t.Parallel()

	registry := NewMethodRegistry()
	registry.RegisterAll(SendHandlers())

	req := &RequestFrame{
		Method: "poll",
		Params: map[string]interface{}{
			"to":       "cid_group_001",
			"question": "是否继续？",
			"options":  []interface{}{"是", "否"},
			"channel":  "dingtalk",
		},
	}

	var gotOK bool
	var gotErr *ErrorShape
	HandleGatewayRequest(registry, req, nil, &GatewayMethodContext{
		OutboundPipe: &fakeOutboundPipeWithErr{
			pollErr: channels.NewSendError(channels.ChannelDingTalk, channels.SendErrUnavailable,
				"poll service unavailable").WithRetryable(true),
		},
	}, func(ok bool, payload interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	})

	if gotOK {
		t.Fatalf("poll should fail when outbound pipeline returns structured error")
	}
	if gotErr == nil {
		t.Fatalf("expected error shape")
	}
	if gotErr.Code != ErrCodeServiceUnavailable {
		t.Fatalf("unexpected error code: %s", gotErr.Code)
	}
	if gotErr.Retryable == nil || !*gotErr.Retryable {
		t.Fatalf("expected retryable shape for unavailable error")
	}
}

func TestHandleSend_SuccessWithFakePlugin(t *testing.T) {
	t.Parallel()

	mgr := channels.NewManager()
	mgr.RegisterPlugin(&fakeChannelSendPlugin{
		id: channels.ChannelDingTalk,
		sendResult: &channels.OutboundSendResult{
			Channel: string(channels.ChannelDingTalk),
			ChatID:  "cid_group_001",
		},
	})

	registry := NewMethodRegistry()
	registry.RegisterAll(SendHandlers())

	req := &RequestFrame{
		Method: "send",
		Params: map[string]interface{}{
			"message": "hello",
			"to":      "cid_group_001",
			"channel": "dingtalk",
		},
	}

	var gotOK bool
	var gotPayload interface{}
	HandleGatewayRequest(registry, req, nil, &GatewayMethodContext{
		ChannelMgr: mgr,
	}, func(ok bool, payload interface{}, err *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	})

	if !gotOK {
		t.Fatalf("send should succeed")
	}
	payloadMap, ok := gotPayload.(map[string]interface{})
	if !ok {
		t.Fatalf("payload type mismatch: %T", gotPayload)
	}
	if payloadMap["channel"] != "dingtalk" {
		t.Fatalf("unexpected payload channel: %+v", payloadMap)
	}
}

var _ channels.Plugin = (*fakeChannelSendPlugin)(nil)
var _ channels.MessageSender = (*fakeChannelSendPlugin)(nil)
var _ OutboundPipeline = (*fakeOutboundPipeWithErr)(nil)
