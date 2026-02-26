package whatsapp

import (
	"regexp"
	"strings"
)

// WhatsApp vCard 解析 — 继承自 src/web/vcard.ts (83L)

// ParsedVcard 解析后的 vCard 数据
type ParsedVcard struct {
	Name   string   `json:"name,omitempty"`
	Phones []string `json:"phones"`
}

var (
	vcardNewlineRe    = regexp.MustCompile(`(?i)\\n`)
	vcardEscCommaRe   = regexp.MustCompile(`\\,`)
	vcardEscSemiRe    = regexp.MustCompile(`\\;`)
	vcardWhitespaceRe = regexp.MustCompile(`\s+`)
)

// allowedVcardKeys 允许解析的 vCard 属性
var allowedVcardKeys = map[string]bool{
	"FN":  true,
	"N":   true,
	"TEL": true,
}

// ParseVcard 解析 vCard 格式字符串
func ParseVcard(vcard string) ParsedVcard {
	if vcard == "" {
		return ParsedVcard{Phones: []string{}}
	}

	lines := strings.Split(vcard, "\n")
	var nameFromN, nameFromFn string
	var phones []string

	for _, rawLine := range lines {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.ToUpper(line[:colonIdx])
		rawValue := strings.TrimSpace(line[colonIdx+1:])
		if rawValue == "" {
			continue
		}
		baseKey := normalizeVcardKey(key)
		if baseKey == "" || !allowedVcardKeys[baseKey] {
			continue
		}
		value := cleanVcardValue(rawValue)
		if value == "" {
			continue
		}
		switch baseKey {
		case "FN":
			if nameFromFn == "" {
				nameFromFn = normalizeVcardName(value)
			}
		case "N":
			if nameFromN == "" {
				nameFromN = normalizeVcardName(value)
			}
		case "TEL":
			phone := normalizeVcardPhone(value)
			if phone != "" {
				phones = append(phones, phone)
			}
		}
	}

	name := nameFromFn
	if name == "" {
		name = nameFromN
	}
	if phones == nil {
		phones = []string{}
	}
	return ParsedVcard{Name: name, Phones: phones}
}

// normalizeVcardKey 规范化 vCard 键名（如 "item1.TEL;type=CELL" → "TEL"）
func normalizeVcardKey(key string) string {
	primary := key
	if idx := strings.Index(key, ";"); idx != -1 {
		primary = key[:idx]
	}
	if primary == "" {
		return ""
	}
	segments := strings.Split(primary, ".")
	return segments[len(segments)-1]
}

// cleanVcardValue 清理 vCard 值中的转义序列
func cleanVcardValue(value string) string {
	result := vcardNewlineRe.ReplaceAllString(value, " ")
	result = vcardEscCommaRe.ReplaceAllString(result, ",")
	result = vcardEscSemiRe.ReplaceAllString(result, ";")
	return strings.TrimSpace(result)
}

// normalizeVcardName 规范化 vCard 姓名（分号分隔 → 空格分隔）
func normalizeVcardName(value string) string {
	result := strings.ReplaceAll(value, ";", " ")
	result = vcardWhitespaceRe.ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}

// normalizeVcardPhone 规范化 vCard 电话号码
func normalizeVcardPhone(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "tel:") {
		return strings.TrimSpace(trimmed[4:])
	}
	return trimmed
}
