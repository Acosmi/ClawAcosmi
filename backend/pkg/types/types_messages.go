package types

// 消息配置类型 — 继承自 src/config/types.messages.ts

// GroupChatConfig 群聊配置
type GroupChatConfig struct {
	MentionPatterns []string `json:"mentionPatterns,omitempty"`
	HistoryLimit    *int     `json:"historyLimit,omitempty"`
}

// DmConfig 私聊配置
type DmConfig struct {
	HistoryLimit *int `json:"historyLimit,omitempty"`
}

// BroadcastStrategy 广播策略
type BroadcastStrategy string

const (
	BroadcastParallel   BroadcastStrategy = "parallel"
	BroadcastSequential BroadcastStrategy = "sequential"
)

// BroadcastConfig 广播配置
// 原版使用索引签名 [peerId: string]，Go 中用 map 表示
type BroadcastConfig struct {
	Strategy BroadcastStrategy   `json:"strategy,omitempty"`
	Peers    map[string][]string `json:"peers,omitempty"` // peerId → agentIds
}

// AudioTranscriptionConfig 音频转录配置 (@deprecated)
type AudioTranscriptionConfig struct {
	Command        []string `json:"command"`
	TimeoutSeconds *int     `json:"timeoutSeconds,omitempty"`
}

// AudioConfig 音频配置
type AudioConfig struct {
	// @deprecated 使用 tools.media.audio.models 代替
	Transcription *AudioTranscriptionConfig `json:"transcription,omitempty"`
}

// AckReactionScope 确认反应范围
type AckReactionScope string

const (
	AckGroupMentions AckReactionScope = "group-mentions"
	AckGroupAll      AckReactionScope = "group-all"
	AckDirect        AckReactionScope = "direct"
	AckAll           AckReactionScope = "all"
)

// MessagesConfig 消息总配置
// 原版: export type MessagesConfig
type MessagesConfig struct {
	// @deprecated 使用 whatsapp.messagePrefix 代替
	MessagePrefix       string                 `json:"messagePrefix,omitempty"`
	ResponsePrefix      string                 `json:"responsePrefix,omitempty"` // 支持模板变量：{model}, {provider}, {thinkingLevel} 等
	GroupChat           *GroupChatConfig       `json:"groupChat,omitempty"`
	Queue               *QueueConfig           `json:"queue,omitempty"`
	Inbound             *InboundDebounceConfig `json:"inbound,omitempty"`
	AckReaction         string                 `json:"ackReaction,omitempty"`
	AckReactionScope    AckReactionScope       `json:"ackReactionScope,omitempty"`
	RemoveAckAfterReply *bool                  `json:"removeAckAfterReply,omitempty"`
	TTS                 *TtsConfig             `json:"tts,omitempty"`
}

// NativeCommandsSetting 原生命令设置
// 可以是 bool 或 "auto" 字符串，Go 中用 interface{} 表示
type NativeCommandsSetting interface{}

// CommandsConfig 命令配置
// 原版: export type CommandsConfig
type CommandsConfig struct {
	Native           NativeCommandsSetting `json:"native,omitempty"`
	NativeSkills     NativeCommandsSetting `json:"nativeSkills,omitempty"`
	Text             *bool                 `json:"text,omitempty"`
	Bash             *bool                 `json:"bash,omitempty"`
	BashForegroundMs *int                  `json:"bashForegroundMs,omitempty"`
	Config           *bool                 `json:"config,omitempty"`
	Debug            *bool                 `json:"debug,omitempty"`
	Restart          *bool                 `json:"restart,omitempty"`
	UseAccessGroups  *bool                 `json:"useAccessGroups,omitempty"`
	OwnerAllowFrom   []interface{}         `json:"ownerAllowFrom,omitempty"` // string|number 混合数组
}

// ProviderCommandsConfig 频道特定命令配置覆盖
type ProviderCommandsConfig struct {
	Native       NativeCommandsSetting `json:"native,omitempty"`
	NativeSkills NativeCommandsSetting `json:"nativeSkills,omitempty"`
}
