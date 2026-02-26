package discord

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Discord Emoji/Sticker 上传 — 继承自 src/discord/send.emojis-stickers.ts (58L)

// 允许的 emoji MIME 类型
var emojiAllowedMIME = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/jpg":  true,
	"image/gif":  true,
}

// 允许的 sticker MIME 类型
var stickerAllowedMIME = map[string]bool{
	"image/png":        true,
	"image/apng":       true,
	"application/json": true,
}

// UploadEmojiDiscord 上传自定义 emoji
// 继承自 send.emojis-stickers.ts uploadEmojiDiscord (L12-31)
func UploadEmojiDiscord(ctx context.Context, payload DiscordEmojiUpload, token string) (json.RawMessage, error) {
	media, err := loadDiscordMediaWithLimit(payload.MediaURL, DiscordMaxEmojiBytes)
	if err != nil {
		return nil, fmt.Errorf("upload emoji: %w", err)
	}

	contentType := strings.ToLower(media.ContentType)
	if !emojiAllowedMIME[contentType] {
		return nil, fmt.Errorf("discord emoji uploads require a PNG, JPG, or GIF image (got %s)", contentType)
	}

	// data URI 格式：data:<mime>;base64,<data>
	image := fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(media.Data))

	name, err := NormalizeEmojiName(payload.Name, "Emoji name")
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"name":  name,
		"image": image,
	}

	// 处理 roleIds
	var roleIDs []string
	for _, id := range payload.RoleIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			roleIDs = append(roleIDs, trimmed)
		}
	}
	if len(roleIDs) > 0 {
		body["roles"] = roleIDs
	}

	return discordPOST(ctx, fmt.Sprintf("/guilds/%s/emojis", payload.GuildID), token, body)
}

// UploadStickerDiscord 上传自定义 sticker
// 继承自 send.emojis-stickers.ts uploadStickerDiscord (L33-57)
func UploadStickerDiscord(ctx context.Context, payload DiscordStickerUpload, token string) (json.RawMessage, error) {
	media, err := loadDiscordMediaWithLimit(payload.MediaURL, DiscordMaxStickerBytes)
	if err != nil {
		return nil, fmt.Errorf("upload sticker: %w", err)
	}

	contentType := strings.ToLower(media.ContentType)
	if !stickerAllowedMIME[contentType] {
		return nil, fmt.Errorf("discord sticker uploads require a PNG, APNG, or Lottie JSON file (got %s)", contentType)
	}

	name, err := NormalizeEmojiName(payload.Name, "Sticker name")
	if err != nil {
		return nil, err
	}
	description, err := NormalizeEmojiName(payload.Description, "Sticker description")
	if err != nil {
		return nil, err
	}
	tags, err := NormalizeEmojiName(payload.Tags, "Sticker tags")
	if err != nil {
		return nil, err
	}

	// Sticker 上传使用 multipart/form-data
	stickerPayload := map[string]interface{}{
		"name":        name,
		"description": description,
		"tags":        tags,
	}

	stickerMedia := &discordMedia{
		Data:        media.Data,
		FileName:    media.FileName,
		ContentType: contentType,
	}
	if stickerMedia.FileName == "upload" {
		stickerMedia.FileName = "sticker"
	}

	return discordMultipartPOST(ctx, fmt.Sprintf("/guilds/%s/stickers", payload.GuildID), token, stickerPayload, stickerMedia)
}
