package discord

// Discord 系统事件处理 — 继承自 src/discord/monitor/system-events.ts (55L)

// enqueueDiscordSystemEvent 已定义在 monitor_provider.go 中。
// 本文件包含系统事件格式化辅助函数。

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// FormatMemberEventText 格式化成员事件文本。
func FormatMemberEventText(action, guildID, userName, userID string) string {
	if userName != "" {
		return fmt.Sprintf("Member %s guild %s: %s (%s)", action, guildID, userName, userID)
	}
	return fmt.Sprintf("Member %s guild %s: %s", action, guildID, userID)
}

// FormatChannelEventText 格式化频道事件文本。
func FormatChannelEventText(action, channelName, channelID, guildID string) string {
	if channelName != "" {
		return fmt.Sprintf("Channel %s: #%s (%s) in guild %s", action, channelName, channelID, guildID)
	}
	return fmt.Sprintf("Channel %s: %s in guild %s", action, channelID, guildID)
}

// FormatDiscordSystemMessageEvent returns a human-readable string describing a
// Discord system message event (e.g. member join, boost, pin, thread creation).
// It mirrors the 17-case switch in the TS resolveDiscordSystemEvent (system-events.ts).
//
// The location parameter identifies where the event occurred (e.g. "ServerName #channel",
// "DM", "Group DM #channel"), matching the TS `location` parameter.
//
// Returns "" if the message type is a normal user message (default, reply,
// slash command) or an unrecognised system type.
func FormatDiscordSystemMessageEvent(m *discordgo.MessageCreate, location string) string {
	if m == nil {
		return ""
	}

	// Use FormatDiscordUserTag for author label, matching TS formatDiscordUserTag behaviour.
	authorTag := ""
	if m.Author != nil {
		authorTag = FormatDiscordUserTag(m.Author.Username, m.Author.Discriminator, m.Author.ID)
	}

	switch m.Type {
	// --- Non-system types: return empty ---
	case discordgo.MessageTypeDefault: // 0
		return ""
	case discordgo.MessageTypeReply: // 19
		return ""
	case discordgo.MessageTypeChatInputCommand: // 20 (slash command)
		return ""

	// --- System event types ---
	case discordgo.MessageTypeRecipientAdd: // 1
		return buildSystemEventText(authorTag, "added a recipient", location)
	case discordgo.MessageTypeRecipientRemove: // 2
		return buildSystemEventText(authorTag, "removed a recipient", location)
	case discordgo.MessageTypeCall: // 3
		return buildSystemEventText(authorTag, "started a call", location)
	case discordgo.MessageTypeChannelNameChange: // 4
		return buildSystemEventText(authorTag, "changed the channel name", location)
	case discordgo.MessageTypeChannelIconChange: // 5
		return buildSystemEventText(authorTag, "changed the channel icon", location)
	case discordgo.MessageTypeChannelPinnedMessage: // 6
		return buildSystemEventText(authorTag, "pinned a message", location)
	case discordgo.MessageTypeGuildMemberJoin: // 7
		return buildSystemEventText(authorTag, "user joined", location)
	case discordgo.MessageTypeUserPremiumGuildSubscription: // 8
		return buildSystemEventText(authorTag, "boosted the server", location)
	case discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne: // 9
		return buildSystemEventText(authorTag, "boosted the server (Tier 1 reached)", location)
	case discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo: // 10
		return buildSystemEventText(authorTag, "boosted the server (Tier 2 reached)", location)
	case discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree: // 11
		return buildSystemEventText(authorTag, "boosted the server (Tier 3 reached)", location)
	case discordgo.MessageTypeChannelFollowAdd: // 12
		return buildSystemEventText(authorTag, "added a channel follow", location)
	case discordgo.MessageTypeGuildDiscoveryDisqualified: // 14
		return buildSystemEventText(authorTag, "server disqualified from discovery", location)
	case discordgo.MessageTypeGuildDiscoveryRequalified: // 15
		return buildSystemEventText(authorTag, "server requalified for discovery", location)
	case discordgo.MessageTypeThreadCreated: // 18
		return buildSystemEventText(authorTag, "created a thread", location)
	case discordgo.MessageTypeThreadStarterMessage: // 21
		return buildSystemEventText(authorTag, "thread starter message", location)
	case discordgo.MessageTypeContextMenuCommand: // 23
		return buildSystemEventText(authorTag, "used a context menu command", location)

	// Types 24+ are defined in Discord API but not yet in discordgo v0.29.
	// Use raw integer constants for completeness (mirrors TS AutoModerationAction=24,
	// StageStart=27, StageEnd=28, StageSpeaker=29, StageTopic=31, etc.).
	case discordgo.MessageType(24): // AutoModerationAction
		return buildSystemEventText(authorTag, "auto moderation action", location)
	case discordgo.MessageType(27): // StageStart
		return buildSystemEventText(authorTag, "stage started", location)
	case discordgo.MessageType(28): // StageEnd
		return buildSystemEventText(authorTag, "stage ended", location)
	case discordgo.MessageType(29): // StageSpeaker
		return buildSystemEventText(authorTag, "stage speaker updated", location)
	case discordgo.MessageType(31): // StageTopic
		return buildSystemEventText(authorTag, "stage topic updated", location)
	case discordgo.MessageType(36): // GuildIncidentAlertModeEnabled
		return buildSystemEventText(authorTag, "raid protection enabled", location)
	case discordgo.MessageType(37): // GuildIncidentAlertModeDisabled
		return buildSystemEventText(authorTag, "raid protection disabled", location)
	case discordgo.MessageType(38): // GuildIncidentReportRaid
		return buildSystemEventText(authorTag, "raid reported", location)
	case discordgo.MessageType(39): // GuildIncidentReportFalseAlarm
		return buildSystemEventText(authorTag, "raid report marked false alarm", location)
	case discordgo.MessageType(46): // PollResult
		return buildSystemEventText(authorTag, "poll results posted", location)
	case discordgo.MessageType(44): // PurchaseNotification
		return buildSystemEventText(authorTag, "purchase notification", location)

	default:
		return ""
	}
}

// buildSystemEventText constructs "Discord system: <actor> <action> in <location>",
// matching the TS buildDiscordSystemEvent output format.
func buildSystemEventText(authorTag, action, location string) string {
	actor := ""
	if authorTag != "" {
		actor = authorTag + " "
	}
	loc := ""
	if location != "" {
		loc = " in " + location
	}
	return fmt.Sprintf("Discord system: %s%s%s", actor, action, loc)
}
