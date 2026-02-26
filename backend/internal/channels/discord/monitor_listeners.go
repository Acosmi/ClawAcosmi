package discord

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Discord 事件监听器基础设施 — 翻译自 src/discord/monitor/listeners.ts (322L)
// 提供慢监听器检测、panic 恢复包装和反应事件处理。

// ────────────────────────────────────────────
// Constants
// ────────────────────────────────────────────

// DiscordSlowListenerThresholdMs is the threshold in milliseconds after which
// a handler execution is logged as slow. Matches the TS constant
// DISCORD_SLOW_LISTENER_THRESHOLD_MS = 30_000.
// TS ref: DISCORD_SLOW_LISTENER_THRESHOLD_MS (listeners.ts L12)
const DiscordSlowListenerThresholdMs = 30_000

// ────────────────────────────────────────────
// Slow listener detection
// ────────────────────────────────────────────

// LogSlowDiscordListener logs a warning when a Discord event handler exceeds the
// slow listener threshold. Called internally by the Wrap* functions.
// TS ref: logSlowDiscordListener (listeners.ts L14-27)
func LogSlowDiscordListener(logger *slog.Logger, listenerName, eventName string, durationMs int64) {
	if durationMs < DiscordSlowListenerThresholdMs {
		return
	}
	logger.Warn("slow Discord listener detected",
		"listener", listenerName,
		"event", eventName,
		"durationMs", durationMs,
		"thresholdMs", DiscordSlowListenerThresholdMs,
	)
}

// ────────────────────────────────────────────
// Handler wrappers with recover + slow detection
// ────────────────────────────────────────────

// WrapDiscordMessageHandler wraps a message handler function with panic recovery
// and slow listener detection. Returns a function suitable for discordgo.Session.AddHandler.
// TS ref: DiscordMessageListener class (listeners.ts L40-80)
func WrapDiscordMessageHandler(
	monCtx *DiscordMonitorContext,
	name string,
	handler func(*DiscordMonitorContext, *discordgo.MessageCreate),
) func(*discordgo.Session, *discordgo.MessageCreate) {
	return func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		start := time.Now()

		defer func() {
			if r := recover(); r != nil {
				monCtx.Logger.Error("panic in Discord message handler",
					"listener", name,
					"event", "MessageCreate",
					"panic", fmt.Sprintf("%v", r),
					"stack", string(debug.Stack()),
				)
			}

			durationMs := time.Since(start).Milliseconds()
			LogSlowDiscordListener(monCtx.Logger, name, "MessageCreate", durationMs)
		}()

		handler(monCtx, m)
	}
}

// WrapDiscordReactionAddHandler wraps a reaction-add handler with panic recovery
// and slow listener detection. Returns a function suitable for discordgo.Session.AddHandler.
// TS ref: DiscordReactionListener class (listeners.ts L82-160)
func WrapDiscordReactionAddHandler(
	monCtx *DiscordMonitorContext,
	name string,
	handler func(*DiscordMonitorContext, *discordgo.MessageReactionAdd),
) func(*discordgo.Session, *discordgo.MessageReactionAdd) {
	return func(_ *discordgo.Session, r *discordgo.MessageReactionAdd) {
		start := time.Now()

		defer func() {
			if rec := recover(); rec != nil {
				monCtx.Logger.Error("panic in Discord reaction-add handler",
					"listener", name,
					"event", "MessageReactionAdd",
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()),
				)
			}

			durationMs := time.Since(start).Milliseconds()
			LogSlowDiscordListener(monCtx.Logger, name, "MessageReactionAdd", durationMs)
		}()

		handler(monCtx, r)
	}
}

// WrapDiscordReactionRemoveHandler wraps a reaction-remove handler with panic recovery
// and slow listener detection. Returns a function suitable for discordgo.Session.AddHandler.
// TS ref: DiscordReactionRemoveListener class (listeners.ts L162-240)
func WrapDiscordReactionRemoveHandler(
	monCtx *DiscordMonitorContext,
	name string,
	handler func(*DiscordMonitorContext, *discordgo.MessageReactionRemove),
) func(*discordgo.Session, *discordgo.MessageReactionRemove) {
	return func(_ *discordgo.Session, r *discordgo.MessageReactionRemove) {
		start := time.Now()

		defer func() {
			if rec := recover(); rec != nil {
				monCtx.Logger.Error("panic in Discord reaction-remove handler",
					"listener", name,
					"event", "MessageReactionRemove",
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()),
				)
			}

			durationMs := time.Since(start).Milliseconds()
			LogSlowDiscordListener(monCtx.Logger, name, "MessageReactionRemove", durationMs)
		}()

		handler(monCtx, r)
	}
}

// WrapDiscordPresenceHandler wraps a presence update handler with panic recovery
// and slow listener detection. Returns a function suitable for discordgo.Session.AddHandler.
// TS ref: DiscordPresenceListener class (listeners.ts L282-322)
func WrapDiscordPresenceHandler(
	monCtx *DiscordMonitorContext,
	name string,
	handler func(*DiscordMonitorContext, *discordgo.PresenceUpdate),
) func(*discordgo.Session, *discordgo.PresenceUpdate) {
	return func(_ *discordgo.Session, p *discordgo.PresenceUpdate) {
		start := time.Now()

		defer func() {
			if rec := recover(); rec != nil {
				monCtx.Logger.Error("panic in Discord presence handler",
					"listener", name,
					"event", "PresenceUpdate",
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()),
				)
			}

			durationMs := time.Since(start).Milliseconds()
			LogSlowDiscordListener(monCtx.Logger, name, "PresenceUpdate", durationMs)
		}()

		handler(monCtx, p)
	}
}

// ────────────────────────────────────────────
// Reaction event handling
// ────────────────────────────────────────────

// HandleDiscordReactionEvent handles a Discord reaction event (add or remove) with
// full guild config resolution, channel config fallback, emoji formatting, and
// system event enqueueing.
//
// The action parameter should be "added" or "removed".
//
// Processing steps:
//  1. Filter bot users
//  2. Only guild reactions (skip DM reactions)
//  3. Resolve guild entry from config
//  4. Fetch channel info, detect threads
//  5. Resolve channel config with fallback (thread -> parent)
//  6. Check channel allowed
//  7. Check reaction notification mode (own/all/none)
//  8. Format reaction emoji and actor labels
//  9. Build system event text
//  10. Resolve agent route
//  11. Enqueue system event
//
// TS ref: handleDiscordReactionEvent (listeners.ts L95-240)
func HandleDiscordReactionEvent(
	monCtx *DiscordMonitorContext,
	userID string,
	guildID string,
	channelID string,
	messageID string,
	emojiID string,
	emojiName string,
	action string,
) {
	logger := monCtx.Logger.With(
		"action", "reaction-"+action,
		"user", userID,
		"channel", channelID,
		"message", messageID,
	)

	// 1. Filter bot users
	if userID == monCtx.BotUserID {
		logger.Debug("reaction from self, skipping")
		return
	}

	// 2. Only guild reactions (skip DM reactions)
	if guildID == "" {
		logger.Debug("reaction in DM, skipping")
		return
	}

	// 3. Resolve guild entry from config
	monCtx.mu.RLock()
	guildConfigs := monCtx.GuildConfigs
	monCtx.mu.RUnlock()

	guildName := resolveGuildName(monCtx.Session, guildID)
	guildInfo := ResolveDiscordGuildEntry(guildID, guildName, guildConfigs)
	if guildInfo == nil {
		logger.Debug("guild not in config, skipping reaction",
			"guildID", guildID,
			"guildName", guildName,
		)
		return
	}

	// 4. Fetch channel info, detect threads
	channelName := ""
	isThread := false
	threadParentID := ""
	threadParentName := ""

	ch, chErr := monCtx.Session.State.Channel(channelID)
	if chErr == nil && ch != nil {
		channelName = ch.Name
		if ch.IsThread() {
			isThread = true
			if ch.ParentID != "" {
				parent, parentErr := monCtx.Session.State.Channel(ch.ParentID)
				if parentErr == nil && parent != nil {
					threadParentID = parent.ID
					threadParentName = parent.Name
				}
			}
		}
	}

	// 5. Resolve channel config with fallback (thread -> parent)
	configChannelName := channelName
	if isThread && threadParentName != "" {
		configChannelName = threadParentName
	}
	configChannelSlug := ""
	if configChannelName != "" {
		configChannelSlug = NormalizeDiscordSlug(configChannelName)
	}

	channelConfig := ResolveDiscordChannelConfig(guildInfo, channelID, configChannelName, configChannelSlug)
	if channelConfig == nil && isThread && threadParentID != "" {
		channelConfig = ResolveDiscordChannelConfig(guildInfo, threadParentID, threadParentName, NormalizeDiscordSlug(threadParentName))
	}

	// 6. Check channel allowed
	if channelConfig != nil && !channelConfig.Allowed {
		logger.Debug("reaction in disallowed channel, skipping",
			"channelName", channelName,
		)
		return
	}

	// 7. Check reaction notification mode (own/all/none)
	reactionMode := guildInfo.ReactionNotification
	if reactionMode == "" {
		reactionMode = "own" // default: notify only on own messages (对齐 TS guildInfo?.reactionNotifications ?? "own")
	}
	if reactionMode == "none" {
		logger.Debug("reaction notifications disabled for guild")
		return
	}
	// "own" mode: only notify when a reaction is on the bot's own messages.
	// Fetch message to check author (对齐 TS handleDiscordReactionEvent L247-260)
	if reactionMode == "own" {
		msg, msgErr := monCtx.Session.ChannelMessage(channelID, messageID)
		if msgErr != nil || msg == nil || msg.Author == nil {
			logger.Debug("own mode: cannot fetch message author, skipping")
			return
		}
		if msg.Author.ID != monCtx.BotUserID {
			logger.Debug("own mode: reaction not on bot message, skipping",
				"messageAuthor", msg.Author.ID,
			)
			return
		}
	}

	// 8. Format reaction emoji and actor labels
	emojiLabel := FormatDiscordReactionEmoji(emojiID, emojiName)

	// Resolve the reacting user's display name if possible
	actorLabel := userID
	member, memberErr := monCtx.Session.State.Member(guildID, userID)
	if memberErr == nil && member != nil && member.User != nil {
		actorLabel = FormatDiscordUserTag(member.User.Username, member.User.Discriminator, userID)
	}

	// 9. Build system event text
	locationLabel := ""
	if guildName != "" {
		locationLabel = guildName + " #" + channelName
	} else {
		locationLabel = "#" + channelName
	}
	if isThread && channelName != "" {
		locationLabel = locationLabel + " (thread)"
	}

	systemText := fmt.Sprintf(
		"Reaction %s: %s by %s on message %s in %s",
		action, emojiLabel, actorLabel, messageID, locationLabel,
	)

	// 10. Resolve agent route
	sessionKey := ""
	contextKey := fmt.Sprintf("discord:reaction:%s:%s:%s:%s", action, messageID, userID, emojiLabel)

	if monCtx.Deps != nil && monCtx.Deps.ResolveAgentRoute != nil {
		route, err := monCtx.Deps.ResolveAgentRoute(DiscordAgentRouteParams{
			Channel:   "discord",
			AccountID: monCtx.AccountID,
			PeerKind:  "channel",
			PeerID:    fmt.Sprintf("%s:%s", guildID, channelID),
		})
		if err == nil && route != nil {
			sessionKey = route.SessionKey
		}
	}

	// 11. Enqueue system event
	if monCtx.Deps != nil && monCtx.Deps.EnqueueSystemEvent != nil {
		if err := monCtx.Deps.EnqueueSystemEvent(systemText, sessionKey, contextKey); err != nil {
			logger.Error("failed to enqueue reaction system event",
				"error", err,
			)
		}
	}

	logger.Debug("reaction event processed",
		"action", action,
		"emoji", emojiLabel,
		"actor", actorLabel,
		"reactionMode", reactionMode,
	)
}

// ────────────────────────────────────────────
// Presence update handling
// ────────────────────────────────────────────

// HandleDiscordPresenceUpdate handles a Discord presence update event by updating
// the presence cache. This is the canonical handler used by the event binding layer.
// TS ref: DiscordPresenceListener (listeners.ts L282-322)
func HandleDiscordPresenceUpdate(monCtx *DiscordMonitorContext, p *discordgo.PresenceUpdate) {
	if p == nil || p.User == nil || p.User.ID == "" {
		return
	}
	monCtx.PresenceCache.Update(monCtx.AccountID, p.User.ID, PresenceData{
		Status:       string(p.Status),
		Activities:   p.Activities,
		ClientStatus: &p.ClientStatus,
	})
}
