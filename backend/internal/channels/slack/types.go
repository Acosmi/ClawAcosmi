// Package slack 实现 Slack 频道适配器。
// 继承自 src/slack/ 的完整逻辑。
package slack

// Slack 事件/消息类型 — 继承自 src/slack/types.ts (38L)

// SlackChannelType Slack 频道类型
type SlackChannelType string

const (
	SlackChannelTypeIM      SlackChannelType = "im"
	SlackChannelTypeMPIM    SlackChannelType = "mpim"
	SlackChannelTypeChannel SlackChannelType = "channel"
	SlackChannelTypeGroup   SlackChannelType = "group"
)

// SlackFile Slack 文件附件
type SlackFile struct {
	ID                 string `json:"id,omitempty"`
	Name               string `json:"name,omitempty"`
	MimeType           string `json:"mimetype,omitempty"`
	Size               int    `json:"size,omitempty"`
	URLPrivate         string `json:"url_private,omitempty"`
	URLPrivateDownload string `json:"url_private_download,omitempty"`
}

// SlackMessageEvent Slack 消息事件
type SlackMessageEvent struct {
	Type         string           `json:"type"`
	User         string           `json:"user,omitempty"`
	BotID        string           `json:"bot_id,omitempty"`
	Subtype      string           `json:"subtype,omitempty"`
	Username     string           `json:"username,omitempty"`
	Text         string           `json:"text,omitempty"`
	Ts           string           `json:"ts,omitempty"`
	ThreadTs     string           `json:"thread_ts,omitempty"`
	EventTs      string           `json:"event_ts,omitempty"`
	ParentUserID string           `json:"parent_user_id,omitempty"`
	Channel      string           `json:"channel"`
	ChannelType  SlackChannelType `json:"channel_type,omitempty"`
	Files        []SlackFile      `json:"files,omitempty"`
}

// SlackAppMentionEvent Slack App Mention 事件
type SlackAppMentionEvent struct {
	Type         string           `json:"type"`
	User         string           `json:"user,omitempty"`
	BotID        string           `json:"bot_id,omitempty"`
	Username     string           `json:"username,omitempty"`
	Text         string           `json:"text,omitempty"`
	Ts           string           `json:"ts,omitempty"`
	ThreadTs     string           `json:"thread_ts,omitempty"`
	EventTs      string           `json:"event_ts,omitempty"`
	ParentUserID string           `json:"parent_user_id,omitempty"`
	Channel      string           `json:"channel"`
	ChannelType  SlackChannelType `json:"channel_type,omitempty"`
}
