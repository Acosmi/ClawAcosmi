package channels

// 确认反应门控 — 继承自 src/channels/ack-reactions.ts (104 行)

// AckReactionScope 确认反应作用域
type AckReactionScope string

const (
	AckScopeAll           AckReactionScope = "all"
	AckScopeDirect        AckReactionScope = "direct"
	AckScopeGroupAll      AckReactionScope = "group-all"
	AckScopeGroupMentions AckReactionScope = "group-mentions"
	AckScopeOff           AckReactionScope = "off"
	AckScopeNone          AckReactionScope = "none"
)

// WhatsAppAckReactionMode WhatsApp 专属确认反应模式
type WhatsAppAckReactionMode string

const (
	WhatsAppAckAlways   WhatsAppAckReactionMode = "always"
	WhatsAppAckMentions WhatsAppAckReactionMode = "mentions"
	WhatsAppAckNever    WhatsAppAckReactionMode = "never"
)

// AckReactionGateParams 确认反应门控参数
type AckReactionGateParams struct {
	Scope                 AckReactionScope
	IsDirect              bool
	IsGroup               bool
	IsMentionableGroup    bool
	RequireMention        bool
	CanDetectMention      bool
	EffectiveWasMentioned bool
	ShouldBypassMention   bool
}

// ShouldAckReaction 判断是否应发送确认反应
func ShouldAckReaction(p AckReactionGateParams) bool {
	scope := p.Scope
	if scope == "" {
		scope = AckScopeGroupMentions
	}
	switch scope {
	case AckScopeOff, AckScopeNone:
		return false
	case AckScopeAll:
		return true
	case AckScopeDirect:
		return p.IsDirect
	case AckScopeGroupAll:
		return p.IsGroup
	case AckScopeGroupMentions:
		if !p.IsMentionableGroup || !p.RequireMention || !p.CanDetectMention {
			return false
		}
		return p.EffectiveWasMentioned || p.ShouldBypassMention
	default:
		return false
	}
}

// ShouldAckReactionForWhatsApp WhatsApp 专属确认反应判断
func ShouldAckReactionForWhatsApp(emoji string, isDirect, isGroup, directEnabled bool, groupMode WhatsAppAckReactionMode, wasMentioned, groupActivated bool) bool {
	if emoji == "" {
		return false
	}
	if isDirect {
		return directEnabled
	}
	if !isGroup {
		return false
	}
	switch groupMode {
	case WhatsAppAckNever:
		return false
	case WhatsAppAckAlways:
		return true
	default:
		return ShouldAckReaction(AckReactionGateParams{
			Scope:                 AckScopeGroupMentions,
			IsDirect:              false,
			IsGroup:               true,
			IsMentionableGroup:    true,
			RequireMention:        true,
			CanDetectMention:      true,
			EffectiveWasMentioned: wasMentioned,
			ShouldBypassMention:   groupActivated,
		})
	}
}
