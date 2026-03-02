package media

// ============================================================================
// media/types.go — oa-media sub-agent public types
// All shared data structures for trending topics, content drafts,
// publish results, and social interactions.
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P0-2, P0-3
// ============================================================================

import (
	"time"

	"github.com/openacosmi/claw-acismi/internal/channels"
)

// ---------- Channel IDs (P0-3) ----------
// Defined here to avoid modifying the main channels.go during development.
// TODO(integration): merge into channels/channels.go when oa-media is integrated.

const (
	// ChannelWeChatMP WeChat Official Account (公众号) channel.
	ChannelWeChatMP channels.ChannelID = "wechat_mp"
	// ChannelXiaohongshu Xiaohongshu (小红书) channel.
	ChannelXiaohongshu channels.ChannelID = "xiaohongshu"
)

// ---------- Platform / Style / Status enums ----------

// Platform target publishing platform.
type Platform string

const (
	PlatformWeChat      Platform = "wechat"
	PlatformXiaohongshu Platform = "xiaohongshu"
	PlatformWebsite     Platform = "website"
)

// ContentStyle content writing style.
type ContentStyle string

const (
	StyleInformative  ContentStyle = "informative"
	StyleCasual       ContentStyle = "casual"
	StyleProfessional ContentStyle = "professional"
)

// isValidPlatform 校验 Platform 枚举值。
func isValidPlatform(p Platform) bool {
	switch p {
	case PlatformWeChat, PlatformXiaohongshu, PlatformWebsite:
		return true
	default:
		return false
	}
}

// isValidStyle 校验 ContentStyle 枚举值。
func isValidStyle(s ContentStyle) bool {
	switch s {
	case StyleInformative, StyleCasual, StyleProfessional:
		return true
	default:
		return false
	}
}

// DraftStatus lifecycle status of a content draft.
type DraftStatus string

const (
	DraftStatusDraft         DraftStatus = "draft"
	DraftStatusPendingReview DraftStatus = "pending_review"
	DraftStatusApproved      DraftStatus = "approved"
	DraftStatusPublished     DraftStatus = "published"
)

// InteractionType type of social interaction.
type InteractionType string

const (
	InteractionComment InteractionType = "comment"
	InteractionDM      InteractionType = "dm"
)

// ---------- Core data structures ----------

// TrendingTopic represents a single trending topic from any source.
type TrendingTopic struct {
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	URL       string    `json:"url,omitempty"`
	HeatScore float64   `json:"heat_score"`
	Category  string    `json:"category,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

// ContentDraft represents a content draft awaiting review or publication.
type ContentDraft struct {
	ID        string       `json:"id"`
	Title     string       `json:"title"`
	Body      string       `json:"body"`
	Images    []string     `json:"images,omitempty"`
	Tags      []string     `json:"tags,omitempty"`
	Platform  Platform     `json:"platform"`
	Style     ContentStyle `json:"style"`
	Status    DraftStatus  `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// PublishResult represents the outcome of a publish operation.
type PublishResult struct {
	Platform    Platform  `json:"platform"`
	PostID      string    `json:"post_id,omitempty"`
	URL         string    `json:"url,omitempty"`
	Status      string    `json:"status"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// InteractionItem represents a social interaction (comment or DM).
type InteractionItem struct {
	Type       InteractionType `json:"type"`
	Platform   Platform        `json:"platform"`
	NoteID     string          `json:"note_id,omitempty"`
	AuthorName string          `json:"author_name"`
	Content    string          `json:"content"`
	Timestamp  time.Time       `json:"timestamp"`
}
