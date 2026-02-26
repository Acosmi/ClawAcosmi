package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Discord 权限计算 — 继承自 src/discord/send.permissions.ts (155L)
// 使用 math/big 替代 TS 的 BigInt

// Discord 权限位定义（discord-api-types/v10 PermissionFlagsBits）
var permissionFlagBits = map[string]*big.Int{
	"CreateInstantInvite":              new(big.Int).SetUint64(1 << 0),
	"KickMembers":                      new(big.Int).SetUint64(1 << 1),
	"BanMembers":                       new(big.Int).SetUint64(1 << 2),
	"Administrator":                    new(big.Int).SetUint64(1 << 3),
	"ManageChannels":                   new(big.Int).SetUint64(1 << 4),
	"ManageGuild":                      new(big.Int).SetUint64(1 << 5),
	"AddReactions":                     new(big.Int).SetUint64(1 << 6),
	"ViewAuditLog":                     new(big.Int).SetUint64(1 << 7),
	"PrioritySpeaker":                  new(big.Int).SetUint64(1 << 8),
	"Stream":                           new(big.Int).SetUint64(1 << 9),
	"ViewChannel":                      new(big.Int).SetUint64(1 << 10),
	"SendMessages":                     new(big.Int).SetUint64(1 << 11),
	"SendTTSMessages":                  new(big.Int).SetUint64(1 << 12),
	"ManageMessages":                   new(big.Int).SetUint64(1 << 13),
	"EmbedLinks":                       new(big.Int).SetUint64(1 << 14),
	"AttachFiles":                      new(big.Int).SetUint64(1 << 15),
	"ReadMessageHistory":               new(big.Int).SetUint64(1 << 16),
	"MentionEveryone":                  new(big.Int).SetUint64(1 << 17),
	"UseExternalEmojis":                new(big.Int).SetUint64(1 << 18),
	"ViewGuildInsights":                new(big.Int).SetUint64(1 << 19),
	"Connect":                          new(big.Int).SetUint64(1 << 20),
	"Speak":                            new(big.Int).SetUint64(1 << 21),
	"MuteMembers":                      new(big.Int).SetUint64(1 << 22),
	"DeafenMembers":                    new(big.Int).SetUint64(1 << 23),
	"MoveMembers":                      new(big.Int).SetUint64(1 << 24),
	"UseVAD":                           new(big.Int).SetUint64(1 << 25),
	"ChangeNickname":                   new(big.Int).SetUint64(1 << 26),
	"ManageNicknames":                  new(big.Int).SetUint64(1 << 27),
	"ManageRoles":                      new(big.Int).SetUint64(1 << 28),
	"ManageWebhooks":                   new(big.Int).SetUint64(1 << 29),
	"ManageGuildExpressions":           new(big.Int).SetUint64(1 << 30),
	"UseApplicationCommands":           new(big.Int).SetUint64(1 << 31),
	"RequestToSpeak":                   new(big.Int).SetUint64(1 << 32),
	"ManageEvents":                     new(big.Int).SetUint64(1 << 33),
	"ManageThreads":                    new(big.Int).SetUint64(1 << 34),
	"CreatePublicThreads":              new(big.Int).SetUint64(1 << 35),
	"CreatePrivateThreads":             new(big.Int).SetUint64(1 << 36),
	"UseExternalStickers":              new(big.Int).SetUint64(1 << 37),
	"SendMessagesInThreads":            new(big.Int).SetUint64(1 << 38),
	"UseEmbeddedActivities":            new(big.Int).SetUint64(1 << 39),
	"ModerateMembers":                  new(big.Int).SetUint64(1 << 40),
	"ViewCreatorMonetizationAnalytics": new(big.Int).SetUint64(1 << 41),
	"UseSoundboard":                    new(big.Int).SetUint64(1 << 42),
	"UseExternalSounds":                new(big.Int).SetUint64(1 << 45),
	"SendVoiceMessages":                new(big.Int).SetUint64(1 << 46),
}

// Discord 频道类型（线程相关）
const (
	channelTypeGuildNewsThread    = 10
	channelTypeGuildPublicThread  = 11
	channelTypeGuildPrivateThread = 12
	channelTypeGuildForum         = 15
	channelTypeGuildMedia         = 16
)

func addPermissionBits(base *big.Int, add string) *big.Int {
	if add == "" {
		return base
	}
	addBits, ok := new(big.Int).SetString(add, 10)
	if !ok {
		return base
	}
	return new(big.Int).Or(base, addBits)
}

func removePermissionBits(base *big.Int, deny string) *big.Int {
	if deny == "" {
		return base
	}
	denyBits, ok := new(big.Int).SetString(deny, 10)
	if !ok {
		return base
	}
	return new(big.Int).AndNot(base, denyBits)
}

func bitfieldToPermissions(bitfield *big.Int) []string {
	var perms []string
	for name, value := range permissionFlagBits {
		check := new(big.Int).And(bitfield, value)
		if check.Cmp(value) == 0 {
			perms = append(perms, name)
		}
	}
	sort.Strings(perms)
	return perms
}

// IsThreadChannelType 判断频道类型是否为线程
func IsThreadChannelType(channelType *int) bool {
	if channelType == nil {
		return false
	}
	t := *channelType
	return t == channelTypeGuildNewsThread || t == channelTypeGuildPublicThread || t == channelTypeGuildPrivateThread
}

// permissionOverwrite 权限覆盖
type permissionOverwrite struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Allow string `json:"allow,omitempty"`
	Deny  string `json:"deny,omitempty"`
}

// apiRole API 角色
type apiRole struct {
	ID          string `json:"id"`
	Permissions string `json:"permissions"`
}

// FetchChannelPermissionsDiscord 获取频道权限
func FetchChannelPermissionsDiscord(ctx context.Context, channelID string, token string) (*DiscordPermissionsSummary, error) {
	// 获取频道信息
	channelData, err := discordGET(ctx, fmt.Sprintf("/channels/%s", channelID), token)
	if err != nil {
		return nil, fmt.Errorf("fetch channel: %w", err)
	}

	var channel struct {
		Type                 *int                  `json:"type,omitempty"`
		GuildID              string                `json:"guild_id,omitempty"`
		PermissionOverwrites []permissionOverwrite `json:"permission_overwrites,omitempty"`
	}
	if err := json.Unmarshal(channelData, &channel); err != nil {
		return nil, fmt.Errorf("parse channel: %w", err)
	}

	// DM 频道
	if channel.GuildID == "" {
		return &DiscordPermissionsSummary{
			ChannelID:   channelID,
			Permissions: nil,
			Raw:         "0",
			IsDM:        true,
			ChannelType: channel.Type,
		}, nil
	}

	// 获取 bot 用户 ID
	meData, err := discordGET(ctx, "/users/@me", token)
	if err != nil {
		return nil, fmt.Errorf("fetch bot user: %w", err)
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(meData, &me); err != nil {
		return nil, fmt.Errorf("parse bot user: %w", err)
	}

	// 获取 guild 和 member 信息
	guildData, err := discordGET(ctx, fmt.Sprintf("/guilds/%s", channel.GuildID), token)
	if err != nil {
		return nil, fmt.Errorf("fetch guild: %w", err)
	}
	memberData, err := discordGET(ctx, fmt.Sprintf("/guilds/%s/members/%s", channel.GuildID, me.ID), token)
	if err != nil {
		return nil, fmt.Errorf("fetch member: %w", err)
	}

	var guild struct {
		Roles []apiRole `json:"roles"`
	}
	if err := json.Unmarshal(guildData, &guild); err != nil {
		return nil, fmt.Errorf("parse guild: %w", err)
	}

	var member struct {
		Roles []string `json:"roles"`
	}
	if err := json.Unmarshal(memberData, &member); err != nil {
		return nil, fmt.Errorf("parse member: %w", err)
	}

	// 构建角色映射
	rolesMap := make(map[string]apiRole)
	for _, role := range guild.Roles {
		rolesMap[role.ID] = role
	}

	// 计算基础权限
	base := new(big.Int)
	if everyoneRole, ok := rolesMap[channel.GuildID]; ok {
		base = addPermissionBits(base, everyoneRole.Permissions)
	}
	for _, roleID := range member.Roles {
		if role, ok := rolesMap[roleID]; ok {
			base = addPermissionBits(base, role.Permissions)
		}
	}

	// 应用频道覆盖
	permissions := new(big.Int).Set(base)

	// @everyone 覆盖
	for _, ow := range channel.PermissionOverwrites {
		if ow.ID == channel.GuildID {
			permissions = removePermissionBits(permissions, ow.Deny)
			permissions = addPermissionBits(permissions, ow.Allow)
		}
	}
	// 角色覆盖
	memberRoleSet := make(map[string]bool)
	for _, r := range member.Roles {
		memberRoleSet[r] = true
	}
	for _, ow := range channel.PermissionOverwrites {
		if memberRoleSet[ow.ID] {
			permissions = removePermissionBits(permissions, ow.Deny)
			permissions = addPermissionBits(permissions, ow.Allow)
		}
	}
	// 成员覆盖
	for _, ow := range channel.PermissionOverwrites {
		if ow.ID == me.ID {
			permissions = removePermissionBits(permissions, ow.Deny)
			permissions = addPermissionBits(permissions, ow.Allow)
		}
	}

	return &DiscordPermissionsSummary{
		ChannelID:   channelID,
		GuildID:     channel.GuildID,
		Permissions: bitfieldToPermissions(permissions),
		Raw:         permissions.String(),
		IsDM:        false,
		ChannelType: channel.Type,
	}, nil
}

// sendPermissionsContains 检查权限列表是否包含指定权限
func sendPermissionsContains(perms []string, perm string) bool {
	for _, p := range perms {
		if strings.EqualFold(p, perm) {
			return true
		}
	}
	return false
}

// _ 确保 sendPermissionsContains 被编译器接受
var _ = sendPermissionsContains
