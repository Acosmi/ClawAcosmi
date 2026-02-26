//go:build !cgo

package uhms

// countTokensBPE falls back to rune-based estimation when CGO is not available.
func countTokensBPE(text string) int {
	return countTokensRune(text)
}

// truncateToTokensBPE falls back to rune-based truncation when CGO is not available.
func truncateToTokensBPE(text string, maxTokens int) string {
	return truncateToTokensRune(text, maxTokens)
}
