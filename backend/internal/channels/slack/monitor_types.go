package slack

// Slack 监控类型 — 继承自 src/slack/monitor/types.ts (90L)
// 这些类型在 Go 端作为 monitor 相关函数的参数和返回值使用。

// MonitorSlackMode 监控模式
type MonitorSlackMode string

const (
	MonitorModeSocket MonitorSlackMode = "socket"
	MonitorModeHTTP   MonitorSlackMode = "http"
)

// SlackReactionEvent Slack 反应事件
type SlackReactionEvent struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Reaction string `json:"reaction"`
	Item     struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Ts      string `json:"ts"`
	} `json:"item"`
	ItemUser string `json:"item_user,omitempty"`
	EventTs  string `json:"event_ts,omitempty"`
}

// SlackMemberChannelEvent Slack 成员频道事件（频道加入/离开）
type SlackMemberChannelEvent struct {
	Type        string `json:"type"` // "member_joined_channel" | "member_left_channel"
	User        string `json:"user"`
	Channel     string `json:"channel"`
	Team        string `json:"team,omitempty"`
	Inviter     string `json:"inviter,omitempty"`
	EventTs     string `json:"event_ts,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`
}

// SlackChannelEvent Slack 频道事件（频道重命名/归档/解归档/删除/创建/ID 变更）
type SlackChannelEvent struct {
	Type    string `json:"type"`
	Channel struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	} `json:"channel,omitempty"`
	EventTs string `json:"event_ts,omitempty"`
	// channel_id_changed 专属字段
	OldChannelID string `json:"old_channel_id,omitempty"`
	NewChannelID string `json:"new_channel_id,omitempty"`
}

// SlackPinEvent Slack 固定消息事件
type SlackPinEvent struct {
	Type    string `json:"type"` // "pin_added" | "pin_removed"
	User    string `json:"user,omitempty"`
	Channel string `json:"channel_id,omitempty"`
	Item    struct {
		Type    string `json:"type"`
		Channel string `json:"channel,omitempty"`
		Message *struct {
			Ts   string `json:"ts,omitempty"`
			Text string `json:"text,omitempty"`
		} `json:"message,omitempty"`
	} `json:"item,omitempty"`
	EventTs string `json:"event_ts,omitempty"`
}

// MonitorSlackOpts 监控启动选项
type MonitorSlackOpts struct {
	AccountID     string
	Mode          MonitorSlackMode
	BotToken      string
	AppToken      string
	SigningSecret string
	WebhookPath   string
}
