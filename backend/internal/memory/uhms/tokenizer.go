package uhms

// countTokensRune estimates token count using rune-based heuristic.
// ~4 chars/token (English), ~1.5 chars/token (Chinese). ±40% error.
func countTokensRune(text string) int {
	if len(text) == 0 {
		return 0
	}
	return len([]rune(text)) * 2 / 3
}

// truncateToTokensRune truncates text to approximately maxTokens using rune estimation.
func truncateToTokensRune(text string, maxTokens int) string {
	if len(text) == 0 || maxTokens <= 0 {
		return ""
	}
	runes := []rune(text)
	// ~1.5 runes per token
	maxRunes := maxTokens * 3 / 2
	if maxRunes >= len(runes) {
		return text
	}
	return string(runes[:maxRunes])
}
