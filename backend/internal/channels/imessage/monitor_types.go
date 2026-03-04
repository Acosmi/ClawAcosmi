//go:build darwin

package imessage

import (
	"encoding/json"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// iMessage 监控类型定义 — 继承自 src/imessage/monitor/types.ts (41L)

// IMessageAttachment iMessage 附件
type IMessageAttachment struct {
	OriginalPath *string `json:"original_path,omitempty"`
	MimeType     *string `json:"mime_type,omitempty"`
	Missing      *bool   `json:"missing,omitempty"`
}

// IMessagePayload iMessage 入站消息载荷
type IMessagePayload struct {
	ID             *json.Number         `json:"id,omitempty"`
	ChatID         *int                 `json:"chat_id,omitempty"`
	Sender         *string              `json:"sender,omitempty"`
	IsFromMe       *bool                `json:"is_from_me,omitempty"`
	Text           *string              `json:"text,omitempty"`
	ReplyToID      json.RawMessage      `json:"reply_to_id,omitempty"` // string | number | null
	ReplyToText    *string              `json:"reply_to_text,omitempty"`
	ReplyToSender  *string              `json:"reply_to_sender,omitempty"`
	CreatedAt      *string              `json:"created_at,omitempty"`
	Attachments    []IMessageAttachment `json:"attachments,omitempty"`
	ChatIdentifier *string              `json:"chat_identifier,omitempty"`
	ChatGUID       *string              `json:"chat_guid,omitempty"`
	ChatName       *string              `json:"chat_name,omitempty"`
	Participants   []string             `json:"participants,omitempty"`
	IsGroup        *bool                `json:"is_group,omitempty"`
}

// MonitorIMessageOpts 入站监控选项
type MonitorIMessageOpts struct {
	CliPath            string
	DbPath             string
	AccountID          string
	Config             *types.OpenAcosmiConfig
	AllowFrom          []interface{}
	GroupAllowFrom     []interface{}
	IncludeAttachments *bool
	MediaMaxMB         *int
	RequireMention     *bool
	Deps               *MonitorDeps // DI 依赖注入
}
