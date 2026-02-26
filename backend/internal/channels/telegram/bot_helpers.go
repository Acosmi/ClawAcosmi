package telegram

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Telegram Bot 辅助函数 — 继承自 src/telegram/bot/helpers.ts (444L)

const telegramGeneralTopicID = 1

// utf16SliceToString 根据 UTF-16 偏移量从字符串中切片。
// Telegram Bot API 使用 UTF-16 code units 作为 entity offset（与 JavaScript 字符串一致）。
// Go 的 rune 是 UTF-32 code points，需要转换。
func utf16SliceToString(text string, offset, length int) string {
	runes := []rune(text)
	utf16Pos := 0
	startRune := -1
	endRune := -1
	target := offset + length

	for i, r := range runes {
		if utf16Pos == offset && startRune == -1 {
			startRune = i
		}
		if r > 0xFFFF {
			utf16Pos += 2 // supplementary plane: 2 UTF-16 code units
		} else {
			utf16Pos++
		}
		if utf16Pos >= target && endRune == -1 {
			endRune = i + 1
			break
		}
	}
	if startRune == -1 {
		return ""
	}
	if endRune == -1 {
		endRune = len(runes)
	}
	return string(runes[startRune:endRune])
}

// utf16SpliceRunes 根据 UTF-16 偏移量替换字符串中的片段（从后向前替换用）。
func utf16SpliceRunes(runes []rune, offset, length int, replacement []rune) []rune {
	utf16Pos := 0
	startRune := -1
	endRune := -1
	target := offset + length

	for i, r := range runes {
		if utf16Pos == offset && startRune == -1 {
			startRune = i
		}
		if r > 0xFFFF {
			utf16Pos += 2
		} else {
			utf16Pos++
		}
		if utf16Pos >= target && endRune == -1 {
			endRune = i + 1
			break
		}
	}
	if startRune == -1 {
		return runes
	}
	if endRune == -1 {
		endRune = len(runes)
	}
	result := make([]rune, 0, len(runes)-endRune+startRune+len(replacement))
	result = append(result, runes[:startRune]...)
	result = append(result, replacement...)
	result = append(result, runes[endRune:]...)
	return result
}

// TelegramThreadSpec 线程/话题配置
type TelegramThreadSpec struct {
	ID    *int
	Scope string // "dm", "forum", "none"
}

// ResolveTelegramForumThreadID 解析论坛话题 ID
func ResolveTelegramForumThreadID(isForum bool, messageThreadID *int) *int {
	if !isForum {
		return nil
	}
	if messageThreadID == nil {
		id := telegramGeneralTopicID
		return &id
	}
	return messageThreadID
}

// ResolveTelegramThreadSpec 解析线程规范
func ResolveTelegramThreadSpec(isGroup, isForum bool, messageThreadID *int) TelegramThreadSpec {
	if isGroup {
		id := ResolveTelegramForumThreadID(isForum, messageThreadID)
		scope := "none"
		if isForum {
			scope = "forum"
		}
		return TelegramThreadSpec{ID: id, Scope: scope}
	}
	if messageThreadID == nil {
		return TelegramThreadSpec{Scope: "dm"}
	}
	return TelegramThreadSpec{ID: messageThreadID, Scope: "dm"}
}

// BuildTelegramThreadParams 构建 API 调用的线程参数
func BuildTelegramThreadParams(thread *TelegramThreadSpec) map[string]int {
	if thread == nil || thread.ID == nil {
		return nil
	}
	normalized := int(math.Trunc(float64(*thread.ID)))
	if normalized == telegramGeneralTopicID && thread.Scope == "forum" {
		return nil
	}
	return map[string]int{"message_thread_id": normalized}
}

// BuildTypingThreadParams 构建打字指示器的线程参数
func BuildTypingThreadParams(messageThreadID *int) map[string]int {
	if messageThreadID == nil {
		return nil
	}
	return map[string]int{"message_thread_id": int(math.Trunc(float64(*messageThreadID)))}
}

// ResolveTelegramStreamMode 解析流模式
func ResolveTelegramStreamMode(streamMode string) TelegramStreamMode {
	raw := strings.TrimSpace(strings.ToLower(streamMode))
	switch TelegramStreamMode(raw) {
	case StreamModeOff, StreamModePartial, StreamModeBlock:
		return TelegramStreamMode(raw)
	default:
		return StreamModePartial
	}
}

// BuildTelegramGroupPeerID 构建群组 peer ID
func BuildTelegramGroupPeerID(chatID interface{}, messageThreadID *int) string {
	id := fmt.Sprintf("%v", chatID)
	if messageThreadID != nil {
		return fmt.Sprintf("%s:topic:%d", id, *messageThreadID)
	}
	return id
}

// BuildTelegramGroupFrom 构建群组 from 标识
func BuildTelegramGroupFrom(chatID interface{}, messageThreadID *int) string {
	return "telegram:group:" + BuildTelegramGroupPeerID(chatID, messageThreadID)
}

// BuildTelegramParentPeer 构建父级 peer（用于绑定继承）
func BuildTelegramParentPeer(isGroup bool, resolvedThreadID *int, chatID interface{}) *struct {
	Kind string
	ID   string
} {
	if !isGroup || resolvedThreadID == nil {
		return nil
	}
	return &struct {
		Kind string
		ID   string
	}{Kind: "group", ID: fmt.Sprintf("%v", chatID)}
}

// BuildSenderName 从消息中提取发送者名称
func BuildSenderName(msg *TelegramMessage) string {
	if msg.From == nil {
		return ""
	}
	parts := []string{}
	if msg.From.FirstName != "" {
		parts = append(parts, msg.From.FirstName)
	}
	if msg.From.LastName != "" {
		parts = append(parts, msg.From.LastName)
	}
	name := strings.TrimSpace(strings.Join(parts, " "))
	if name != "" {
		return name
	}
	return msg.From.Username
}

// BuildSenderLabel 构建发送者标签
func BuildSenderLabel(msg *TelegramMessage, senderID string) string {
	name := BuildSenderName(msg)
	var username string
	if msg.From != nil && msg.From.Username != "" {
		username = "@" + msg.From.Username
	}

	var label string
	if name != "" && username != "" {
		label = fmt.Sprintf("%s (%s)", name, username)
	} else if name != "" {
		label = name
	} else if username != "" {
		label = username
	}

	fallbackID := strings.TrimSpace(senderID)
	if fallbackID == "" && msg.From != nil {
		fallbackID = strconv.FormatInt(msg.From.ID, 10)
	}

	idPart := ""
	if fallbackID != "" {
		idPart = "id:" + fallbackID
	}

	if label != "" && idPart != "" {
		return label + " " + idPart
	}
	if label != "" {
		return label
	}
	if idPart != "" {
		return idPart
	}
	return "id:unknown"
}

// BuildGroupLabel 构建群组标签
func BuildGroupLabel(msg *TelegramMessage, chatID interface{}, messageThreadID *int) string {
	title := msg.Chat.Title
	topicSuffix := ""
	if messageThreadID != nil {
		topicSuffix = fmt.Sprintf(" topic:%d", *messageThreadID)
	}
	if title != "" {
		return fmt.Sprintf("%s id:%v%s", title, chatID, topicSuffix)
	}
	return fmt.Sprintf("group:%v%s", chatID, topicSuffix)
}

// HasBotMention 检查消息中是否提及了 bot
func HasBotMention(msg *TelegramMessage, botUsername string) bool {
	text := strings.ToLower(msg.Text)
	if text == "" {
		text = strings.ToLower(msg.Caption)
	}
	mention := "@" + strings.ToLower(botUsername)
	if strings.Contains(text, mention) {
		return true
	}

	entities := msg.Entities
	if len(entities) == 0 {
		entities = msg.CaptionEntities
	}

	rawText := msg.Text
	if rawText == "" {
		rawText = msg.Caption
	}

	for _, ent := range entities {
		if ent.Type != "mention" {
			continue
		}
		// Telegram entity offsets are in UTF-16 code units
		slice := strings.ToLower(utf16SliceToString(rawText, ent.Offset, ent.Length))
		if slice == mention {
			return true
		}
	}
	return false
}

// ExpandTextLinks 展开 text_link 实体为 Markdown 链接
func ExpandTextLinks(text string, entities []TelegramEntity) string {
	if text == "" || len(entities) == 0 {
		return text
	}

	// 筛选 text_link 实体并按 offset 逆序
	var textLinks []TelegramEntity
	for _, e := range entities {
		if e.Type == "text_link" && e.URL != "" {
			textLinks = append(textLinks, e)
		}
	}
	if len(textLinks) == 0 {
		return text
	}

	// 逆序排列以便从后向前替换
	for i := 0; i < len(textLinks); i++ {
		for j := i + 1; j < len(textLinks); j++ {
			if textLinks[i].Offset < textLinks[j].Offset {
				textLinks[i], textLinks[j] = textLinks[j], textLinks[i]
			}
		}
	}

	// 从后向前替换，使用 UTF-16 偏移量（Telegram entity offset 为 UTF-16 code units）
	runes := []rune(text)
	for _, ent := range textLinks {
		linkText := utf16SliceToString(text, ent.Offset, ent.Length)
		markdown := fmt.Sprintf("[%s](%s)", linkText, ent.URL)
		runes = utf16SpliceRunes(runes, ent.Offset, ent.Length, []rune(markdown))
	}
	return string(runes)
}

// ResolveTelegramReplyID 解析回复消息 ID
func ResolveTelegramReplyID(raw string) *int {
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &n
}

// TelegramReplyTarget 回复目标描述
type TelegramReplyTarget struct {
	ID     string
	Sender string
	Body   string
	Kind   string // "reply" | "quote"
}

// DescribeReplyTarget 描述消息的回复目标
func DescribeReplyTarget(msg *TelegramMessage) *TelegramReplyTarget {
	reply := msg.ReplyToMessage
	quote := msg.Quote

	var body string
	kind := "reply"

	if quote != nil && strings.TrimSpace(quote.Text) != "" {
		body = strings.TrimSpace(quote.Text)
		kind = "quote"
	}

	if body == "" && reply != nil {
		replyBody := strings.TrimSpace(reply.Text)
		if replyBody == "" {
			replyBody = strings.TrimSpace(reply.Caption)
		}
		body = replyBody
		if body == "" {
			if len(reply.Photo) > 0 {
				body = "<media:image>"
			} else if reply.Video != nil {
				body = "<media:video>"
			} else if reply.Audio != nil || reply.Voice != nil {
				body = "<media:audio>"
			} else if reply.Document != nil {
				body = "<media:document>"
			} else if loc := ExtractTelegramLocation(reply); loc != nil {
				body = FormatLocationText(loc)
			}
		}
	}

	if body == "" {
		return nil
	}

	sender := "unknown sender"
	if reply != nil {
		if s := BuildSenderName(reply); s != "" {
			sender = s
		}
	}

	var id string
	if reply != nil {
		id = strconv.Itoa(reply.MessageID)
	}

	return &TelegramReplyTarget{ID: id, Sender: sender, Body: body, Kind: kind}
}

// TelegramForwardedContext 转发上下文
type TelegramForwardedContext struct {
	From          string
	Date          int
	FromType      string
	FromID        string
	FromUsername  string
	FromTitle     string
	FromSignature string
	FromChatType  string
	FromMessageID int
}

// NormalizeForwardedContext 从消息中提取转发来源
func NormalizeForwardedContext(msg *TelegramMessage) *TelegramForwardedContext {
	origin := msg.ForwardOrigin
	if origin == nil {
		return nil
	}

	switch origin.Type {
	case "user":
		if origin.SenderUser == nil {
			return nil
		}
		u := origin.SenderUser
		label := buildUserLabel(u)
		return &TelegramForwardedContext{
			From:         label,
			Date:         origin.Date,
			FromType:     "user",
			FromID:       strconv.FormatInt(u.ID, 10),
			FromUsername: u.Username,
			FromTitle:    strings.TrimSpace(u.FirstName + " " + u.LastName),
		}
	case "hidden_user":
		name := strings.TrimSpace(origin.SenderUserName)
		if name == "" {
			return nil
		}
		return &TelegramForwardedContext{
			From: name, Date: origin.Date, FromType: "hidden_user", FromTitle: name,
		}
	case "chat":
		if origin.SenderChat == nil {
			return nil
		}
		return buildForwardedChat(origin.SenderChat, origin.Date, "chat", origin.AuthorSig, 0)
	case "channel":
		if origin.Chat == nil {
			return nil
		}
		return buildForwardedChat(origin.Chat, origin.Date, "channel", origin.AuthorSig, origin.MessageID)
	default:
		return nil
	}
}

func buildUserLabel(u *TelegramUser) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" && u.Username != "" {
		return fmt.Sprintf("%s (@%s)", name, u.Username)
	}
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("user:%d", u.ID)
}

func buildForwardedChat(chat *TelegramChat, date int, fwdType, sig string, msgID int) *TelegramForwardedContext {
	title := strings.TrimSpace(chat.Title)
	username := strings.TrimSpace(chat.Username)
	id := strconv.FormatInt(chat.ID, 10)

	display := title
	if display == "" && username != "" {
		display = "@" + username
	}
	if display == "" {
		display = fwdType + ":" + id
	}

	from := display
	if s := strings.TrimSpace(sig); s != "" {
		from = fmt.Sprintf("%s (%s)", display, s)
	}

	return &TelegramForwardedContext{
		From:          from,
		Date:          date,
		FromType:      fwdType,
		FromID:        id,
		FromUsername:  username,
		FromTitle:     title,
		FromSignature: strings.TrimSpace(sig),
		FromChatType:  chat.Type,
		FromMessageID: msgID,
	}
}

// ExtractTelegramLocation 从消息中提取位置信息
func ExtractTelegramLocation(msg *TelegramMessage) *NormalizedLocation {
	if msg.Venue != nil {
		return &NormalizedLocation{
			Latitude:  msg.Venue.Location.Latitude,
			Longitude: msg.Venue.Location.Longitude,
			Accuracy:  msg.Venue.Location.HorizontalAccuracy,
			Name:      msg.Venue.Title,
			Address:   msg.Venue.Address,
			Source:    "place",
		}
	}
	if msg.Location != nil {
		isLive := msg.Location.LivePeriod != nil && *msg.Location.LivePeriod > 0
		source := "pin"
		if isLive {
			source = "live"
		}
		return &NormalizedLocation{
			Latitude:  msg.Location.Latitude,
			Longitude: msg.Location.Longitude,
			Accuracy:  msg.Location.HorizontalAccuracy,
			Source:    source,
			IsLive:    isLive,
		}
	}
	return nil
}

// resolveLocationSource 推断位置来源（对齐 TS location.ts resolveLocation）。
func resolveLocationSource(loc *NormalizedLocation) string {
	if loc.Source != "" {
		return loc.Source
	}
	if loc.IsLive {
		return "live"
	}
	if loc.Name != "" || loc.Address != "" {
		return "place"
	}
	return "pin"
}

// formatCoords 格式化坐标为 6 位小数（对齐 TS formatCoords）。
func formatCoords(lat, lon float64) string {
	return fmt.Sprintf("%.6f, %.6f", lat, lon)
}

// formatAccuracy 格式化精度（对齐 TS formatAccuracy）。
func formatAccuracy(accuracy *float64) string {
	if accuracy == nil {
		return ""
	}
	return fmt.Sprintf(" \u00b1%.0fm", math.Round(*accuracy))
}

// FormatLocationText 格式化位置描述文本。
// 对齐 TS location.ts formatLocationText: emoji + 精度 + caption + live/place/pin 区分。
func FormatLocationText(loc *NormalizedLocation) string {
	if loc == nil {
		return ""
	}
	source := resolveLocationSource(loc)
	coords := formatCoords(loc.Latitude, loc.Longitude)
	accuracy := formatAccuracy(loc.Accuracy)

	var header string
	if source == "live" || loc.IsLive {
		header = fmt.Sprintf("\U0001f6f0 Live location: %s%s", coords, accuracy)
	} else if loc.Name != "" || loc.Address != "" {
		parts := make([]string, 0, 2)
		if loc.Name != "" {
			parts = append(parts, loc.Name)
		}
		if loc.Address != "" {
			parts = append(parts, loc.Address)
		}
		label := strings.Join(parts, " \u2014 ")
		header = fmt.Sprintf("\U0001f4cd %s (%s%s)", label, coords, accuracy)
	} else {
		header = fmt.Sprintf("\U0001f4cd %s%s", coords, accuracy)
	}

	if loc.Caption != "" {
		caption := strings.TrimSpace(loc.Caption)
		if caption != "" {
			return header + "\n" + caption
		}
	}
	return header
}

// ResolveTelegramTargetChatType 解析目标聊天类型
func ResolveTelegramTargetChatType(target string) string {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "unknown"
	}
	parsed := ParseTelegramTarget(trimmed)
	chatID := strings.TrimSpace(parsed.ChatID)
	if chatID == "" {
		return "unknown"
	}
	if matched, _ := regexp.MatchString(`^-?\d+$`, chatID); matched {
		if strings.HasPrefix(chatID, "-") {
			return "group"
		}
		return "direct"
	}
	return "unknown"
}
