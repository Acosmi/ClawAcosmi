package pipeline

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// PIIMatch represents a single detected PII entity.
type PIIMatch struct {
	EntityType string  `json:"entity_type"`
	Original   string  `json:"original"`
	Masked     string  `json:"masked"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence"`
}

// PIIFilterResult holds the output of a PII scan.
type PIIFilterResult struct {
	OriginalText string     `json:"original_text"`
	FilteredText string     `json:"filtered_text"`
	Matches      []PIIMatch `json:"matches,omitempty"`
	PIIDetected  bool       `json:"pii_detected"`
}

// piiPattern describes one regex-based PII detector.
type piiPattern struct {
	Regex       *regexp.Regexp
	MaskStyle   string // "partial" or "full"
	Description string
}

// PIIFilter detects and masks PII in text using regex patterns.
type PIIFilter struct {
	patterns map[string]piiPattern
	maskChar rune
}

// defaultPatterns returns the built-in PII detection rules.
func defaultPatterns() map[string]piiPattern {
	return map[string]piiPattern{
		"cn_id_card": {
			Regex:       regexp.MustCompile(`(?:^|[^\d])\d{17}[\dXx](?:[^\d]|$)`),
			MaskStyle:   "partial",
			Description: "Chinese ID card number",
		},
		"cn_phone": {
			Regex:       regexp.MustCompile(`(?:^|[^\d])1[3-9]\d{9}(?:[^\d]|$)`),
			MaskStyle:   "partial",
			Description: "Chinese mobile phone",
		},
		"intl_phone": {
			Regex:       regexp.MustCompile(`\+\d{1,3}[-\s]?\d{6,14}`),
			MaskStyle:   "partial",
			Description: "International phone number",
		},
		"email": {
			Regex:       regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
			MaskStyle:   "partial",
			Description: "Email address",
		},
		"bank_card": {
			Regex:       regexp.MustCompile(`(?:^|[^\d])\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4}(?:[^\d]|$)`),
			MaskStyle:   "partial",
			Description: "Bank card number",
		},
		"ip_address": {
			Regex:       regexp.MustCompile(`(?:^|[^\d])\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(?:[^\d]|$)`),
			MaskStyle:   "full",
			Description: "IP address",
		},
		"passport": {
			Regex:       regexp.MustCompile(`(?:^|[^A-Z])[A-Z]\d{8}(?:[^\d]|$)`),
			MaskStyle:   "full",
			Description: "Passport number",
		},
	}
}

// NewPIIFilter creates a filter with default patterns and '*' mask.
func NewPIIFilter() *PIIFilter {
	return &PIIFilter{
		patterns: defaultPatterns(),
		maskChar: '*',
	}
}

// NewPIIFilterWithTypes creates a filter that only detects the specified types.
func NewPIIFilterWithTypes(enabledTypes []string) *PIIFilter {
	all := defaultPatterns()
	selected := make(map[string]piiPattern, len(enabledTypes))
	for _, t := range enabledTypes {
		if p, ok := all[t]; ok {
			selected[t] = p
		}
	}
	return &PIIFilter{
		patterns: selected,
		maskChar: '*',
	}
}

// Filter scans text for PII and returns the masked version.
func (f *PIIFilter) Filter(text string) PIIFilterResult {
	var matches []PIIMatch

	for entityType, pat := range f.patterns {
		locs := pat.Regex.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			original := text[loc[0]:loc[1]]
			// Trim leading/trailing non-digit boundary chars from lookaround emulation.
			trimmed, trimStart := trimBoundary(original)
			masked := f.maskText(trimmed, pat.MaskStyle)
			matches = append(matches, PIIMatch{
				EntityType: entityType,
				Original:   trimmed,
				Masked:     masked,
				Start:      loc[0] + trimStart,
				End:        loc[0] + trimStart + len(trimmed),
				Confidence: 1.0,
			})
		}
	}

	// Sort by start position descending for safe in-place replacement.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start > matches[j].Start
	})

	filtered := text
	for _, m := range matches {
		filtered = filtered[:m.Start] + m.Masked + filtered[m.End:]
	}

	// Re-sort ascending for output.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start < matches[j].Start
	})

	return PIIFilterResult{
		OriginalText: text,
		FilteredText: filtered,
		Matches:      matches,
		PIIDetected:  len(matches) > 0,
	}
}

// FilterBatch filters multiple texts.
func (f *PIIFilter) FilterBatch(texts []string) []PIIFilterResult {
	results := make([]PIIFilterResult, len(texts))
	for i, t := range texts {
		results[i] = f.Filter(t)
	}
	return results
}

// IsSafe returns true if no PII is detected.
func (f *PIIFilter) IsSafe(text string) bool {
	for _, pat := range f.patterns {
		if pat.Regex.MatchString(text) {
			return false
		}
	}
	return true
}

// maskText masks a string based on style.
func (f *PIIFilter) maskText(text, style string) string {
	mc := string(f.maskChar)
	if style == "partial" && len(text) > 6 {
		return text[:3] + strings.Repeat(mc, len(text)-6) + text[len(text)-3:]
	}
	return strings.Repeat(mc, len(text))
}

// trimBoundary strips leading/trailing non-matching boundary characters
// that leaked through Go regex emulation of lookbehind/lookahead.
func trimBoundary(s string) (string, int) {
	start := 0
	end := len(s)
	if len(s) > 0 && !isDigitOrPlus(s[0]) && !isUpperLetter(s[0]) {
		start = 1
	}
	if len(s) > 1 && !isDigitOrX(s[end-1]) && !isUpperLetter(s[end-1]) {
		end--
	}
	return s[start:end], start
}

func isDigitOrPlus(b byte) bool {
	return (b >= '0' && b <= '9') || b == '+'
}

func isDigitOrX(b byte) bool {
	return (b >= '0' && b <= '9') || b == 'X' || b == 'x'
}

func isUpperLetter(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

// Ensure PIIMatch implements fmt.Stringer for debugging.
func (m PIIMatch) String() string {
	return fmt.Sprintf("[%s] %s → %s (%d:%d)", m.EntityType, m.Original, m.Masked, m.Start, m.End)
}
