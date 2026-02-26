package line

import "fmt"

// TS 对照: src/line/flex-templates.ts (1512L)
// 审计补全: 核心 Flex 卡片模板构建器

// ---------- 辅助类型 ----------

// ListItem 列表项。
type ListItem struct {
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle,omitempty"`
	Action   *FlexAction `json:"action,omitempty"`
}

// CardAction 卡片按钮动作。
type CardAction struct {
	Label  string     `json:"label"`
	Action FlexAction `json:"action"`
}

// ---------- Info Card ----------

// CreateInfoCard 创建信息卡片。
func CreateInfoCard(title, body, footer string) FlexBubble {
	bubble := NewFlexBubble()
	bubble.Size = "mega"

	// Header
	headerBox := NewFlexBox("vertical",
		NewFlexText(title, "lg", "bold", "#1DB446"),
	)
	headerBox.Padding = "lg"
	bubble.Header = &headerBox

	// Body
	bodyContents := []FlexComponent{
		NewFlexText(body, "sm", "regular", "#555555"),
	}
	bodyBox := NewFlexBox("vertical", bodyContents...)
	bodyBox.Padding = "lg"
	bodyBox.Spacing = "sm"
	bubble.Body = &bodyBox

	// Footer
	if footer != "" {
		footerBox := NewFlexBox("vertical",
			NewFlexText(footer, "xs", "regular", "#aaaaaa"),
		)
		footerBox.Padding = "lg"
		bubble.Footer = &footerBox
	}

	return bubble
}

// ---------- List Card ----------

// CreateListCard 创建列表卡片。
func CreateListCard(title string, items []ListItem) FlexBubble {
	bubble := NewFlexBubble()
	bubble.Size = "mega"

	headerBox := NewFlexBox("vertical",
		NewFlexText(title, "lg", "bold", "#1DB446"),
	)
	headerBox.Padding = "lg"
	bubble.Header = &headerBox

	bodyContents := make([]FlexComponent, 0, len(items)*2)
	for i, item := range items {
		if i > 0 {
			bodyContents = append(bodyContents, NewFlexSeparator())
		}
		label := fmt.Sprintf("• %s", item.Title)
		text := NewFlexText(label, "sm", "regular", "#555555")
		if item.Action != nil {
			text.Action = item.Action
		}
		bodyContents = append(bodyContents, text)
		if item.Subtitle != "" {
			bodyContents = append(bodyContents, NewFlexText("  "+item.Subtitle, "xs", "regular", "#aaaaaa"))
		}
	}

	bodyBox := NewFlexBox("vertical", bodyContents...)
	bodyBox.Padding = "lg"
	bodyBox.Spacing = "sm"
	bubble.Body = &bodyBox
	return bubble
}

// ---------- Image Card ----------

// CreateImageCard 创建图片卡片。
func CreateImageCard(imageURL, title, body string) FlexBubble {
	bubble := NewFlexBubble()
	bubble.Size = "mega"

	// Image as hero component is represented via body with image
	headerBox := NewFlexBox("vertical",
		FlexComponent{
			Type: "image",
			Text: imageURL, // URL stored in text field for simplicity
			Size: "full",
		},
	)
	bubble.Header = &headerBox

	bodyContents := []FlexComponent{
		NewFlexText(title, "lg", "bold", "#333333"),
	}
	if body != "" {
		bodyContents = append(bodyContents, NewFlexText(body, "sm", "regular", "#555555"))
	}
	bodyBox := NewFlexBox("vertical", bodyContents...)
	bodyBox.Padding = "lg"
	bodyBox.Spacing = "sm"
	bubble.Body = &bodyBox
	return bubble
}

// ---------- Action Card ----------

// CreateActionCard 创建带按钮的操作卡片。
func CreateActionCard(title, body string, actions []CardAction) FlexBubble {
	bubble := NewFlexBubble()
	bubble.Size = "mega"

	headerBox := NewFlexBox("vertical",
		NewFlexText(title, "lg", "bold", "#1DB446"),
	)
	headerBox.Padding = "lg"
	bubble.Header = &headerBox

	bodyBox := NewFlexBox("vertical",
		NewFlexText(body, "sm", "regular", "#555555"),
	)
	bodyBox.Padding = "lg"
	bubble.Body = &bodyBox

	if len(actions) > 0 {
		footerContents := make([]FlexComponent, len(actions))
		for i, a := range actions {
			footerContents[i] = FlexComponent{
				Type:   "button",
				Action: &FlexAction{Type: a.Action.Type, Label: a.Label, URI: a.Action.URI, Text: a.Action.Text, Data: a.Action.Data},
			}
		}
		footerBox := NewFlexBox("vertical", footerContents...)
		footerBox.Spacing = "sm"
		bubble.Footer = &footerBox
	}
	return bubble
}

// ---------- Notification Bubble ----------

// NotificationType 通知类型。
type NotificationType string

const (
	NotifyInfo    NotificationType = "info"
	NotifySuccess NotificationType = "success"
	NotifyWarning NotificationType = "warning"
	NotifyError   NotificationType = "error"
)

func notifyColor(t NotificationType) string {
	switch t {
	case NotifySuccess:
		return "#1DB446"
	case NotifyWarning:
		return "#FFB300"
	case NotifyError:
		return "#DD2C00"
	default:
		return "#2196F3"
	}
}

func notifyIcon(t NotificationType) string {
	switch t {
	case NotifySuccess:
		return "✅"
	case NotifyWarning:
		return "⚠️"
	case NotifyError:
		return "❌"
	default:
		return "ℹ️"
	}
}

// CreateNotificationBubble 创建通知气泡。
func CreateNotificationBubble(text string, t NotificationType, title string) FlexBubble {
	bubble := NewFlexBubble()
	bubble.Size = "kilo"
	color := notifyColor(t)
	icon := notifyIcon(t)

	contents := []FlexComponent{
		NewFlexText(icon+" "+title, "md", "bold", color),
		NewFlexText(text, "sm", "regular", "#555555"),
	}
	bodyBox := NewFlexBox("vertical", contents...)
	bodyBox.Padding = "lg"
	bodyBox.Spacing = "sm"
	bubble.Body = &bodyBox
	return bubble
}

// ---------- Receipt Card ----------

// ReceiptItem 收据行。
type ReceiptItem struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Highlight bool   `json:"highlight,omitempty"`
}

// ReceiptTotal 收据合计。
type ReceiptTotal struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// CreateReceiptCard 创建收据/摘要卡片。
func CreateReceiptCard(title, subtitle string, items []ReceiptItem, total *ReceiptTotal, footer string) FlexBubble {
	bubble := NewFlexBubble()
	bubble.Size = "mega"

	headerContents := []FlexComponent{
		NewFlexText(title, "lg", "bold", "#1DB446"),
	}
	if subtitle != "" {
		headerContents = append(headerContents, NewFlexText(subtitle, "xs", "regular", "#aaaaaa"))
	}
	headerBox := NewFlexBox("vertical", headerContents...)
	headerBox.Padding = "lg"
	bubble.Header = &headerBox

	bodyContents := make([]FlexComponent, 0, len(items)*2+2)
	for _, item := range items {
		weight := "regular"
		color := "#555555"
		if item.Highlight {
			weight = "bold"
			color = "#1DB446"
		}
		row := NewFlexBox("horizontal",
			NewFlexText(item.Name, "sm", "regular", "#555555"),
			NewFlexText(item.Value, "sm", weight, color),
		)
		bodyContents = append(bodyContents, FlexComponent{Type: "box"})
		bodyContents[len(bodyContents)-1] = FlexComponent{
			Type:   "text",
			Text:   item.Name + ": " + item.Value,
			Size:   "sm",
			Color:  color,
			Weight: weight,
		}
		_ = row
	}

	if total != nil {
		bodyContents = append(bodyContents, NewFlexSeparator())
		bodyContents = append(bodyContents, NewFlexText(total.Label+": "+total.Value, "md", "bold", "#1DB446"))
	}

	bodyBox := NewFlexBox("vertical", bodyContents...)
	bodyBox.Padding = "lg"
	bodyBox.Spacing = "sm"
	bubble.Body = &bodyBox

	if footer != "" {
		footerBox := NewFlexBox("vertical",
			NewFlexText(footer, "xs", "regular", "#aaaaaa"),
		)
		bubble.Footer = &footerBox
	}
	return bubble
}

// ---------- Carousel ----------

// CreateCarousel 创建轮播容器(最多 12 个 bubble)。
func CreateCarousel(bubbles []FlexBubble) FlexContainer {
	if len(bubbles) > 12 {
		bubbles = bubbles[:12]
	}
	return FlexContainer{
		Type:     "carousel",
		Contents: bubbles,
	}
}

// CreateCarouselMessage 将轮播包装为 FlexMessage。
func CreateCarouselMessage(altText string, bubbles []FlexBubble) FlexMessage {
	return FlexMessage{
		Type:     "flex",
		AltText:  altText,
		Contents: CreateCarousel(bubbles),
	}
}
