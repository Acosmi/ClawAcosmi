package telegram

// Telegram Bot 类型 — 继承自 src/telegram/bot/types.ts (30L)

// TelegramStreamMode Bot 流模式
type TelegramStreamMode string

const (
	StreamModeOff     TelegramStreamMode = "off"
	StreamModePartial TelegramStreamMode = "partial"
	StreamModeBlock   TelegramStreamMode = "block"
)

// TelegramMessage Telegram 消息结构（简化版 grammyjs/types Message）
type TelegramMessage struct {
	MessageID       int               `json:"message_id"`
	Date            int               `json:"date,omitempty"`
	From            *TelegramUser     `json:"from,omitempty"`
	Chat            TelegramChat      `json:"chat"`
	Text            string            `json:"text,omitempty"`
	Caption         string            `json:"caption,omitempty"`
	Entities        []TelegramEntity  `json:"entities,omitempty"`
	CaptionEntities []TelegramEntity  `json:"caption_entities,omitempty"`
	ReplyToMessage  *TelegramMessage  `json:"reply_to_message,omitempty"`
	Quote           *TelegramQuote    `json:"quote,omitempty"`
	ForwardOrigin   *MessageOrigin    `json:"forward_origin,omitempty"`
	MessageThreadID *int              `json:"message_thread_id,omitempty"`
	Photo           []interface{}     `json:"photo,omitempty"`
	Video           interface{}       `json:"video,omitempty"`
	VideoNote       interface{}       `json:"video_note,omitempty"`
	Audio           interface{}       `json:"audio,omitempty"`
	Voice           interface{}       `json:"voice,omitempty"`
	Document        interface{}       `json:"document,omitempty"`
	Sticker         *TelegramSticker  `json:"sticker,omitempty"`
	Location        *TelegramLocation `json:"location,omitempty"`
	Venue           *TelegramVenue    `json:"venue,omitempty"`
	MediaGroupID    string            `json:"media_group_id,omitempty"`
}

// TelegramUser Telegram 用户
type TelegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	IsBot     bool   `json:"is_bot"`
}

// TelegramChat Telegram 聊天
type TelegramChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"` // "private", "group", "supergroup", "channel"
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
	IsForum  bool   `json:"is_forum,omitempty"`
}

// TelegramEntity 消息实体
type TelegramEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	URL    string `json:"url,omitempty"`
}

// TelegramQuote 引用
type TelegramQuote struct {
	Text string `json:"text,omitempty"`
}

// MessageOrigin 转发来源
type MessageOrigin struct {
	Type           string        `json:"type"` // "user", "hidden_user", "chat", "channel"
	Date           int           `json:"date,omitempty"`
	SenderUser     *TelegramUser `json:"sender_user,omitempty"`
	SenderUserName string        `json:"sender_user_name,omitempty"`
	SenderChat     *TelegramChat `json:"sender_chat,omitempty"`
	Chat           *TelegramChat `json:"chat,omitempty"`
	AuthorSig      string        `json:"author_signature,omitempty"`
	MessageID      int           `json:"message_id,omitempty"`
}

// TelegramSticker 贴纸
type TelegramSticker struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Emoji        string `json:"emoji,omitempty"`
	SetName      string `json:"set_name,omitempty"`
	IsAnimated   bool   `json:"is_animated,omitempty"` // DY-023: 动画贴纸标记
	IsVideo      bool   `json:"is_video,omitempty"`    // DY-023: 视频贴纸标记
}

// StickerMetadata 贴纸元数据（context enrichment + cache）
type StickerMetadata struct {
	Emoji             string `json:"emoji,omitempty"`
	SetName           string `json:"setName,omitempty"`
	FileID            string `json:"fileId,omitempty"`
	FileUniqueID      string `json:"fileUniqueId,omitempty"`
	CachedDescription string `json:"cachedDescription,omitempty"`
}

// TelegramLocation 位置
type TelegramLocation struct {
	Latitude           float64  `json:"latitude"`
	Longitude          float64  `json:"longitude"`
	HorizontalAccuracy *float64 `json:"horizontal_accuracy,omitempty"`
	LivePeriod         *int     `json:"live_period,omitempty"`
}

// TelegramVenue 场所
type TelegramVenue struct {
	Location TelegramLocation `json:"location"`
	Title    string           `json:"title"`
	Address  string           `json:"address"`
}

// NormalizedLocation 归一化位置（继承自 channels/location.ts）
type NormalizedLocation struct {
	Latitude  float64
	Longitude float64
	Accuracy  *float64
	Name      string
	Address   string
	Source    string // "place", "pin", "live"
	IsLive    bool
	Caption   string
}
