package tui

import (
	"encoding/json"
	"strings"
)

// parseJSON attempts to unmarshal JSON from a string that may contain
// surrounding whitespace or markdown fencing.
func parseJSON(text string, v any) error {
	text = strings.TrimSpace(text)
	// Try parsing directly
	if err := json.Unmarshal([]byte(text), v); err == nil {
		return nil
	}
	// Try stripping markdown code block fences
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			inner := strings.Join(lines[1:len(lines)-1], "\n")
			if err := json.Unmarshal([]byte(inner), v); err == nil {
				return nil
			}
		}
	}
	return json.Unmarshal([]byte(text), v)
}
