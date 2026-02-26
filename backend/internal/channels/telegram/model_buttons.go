package telegram

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Telegram 模型选择按钮 — 继承自 src/telegram/model-buttons.ts (218L)

const modelsPageSize = 8
const maxCallbackDataBytes = 64

// ButtonRow 内联按钮行
type ButtonRow []struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// ParsedModelCallback 解析后的模型回调
type ParsedModelCallback struct {
	Type     string // "providers", "list", "select", "back"
	Provider string
	Model    string
	Page     int
}

var (
	mdlListRe = regexp.MustCompile(`(?i)^mdl_list_([a-z0-9_-]+)_(\d+)$`)
	mdlSelRe  = regexp.MustCompile(`^mdl_sel_(.+)$`)
)

// ParseModelCallbackData 解析模型回调数据
func ParseModelCallbackData(data string) *ParsedModelCallback {
	trimmed := strings.TrimSpace(data)
	if !strings.HasPrefix(trimmed, "mdl_") {
		return nil
	}
	if trimmed == "mdl_prov" {
		return &ParsedModelCallback{Type: "providers"}
	}
	if trimmed == "mdl_back" {
		return &ParsedModelCallback{Type: "back"}
	}
	if m := mdlListRe.FindStringSubmatch(trimmed); m != nil {
		page := 1
		if _, err := fmt.Sscanf(m[2], "%d", &page); err == nil && page >= 1 {
			return &ParsedModelCallback{Type: "list", Provider: m[1], Page: page}
		}
	}
	if m := mdlSelRe.FindStringSubmatch(trimmed); m != nil {
		ref := m[1]
		idx := strings.Index(ref, "/")
		if idx > 0 && idx < len(ref)-1 {
			return &ParsedModelCallback{Type: "select", Provider: ref[:idx], Model: ref[idx+1:]}
		}
	}
	return nil
}

// ProviderInfo 提供商信息
type ProviderInfo struct {
	ID    string
	Count int
}

// BuildProviderKeyboard 构建提供商选择键盘（每行2个）
func BuildProviderKeyboard(providers []ProviderInfo) [][]InlineButton {
	if len(providers) == 0 {
		return nil
	}
	var rows [][]InlineButton
	var row []InlineButton
	for _, p := range providers {
		row = append(row, InlineButton{
			Text:         fmt.Sprintf("%s (%d)", p.ID, p.Count),
			CallbackData: fmt.Sprintf("mdl_list_%s_1", p.ID),
		})
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows
}

// BuildModelsKeyboard 构建模型列表键盘（带分页）
// pageSize <= 0 时使用默认值 modelsPageSize。
func BuildModelsKeyboard(provider string, models []string, currentModel string, currentPage, totalPages, pageSize int) [][]InlineButton {
	if pageSize <= 0 {
		pageSize = modelsPageSize
	}
	if len(models) == 0 {
		return [][]InlineButton{{
			{Text: "<< Back", CallbackData: "mdl_back"},
		}}
	}

	var rows [][]InlineButton
	start := (currentPage - 1) * pageSize
	end := start + pageSize
	if end > len(models) {
		end = len(models)
	}

	currentModelID := currentModel
	if idx := strings.Index(currentModel, "/"); idx > 0 {
		currentModelID = currentModel[idx+1:]
	}

	for _, model := range models[start:end] {
		cbData := fmt.Sprintf("mdl_sel_%s/%s", provider, model)
		if len([]byte(cbData)) > maxCallbackDataBytes {
			continue
		}
		display := truncateModelID(model, 38)
		if model == currentModelID {
			display += " ✓"
		}
		rows = append(rows, []InlineButton{{Text: display, CallbackData: cbData}})
	}

	// 分页行
	if totalPages > 1 {
		var pgRow []InlineButton
		if currentPage > 1 {
			pgRow = append(pgRow, InlineButton{Text: "◀ Prev", CallbackData: fmt.Sprintf("mdl_list_%s_%d", provider, currentPage-1)})
		}
		pgRow = append(pgRow, InlineButton{Text: fmt.Sprintf("%d/%d", currentPage, totalPages), CallbackData: fmt.Sprintf("mdl_list_%s_%d", provider, currentPage)})
		if currentPage < totalPages {
			pgRow = append(pgRow, InlineButton{Text: "Next ▶", CallbackData: fmt.Sprintf("mdl_list_%s_%d", provider, currentPage+1)})
		}
		rows = append(rows, pgRow)
	}

	rows = append(rows, []InlineButton{{Text: "<< Back", CallbackData: "mdl_back"}})
	return rows
}

// BuildBrowseProvidersButton 构建浏览提供商按钮
func BuildBrowseProvidersButton() [][]InlineButton {
	return [][]InlineButton{{
		{Text: "Browse providers", CallbackData: "mdl_prov"},
	}}
}

func truncateModelID(modelID string, maxLen int) string {
	runes := []rune(modelID)
	if len(runes) <= maxLen {
		return modelID
	}
	return "…" + string(runes[len(runes)-(maxLen-1):])
}

// GetModelsPageSize 获取分页大小
func GetModelsPageSize() int {
	return modelsPageSize
}

// CalculateTotalPages 计算总页数
func CalculateTotalPages(totalModels, pageSize int) int {
	if pageSize <= 0 {
		pageSize = modelsPageSize
	}
	return int(math.Ceil(float64(totalModels) / float64(pageSize)))
}
