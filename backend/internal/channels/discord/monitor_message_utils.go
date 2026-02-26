package discord

import (
	"fmt"
	"strings"
)

// Discord 消息工具 — 继承自 src/discord/monitor/message-utils.ts (287L)
// 仅移植类型和纯逻辑函数；media fetch/store 依赖延迟到 Phase 7

// DiscordMediaInfo 媒体信息
type DiscordMediaInfo struct {
	Path        string `json:"path"`
	ContentType string `json:"contentType,omitempty"`
	Placeholder string `json:"placeholder"`
}

// DiscordChannelInfo 频道信息
type DiscordChannelInfo struct {
	Type     int    `json:"type"`
	Name     string `json:"name,omitempty"`
	Topic    string `json:"topic,omitempty"`
	ParentID string `json:"parentId,omitempty"`
	OwnerID  string `json:"ownerId,omitempty"`
}

// DiscordSnapshotAuthor 快照作者
type DiscordSnapshotAuthor struct {
	ID            string `json:"id,omitempty"`
	Username      string `json:"username,omitempty"`
	Discriminator string `json:"discriminator,omitempty"`
	GlobalName    string `json:"global_name,omitempty"`
	Name          string `json:"name,omitempty"`
}

// DiscordSnapshotAttachment 快照附件
type DiscordSnapshotAttachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Size        int    `json:"size,omitempty"`
	URL         string `json:"url,omitempty"`
	ProxyURL    string `json:"proxy_url,omitempty"`
}

// DiscordSnapshotEmbed 快照嵌入
type DiscordSnapshotEmbed struct {
	Description string `json:"description,omitempty"`
	Title       string `json:"title,omitempty"`
}

// DiscordSnapshotMessage 快照消息
type DiscordSnapshotMessage struct {
	Content     string                      `json:"content,omitempty"`
	Embeds      []DiscordSnapshotEmbed      `json:"embeds,omitempty"`
	Attachments []DiscordSnapshotAttachment `json:"attachments,omitempty"`
	Author      *DiscordSnapshotAuthor      `json:"author,omitempty"`
}

// DiscordMessageSnapshot 消息快照
type DiscordMessageSnapshot struct {
	Message *DiscordSnapshotMessage `json:"message,omitempty"`
}

// InferAttachmentPlaceholder 推断附件占位符
// TS ref: uses <media:TYPE> format (e.g. <media:image>, <media:video>, etc.)
func InferAttachmentPlaceholder(filename, contentType string) string {
	ct := strings.ToLower(contentType)
	if strings.HasPrefix(ct, "image/") {
		return fmt.Sprintf("<media:image>%s</media:image>", filename)
	}
	if strings.HasPrefix(ct, "video/") {
		return fmt.Sprintf("<media:video>%s</media:video>", filename)
	}
	if strings.HasPrefix(ct, "audio/") {
		return fmt.Sprintf("<media:audio>%s</media:audio>", filename)
	}
	return fmt.Sprintf("<media:file>%s</media:file>", filename)
}

// InferStickerPlaceholder 推断贴纸占位符
// TS ref: uses <media:sticker> format for sticker attachments
func InferStickerPlaceholder(stickerName string) string {
	return fmt.Sprintf("<media:sticker>%s</media:sticker>", stickerName)
}

// BuildDiscordAttachmentPlaceholder 构建附件占位符
func BuildDiscordAttachmentPlaceholder(attachments []DiscordSnapshotAttachment) string {
	if len(attachments) == 0 {
		return ""
	}
	var parts []string
	for _, a := range attachments {
		parts = append(parts, InferAttachmentPlaceholder(a.Filename, a.ContentType))
	}
	return strings.Join(parts, "\n")
}

// ResolveDiscordSnapshotMessageText 从快照提取文本
func ResolveDiscordSnapshotMessageText(snap DiscordSnapshotMessage) string {
	var parts []string
	if snap.Content != "" {
		parts = append(parts, snap.Content)
	}
	for _, embed := range snap.Embeds {
		if embed.Description != "" {
			parts = append(parts, embed.Description)
		} else if embed.Title != "" {
			parts = append(parts, embed.Title)
		}
	}
	return strings.Join(parts, "\n")
}

// FormatDiscordSnapshotAuthor 格式化快照作者
func FormatDiscordSnapshotAuthor(author *DiscordSnapshotAuthor) string {
	if author == nil {
		return ""
	}
	display := strings.TrimSpace(author.GlobalName)
	if display == "" {
		display = strings.TrimSpace(author.Name)
	}
	if display == "" {
		display = strings.TrimSpace(author.Username)
	}
	if display == "" {
		return ""
	}
	disc := strings.TrimSpace(author.Discriminator)
	if disc != "" && disc != "0" && author.Username != "" {
		tag := author.Username + "#" + disc
		if tag != display {
			return display + " (" + tag + ")"
		}
	}
	return display
}

// BuildDiscordMediaPayload 构建媒体负载
func BuildDiscordMediaPayload(mediaList []DiscordMediaInfo) map[string]interface{} {
	if len(mediaList) == 0 {
		return nil
	}
	result := map[string]interface{}{}
	if len(mediaList) == 1 {
		m := mediaList[0]
		result["MediaPath"] = m.Path
		if m.ContentType != "" {
			result["MediaType"] = m.ContentType
		}
		return result
	}
	var paths, types []string
	for _, m := range mediaList {
		paths = append(paths, m.Path)
		types = append(types, m.ContentType)
	}
	result["MediaPaths"] = paths
	result["MediaTypes"] = types
	return result
}
