package channels

import (
	"errors"
	"testing"
)

type sendCapablePlugin struct {
	id      ChannelID
	sendFn  func(params OutboundSendParams) (*OutboundSendResult, error)
	started bool
	stopped bool
}

func (p *sendCapablePlugin) ID() ChannelID { return p.id }
func (p *sendCapablePlugin) Start(string) error {
	p.started = true
	return nil
}
func (p *sendCapablePlugin) Stop(string) error {
	p.stopped = true
	return nil
}
func (p *sendCapablePlugin) SendMessage(params OutboundSendParams) (*OutboundSendResult, error) {
	if p.sendFn != nil {
		return p.sendFn(params)
	}
	return &OutboundSendResult{Channel: string(p.id), ChatID: params.To}, nil
}

func TestManagerSendMessagePluginNotRegistered(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	_, err := mgr.SendMessage(ChannelDingTalk, OutboundSendParams{To: "u1", Text: "hello"})
	if err == nil {
		t.Fatalf("expected error")
	}

	sendErr, ok := AsSendError(err)
	if !ok || sendErr == nil {
		t.Fatalf("expected SendError, got %T", err)
	}
	if sendErr.Code != SendErrUnavailable {
		t.Fatalf("code=%s want=%s", sendErr.Code, SendErrUnavailable)
	}
	if sendErr.Operation != "manager.resolve_plugin" {
		t.Fatalf("operation=%s", sendErr.Operation)
	}
}

func TestManagerSendMessageUnsupportedSender(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	mgr.RegisterPlugin(newMockPlugin(ChannelWeCom))

	_, err := mgr.SendMessage(ChannelWeCom, OutboundSendParams{To: "u1", Text: "hello"})
	if err == nil {
		t.Fatalf("expected error")
	}

	sendErr, ok := AsSendError(err)
	if !ok || sendErr == nil {
		t.Fatalf("expected SendError, got %T", err)
	}
	if sendErr.Code != SendErrUnsupportedFeature {
		t.Fatalf("code=%s want=%s", sendErr.Code, SendErrUnsupportedFeature)
	}
}

func TestManagerSendMessageWrapsPlainPluginError(t *testing.T) {
	t.Parallel()

	cause := errors.New("network down")
	mgr := NewManager()
	mgr.RegisterPlugin(&sendCapablePlugin{
		id: ChannelDingTalk,
		sendFn: func(params OutboundSendParams) (*OutboundSendResult, error) {
			return nil, cause
		},
	})

	_, err := mgr.SendMessage(ChannelDingTalk, OutboundSendParams{To: "u1", Text: "hello"})
	if err == nil {
		t.Fatalf("expected error")
	}

	sendErr, ok := AsSendError(err)
	if !ok || sendErr == nil {
		t.Fatalf("expected SendError, got %T", err)
	}
	if sendErr.Code != SendErrUpstream {
		t.Fatalf("code=%s want=%s", sendErr.Code, SendErrUpstream)
	}
	if sendErr.Operation != "manager.send" {
		t.Fatalf("operation=%s", sendErr.Operation)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause")
	}
}

func TestManagerSendMessagePassesThroughSendError(t *testing.T) {
	t.Parallel()

	expected := NewSendError(ChannelWeCom, SendErrInvalidRequest, "bad payload").WithOperation("plugin.validate")
	mgr := NewManager()
	mgr.RegisterPlugin(&sendCapablePlugin{
		id: ChannelWeCom,
		sendFn: func(params OutboundSendParams) (*OutboundSendResult, error) {
			return nil, expected
		},
	})

	_, err := mgr.SendMessage(ChannelWeCom, OutboundSendParams{To: "u1", Text: "hello"})
	if err == nil {
		t.Fatalf("expected error")
	}

	sendErr, ok := AsSendError(err)
	if !ok || sendErr == nil {
		t.Fatalf("expected SendError, got %T", err)
	}
	if sendErr.Code != SendErrInvalidRequest {
		t.Fatalf("code=%s want=%s", sendErr.Code, SendErrInvalidRequest)
	}
	if sendErr.Operation != "plugin.validate" {
		t.Fatalf("operation=%s", sendErr.Operation)
	}
}

func TestManagerSendMessageSuccess(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	mgr.RegisterPlugin(&sendCapablePlugin{
		id: ChannelDingTalk,
		sendFn: func(params OutboundSendParams) (*OutboundSendResult, error) {
			return &OutboundSendResult{
				Channel: string(ChannelDingTalk),
				ChatID:  params.To,
			}, nil
		},
	})

	result, err := mgr.SendMessage(ChannelDingTalk, OutboundSendParams{To: "cid-1", Text: "hello"})
	if err != nil {
		t.Fatalf("send should succeed: %v", err)
	}
	if result == nil || result.ChatID != "cid-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
