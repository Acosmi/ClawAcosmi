package feishu

// cards.go — 飞书互动卡片构建器（处理中/完成/失败）
// 所有卡片 config 含 "update_multi": true 以支持 Patch API 原地更新。

import "encoding/json"

// maxDivTextLen 飞书单个 lark_md div 元素的推荐上限（字符数）。
const maxDivTextLen = 2500

// BuildProcessingCard 构建"正在处理"蓝色卡片 JSON。
// 显示通用处理提示，不回显用户原文。
func BuildProcessingCard(_ string) string {
	card := map[string]interface{}{
		"config": map[string]interface{}{
			"update_multi": true,
		},
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": "⏳ 正在处理...",
			},
			"template": "blue",
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "plain_text",
					"content": "任务正在处理中，请稍候...",
				},
			},
		},
	}
	b, _ := json.Marshal(card)
	return string(b)
}

// BuildResultCard 构建"处理完成"绿色卡片 JSON。
// text 为 Agent 回复的富文本（支持 lark_md 语法）。
// imageKeys 为已上传的飞书图片 key 列表，嵌入卡片 img 元素（可为空）。
// 超长文本自动按 maxDivTextLen 分段为多个 div 元素。
func BuildResultCard(text string, imageKeys ...string) string {
	var elements []interface{}

	// 先放文本
	if text != "" {
		elements = append(elements, splitTextToElements(text)...)
	}

	// 追加图片元素
	for _, key := range imageKeys {
		if key == "" {
			continue
		}
		elements = append(elements, map[string]interface{}{
			"tag":     "img",
			"img_key": key,
			"alt": map[string]interface{}{
				"tag":     "plain_text",
				"content": "",
			},
		})
	}

	// 防止空 elements（飞书要求至少一个元素）
	if len(elements) == 0 {
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "（无内容）",
			},
		})
	}

	card := map[string]interface{}{
		"config": map[string]interface{}{
			"update_multi": true,
		},
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": "✅ 处理完成",
			},
			"template": "green",
		},
		"elements": elements,
	}
	b, _ := json.Marshal(card)
	return string(b)
}

// BuildErrorCard 构建"处理失败"红色卡片 JSON。
func BuildErrorCard(errMsg string) string {
	card := map[string]interface{}{
		"config": map[string]interface{}{
			"update_multi": true,
		},
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": "❌ 处理失败",
			},
			"template": "red",
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "plain_text",
					"content": errMsg,
				},
			},
		},
	}
	b, _ := json.Marshal(card)
	return string(b)
}

// splitTextToElements 将长文本拆分为多个 lark_md div 元素。
func splitTextToElements(text string) []interface{} {
	if len([]rune(text)) <= maxDivTextLen {
		return []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": text,
				},
			},
		}
	}

	var elements []interface{}
	runes := []rune(text)
	for i := 0; i < len(runes); {
		end := i + maxDivTextLen
		if end > len(runes) {
			end = len(runes)
		}
		segment := string(runes[i:end])
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": segment,
			},
		})
		i = end
	}
	return elements
}
