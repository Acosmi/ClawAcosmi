package bridge

import (
	"context"
	"encoding/json"
	"testing"
)

// ── Mock: Telegram ────────────────────────────────────────────────────

type mockTelegramDeps struct {
	sendMsgCalled bool
	lastTo        string
	lastText      string
}

func (m *mockTelegramDeps) SendMessage(_ context.Context, to, text string, _ TelegramBridgeSendOpts) (string, string, error) {
	m.sendMsgCalled = true
	m.lastTo = to
	m.lastText = text
	return "123", "456", nil
}
func (m *mockTelegramDeps) EditMessage(_ context.Context, _ string, _ int, _ string, _ [][]TelegramButton) error {
	return nil
}
func (m *mockTelegramDeps) DeleteMessage(_ context.Context, _ string, _ int) error { return nil }
func (m *mockTelegramDeps) ReactMessage(_ context.Context, _ string, _ int, _ string, _ bool) error {
	return nil
}
func (m *mockTelegramDeps) SendSticker(_ context.Context, _, _ string) (string, string, error) {
	return "s1", "c1", nil
}
func (m *mockTelegramDeps) SearchStickers(_ string, _ int) []StickerResult { return nil }
func (m *mockTelegramDeps) GetStickerCacheStats() map[string]interface{} {
	return map[string]interface{}{"total": 0}
}
func (m *mockTelegramDeps) CallAPI(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func allowAll(_ string) bool { return true }
func denyAll(_ string) bool  { return false }

func TestTelegramSendMessage(t *testing.T) {
	deps := &mockTelegramDeps{}
	params := map[string]interface{}{
		"action":  "sendMessage",
		"to":      "12345",
		"content": "hello",
	}
	result, err := HandleTelegramAction(context.Background(), params, allowAll, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success result")
	}
	if !deps.sendMsgCalled {
		t.Fatal("expected SendMessage to be called")
	}
	if deps.lastTo != "12345" || deps.lastText != "hello" {
		t.Fatalf("unexpected params: to=%s text=%s", deps.lastTo, deps.lastText)
	}
}

func TestTelegramActionGateBlocks(t *testing.T) {
	deps := &mockTelegramDeps{}
	params := map[string]interface{}{
		"action":  "sendMessage",
		"to":      "12345",
		"content": "hello",
	}
	_, err := HandleTelegramAction(context.Background(), params, denyAll, deps)
	if err == nil {
		t.Fatal("expected error when action gate blocks")
	}
}

func TestTelegramUnknownAction(t *testing.T) {
	deps := &mockTelegramDeps{}
	params := map[string]interface{}{"action": "nonexistent"}
	_, err := HandleTelegramAction(context.Background(), params, allowAll, deps)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestTelegramReact(t *testing.T) {
	deps := &mockTelegramDeps{}
	params := map[string]interface{}{
		"action":    "react",
		"chatId":    "111",
		"messageId": float64(222),
		"emoji":     "👍",
	}
	result, err := HandleTelegramAction(context.Background(), params, allowAll, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected ok")
	}
}

// ── Mock: Slack ───────────────────────────────────────────────────────

type mockSlackDeps struct {
	sendCalled bool
	lastChanID string
}

func (m *mockSlackDeps) SendMessage(_ context.Context, ch, text, ts string) (string, error) {
	m.sendCalled = true
	m.lastChanID = ch
	return "1234567890.123456", nil
}
func (m *mockSlackDeps) EditMessage(_ context.Context, _, _, _ string) error { return nil }
func (m *mockSlackDeps) DeleteMessage(_ context.Context, _, _ string) error  { return nil }
func (m *mockSlackDeps) ReadMessages(_ context.Context, _ string, _ SlackReadOpts) (interface{}, error) {
	return map[string]interface{}{"messages": []interface{}{}}, nil
}
func (m *mockSlackDeps) ReactMessage(_ context.Context, _, _, _ string) error   { return nil }
func (m *mockSlackDeps) RemoveReaction(_ context.Context, _, _, _ string) error { return nil }
func (m *mockSlackDeps) RemoveOwnReactions(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockSlackDeps) ListReactions(_ context.Context, _, _ string) (interface{}, error) {
	return []interface{}{}, nil
}
func (m *mockSlackDeps) PinMessage(_ context.Context, _, _ string) error   { return nil }
func (m *mockSlackDeps) UnpinMessage(_ context.Context, _, _ string) error { return nil }
func (m *mockSlackDeps) ListPins(_ context.Context, _ string) (interface{}, error) {
	return []interface{}{}, nil
}
func (m *mockSlackDeps) GetMemberInfo(_ context.Context, _ string) (interface{}, error) {
	return map[string]interface{}{}, nil
}
func (m *mockSlackDeps) ListEmojis(_ context.Context) (interface{}, error) {
	return map[string]interface{}{}, nil
}

func TestSlackSendMessage(t *testing.T) {
	deps := &mockSlackDeps{}
	params := map[string]interface{}{
		"action":  "sendMessage",
		"to":      "C12345",
		"content": "hello slack",
	}
	result, err := HandleSlackAction(context.Background(), params, allowAll, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success")
	}
	if !deps.sendCalled || deps.lastChanID != "C12345" {
		t.Fatal("SendMessage not called correctly")
	}
}

func TestSlackActionGateBlocks(t *testing.T) {
	deps := &mockSlackDeps{}
	params := map[string]interface{}{
		"action":    "react",
		"channelId": "C1",
		"messageId": "ts1",
		"emoji":     "👍",
	}
	_, err := HandleSlackAction(context.Background(), params, denyAll, nil, deps)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSlackReact(t *testing.T) {
	deps := &mockSlackDeps{}
	params := map[string]interface{}{
		"action":    "react",
		"channelId": "C1",
		"messageId": "ts1",
		"emoji":     "thumbsup",
	}
	result, err := HandleSlackAction(context.Background(), params, allowAll, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected ok")
	}
}

func TestSlackUnknownAction(t *testing.T) {
	deps := &mockSlackDeps{}
	params := map[string]interface{}{"action": "nope"}
	_, err := HandleSlackAction(context.Background(), params, allowAll, nil, deps)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Mock: Discord ─────────────────────────────────────────────────────

type mockDiscordDeps struct {
	sendCalled bool
}

func (m *mockDiscordDeps) SendMessage(_ context.Context, _, _, _ string, _ DiscordBridgeSendOpts) (string, string, error) {
	m.sendCalled = true
	return "m1", "c1", nil
}
func (m *mockDiscordDeps) EditMessage(_ context.Context, _, _, _, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) DeleteMessage(_ context.Context, _, _, _ string) error { return nil }
func (m *mockDiscordDeps) ReadMessages(_ context.Context, _, _ string, _ *int, _, _, _ string) ([]json.RawMessage, error) {
	return nil, nil
}
func (m *mockDiscordDeps) FetchMessage(_ context.Context, _, _, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) ReactMessage(_ context.Context, _, _, _, _ string) error   { return nil }
func (m *mockDiscordDeps) RemoveReaction(_ context.Context, _, _, _, _ string) error { return nil }
func (m *mockDiscordDeps) RemoveOwnReactions(_ context.Context, _, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockDiscordDeps) FetchReactions(_ context.Context, _, _, _ string, _ int) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) SendSticker(_ context.Context, _ string, _ []string, _, _ string) (string, string, error) {
	return "s1", "c1", nil
}
func (m *mockDiscordDeps) SendPoll(_ context.Context, _ string, _ interface{}, _, _ string) (string, string, error) {
	return "p1", "c1", nil
}
func (m *mockDiscordDeps) FetchPermissions(_ context.Context, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) PinMessage(_ context.Context, _, _, _ string) error   { return nil }
func (m *mockDiscordDeps) UnpinMessage(_ context.Context, _, _, _ string) error { return nil }
func (m *mockDiscordDeps) ListPins(_ context.Context, _, _ string) ([]json.RawMessage, error) {
	return nil, nil
}
func (m *mockDiscordDeps) CreateThread(_ context.Context, _ string, _ DiscordBridgeThreadCreate, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) ListThreads(_ context.Context, _, _, _ string, _ bool, _ string, _ int) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) SearchMessages(_ context.Context, _, _, _ string, _, _ []string, _ int) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) FetchMemberInfo(_ context.Context, _, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) FetchRoleInfo(_ context.Context, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) AddRole(_ context.Context, _, _, _, _ string) error    { return nil }
func (m *mockDiscordDeps) RemoveRole(_ context.Context, _, _, _, _ string) error { return nil }
func (m *mockDiscordDeps) FetchChannelInfo(_ context.Context, _, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) ListChannels(_ context.Context, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) FetchVoiceStatus(_ context.Context, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) CreateChannel(_ context.Context, _, _, _ string, _ *int, _, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) EditChannel(_ context.Context, _, _ string, _ map[string]interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) DeleteChannel(_ context.Context, _, _ string) error { return nil }
func (m *mockDiscordDeps) MoveChannel(_ context.Context, _, _, _ string, _ *string, _ *int) error {
	return nil
}
func (m *mockDiscordDeps) SetChannelPermission(_ context.Context, _, _, _ string, _ int, _, _ string) error {
	return nil
}
func (m *mockDiscordDeps) RemoveChannelPermission(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *mockDiscordDeps) UploadEmoji(_ context.Context, _, _, _, _ string, _ []string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) UploadSticker(_ context.Context, _, _, _, _, _, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockDiscordDeps) ListScheduledEvents(_ context.Context, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) TimeoutMember(_ context.Context, _, _, _ string, _ int, _, _ string) (interface{}, error) {
	return nil, nil
}
func (m *mockDiscordDeps) KickMember(_ context.Context, _, _, _, _ string) error { return nil }
func (m *mockDiscordDeps) BanMember(_ context.Context, _, _, _, _ string, _ int) error {
	return nil
}
func (m *mockDiscordDeps) SetPresence(_ context.Context, _ string, _ []DiscordBridgeActivity) error {
	return nil
}
func (m *mockDiscordDeps) IsGatewayConnected(_ context.Context) bool { return true }

func TestDiscordSendMessage(t *testing.T) {
	deps := &mockDiscordDeps{}
	params := map[string]interface{}{
		"action":  "sendMessage",
		"to":      "123456",
		"content": "hello discord",
		"_token":  "bot-token",
	}
	result, err := HandleDiscordAction(context.Background(), params, allowAll, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success")
	}
	if !deps.sendCalled {
		t.Fatal("SendMessage not called")
	}
}

func TestDiscordActionGateBlocks(t *testing.T) {
	deps := &mockDiscordDeps{}
	params := map[string]interface{}{
		"action":  "sendMessage",
		"to":      "123",
		"content": "hi",
		"_token":  "t",
	}
	_, err := HandleDiscordAction(context.Background(), params, denyAll, deps)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDiscordUnknownAction(t *testing.T) {
	deps := &mockDiscordDeps{}
	params := map[string]interface{}{"action": "nope", "_token": "t"}
	_, err := HandleDiscordAction(context.Background(), params, allowAll, deps)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDiscordModeration(t *testing.T) {
	deps := &mockDiscordDeps{}
	params := map[string]interface{}{
		"action":  "kick",
		"guildId": "g1",
		"userId":  "u1",
		"_token":  "t",
	}
	result, err := HandleDiscordAction(context.Background(), params, allowAll, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected ok")
	}
}

// ── Mock: Feishu ──────────────────────────────────────────────────────

type mockFeishuDeps struct {
	sendCalled bool
	lastTo     string
	lastText   string
}

func (m *mockFeishuDeps) SendMessage(_ context.Context, to, text string) (string, error) {
	m.sendCalled = true
	m.lastTo = to
	m.lastText = text
	return "om_msg_001", nil
}

func TestFeishuSendMessage(t *testing.T) {
	deps := &mockFeishuDeps{}
	params := map[string]interface{}{
		"action": "send_message",
		"to":     "oc_12345",
		"text":   "hello feishu",
	}
	result, err := HandleFeishuAction(context.Background(), params, allowAll, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success result")
	}
	if !deps.sendCalled {
		t.Fatal("expected SendMessage to be called")
	}
	if deps.lastTo != "oc_12345" || deps.lastText != "hello feishu" {
		t.Fatalf("unexpected params: to=%s text=%s", deps.lastTo, deps.lastText)
	}
}

func TestFeishuActionGateBlocks(t *testing.T) {
	deps := &mockFeishuDeps{}
	params := map[string]interface{}{
		"action": "send_message",
		"to":     "oc_12345",
		"text":   "hello",
	}
	result, err := HandleFeishuAction(context.Background(), params, denyAll, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when action gate blocks")
	}
}

func TestFeishuUnknownAction(t *testing.T) {
	deps := &mockFeishuDeps{}
	params := map[string]interface{}{"action": "nonexistent"}
	result, err := HandleFeishuAction(context.Background(), params, allowAll, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown action")
	}
}
