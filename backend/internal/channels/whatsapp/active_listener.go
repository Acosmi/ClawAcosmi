package whatsapp

import (
	"fmt"
	"sync"
)

// WhatsApp 活跃监听器 — 继承自 src/web/active-listener.ts (84L)

// PollInput 投票输入（从 polls 包移入避免循环导入）
type PollInput struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// ActiveWebSendOptions 发送选项
type ActiveWebSendOptions struct {
	GifPlayback bool   `json:"gifPlayback,omitempty"`
	AccountID   string `json:"accountId,omitempty"`
}

// ActiveWebListener WhatsApp Web 活跃监听器接口
type ActiveWebListener interface {
	SendMessage(to, text string, mediaBuffer []byte, mediaType string, options *ActiveWebSendOptions) (messageID string, err error)
	SendPoll(to string, poll PollInput) (messageID string, err error)
	SendReaction(chatJid, messageID, emoji string, fromMe bool, participant string) error
	SendComposingTo(to string) error
	Close() error
}

var (
	listenersMu sync.RWMutex
	listeners   = make(map[string]ActiveWebListener)
)

// ResolveWebAccountID 规范化账户 ID
func ResolveWebAccountID(accountID string) string {
	id := accountID
	if id == "" {
		id = defaultAccountID
	}
	return id
}

// RequireActiveWebListener 获取活跃监听器（不存在则返回错误）
func RequireActiveWebListener(accountID string) (string, ActiveWebListener, error) {
	id := ResolveWebAccountID(accountID)
	listenersMu.RLock()
	defer listenersMu.RUnlock()
	listener, ok := listeners[id]
	if !ok || listener == nil {
		return id, nil, fmt.Errorf(
			"no active WhatsApp Web listener (account: %s). Start the gateway, then link WhatsApp",
			id,
		)
	}
	return id, listener, nil
}

// SetActiveWebListener 设置/清除活跃监听器
func SetActiveWebListener(accountID string, listener ActiveWebListener) {
	id := ResolveWebAccountID(accountID)
	listenersMu.Lock()
	defer listenersMu.Unlock()
	if listener == nil {
		delete(listeners, id)
	} else {
		listeners[id] = listener
	}
}

// GetActiveWebListener 获取活跃监听器（可能为 nil）
func GetActiveWebListener(accountID string) ActiveWebListener {
	id := ResolveWebAccountID(accountID)
	listenersMu.RLock()
	defer listenersMu.RUnlock()
	return listeners[id]
}
