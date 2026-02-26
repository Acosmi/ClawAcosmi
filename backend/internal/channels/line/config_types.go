package line

// TS 对照: src/line/types.ts (155L) — 审计补全: config/account/group 类型

// ---------- Config 类型 ----------

// LineTokenSource 令牌来源。
type LineTokenSource string

const (
	TokenSourceConfig LineTokenSource = "config"
	TokenSourceEnv    LineTokenSource = "env"
	TokenSourceFile   LineTokenSource = "file"
	TokenSourceNone   LineTokenSource = "none"
)

// LineConfig LINE 频道配置。
type LineConfig struct {
	Enabled            bool                         `json:"enabled,omitempty"`
	ChannelAccessToken string                       `json:"channelAccessToken,omitempty"`
	ChannelSecret      string                       `json:"channelSecret,omitempty"`
	TokenFile          string                       `json:"tokenFile,omitempty"`
	SecretFile         string                       `json:"secretFile,omitempty"`
	Name               string                       `json:"name,omitempty"`
	AllowFrom          []string                     `json:"allowFrom,omitempty"`
	GroupAllowFrom     []string                     `json:"groupAllowFrom,omitempty"`
	DMPolicy           string                       `json:"dmPolicy,omitempty"`    // "open"|"allowlist"|"pairing"|"disabled"
	GroupPolicy        string                       `json:"groupPolicy,omitempty"` // "open"|"allowlist"|"disabled"
	ResponsePrefix     string                       `json:"responsePrefix,omitempty"`
	MediaMaxMB         int                          `json:"mediaMaxMb,omitempty"`
	WebhookPath        string                       `json:"webhookPath,omitempty"`
	Accounts           map[string]LineAccountConfig `json:"accounts,omitempty"`
	Groups             map[string]LineGroupConfig   `json:"groups,omitempty"`
}

// LineAccountConfig LINE 帐号配置。
type LineAccountConfig struct {
	Enabled            bool                       `json:"enabled,omitempty"`
	ChannelAccessToken string                     `json:"channelAccessToken,omitempty"`
	ChannelSecret      string                     `json:"channelSecret,omitempty"`
	TokenFile          string                     `json:"tokenFile,omitempty"`
	SecretFile         string                     `json:"secretFile,omitempty"`
	Name               string                     `json:"name,omitempty"`
	AllowFrom          []string                   `json:"allowFrom,omitempty"`
	GroupAllowFrom     []string                   `json:"groupAllowFrom,omitempty"`
	DMPolicy           string                     `json:"dmPolicy,omitempty"`
	GroupPolicy        string                     `json:"groupPolicy,omitempty"`
	ResponsePrefix     string                     `json:"responsePrefix,omitempty"`
	MediaMaxMB         int                        `json:"mediaMaxMb,omitempty"`
	WebhookPath        string                     `json:"webhookPath,omitempty"`
	Groups             map[string]LineGroupConfig `json:"groups,omitempty"`
}

// LineGroupConfig LINE 群组配置。
type LineGroupConfig struct {
	Enabled        bool     `json:"enabled,omitempty"`
	AllowFrom      []string `json:"allowFrom,omitempty"`
	RequireMention bool     `json:"requireMention,omitempty"`
	SystemPrompt   string   `json:"systemPrompt,omitempty"`
	Skills         []string `json:"skills,omitempty"`
}

// ResolvedLineAccount 解析后的帐号。
type ResolvedLineAccount struct {
	AccountID          string          `json:"accountId"`
	Name               string          `json:"name,omitempty"`
	Enabled            bool            `json:"enabled"`
	ChannelAccessToken string          `json:"channelAccessToken"`
	ChannelSecret      string          `json:"channelSecret"`
	TokenSource        LineTokenSource `json:"tokenSource"`
	// Config 帐号有效配置（merged from account + top-level config）。
	Config ResolvedLineConfig `json:"config,omitempty"`
}

// ResolvedLineConfig 合并后的帐号有效配置。
type ResolvedLineConfig struct {
	AllowFrom      []string                   `json:"allowFrom,omitempty"`
	GroupAllowFrom []string                   `json:"groupAllowFrom,omitempty"`
	DMPolicy       string                     `json:"dmPolicy,omitempty"`
	GroupPolicy    string                     `json:"groupPolicy,omitempty"`
	Groups         map[string]LineGroupConfig `json:"groups,omitempty"`
}

// ---------- 发送结果 ----------

// LineSendResult 消息发送结果。
type LineSendResult struct {
	MessageID string `json:"messageId"`
	ChatID    string `json:"chatId"`
}

// ---------- Probe 结果 ----------

// LineProbeResult bot info 查询结果。
type LineProbeResult struct {
	OK    bool            `json:"ok"`
	Bot   *LineBotProfile `json:"bot,omitempty"`
	Error string          `json:"error,omitempty"`
}

// LineBotProfile bot 资料。
type LineBotProfile struct {
	DisplayName string `json:"displayName,omitempty"`
	UserID      string `json:"userId,omitempty"`
	BasicID     string `json:"basicId,omitempty"`
	PictureURL  string `json:"pictureUrl,omitempty"`
}

// ---------- Template Message ----------

// LineTemplateMessagePayload 模板消息。
type LineTemplateMessagePayload struct {
	Type    string `json:"type"` // "confirm"|"buttons"|"carousel"
	AltText string `json:"altText,omitempty"`
	// confirm
	Text         string `json:"text,omitempty"`
	ConfirmLabel string `json:"confirmLabel,omitempty"`
	ConfirmData  string `json:"confirmData,omitempty"`
	CancelLabel  string `json:"cancelLabel,omitempty"`
	CancelData   string `json:"cancelData,omitempty"`
	// buttons
	Title        string           `json:"title,omitempty"`
	ThumbnailURL string           `json:"thumbnailImageUrl,omitempty"`
	Actions      []TemplateAction `json:"actions,omitempty"`
	// carousel
	Columns []CarouselColumn `json:"columns,omitempty"`
}

// TemplateAction 模板动作。
type TemplateAction struct {
	Type  string `json:"type"` // "message"|"uri"|"postback"
	Label string `json:"label"`
	Data  string `json:"data,omitempty"`
	URI   string `json:"uri,omitempty"`
	Text  string `json:"text,omitempty"`
}

// CarouselColumn 轮播列。
type CarouselColumn struct {
	Title        string           `json:"title,omitempty"`
	Text         string           `json:"text"`
	ThumbnailURL string           `json:"thumbnailImageUrl,omitempty"`
	Actions      []TemplateAction `json:"actions,omitempty"`
}

// ---------- Channel Data ----------

// LineChannelData LINE 特有的频道数据。
type LineChannelData struct {
	QuickReplies    []string                    `json:"quickReplies,omitempty"`
	Location        *LocationData               `json:"location,omitempty"`
	FlexMessage     *LineFlexMessagePayload     `json:"flexMessage,omitempty"`
	TemplateMessage *LineTemplateMessagePayload `json:"templateMessage,omitempty"`
}

// LocationData 位置数据。
type LocationData struct {
	Title     string  `json:"title"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// LineFlexMessagePayload flex 消息载荷。
type LineFlexMessagePayload struct {
	AltText  string      `json:"altText"`
	Contents interface{} `json:"contents"`
}
